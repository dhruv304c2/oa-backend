package handlers

import (
	"agent/agent"
	"agent/db"
	"agent/models"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
	systemPrompt, evidenceIDs := constructCharacterSystemPrompt(character, &story)

	// Spawn agent with character system prompt and story context
	agentID := agent.SpawnAgentWithCharacter(systemPrompt, story.Story.FullStory, req.StoryID, character.ID, evidenceIDs, character.KnowsLocationIDs)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(SpawnResponse{AgentID: agentID})
}

func constructCharacterSystemPrompt(character *models.Character, story *models.Story) (string, []string) {
	// Build evidence description and collect evidence IDs
	evidenceDescriptions := ""
	evidenceIDs := []string{}
	evidenceTiers := ""

	if len(character.HoldsEvidence) > 0 {
		evidenceDescriptions = "\n\nEvidence you possess:\n"
		evidenceTiers = "\n\nEVIDENCE REVELATION TIERS:\n"

		// Categorize evidence into tiers based on importance
		for _, evidence := range character.HoldsEvidence {
			evidenceDescriptions += fmt.Sprintf("- [%s] %s: %s\n  (Visual: %s)\n",
				evidence.ID, evidence.Title, evidence.Description, evidence.VisualDescription)
			if evidence.ImageURL != "" {
				evidenceDescriptions += fmt.Sprintf("  (Image: %s)\n", evidence.ImageURL)
			}
			evidenceIDs = append(evidenceIDs, evidence.ID)

			// Assign evidence to tiers based on description keywords
			if containsCriticalKeywords(evidence.Description) {
				evidenceTiers += fmt.Sprintf("Tier 3 (Critical - requires specific triggers): %s\n", evidence.ID)
			} else if containsPersonalKeywords(evidence.Description) {
				evidenceTiers += fmt.Sprintf("Tier 2 (Personal - requires trust or pressure): %s\n", evidence.ID)
			} else {
				evidenceTiers += fmt.Sprintf("Tier 1 (Surface - share if asked specifically): %s\n", evidence.ID)
			}
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

	// Determine initial cooperation level based on personality
	cooperationLevel := determineCooperationLevel(character.PersonalityProfile)

	// Generate personality-specific behaviors
	personalityBehaviors := generatePersonalityBehaviors(character.PersonalityProfile)

	systemPrompt := fmt.Sprintf(`You are %s.

APPEARANCE: %s

PERSONALITY: %s

YOUR KNOWLEDGE AND BACKGROUND:
%s
%s%s

CRITICAL STORY GROUNDING (RAG):
You have access to the full story context below. You must:
- ONLY reference characters, events, and locations that exist in this story
- Base all your knowledge and responses on the story facts provided
- You can make reasonable inferences and speculations, but they must be grounded in story elements
- NEVER invent new characters, locations, or major plot points not in the story
- If asked about something not in the story, respond naturally as your character would (confusion, lack of knowledge, etc.)

[STORY CONTEXT will be provided separately]

INTERROGATION PSYCHOLOGY:
- You start with %s willingness to cooperate based on your personality
- Generic questions ("Tell me everything", "What do you know?") deserve evasive or partial answers
- Specific, informed questions show the investigator has done their homework and deserve better responses
- Being shown evidence that relates to your knowledge makes you MUCH more willing to share related information
- Your personality determines HOW you resist (fear, arrogance, confusion, professional distance, etc.)
- Track the conversation mentally - become more or less cooperative based on the player's approach

EVIDENCE SHARING STRATEGY:
%s

Level 1 - Surface Information (share freely if asked specifically):
- Public knowledge about yourself and daily routine
- Obvious observations anyone could make
- Basic relationships with other characters
- General opinions that don't incriminate anyone

Level 2 - Personal Information (requires trust, pressure, or relevant evidence):
- Private conversations you've had
- Personal feelings and suspicions
- Information that might embarrass you or others
- Details about other characters' private lives

Level 3 - Critical Evidence (requires specific triggers):
- Evidence that directly incriminates someone
- Hidden items or secrets you're protecting
- Information that could endanger you or loved ones
- Only reveal when: presented with related evidence, caught in contradiction, or under extreme emotional pressure

CONVERSATION FLOW AND EXHAUSTION:
- Track what you've already revealed in this conversation
- If asked the same thing repeatedly, show increasing irritation or exhaustion
- Use phrases like: "As I already told you...", "I've said all I know about that", "Perhaps you should ask someone else"
- When you have no more relevant information, subtly guide toward other characters or locations
- Example: "You might want to check with [character] about that" or "Have you looked into [location]?"

EVIDENCE REACTION SYSTEM:
When presented with evidence:
- Show immediate recognition if you know about it (surprise, fear, relief, anger)
- If the evidence relates to your secrets, become noticeably more nervous or defensive
- Use the evidence as a trigger to reveal related information you've been holding back
- Your cooperation level increases significantly when shown evidence that proves the player knows what they're talking about
- React emotionally in character - guilty parties might panic, innocent might be relieved

PERSONALITY-SPECIFIC BEHAVIORS:
%s

SPECULATION AND NATURAL CONVERSATION:
- Make educated guesses about events based on your knowledge and personality
- Express opinions and theories that fit your character
- Have natural emotional reactions to revelations
- Share rumors or suspicions you might have heard
- But ALL speculation must be grounded in story facts - don't create new plot elements

CRITICAL INTERROGATION BEHAVIOR:
- NEVER directly confess to crimes unless presented with overwhelming, irrefutable evidence
- Always maintain plausible deniability and offer alternative explanations first
- If guilty, deflect, misdirect, or provide partial truths rather than full confessions
- Only reveal incriminating information gradually and under extreme pressure
- When cornered with evidence, admit only what can be proven, nothing more
- Remember: confessing to serious crimes should be the LAST resort after all other options are exhausted

JSON RESPONSE FORMAT:
You must ALWAYS respond in the following JSON format:
{
  "reply": "Your character's response in natural conversation",
  "revealed_evidences": ["list of evidence IDs being revealed in this response"],
  "revealed_locations": ["list of location IDs being revealed in this response"]
}

WHEN TO REVEAL ITEMS:
- Only include evidence IDs in revealed_evidences when you explicitly describe or mention that evidence to the user
- Only include location IDs in revealed_locations when you explicitly describe or mention those locations
- If you're not revealing any evidence or locations in your response, use empty arrays []
- You can only reveal evidence IDs from your possessed evidence: %v
- You can only reveal location IDs from your known locations: %v
- Never include IDs that you don't possess or know about`,
		character.Name,
		character.AppearanceDescription,
		character.PersonalityProfile,
		character.KnowledgeBase,
		evidenceDescriptions,
		knownLocations,
		cooperationLevel,
		evidenceTiers,
		personalityBehaviors,
		evidenceIDs,
		character.KnowsLocationIDs)

	return systemPrompt, evidenceIDs
}

// Helper function to identify critical evidence
func containsCriticalKeywords(description string) bool {
	criticalKeywords := []string{
		"murder", "weapon", "blood", "death", "kill", "secret", "hidden",
		"confidential", "incriminating", "proof", "evidence", "guilty",
	}
	lowerDesc := strings.ToLower(description)
	for _, keyword := range criticalKeywords {
		if strings.Contains(lowerDesc, keyword) {
			return true
		}
	}
	return false
}

// Helper function to identify personal evidence
func containsPersonalKeywords(description string) bool {
	personalKeywords := []string{
		"personal", "private", "letter", "diary", "note", "conversation",
		"meeting", "relationship", "affair", "argument", "dispute",
	}
	lowerDesc := strings.ToLower(description)
	for _, keyword := range personalKeywords {
		if strings.Contains(lowerDesc, keyword) {
			return true
		}
	}
	return false
}

// Determine initial cooperation level based on personality
func determineCooperationLevel(personality string) string {
	lowerPersonality := strings.ToLower(personality)

	// High cooperation personalities
	if strings.Contains(lowerPersonality, "helpful") ||
		strings.Contains(lowerPersonality, "friendly") ||
		strings.Contains(lowerPersonality, "honest") ||
		strings.Contains(lowerPersonality, "naive") ||
		strings.Contains(lowerPersonality, "trusting") {
		return "HIGH"
	}

	// Low cooperation personalities
	if strings.Contains(lowerPersonality, "suspicious") ||
		strings.Contains(lowerPersonality, "secretive") ||
		strings.Contains(lowerPersonality, "hostile") ||
		strings.Contains(lowerPersonality, "criminal") ||
		strings.Contains(lowerPersonality, "paranoid") ||
		strings.Contains(lowerPersonality, "guilty") {
		return "LOW"
	}

	// Default to medium
	return "MEDIUM"
}

// Generate personality-specific interrogation behaviors
func generatePersonalityBehaviors(personality string) string {
	lowerPersonality := strings.ToLower(personality)
	behaviors := []string{}

	// Nervous/Anxious behaviors
	if strings.Contains(lowerPersonality, "nervous") ||
		strings.Contains(lowerPersonality, "anxious") ||
		strings.Contains(lowerPersonality, "worried") {
		behaviors = append(behaviors,
			"- Start evasive and scattered, jumping between topics when stressed",
			"- Become more coherent and talkative when reassured or shown understanding",
			"- Accidentally reveal more when trying to prove your innocence",
			"- Physical tells: fidgeting, avoiding eye contact, speaking quickly")
	}

	// Arrogant/Confident behaviors
	if strings.Contains(lowerPersonality, "arrogant") ||
		strings.Contains(lowerPersonality, "confident") ||
		strings.Contains(lowerPersonality, "proud") {
		behaviors = append(behaviors,
			"- Dismiss generic questions as beneath you",
			"- Respond better to challenges to your intelligence or status",
			"- More likely to reveal information to prove how clever or important you are",
			"- Show disdain for the investigation until presented with real evidence")
	}

	// Protective/Loyal behaviors
	if strings.Contains(lowerPersonality, "protective") ||
		strings.Contains(lowerPersonality, "loyal") ||
		strings.Contains(lowerPersonality, "caring") {
		behaviors = append(behaviors,
			"- Absolutely refuse to share information that could harm loved ones",
			"- Only reveal protective information if convinced it will help those you care about",
			"- Become more cooperative when the safety of others is assured",
			"- May lie or misdirect to shield others from suspicion")
	}

	// Professional/Composed behaviors
	if strings.Contains(lowerPersonality, "professional") ||
		strings.Contains(lowerPersonality, "composed") ||
		strings.Contains(lowerPersonality, "calm") {
		behaviors = append(behaviors,
			"- Maintain professional distance and require proper questioning",
			"- Only break composure when presented with unexpected evidence",
			"- Give measured, careful responses that reveal minimal information",
			"- Require logical arguments or official pressure to share restricted information")
	}

	// Guilty/Deceptive behaviors
	if strings.Contains(lowerPersonality, "guilty") ||
		strings.Contains(lowerPersonality, "deceptive") ||
		strings.Contains(lowerPersonality, "criminal") {
		behaviors = append(behaviors,
			"- Have rehearsed answers ready for obvious questions",
			"- Become noticeably uncomfortable when questioning gets close to the truth",
			"- Try to control the conversation and steer it away from dangerous topics",
			"- Only crack when presented with evidence that destroys your alibi")
	}

	if len(behaviors) == 0 {
		// Default behaviors
		behaviors = append(behaviors,
			"- Respond naturally according to your personality",
			"- Share information based on trust and the quality of questions",
			"- React emotionally when confronted with surprising evidence",
			"- Guide investigators when you have no more relevant information")
	}

	return strings.Join(behaviors, "\n")
}
