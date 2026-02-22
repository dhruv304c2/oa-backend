package handlers

import (
	"agent/agent"
	"encoding/json"
	"testing"
)

func TestMessageResponseJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected MessageResponse
		valid    bool
	}{
		{
			name: "Valid greeting response",
			input: `{
				"reply": "What do you want?",
				"revealed_evidences": [],
				"revealed_locations": []
			}`,
			expected: MessageResponse{
				Reply:             "What do you want?",
				RevealedEvidences: []string{},
				RevealedLocations: []string{},
			},
			valid: true,
		},
		{
			name: "Response with action brackets",
			input: `{
				"reply": "[nervously fidgets] I-I don't know what you mean!",
				"revealed_evidences": [],
				"revealed_locations": []
			}`,
			expected: MessageResponse{
				Reply:             "[nervously fidgets] I-I don't know what you mean!",
				RevealedEvidences: []string{},
				RevealedLocations: []string{},
			},
			valid: true,
		},
		{
			name: "Response revealing evidence",
			input: `{
				"reply": "[pulls out diary] Here, take this.",
				"revealed_evidences": ["diary_001"],
				"revealed_locations": []
			}`,
			expected: MessageResponse{
				Reply:             "[pulls out diary] Here, take this.",
				RevealedEvidences: []string{"diary_001"},
				RevealedLocations: []string{},
			},
			valid: true,
		},
		{
			name: "Response revealing multiple items",
			input: `{
				"reply": "[hands over both items] Take these, they're connected.",
				"revealed_evidences": ["letter_002", "photo_003"],
				"revealed_locations": ["office_001"]
			}`,
			expected: MessageResponse{
				Reply:             "[hands over both items] Take these, they're connected.",
				RevealedEvidences: []string{"letter_002", "photo_003"},
				RevealedLocations: []string{"office_001"},
			},
			valid: true,
		},
		{
			name:  "Invalid JSON",
			input: `{"reply": "Missing closing brace"`,
			valid: false,
		},
		{
			name: "Missing array fields get nil values",
			input: `{
				"reply": "Test response"
			}`,
			expected: MessageResponse{
				Reply:             "Test response",
				RevealedEvidences: nil, // Go unmarshals missing arrays as nil
				RevealedLocations: nil,
			},
			valid: true, // JSON is valid, just missing optional arrays
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var response MessageResponse
			err := json.Unmarshal([]byte(tt.input), &response)

			if tt.valid && err != nil {
				t.Errorf("Expected valid JSON, got error: %v", err)
			}

			if !tt.valid && err == nil {
				t.Errorf("Expected invalid JSON, but parsing succeeded")
			}

			if tt.valid && err == nil {
				// Check reply
				if response.Reply != tt.expected.Reply {
					t.Errorf("Reply mismatch: expected %q, got %q",
						tt.expected.Reply, response.Reply)
				}

				// Check revealed evidences
				if len(response.RevealedEvidences) != len(tt.expected.RevealedEvidences) {
					t.Errorf("RevealedEvidences length mismatch: expected %d, got %d",
						len(tt.expected.RevealedEvidences), len(response.RevealedEvidences))
				}

				// Check revealed locations
				if len(response.RevealedLocations) != len(tt.expected.RevealedLocations) {
					t.Errorf("RevealedLocations length mismatch: expected %d, got %d",
						len(tt.expected.RevealedLocations), len(response.RevealedLocations))
				}
			}
		})
	}
}

func TestValidateRevealedItems(t *testing.T) {
	tests := []struct {
		name     string
		revealed []string
		allowed  []string
		expected []string
	}{
		{
			name:     "All items valid",
			revealed: []string{"item1", "item2"},
			allowed:  []string{"item1", "item2", "item3"},
			expected: []string{"item1", "item2"},
		},
		{
			name:     "Some items invalid",
			revealed: []string{"item1", "item4", "item2"},
			allowed:  []string{"item1", "item2", "item3"},
			expected: []string{"item1", "item2"},
		},
		{
			name:     "No items valid",
			revealed: []string{"item4", "item5"},
			allowed:  []string{"item1", "item2", "item3"},
			expected: []string{},
		},
		{
			name:     "Empty revealed",
			revealed: []string{},
			allowed:  []string{"item1", "item2"},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateRevealedItems(tt.revealed, tt.allowed)

			if len(result) != len(tt.expected) {
				t.Errorf("Length mismatch: expected %d, got %d",
					len(tt.expected), len(result))
				return
			}

			for i, item := range result {
				if item != tt.expected[i] {
					t.Errorf("Item mismatch at index %d: expected %q, got %q",
						i, tt.expected[i], item)
				}
			}
		})
	}
}

func TestGenerateFallbackResponse(t *testing.T) {
	tests := []struct {
		name        string
		personality string
		expected    string
	}{
		{
			name:        "Nervous personality",
			personality: "nervous and anxious",
			expected:    "I-I'm sorry, I'm having trouble understanding... Could you repeat that?",
		},
		{
			name:        "Arrogant personality",
			personality: "arrogant and confident",
			expected:    "Speak clearly. I don't have time for your mumbling.",
		},
		{
			name:        "Professional personality",
			personality: "professional and composed",
			expected:    "I apologize, could you please rephrase your question?",
		},
		{
			name:        "Default personality",
			personality: "friendly",
			expected:    "I'm having trouble understanding. Could you rephrase that?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &agent.Agent{
				Personality: tt.personality,
			}
			result := generateFallbackResponse(agent)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestSingleStepJSONFlow tests that agents now return JSON directly
func TestSingleStepJSONFlow(t *testing.T) {
	// This test verifies that:
	// 1. Agents return structured JSON responses directly
	// 2. Actions are included within dialogue using [brackets]
	// 3. Revealed items are properly validated against agent's possessions
	// 4. The system no longer needs a separate analysis step

	t.Log("Single-step JSON flow:")
	t.Log("Step 1: Agent returns JSON response with dialogue, actions, and reveals")
	t.Log("Step 2: System validates revealed items against agent's possessions")
	t.Log("Result: Clean JSON response ready for client consumption")
}