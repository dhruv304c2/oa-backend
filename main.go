package main

import (
	"fmt"
	"log"
	"net/http"

	"agent/db"
	"agent/handlers"
	"agent/middleware"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Initialize MongoDB connection

	err = db.InitMongoDB()
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}
	defer db.Close()

	// Create database indexes
	db.CreateAgentIndexes()

	// Set up HTTP handlers with CORS
	http.HandleFunc("/spawn", middleware.EnableCORS(handlers.SpawnAgentHandler))
	http.HandleFunc("/message", middleware.EnableCORS(handlers.MessageHandler))
	http.HandleFunc("/agent/history", middleware.EnableCORS(handlers.HistoryHandler))
	http.HandleFunc("/score", middleware.EnableCORS(handlers.ScoreTheoryHandler))
	http.HandleFunc("/feed", middleware.EnableCORS(handlers.FeedHandler))
	http.HandleFunc("/story", middleware.EnableCORS(handlers.StoryDetailHandler))
	http.HandleFunc("/stories/", middleware.EnableCORS(handlers.StoryDetailRESTHandler)) // RESTful route
	//http.HandleFunc("/delete", middleware.EnableCORS(handlers.DeleteAgentHandler))

	fmt.Println("Server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
