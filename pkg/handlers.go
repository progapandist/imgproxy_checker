package pkg

import (
	"fmt"
	"net/http"
	"time"
)

func HandleURL(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	pageURL := r.URL.Query().Get("url")
	if pageURL == "" {
		http.Error(w, "Please provide a URL parameter.", http.StatusBadRequest)
		return
	}

	results, totalOriginalSize, totalOptimizedSize, numImages, scrollDuration := FetchAndProcessImages(pageURL)

	// Count occurrences of each image
	imageCount := make(map[string]int)
	for _, result := range results {
		imageCount[result.imageURL]++
	}

	// Output the results
	for imageURL, count := range imageCount {
		result := results[0]
		for _, r := range results {
			if r.imageURL == imageURL {
				result = r
				break
			}
		}
		_, _ = fmt.Fprintf(w, "Image URL: %s (%d times), Original Size: %d bytes, Optimized Size: %d bytes\n", imageURL, count, result.originalSize, result.optimizedSize)
	}

	_, _ = fmt.Fprintf(w, "\nTotal image size: %.2f KB\nTotal optimized image size: %.2f KB\nSize difference: %.2f KB\n", float64(totalOriginalSize)/1024, float64(totalOptimizedSize)/1024, float64(totalOriginalSize-totalOptimizedSize)/1024)
	// Calculate loading times
	loadingTimes := calculateLoadingTimes(totalOriginalSize, totalOptimizedSize)
	_, _ = fmt.Fprintf(w, "\nLoading times for different connection speeds (Original / Optimized):\n2G: %.2f seconds / %.2f seconds\n3G: %.2f seconds / %.2f seconds\n4G: %.2f seconds / %.2f seconds\nWifi: %.2f seconds / %.2f seconds\n", loadingTimes["2g"].original, loadingTimes["2g"].optimized, loadingTimes["3g"].original, loadingTimes["3g"].optimized, loadingTimes["4g"].original, loadingTimes["4g"].optimized, loadingTimes["wifi"].original, loadingTimes["wifi"].optimized)

	_, _ = fmt.Fprintf(w, "\nNumber of images processed: %d\nScroll duration: %s\n", numImages, scrollDuration)

	_, _ = fmt.Fprintf(w, "\nOriginal URL: %s\nProcessing time: %s", pageURL, time.Since(startTime))
}
