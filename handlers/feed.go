package handlers

import (
	"agent/db"
	"agent/models"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StoryFeedItem struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	CoverImageURL string    `json:"cover_image_url,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type FeedResponse struct {
	Stories []StoryFeedItem `json:"stories"`
	Count   int             `json:"count"`
}

func FeedHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Fetch all stories from MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := db.GetCollection("stories")
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		http.Error(w, "Failed to fetch stories", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var stories []models.Story
	if err = cursor.All(ctx, &stories); err != nil {
		http.Error(w, "Failed to decode stories", http.StatusInternalServerError)
		return
	}

	// Transform to feed format
	feedItems := make([]StoryFeedItem, 0, len(stories))
	for _, story := range stories {
		feedItem := StoryFeedItem{
			ID:            story.ID.Hex(),
			Title:         story.Story.Title,
			Description:   story.Story.NewsArticle.Content,
			CoverImageURL: story.Story.CoverImageURL,
			CreatedAt:     story.CreatedAt,
			UpdatedAt:     story.UpdatedAt,
		}
		feedItems = append(feedItems, feedItem)
	}

	// Return response
	response := FeedResponse{
		Stories: feedItems,
		Count:   len(feedItems),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

type StoryDetailResponse struct {
	ID            string             `json:"id"`
	Title         string             `json:"title"`
	NewsArticle   models.NewsArticle `json:"news_article"`
	CoverImageURL string             `json:"cover_image_url,omitempty"`
	Characters    []CharacterSummary `json:"characters"`
	Locations     []LocationSummary  `json:"locations"`
	CreatedAt     time.Time          `json:"created_at"`
}

type CharacterSummary struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	ImageURL         string            `json:"image_url,omitempty"`
	HoldsEvidence    []models.Evidence `json:"holds_evidence"`
	KnowsLocationIDs []string          `json:"knows_location_ids"`
}

type LocationSummary struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	Description            string   `json:"description"`
	ImageURL               string   `json:"image_url,omitempty"`
	CharacterIDsInLocation []string `json:"character_ids_in_location"`
}

func StoryDetailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get story ID from URL parameter
	storyID := r.URL.Query().Get("id")
	if storyID == "" {
		http.Error(w, "Story ID is required", http.StatusBadRequest)
		return
	}

	// Convert story ID string to ObjectID
	storyObjID, err := primitive.ObjectIDFromHex(storyID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid story ID"})
		return
	}

	// Fetch story from MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var story models.Story
	collection := db.GetCollection("stories")
	err = collection.FindOne(ctx, bson.M{"_id": storyObjID}).Decode(&story)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Story not found"})
		return
	}

	// Transform characters to summary
	characters := make([]CharacterSummary, 0, len(story.Story.Characters))
	for _, char := range story.Story.Characters {
		characters = append(characters, CharacterSummary{
			ID:               char.ID,
			Name:             char.Name,
			Description:      char.AppearanceDescription,
			ImageURL:         char.ImageURL,
			HoldsEvidence:    char.HoldsEvidence,
			KnowsLocationIDs: char.KnowsLocationIDs,
		})
	}

	// Transform locations to summary
	locations := make([]LocationSummary, 0, len(story.Story.Locations))
	for _, loc := range story.Story.Locations {
		locations = append(locations, LocationSummary{
			ID:                     loc.ID,
			Name:                   loc.LocationName,
			Description:            loc.VisualDescription,
			ImageURL:               loc.ImageURL,
			CharacterIDsInLocation: loc.CharacterIDsInLocation,
		})
	}

	// Build response
	response := StoryDetailResponse{
		ID:            story.ID.Hex(),
		Title:         story.Story.Title,
		NewsArticle:   story.Story.NewsArticle,
		CoverImageURL: story.Story.CoverImageURL,
		Characters:    characters,
		Locations:     locations,
		CreatedAt:     story.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
