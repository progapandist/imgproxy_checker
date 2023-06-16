package pkg

import (
	"bytes"
	"fmt"
	"net/http"
	"path/filepath"
	"time"
)

func HandleURL(w http.ResponseWriter, r *http.Request, ngrokURL string) {
	startTime := time.Now()

	pageURL := r.URL.Query().Get("url")
	if pageURL == "" {
		http.Error(w, "Please provide a URL parameter.", http.StatusBadRequest)
		return
	}

	results, totalOriginalSize, totalOptimizedSize := FetchAndProcessImages(pageURL, ngrokURL)

	// Output the results
	for _, result := range results {
		_, _ = fmt.Fprintf(w, "Image URL: %s, Original Size: %.2f MB, Optimized Size: %.2f MB\n", result.imageURL, float64(result.originalSize)/1024/1024, float64(result.optimizedSize)/1024/1024)
	}

	_, _ = fmt.Fprintf(w, "\nTotal image size: %.2f MB\nTotal optimized image size: %.2f MB\nSize difference: %.2f MB\n", float64(totalOriginalSize)/1024/1024, float64(totalOptimizedSize)/1024/1024, float64(totalOriginalSize-totalOptimizedSize)/1024/1024)

	// Calculate loading times
	loadingTimes := calculateLoadingTimes(totalOriginalSize, totalOptimizedSize)
	_, _ = fmt.Fprintf(w, "\nLoading times for different connection speeds (Original / Optimized):\n2G: %.2f seconds / %.2f seconds\n3G: %.2f seconds / %.2f seconds\n4G: %.2f seconds / %.2f seconds\nWifi: %.2f seconds / %.2f seconds\n", loadingTimes["2g"].original, loadingTimes["2g"].optimized, loadingTimes["3g"].original, loadingTimes["3g"].optimized, loadingTimes["4g"].original, loadingTimes["4g"].optimized, loadingTimes["wifi"].original, loadingTimes["wifi"].optimized)

	_, _ = fmt.Fprintf(w, "\nOriginal URL: %s\nProcessing time: %s", pageURL, time.Since(startTime))
}

func ServeImage(w http.ResponseWriter, r *http.Request) {
	imageName := filepath.Base(r.URL.Path)
	data, ok := imageMap[imageName]
	if !ok {
		http.NotFound(w, r)
		return
	}

	reader := bytes.NewReader(data)
	http.ServeContent(w, r, imageName, time.Now(), reader)
}
