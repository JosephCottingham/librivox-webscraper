package main

import (
	"fmt"
	"os"
	"log"
	"context"
	"strconv"
	"time"
	"flag"
	"encoding/json"
	"math/rand"
	"errors"
	// "strings"
	"io"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"net/http"
)


type Book struct {
	Title string `json:"title"`
	Author string `json:"author"`
	Reader string `json:"reader"`
	Language string `json:"language"`
	Genre string `json:"genre"`
	Audio_file_count string `json:"audio_file_count"`
	Audio_download_url string `json:"Audio_download_url"`
}

func DownloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func randomString(length int) string {
    rand.Seed(time.Now().UnixNano())
    b := make([]byte, length)
    rand.Read(b)
    return fmt.Sprintf("%x", b)[:length]
}

func save(htmlString string, fileName string, outputDir string) {
	createDir(outputDir)
	htmlDir := outputDir + "/" + fileName
	fmt.Println(htmlDir)
	htmlFile, _ := os.Create(htmlDir)
	defer htmlFile.Close()
	htmlFile.WriteString(htmlString)
}

func createDir(outputDir string) {
	if _, err := os.Stat(outputDir); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(outputDir, os.ModePerm)
		if err != nil {
			log.Println(err)
		}
	}
}

func main() {
	outputDirPointer := flag.String("output", "output", "a string")
	outDir := *outputDirPointer

	fmt.Println("Starting")
	
	// tracks the current search page (pagniation on the website)
	var cur_page_counter int16 = 0

	// This max page counter is set as of 5/14/2022 and is subject to change (1516)
	var max_page_counter int16 = 1516

	var book_pages []string

	var nodes []*cdp.Node

	ctxt, cancel := chromedp.NewContext(
		context.Background(),
	)
	// ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	for cur_page_counter <= max_page_counter {
		cur_page_counter++
		fmt.Printf("Page %d\n", cur_page_counter)
		search_url := "https://librivox.org/search?primary_key=1&search_category=language&search_page=" + strconv.FormatInt(int64(cur_page_counter), 10) + "&search_form=get_results"

		fmt.Printf("%s\n", search_url)
		
		err := chromedp.Run(ctxt, collectBookUrls(search_url, &nodes))
		if err != nil {
			fmt.Println(err)
		}

		for _, n := range nodes {
			book_pages = append(book_pages, n.AttributeValue("href"))
		}

		fmt.Printf("Complete %d\n", cur_page_counter)
	}

	
	createDir(outDir)
	outDir=outDir+"/"+time.Now().Format("2006-01-02-15:04:05")
	createDir(outDir)

	for _, book_url := range book_pages {
		fmt.Println(book_url)
		b := Book{}
		var nodesBtn []*cdp.Node
		err := chromedp.Run(ctxt, collectBookData(book_url, &b.Title, &b.Author, &b.Reader, &b.Language, &b.Genre, &b.Audio_file_count, &b.Audio_download_url))
		if err != nil {
			fmt.Println(err)
		}

		fmt.Println(len(nodesBtn))
		for _, n := range nodesBtn {
			fmt.Println("Test")
			fmt.Println(n.AttributeValue("href"))
		}

		j, err := json.Marshal(&b)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(string(j))

		tempDir := outDir+"/"+randomString(16)

		save(string(j), "meta.json", tempDir)
			

		fmt.Printf("Complete %s\n", b.Title)
		err = DownloadFile(tempDir+"audio.zip", b.Audio_download_url)
		if err != nil {
			panic(err)
		}
		// fmt.Printf("%s %s %s %s %s\n", author, reader, language, genre, audio_file_count)
	}

	// err = chromedp.Shutdown(ctxt)

	// err = chromedp.Wait()
}



func collectBookUrls(url string, nodes *[]*cdp.Node) chromedp.Tasks{
	// Gather all the book urls on a single page
	return chromedp.Tasks{
		chromedp.Navigate(url),
		// chromedp.Sleep(.1 * time.Second),
		chromedp.Nodes(".catalog-result .book-cover", nodes),
	}
}


func collectBookData(url string, title *string, author *string, reader *string, language *string, genre *string, audio_file_count *string, audio_download_url *string) chromedp.Tasks{
	// Gather all the book urls on a single page
	var ok bool
	return chromedp.Tasks{
		chromedp.Navigate(url),
		chromedp.Text(".content-wrap h1", title),
		chromedp.Text(".book-page-author a", author, chromedp.AtLeast(0)),
		chromedp.Text(".product-details dd:nth-child(4)", reader, chromedp.AtLeast(0)),
		// chromedp.Text(".book-page-genre:nth-child(2)", language, chromedp.AtLeast(0)),
		// chromedp.Text(".book-page-genre:nth-child(1)", genre, chromedp.AtLeast(0)),
		chromedp.Text(".chapter-download tbody:last-child tr:last-child td:first-child", audio_file_count),
		chromedp.WaitReady(`.listen-download dd:nth-child(2) .book-download-btn`, chromedp.ByQuery),
		chromedp.AttributeValue(".listen-download dd:nth-child(2) .book-download-btn", "href", audio_download_url, &ok),
		// chromedp.Text(".listen-download dd:nth-child(2) .book-download-btn", audio_download_url),
	}
}




