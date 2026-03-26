package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Story represents the full story document from MongoDB
type Story struct {
	ID        primitive.ObjectID `bson:"_id" json:"id"`
	Story     StoryContent       `bson:"story" json:"story"`
	RawStory  string             `bson:"raw_story" json:"raw_story"`
	Theme     string             `bson:"theme" json:"theme"`
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

// InGameCharacterVisualData represents the visual data for rendering a character in-game
type InGameCharacterVisualData struct {
	Body    string `bson:"body" json:"body"`
	Head    string `bson:"head" json:"head"`
	Ears    string `bson:"ears" json:"ears"`
	Eyes    string `bson:"eyes" json:"eyes"`
	Mouth   string `bson:"mouth" json:"mouth"`
	Hair    string `bson:"hair" json:"hair"`
	Armor   string `bson:"armor" json:"armor"`
	Helmet  string `bson:"helmet" json:"helmet"`
	Weapon  string `bson:"weapon" json:"weapon"`
	Shield  string `bson:"shield" json:"shield"`
	Cape    string `bson:"cape" json:"cape"`
	Back    string `bson:"back" json:"back"`
	Mask    string `bson:"mask" json:"mask"`
	Horns   string `bson:"horns" json:"horns"`
	Firearm string `bson:"firearm" json:"firearm"`
}

// Character represents a character in the story
type Character struct {
	ID                        string                     `bson:"id" json:"id"`
	Name                      string                     `bson:"name" json:"name"`
	Gender                    string                     `bson:"gender" json:"gender"`
	AppearanceDescription     string                     `bson:"appearance_description" json:"appearance_description"`
	InGameCharacterVisualData *InGameCharacterVisualData `bson:"in_game_character_visual_data,omitempty" json:"in_game_character_visual_data,omitempty"`
	PersonalityProfile        string                     `bson:"personality_profile" json:"personality_profile"`
	KnowledgeBase             string                     `bson:"knowledge_base" json:"knowledge_base"`
	HoldsEvidence             []Evidence                 `bson:"holds_evidence" json:"holds_evidence"`
	KnowsLocationIDs          []string                   `bson:"knows_location_ids" json:"knows_location_ids"`
	ImageURL                  string                     `bson:"image_url,omitempty" json:"image_url,omitempty"`
	BaseReputation            int                        `bson:"base_reputation" json:"base_reputation"`
	BaseIntimidation          int                        `bson:"base_intimidation" json:"base_intimidation"`
	ProvidesHints             []string                   `bson:"provides_hints" json:"provides_hints"`
}

// Evidence represents evidence held by a character
type Evidence struct {
	ID                string `bson:"id" json:"id"`
	Title             string `bson:"title" json:"title"`
	Description       string `bson:"description" json:"description"`
	VisualDescription string `bson:"visual_description" json:"visual_description"`
	ImageURL          string `bson:"image_url,omitempty" json:"image_url,omitempty"`
	IsCritical        bool   `bson:"is_critical" json:"is_critical"`
	MinReputation     int    `bson:"min_reputation" json:"min_reputation"`
	MinIntimidation   int    `bson:"min_intimidation" json:"min_intimidation"`
}

// CodeHint represents a hint for unlocking a container
type CodeHint struct {
	Type        string `bson:"type" json:"type"`
	Description string `bson:"description" json:"description"`
	Source      string `bson:"source" json:"source"`
}

// Container represents a locked container within a location
type Container struct {
	ID               string     `bson:"id" json:"id"`
	Name             string     `bson:"name" json:"name"`
	Type             string     `bson:"type" json:"type"`
	Description      string     `bson:"description" json:"description"`
	UnlockCode       string     `bson:"unlock_code" json:"unlock_code"`
	CodeHint         CodeHint   `bson:"code_hint" json:"code_hint"`
	ContainsEvidence []Evidence `bson:"contains_evidence" json:"contains_evidence"`
	IsLocked         bool       `bson:"is_locked" json:"is_locked"`
	Difficulty       string     `bson:"difficulty" json:"difficulty"`
}

// Location represents a location in the story
type Location struct {
	ID                     string      `bson:"id" json:"id"`
	LocationName           string      `bson:"location_name" json:"location_name"`
	VisualDescription      string      `bson:"visual_description" json:"visual_description"`
	CharacterIDsInLocation []string    `bson:"character_ids_in_location" json:"character_ids_in_location"`
	ImageURL               string      `bson:"image_url,omitempty" json:"image_url,omitempty"`
	Containers             []Container `bson:"containers" json:"containers"`
}
