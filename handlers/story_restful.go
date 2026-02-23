package handlers

import (
	"net/http"
	"strings"
)

// StoryDetailRESTHandler handles RESTful paths like /stories/ID
func StoryDetailRESTHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract story ID from path
	path := strings.TrimPrefix(r.URL.Path, "/stories/")
	if path == "" || path == r.URL.Path {
		http.Error(w, "Story ID is required", http.StatusBadRequest)
		return
	}

	// Set the story ID as a query parameter and call the existing handler
	r.URL.RawQuery = "id=" + path
	StoryDetailHandler(w, r)
}
