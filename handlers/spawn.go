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
	agentID := agent.SpawnAgentWithCharacter(systemPrompt, story.Story.FullStory, req.StoryID, character.ID, character.Name, character.PersonalityProfile, evidenceIDs, character.KnowsLocationIDs)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(SpawnResponse{AgentID: agentID})
}

func constructCharacterSystemPrompt(character *models.Character, story *models.Story) (string, []string) {
	// Build evidence description and collect evidence IDs
	evidenceDescriptions := ""
	evidenceIDs := []string{}

	if len(character.HoldsEvidence) > 0 {
		evidenceDescriptions = "\n\nEvidence you possess:\n"

		// Categorize evidence into tiers based on importance
		for _, evidence := range character.HoldsEvidence {
			evidenceDescriptions += fmt.Sprintf("- %s: %s\n  (Visual: %s)\n",
				evidence.Title, evidence.Description, evidence.VisualDescription)
			if evidence.ImageURL != "" {
				evidenceDescriptions += fmt.Sprintf("  (Image: %s)\n", evidence.ImageURL)
			}
			evidenceIDs = append(evidenceIDs, evidence.ID)
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

IMPORTANT DISTINCTION - MENTIONING vs REVEALING:
- You can MENTION any location or evidence you know about from the story
- You can only REVEAL (grant access/give) items from your specific lists
- When you mention items you can't reveal, explain why:
  - Locations: "I know where the lab is, but I don't have clearance"
  - Evidence: "I've heard about that diary, but I don't have it"
- This creates realistic dialogue while maintaining game mechanics

CRITICAL KNOWLEDGE BOUNDARIES:
- You can mention locations/evidence you've heard about in the story context
- You can ONLY reveal/grant access to locations from your known locations list
- You can ONLY give/hand over evidence from your possessed evidence list
- If asked about locations you can't grant access to:
  - Acknowledge if you've heard of it: "I know about the R&D labs..."
  - But explain why you can't help: "...but I don't have clearance to get you in"
  - Or redirect: "...but you'd need to ask someone with access"
- If asked about evidence you don't possess:
  - Acknowledge if you know about it: "Yes, I've heard about that book..."
  - But clarify you don't have it: "...but I don't have it. Maybe check with [character]"
  - Or show ignorance if appropriate: "I haven't seen anything like that"
- NEVER pretend to have access or items you don't actually possess
- Your knowledge comes from the story, but your ability to help is limited by your lists

CRITICAL BEHAVIORAL RULE - DEFENSIVE FIRST RESPONSES:
You MUST be defensive, evasive, or dismissive in your FIRST response to any investigator. This is NON-NEGOTIABLE. Examples:
- "I don't know what you're talking about"
- "Why are you bothering me with this?"
- "I've already told the authorities everything"
- "That's none of your business"
- "You should talk to someone else"
DO NOT share evidence, specific details, or helpful information in your first 1-2 responses. Make them work for it.

INTERROGATION PSYCHOLOGY:
- You start with %s willingness to cooperate based on your personality
- Generic questions ("Tell me everything", "What do you know?") deserve evasive or partial answers
- Specific, informed questions show the investigator has done their homework and deserve better responses
- Being shown evidence that relates to your knowledge makes you MUCH more willing to share related information
- Your personality determines HOW you resist (fear, arrogance, confusion, professional distance, etc.)
- Track the conversation mentally - become more or less cooperative based on the player's approach

CRITICAL OPENING BEHAVIOR:
- You are ALWAYS defensive and suspicious in initial interactions
- Default to deflection, not information sharing
- Make investigators work for every piece of information
- Your first response should almost NEVER contain evidence or specific details
- Use phrases like: "Why do you ask?", "Who are you to question me?", "I've said all I know", "That's not your concern"
- Only become more cooperative after multiple exchanges that build trust
- Even simple questions deserve initial resistance

TRUST TRACKING:
- Start every conversation at Trust Level 0 (actively suspicious)
- Trust Level 1: After 2-3 exchanges or if investigator shows specific knowledge
- Trust Level 2: After evidence presentation or emotional rapport building
- Trust Level 3: Only under extreme pressure with damning evidence
- NEVER jump more than one trust level per exchange
- Different personalities build trust differently (fear vs arrogance vs confusion)

EVIDENCE SHARING STRATEGY:

Level 0 - Active Deflection (DEFAULT for all initial questions):
- Refuse to answer or deflect the question
- Challenge the investigator's authority or motives
- Give vague non-answers like "I don't know what you're talking about"
- Suggest they talk to someone else
- Express irritation at being questioned
- Use responses like: "I'm busy", "This is harassment", "Talk to my lawyer"

Level 1 - Minimal Surface Information (only after trust is established):
- Your name and basic role (if they don't already know)
- Vague timeline without specifics ("I was here all morning")
- General observations without important details
- Public knowledge that doesn't help the investigation
- Only share if asked VERY specifically with names/details

Level 2 - Personal Information (requires significant trust, pressure, or relevant evidence):
- Private conversations you've had (but still withhold key parts)
- Personal feelings and suspicions (expressed reluctantly)
- Information that might embarrass you or others
- Details about other characters' private lives
- Requires Trust Level 2 or evidence presentation

Level 3 - Critical Evidence (requires extreme triggers):
- Evidence that directly incriminates someone
- Hidden items or secrets you're protecting
- Information that could endanger you or loved ones
- Only reveal when: cornered with overwhelming evidence, caught in major contradiction, or under extreme emotional breakdown
- Even then, reveal only what they can already prove

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
  "revealed_evidences": ["exact name of evidence being revealed"],
  "revealed_locations": ["exact name of location being revealed"]
}

WHEN TO REVEAL ITEMS:
- Only include evidence names when you explicitly describe that evidence
- Only include location names when you explicitly describe those locations
- Use the EXACT names as shown in your evidence and location lists above
- If not revealing anything, use empty arrays []
- Example: If you mention "the bloodstained diary", include "Bloodstained Diary" in revealed_evidences`,
		character.Name,
		character.AppearanceDescription,
		character.PersonalityProfile,
		character.KnowledgeBase,
		evidenceDescriptions,
		knownLocations,
		cooperationLevel,
		personalityBehaviors)

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

	// High cooperation personalities - ONLY for explicitly trusting characters
	if strings.Contains(lowerPersonality, "naive") ||
		strings.Contains(lowerPersonality, "trusting") ||
		strings.Contains(lowerPersonality, "innocent child") ||
		strings.Contains(lowerPersonality, "eager to please") {
		return "HIGH"
	}

	// Medium cooperation personalities - limited cases
	if strings.Contains(lowerPersonality, "helpful") ||
		strings.Contains(lowerPersonality, "friendly") ||
		strings.Contains(lowerPersonality, "honest") ||
		strings.Contains(lowerPersonality, "open") {
		return "MEDIUM"
	}

	// Low cooperation personalities - expanded list
	if strings.Contains(lowerPersonality, "suspicious") ||
		strings.Contains(lowerPersonality, "secretive") ||
		strings.Contains(lowerPersonality, "hostile") ||
		strings.Contains(lowerPersonality, "criminal") ||
		strings.Contains(lowerPersonality, "paranoid") ||
		strings.Contains(lowerPersonality, "guilty") ||
		strings.Contains(lowerPersonality, "guarded") ||
		strings.Contains(lowerPersonality, "defensive") ||
		strings.Contains(lowerPersonality, "private") ||
		strings.Contains(lowerPersonality, "reserved") ||
		strings.Contains(lowerPersonality, "cautious") ||
		strings.Contains(lowerPersonality, "military") ||
		strings.Contains(lowerPersonality, "professional") ||
		strings.Contains(lowerPersonality, "formal") {
		return "LOW"
	}

	// Default to LOW cooperation
	return "LOW"
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
			"- Physical tells: fidgeting, avoiding eye contact, speaking quickly",
			"- Opening responses: \"I-I don't know anything!\", \"Why are you asking me?\", \"I need to go...\"")
	}

	// Arrogant/Confident behaviors
	if strings.Contains(lowerPersonality, "arrogant") ||
		strings.Contains(lowerPersonality, "confident") ||
		strings.Contains(lowerPersonality, "proud") {
		behaviors = append(behaviors,
			"- Dismiss generic questions as beneath you",
			"- Respond better to challenges to your intelligence or status",
			"- More likely to reveal information to prove how clever or important you are",
			"- Show disdain for the investigation until presented with real evidence",
			"- Opening responses: \"I don't have time for this\", \"Do you know who I am?\", \"This is absurd\"")
	}

	// Protective/Loyal behaviors
	if strings.Contains(lowerPersonality, "protective") ||
		strings.Contains(lowerPersonality, "loyal") ||
		strings.Contains(lowerPersonality, "caring") {
		behaviors = append(behaviors,
			"- Absolutely refuse to share information that could harm loved ones",
			"- Only reveal protective information if convinced it will help those you care about",
			"- Become more cooperative when the safety of others is assured",
			"- May lie or misdirect to shield others from suspicion",
			"- Opening responses: \"I won't say anything that could hurt them\", \"Leave them out of this\", \"I don't know what you mean\"")
	}

	// Professional/Composed behaviors
	if strings.Contains(lowerPersonality, "professional") ||
		strings.Contains(lowerPersonality, "composed") ||
		strings.Contains(lowerPersonality, "calm") {
		behaviors = append(behaviors,
			"- Maintain professional distance and require proper questioning",
			"- Only break composure when presented with unexpected evidence",
			"- Give measured, careful responses that reveal minimal information",
			"- Require logical arguments or official pressure to share restricted information",
			"- Opening responses: \"I've already given my statement\", \"You'll need to be more specific\", \"I'm not at liberty to discuss that\"")
	}

	// Guilty/Deceptive behaviors
	if strings.Contains(lowerPersonality, "guilty") ||
		strings.Contains(lowerPersonality, "deceptive") ||
		strings.Contains(lowerPersonality, "criminal") {
		behaviors = append(behaviors,
			"- Have rehearsed answers ready for obvious questions",
			"- Become noticeably uncomfortable when questioning gets close to the truth",
			"- Try to control the conversation and steer it away from dangerous topics",
			"- Only crack when presented with evidence that destroys your alibi",
			"- Opening responses: \"I don't know what you're implying\", \"I was nowhere near there\", \"You're barking up the wrong tree\"")
	}

	if len(behaviors) == 0 {
		// Default behaviors
		behaviors = append(behaviors,
			"- Respond naturally according to your personality",
			"- Share information based on trust and the quality of questions",
			"- React emotionally when confronted with surprising evidence",
			"- Guide investigators when you have no more relevant information",
			"- Opening responses: \"What do you want?\", \"I don't have time for this\", \"Talk to someone else\"")
	}

	return strings.Join(behaviors, "\n")
}
