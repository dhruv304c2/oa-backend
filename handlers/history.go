package handlers

import (
	"agent/db"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

type HistoryRequest struct {
	AgentID string `json:"agent_id"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}

type HistoryMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type HistoryResponse struct {
	AgentID  string           `json:"agent_id"`
	Messages []HistoryMessage `json:"messages"`
	Total    int64            `json:"total"`
	HasMore  bool             `json:"has_more"`
}

func HistoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req HistoryRequest

	if r.Method == http.MethodGet {
		// Parse query parameters
		req.AgentID = r.URL.Query().Get("agent_id")
		req.Limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
		req.Offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	} else {
		// Parse JSON body
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
	}

	// Set defaults
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 50
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	// Fetch from database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messages, total, err := db.GetConversationHistory(ctx, req.AgentID, req.Limit, req.Offset)
	if err != nil {
		http.Error(w, "Failed to fetch history", http.StatusInternalServerError)
		return
	}

	// Convert to response format
	var historyMessages []HistoryMessage
	for _, msg := range messages {
		historyMessages = append(historyMessages, HistoryMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
		})
	}

	response := HistoryResponse{
		AgentID:  req.AgentID,
		Messages: historyMessages,
		Total:    total,
		HasMore:  int64(req.Offset+req.Limit) < total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}