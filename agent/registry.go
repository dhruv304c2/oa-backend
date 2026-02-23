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
	"agent/models"
	"agent/prompts"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/genai"
)

var (
	AgentRegistry = make(map[string]*Agent)
	mu            sync.Mutex
)

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

		// Check if this is the system prompt (first model message)
		if i == 0 && conv.Role == "model" && conv.Index == 0 {
			log.Printf("[AGENT_LOAD_REGEN] Regenerating system prompt for agent %s", agentDoc.CharacterName)

			// Fetch the story
			var story models.Story
			storyCollection := db.GetCollection("stories")
			err := storyCollection.FindOne(ctx, bson.M{"_id": agentDoc.StoryID}).Decode(&story)
			if err != nil {
				log.Printf("[AGENT_LOAD_REGEN_ERROR] Failed to fetch story: %v. Using existing prompt.", err)
				agent.History = append(agent.History, genai.NewContentFromText(conv.Content, role))
				continue
			}

			// Find the character
			var character *models.Character
			for _, char := range story.Story.Characters {
				if char.ID == agentDoc.CharacterID {
					character = &char
					break
				}
			}

			if character == nil {
				log.Printf("[AGENT_LOAD_REGEN_ERROR] Character %s not found. Using existing prompt.", agentDoc.CharacterID)
				agent.History = append(agent.History, genai.NewContentFromText(conv.Content, role))
				continue
			}

			// Generate fresh system prompt using the existing function
			systemPrompt, _ := prompts.ConstructCharacterSystemPrompt(character, &story)

			// Add story context as done during spawn
			fullSystemPrompt := fmt.Sprintf("%s\n\n[STORY CONTEXT FOR REFERENCE]:\n%s",
				systemPrompt, story.Story.FullStory)

			// Use the regenerated prompt
			agent.History = append(agent.History, genai.NewContentFromText(fullSystemPrompt, role))

			// Update in database asynchronously
			go func(agentID primitive.ObjectID, newPrompt string) {
				updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				filter := bson.M{
					"agent_id": agentID,
					"index":    0,
					"role":     "model",
				}
				update := bson.M{
					"$set": bson.M{
						"content":    newPrompt,
						"updated_at": time.Now(),
					},
				}

				conversationCollection := db.GetCollection("conversations")
				_, err := conversationCollection.UpdateOne(updateCtx, filter, update)
				if err != nil {
					log.Printf("[AGENT_LOAD_REGEN_DB] Failed to update system prompt in DB: %v", err)
				} else {
					log.Printf("[AGENT_LOAD_REGEN_DB] Successfully updated system prompt in database")
				}
			}(agentDoc.ID, fullSystemPrompt)

			log.Printf("[AGENT_LOAD_REGEN_SUCCESS] Regenerated system prompt for agent %s", agentDoc.CharacterName)
		} else {
			// Regular message, append as normal
			agent.History = append(agent.History, genai.NewContentFromText(conv.Content, role))
		}
	}

	log.Printf("[AGENT_LOAD_SUCCESS] Loaded agent %s with %d conversation messages", agentDoc.CharacterName, len(agent.History))

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
