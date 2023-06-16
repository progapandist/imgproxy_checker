package pkg

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/PuerkitoBio/goquery"
)

type imageSizeResult struct {
	imageURL          string
	originalSize      int
	optimizedSize     int
	originalSizeError error
	optSizeError      error
}

func FetchAndProcessImages(pageURL string) ([]imageSizeResult, int, int) {
	imageURLs := fetchAndParsePage(pageURL)
	fmt.Printf("Fetched %d image URLs\n", len(imageURLs)) // Debug print

	var results []imageSizeResult
	totalOriginalSize := 0
	totalOptimizedSize := 0

	for _, imageURL := range imageURLs {
		originalSize, originalSizeError := getImageSize(imageURL)
		var optimizedSize int
		var optSizeError error

		if originalSizeError == nil {
			optimizedImageURL := fmt.Sprintf("https://imgproxy.progapanda.org/unsafe/plain/%s@avif", url.PathEscape(imageURL))
			optimizedSize, optSizeError = getImageSize(optimizedImageURL)
		}

		result := imageSizeResult{
			imageURL:          imageURL,
			originalSize:      originalSize,
			optimizedSize:     optimizedSize,
			originalSizeError: originalSizeError,
			optSizeError:      optSizeError,
		}

		results = append(results, result)
		totalOriginalSize += result.originalSize
		totalOptimizedSize += result.optimizedSize
	}

	return results, totalOriginalSize, totalOptimizedSize
}

func getImageSize(imageURL string) (int, error) {
	resp, err := http.Get(imageURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("Error reading image content for %s: %v", imageURL, err)
	}

	return len(body), nil
}

func fetchAndParsePage(pageURL string) []string {
	resp, err := http.Get(pageURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil
	}

	return extractImageURLs(doc, pageURL)
}
