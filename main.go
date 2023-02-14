package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

// Log a message, err and kill it
// Size and job struct
type Size struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}
type Job struct {
	URL     string
	Try     int
	err     error
	Success bool
}

var flip bool

func init() {
	//
	flipImageNaming := flag.Bool("flip", false, "flips image naming")
	flag.Parse()
	if *flipImageNaming {
		flip = true
		fmt.Println("Congrats, you read this code so we are going to flip the image naming order ")
	}

}
func main() {

	// read sizes file and unmarshal intro slice of Size
	var sizes []Size
	file, _ := ioutil.ReadFile("sizes.json")
	err := json.Unmarshal([]byte(file), &sizes)
	if err != nil {
		fmt.Println("err", err)
	}
	// range over sizes to show in the CLI
	for i, v := range sizes {
		fmt.Println("Size# ", i, " Width: ", v.Width, " Height: ", v.Height)
	}

	// Reading over file and splitting on each line
	urls := readFileURLs()

	// checks to make sure there is a output directory, if not it makes one
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("could not get cwd %s", err)
	}
	if err := os.Mkdir(filepath.Join(cwd, "out"), 0777); err != nil && !os.IsExist(err) {
		if err != nil {
			log.Fatalf("could not create output directory %s", err)
		}
	}

	// starting playwright
	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("could not launch playwright: %w", err)
	}
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true), // change to false if you want lots of browser windows and to have a hard time closing a terminal
	})
	if err != nil {
		log.Fatalf("could not launch Chromium: %w", err)

	}
	// Used to set max number of jobs to run at a time
	numberOfJobs := int(math.Min(30, float64(len(urls))))

	jobs := make(chan Job, numberOfJobs)
	results := make(chan Job, numberOfJobs)

	// concurrent worker
	for w := 1; w <= 50; w++ {
		go worker(w, jobs, results, browser, sizes)
	}

	// ranging over each job and pushes to hobs channel
	for _, url := range urls[:numberOfJobs] {
		jobs <- Job{
			URL: url,
		}
	}
	for a := 0; a < numberOfJobs; a++ {
		job := <-results
		if job.Success {
			fmt.Println("success:", job.URL)
		} else {
			fmt.Println("error:", job.URL, job.err)
		}
	}

	close(jobs)
	close(results)

	if err != nil {
		log.Fatalf("could not close browser or stop playwright %s", browser.Close())
	}
}

func readFileURLs() []string {
	readFile, err := os.Open("./urls.txt")

	if err != nil {
		fmt.Println("there was an error with your urls.txt file, please check it again")
	}
	fileScanner := bufio.NewScanner(readFile)

	fileScanner.Split(bufio.ScanLines)

	var u []string
	for fileScanner.Scan() {
		var x = fileScanner.Text()
		u = append(u, x)
		fmt.Println("URL IS", x)
	}
	return u
}

// processJob is what starts the browser. It takes a list of sizes and runs it through each job/url passed in
func processJob(browser playwright.Browser, job Job, ctx context.Context, sizes []Size) error {
	// looping over each size, we are starting a new browser window
	for _, size := range sizes {
		context, err := browser.NewContext(playwright.BrowserNewContextOptions{
			UserAgent: playwright.String("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/84.0.4147.135 Safari/537.36"),
			Viewport: &playwright.BrowserNewContextOptionsViewport{
				Width:  playwright.Int(size.Width),
				Height: playwright.Int(size.Height),
			},
		})
		if err != nil {
			return fmt.Errorf("could not create context: %w", err)
		}
		defer context.Close()
		go func() {
			<-ctx.Done()
			context.Close()
		}()
		// creating page
		page, err := context.NewPage(playwright.BrowserNewPageOptions{})
		// actually visiting the page
		_, err = page.Goto(job.URL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateNetworkidle,
		})
		if err != nil {
			return fmt.Errorf("could not goto: %s: %v", job.URL, err)
		}
		// writing to the working directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("could not get cwd %w", err)
		}
		// rough formatting turning urls with slashes into something that can be stored as an image
		x := strings.Replace(job.URL, "/", "_", -1)
		x = strings.Replace(x, "https", "", -1)

		if flip {
			x = strconv.Itoa(size.Width) + "x" + strconv.Itoa(size.Height) + x + ".png"
		} else {
			x = x + strconv.Itoa(size.Width) + "x" + strconv.Itoa(size.Height) + ".png"
		}

		fmt.Println("x", x) // printing URL/image that is being saved
		_, err = page.Screenshot(playwright.PageScreenshotOptions{
			Path:     playwright.String(filepath.Join(cwd, "out", x)),
			FullPage: playwright.Bool(true),
		})
		if err != nil {
			return fmt.Errorf("could not screenshot: %w", err)
		}

	}
	return nil
}

// worker processes the jobs which are passed in through a channel
func worker(id int, jobs chan Job, results chan<- Job, browser playwright.Browser, sizes []Size) {
	for job := range jobs {
		fmt.Printf("starting (try: %d): %s\n", job.Try, job.URL)
		// can adjust so it loops again if it fails
		if job.Try >= 1 {
			job.Success = false
			job.err = fmt.Errorf("Stopped with domain %s (%w)", job.URL, job.err)
			results <- job
			continue
		}
		//setting maximum context time and when it errors
		jobCtx, cancel := context.WithTimeout(context.Background(), time.Second*120)
		internalJobError := make(chan error, 1)
		internalJobError <- processJob(browser, job, jobCtx, sizes)
		cancel()

		select {
		case <-jobCtx.Done():
			job.err = fmt.Errorf("timeout (try: %d)", job.Try+1)
			job.Success = false
			job.Try++
			jobs <- job
		case err := <-internalJobError:
			// if it errors push it back to the channel and cancel the context
			if err != nil {
				job.err = err
				job.Success = false
				job.Try++
				jobs <- job
				cancel()
			} else {
				job.Success = true
				job.err = nil
				results <- job
			}
		}
	}
}
