package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

// Story represents the full story document from MongoDB
type Story struct {
	ID        primitive.ObjectID `bson:"_id" json:"id"`
	Story     StoryContent       `bson:"story" json:"story"`
	RawStory  string             `bson:"raw_story" json:"raw_story"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

// StoryContent contains the main story content
type StoryContent struct {
	Title               string      `bson:"title" json:"title"`
	NewsArticle         NewsArticle `bson:"news_article" json:"news_article"`
	StartingLocationIDs []string    `bson:"starting_location_ids" json:"starting_location_ids"`
	Characters          []Character `bson:"characters" json:"characters"`
	Locations           []Location  `bson:"locations" json:"locations"`
	FullStory           string      `bson:"full_story" json:"full_story"`
	CoverImageURL       string      `bson:"cover_image_url,omitempty" json:"cover_image_url,omitempty"`
}

// NewsArticle represents the news article within the story
type NewsArticle struct {
	Title   string `bson:"title" json:"title"`
	Content string `bson:"content" json:"content"`
}

// Character represents a character in the story
type Character struct {
	ID                    string     `bson:"id" json:"id"`
	Name                  string     `bson:"name" json:"name"`
	AppearanceDescription string     `bson:"appearance_description" json:"appearance_description"`
	PersonalityProfile    string     `bson:"personality_profile" json:"personality_profile"`
	KnowledgeBase         string     `bson:"knowledge_base" json:"knowledge_base"`
	HoldsEvidence         []Evidence `bson:"holds_evidence" json:"holds_evidence"`
	KnowsLocationIDs      []string   `bson:"knows_location_ids" json:"knows_location_ids"`
	ImageURL              string     `bson:"image_url,omitempty" json:"image_url,omitempty"`
}

// Evidence represents evidence held by a character
type Evidence struct {
	ID                string `bson:"id" json:"id"`
	Title             string `bson:"title" json:"title"`
	Description       string `bson:"description" json:"description"`
	VisualDescription string `bson:"visual_description" json:"visual_description"`
	ImageURL          string `bson:"image_url,omitempty" json:"image_url,omitempty"`
}

// Location represents a location in the story
type Location struct {
	ID                     string   `bson:"id" json:"id"`
	LocationName           string   `bson:"location_name" json:"location_name"`
	VisualDescription      string   `bson:"visual_description" json:"visual_description"`
	CharacterIDsInLocation []string `bson:"character_ids_in_location" json:"character_ids_in_location"`
	ImageURL               string   `bson:"image_url,omitempty" json:"image_url,omitempty"`
}
