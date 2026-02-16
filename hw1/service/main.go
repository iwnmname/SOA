package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
		if err != nil {
			return
		}
	})

	log.Println("Order Service starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
