package scraper

import (
	"context"

	"mairu/internal/contextsrv"
)

func ScrapeAndIngest(ctx context.Context, options ScrapeOptions, service contextsrv.Service, geminiAPIKey string) (*ScrapeResult, error) {
	out := make(chan CrawledPage, 10)
	go Crawl(options.CrawlOptions, out)

	result := &ScrapeResult{}
	for page := range out {
		result.PagesTotal++
		content := ExtractContent(page.HTML, options.Selector)
		if content.Markdown == "" {
			result.PagesSkipped++
			continue
		}

		summary := SummarizePage(ctx, geminiAPIKey, content.Title, content.Markdown, page.URL)

		if !options.DryRun && service != nil {
			uri := URLToURI(page.URL)
			parentURI := URLToParentURI(page.URL)

			// Just a basic save (the TS codebase did deduplication, etc.)
			// We can just add it via the service.
			_, err := service.CreateContextNode(contextsrv.ContextCreateInput{
				URI:       uri,
				Project:   options.Project,
				ParentURI: parentURI,
				Name:      content.Title,
				Abstract:  summary.Abstract,
				Overview:  summary.Overview,
				Content:   content.Markdown,
			})
			if err != nil {
				result.Errors = append(result.Errors, ScrapeError{URL: page.URL, Error: err.Error()})
			} else {
				result.PagesStored++
			}
		}
	}

	return result, nil
}
