package pkg

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/PuerkitoBio/goquery"
)

type imageSizeResult struct {
	imageURL      string
	originalSize  int
	optimizedSize int
}

func FetchAndProcessImages(pageURL string) ([]imageSizeResult, int, int) {
	imageURLs := fetchAndParsePage(pageURL)
	fmt.Printf("Fetched %d image URLs\n", len(imageURLs)) // Debug print

	var results []imageSizeResult
	totalOriginalSize := 0
	totalOptimizedSize := 0

	db, err := initDB()
	if err != nil {
		fmt.Printf("Error initializing database: %v\n", err)
		return nil, 0, 0
	}
	defer db.Close()

	for _, imageURL := range imageURLs {
		// Check if image data exists in the database
		existingData, err := getImageDataByURL(db, pageURL, imageURL)
		if err != nil {
			fmt.Printf("Error querying image data from the database: %v\n", err)
			continue
		}

		var result imageSizeResult
		if existingData == nil {
			originalSize, originalSizeError := getImageSize(imageURL)
			var optimizedSize int
			var optSizeError error

			if originalSizeError == nil {
				optimizedImageURL := fmt.Sprintf("https://imgproxy.progapanda.org/unsafe/plain/%s@avif", url.PathEscape(imageURL))
				optimizedSize, optSizeError = getImageSize(optimizedImageURL)
			}

			if originalSizeError != nil || optSizeError != nil {
				fmt.Printf("Error processing image %s: originalSizeError=%v, optSizeError=%v\n", imageURL, originalSizeError, optSizeError)
				continue
			}

			result = imageSizeResult{
				imageURL:      imageURL,
				originalSize:  originalSize,
				optimizedSize: optimizedSize,
			}

			_, err := insertImageData(db, pageURL, result)
			if err != nil {
				fmt.Printf("Error inserting image data into the database: %v\n", err)
			}
		} else {
			result = *existingData
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
