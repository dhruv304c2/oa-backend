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

// analyzeAndProcessResponse analyzes a natural language response to extract reveals and modify for unavailable items
func analyzeAndProcessResponse(naturalResponse string, agent *agent.Agent, story *models.Story) (*MessageResponse, error) {
	// Fetch character's evidence and locations (reuse existing functions)
	characterEvidence, err := fetchEvidenceDetails(agent.StoryID, agent.HoldsEvidenceIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch evidence: %w", err)
	}

	characterLocations, err := fetchLocationDetailsForIDs(agent.StoryID, agent.KnowsLocationIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch locations: %w", err)
	}

	// Log character's possessions
	log.Printf("[MESSAGE_ANALYSIS_DATA] Agent %s has %d evidence items and %d locations",
		agent.CharacterName, len(characterEvidence), len(characterLocations))

	// Build analysis prompt
	analysisPrompt := fmt.Sprintf(`You are analyzing a character's dialogue response for a detective game.

CHARACTER PROFILE:
- Name: %s
- Personality: %s

EVIDENCE THIS CHARACTER POSSESSES:
%s

LOCATIONS THIS CHARACTER KNOWS:
%s

CHARACTER'S RESPONSE TO ANALYZE:
"%s"

TASKS:
1. First, scan the response for ANY specific evidence or location mentions
2. For each mentioned item, check if it exists in the character's possession lists above
3. Modify the response based on availability:
   - If character HAS the item: Keep as-is, mark as revealed if actively showing/describing
   - If character DOESN'T HAVE the item: MUST modify to be vague or explain lack of knowledge

MODIFICATION RULES FOR UNAVAILABLE ITEMS:
- Location not in list: Change specific directions to vague references
  - BAD: "Go to the Medical Bay on Deck 7, Sector Gamma"
  - GOOD: "I believe there's a medical facility somewhere on the station"
- Evidence not in list: Change to hearsay or lack of possession
  - BAD: "Here's the murder weapon, a silver knife"
  - GOOD: "I've heard about a murder weapon but I don't have access to it"

REVEAL DETECTION:
- Only mark as "revealed" when character actively shows/hands over items they ACTUALLY possess
- Casual mentions are not reveals
- Never mark unavailable items as revealed

Return JSON:
{
  "reply": "The final response (modified if needed, otherwise original) - IMPORTANT: Remove any action narration like 'I sigh' or 'I turn back' and keep only the spoken dialogue",
  "revealed_evidences": ["IDs of evidence being revealed - use the ID field, not the name - ONLY items the character possesses"],
  "revealed_locations": ["IDs of locations being revealed - use the ID field, not the name - ONLY locations the character knows"]
}

CRITICAL: If a character mentions a location/evidence not in their possession lists, you MUST modify the response to be vague or indicate lack of knowledge. The character CANNOT give specific details about things they don't know.`,
		agent.CharacterName,
		agent.Personality,
		formatCharacterEvidence(characterEvidence),
		formatCharacterLocations(characterLocations),
		naturalResponse)

	// Log the full analysis prompt
	log.Printf("[MESSAGE_ANALYSIS_PROMPT] Full analysis prompt for %s:\n%s", agent.CharacterName, analysisPrompt)

	// Create client and call Gemini
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

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
		[]*genai.Content{genai.NewContentFromText(analysisPrompt, genai.RoleUser)},
		genConfig)
	if err != nil {
		return nil, err
	}

	// Log the raw analysis response
	rawAnalysisResponse := resp.Text()
	log.Printf("[MESSAGE_ANALYSIS_RESPONSE] Raw analysis response for %s: %s", agent.CharacterName, rawAnalysisResponse)

	// Parse the analysis response
	var analysisResult MessageResponse
	if err := json.Unmarshal([]byte(rawAnalysisResponse), &analysisResult); err != nil {
		return nil, fmt.Errorf("failed to parse analysis: %w", err)
	}

	// Log if the response was modified
	if analysisResult.Reply != naturalResponse {
		log.Printf("[MESSAGE_ANALYSIS_MODIFIED] Response was modified for %s. Original length: %d, Modified length: %d",
			agent.CharacterName, len(naturalResponse), len(analysisResult.Reply))
	}

	return &analysisResult, nil
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

		if err := db.SaveConversationMessageWithVersions(ctx, agentID, fullContent, clientContent, "user", index); err != nil {
			log.Printf("Failed to persist user message: %v", err)
		}
	}(req.AgentID, userMessage, len(agentObj.History)-1)

	// Create Gemini client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("GEMINI_API_KEY"),
	})
	if err != nil {
		log.Printf("[MESSAGE_ERROR] Failed to create Gemini client for agent %s: %v", agentObj.CharacterName, err)
		http.Error(w, "Failed to create client", http.StatusInternalServerError)
		return
	}

	// Step 1: Get natural language response (no JSON format)
	// Ensure we don't have any nil entries in history
	validHistory := make([]*genai.Content, 0, len(agentObj.History))
	for i, content := range agentObj.History {
		if content != nil {
			validHistory = append(validHistory, content)
		} else {
			log.Printf("[MESSAGE_WARNING] Found nil content at index %d", i)
		}
	}

	log.Printf("[MESSAGE_DEBUG] Calling Gemini for agent %s with history length: %d",
		agentObj.CharacterName, len(validHistory))
	resp, err := client.Models.GenerateContent(ctx, "gemini-2.5-flash", validHistory, nil) // No genConfig with JSON format
	if err != nil {
		log.Printf("[MESSAGE_ERROR] Failed to get Gemini response for agent %s: %v", agentObj.CharacterName, err)
		log.Printf("[MESSAGE_DEBUG] Valid history length: %d (original: %d)", len(validHistory), len(agentObj.History))
		// Log history entries for debugging, especially around the error index
		for i := range validHistory {
			// Log more entries, especially around index 14 where the error occurred
			if i < 3 || (i >= 13 && i <= 15) {
				log.Printf("[MESSAGE_DEBUG] ValidHistory[%d]: Content exists", i)
			}
		}
		http.Error(w, "Failed to get response", http.StatusInternalServerError)
		return
	}

	// Update agentObj.History to use the validated history
	agentObj.History = validHistory

	// Get plain text response
	naturalResponse := resp.Text()
	log.Printf("[MESSAGE_NATURAL] Agent %s natural response: %s",
		agentObj.CharacterName, naturalResponse)

	// Step 2: Analyze and process the natural response
	var aiResponse *MessageResponse

	// Fetch the story for analysis
	storyObjID, err := primitive.ObjectIDFromHex(agentObj.StoryID)
	if err != nil {
		log.Printf("[MESSAGE_ERROR] Failed to parse story ID: %v", err)
		// Fallback to natural response with no reveals
		aiResponse = &MessageResponse{
			Reply:             naturalResponse,
			RevealedEvidences: []string{},
			RevealedLocations: []string{},
		}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var story models.Story
		collection := db.GetCollection("stories")
		err = collection.FindOne(ctx, bson.M{"_id": storyObjID}).Decode(&story)

		if err != nil {
			log.Printf("[MESSAGE_ERROR] Failed to fetch story: %v", err)
			// Fallback to natural response with no reveals
			aiResponse = &MessageResponse{
				Reply:             naturalResponse,
				RevealedEvidences: []string{},
				RevealedLocations: []string{},
			}
		} else {
			// Analyze the natural response
			aiResponse, err = analyzeAndProcessResponse(naturalResponse, agentObj, &story)
			if err != nil {
				log.Printf("[MESSAGE_ANALYSIS_ERROR] Failed to analyze response: %v", err)
				// Fallback: use natural response with no reveals
				aiResponse = &MessageResponse{
					Reply:             naturalResponse,
					RevealedEvidences: []string{},
					RevealedLocations: []string{},
				}
			} else {
				log.Printf("[MESSAGE_ANALYSIS_SUCCESS] Analysis complete - Reply length: %d, Revealed evidence: %d, Revealed locations: %d",
					len(aiResponse.Reply), len(aiResponse.RevealedEvidences), len(aiResponse.RevealedLocations))

				// Handle the revealed items arrays (analysis now returns IDs directly)
				originalEvidenceCount := len(aiResponse.RevealedEvidences)
				originalLocationCount := len(aiResponse.RevealedLocations)

				aiResponse.RevealedEvidences = validateRevealedItems(aiResponse.RevealedEvidences, agentObj.HoldsEvidenceIDs)
				aiResponse.RevealedLocations = validateRevealedItems(aiResponse.RevealedLocations, agentObj.KnowsLocationIDs)

				// Log if items were filtered out
				if len(aiResponse.RevealedEvidences) < originalEvidenceCount {
					log.Printf("[MESSAGE_VALIDATION] Filtered out %d invalid evidence reveals for %s",
						originalEvidenceCount - len(aiResponse.RevealedEvidences), agentObj.CharacterName)
				}
				if len(aiResponse.RevealedLocations) < originalLocationCount {
					log.Printf("[MESSAGE_VALIDATION] Filtered out %d invalid location reveals for %s",
						originalLocationCount - len(aiResponse.RevealedLocations), agentObj.CharacterName)
				}

				updateAgentTracking(agentObj, aiResponse.RevealedEvidences, aiResponse.RevealedLocations)
			}
		}
	}

	// Add the reply to history (ensure it's not empty)
	if strings.TrimSpace(aiResponse.Reply) == "" {
		log.Printf("[MESSAGE_WARNING] AI returned empty response, using default message")
		aiResponse.Reply = "I apologize, but I couldn't formulate a proper response. Could you please rephrase your question?"
	}
	agentObj.History = append(agentObj.History, genai.NewContentFromText(aiResponse.Reply, genai.RoleModel))

	// Save AI response asynchronously with both versions
	go func(agentID, processedContent, naturalContent string, index int) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// For AI responses, natural response is the full content, processed is the client content
		if err := db.SaveConversationMessageWithVersions(ctx, agentID, naturalContent, processedContent, "model", index); err != nil {
			log.Printf("Failed to persist AI response: %v", err)
		}
	}(req.AgentID, aiResponse.Reply, naturalResponse, len(agentObj.History)-1)

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

