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
	"strings"
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
				userMessage += fmt.Sprintf("\n- %s: %s\n  (Visual: %s)",
					evidence.Title, evidence.Description, evidence.VisualDescription)
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
		// First, we need to fetch the story to build name mappings
		storyObjID, err := primitive.ObjectIDFromHex(agentObj.StoryID)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			var story models.Story
			collection := db.GetCollection("stories")
			err = collection.FindOne(ctx, bson.M{"_id": storyObjID}).Decode(&story)

			if err == nil {
				// Build name-to-ID mappings
				evidenceMap := buildEvidenceNameMap(&story)
				locationMap := buildLocationNameMap(&story)

				// Convert natural names to IDs
				evidenceIDs := mapRevealedNamesToIDs(aiResponse.RevealedEvidences, evidenceMap)
				locationIDs := mapRevealedNamesToIDs(aiResponse.RevealedLocations, locationMap)

				// Validate and track using IDs internally
				aiResponse.RevealedEvidences = validateRevealedItems(evidenceIDs, agentObj.HoldsEvidenceIDs)
				aiResponse.RevealedLocations = validateRevealedItems(locationIDs, agentObj.KnowsLocationIDs)

				// Update tracking
				updateAgentTracking(agentObj, aiResponse.RevealedEvidences, aiResponse.RevealedLocations)
			}
		}

		// Remove the verification system call (comment out line 122)
		// aiResponse, err = verifyAndModifyResponse(aiResponse, agentObj, "")
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

// buildEvidenceNameMap creates a mapping from evidence names to IDs
func buildEvidenceNameMap(story *models.Story) map[string]string {
	nameToID := make(map[string]string)
	for _, char := range story.Story.Characters {
		for _, ev := range char.HoldsEvidence {
			// Use lowercase for case-insensitive matching
			nameToID[strings.ToLower(ev.Title)] = ev.ID
		}
	}
	return nameToID
}

// buildLocationNameMap creates a mapping from location names to IDs
func buildLocationNameMap(story *models.Story) map[string]string {
	nameToID := make(map[string]string)
	for _, loc := range story.Story.Locations {
		// Use lowercase for case-insensitive matching
		nameToID[strings.ToLower(loc.LocationName)] = loc.ID
	}
	return nameToID
}

// mapRevealedNamesToIDs converts natural names to IDs using the mapping
func mapRevealedNamesToIDs(names []string, nameMap map[string]string) []string {
	var ids []string
	for _, name := range names {
		if id, exists := nameMap[strings.ToLower(strings.TrimSpace(name))]; exists {
			ids = append(ids, id)
		}
	}
	return ids
}

// ExtractedItems holds the extracted location and evidence mentions from a dialogue
// type ExtractedItems struct {
// 	Locations         []string `json:"locations"`
// 	Evidence          []string `json:"evidence"`
// 	AmbiguousMentions []string `json:"ambiguous_mentions,omitempty"`
// }

// extractMentionedItems uses LLM to extract all location/evidence mentions from dialogue
// func extractMentionedItems(dialogue string, storyContext string, storyID string) (*ExtractedItems, error) {
// 	// Fetch the story to get all available locations and evidence for reference
// 	storyObjID, err := primitive.ObjectIDFromHex(storyID)
// 	if err != nil {
// 		return nil, err
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
// 	defer cancel()

// 	var story models.Story
// 	collection := db.GetCollection("stories")
// 	err = collection.FindOne(ctx, bson.M{"_id": storyObjID}).Decode(&story)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Build reference lists
// 	var allLocationNames []string
// 	for _, loc := range story.Story.Locations {
// 		allLocationNames = append(allLocationNames, fmt.Sprintf("%s (ID: %s)", loc.LocationName, loc.ID))
// 	}

// 	var allEvidenceNames []string
// 	for _, char := range story.Story.Characters {
// 		for _, ev := range char.HoldsEvidence {
// 			allEvidenceNames = append(allEvidenceNames, fmt.Sprintf("%s (ID: %s)", ev.Title, ev.ID))
// 		}
// 	}

// 	// Construct extraction prompt
// 	extractionPrompt := fmt.Sprintf(`Analyze this character dialogue and extract ALL mentions of locations and evidence.

// Dialogue: "%s"

// Story context for reference:
// - Known locations in story: %v
// - Known evidence types in story: %v

// Extract any mention of:
// - Physical locations (buildings, rooms, areas)
// - Evidence items (objects, documents, clues)
// - References to places or items, even indirect

