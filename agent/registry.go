package agent

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"agent/db"
	dbModels "agent/db/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/genai"
)

var (
	AgentRegistry = make(map[string]*Agent)
	mu            sync.Mutex
)

func SpawnAgent(systemPrompt string) string {
	rand.Seed(time.Now().UnixNano())
	agentID := fmt.Sprintf("agent-%d", rand.Intn(1000000))

	agent := &Agent{
		ID:      agentID,
		History: []*genai.Content{
			genai.NewContentFromText(systemPrompt, genai.RoleUser),
		},
	}

	mu.Lock()
	AgentRegistry[agentID] = agent
	mu.Unlock()

	return agentID
}

// SpawnAgentWithCharacter creates a new agent with character-specific system prompt
func SpawnAgentWithCharacter(systemPrompt, storyContext, storyID, characterID, characterName, personality string, evidenceIDs []string, locationIDs []string) string {
	rand.Seed(time.Now().UnixNano())
	agentID := fmt.Sprintf("agent-%d", rand.Intn(1000000))

	// Combine system prompt and story context into one comprehensive system prompt
	fullSystemPrompt := fmt.Sprintf("%s\n\n[STORY CONTEXT FOR REFERENCE]:\n%s", systemPrompt, storyContext)

	// Create system content as the initial state
	systemContent := genai.NewContentFromText(fullSystemPrompt, genai.RoleModel)

	agent := &Agent{
		ID:                  agentID,
		History:             []*genai.Content{systemContent},
		StoryID:             storyID,
		CharacterID:         characterID,
		CharacterName:       characterName,
		Personality:         personality,
		HoldsEvidenceIDs:    evidenceIDs,
		KnowsLocationIDs:    locationIDs,
		RevealedEvidenceIDs: make(map[string]bool),
		RevealedLocationIDs: make(map[string]bool),
	}

	mu.Lock()
	AgentRegistry[agentID] = agent
	mu.Unlock()

	return agentID
}

func GetAgentByID(id string) (*Agent, bool) {
	mu.Lock()
	agent, ok := AgentRegistry[id]
	mu.Unlock()

	// If agent is in memory, return it
	if ok {
		log.Printf("[AGENT_GET] Agent %s found in memory", id)
		return agent, true
	}

	// Agent not in memory, try to load from database
	log.Printf("[AGENT_GET] Agent %s not in memory, loading from database", id)
	loadedAgent, err := LoadAgentFromDatabase(id)
	if err != nil {
		log.Printf("[AGENT_GET_ERROR] Failed to load agent %s from database: %v", id, err)
		return nil, false
	}

	// Add to registry for future requests
	mu.Lock()
	AgentRegistry[id] = loadedAgent
	mu.Unlock()

	return loadedAgent, true
}

// SpawnAgentWithCharacterAndID creates a new agent with a specific ID and character-specific system prompt
func SpawnAgentWithCharacterAndID(agentID, systemPrompt, storyContext, storyID, characterID, characterName, personality string, evidenceIDs []string, locationIDs []string) {
	// Combine system prompt and story context into one comprehensive system prompt
	fullSystemPrompt := fmt.Sprintf("%s\n\n[STORY CONTEXT FOR REFERENCE]:\n%s", systemPrompt, storyContext)

	// Create system content as the initial state
	systemContent := genai.NewContentFromText(fullSystemPrompt, genai.RoleModel)

	agent := &Agent{
		ID:                  agentID,
		History:             []*genai.Content{systemContent},
		StoryID:             storyID,
		CharacterID:         characterID,
		CharacterName:       characterName,
		Personality:         personality,
		HoldsEvidenceIDs:    evidenceIDs,
		KnowsLocationIDs:    locationIDs,
		RevealedEvidenceIDs: make(map[string]bool),
		RevealedLocationIDs: make(map[string]bool),
	}

	mu.Lock()
	AgentRegistry[agentID] = agent
	mu.Unlock()
}

func DeleteAgent(id string) {
	mu.Lock()
	defer mu.Unlock()
	delete(AgentRegistry, id)
}

