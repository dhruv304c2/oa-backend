package agent

import "google.golang.org/genai"

type Agent struct {
	ID                  string
	History             []*genai.Content
	StoryID             string              // Story ID for database queries
	CharacterID         string              // Character ID this agent represents
	HoldsEvidenceIDs    []string           // Evidence IDs character has
	KnowsLocationIDs    []string           // Location IDs character knows
	RevealedEvidenceIDs map[string]bool    // Track revealed evidence
	RevealedLocationIDs map[string]bool    // Track revealed locations
}
