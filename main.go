package main

import (
	"bytes"
	"encoding/base64"
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

func main() {
	ngrokURL := "https://progapanda.ngrok.app/" // Replace with your ngrok URL

	http.HandleFunc("/images/", serveImage)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		pageURL := r.URL.Query().Get("url")
		if pageURL == "" {
			fmt.Fprintf(w, "Please provide a URL parameter.")
			return
		}

		resp, err := http.Get(pageURL)
		if err != nil {
			fmt.Fprintf(w, "Error fetching URL: %v", err)
			return
		}
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			fmt.Fprintf(w, "Error parsing HTML: %v", err)
			return
		}

		imageURLs := extractImageURLs(doc, pageURL)
		totalSize := 0
		optimizedTotalSize := 0
		for _, imageURL := range imageURLs {
			size, err := getImageSize(imageURL)
			if err != nil {
				fmt.Fprintf(w, "Error fetching image size: %v\n", err)
				continue
			}
			totalSize += size

			var optimizedSize int
			if strings.HasPrefix(imageURL, "data:") {
				optimizedImageURL := imageURL // Keep the data URI as is for optimization
				optimizedSize, err = getImageSize(optimizedImageURL)
				if err != nil {
					fmt.Fprintf(w, "Error fetching optimized image size: %v\n", err)
					continue
				}
				optimizedTotalSize += optimizedSize
			} else {
				localFilePath, err := downloadImageToLocal(imageURL)
				if err != nil {
					fmt.Fprintf(w, "Error downloading image to local: %v\n", err)
					continue
				}
				defer os.Remove(localFilePath) // Clean up the temporary file
				publicImageURL := fmt.Sprintf("%s/images/%s", ngrokURL, filepath.Base(localFilePath))
				optimizedImageURL := fmt.Sprintf("https://imgproxy.progapanda.org/unsafe/plain/%s@avif", url.PathEscape(publicImageURL))
				optimizedSize, err = getImageSize(optimizedImageURL)
				if err != nil {
					fmt.Fprintf(w, "Error fetching optimized image size: %v\n", err)
					continue
				}
				optimizedTotalSize += optimizedSize
			}

			fmt.Fprintf(w, "Image URL: %s, Original Size: %d bytes, Optimized Size: %d bytes\n", imageURL, size, optimizedSize)
		}

		fmt.Fprintf(w, "\nTotal image size: %d bytes\n", totalSize)
		fmt.Fprintf(w, "Total optimized image size: %d bytes\n", optimizedTotalSize)
		fmt.Fprintf(w, "Size difference: %d bytes\n", totalSize-optimizedTotalSize)

		fmt.Fprintf(w, "\nLoading times for different connection speeds (Original / Optimized):\n")
		printLoadingTimes(w, totalSize, optimizedTotalSize)
	})

	fmt.Println("Server listening on http://localhost:8080")
	http.ListenAndServe("localhost:8080", nil)
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
			imageURLs = append(imageURLs, base.ResolveReference(srcURL).String())
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
				imageURLs = append(imageURLs, base.ResolveReference(bgImageURL).String())
			}
		}
	})

	return imageURLs
}

func getImageSize(imageURL string) (int, error) {
	if strings.HasPrefix(imageURL, "data:") {
		// Find the data in the data URI
		commaIndex := strings.Index(imageURL, ",")
		if commaIndex == -1 {
			return 0, fmt.Errorf("Invalid data URI: %s", imageURL)
		}
		data := imageURL[commaIndex+1:]

		var decodedData []byte
		var err error

		if strings.Contains(imageURL, "base64") {
			// Decode the base64 data
			decodedData, err = base64.StdEncoding.DecodeString(data)
			if err != nil {
				return 0, fmt.Errorf("Error decoding base64 data for %s: %v", imageURL, err)
			}
		} else {
			// Decode the URL-encoded data
			decodedString, err := url.QueryUnescape(data)
			if err != nil {
				return 0, fmt.Errorf("Error decoding URL-encoded data for %s: %v", imageURL, err)
			}
			decodedData = []byte(decodedString)
		}

		return len(decodedData), nil
	}

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

func printLoadingTimes(w http.ResponseWriter, totalSize int, optimizedTotalSize int) {
	connectionSpeeds := map[string]float64{
		"2G":   50,         // 50 Kbps
		"3G":   384,        // 384 Kbps
		"4G":   100 * 1024, // 100 Mbps
		"Wifi": 300 * 1024, // 300 Mbps
	}

	for speed, kbps := range connectionSpeeds {
		loadingTime := float64(totalSize) * 8 / (kbps * 1024 / 8)                   // Calculate loading time in seconds for original images
		optimizedLoadingTime := float64(optimizedTotalSize) * 8 / (kbps * 1024 / 8) // Calculate loading time in seconds for optimized images
		fmt.Fprintf(w, "%s: %.2f seconds / %.2f seconds\n", speed, loadingTime, optimizedLoadingTime)
	}
}

func serveImage(w http.ResponseWriter, r *http.Request) {
	imageKey := r.URL.Path[len("/images/"):]
	imageData, exists := imageMap[imageKey]
	if !exists {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, imageKey, time.Now(), bytes.NewReader(imageData))
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
