package main

import (
	"fmt"
	"log"
	"net/http"

	"agent/handlers"
)

func main() {
	http.HandleFunc("/spawn", handlers.SpawnAgentHandler)
	http.HandleFunc("/message", handlers.MessageHandler)
	//http.HandleFunc("/delete", handlers.DeleteAgentHandler)

	fmt.Println("Server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
