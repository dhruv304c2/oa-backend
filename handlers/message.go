package handlers

import (
	"agent/agent"
	"agent/db"
	"agent/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
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

	// Save user message asynchronously
	go func(agentID, content string, index int) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := db.SaveConversationMessage(ctx, agentID, content, "user", index); err != nil {
			log.Printf("Failed to persist user message: %v", err)
		}
	}(req.AgentID, userMessage, len(agentObj.History)-1)

	// Create Gemini client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("GEMINI_API_KEY"),
	})
	if err != nil {
		http.Error(w, "Failed to create client", http.StatusInternalServerError)
		return
	}

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
		storyObjID, err := primitive.ObjectIDFromHex(agentObj.StoryID)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			var story models.Story
			collection := db.GetCollection("stories")
			err = collection.FindOne(ctx, bson.M{"_id": storyObjID}).Decode(&story)

			if err == nil {
				// Extract ALL mentions from the dialogue
				mentions, err := extractMentionsFromDialogue(aiResponse.Reply, &story)
				if err != nil {
					fmt.Printf("Warning: Failed to extract mentions: %v\n", err)
				} else {
					// Find unavailable items
					unavailableLocations := findUnavailableLocations(mentions.Locations, agentObj.KnowsLocationIDs)
					unavailableEvidence := findUnavailableEvidence(mentions.Evidence, agentObj.HoldsEvidenceIDs)

					// Modify dialogue if needed
					if len(unavailableLocations) > 0 || len(unavailableEvidence) > 0 {
						modifiedReply, err := modifyDialogueForUnavailableItems(
							aiResponse.Reply,
							unavailableLocations,
							unavailableEvidence,
							agentObj)

						if err == nil {
							aiResponse.Reply = modifiedReply
						} else {
							fmt.Printf("Warning: Failed to modify dialogue: %v\n", err)
						}
					}
				}

				// Now handle the revealed items arrays
				evidenceMap := buildEvidenceNameMap(&story)
				locationMap := buildLocationNameMap(&story)

				evidenceIDs := mapRevealedNamesToIDs(aiResponse.RevealedEvidences, evidenceMap)
				locationIDs := mapRevealedNamesToIDs(aiResponse.RevealedLocations, locationMap)

				aiResponse.RevealedEvidences = validateRevealedItems(evidenceIDs, agentObj.HoldsEvidenceIDs)
				aiResponse.RevealedLocations = validateRevealedItems(locationIDs, agentObj.KnowsLocationIDs)

				updateAgentTracking(agentObj, aiResponse.RevealedEvidences, aiResponse.RevealedLocations)
			}
		}
	}

	// Add the reply to history
	agentObj.History = append(agentObj.History, genai.NewContentFromText(aiResponse.Reply, genai.RoleModel))

	// Save AI response asynchronously
	go func(agentID, content string, index int) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := db.SaveConversationMessage(ctx, agentID, content, "model", index); err != nil {
			log.Printf("Failed to persist AI response: %v", err)
		}
	}(req.AgentID, aiResponse.Reply, len(agentObj.History)-1)

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

// buildEvidenceNameMap creates a mapping from evidence names to IDs
func buildEvidenceNameMap(story *models.Story) map[string]string {
	nameToID := make(map[string]string)
	for _, char := range story.Story.Characters {
		for _, ev := range char.HoldsEvidence {
			nameToID[strings.ToLower(ev.Title)] = ev.ID
		}
	}
	return nameToID
}

