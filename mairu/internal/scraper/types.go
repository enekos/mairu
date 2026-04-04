package scraper

type CrawlOptions struct {
	SeedURL     string
	MaxDepth    int
	MaxPages    int
	Concurrency int
	DelayMs     int
	URLPattern  string
	WaitUntil   string // not strictly used if purely HTTP, but mapped
	Selector    string // CSS selector
}

type CrawledPage struct {
	URL   string
	HTML  string
	Title string
	Links []string
	Depth int
}

type Section struct {
	Heading string
	Content string
	Level   int
}

type ExtractedContent struct {
	Title     string
	Markdown  string
	Sections  []Section
	WordCount int
}

type PageSummary struct {
	Abstract       string   `json:"abstract"`
	Overview       string   `json:"overview"`
	AIIntent       *string  `json:"ai_intent"`
	AITopics       []string `json:"ai_topics"`
	AIQualityScore int      `json:"ai_quality_score"`
}

type ScrapeOptions struct {
	CrawlOptions
	Project       string
	SplitSections bool
	DryRun        bool
	UseRouter     bool
}

type ScrapeResult struct {
	PagesTotal     int
	PagesStored    int
	PagesUpdated   int
	PagesSkipped   int
	SectionsStored int
	Errors         []ScrapeError
}

type ScrapeError struct {
	URL   string
	Error string
}

type CacheEntry struct {
	ContentHash string `json:"contentHash"`
	ScrapedAt   string `json:"scrapedAt"`
	URI         string `json:"uri"`
}
