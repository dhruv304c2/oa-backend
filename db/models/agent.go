package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AgentDocument struct {
	ID                  primitive.ObjectID   `bson:"_id,omitempty"`
	StoryID             primitive.ObjectID   `bson:"story_id"`
	CharacterID         string               `bson:"character_id"`
	CharacterName       string               `bson:"character_name"`
	Personality         string               `bson:"personality"`
	HoldsEvidenceIDs    []string             `bson:"holds_evidence_ids"`
	KnowsLocationIDs    []string             `bson:"knows_location_ids"`
	RevealedEvidenceIDs map[string]bool      `bson:"revealed_evidence_ids"`
	RevealedLocationIDs map[string]bool      `bson:"revealed_location_ids"`
	CreatedAt           time.Time            `bson:"created_at"`
	UpdatedAt           time.Time            `bson:"updated_at"`
}

type ConversationDocument struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	AgentID   primitive.ObjectID `bson:"agent_id"`
	Role      string             `bson:"role"`      // "user" or "model"
	Content   string             `bson:"content"`
	Timestamp time.Time          `bson:"timestamp"`
	Index     int                `bson:"index"`     // Position in conversation
}