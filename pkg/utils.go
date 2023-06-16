package pkg

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type loadingTime struct {
	original  float64
	optimized float64
}

func extractImageURLs(doc *goquery.Document, baseURL string) []string {
	var imageURLs []string

	base, err := url.Parse(baseURL)
	if err != nil {
		return imageURLs
	}

	// Extract URLs from img tags
	doc.Find("img").Each(func(_ int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if exists {
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

	// Extract URLs from background-image in style attributes
	doc.Find("*[style]").Each(func(_ int, s *goquery.Selection) {
		style, _ := s.Attr("style")
		if strings.Contains(style, "background-image") {
			re := regexp.MustCompile(`url\(['"]?(.*?)['"]?\)`)
			matches := re.FindStringSubmatch(style)
			if len(matches) > 1 {
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
