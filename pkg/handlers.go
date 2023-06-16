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

	// Output the results
	for _, result := range results {
		_, _ = fmt.Fprintf(w, "Image URL: %s, Original Size: %d bytes, Optimized Size: %d bytes\n", result.imageURL, result.originalSize, result.optimizedSize)
	}

	_, _ = fmt.Fprintf(w, "\nTotal image size: %d bytes\nTotal optimized image size: %d bytes\nSize difference: %d bytes\n", totalOriginalSize, totalOptimizedSize, totalOriginalSize-totalOptimizedSize)

	// Calculate loading times
	loadingTimes := calculateLoadingTimes(totalOriginalSize, totalOptimizedSize)
	_, _ = fmt.Fprintf(w, "\nLoading times for different connection speeds (Original / Optimized):\n2G: %.2f seconds / %.2f seconds\n3G: %.2f seconds / %.2f seconds\n4G: %.2f seconds / %.2f seconds\nWifi: %.2f seconds / %.2f seconds\n", loadingTimes["2g"].original, loadingTimes["2g"].optimized, loadingTimes["3g"].original, loadingTimes["3g"].optimized, loadingTimes["4g"].original, loadingTimes["4g"].optimized, loadingTimes["wifi"].original, loadingTimes["wifi"].optimized)

	_, _ = fmt.Fprintf(w, "\nNumber of images processed: %d\nScroll duration: %s\n", numImages, scrollDuration)

	_, _ = fmt.Fprintf(w, "\nOriginal URL: %s\nProcessing time: %s", pageURL, time.Since(startTime))
}
