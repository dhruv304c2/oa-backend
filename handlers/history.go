package handlers

import (
	"agent/db"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type HistoryRequest struct {
	SessionID string `json:"session_id"`
	AgentID   string `json:"agent_id,omitempty"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}

type HistoryMessage struct {
	Role              string          `json:"role"`
	Content           json.RawMessage `json:"content"`
	Timestamp         time.Time       `json:"timestamp"`
	Sequence          int             `json:"sequence"`
	RevealedEvidences []string        `json:"revealed_evidences,omitempty"`
	RevealedLocations []string        `json:"revealed_locations,omitempty"`
}

type HistoryResponse struct {
	SessionID string           `json:"session_id"`
	Messages  []HistoryMessage `json:"messages"`
	Total     int64            `json:"total"`
	HasMore   bool             `json:"has_more"`
}

type chatMessageDocument struct {
	SessionID string    `bson:"session_id"`
	Role      string    `bson:"role"`
	Content   string    `bson:"content"`
	Timestamp time.Time `bson:"timestamp"`
	Sequence  int       `bson:"sequence"`
}

func HistoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req HistoryRequest

	if r.Method == http.MethodGet {
		// Parse query parameters
		req.SessionID = r.URL.Query().Get("session_id")
		// Backward compatibility for old clients
		if req.SessionID == "" {
			req.SessionID = r.URL.Query().Get("agent_id")
		}
		req.Limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
		req.Offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	} else {
		// Parse JSON body
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		if req.SessionID == "" {
			req.SessionID = req.AgentID
		}
	}

	if req.SessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 50
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	// Fetch from datastore database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := db.GetDataStoreCollection("chat_messages")
	if collection == nil {
		http.Error(w, "Datastore not initialized", http.StatusInternalServerError)
		return
	}

	filter := bson.M{"session_id": req.SessionID}
	total, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		http.Error(w, "Failed to count history", http.StatusInternalServerError)
		return
	}

	findOpts := options.Find().
		SetSort(bson.D{{Key: "sequence", Value: 1}}).
		SetLimit(int64(req.Limit)).
		SetSkip(int64(req.Offset))

	cursor, err := collection.Find(ctx, filter, findOpts)
	if err != nil {
		http.Error(w, "Failed to fetch history", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var dsMessages []chatMessageDocument
	if err := cursor.All(ctx, &dsMessages); err != nil {
		http.Error(w, "Failed to decode history", http.StatusInternalServerError)
		return
	}

	historyMessages := make([]HistoryMessage, 0, len(dsMessages))
	for _, msg := range dsMessages {
		content := normalizeContentPayload(msg.Content)
		revealedEvidences, revealedLocations := extractReveals(msg.Content)

		if content == nil {
			continue
		}

		historyMessages = append(historyMessages, HistoryMessage{
			Role:              msg.Role,
			Content:           content,
			Timestamp:         msg.Timestamp,
			Sequence:          msg.Sequence,
			RevealedEvidences: revealedEvidences,
			RevealedLocations: revealedLocations,
		})
	}

	response := HistoryResponse{
		SessionID: req.SessionID,
		Messages:  historyMessages,
		Total:     total,
		HasMore:   int64(req.Offset+req.Limit) < total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// normalizeContentPayload attempts to convert the stored string content into JSON for responses
func normalizeContentPayload(content string) json.RawMessage {
	if content == "" {
		return nil
	}

	if json.Valid([]byte(content)) {
		return json.RawMessage(content)
	}

	return json.RawMessage([]byte(`"` + content + `"`))
}

// extractReveals pulls reveal metadata from the stored content payloads (if present)
func extractReveals(content string) ([]string, []string) {
	var payload struct {
		RevealedEvidences []string `json:"revealed_evidences"`
		RevealedLocations []string `json:"revealed_locations"`
	}

	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return nil, nil
	}

	return payload.RevealedEvidences, payload.RevealedLocations
}
