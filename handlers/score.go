package handlers

import (
	"agent/config"
	"agent/db"
	"agent/models"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/genai"
)

type ScoreRequest struct {
	StoryID            string   `json:"story_id"`
	Theory             string   `json:"theory"`
	DiscoveredEvidence []string `json:"discovered_evidence,omitempty"`
}

type ScoreResponse struct {
	Score  int    `json:"score"`
	Reason string `json:"reason"`
}

// formatDiscoveredEvidence formats the discovered evidence for the scoring prompt
func formatDiscoveredEvidence(evidenceList []models.Evidence) string {
	if len(evidenceList) == 0 {
		return "No evidence discovered."
	}

	var formatted string
	for _, evidence := range evidenceList {
		formatted += fmt.Sprintf("- [%s] %s: %s\n",
			evidence.ID, evidence.Title, evidence.Description)
	}
	return formatted
}

func ScoreTheoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ScoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Convert story ID string to ObjectID
	storyObjID, err := primitive.ObjectIDFromHex(req.StoryID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid story ID"})
		return
	}

	// Fetch story from MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	// Fetch evidence details if provided
	var evidenceDetails []models.Evidence
	if len(req.DiscoveredEvidence) > 0 {
		evidenceDetails, err = fetchEvidenceDetails(req.StoryID, req.DiscoveredEvidence)
		if err != nil {
			// Log the error but continue with scoring without evidence details
			// This ensures backward compatibility
			evidenceDetails = []models.Evidence{}
		}
	}

	// Create Gemini client
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: config.GetGeminiAPIKey(),
	})
	if err != nil {
		http.Error(w, "Failed to create AI client", http.StatusInternalServerError)
		return
	}

	// Construct prompt for scoring
	prompt := fmt.Sprintf(`You are a mystery game judge. Compare the player's theory to the actual story and score their accuracy.

ACTUAL STORY:
%s

EVIDENCE THE PLAYER HAS DISCOVERED:
%s

PLAYER'S THEORY:
%s

Score the player's theory based on:
1. Correct identification of the culprit (30 points)
2. Understanding of motive (20 points)
3. Correct sequence of events (20 points)
4. Effective use of discovered evidence (20 points)
   - Did they find the right evidence?
   - Did they correctly interpret the evidence?
   - Is their theory supported by the evidence they found?
5. Understanding of relationships between characters (10 points)

Additional considerations:
- If they missed critical evidence, note what they should have found
- If they have the right evidence but wrong conclusions, partial credit
- Bonus points if they found particularly hidden or clever evidence

Respond in JSON format:
{
  "score": <number between 0-100>,
  "reason": "<brief explanation including what evidence they used well or missed>"
}

Be fair but precise in scoring. If they got the main culprit wrong, they cannot score above 60.`,
		story.Story.FullStory,
		formatDiscoveredEvidence(evidenceDetails),
		req.Theory)

	// Configure generation for JSON output
	genConfig := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	}

	// Get AI response
	resp, err := client.Models.GenerateContent(ctx, config.GetGeminiModel(),
		[]*genai.Content{genai.NewContentFromText(prompt, genai.RoleUser)},
		genConfig)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Failed to generate score: %v", err),
		})
		return
	}

	// Parse the JSON response
	var scoreResp ScoreResponse
	if err := json.Unmarshal([]byte(resp.Text()), &scoreResp); err != nil {
		// Fallback response if parsing fails
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ScoreResponse{
			Score:  0,
			Reason: "Failed to process theory",
		})
		return
	}

	// Return the score
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(scoreResp)
}
