package handlers

import (
	"agent/agent"
	"agent/db"
	"agent/models"
	"context"
	"encoding/json"
	"fmt"
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

	// Construct system prompt for the character
	systemPrompt := constructCharacterSystemPrompt(character, &story)

	// Spawn agent with character system prompt and story context
	agentID := agent.SpawnAgentWithCharacter(systemPrompt, story.Story.FullStory)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(SpawnResponse{AgentID: agentID})
}

func constructCharacterSystemPrompt(character *models.Character, story *models.Story) string {
	// Build evidence description
	evidenceDescriptions := ""
	if len(character.HoldsEvidence) > 0 {
		evidenceDescriptions = "\n\nEvidence you possess:\n"
		for _, evidence := range character.HoldsEvidence {
			evidenceDescriptions += fmt.Sprintf("- [%s] %s\n  (Visual: %s)\n",
				evidence.ID, evidence.Description, evidence.VisualDescription)
		}
	}

	// Build known locations
	knownLocations := ""
	if len(character.KnowsLocationIDs) > 0 {
		knownLocations = "\n\nLocations you are familiar with:\n"
		for _, locID := range character.KnowsLocationIDs {
			// Find location details
			for _, loc := range story.Story.Locations {
				if loc.ID == locID {
					knownLocations += fmt.Sprintf("- %s: %s\n", loc.LocationName, loc.VisualDescription)
					break
				}
			}
		}
	}

	systemPrompt := fmt.Sprintf(`You are %s.

APPEARANCE: %s

PERSONALITY: %s

YOUR KNOWLEDGE AND BACKGROUND:
%s
%s%s

IMPORTANT INSTRUCTIONS:
- You must stay in character at all times as %s
- Respond based on your personality profile and knowledge base
- You can only share information that aligns with what your character would know
- Maintain consistency with your character's personality traits
- If asked about something outside your knowledge base, respond as your character would (perhaps with suspicion, curiosity, denial, etc.)
- React to questions and situations according to your personality profile
- Remember your relationships and attitudes toward other characters in the story`,
		character.Name,
		character.AppearanceDescription,
		character.PersonalityProfile,
		character.KnowledgeBase,
		evidenceDescriptions,
		knownLocations,
		character.Name)

	return systemPrompt
}
