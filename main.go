package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type SeoData struct {
	URL             string
	Title           string
	H1              string
	MetaDescription string
	StatusCode      int
}

type Parser interface {
	getSEOData(resp *http.Response) (SeoData, error)
}

type DefaultParser struct {
}

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.5060.134 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:94.0) Gecko/20100101 Firefox/94.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Version/14.0.3 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.902.67 Safari/537.36 Edg/92.0.902.67",
}

func randomUserAgents() string {
	rand.NewSource(time.Now().Unix())
	randNum := rand.Int() % len(userAgents)
	return userAgents[randNum]
}

func isSitemap(urls []string) ([]string, []string) {
	sitemapFiles := []string{}
	pages := []string{}
	for _, page := range urls {
		foundSitemap := strings.Contains(page, "xml")
		if foundSitemap == true {
			fmt.Println("Found Sitemap", page)
			sitemapFiles = append(sitemapFiles, page)
		} else {
			pages = append(pages, page)
		}
	}
	return sitemapFiles, pages
}

func extractSiteMapUrls(startURL string) []string {
	worklist := make(chan []string)
	toCrawl := []string{}
	var n int
	n++
	go func() { worklist <- []string{startURL} }()

	for ; n > 0; n-- {

		list := <-worklist
		for _, link := range list {
			n++
			go func(link string) {
				response, err := makeRequest(link)
				if err != nil {
					log.Printf("Error retrieving URL: %s", link)
				}
				urls, _ := extractUrls(response)
				if err != nil {
					log.Printf("Error extracting document from response, URL: %s", link)
				}
				sitemapFiles, pages := isSitemap(urls)
				if sitemapFiles != nil {
					worklist <- sitemapFiles
				}
				for _, page := range pages {
					toCrawl = append(toCrawl, page)
				}
			}(link)
		}
	}
	return toCrawl

}

func makeRequest(url string) (*http.Response, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", randomUserAgents())
	if err != nil {
		return nil, err
	}
	res, err := client.Do((req))
	if err != nil {
		return nil, err
	}
	return res, nil
}

func scrapeURLs(urls []string, parser Parser, concurrency int) []SeoData {
	tokens := make(chan struct{}, concurrency)
	var n int
	worklist := make(chan []string)
	results := []SeoData{}

	go func() { worklist <- urls }()
	for ; n > 0; n-- {
		list := <-worklist
		for _, url := range list {
			if url != "" {
				n++
				go func(url string, token chan struct{}) {
					log.Printf("Requesting URL: %s", url)
					res, err := scrapePage(url, tokens, parser)
					if err != nil {
						log.Printf("Encountered error, URL:%s", url)
					} else {
						results = append(results, res)
					}
					worklist <- []string{}
				}(url, tokens)
			}
		}
	}
	return results
}

func extractUrls(response *http.Response) ([]string, error) {
	doc, err := goquery.NewDocumentFromResponse(response)
	if err != nil {
		return nil, err
	}
	results := []string{}
	sel := doc.Find("loc")
	for i := range sel.Nodes {
		loc := sel.Eq(i)
		result := loc.Text()
		results = append(results, result)
	}
	return results, nil
}

func scrapePage(url string, token chan struct{}, parser Parser) (SeoData, error) {
	res, err := crawlPage(url, token)
	if err != nil {
		return SeoData{}, err
	}
	data, err := parser.getSEOData(res)
	if err != nil {
		return SeoData{}, err
	}
	return data, nil
}

func crawlPage(url string, tokens chan struct{}) (*http.Response, error) {
	// crawl page takes the url as well as the token
	tokens <- struct{}{}
	resp, err := makeRequest(url)
	<-tokens
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (d DefaultParser) getSEOData(resp *http.Response) (SeoData, error) {
	doc, err := goquery.NewDocumentFromResponse(resp)

	if err != nil {
		return SeoData{}, nil
	}
	results := SeoData{}
	results.URL = resp.Request.URL.String()
	results.StatusCode = resp.StatusCode
	results.Title = doc.Find("title").First().Text()
	results.H1 = doc.Find("h1").First().Text()
	results.MetaDescription, _ = doc.Find("meta[name^=description]").Attr("content")
	return results, nil
}

func ScrapeSiteMap(url string, parser Parser, concurrency int) []SeoData {
	results := extractSiteMapUrls(url)
	res := scrapeURLs(results, parser, concurrency)
	return res
}

func main() {
	p := DefaultParser{}
	results := ScrapeSiteMap("https://www.google.com/sitemap.xml", p, 10)
	for _, res := range results {
		fmt.Println(res)
	}
}