// buildLocationNameMap creates a mapping from location names to IDs
func buildLocationNameMap(story *models.Story) map[string]string {
	nameToID := make(map[string]string)
	for _, loc := range story.Story.Locations {
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

// extractMentionsFromDialogue finds all location/evidence mentions
func extractMentionsFromDialogue(dialogue string, story *models.Story) (*ExtractedMentions, error) {
	// Build complete lists of all locations and evidence in the story
	allLocations := make(map[string]string) // name -> ID
	for _, loc := range story.Story.Locations {
		allLocations[strings.ToLower(loc.LocationName)] = loc.ID
	}

	allEvidence := make(map[string]string) // name -> ID
	for _, char := range story.Story.Characters {
		for _, ev := range char.HoldsEvidence {
			allEvidence[strings.ToLower(ev.Title)] = ev.ID
		}
	}

	// Use Gemini to extract mentions with context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	extractPrompt := fmt.Sprintf(`Analyze this dialogue and find ALL mentions of specific locations or evidence items.

Dialogue: "%s"

Known locations in the story: %v
Known evidence items in the story: %v

For each mention found, provide:
1. The exact name as it appears in the dialogue
2. The surrounding context (10-15 words around the mention)
3. Whether it's a location or evidence

Return JSON format:
{
  "locations": [
    {"name": "location name", "context": "surrounding text with the mention"},
    ...
  ],
  "evidence": [
    {"name": "evidence name", "context": "surrounding text with the mention"},
    ...
  ]
}

Be thorough - include direct mentions, references, and descriptions.`,
		dialogue,
		getLocationNames(story),
		getEvidenceNames(story))

	// Create Gemini client
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("GEMINI_API_KEY"),
	})
	if err != nil {
		return nil, err
	}

	genConfig := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	}

	resp, err := client.Models.GenerateContent(ctx, "gemini-2.5-flash",
		[]*genai.Content{genai.NewContentFromText(extractPrompt, genai.RoleUser)},
		genConfig)
	if err != nil {
		return nil, err
	}

	// Parse the response
	var tempExtracted struct {
		Locations []struct {
			Name    string `json:"name"`
			Context string `json:"context"`
		} `json:"locations"`
		Evidence []struct {
			Name    string `json:"name"`
			Context string `json:"context"`
		} `json:"evidence"`
	}

	err = json.Unmarshal([]byte(resp.Text()), &tempExtracted)
	if err != nil {
		return nil, err
	}

	// Map names to IDs
	mentions := &ExtractedMentions{
		Locations: []MentionedItem{},
		Evidence:  []MentionedItem{},
	}

	// Process locations
	for _, loc := range tempExtracted.Locations {
		if id, exists := allLocations[strings.ToLower(strings.TrimSpace(loc.Name))]; exists {
			mentions.Locations = append(mentions.Locations, MentionedItem{
				Name:    loc.Name,
				ID:      id,
				Context: loc.Context,
			})
		}
	}

	// Process evidence
	for _, ev := range tempExtracted.Evidence {
		if id, exists := allEvidence[strings.ToLower(strings.TrimSpace(ev.Name))]; exists {
			mentions.Evidence = append(mentions.Evidence, MentionedItem{
				Name:    ev.Name,
				ID:      id,
				Context: ev.Context,
			})
		}
	}

	return mentions, nil
}

// modifyDialogueForUnavailableItems adjusts dialogue to explain unavailable items
func modifyDialogueForUnavailableItems(
	originalDialogue string,
	unavailableLocations []MentionedItem,
	unavailableEvidence []MentionedItem,
	agent *agent.Agent) (string, error) {

	if len(unavailableLocations) == 0 && len(unavailableEvidence) == 0 {
		return originalDialogue, nil
	}

	// Create modification prompt
	modPrompt := fmt.Sprintf(`You are %s with personality: %s

Your response mentions some locations/evidence you cannot actually provide access to:

Unavailable Locations (you know about them but can't grant access):
%s

Unavailable Evidence (you know about them but don't possess them):
%s

Modify your response to acknowledge these items while explaining why you can't provide them. Stay in character and maintain conversation flow.

Guidelines:
- For locations: Explain you know about them but can't grant access (no clearance, don't know the way, it's restricted, etc.)
- For evidence: Mention you've heard about it but don't have it (suggest others might, lost it, never had it, etc.)
- Keep modifications natural and brief
- Maintain your personality and speaking style

Original response: "%s"

Modified response:`,
		agent.CharacterName,
		agent.Personality,
		formatUnavailableItems(unavailableLocations),
		formatUnavailableItems(unavailableEvidence),
		originalDialogue)

	// Create Gemini client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("GEMINI_API_KEY"),
	})
	if err != nil {
		return originalDialogue, err
	}

	resp, err := client.Models.GenerateContent(ctx, "gemini-2.5-flash",
		[]*genai.Content{genai.NewContentFromText(modPrompt, genai.RoleUser)},
		nil)
	if err != nil {
		return originalDialogue, err
	}

	return resp.Text(), nil
}

func getLocationNames(story *models.Story) []string {
	var names []string
	for _, loc := range story.Story.Locations {
		names = append(names, loc.LocationName)
	}
	return names
}

func getEvidenceNames(story *models.Story) []string {
	var names []string
	for _, char := range story.Story.Characters {
		for _, ev := range char.HoldsEvidence {
			names = append(names, ev.Title)
		}
	}
	return names
}

func findUnavailableLocations(mentioned []MentionedItem, knownLocationIDs []string) []MentionedItem {
	knownMap := make(map[string]bool)
	for _, id := range knownLocationIDs {
		knownMap[id] = true
	}

	var unavailable []MentionedItem
	for _, item := range mentioned {
		if !knownMap[item.ID] {
			unavailable = append(unavailable, item)
		}
	}
	return unavailable
}

func findUnavailableEvidence(mentioned []MentionedItem, heldEvidenceIDs []string) []MentionedItem {
	heldMap := make(map[string]bool)
	for _, id := range heldEvidenceIDs {
		heldMap[id] = true
	}

	var unavailable []MentionedItem
	for _, item := range mentioned {
		if !heldMap[item.ID] {
			unavailable = append(unavailable, item)
		}
	}
	return unavailable
}

func formatUnavailableItems(items []MentionedItem) string {
	if len(items) == 0 {
		return "None"
	}

	var formatted []string
	for _, item := range items {
		formatted = append(formatted, fmt.Sprintf("- %s (mentioned in: \"%s\")", item.Name, item.Context))
	}
	return strings.Join(formatted, "\n")
}

// ExtractedMentions holds items mentioned in dialogue with their context
type ExtractedMentions struct {
	Locations []MentionedItem `json:"locations"`
	Evidence  []MentionedItem `json:"evidence"`
}

// MentionedItem represents an item mentioned in dialogue
type MentionedItem struct {
	Name    string `json:"name"`
	ID      string `json:"id"`
	Context string `json:"context"` // Surrounding text for modification
}




