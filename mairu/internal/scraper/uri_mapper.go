package scraper

import (
	"net/url"
	"regexp"
	"strings"
)

const URIPrefix = "contextfs://scraped/"

func domainSlug(host string) string {
	host = strings.ToLower(host)
	host = strings.TrimPrefix(host, "www.")
	host = strings.ReplaceAll(host, ".", "-")
	host = strings.ReplaceAll(host, ":", "-")
	return host
}

func NormalizeURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	parsed.Fragment = ""
	res := parsed.Scheme + "://" + parsed.Host + parsed.Path
	if strings.HasSuffix(res, "/") && res != parsed.Scheme+"://"+parsed.Host+"/" {
		res = res[:len(res)-1]
	}
	return strings.TrimSuffix(res, "/")
}

var nonAlnumRegex = regexp.MustCompile(`[^a-z0-9]+`)

func URLToURI(rawURL string, sectionHeading ...string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return URIPrefix + "invalid"
	}
	slug := domainSlug(parsed.Host)

	parts := strings.Split(parsed.Path, "/")
	var validParts []string
	for _, p := range parts {
		if p != "" {
			validParts = append(validParts, url.QueryEscape(strings.ToLower(p)))
		}
	}

	base := URIPrefix + slug
	if len(validParts) > 0 {
		base += "/" + strings.Join(validParts, "/")
	}

	if len(sectionHeading) > 0 && sectionHeading[0] != "" {
		headingSlug := strings.ToLower(sectionHeading[0])
		headingSlug = nonAlnumRegex.ReplaceAllString(headingSlug, "-")
		headingSlug = strings.Trim(headingSlug, "-")
		return base + "/" + headingSlug
	}

	return base
}

func URLToParentURI(rawURL string) *string {
	uri := URLToURI(rawURL)
	parts := strings.Split(uri, "/")
	if len(parts) <= 4 {
		return nil
	}
	res := strings.Join(parts[:len(parts)-1], "/")
	return &res
}
