package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

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

type loadingTime struct {
	original  float64
	optimized float64
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

func main() {
	ngrokURL := "https://progapanda.ngrok.app/" // Replace with your ngrok URL

	http.HandleFunc("/images/", serveImage)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleURL(w, r, ngrokURL)
	})

	fmt.Println("Server listening on http://localhost:8080")
	err := http.ListenAndServe("localhost:8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

func handleURL(w http.ResponseWriter, r *http.Request, ngrokURL string) {
	startTime := time.Now()

	pageURL := r.URL.Query().Get("url")
	if pageURL == "" {
		fmt.Fprintf(w, "Please provide a URL parameter.")
		return
	}

	imageURLs := fetchAndParsePage(pageURL)

	var results []imageSizeResult

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
		}
	}

	// Output the results
	totalOriginalSize := 0
	totalOptimizedSize := 0
	for _, result := range results {
		fmt.Fprintf(w, "Image URL: %s, Original Size: %d bytes, Optimized Size: %d bytes\n", result.imageURL, result.originalSize, result.optimizedSize)
		totalOriginalSize += result.originalSize
		totalOptimizedSize += result.optimizedSize
	}

	fmt.Fprintf(w, "\nTotal image size: %d bytes\nTotal optimized image size: %d bytes\nSize difference: %d bytes\n", totalOriginalSize, totalOptimizedSize, totalOriginalSize-totalOptimizedSize)

	// Calculate loading times
	loadingTimes := calculateLoadingTimes(totalOriginalSize, totalOptimizedSize)
	fmt.Fprintf(w, "\nLoading times for different connection speeds (Original / Optimized):\n2G: %.2f seconds / %.2f seconds\n3G: %.2f seconds / %.2f seconds\n4G: %.2f seconds / %.2f seconds\nWifi: %.2f seconds / %.2f seconds\n", loadingTimes["2g"].original, loadingTimes["2g"].optimized, loadingTimes["3g"].original, loadingTimes["3g"].optimized, loadingTimes["4g"].original, loadingTimes["4g"].optimized, loadingTimes["wifi"].original, loadingTimes["wifi"].optimized)

	fmt.Fprintf(w, "\nOriginal URL: %s\nProcessing time: %s", pageURL, time.Since(startTime))
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

func serveImage(w http.ResponseWriter, r *http.Request) {
	imageName := filepath.Base(r.URL.Path)
	data, ok := imageMap[imageName]
	if !ok {
		http.NotFound(w, r)
		return
	}

	reader := bytes.NewReader(data)
	http.ServeContent(w, r, imageName, time.Now(), reader)
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
