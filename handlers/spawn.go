package handlers

import (
	"agent/agent"
	"fmt"
	"net/http"
)

func SpawnAgentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agentID := agent.SpawnAgent("HI!")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"agent_id":"%s"}`, agentID)
}
