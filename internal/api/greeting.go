package api

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

func (s Server) greetingHandler(w http.ResponseWriter, r *http.Request) {
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

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(greetingResponse{Message: "Hello, " + name + "!"}); err != nil {
		log.Printf("encode greeting response: %v", err)
	}
}
