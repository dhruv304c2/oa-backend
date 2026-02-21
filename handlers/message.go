package handlers

import (
	"agent/agent"
	"agent/db"
	"agent/models"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/genai"
)

type MessageRequest struct {
	AgentID           string   `json:"agent_id"`
	Message           string   `json:"message"`
	PresentedEvidence []string `json:"presented_evidence,omitempty"`
	LocationID        string   `json:"location_id,omitempty"`
}

type MessageResponse struct {
	Reply             string   `json:"reply"`
	RevealedEvidences []string `json:"revealed_evidences"`
	RevealedLocations []string `json:"revealed_locations"`
}

func MessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MessageRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	agentObj, ok := agent.GetAgentByID(req.AgentID)
	if !ok {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	// Process location context
	userMessage := req.Message
	if req.LocationID != "" {
		locationDetails, err := fetchLocationDetails(agentObj.StoryID, req.LocationID)
		if err != nil {
			http.Error(w, "Failed to fetch location details", http.StatusInternalServerError)
			return
		}

		if locationDetails != nil {
			userMessage = fmt.Sprintf("[CURRENT LOCATION: %s - %s]\n\n%s",
				locationDetails.LocationName, locationDetails.VisualDescription, userMessage)
		}
	}

	// Process presented evidence
	if len(req.PresentedEvidence) > 0 {
		evidenceDetails, err := fetchEvidenceDetails(agentObj.StoryID, req.PresentedEvidence)
		if err != nil {
			http.Error(w, "Failed to fetch evidence details", http.StatusInternalServerError)
			return
		}

		// Append evidence details to the user message
		if len(evidenceDetails) > 0 {
			userMessage += "\n\n[USER IS PRESENTING THE FOLLOWING EVIDENCE TO YOU]:"
			for _, evidence := range evidenceDetails {
				userMessage += fmt.Sprintf("\n- [%s] %s: %s\n  (Visual: %s)",
					evidence.ID, evidence.Title, evidence.Description, evidence.VisualDescription)
				if evidence.ImageURL != "" {
					userMessage += fmt.Sprintf("\n  (Image: %s)", evidence.ImageURL)
				}
			}
		}
	}

	// Add user message to history
	agentObj.History = append(agentObj.History, genai.NewContentFromText(userMessage, genai.RoleUser))

	// Create Gemini client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("GEMINI_API_KEY"),
	})
	if err != nil {
		http.Error(w, "Failed to create client", http.StatusInternalServerError)
		return
	}

	// Configure generation for JSON output
	genConfig := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	}

	resp, err := client.Models.GenerateContent(ctx, "gemini-2.5-flash", agentObj.History, genConfig)
	if err != nil {
		http.Error(w, "Failed to get response", http.StatusInternalServerError)
		return
	}

	// Parse the JSON response
	aiResponse, err := parseAIResponse(resp.Text())
	if err != nil {
		// Fallback to plain text response if JSON parsing fails
		aiResponse = &MessageResponse{
			Reply:             resp.Text(),
			RevealedEvidences: []string{},
			RevealedLocations: []string{},
		}
	} else {
		// Validate revealed items against what the agent actually has
		aiResponse.RevealedEvidences = validateRevealedItems(aiResponse.RevealedEvidences, agentObj.HoldsEvidenceIDs)
		aiResponse.RevealedLocations = validateRevealedItems(aiResponse.RevealedLocations, agentObj.KnowsLocationIDs)

		// Update tracking
		updateAgentTracking(agentObj, aiResponse.RevealedEvidences, aiResponse.RevealedLocations)
	}

	// Add the reply to history
	agentObj.History = append(agentObj.History, genai.NewContentFromText(aiResponse.Reply, genai.RoleModel))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(aiResponse)
}

// parseAIResponse parses the JSON response from the AI
func parseAIResponse(text string) (*MessageResponse, error) {
	var response MessageResponse
	err := json.Unmarshal([]byte(text), &response)
	return &response, err
}

// validateRevealedItems filters out any IDs that the agent doesn't actually possess
func validateRevealedItems(revealed []string, allowed []string) []string {
	allowedMap := make(map[string]bool)
	for _, id := range allowed {
		allowedMap[id] = true
	}

	var validated []string
	for _, id := range revealed {
		if allowedMap[id] {
			validated = append(validated, id)
		}
	}
	return validated
}

// updateAgentTracking updates the agent's tracking of revealed items
func updateAgentTracking(agent *agent.Agent, evidences []string, locations []string) {
	for _, id := range evidences {
		agent.RevealedEvidenceIDs[id] = true
	}
	for _, id := range locations {
		agent.RevealedLocationIDs[id] = true
	}
}

// fetchEvidenceDetails queries the database for evidence details
func fetchEvidenceDetails(storyID string, evidenceIDs []string) ([]models.Evidence, error) {
	// Convert story ID string to ObjectID
	storyObjID, err := primitive.ObjectIDFromHex(storyID)
	if err != nil {
		return nil, err
	}

	// Fetch story from MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var story models.Story
	collection := db.GetCollection("stories")
	err = collection.FindOne(ctx, bson.M{"_id": storyObjID}).Decode(&story)
	if err != nil {
		return nil, err
	}

	// Find requested evidence in the story
	var evidenceDetails []models.Evidence
	evidenceMap := make(map[string]bool)
	for _, id := range evidenceIDs {
		evidenceMap[id] = true
	}

	// Search through all characters to find the evidence
	for _, character := range story.Story.Characters {
		for _, evidence := range character.HoldsEvidence {
			if evidenceMap[evidence.ID] {
				evidenceDetails = append(evidenceDetails, evidence)
			}
		}
	}

	return evidenceDetails, nil
}

// fetchLocationDetails queries the database for location details
func fetchLocationDetails(storyID string, locationID string) (*models.Location, error) {
	// Convert story ID string to ObjectID
	storyObjID, err := primitive.ObjectIDFromHex(storyID)
	if err != nil {
		return nil, err
	}

	// Fetch story from MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var story models.Story
	collection := db.GetCollection("stories")
	err = collection.FindOne(ctx, bson.M{"_id": storyObjID}).Decode(&story)
	if err != nil {
		return nil, err
	}

	// Find the requested location in the story
	for _, location := range story.Story.Locations {
		if location.ID == locationID {
			return &location, nil
		}
	}

	return nil, nil
}
