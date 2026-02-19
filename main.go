package main

import (
	"fmt"
	"log"
	"net/http"

	"agent/db"
	"agent/handlers"
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

	// Set up HTTP handlers
	http.HandleFunc("/spawn", handlers.SpawnAgentHandler)
	http.HandleFunc("/message", handlers.MessageHandler)
	//http.HandleFunc("/delete", handlers.DeleteAgentHandler)

	fmt.Println("Server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
