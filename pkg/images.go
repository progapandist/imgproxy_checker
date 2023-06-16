package pkg

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var imageMap = make(map[string][]byte)
var processedImages = make(map[string]bool)

type imageSizeResult struct {
	imageURL          string
	originalSize      int
	optimizedSize     int
	originalSizeError error
	optSizeError      error
}

func FetchAndProcessImages(pageURL string, ngrokURL string) ([]imageSizeResult, int, int) {
	imageURLs := fetchAndParsePage(pageURL)

	var results []imageSizeResult
	totalOriginalSize := 0
	totalOptimizedSize := 0

	for _, imageURL := range imageURLs {
		if _, exists := processedImages[imageURL]; !exists {
			processedImages[imageURL] = true

			originalSize, originalSizeError := getImageSize(imageURL)
			var optimizedSize int
			var optSizeError error

			if originalSizeError == nil {
				if strings.HasPrefix(imageURL, "data:") {
					optimizedImageURL := imageURL
					optimizedSize, optSizeError = getImageSize(optimizedImageURL)
				} else {
					localFilePath, err := downloadImageToLocal(imageURL)
					if err == nil {
						publicImageURL := fmt.Sprintf("%s/images/%s", ngrokURL, filepath.Base(localFilePath))
						optimizedImageURL := fmt.Sprintf("https://imgproxy.progapanda.org/unsafe/plain/%s@avif", url.PathEscape(publicImageURL))
						optimizedSize, optSizeError = getImageSize(optimizedImageURL)
						os.Remove(localFilePath) // Remove the temporary file after getting the optimized image size
					}
				}
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

func downloadImageToLocal(imageURL string) (string, error) {
	resp, err := http.Get(imageURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Error reading image content for %s: %v", imageURL, err)
	}

	tmpFile, err := ioutil.TempFile("", "image-*.tmp")
	if err != nil {
		return "", err
	}

	_, err = tmpFile.Write(content)
	if err != nil {
		tmpFile.Close()
		return "", err
	}

	tmpFile.Close()

	imageMap[filepath.Base(tmpFile.Name())] = content

	return filepath.Abs(tmpFile.Name())
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
