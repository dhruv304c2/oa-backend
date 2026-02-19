package handlers

import (
	"agent/agent"
	"context"
	"encoding/json"
	"net/http"
	"os"

	"google.golang.org/genai"
)

type MessageRequest struct {
	AgentID string `json:"agent_id"`
	Message string `json:"message"`
}

type MessageResponse struct {
	Reply string `json:"reply"`
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

	// Add user message to history
	agentObj.History = append(agentObj.History, genai.NewContentFromText(req.Message, genai.RoleUser))

	// Create Gemini client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("GEMINI_API_KEY"),
	})
	if err != nil {
		http.Error(w, "Failed to create client", http.StatusInternalServerError)
		return
	}

	resp, err := client.Models.GenerateContent(ctx, "gemini-2.5-flash", agentObj.History, nil)
	if err != nil {
		http.Error(w, "Failed to get response", http.StatusInternalServerError)
		return
	}

	reply := resp.Text()
	agentObj.History = append(agentObj.History, genai.NewContentFromText(reply, genai.RoleModel))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MessageResponse{Reply: reply})
}
