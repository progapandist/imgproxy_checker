package pkg

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

type imageSizeResult struct {
	imageURL      string
	originalSize  int
	optimizedSize int
	timestamp     int64 // Add a timestamp field
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
		// Check if URL is valid
		if !isValidImageURL(imageURL) {
			continue
		}
		// Check if image data exists in the database
		existingData, err := getImageDataByURL(db, pageURL, imageURL)
		if err != nil {
			fmt.Printf("Error querying image data from the database: %v\n", err)
			continue
		}

		var result imageSizeResult
		now := time.Now().Unix()
		if existingData == nil || now-existingData.timestamp > 24*60*60 { // Refetch if more than 24 hours have passed
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
				timestamp:     now, // Add the current timestamp
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
	resp, err := http.Head(imageURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to get image: %s", resp.Status)
	}

	contentLength := resp.Header.Get("Content-Length")
	if contentLength != "" {
		size, err := strconv.ParseInt(contentLength, 10, 64)
		if err == nil {
			return int(size), nil
		}
	}

	// Fallback to GET request if Content-Length header is missing or invalid
	resp, err = http.Get(imageURL)
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
	urls := []string{}

	browser := rod.New().ControlURL(launcher.New().MustLaunch()).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage(pageURL)
	defer page.MustClose()

	page.MustWaitLoad()
	imgElements := page.MustElements("img")

	for _, img := range imgElements {
		src, _ := img.Attribute("src")
		if src != nil && *src != "" {
			urls = append(urls, *src)
		}
	}

	// Fetch background images from inline styles and external stylesheets
	bgImagesJSON := page.MustEval(`() => {
		try {
			return Array.from(document.querySelectorAll("*")).map(el => {
				const style = getComputedStyle(el);
				const bgImage = style.backgroundImage;
				const match = bgImage.match(/url\\(['"]?(.*?)['"]?\\)/);
				return match ? match[1] : null;
			}).filter(url => url !== null);
		} catch (error) {
			console.error("Error fetching background images:", error);
			return [];
		}
	}`)

	// Fetch images from JavaScript
	jsCode := `
	() => {
		try {
			return Array.from(document.querySelectorAll("script[src]")).flatMap(script => {
				const regex = /['"]((?:https?:)?\/\/[^'"]+\\.(?:jpg|jpeg|png|gif|webp|bmp|tiff|avif)(?:\\?[^'"]*)?)['"]/ig;
				return Array.from(script.textContent.matchAll(regex)).map(match => match[1]);
			}).filter(url => url);
		} catch (error) {
			console.error("Error fetching JS images:", error);
			return [];
		}
	}`
	re := regexp.MustCompile(`\\\\`)
	jsCode = re.ReplaceAllString(jsCode, "\\")

	jsImagesJSON := page.MustEval(jsCode)

	var bgImages []interface{}
	err := bgImagesJSON.Unmarshal(&bgImages)
	if err == nil {
		for _, bgImage := range bgImages {
			urls = append(urls, bgImage.(string))
		}
	}

	var jsImages []interface{}
	err = jsImagesJSON.Unmarshal(&jsImages)
	if err == nil {
		for _, jsImage := range jsImages {
			urls = append(urls, jsImage.(string))
		}
	}

	return urls
}
