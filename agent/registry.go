package agent

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

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
func SpawnAgentWithCharacter(systemPrompt, storyContext, storyID, characterID string, evidenceIDs []string, locationIDs []string) string {
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
	defer mu.Unlock()
	agent, ok := AgentRegistry[id]
	return agent, ok
}

func DeleteAgent(id string) {
	mu.Lock()
	defer mu.Unlock()
	delete(AgentRegistry, id)
}
