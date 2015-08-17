package main

import (
	"archive/zip"
	"encoding/xml"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// шаблон html страницы
var t *template.Template

// переменная, присоединяющаяся к имени создаваемых файлов обложки и каждый раз увеличивающаяяся на единицу
// для исключения случаев с одинаковым названием файлов обложек в разных книгах и их перезаписи
var i int

// структура хранящая информацию о книгах
type Book struct {
	Name        string
	Picture     string
	Author      string
	Title       string
	Language    string
	Description string
}

// slice книг
var books []Book

// структуры xml файлов
type metaData struct {
	XMLName     xml.Name `xml:"metadata"`
	Author      string   `xml:"creator"`
	Title       string   `xml:"title"`
	Language    string   `xml:"language"`
	Description string   `xml:"description"`
}
type item struct {
	XMLName   xml.Name `xml:"item"`
	Href      string   `xml:"href,attr"`
	Id        string   `xml:"id,attr"`
	MediaType string   `xml:"media-type,attr"`
}
type manifest struct {
	XMLName xml.Name `xml:"manifest"`
	Items   []item   `xml:"item"`
}
type Package struct {
	XMLName  xml.Name `xml:"package"`
	Metadata metaData `xml:"metadata"`
	Manifest manifest `xml:"manifest"`
}
type rootfile struct {
	XMLName   xml.Name `xml:"rootfile"`
	FullPath  string   `xml:"full-path,attr"`
	MediaType string   `xml:"media-type, attr"`
}
type rootfiles struct {
	XMLName  xml.Name `xml:"rootfiles"`
	Rootfile rootfile `xml:"rootfile"`
}
type Container struct {
	XMLName   xml.Name  `xml:"container"`
	Rootfiles rootfiles `xml:"rootfiles"`
}

// папка с книгами
const (
	mainDirectory = "files/books/"
)

func init() {
	var err error
	t, err = template.ParseFiles("index.html")
	if err != nil {
		log.Fatal("error:", err.Error())
	}
}

func main() {
	i = 0
	books = make([]Book, 0)

	log.Printf("Server start.")
	http.HandleFunc("/", page)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("files"))))

	var book string

	directory, err := os.Open(mainDirectory)
	if err != nil {
		log.Fatal("Error: ", err)
	}
	defer directory.Close()
	fileNames, err := directory.Readdirnames(0)
	if err != nil {
		log.Fatal("Error0: ", err)
	}

	for _, file := range fileNames {
		if strings.Contains(file, ".epub") {
			book = mainDirectory + file
			process(book)
		}
	}

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func page(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/html")
	t.ExecuteTemplate(writer, "index", books)
}

// функция обрабатывает книгу book, вычленяя нужную информацию
func process(book string) {

	reader, err := zip.OpenReader(book)
	if err != nil {
		log.Println("Error: can't open book")
		return
	}
	defer reader.Close()

	// разархивирование файла xml хранящего информацию о расположении файла с метаданными
	if err := unpackFile(reader, book, "", 0); err != nil {
		log.Println(book)
		log.Fatal("Error1: ", err)
	}

	// нахождение расположения файла с метаданными
	data, err := ioutil.ReadFile(mainDirectory + "META-INF/container.xml")
	if err != nil {
		log.Println(book)
		log.Fatal("Error2: ", err)
	}
	var c Container
	xml.Unmarshal(data, &c)
	pathToMetaFile := mainDirectory + c.Rootfiles.Rootfile.FullPath

	// разархивирование файла с метаданными
	if err := unpackFile(reader, book, c.Rootfiles.Rootfile.FullPath, 1); err != nil {
		log.Println(book)
		log.Fatal("Error3: ", err)
	}

	// чтение нужной информации из файла с данными о книге
	data, err = ioutil.ReadFile(pathToMetaFile)
	if err != nil {
		log.Println(book)
		log.Fatal("Error4: ", err)
	}
	var p Package
	xml.Unmarshal(data, &p)

	// нахождение обложки
	var str string
	for _, item := range p.Manifest.Items {
		if (strings.Contains(item.Href, "co") || strings.Contains(item.Id, "co")) && item.MediaType == "image/jpeg" {
			str = item.Href
			break
		}
	}

	// разархивирование обложки
	if err = unpackFile(reader, book, str, 2); err != nil {
		log.Println(book)
		log.Fatal("Error5: ", err)
	}
	split := strings.Split(str, "/")
	if len(split) > 1 {
		str = split[len(split)-1]
	}
	// окончательный путь к распакованной обложке
	pathToPictureFile := mainDirectory + "pictures/" + strconv.Itoa(i) + str

	// изменение путей и информации для корректного отображения на сервере и добавление в slice книг
	book = strings.Replace(book, "files/", "/static/", 1)
	pathToPictureFile = strings.Replace(pathToPictureFile, "files/", "/static/", 1)
	p.Metadata.Author = "Автор книги: " + p.Metadata.Author
	switch p.Metadata.Language {
	case "ru":
		p.Metadata.Language = "Язык: Русский"
	case "en":
		p.Metadata.Language = "Язык: Английский"
	case "fr":
		p.Metadata.Language = "Язык: Французский"
	case "de":
		p.Metadata.Language = "Язык: Немецкий"
	}
	p.Metadata.Description = "Описание:" + p.Metadata.Description
	books = append(books, Book{book, pathToPictureFile, p.Metadata.Author, p.Metadata.Title, p.Metadata.Language, p.Metadata.Description})
	log.Println("Was added book: " + p.Metadata.Title)
}

// функция находит нужный файл в архиве и распаковывает его с помощью функции createFile
func unpackFile(reader *zip.ReadCloser, book string, way string, flag int) error {
	for _, zipFile := range reader.Reader.File {
		switch flag {
		case 0:
			if zipFile.Name == "META-INF/container.xml" {
				if err := os.MkdirAll("files/books/META-INF/", 0755); err != nil {
					return err
				}
				if err := createFile(zipFile.Name, zipFile, 0); err != nil {
					return err
				}
			}
		case 1:
			if zipFile.Name == way {
				splitWay := strings.Split(way, "/")
				if len(splitWay) > 1 {
					if err := os.MkdirAll("files/books/"+splitWay[0]+"/", 0755); err != nil {
						return err
					}
				}
				if err := createFile(zipFile.Name, zipFile, 0); err != nil {
					return err
				}
			}
		case 2:
			splitWay := strings.Split(way, "/")
			if len(splitWay) > 1 {
				way = splitWay[len(splitWay)-1]
			}
			if strings.Contains(zipFile.Name, way) {
				if err := os.MkdirAll("files/books/pictures/", 0755); err != nil {
					return err
				}
				if err := createFile(way, zipFile, 1); err != nil {
					return err
				}
			}
		}

	}
	return nil
}

func createFile(filename string, zipFile *zip.File, flag int) error {
	if filename == "" {
		return nil
	}
	var writer *os.File
	var err error
	if flag == 0 {
		writer, err = os.Create("files/books/" + filename)
	} else {
		i++
		writer, err = os.Create("files/books/pictures/" + strconv.Itoa(i) + filename)
	}
	if err != nil {
		return err
	}
	defer writer.Close()
	reader, err := zipFile.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	if _, err = io.Copy(writer, reader); err != nil {
		return err
	}
	return nil
}
