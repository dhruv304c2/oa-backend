package handlers

import (
	"agent/models"
	"reflect"
	"sort"
	"testing"
)

func TestLocationRevealDetector(t *testing.T) {
	mockStory := &models.Story{
		Story: models.StoryContent{
			Locations: []models.Location{
				{ID: "loc_1", LocationName: "Secret Lab"},
				{ID: "loc_2", LocationName: "Captain's Office"},
				{ID: "loc_3", LocationName: "Engine Room"},
				{ID: "loc_4", LocationName: "The Docks"},
			},
		},
	}

	detector := NewLocationRevealDetector(mockStory)

	tests := []struct {
		name     string
		dialogue string
		expected []string
	}{
		{
			name:     "Direct reveal with 'meet me at'",
			dialogue: "Meet me at the secret lab tonight.",
			expected: []string{"loc_1"},
		},
		{
			name:     "Direct reveal with 'find me at'",
			dialogue: "You can find me at the Captain's Office after dark.",
			expected: []string{"loc_2"},
		},
		{
			name:     "Action reveal with key",
			dialogue: "[hands over key] This will get you into the Captain's Office.",
			expected: []string{"loc_2"},
		},
		{
			name:     "Action reveal with map",
			dialogue: "[shows map] Here's how to get to the engine room.",
			expected: []string{"loc_3"},
		},
		{
			name:     "Multiple reveals",
			dialogue: "I'll take you to the engine room, then we can go to the secret lab.",
			expected: []string{"loc_1", "loc_3"},
		},
		{
			name:     "Just mentioning location - no reveal",
			dialogue: "I heard something about the engine room, but I don't know where it is.",
			expected: []string{},
		},
		{
			name:     "Access granting",
			dialogue: "I have access to the secret lab. I can get you in.",
			expected: []string{"loc_1"},
		},
		{
			name:     "Password reveal",
			dialogue: "The password for the Captain's Office is 'thunderbolt'.",
			expected: []string{"loc_2"},
		},
		{
			name:     "Permission granting",
			dialogue: "The secret lab is open to you now. Tell them I sent you.",
			expected: []string{"loc_1"},
		},
		{
			name:     "Meeting with time indicator",
			dialogue: "I'll see you at the docks tomorrow night.",
			expected: []string{"loc_4"},
		},
		{
			name:     "Vague mention - no reveal",
			dialogue: "The captain's office? Yeah, I know it exists.",
			expected: []string{},
		},
		{
			name:     "Denial - no reveal",
			dialogue: "I can't tell you where the secret lab is.",
			expected: []string{},
		},
		{
			name:     "Arranging access",
			dialogue: "I've arranged access to the engine room for you.",
			expected: []string{"loc_3"},
		},
		{
			name:     "Multiple actions",
			dialogue: "[hands over key] This opens the captain's office. [draws map] And here's how to find the secret lab.",
			expected: []string{"loc_1", "loc_2"},
		},
		{
			name:     "Case insensitive",
			dialogue: "MEET ME AT THE SECRET LAB!",
			expected: []string{"loc_1"},
		},
		{
			name:     "Location in the middle of reveal phrase",
			dialogue: "You should head to the engine room immediately.",
			expected: []string{"loc_3"},
		},
		{
			name:     "Expecting someone",
			dialogue: "They're expecting you at the captain's office.",
			expected: []string{"loc_2"},
		},
		{
			name:     "Way into location",
			dialogue: "I know a way into the secret lab that nobody else knows about.",
			expected: []string{"loc_1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectRevealedLocations(tt.dialogue)

			// Sort both slices for consistent comparison
			sort.Strings(result)
			sort.Strings(tt.expected)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("DetectRevealedLocations() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestWithinProximity(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		str1        string
		str2        string
		maxDistance int
		expected    bool
	}{
		{
			name:        "Within proximity",
			text:        "meet me at the secret lab",
			str1:        "meet me at",
			str2:        "secret lab",
			maxDistance: 20,
			expected:    true,
		},
		{
			name:        "Too far apart",
			text:        "meet me at the location which is called the secret lab",
			str1:        "meet me at",
			str2:        "secret lab",
			maxDistance: 20,
			expected:    false,
		},
		{
			name:        "Reverse order still within",
			text:        "the secret lab is where you should meet me at",
			str1:        "meet me at",
			str2:        "secret lab",
			maxDistance: 50,
			expected:    true,
		},
		{
			name:        "One string not found",
			text:        "come to the secret lab",
			str1:        "meet me at",
			str2:        "secret lab",
			maxDistance: 50,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := withinProximity(tt.text, tt.str1, tt.str2, tt.maxDistance)
			if result != tt.expected {
				t.Errorf("withinProximity() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestUniqueStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "No duplicates",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "With duplicates",
			input:    []string{"a", "b", "a", "c", "b"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "Empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "All duplicates",
			input:    []string{"a", "a", "a"},
			expected: []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := uniqueStrings(tt.input)
			// Sort for consistent comparison since order might vary
			sort.Strings(result)
			sort.Strings(tt.expected)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("uniqueStrings() = %v, want %v", result, tt.expected)
			}
		})
	}
}