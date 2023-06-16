package main

import (
	"fmt"
	"net/http"

	"github.com/progapandist/imgproxy_checker/pkg"
)

func main() {

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		pkg.HandleURL(w, r)
	})

	fmt.Println("Server listening on http://localhost:8080")
	err := http.ListenAndServe("localhost:8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
