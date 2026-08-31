package main

import (
	"log"
	"net/http"
	"os"

	"github.com/SaadShakeel1/TaskFlow/internal/api"
	"github.com/SaadShakeel1/TaskFlow/internal/database"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL must be set")
	}

	db, err := database.Open(databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	handler := api.NewRouter(db)

	log.Println("server listening on http://localhost:8081")

	if err := http.ListenAndServe(":8081", handler); err != nil {
		log.Fatal(err)
	}
}
