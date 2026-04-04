package scraper

import (
	"net/url"
	"strings"

	markdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/go-shiori/go-readability"
)

func ExtractContent(html string, selector string) ExtractedContent {
	// Parse HTML with goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ExtractedContent{}
	}

	title := doc.Find("title").Text()
	title = strings.TrimSpace(title)

	var root *goquery.Selection
	if selector != "" {
		root = doc.Find(selector).First()
	}
	if root == nil || root.Length() == 0 {
		root = doc.Selection
	}

	htmlForReadability, _ := root.Html()
	if root == doc.Selection {
		htmlForReadability = html
	}

	// Readability fallback structure
	parsedURL, _ := url.Parse("http://localhost") // readability requires a URL sometimes, but it's just for base
	article, err := readability.FromReader(strings.NewReader(htmlForReadability), parsedURL)

	var contentHtml string
	if err == nil && article.Content != "" {
		contentHtml = article.Content
		if article.Title != "" {
			title = article.Title
		}
	} else {
		// Fallback
		if root == doc.Selection {
			body := doc.Find("body")
			if body.Length() > 0 {
				contentHtml, _ = body.Html()
			}
		} else {
			contentHtml, _ = root.Html()
		}
	}

	originalH1 := strings.TrimSpace(doc.Find("h1").First().Text())

	md := htmlToMarkdown(contentHtml)
	wordCount := len(strings.Fields(md))

	sections := splitSections(contentHtml, originalH1)

	return ExtractedContent{
		Title:     title,
		Markdown:  md,
		Sections:  sections,
		WordCount: wordCount,
	}
}

func htmlToMarkdown(html string) string {
	md, _ := markdown.ConvertString(html)
	return md
}

func splitSections(html string, skipHeading string) []Section {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<div>" + html + "</div>"))
	if err != nil {
		return nil
	}

	var sections []Section
	var currentHeading *Section
	var currentContent []string

	flush := func() {
		if currentHeading != nil && len(currentContent) > 0 {
			combined := strings.Join(currentContent, "\n")
			currentHeading.Content = htmlToMarkdown(combined)
			sections = append(sections, *currentHeading)
		}
		currentContent = []string{}
	}

	doc.Find("div").First().Children().Each(func(i int, s *goquery.Selection) {
		tag := strings.ToLower(goquery.NodeName(s))

		if tag == "h2" || tag == "h3" {
			flush()
			headingText := strings.TrimSpace(s.Text())
			if skipHeading != "" && headingText == skipHeading {
				currentHeading = nil
				return
			}
			level := 2
			if tag == "h3" {
				level = 3
			}
			currentHeading = &Section{
				Heading: headingText,
				Level:   level,
			}
		} else if currentHeading != nil {
			h, _ := goquery.OuterHtml(s)
			currentContent = append(currentContent, h)
		}
	})
	flush()

	return sections
}
