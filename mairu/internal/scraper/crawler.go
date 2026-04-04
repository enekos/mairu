package scraper

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var assetExtensions = regexp.MustCompile(`(?i)\.(png|jpg|jpeg|gif|svg|webp|ico|css|js|ts|woff|woff2|ttf|eot|pdf|zip|tar|gz|mp4|mp3|wav)$`)

func shouldFollowURL(testURL, seedOrigin, urlPattern string) bool {
	if testURL == "" || strings.HasPrefix(testURL, "#") || strings.HasPrefix(testURL, "mailto:") || strings.HasPrefix(testURL, "tel:") || strings.HasPrefix(testURL, "javascript:") {
		return false
	}
	parsed, err := url.Parse(testURL)
	if err != nil {
		return false
	}
	seedParsed, err := url.Parse(seedOrigin)
	if err != nil {
		return false
	}
	if parsed.Host != "" && parsed.Host != seedParsed.Host {
		return false
	}
	if assetExtensions.MatchString(parsed.Path) {
		return false
	}
	if urlPattern != "" {
		matched, _ := regexp.MatchString(urlPattern, parsed.Path)
		if !matched {
			return false
		}
	}
	return true
}

func normalizeLinks(hrefs []string, baseURL string) []string {
	seen := make(map[string]bool)
	var results []string
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}

	for _, href := range hrefs {
		parsed, err := url.Parse(href)
		if err != nil {
			continue
		}
		abs := base.ResolveReference(parsed)
		abs.Fragment = ""
		normalized := abs.String()
		if strings.HasSuffix(normalized, "/") && len(normalized) > 1 {
			normalized = strings.TrimSuffix(normalized, "/")
		}
		if !seen[normalized] {
			seen[normalized] = true
			results = append(results, normalized)
		}
	}
	return results
}

func filterLinks(urls []string, seedOrigin, urlPattern string) []string {
	var results []string
	for _, u := range urls {
		if shouldFollowURL(u, seedOrigin, urlPattern) {
			results = append(results, u)
		}
	}
	return results
}

func fetchPage(targetURL string) (*CrawledPage, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mairu-crawler/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	html := string(bodyBytes)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	title := doc.Find("title").Text()
	var hrefs []string
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			hrefs = append(hrefs, href)
		}
	})

	return &CrawledPage{
		URL:   targetURL,
		HTML:  html,
		Title: title,
		Links: filterLinks(normalizeLinks(hrefs, targetURL), targetURL, ""),
	}, nil
}

// Crawl performs a breadth-first search of the given seed URL.
// Since Go doesn't have async generators like TS, we use channels.
func Crawl(options CrawlOptions, out chan<- CrawledPage) {
	defer close(out)

	visited := make(map[string]bool)
	type queueItem struct {
		url   string
		depth int
	}
	queue := []queueItem{{url: options.SeedURL, depth: 0}}
	pageCount := 0

	for len(queue) > 0 && pageCount < options.MaxPages {
		currentLevel := queue
		queue = nil

		var toProcess []queueItem
		for _, item := range currentLevel {
			if !visited[item.url] {
				visited[item.url] = true
				toProcess = append(toProcess, item)
			}
		}

		concurrency := options.Concurrency
		if concurrency <= 0 {
			concurrency = 1
		}

		for i := 0; i < len(toProcess); i += concurrency {
			if pageCount >= options.MaxPages {
				break
			}
			end := i + concurrency
			if end > len(toProcess) {
				end = len(toProcess)
			}
			chunk := toProcess[i:end]

			var wg sync.WaitGroup
			results := make([]*CrawledPage, len(chunk))
			for j, item := range chunk {
				wg.Add(1)
				go func(idx int, qItem queueItem) {
					defer wg.Done()
					page, err := fetchPage(qItem.url)
					if err == nil && page != nil {
						page.Depth = qItem.depth
						results[idx] = page
					}
				}(j, item)
			}
			wg.Wait()

			for _, page := range results {
				if page == nil || pageCount >= options.MaxPages {
					continue
				}
				page.Links = filterLinks(page.Links, options.SeedURL, options.URLPattern)
				pageCount++
				out <- *page

				if page.Depth < options.MaxDepth {
					for _, link := range page.Links {
						if !visited[link] && pageCount < options.MaxPages {
							queue = append(queue, queueItem{url: link, depth: page.Depth + 1})
						}
					}
				}
			}

			if options.DelayMs > 0 && len(queue) > 0 {
				time.Sleep(time.Duration(options.DelayMs) * time.Millisecond)
			}
		}
	}
}
