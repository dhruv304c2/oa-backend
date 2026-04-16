package handlers

import (
	"agent/db"
	"agent/db/models"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FeedbackRequest struct {
	StoryID           string `json:"story_id"`
	PlayerID          string `json:"player_id,omitempty"`
	QuestStoryRating  int    `json:"quest_story_rating"`
	NPCDialogueRating int   `json:"npc_dialogue_rating"`
	GameplayRating    int    `json:"gameplay_rating"`
	OverallRating     int    `json:"overall_rating"`
	WouldPlayAgain    string `json:"would_play_again"`
	LikedText         string `json:"liked_text,omitempty"`
	DislikedText      string `json:"disliked_text,omitempty"`
}

type FeedbackListRequest struct {
	StoryID string `json:"story_id"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}

func isValidRating(r int) bool {
	return r >= 1 && r <= 5
}

func isValidWouldPlayAgain(s string) bool {
	return s == "yes" || s == "maybe" || s == "no"
}

func SubmitFeedbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Validate story_id
	if req.StoryID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "story_id is required"})
		return
	}

	storyObjID, err := primitive.ObjectIDFromHex(req.StoryID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid story ID"})
		return
	}

	// Validate ratings
	if !isValidRating(req.QuestStoryRating) || !isValidRating(req.NPCDialogueRating) ||
		!isValidRating(req.GameplayRating) || !isValidRating(req.OverallRating) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "All ratings must be between 1 and 5"})
		return
	}

	// Validate would_play_again
	if !isValidWouldPlayAgain(req.WouldPlayAgain) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "would_play_again must be 'yes', 'maybe', or 'no'"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	doc := &models.FeedbackDocument{
		StoryID:           storyObjID,
		PlayerID:          req.PlayerID,
		QuestStoryRating:  req.QuestStoryRating,
		NPCDialogueRating: req.NPCDialogueRating,
		GameplayRating:    req.GameplayRating,
		OverallRating:     req.OverallRating,
		WouldPlayAgain:    req.WouldPlayAgain,
		LikedText:         req.LikedText,
		DislikedText:      req.DislikedText,
	}

	id, err := db.SaveFeedback(ctx, doc)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save feedback"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"id":      id.Hex(),
		"message": "Feedback submitted successfully",
	})
}

func GetFeedbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FeedbackListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if req.StoryID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "story_id is required"})
		return
	}

	storyObjID, err := primitive.ObjectIDFromHex(req.StoryID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid story ID"})
		return
	}

	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 50
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	feedbacks, total, err := db.GetFeedbackByStory(ctx, storyObjID, req.Limit, req.Offset)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to fetch feedback"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"feedback": feedbacks,
		"total":    total,
		"limit":    req.Limit,
		"offset":   req.Offset,
	})
}
