package handlers

import (
	"agent/agent"
	"agent/config"
	"agent/db"
	"agent/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/genai"
)

type MessageRequest struct {
	AgentID           string   `json:"agent_id"`
	Message           string   `json:"message"`
	PresentedEvidence []string `json:"presented_evidence_ids,omitempty"`
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

	log.Printf("[MESSAGE_REQUEST] Received request for agent %s", req.AgentID)
	agentObj, ok := agent.GetAgentByID(req.AgentID)
	if !ok {
		log.Printf("[MESSAGE_ERROR] Agent %s not found in memory or database", req.AgentID)
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}
	log.Printf("[MESSAGE_AGENT_FOUND] Agent %s (%s) retrieved successfully", agentObj.CharacterName, req.AgentID)

	// Validate agent has required fields after loading from DB
	if agentObj.StoryID == "" {
		log.Printf("[MESSAGE_ERROR] Agent %s has empty StoryID", agentObj.CharacterName)
		http.Error(w, "Agent configuration invalid", http.StatusInternalServerError)
		return
	}

	// Process location context
	userMessage := req.Message
	if req.LocationID != "" {
		locationDetails, err := fetchLocationDetails(agentObj.StoryID, req.LocationID)
		if err != nil {
			log.Printf("[MESSAGE_ERROR] Failed to fetch location details for agent %s, location %s: %v",
				agentObj.CharacterName, req.LocationID, err)
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
			log.Printf("[MESSAGE_ERROR] Failed to fetch evidence details for agent %s, evidence IDs %v: %v",
				agentObj.CharacterName, req.PresentedEvidence, err)
			http.Error(w, "Failed to fetch evidence details", http.StatusInternalServerError)
			return
		}

		// Append evidence details to the user message
		if len(evidenceDetails) > 0 {
			userMessage += "\n\n========================================\n"
			userMessage += "[USER IS PRESENTING THE FOLLOWING EVIDENCE TO YOU]:\n"
			userMessage += "========================================\n"
			for _, evidence := range evidenceDetails {
				userMessage += fmt.Sprintf("EVIDENCE: %s\n", evidence.Title)
				userMessage += fmt.Sprintf("Description: %s\n", evidence.Description)
				userMessage += fmt.Sprintf("Visual: %s\n", evidence.VisualDescription)
				if evidence.ImageURL != "" {
					userMessage += fmt.Sprintf("Image: %s\n", evidence.ImageURL)
				}
				userMessage += "----------------------------------------\n"
			}

			// Add verification logging
			log.Printf("[MESSAGE_EVIDENCE] Added %d evidence items to message for agent %s. Total message length: %d",
				len(evidenceDetails), agentObj.CharacterName, len(userMessage))
		}
	}

	// Add user message to history (validate it's not empty)
	if strings.TrimSpace(userMessage) == "" {
		log.Printf("[MESSAGE_ERROR] Received empty user message")
		http.Error(w, "Message cannot be empty", http.StatusBadRequest)
		return
	}

	log.Printf("[MESSAGE_DEBUG] Adding user message to history. Current history length: %d, Message length: %d",
		len(agentObj.History), len(userMessage))
	agentObj.History = append(agentObj.History, genai.NewContentFromText(userMessage, genai.RoleUser))

	// Save user message asynchronously with both versions
	go func(agentID, fullContent string, index int) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Extract clean version for client
		clientContent := extractClientContent(fullContent, "user")

		if err := db.SaveConversationMessageWithVersions(ctx, agentID, fullContent, clientContent, "user", index, nil, nil); err != nil {
			log.Printf("Failed to persist user message: %v", err)
		}
	}(req.AgentID, userMessage, len(agentObj.History)-1)

	// Create Gemini client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: config.GetGeminiAPIKey(),
	})
	if err != nil {
		log.Printf("[MESSAGE_ERROR] Failed to create Gemini client for agent %s: %v", agentObj.CharacterName, err)
		http.Error(w, "Failed to create client", http.StatusInternalServerError)
		return
	}

	// Generate JSON response directly
	// Ensure we don't have any nil entries in history
	validHistory := make([]*genai.Content, 0, len(agentObj.History))
	for i, content := range agentObj.History {
		if content != nil {
			validHistory = append(validHistory, content)
		} else {
			log.Printf("[MESSAGE_WARNING] Found nil content at index %d", i)
		}
	}

	genConfig := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	}

	log.Printf("[MESSAGE_DEBUG] Calling Gemini for agent %s with history length: %d",
		agentObj.CharacterName, len(validHistory))
	resp, err := client.Models.GenerateContent(ctx, config.GetGeminiModel(), validHistory, genConfig)
	if err != nil {
		log.Printf("[MESSAGE_ERROR] Failed to get JSON response for agent %s: %v",
			agentObj.CharacterName, err)
		http.Error(w, "Failed to get response", http.StatusInternalServerError)
		return
	}

	// Update agentObj.History to use the validated history
	agentObj.History = validHistory

	// Parse JSON response
	var aiResponse MessageResponse
	rawResponse := resp.Text()
	log.Printf("[MESSAGE_JSON_RAW] Agent %s raw JSON: %s", agentObj.CharacterName, rawResponse)

	if err := json.Unmarshal([]byte(rawResponse), &aiResponse); err != nil {
		log.Printf("[MESSAGE_JSON_ERROR] Failed to parse JSON for %s: %v",
			agentObj.CharacterName, err)

		// Retry with format clarification
		retryResponse, retryErr := retryWithJSONFormat(ctx, agentObj, validHistory)
		if retryErr != nil {
			log.Printf("[MESSAGE_RETRY_ERROR] Retry failed for %s: %v",
				agentObj.CharacterName, retryErr)
			// Fallback to safe response
			aiResponse = MessageResponse{
				Reply:             generateFallbackResponse(agentObj),
				RevealedEvidences: []string{},
				RevealedLocations: []string{},
			}
		} else {
			aiResponse = *retryResponse
		}
	}

	// Validate revealed items match character's possessions
	originalEvidenceCount := len(aiResponse.RevealedEvidences)
	originalLocationCount := len(aiResponse.RevealedLocations)

	aiResponse.RevealedEvidences = validateRevealedItems(aiResponse.RevealedEvidences, agentObj.HoldsEvidenceIDs)

	// Use location detector instead of validation against KnowsLocationIDs
	storyObjID, err := primitive.ObjectIDFromHex(agentObj.StoryID)
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var story models.Story
		collection := db.GetCollection("stories")
		err = collection.FindOne(ctx, bson.M{"_id": storyObjID}).Decode(&story)

		if err == nil {
			// Use location detector as sole source of location reveals
			detector := NewLocationRevealDetector(&story)
			aiResponse.RevealedLocations = detector.DetectRevealedLocations(aiResponse.Reply)

			log.Printf("[MESSAGE_LOCATION_DETECTION] Agent %s - Detector found locations: %v",
				agentObj.CharacterName, aiResponse.RevealedLocations)
		} else {
			log.Printf("[MESSAGE_ERROR] Failed to fetch story for location detection: %v", err)
			aiResponse.RevealedLocations = []string{}
		}
	} else {
		log.Printf("[MESSAGE_ERROR] Invalid story ID for location detection: %v", err)
		aiResponse.RevealedLocations = []string{}
	}

	// Log if items were filtered
	if len(aiResponse.RevealedEvidences) < originalEvidenceCount {
		log.Printf("[MESSAGE_VALIDATION] Filtered out %d invalid evidence reveals for %s",
			originalEvidenceCount-len(aiResponse.RevealedEvidences), agentObj.CharacterName)
	}
	// Log detected locations vs. AI provided locations (for debugging)
	if originalLocationCount != len(aiResponse.RevealedLocations) {
		log.Printf("[MESSAGE_LOCATION_DETECTION] Agent %s - AI provided %d locations, detector found %d locations",
			agentObj.CharacterName, originalLocationCount, len(aiResponse.RevealedLocations))
	}

	// Update tracking
	updateAgentTracking(agentObj, aiResponse.RevealedEvidences, aiResponse.RevealedLocations)

	// Add the reply to history (ensure it's not empty)
	if strings.TrimSpace(aiResponse.Reply) == "" {
		log.Printf("[MESSAGE_WARNING] AI returned empty response, using default message")
		aiResponse.Reply = "I apologize, but I couldn't formulate a proper response. Could you please rephrase your question?"
	}
	agentObj.History = append(agentObj.History, genai.NewContentFromText(aiResponse.Reply, genai.RoleModel))

	// Save AI response asynchronously with both versions
	go func(agentID, processedContent, naturalContent string, revealedEvidences []string, revealedLocations []string, index int) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// For AI responses, natural response is the full content, processed is the client content
		if err := db.SaveConversationMessageWithVersions(ctx, agentID, naturalContent, processedContent, "model", index, revealedEvidences, revealedLocations); err != nil {
			log.Printf("Failed to persist AI response: %v", err)
		}
	}(req.AgentID, aiResponse.Reply, naturalResponse, aiResponse.RevealedEvidences, aiResponse.RevealedLocations, len(agentObj.History)-1)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(aiResponse)
}

