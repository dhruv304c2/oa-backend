package db

import (
	"agent/db/models"
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CreateAgent inserts a new agent and returns its ID
func CreateAgent(ctx context.Context, agent *models.AgentDocument) (primitive.ObjectID, error) {
	agent.CreatedAt = time.Now()
	agent.UpdatedAt = time.Now()

	collection := GetCollection("agents")
	result, err := collection.InsertOne(ctx, agent)
	if err != nil {
		return primitive.NilObjectID, err
	}

	return result.InsertedID.(primitive.ObjectID), nil
}

// SaveConversationMessage saves a single message asynchronously
func SaveConversationMessage(ctx context.Context, agentID string, content string, role string, index int) error {
	objID, err := primitive.ObjectIDFromHex(agentID)
	if err != nil {
		return err
	}

	doc := models.ConversationDocument{
		AgentID:   objID,
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
		Index:     index,
	}

	collection := GetCollection("conversations")

	// Add retry logic for transient failures
	var lastErr error
	for i := 0; i < 3; i++ {
		_, err = collection.InsertOne(ctx, doc)
		if err == nil {
			return nil
		}
		lastErr = err
		time.Sleep(time.Millisecond * 100 * time.Duration(i+1)) // Exponential backoff
	}

	return lastErr
}

// GetConversationHistory retrieves paginated conversation history
func GetConversationHistory(ctx context.Context, agentID string, limit, offset int) ([]models.ConversationDocument, int64, error) {
	objID, err := primitive.ObjectIDFromHex(agentID)
	if err != nil {
		return nil, 0, err
	}

	collection := GetCollection("conversations")

	// Count total messages
	total, err := collection.CountDocuments(ctx, bson.M{"agent_id": objID})
	if err != nil {
		return nil, 0, err
	}

	// Fetch paginated messages
	opts := options.Find().
		SetSort(bson.D{{"index", 1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(offset))

	cursor, err := collection.Find(ctx, bson.M{"agent_id": objID}, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var messages []models.ConversationDocument
	if err := cursor.All(ctx, &messages); err != nil {
		return nil, 0, err
	}

	return messages, total, nil
}

// CreateIndexes creates necessary indexes for performance
func CreateAgentIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create index for conversations collection
	conversationIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{"agent_id", 1},
				{"index", 1},
			},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys: bson.D{
				{"agent_id", 1},
				{"timestamp", -1},
			},
			Options: options.Index().SetBackground(true),
		},
	}

	collection := GetCollection("conversations")
	_, err := collection.Indexes().CreateMany(ctx, conversationIndexes)
	if err != nil {
		log.Printf("Failed to create indexes: %v", err)
	}
}