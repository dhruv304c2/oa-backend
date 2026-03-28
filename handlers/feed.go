package handlers

import (
	"agent/db"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func FeedHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := db.GetCollection("stories")
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		http.Error(w, "Failed to fetch stories", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var stories []bson.M
	if err = cursor.All(ctx, &stories); err != nil {
		http.Error(w, "Failed to decode stories", http.StatusInternalServerError)
		return
	}

	feedItems := make([]bson.M, 0, len(stories))
	for _, s := range stories {
		item := bson.M{
			"id":         s["_id"],
			"created_at": s["created_at"],
			"updated_at": s["updated_at"],
		}
		if story, ok := s["story"].(bson.M); ok {
			item["title"] = story["title"]
			item["cover_image_url"] = story["cover_image_url"]
			if newsArticle, ok := story["news_article"].(bson.M); ok {
				item["description"] = newsArticle["content"]
			}
		}
		feedItems = append(feedItems, item)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(feedItems)
}

func FeedHandlerV2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	collectionName := r.URL.Query().Get("collection")
	if collectionName == "" {
		collectionName = "stories"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := db.GetCollection(collectionName)
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		http.Error(w, "Failed to fetch stories", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var stories []bson.M
	if err = cursor.All(ctx, &stories); err != nil {
		http.Error(w, "Failed to decode stories", http.StatusInternalServerError)
		return
	}

	feedItems := make([]bson.M, 0, len(stories))
	for _, s := range stories {
		item := bson.M{
			"id":         s["_id"],
			"created_at": s["created_at"],
			"updated_at": s["updated_at"],
		}
		if story, ok := s["story"].(bson.M); ok {
			item["title"] = story["title"]
			item["cover_image_url"] = story["cover_image_url"]
			if newsArticle, ok := story["news_article"].(bson.M); ok {
				item["description"] = newsArticle["content"]
			}
		}
		feedItems = append(feedItems, item)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(feedItems)
}

func StoryDetailHandlerV2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	storyID := r.URL.Query().Get("id")
	if storyID == "" {
		http.Error(w, "Story ID is required", http.StatusBadRequest)
		return
	}

	collectionName := r.URL.Query().Get("collection")
	if collectionName == "" {
		collectionName = "stories"
	}

	storyObjID, err := primitive.ObjectIDFromHex(storyID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid story ID"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var story bson.M
	collection := db.GetCollection(collectionName)
	err = collection.FindOne(ctx, bson.M{"_id": storyObjID}).Decode(&story)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Story not found"})
		return
	}

	story["id"] = story["_id"]
	delete(story, "_id")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(story)
}

func StoryDetailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	storyID := r.URL.Query().Get("id")
	if storyID == "" {
		http.Error(w, "Story ID is required", http.StatusBadRequest)
		return
	}

	storyObjID, err := primitive.ObjectIDFromHex(storyID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid story ID"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var story bson.M
	collection := db.GetCollection("stories")
	err = collection.FindOne(ctx, bson.M{"_id": storyObjID}).Decode(&story)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Story not found"})
		return
	}

	story["id"] = story["_id"]
	delete(story, "_id")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(story)
}
