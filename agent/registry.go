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

func SpawnAgent() string {
	rand.Seed(time.Now().UnixNano())
	agentID := fmt.Sprintf("agent-%d", rand.Intn(1000000))

	agent := &Agent{
		ID:      agentID,
		History: []*genai.Content{},
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
