package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

type greetingRequest struct {
	Name string `json:"name"`
}

type greetingResponse struct {
	Message string `json:"message"`
}

type healthResponse struct {
	Status string `json:"status"`
}

func main() {
	mux := newMux()

	log.Println("server listening on http://localhost:8081")

	if err := http.ListenAndServe(":8081", mux); err != nil {
		log.Fatal(err)
	}
}

func newMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /greet", greetingHandler)

	return mux
}

func greetingHandler(w http.ResponseWriter, r *http.Request) {
	var request greetingRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "request body must be valid JSON", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(request.Name)

	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	response := greetingResponse{
		Message: "Hello, " + name + "!",
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("encode greeting response: %v", err)
	}

}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := healthResponse{
		Status: "ok",
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("encode health response: %v", err)
	}
}
