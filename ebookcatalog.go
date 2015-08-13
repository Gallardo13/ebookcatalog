package main

import (
	"archive/zip"

	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// шаблон html страницы
var t *template.Template

// переменная, присоединяющаяся к имени создаваемых файлов обложки и каждый раз увеличивающаяяся на единицу
// для исключения случаев и одинаковым названием файлов обложек в разных книгах и их перезаписи
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

// папка с книгами
const (
	mainDirectory = "/Users/Gallardo/Desktop/golang/ebookcatalog/files/books/"
)

func init() {
	var err error
	t, err = template.ParseFiles("/Users/Gallardo/Desktop/golang/ebookcatalog/index.html")
	if err != nil {
		log.Fatal("error:", err.Error())
	}
}

func main() {
	i = 0
	books = make([]Book, 0)

	log.Printf("Server start.")
	http.HandleFunc("/", page)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("/Users/Gallardo/Desktop/golang/ebookcatalog/files"))))

	var book string

	directory, err := os.Open("/Users/Gallardo/Desktop/golang/ebookcatalog/files/books/")
	if err != nil {
		log.Fatal("Error")
	}
	defer directory.Close()
	fileNames, err := directory.Readdirnames(0)
	if err != nil {
		log.Fatal("Error0")
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
	// разархивирование файла xml хранящего информацию о расположении файла с метаданными
	if err := unpackFile(book, "", 0); err != nil {
		log.Println(book)
		log.Fatal("Error1")
	}

	// нахождение расположения файла с метаданными
	data, err := ioutil.ReadFile(mainDirectory + "META-INF/container.xml")
	if err != nil {
		log.Println(book)
		log.Fatal("Error2")
	}
	regex1, _ := regexp.Compile("<rootfile full-path=\"([a-zA-Z0-9]|/|(_))*.[a-z]*\"")
	str := string(regex1.Find(data))
	str = strings.Replace(str, "<rootfile full-path=\"", "", 1)
	str = strings.Replace(str, "\"", "", 1)
	pathToMetaFile := mainDirectory + str

	// разархивирование файла с метаданными
	if err := unpackFile(book, str, 1); err != nil {
		log.Println(book)
		log.Fatal("Error3")
	}

	// нахождение нужной информации о книге
	data, err = ioutil.ReadFile(pathToMetaFile)
	if err != nil {
		log.Println(book)
		log.Fatal("Error4")
	}

	// нахождение обложки
	regex2, _ := regexp.Compile("href=\"([a-zA-Z0-9]|/|(_))*.jpg\"")

	str = string(regex2.Find(data))
	str = strings.Replace(str, "href=\"", "", 1)
	str = strings.Replace(str, "\"", "", 1)
	m := strings.Split(str, "/")
	if len(m) > 1 {
		str = m[len(m)-1]
	}

	// разархивирование обложки
	if err = unpackFile(book, str, 2); err != nil {
		log.Println(book)
		log.Fatal("Error5")
	}
	// окончательный путь к распакованной обложке
	pathToPictureFile := mainDirectory + "pictures/" + strconv.Itoa(i) + str

	// нахождение автора
	regex3, _ := regexp.Compile("opf:role=\"aut\">(.*)</dc:creator>")
	author := string(regex3.Find(data))
	author = strings.Replace(author, "opf:role=\"aut\">", "", 1)
	author = strings.Replace(author, "</dc:creator>", "", 1)

	//нахождение названия книги
	regex4, _ := regexp.Compile("<dc:title>(.*)</dc:title>")
	title := string(regex4.Find(data))
	title = strings.Replace(title, "<dc:title>", "", 1)
	title = strings.Replace(title, "</dc:title>", "", 1)

	// нахождение языка
	regex5, _ := regexp.Compile("((RFC3066\")|(dc:language))>(.*)</dc:language>")
	language := string(regex5.Find(data))
	language = strings.Replace(language, "</dc:language>", "", 1)
	language = strings.Replace(language, "dc:language>", "", 1)
	language = strings.Replace(language, "RFC3066\">", "", 1)

	//нахождение краткого содержания
	regex6, _ := regexp.Compile("<dc:description>(.|\n)*</dc:description>")
	description := string(regex6.Find(data))
	description = strings.Replace(description, "<dc:description>", "", 1)
	description = strings.Replace(description, "</dc:description>", "", 1)

	// изменение путей и информации для корректного отображения на сервере и добавление в slice книг
	book = strings.Replace(book, "/Users/Gallardo/Desktop/golang/ebookcatalog/files/", "/static/", 1)
	pathToPictureFile = strings.Replace(pathToPictureFile, "/Users/Gallardo/Desktop/golang/ebookcatalog/files/", "/static/", 1)
	author = "Автор книги: " + author
	switch language {
	case "ru":
		language = "Язык: Русский"
	case "en":
		language = "Язык: Английский"
	case "fr":
		language = "Язык: Французский"
	case "de":
		language = "Язык: Немецкий"
	}
	description = "Описание:" + description
	books = append(books, Book{book, pathToPictureFile, author, title, language, description})
	log.Println("Was added book: " + title)
}

// функция находит нужный файл в архиве и распаковывает его с помощью функции createFile
func unpackFile(book string, way string, flag int) error {
	reader, err := zip.OpenReader(book)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, zipFile := range reader.Reader.File {
		switch flag {
		case 0:
			if zipFile.Name == "META-INF/container.xml" {
				if err = os.MkdirAll("/Users/Gallardo/Desktop/golang/ebookcatalog/files/books/META-INF/", 0755); err != nil {
					return err
				}
				if err = createFile(zipFile.Name, zipFile, 0); err != nil {
					return err
				}
			}
		case 1:
			if zipFile.Name == way {
				splitWay := strings.Split(way, "/")
				if len(splitWay) > 1 {
					if err = os.MkdirAll("/Users/Gallardo/Desktop/golang/ebookcatalog/files/books/"+splitWay[0]+"/", 0755); err != nil {
						return err
					}
				}
				if err = createFile(zipFile.Name, zipFile, 0); err != nil {
					return err
				}
			}
		case 2:
			splitWay := strings.Split(way, "/")
			if len(splitWay) > 1 {
				way = splitWay[len(splitWay)-1]
			}
			if strings.Contains(zipFile.Name, way) {
				if err = os.MkdirAll("/Users/Gallardo/Desktop/golang/ebookcatalog/files/books/pictures/", 0755); err != nil {
					return err
				}
				if err = createFile(way, zipFile, 1); err != nil {
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
		writer, err = os.Create("/Users/Gallardo/Desktop/golang/ebookcatalog/files/books/" + filename)
	} else {
		i++
		writer, err = os.Create("/Users/Gallardo/Desktop/golang/ebookcatalog/files/books/pictures/" + strconv.Itoa(i) + filename)
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
