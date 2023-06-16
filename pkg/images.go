package pkg

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

type imageSizeResult struct {
	imageURL      string
	originalSize  int
	optimizedSize int
	timestamp     int64
}

type loadingTime struct {
	original  float64
	optimized float64
}

var backgroundImageSelector = `[style*="background-image"]`

func resolveURL(rawURL, baseURL string) string {
	fmt.Printf("rawURL: %s, baseURL: %s\n", rawURL, baseURL)
	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		fmt.Printf("Error parsing baseURL: %v\n", err)
		return rawURL
	}

	parsedRawURL, err := url.Parse(rawURL)
	if err != nil {
		fmt.Printf("Error parsing rawURL: %v\n", err)
		return rawURL
	}

	resolvedURL := parsedBaseURL.ResolveReference(parsedRawURL)
	return resolvedURL.String()
}

func isValidImageURL(src string) bool {
	parsedURL, err := url.Parse(src)
	if err != nil {
		return false
	}

	if parsedURL.Scheme == "" {
		return false
	}

	imagePattern := regexp.MustCompile(`\.(jpg|jpeg|png|gif|webp|bmp|tiff|avif)$`)
	return !strings.HasPrefix(src, "data:") && !strings.HasSuffix(parsedURL.Path, ".svg") && imagePattern.MatchString(parsedURL.Path)
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

func extractImageURLsFromRodPage(page *rod.Page, pageURL string) []string {
	imageURLs := []string{}
	elements, err := page.Elements("img")
	if err != nil {
		return imageURLs
	}

	for _, element := range elements {
		rawURL, err := element.Attribute("src")
		if err != nil {
			continue
		}
		if rawURL == nil || *rawURL == "" {
			continue
		}
		resolvedURL := resolveURL(*rawURL, pageURL)
		imageURLs = append(imageURLs, resolvedURL)
	}

	bgElements, err := page.Elements(backgroundImageSelector)
	if err != nil {
		return imageURLs
	}
	for _, element := range bgElements {
		style, err := element.Attribute("style")
		if err == nil && style != nil {
			imageURLs = append(imageURLs, extractImageURLsFromStyle(*style, pageURL)...)
		}
	}

	styleElements, err := page.Elements("style")
	if err != nil {
		return imageURLs
	}
	for _, element := range styleElements {
		styleContent, err := element.Text()
		if err == nil {
			imageURLs = append(imageURLs, extractImageURLsFromStyle(styleContent, pageURL)...)
		}
	}

	return imageURLs
}

func extractImageURLsFromStyle(styleContent string, pageURL string) []string {
	var imageURLs []string

	re := regexp.MustCompile(`url\(['"]?(.*?)['"]?\)`)
	matches := re.FindAllStringSubmatch(styleContent, -1)
	for _, match := range matches {
		if len(match) > 1 && isValidImageURL(match[1]) {
			bgImageURL, err := url.Parse(match[1])
			if err != nil {
				continue
			}
			resolvedURL := resolveURL(pageURL, bgImageURL.String())
			if resolvedURL != "" {
				imageURLs = append(imageURLs, resolvedURL)
			}
		}
	}

	return imageURLs
}

func fetchAndParsePage(pageURL string) ([]string, time.Duration) {
	imageURLs := []string{}

	launcher := launcher.New().
		Set("disable-web-security", "true").
		Headless(true)
	browser := rod.New().ControlURL(launcher.MustLaunch()).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage(pageURL)
	page.MustWaitLoad()

	customScrollAndWait := `
        () => {
            return new Promise(async (resolve, reject) => {
                const observer = new IntersectionObserver((entries) => {
                    entries.forEach((entry) => {
                        if (entry.isIntersecting) {
                            entry.target.dispatchEvent(new Event('scrollIntoView'));
                        }
                    });
                }, { threshold: 1 });

                const elements = document.querySelectorAll('img');
                elements.forEach((element) => observer.observe(element));

                const timeout = setTimeout(() => {
                    observer.disconnect();
                    reject(new Error('Scroll timeout'));
                }, 3000);

                for (let i = 0; i < elements.length; i++) {
                    elements[i].scrollIntoView();
                    await new Promise((r) => setTimeout(r, 100));
                }

                clearTimeout(timeout);
                observer.disconnect();
                resolve();
            });
        }
    `

	_, err := page.Eval(customScrollAndWait)
	if err != nil {
		fmt.Println("Scroll timeout, closing the page.")
	}

	imageURLs = append(imageURLs, extractImageURLsFromRodPage(page, pageURL)...)
	page.MustClose()

	// Set the limit to how long we simalate scrolling for
	scrollDuration := 1 * time.Second
	return imageURLs, scrollDuration
}

func FetchAndProcessImages(pageURL string) ([]imageSizeResult, int, int, int, time.Duration) {
	imageURLs, scrollDuration := fetchAndParsePage(pageURL)

	numImages := len(imageURLs)
	fmt.Printf("Fetched %d image URLs\n", numImages)

	var results []imageSizeResult
	totalOriginalSize := 0
	totalOptimizedSize := 0

	db, err := initDB()
	if err != nil {
		fmt.Printf("Error initializing database: %v\n", err)
		return nil, 0, 0, 0, 0
	}
	defer db.Close()

	for _, imageURL := range imageURLs {
		if !isValidImageURL(imageURL) {
			fmt.Printf("Skipping invalid image URL: %s\n", imageURL)
			continue
		}
		existingData, err := getImageDataByURL(db, pageURL, imageURL)
		if err != nil {
			fmt.Printf("Error querying image data from the database: %v\n", err)
			continue
		}

		var result imageSizeResult
		now := time.Now().Unix()
		if existingData == nil || now-existingData.timestamp > 24*60*60 {
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
				timestamp:     now,
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

		fmt.Printf("Processed image %s: Original Size: %d bytes, Optimized Size: %d bytes\n", imageURL, result.originalSize, result.optimizedSize)
	}

	return results, totalOriginalSize, totalOptimizedSize, numImages, scrollDuration
}

func calculateLoadingTimes(originalSize int, optimizedSize int) map[string]loadingTime {
	speeds := map[string]float64{
		"2g":   35.0,
		"3g":   200.0,
		"4g":   1000.0,
		"wifi": 5000.0,
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