// LoadAgentFromDatabase loads an agent and its conversation history from the database
func LoadAgentFromDatabase(agentID string) (*Agent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Convert string ID to ObjectID
	objID, err := primitive.ObjectIDFromHex(agentID)
	if err != nil {
		log.Printf("[AGENT_LOAD_ERROR] Invalid agent ID format: %s", agentID)
		return nil, err
	}

	// Fetch agent document
	var agentDoc dbModels.AgentDocument
	collection := db.GetCollection("agents")
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&agentDoc)
	if err != nil {
		log.Printf("[AGENT_LOAD_ERROR] Failed to find agent %s in database: %v", agentID, err)
		return nil, err
	}

	log.Printf("[AGENT_LOAD] Loading agent %s (%s) from database", agentDoc.CharacterName, agentID)

	// Initialize the agent with basic info
	agent := &Agent{
		ID:                  agentID,
		History:             []*genai.Content{},
		StoryID:             agentDoc.StoryID.Hex(),
		CharacterID:         agentDoc.CharacterID,
		CharacterName:       agentDoc.CharacterName,
		Personality:         agentDoc.Personality,
		HoldsEvidenceIDs:    agentDoc.HoldsEvidenceIDs,
		KnowsLocationIDs:    agentDoc.KnowsLocationIDs,
		RevealedEvidenceIDs: agentDoc.RevealedEvidenceIDs,
		RevealedLocationIDs: agentDoc.RevealedLocationIDs,
		LoadedFromDB:        true, // Mark as loaded from DB
	}

	// Initialize maps if nil
	if agent.RevealedEvidenceIDs == nil {
		agent.RevealedEvidenceIDs = make(map[string]bool)
	}
	if agent.RevealedLocationIDs == nil {
		agent.RevealedLocationIDs = make(map[string]bool)
	}

	// Load conversation history
	conversationCollection := db.GetCollection("conversations")

	// Set find options to sort by index
	findOptions := options.Find().SetSort(bson.D{{"index", 1}})

	cursor, err := conversationCollection.Find(ctx,
		bson.M{"agent_id": objID},
		findOptions,
	)
	if err != nil {
		log.Printf("[AGENT_LOAD_WARNING] Failed to load conversation history for agent %s: %v", agentID, err)
		// Continue without history - agent can still function
		return agent, nil
	}
	defer cursor.Close(ctx)

	// Reconstruct conversation history
	var conversations []dbModels.ConversationDocument
	if err := cursor.All(ctx, &conversations); err != nil {
		log.Printf("[AGENT_LOAD_WARNING] Failed to decode conversation history for agent %s: %v", agentID, err)
		return agent, nil
	}

	// Convert conversation documents to genai.Content
	for i, conv := range conversations {
		// Skip empty content messages - Gemini doesn't accept them
		if strings.TrimSpace(conv.Content) == "" {
			log.Printf("[AGENT_LOAD_HISTORY_SKIP] Skipping empty message %d: Role=%s, Index=%d", i, conv.Role, conv.Index)
			continue
		}

		var role genai.Role
		if conv.Role == "user" {
			role = genai.RoleUser
		} else {
			role = genai.RoleModel
		}

		// Log for debugging
		contentLength := len(conv.Content)
		log.Printf("[AGENT_LOAD_HISTORY] Message %d: Role=%s, Index=%d, Length=%d", i, conv.Role, conv.Index, contentLength)

		agent.History = append(agent.History, genai.NewContentFromText(conv.Content, role))
	}

	log.Printf("[AGENT_LOAD_SUCCESS] Loaded agent %s with %d conversation messages", agentDoc.CharacterName, len(agent.History))

	// Check if the first message is a system/model message
	// Gemini expects conversations to start with either user or model, but having a proper context is important
	hasSystemMessage := false
	if len(conversations) > 0 && len(agent.History) > 0 {
		// Check if first message looks like a system prompt (usually longer and from model)
		firstMsg := conversations[0]
		if firstMsg.Role == "model" && firstMsg.Index == 0 {
			hasSystemMessage = true
		}
	}

	// If no system message or empty history, add a more comprehensive one
	if !hasSystemMessage || len(agent.History) == 0 {
		// Build a system message for recovered agents
		systemMessage := fmt.Sprintf(`You are %s with personality: %s.

IMPORTANT: Only provide spoken dialogue - what your character says out loud. Do NOT include action descriptions like "I sigh" or narration. Simply speak as your character would speak.

Continue the conversation naturally based on your character. Stay in character and respond as your character would.

[Note: This agent was loaded from database after server restart. Continue conversation based on available history.]`,
			agent.CharacterName, agent.Personality)

		// Prepend system message to maintain proper conversation flow
		newHistory := []*genai.Content{genai.NewContentFromText(systemMessage, genai.RoleModel)}
		agent.History = append(newHistory, agent.History...)

		log.Printf("[AGENT_LOAD_INFO] Added/prepended system message for agent %s (had system: %v)",
			agentDoc.CharacterName, hasSystemMessage)
	}

	return agent, nil
}

// PreloadActiveAgents can be called on server startup to load recently active agents into memory
// This is optional but can improve initial response times after server restart
func PreloadActiveAgents(hoursAgo int) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Calculate cutoff time
	cutoffTime := time.Now().Add(-time.Duration(hoursAgo) * time.Hour)

	log.Printf("[AGENT_PRELOAD] Loading agents active within last %d hours", hoursAgo)

	// Find agents with recent conversations
	conversationCollection := db.GetCollection("conversations")
	pipeline := []bson.M{
		{"$match": bson.M{
			"timestamp": bson.M{"$gte": cutoffTime},
		}},
		{"$group": bson.M{
			"_id": "$agent_id",
		}},
		{"$limit": 50}, // Limit to prevent memory issues
	}

	cursor, err := conversationCollection.Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("[AGENT_PRELOAD_ERROR] Failed to find recent agents: %v", err)
		return
	}
	defer cursor.Close(ctx)

	var results []struct {
		ID primitive.ObjectID `bson:"_id"`
	}

	if err := cursor.All(ctx, &results); err != nil {
		log.Printf("[AGENT_PRELOAD_ERROR] Failed to decode agent IDs: %v", err)
		return
	}

	// Load each agent
	loaded := 0
	for _, result := range results {
		agentID := result.ID.Hex()
		if _, err := LoadAgentFromDatabase(agentID); err == nil {
			loaded++
		}
	}

	log.Printf("[AGENT_PRELOAD_SUCCESS] Preloaded %d active agents into memory", loaded)
}
