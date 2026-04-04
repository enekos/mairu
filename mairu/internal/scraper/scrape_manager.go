package scraper

import (
	"context"

	"mairu/internal/contextsrv"
)

type NodeStoreFunc func(ctx context.Context, input contextsrv.ContextCreateInput) error

func ScrapeAndIngest(ctx context.Context, options ScrapeOptions, storeFn NodeStoreFunc, geminiAPIKey string) (*ScrapeResult, error) {
	out := make(chan CrawledPage, 10)
	go Crawl(options.CrawlOptions, out)

	result := &ScrapeResult{}
	for page := range out {
		result.PagesTotal++
		content := ExtractContent(page.HTML, options.Selector, page.URL)
		if content.Markdown == "" {
			result.PagesSkipped++
			continue
		}

		summary := SummarizePage(ctx, geminiAPIKey, content.Title, content.Markdown, page.URL)

		if !options.DryRun && storeFn != nil {
			uri := URLToURI(page.URL)
			parentURI := URLToParentURI(page.URL)

			err := storeFn(ctx, contextsrv.ContextCreateInput{
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
