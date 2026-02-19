package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

// Story represents the full story document from MongoDB
type Story struct {
	ID        primitive.ObjectID `bson:"_id"`
	Story     StoryContent      `bson:"story"`
	RawStory  string           `bson:"raw_story"`
	CreatedAt time.Time        `bson:"created_at"`
	UpdatedAt time.Time        `bson:"updated_at"`
}

// StoryContent contains the main story content
type StoryContent struct {
	Title               string         `bson:"title"`
	NewsArticle         NewsArticle    `bson:"news_article"`
	StartingLocationIDs []string       `bson:"starting_location_ids"`
	Characters          []Character    `bson:"characters"`
	Locations           []Location     `bson:"locations"`
	FullStory          string         `bson:"full_story"`
}

// NewsArticle represents the news article within the story
type NewsArticle struct {
	Title   string `bson:"title"`
	Content string `bson:"content"`
}

// Character represents a character in the story
type Character struct {
	ID                    string         `bson:"id"`
	Name                  string         `bson:"name"`
	AppearanceDescription string         `bson:"appearance_description"`
	PersonalityProfile    string         `bson:"personality_profile"`
	KnowledgeBase        string         `bson:"knowledge_base"`
	HoldsEvidence        []Evidence     `bson:"holds_evidence"`
	KnowsLocationIDs     []string       `bson:"knows_location_ids"`
}

// Evidence represents evidence held by a character
type Evidence struct {
	ID                string `bson:"id"`
	Description       string `bson:"description"`
	VisualDescription string `bson:"visual_description"`
}

// Location represents a location in the story
type Location struct {
	ID                    string   `bson:"id"`
	LocationName          string   `bson:"location_name"`
	VisualDescription     string   `bson:"visual_description"`
	CharacterIDsInLocation []string `bson:"character_ids_in_location"`
}