package main

import (
	"fmt"
	"bufio"
	"os"
	"log"
	"context"
	"strconv"
	"time"
	"flag"
	"encoding/json"
	"math/rand"
	"errors"
	"io"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"net/http"
)

// Store device for each book key data and utlized in JSON export.
type Book struct {
	Title string `json:"title"`
	Author string `json:"author"`
	Reader string `json:"reader"`
	Language string `json:"language"`
	Genre string `json:"genre"`
	Audio_file_count string `json:"audio_file_count"`
	Audio_download_url string `json:"Audio_download_url"`
}

func Exists(name string) (bool, error) {
    _, err := os.Stat(name)
    if err == nil {
        return true, nil
    }
    if errors.Is(err, os.ErrNotExist) {
        return false, nil
    }
    return false, err
}

// Downloads a resource for a given URL to given location on disk.
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

// Generates a random string of a given length
func randomString(length int) string {
    rand.Seed(time.Now().UnixNano())
    b := make([]byte, length)
    rand.Read(b)
    return fmt.Sprintf("%x", b)[:length]
}

// Save string as file in a given directory with a given file name.
func save(htmlString string, fileName string, outputDir string) {
	createDir(outputDir)
	htmlDir := outputDir + "/" + fileName
	htmlFile, _ := os.Create(htmlDir)
	defer htmlFile.Close()
	htmlFile.WriteString(htmlString)
}

// Checks if a directory is exist and if it does not then create the directory.
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

	// This max page counter is set as of 5/14/2022 and is subject to change (1516) (english only)
	var max_page_counter int16 = 1516

	var book_pages []string

	var nodes []*cdp.Node

	opts := []chromedp.ExecAllocatorOption{
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36"),
		chromedp.WindowSize(1920, 1080),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.DisableGPU,
	}

	ctx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()


	ctxt, cancel := chromedp.NewContext(ctx)
	// ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create custom directory that ties output to time
	createDir(outDir)
	// outDir=outDir+"/"+time.Now().Format("2006-01-02-15:04:05")
	// createDir(outDir)

	var book_url_path = outDir+"/"+"book_url.txt"
	var state_path = outDir+"/"+"current_row.txt"
	var book_url_exists, _ = Exists(book_url_path)


	if (book_url_exists == false ) {
		book_url_file, _ := os.Create(book_url_path)
		state_file, _ := os.Create(state_path)

		state_file.WriteString("0")

		// Collect book detail view url's on each of the search pages
		for cur_page_counter <= max_page_counter {
			time.Sleep(2)
			// interate the page
			cur_page_counter++
			fmt.Printf("Start Page %d\n", cur_page_counter)
			// create temp url for the page of intrest in this iteration
			search_url := "https://librivox.org/search?primary_key=1&search_category=language&search_page=" + strconv.FormatInt(int64(cur_page_counter), 10) + "&search_form=get_results"
			
			// Scrape all book open buttons on this page.
			err := chromedp.Run(ctxt, collectBookUrls(search_url, &nodes))
			if err != nil {
				fmt.Println(err)
			}

			// Collect each href for the scaped button
			for i, n := range nodes {
				book_pages = append(book_pages, n.AttributeValue("href"))
				fmt.Printf("Book: %d\t", i)
				book_url_file.WriteString(n.AttributeValue("href")+"\n")
				// fmt.Println("\t...Complete")
			}

			fmt.Printf("Complete Page %d\n", cur_page_counter)
			break
		}
		state_file.Close()
		book_url_file.Close()
	}

	book_url_file, _ := os.Open(book_url_path)
	book_url_scanner := bufio.NewScanner(book_url_file)

	state_val_string, _ := os.ReadFile(state_path)
	state_val, _ := strconv.Atoi(string(state_val_string))
	total_lines := 0
	for book_url_scanner.Scan() {
		total_lines++
	}
	book_url_file.Close()
	

	var curr_row = 0
	var scrape_start = false

	book_url_file, _ = os.Open(book_url_path)
	book_url_scanner = bufio.NewScanner(book_url_file)

	for book_url_scanner.Scan() {

		if (curr_row == state_val) {
			scrape_start = true
		}

		if (scrape_start) {
			fmt.Printf("Book: %d / %d\n", state_val, total_lines)
			scrapeBook(book_url_scanner.Text(), outDir, ctxt)
			state_val++
			state_file, _ := os.Create(state_path)
			state_file.WriteString(strconv.Itoa(state_val))
			state_file.Close()
		}
		curr_row++
   }

	// err := chromedp.Shutdown(ctxt)

	// err = chromedp.Wait()
	fmt.Println("Complete")
}


// Collect book information for each detail page and save it to a file, and save the audio zip.
func scrapeBook(book_url string, outDir string, ctxt context.Context) {
    fmt.Println("Time: ", time.Now().Format("2006-01-02 15:04:05"))
	// Stores the book
	b := Book{}
	// Scape the site for data
	err := chromedp.Run(ctxt, collectBookData(book_url, &b.Title, &b.Author, &b.Reader, &b.Language, &b.Genre, &b.Audio_file_count, &b.Audio_download_url))
	if err != nil {
		fmt.Println(err)
	}

	// Format struct as json string
	j, err := json.Marshal(&b)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Save the meta data about the book
	tempDir := outDir+"/"+randomString(16)
	save(string(j), "meta.json", tempDir)
		
	// Download the zip file that holds the audio (leave compressed so that it is more mobile)
	err = DownloadFile(tempDir+"/"+"audio.zip", b.Audio_download_url)
	if err != nil {
		fmt.Println(err)
        return
    }
	fmt.Printf("Complete %s\n", b.Title)
}

// Collect the url of the detail view page of each book.
func collectBookUrls(url string, nodes *[]*cdp.Node) chromedp.Tasks{
	// Gather all the book urls on a single page
	return chromedp.Tasks{
		chromedp.Navigate(url),
		// chromedp.Sleep(.1 * time.Second),
		chromedp.Nodes(".catalog-result .book-cover", nodes),
	}
}

// Gather data about book on book detail page.
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
