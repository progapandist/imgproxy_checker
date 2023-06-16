package pkg

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/cascadia"
)

type loadingTime struct {
	original  float64
	optimized float64
}

// Additional selector for extracting image URLs from styles
var backgroundImageSelector = cascadia.MustCompile(`[style*="background-image"]`)

func isValidImageURL(src string) bool {
	parsedURL, err := url.Parse(src)
	if err != nil {
		return false
	}

	// Check if the URL starts with "http" or "https"
	if parsedURL.Scheme == "" {
		return false
	}

	// Check if the URL has a valid image extension excluding SVGs
	imagePattern := regexp.MustCompile(`\.(jpg|jpeg|png|gif|webp|bmp|tiff|avif)$`)
	return !strings.HasPrefix(src, "data:") && !strings.HasSuffix(parsedURL.Path, ".svg") && imagePattern.MatchString(parsedURL.Path)
}

func extractImageURLs(doc *goquery.Document, baseURL string) []string {
	var imageURLs []string

	base, err := url.Parse(baseURL)
	if err != nil {
		return imageURLs
	}

	doc.Find("img").Each(func(_ int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if exists && isValidImageURL(src) {
			srcURL, err := url.Parse(src)
			if err != nil {
				return
			}
			resolvedURL := base.ResolveReference(srcURL).String()
			if resolvedURL != "" {
				imageURLs = append(imageURLs, resolvedURL)
			}
		}
	})

	doc.FindMatcher(backgroundImageSelector).Each(func(_ int, s *goquery.Selection) {
		style, _ := s.Attr("style")
		if strings.Contains(style, "background-image") {
			re := regexp.MustCompile(`url\(['"]?(.*?)['"]?\)`)
			matches := re.FindStringSubmatch(style)
			if len(matches) > 1 && isValidImageURL(matches[1]) {
				bgImageURL, err := url.Parse(matches[1])
				if err != nil {
					return
				}
				resolvedURL := base.ResolveReference(bgImageURL).String()
				if resolvedURL != "" {
					imageURLs = append(imageURLs, resolvedURL)
				}
			}
		}
	})

	doc.Find("style").Each(func(_ int, s *goquery.Selection) {
		styleContent := s.Text()
		extractImageURLsFromStyle(styleContent, imageURLs, base)
	})

	doc.Find("link[rel='stylesheet']").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			stylesheetURL, err := url.Parse(href)
			if err != nil {
				return
			}
			resolvedURL := base.ResolveReference(stylesheetURL).String()
			resp, err := http.Get(resolvedURL)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			styleContent, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return
			}
			extractImageURLsFromStyle(string(styleContent), imageURLs, base)
		}
	})

	doc.Find("script[src]").Each(func(_ int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if exists {
			jsURL, err := url.Parse(src)
			if err != nil {
				return
			}
			resolvedURL := base.ResolveReference(jsURL).String()
			resp, err := http.Get(resolvedURL)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			jsContent, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return
			}
			imageURLsFromJS := extractImageURLsFromJS(string(jsContent), base)
			imageURLs = append(imageURLs, imageURLsFromJS...)
		}
	})

	return imageURLs
}

func extractImageURLsFromStyle(styleContent string, imageURLs []string, base *url.URL) {
	re := regexp.MustCompile(`url\(['"]?(.*?)['"]?\)`)
	matches := re.FindAllStringSubmatch(styleContent, -1)
	for _, match := range matches {
		if len(match) > 1 && isValidImageURL(match[1]) {
			bgImageURL, err := url.Parse(match[1])
			if err != nil {
				continue
			}
			resolvedURL := base.ResolveReference(bgImageURL).String()
			if resolvedURL != "" {
				imageURLs = append(imageURLs, resolvedURL)
			}
		}
	}
}

func extractImageURLsFromJS(jsContent string, base *url.URL) []string {
	var imageURLs []string

	// Update the regex pattern to match image URLs more specifically
	re := regexp.MustCompile(`(?i)['"]((?:https?:)?\/\/[^'"]+\.(?:jpg|jpeg|png|gif|webp|bmp|tiff|avif)(?:\?[^'"]*)?)['"]`)
	matches := re.FindAllStringSubmatch(jsContent, -1)

	for _, match := range matches {
		if len(match) > 1 {
			imageURL := match[1]
			if isValidImageURL(imageURL) {
				imageURLParsed, err := url.Parse(imageURL)
				if err != nil {
					continue
				}
				resolvedURL := base.ResolveReference(imageURLParsed).String()
				imageURLs = append(imageURLs, resolvedURL)
			}
		}
	}
	return imageURLs
}

func calculateLoadingTimes(originalSize int, optimizedSize int) map[string]loadingTime {
	speeds := map[string]float64{
		"2g":   35.0,   // 35 kbps
		"3g":   200.0,  // 200 kbps
		"4g":   1000.0, // 1000 kbps
		"wifi": 5000.0, // 5000 kbps
	}

	loadingTimes := make(map[string]loadingTime)

	for speed, kbps := range speeds {
		originalTime := (float64(originalSize) * 8) / (kbps * 1000)
		optimizedTime := (float64(optimizedSize) * 8) / (kbps * 1000)
		loadingTimes[speed] = loadingTime{
			original:  originalTime,
			optimized: optimizedTime,
		}
	}

	return loadingTimes
}