// formatCharacterEvidence formats evidence items for the verification prompt
func formatCharacterEvidence(evidence []models.Evidence) string {
	if len(evidence) == 0 {
		return "No evidence items"
	}

	var formatted []string
	for _, e := range evidence {
		formatted = append(formatted, fmt.Sprintf("- [ID: %s] %s: %s", e.ID, e.Title, e.Description))
	}
	return strings.Join(formatted, "\n")
}

// formatCharacterLocations formats locations for the verification prompt
func formatCharacterLocations(locations []models.Location) string {
	if len(locations) == 0 {
		return "No known locations"
	}

	var formatted []string
	for _, l := range locations {
		formatted = append(formatted, fmt.Sprintf("- [ID: %s] %s: %s", l.ID, l.LocationName, l.VisualDescription))
	}
	return strings.Join(formatted, "\n")
}

// verifyDialogueAgainstCharacterKnowledge verifies dialogue mentions against character's actual knowledge
func verifyDialogueAgainstCharacterKnowledge(dialogue string, agent *agent.Agent, story *models.Story) (*ExtractedMentions, error) {
	// Log verification start
	log.Printf("[VERIFY_START] Agent %s - Starting verification with %d known locations, %d held evidence",
		agent.CharacterName, len(agent.KnowsLocationIDs), len(agent.HoldsEvidenceIDs))

	// Fetch character's evidence details
	characterEvidence, err := fetchEvidenceDetails(agent.StoryID, agent.HoldsEvidenceIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch evidence details: %w", err)
	}
	log.Printf("[VERIFY_DATA] Agent %s - Fetched %d evidence items", agent.CharacterName, len(characterEvidence))

	// Fetch character's location details
	characterLocations, err := fetchLocationDetailsForIDs(agent.StoryID, agent.KnowsLocationIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch location details: %w", err)
	}
	log.Printf("[VERIFY_DATA] Agent %s - Fetched %d location details", agent.CharacterName, len(characterLocations))

	// Build verification prompt with only character's items
	verifyPrompt := fmt.Sprintf(`You are verifying dialogue consistency for a character.

CHARACTER PROFILE:
- Name: %s
- Personality: %s

EVIDENCE THIS CHARACTER POSSESSES:
%s

LOCATIONS THIS CHARACTER KNOWS:
%s

DIALOGUE TO VERIFY:
"%s"

TASK: Identify any evidence items or locations mentioned in the dialogue that are NOT in the character's possession/knowledge lists above.

Important:
- Only flag items explicitly mentioned or clearly referenced
- Consider the character's personality when interpreting ambiguous references
- Be precise about what was actually said

Return JSON format:
{
  "unavailable_evidence": [
    {"name": "exact item name mentioned", "context": "the sentence where it was mentioned"}
  ],
  "unavailable_locations": [
    {"name": "exact location name mentioned", "context": "the sentence where it was mentioned"}
  ]
}`,
		agent.CharacterName,
		agent.Personality,
		formatCharacterEvidence(characterEvidence),
		formatCharacterLocations(characterLocations),
		dialogue)

	// Create Gemini client with longer timeout
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("GEMINI_API_KEY"),
	})
	if err != nil {
		return nil, err
	}

	genConfig := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	}

	// Log prompt size for monitoring
	promptLength := len(verifyPrompt)
	log.Printf("[VERIFY_PROMPT] Agent %s - Sending verification prompt (length: %d chars)", agent.CharacterName, promptLength)

	startTime := time.Now()
	resp, err := client.Models.GenerateContent(ctx, "gemini-2.5-flash",
		[]*genai.Content{genai.NewContentFromText(verifyPrompt, genai.RoleUser)},
		genConfig)
	if err != nil {
		log.Printf("[VERIFY_API_FAIL] Agent %s - Gemini API error after %v: %v", agent.CharacterName, time.Since(startTime), err)
		return nil, err
	}

	log.Printf("[VERIFY_API_SUCCESS] Agent %s - Gemini response received in %v", agent.CharacterName, time.Since(startTime))

	// Parse the response
	var verifyResponse struct {
		UnavailableEvidence []struct {
			Name    string `json:"name"`
			Context string `json:"context"`
		} `json:"unavailable_evidence"`
		UnavailableLocations []struct {
			Name    string `json:"name"`
			Context string `json:"context"`
		} `json:"unavailable_locations"`
	}

	responseText := resp.Text()
	log.Printf("[VERIFY_RESPONSE_RAW] Agent %s - Raw response: %s", agent.CharacterName, responseText)

	err = json.Unmarshal([]byte(responseText), &verifyResponse)
	if err != nil {
		log.Printf("[VERIFY_PARSE_FAIL] Agent %s - Failed to parse JSON response: %v", agent.CharacterName, err)
		return nil, err
	}

	log.Printf("[VERIFY_PARSE_SUCCESS] Agent %s - Found %d unavailable evidence, %d unavailable locations",
		agent.CharacterName, len(verifyResponse.UnavailableEvidence), len(verifyResponse.UnavailableLocations))

	// Convert to ExtractedMentions format with IDs
	mentions := &ExtractedMentions{
		Locations: []MentionedItem{},
		Evidence:  []MentionedItem{},
	}

	// Build maps for name->ID lookup
	locationNameMap := buildLocationNameMap(story)
	evidenceNameMap := buildEvidenceNameMap(story)

	// Process unavailable locations
	for _, loc := range verifyResponse.UnavailableLocations {
		if id, exists := locationNameMap[strings.ToLower(strings.TrimSpace(loc.Name))]; exists {
			mentions.Locations = append(mentions.Locations, MentionedItem{
				Name:    loc.Name,
				ID:      id,
				Context: loc.Context,
			})
		}
	}

	// Process unavailable evidence
	for _, ev := range verifyResponse.UnavailableEvidence {
		if id, exists := evidenceNameMap[strings.ToLower(strings.TrimSpace(ev.Name))]; exists {
			mentions.Evidence = append(mentions.Evidence, MentionedItem{
				Name:    ev.Name,
				ID:      id,
				Context: ev.Context,
			})
		}
	}

	return mentions, nil
}

