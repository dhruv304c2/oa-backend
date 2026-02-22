package handlers

import (
	"agent/agent"
	"agent/db"
	dbmodels "agent/db/models"
	"agent/models"
	"agent/prompts"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SpawnRequest struct {
	StoryID     string `json:"story_id"`
	CharacterID string `json:"character_id"`
}

type SpawnResponse struct {
	AgentID string `json:"agent_id"`
	Error   string `json:"error,omitempty"`
}

func SpawnAgentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SpawnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Convert story ID string to ObjectID
	storyObjID, err := primitive.ObjectIDFromHex(req.StoryID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SpawnResponse{Error: "Invalid story ID"})
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
		json.NewEncoder(w).Encode(SpawnResponse{Error: "Story not found"})
		return
	}

	// Find the character in the story
	var character *models.Character
	for _, char := range story.Story.Characters {
		if char.ID == req.CharacterID {
			character = &char
			break
		}
	}

	if character == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(SpawnResponse{Error: "Character not found in story"})
		return
	}

	// Construct system prompt for the character and get evidence IDs
	systemPrompt, evidenceIDs := prompts.ConstructCharacterSystemPrompt(character, &story)

	// Create agent document for database
	agentDoc := &dbmodels.AgentDocument{
		StoryID:             storyObjID,
		CharacterID:         character.ID,
		CharacterName:       character.Name,
		Personality:         character.PersonalityProfile,
		HoldsEvidenceIDs:    evidenceIDs,
		KnowsLocationIDs:    character.KnowsLocationIDs,
		RevealedEvidenceIDs: make(map[string]bool),
		RevealedLocationIDs: make(map[string]bool),
	}

	// Create in database first
	dbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	agentObjID, err := db.CreateAgent(dbCtx, agentDoc)
	if err != nil {
		// Fallback to in-memory only if DB fails
		log.Printf("Failed to create agent in DB: %v", err)
		agentObjID = primitive.NewObjectID()
	}

	// Spawn agent with DB ID
	agentIDStr := agentObjID.Hex()
	agent.SpawnAgentWithCharacterAndID(agentIDStr, systemPrompt, story.Story.FullStory,
		req.StoryID, character.ID, character.Name, character.PersonalityProfile,
		evidenceIDs, character.KnowsLocationIDs)

	// Save the initial system prompt as the first conversation message
	// This ensures the agent can be properly reconstructed after server restart
	fullSystemPrompt := fmt.Sprintf("%s\n\n[STORY CONTEXT FOR REFERENCE]:\n%s", systemPrompt, story.Story.FullStory)
	go func(agentID, content string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// For system prompts, full content has everything but client content is empty (hidden)
		if err := db.SaveConversationMessageWithVersions(ctx, agentID, content, "", "model", 0); err != nil {
			log.Printf("Failed to persist initial system prompt: %v", err)
		} else {
			log.Printf("[SPAWN_SUCCESS] Saved initial system prompt for agent %s", character.Name)
		}
	}(agentIDStr, fullSystemPrompt)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(SpawnResponse{AgentID: agentIDStr})
}
