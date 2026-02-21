package agent

import "google.golang.org/genai"

type Agent struct {
	ID                  string
	History             []*genai.Content
	StoryID             string              // Story ID for database queries
	CharacterID         string              // Character ID this agent represents
	CharacterName       string              // Character name for dialogue
	Personality         string              // Character personality for response modification
	HoldsEvidenceIDs    []string           // Evidence IDs character has
	KnowsLocationIDs    []string           // Location IDs character knows
	RevealedEvidenceIDs map[string]bool    // Track revealed evidence
	RevealedLocationIDs map[string]bool    // Track revealed locations
	LoadedFromDB        bool                // Track if agent was loaded from DB (may need format reminders)
}