// OLD extractMentionsFromDialogue - Deprecated in favor of verifyDialogueAgainstCharacterKnowledge
// This function was causing timeouts because it sent ALL story locations and evidence to Gemini.
// The new verification approach only sends character-specific items, reducing prompt size by ~90%.

// modifyDialogueForUnavailableItems adjusts dialogue to explain unavailable items
func modifyDialogueForUnavailableItems(
	originalDialogue string,
	unavailableLocations []MentionedItem,
	unavailableEvidence []MentionedItem,
	agent *agent.Agent) (string, error) {

	if len(unavailableLocations) == 0 && len(unavailableEvidence) == 0 {
		log.Printf("[MODIFY_SKIP] Agent %s - No items to modify", agent.CharacterName)
		return originalDialogue, nil
	}

	log.Printf("[MODIFY_START] Agent %s - Modifying dialogue for %d locations, %d evidence",
		agent.CharacterName, len(unavailableLocations), len(unavailableEvidence))

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
		log.Printf("[MODIFY_CLIENT_FAIL] Agent %s - Failed to create Gemini client: %v", agent.CharacterName, err)
		return originalDialogue, err
	}

	log.Printf("[MODIFY_API_CALL] Agent %s - Calling Gemini to rewrite dialogue", agent.CharacterName)
	startTime := time.Now()

	resp, err := client.Models.GenerateContent(ctx, "gemini-2.5-flash",
		[]*genai.Content{genai.NewContentFromText(modPrompt, genai.RoleUser)},
		nil)
	if err != nil {
		log.Printf("[MODIFY_API_FAIL] Agent %s - Failed to modify dialogue after %v: %v", agent.CharacterName, time.Since(startTime), err)
		return originalDialogue, err
	}

	modifiedDialogue := resp.Text()
	log.Printf("[MODIFY_API_SUCCESS] Agent %s - Dialogue modified successfully in %v", agent.CharacterName, time.Since(startTime))
	log.Printf("[MODIFY_LENGTH] Agent %s - Original: %d chars, Modified: %d chars",
		agent.CharacterName, len(originalDialogue), len(modifiedDialogue))

	return modifiedDialogue, nil
}

// getLocationNames and getEvidenceNames removed - no longer needed with the new verification approach

// findUnavailableLocations and findUnavailableEvidence are no longer needed
// The new verifyDialogueAgainstCharacterKnowledge function directly returns unavailable items

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




