package handlers

import (
	"agent/agent"
	"encoding/json"
	"net/http"
)

type SpawnRequest struct {
	SystemPrompt string `json:"system_prompt"`
}

type SpawnResponse struct {
	AgentID string `json:"agent_id"`
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

	agentID := agent.SpawnAgent(req.SystemPrompt)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(SpawnResponse{AgentID: agentID})
}
