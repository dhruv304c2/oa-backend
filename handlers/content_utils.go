package handlers

import (
	"regexp"
	"strings"
)

// extractClientContent removes context tags from messages to create client-safe content
func extractClientContent(fullContent string, role string) string {
	// For model messages that are system prompts, hide them completely
	if role == "model" && isSystemPrompt(fullContent) {
		return ""
	}

	// For model messages (AI responses), return as-is
	if role == "model" {
		return fullContent
	}

	// For user messages, remove context tags
	content := fullContent

	// Remove location context tags
	// Pattern: [CURRENT LOCATION: anything until the closing bracket] and following newlines
	locationRegex := regexp.MustCompile(`\[CURRENT LOCATION:[^\]]*\]\s*`)
	content = locationRegex.ReplaceAllString(content, "")

	// Remove evidence presentation section
	// Pattern: [USER IS PRESENTING THE FOLLOWING EVIDENCE TO YOU]: and everything after it
	evidenceRegex := regexp.MustCompile(`\n*\[USER IS PRESENTING THE FOLLOWING EVIDENCE TO YOU\]:[\s\S]*$`)
	content = evidenceRegex.ReplaceAllString(content, "")

	// Clean up any extra whitespace
	content = strings.TrimSpace(content)

	return content
}

// isSystemPrompt checks if a model message is a system prompt that should be hidden
func isSystemPrompt(content string) bool {
	// Check for common system prompt indicators
	systemIndicators := []string{
		"You are",
		"Your personality is",
		"IMPORTANT: Only provide spoken dialogue",
		"Continue the conversation naturally based on your character",
		"[Note: This agent was loaded from database",
		"Stay in character and respond as your character would",
	}

	// If the message contains multiple system prompt indicators, it's likely a system prompt
	matchCount := 0
	for _, indicator := range systemIndicators {
		if strings.Contains(content, indicator) {
			matchCount++
		}
	}

	// Consider it a system prompt if it has 2 or more indicators
	return matchCount >= 2
}