// Retry function for JSON format issues
func retryWithJSONFormat(ctx context.Context, agent *agent.Agent, history []*genai.Content) (*MessageResponse, error) {
	clarification := genai.NewContentFromText(
		`Please respond in valid JSON format:
{
  "reply": "your spoken dialogue with [actions] in brackets",
  "revealed_evidences": ["evidence IDs you're giving"]
}`, genai.RoleUser)

	retryHistory := append(history, clarification)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: config.GetGeminiAPIKey(),
	})
	if err != nil {
		return nil, err
	}

	genConfig := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	}

	resp, err := client.Models.GenerateContent(ctx, config.GetGeminiModel(),
		retryHistory, genConfig)
	if err != nil {
		return nil, err
	}

	var response MessageResponse
	if err := json.Unmarshal([]byte(resp.Text()), &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// Generate personality-appropriate fallback
func generateFallbackResponse(agent *agent.Agent) string {
	personality := strings.ToLower(agent.Personality)

	if strings.Contains(personality, "nervous") {
		return "I-I'm sorry, I'm having trouble understanding... Could you repeat that?"
	} else if strings.Contains(personality, "arrogant") {
		return "Speak clearly. I don't have time for your mumbling."
	} else if strings.Contains(personality, "professional") {
		return "I apologize, could you please rephrase your question?"
	}

	return "I'm having trouble understanding. Could you rephrase that?"
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

// fetchLocationDetailsForIDs retrieves multiple location details by their IDs
func fetchLocationDetailsForIDs(storyID string, locationIDs []string) ([]models.Location, error) {
	storyObjID, err := primitive.ObjectIDFromHex(storyID)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var story models.Story
	collection := db.GetCollection("stories")
	err = collection.FindOne(ctx, bson.M{"_id": storyObjID}).Decode(&story)
	if err != nil {
		return nil, err
	}

	// Create a map for quick lookup
	locationMap := make(map[string]bool)
	for _, id := range locationIDs {
		locationMap[id] = true
	}

	// Filter locations by IDs
	var locations []models.Location
	for _, loc := range story.Story.Locations {
		if locationMap[loc.ID] {
			locations = append(locations, loc)
		}
	}

	return locations, nil
}