// Return JSON:
// {
//   "locations": ["location_id1", "location_id2"],
//   "evidence": ["evidence_id1", "evidence_id2"],
//   "ambiguous_mentions": ["description of unclear references"]
// }

// IMPORTANT: Only include IDs if the dialogue clearly mentions that specific location or evidence.`,
// 		dialogue,
// 		allLocationNames,
// 		allEvidenceNames)

// 	// Create Gemini client
// 	client, err := genai.NewClient(ctx, &genai.ClientConfig{
// 		APIKey: os.Getenv("GEMINI_API_KEY"),
// 	})
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Configure for JSON output
// 	genConfig := &genai.GenerateContentConfig{
// 		ResponseMIMEType: "application/json",
// 	}

// 	resp, err := client.Models.GenerateContent(ctx, "gemini-2.5-flash",
// 		[]*genai.Content{genai.NewContentFromText(extractionPrompt, genai.RoleUser)},
// 		genConfig)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Parse the response
// 	var extracted ExtractedItems
// 	err = json.Unmarshal([]byte(resp.Text()), &extracted)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &extracted, nil
// }

// findUnknownItems returns items that are mentioned but not in the known list
// func findUnknownItems(mentioned []string, known []string) []string {
// 	knownMap := make(map[string]bool)
// 	for _, id := range known {
// 		knownMap[id] = true
// 	}

// 	var unknown []string
// 	for _, id := range mentioned {
// 		if !knownMap[id] {
// 			unknown = append(unknown, id)
// 		}
// 	}
// 	return unknown
// }

// modifyDialogue modifies the dialogue to handle unknown mentions appropriately
// func modifyDialogue(originalDialogue string, unknownLocations []string, unknownEvidence []string, agent *agent.Agent) (string, error) {
// 	if len(unknownLocations) == 0 && len(unknownEvidence) == 0 {
// 		return originalDialogue, nil
// 	}

// 	// Construct modification prompt
// 	modPrompt := fmt.Sprintf(`You are %s with personality: %s

// Your previous response mentioned items you don't actually know about:
// - Unknown locations: %v
// - Unknown evidence: %v

// Modify your response to reflect your lack of knowledge while staying in character:
// - Use natural phrases that fit your personality
// - Don't break the flow of conversation
// - Options: express confusion, redirect, be vague, or show suspicion

// Original response: "%s"

// Modified response:`,
// 		agent.CharacterName,
// 		agent.Personality,
// 		unknownLocations,
// 		unknownEvidence,
// 		originalDialogue)

// 	// Create Gemini client
// 	ctx := context.Background()
// 	client, err := genai.NewClient(ctx, &genai.ClientConfig{
// 		APIKey: os.Getenv("GEMINI_API_KEY"),
// 	})
// 	if err != nil {
// 		return originalDialogue, err
// 	}

// 	resp, err := client.Models.GenerateContent(ctx, "gemini-2.5-flash",
// 		[]*genai.Content{genai.NewContentFromText(modPrompt, genai.RoleUser)},
// 		nil)
// 	if err != nil {
// 		return originalDialogue, err
// 	}

// 	return resp.Text(), nil
// }

// verifyAndModifyResponse checks and modifies responses that mention unknown items
// func verifyAndModifyResponse(response *MessageResponse, agentObj *agent.Agent, storyContext string) (*MessageResponse, error) {
// 	// 1. Extract mentioned items using LLM
// 	extracted, err := extractMentionedItems(response.Reply, storyContext, agentObj.StoryID)
// 	if err != nil {
// 		// Log but don't fail - use original response
// 		fmt.Printf("Warning: Failed to extract mentioned items: %v\n", err)
// 		return response, nil
// 	}

// 	// 2. Compare with agent's known items
// 	unknownLocations := findUnknownItems(extracted.Locations, agentObj.KnowsLocationIDs)
// 	unknownEvidence := findUnknownItems(extracted.Evidence, agentObj.HoldsEvidenceIDs)

// 	// 3. If unknown items found, modify response
// 	if len(unknownLocations) > 0 || len(unknownEvidence) > 0 {
// 		fmt.Printf("Found unknown mentions - Locations: %v, Evidence: %v\n", unknownLocations, unknownEvidence)

// 		modified, err := modifyDialogue(response.Reply, unknownLocations, unknownEvidence, agentObj)
// 		if err != nil {
// 			fmt.Printf("Warning: Failed to modify dialogue: %v\n", err)
// 			return response, nil
// 		}

// 		response.Reply = modified
// 	}

// 	return response, nil
// }
