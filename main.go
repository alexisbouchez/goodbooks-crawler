package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gocolly/colly/v2"
)

type Person struct {
	Name        string   `json:"name"`
	Industries  []string `json:"industries"`
	Occupations []string `json:"occupations"`
	BookSlugs   []string `json:"bookSlugs"`
	ImagePath   string   `json:"imagePath"`
}

type Book struct {
	Title       string   `json:"title"`
	Authors     []string `json:"authors"`
	Genres      []string `json:"genres"`
	Description string   `json:"description"`
	ImagePath   string   `json:"imagePath"`
}

func DownloadImage(url string, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func WriteJSONToFile(file io.Writer, data any) {
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")

	enc.Encode(data)
}

func main() {
	fName := "people.json"
	file, err := os.Create(fName)
	if err != nil {
		log.Fatalf("Cannot create file %q: %s\n", fName, err)
		return
	}
	defer file.Close()

	booksFile, err := os.Create("books.json")
	if err != nil {
		log.Fatalf("Cannot create file %q: %s\n", "books.json", err)
		return
	}
	defer booksFile.Close()

	people := []Person{}
	books := make(map[string]Book)

	getBookBySlug := func(slug string) *Book {
		if book, ok := books[slug]; ok {
			return &book
		}

		return nil
	}

	c := colly.NewCollector(
		colly.AllowedDomains("goodbooks.io", "www.goodbooks.io"),

		colly.CacheDir("./goodbooks_cache"),
	)
	detailCollector := c.Clone()
	bookDetailCollector := c.Clone()

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		if e.Attr("class") != "people-wrap people-wrap-sidebar w-inline-block" {
			return
		}

		personURL := e.Request.AbsoluteURL(e.Attr("href"))
		detailCollector.Visit(personURL)
	})

	bookDetailCollector.OnHTML(".right-side", func(e *colly.HTMLElement) {
		splittedURL := strings.Split(e.Request.URL.String(), "/")
		slug := splittedURL[len(splittedURL)-1]
		book := getBookBySlug(slug)
		if book == nil {
			return
		}
		text := ""

		e.ForEach("p", func(_ int, e *colly.HTMLElement) {
			text += e.Text
		})

		var genres []string

		e.ForEach("div.badge-text", func(_ int, e *colly.HTMLElement) {
			genres = append(genres, e.Text)
		})

		book.Description = text
		book.Genres = genres
		books[slug] = *book
	})

	detailCollector.OnHTML("body", func(e *colly.HTMLElement) {
		title := strings.TrimSpace(e.ChildText("h1.h1"))
		if !strings.HasPrefix(title, "books recommended by") {
			return
		}
		name := strings.Replace(title, "books recommended by ", "", 1)

		var industries []string
		var occupations []string
		var bookSlugs []string

		imageUrl := e.ChildAttr("img.people-photo", "src")
		splitted := strings.Split(imageUrl, "/")
		imagePath := fmt.Sprintf("./photos/%s", splitted[len(splitted)-1])
		DownloadImage(imageUrl, imagePath)

		e.ForEach(".badge.badge-large.w-inline-block", func(_ int, e *colly.HTMLElement) {
			href := e.Attr("href")
			if !strings.HasPrefix(href, "/industries/") {
				return
			}

			industries = append(industries, strings.Replace(href, "/industries/", "", 1))
			occupations = append(occupations, e.Text)
		})

		e.ForEach(".book-wrap", func(_ int, e *colly.HTMLElement) {
			title := e.ChildText("h5")
			author := e.ChildText("h6")
			imageUrl := e.ChildAttr("img.book-cover", "src")
			splitted := strings.Split(imageUrl, "/")
			imagePath := fmt.Sprintf("./covers/%s", splitted[len(splitted)-1])
			href := e.ChildAttr("a", "href")
			splittedHref := strings.Split(href, "/")
			DownloadImage(imageUrl, imagePath)
			slug := splittedHref[len(splittedHref)-1]
			bookSlugs = append(bookSlugs, slug)

			bookFoundBySlug := getBookBySlug(slug)
			if bookFoundBySlug != nil {
				return
			}

			book := Book{Title: title, Authors: strings.Split(author, " & "), ImagePath: imagePath}
			books[slug] = book

			bookDetailCollector.Visit(fmt.Sprintf("https://www.goodbooks.io/books/%s", slug))
		})

		person := Person{Name: name, Industries: industries, Occupations: occupations, BookSlugs: bookSlugs, ImagePath: imagePath}
		people = append(people, person)
	})

	c.Visit("https://www.goodbooks.io/people/")

	WriteJSONToFile(file, people)
	WriteJSONToFile(booksFile, books)
}
