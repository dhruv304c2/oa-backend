package handlers

import (
	"agent/models"
	"log"
	"regexp"
	"strings"
)

// LocationRevealDetector analyzes dialogue to detect location reveals
type LocationRevealDetector struct {
	locations []models.Location
}

// NewLocationRevealDetector creates a new detector with all story locations
func NewLocationRevealDetector(story *models.Story) *LocationRevealDetector {
	return &LocationRevealDetector{
		locations: story.Story.Locations,
	}
}

// DetectRevealedLocations analyzes dialogue and returns location IDs that are being revealed
func (d *LocationRevealDetector) DetectRevealedLocations(dialogue string) []string {
	revealed := []string{}
	dialogueLower := strings.ToLower(dialogue)

	// Pattern 1: Direct location mentions with revealing phrases
	revealPhrases := []string{
		"meet me at",
		"find me at",
		"i'll be at",
		"come to the",
		"go to the",
		"head to the",
		"it's at the",
		"located at",
		"you'll find it at",
		"i'll show you to",
		"i'll take you to",
		"follow me to",
		"let's go to",
		"i can get you into",
		"i have access to",
		"i know a way into",
		"the key to the",
		"the entrance to",
	}

	// Pattern 2: Action-based reveals
	actionPatterns := []string{
		`\[hands over.*key.*\]`,
		`\[gives.*access.*\]`,
		`\[shows.*map.*\]`,
		`\[draws.*map.*\]`,
		`\[writes.*address.*\]`,
		`\[points.*direction.*\]`,
		`\[unlocks.*door.*\]`,
	}

	// Check each location
	for _, location := range d.locations {
		locationNameLower := strings.ToLower(location.LocationName)

		// Check reveal phrases
		for _, phrase := range revealPhrases {
			if strings.Contains(dialogueLower, phrase) &&
				strings.Contains(dialogueLower, locationNameLower) {
				// Found a reveal phrase with location name
				if withinProximity(dialogueLower, phrase, locationNameLower, 50) {
					log.Printf("[LOCATION_DETECTOR] Found reveal: '%s' + '%s'", phrase, location.LocationName)
					revealed = append(revealed, location.ID)
					break
				}
			}
		}

		// Check action patterns
		for _, pattern := range actionPatterns {
			re := regexp.MustCompile(pattern)
			if re.MatchString(dialogueLower) && strings.Contains(dialogueLower, locationNameLower) {
				log.Printf("[LOCATION_DETECTOR] Found action reveal: pattern '%s' with '%s'", pattern, location.LocationName)
				revealed = append(revealed, location.ID)
				break
			}
		}

		// Pattern 3: Specific location-revealing dialogue
		specificPatterns := [][]string{
			{locationNameLower, "here's how to get there"},
			{locationNameLower, "i'll let you in"},
			{locationNameLower, "you have my permission"},
			{locationNameLower, "tell them i sent you"},
			{locationNameLower, "use this to get in"},
			{"password", locationNameLower},
			{"code", locationNameLower},
			{locationNameLower, "is open to you"},
			{locationNameLower, "expecting you"},
			{"arranged access", locationNameLower},
		}

		// Pattern 4: Context-aware moderate detection
		// Check if location is mentioned with future meeting intent
		meetingIndicators := []string{
			"see you", "find you", "waiting", "meet", "rendezvous", "gather",
		}

		timeIndicators := []string{
			"tonight", "tomorrow", "later", "soon", "at midnight", "at dawn",
			"in an hour", "after dark",
		}

		// If location is mentioned with both meeting and time indicators, it's likely a reveal
		locationFound := false
		for _, meeting := range meetingIndicators {
			for _, time := range timeIndicators {
				if strings.Contains(dialogueLower, meeting) &&
					strings.Contains(dialogueLower, time) &&
					strings.Contains(dialogueLower, locationNameLower) {
					log.Printf("[LOCATION_DETECTOR] Found moderate reveal: meeting+time pattern for '%s'", location.LocationName)
					revealed = append(revealed, location.ID)
					locationFound = true
					break
				}
			}
			if locationFound {
				break
			}
		}

		for _, pattern := range specificPatterns {
			allFound := true
			for _, term := range pattern {
				if !strings.Contains(dialogueLower, term) {
					allFound = false
					break
				}
			}
			if allFound {
				log.Printf("[LOCATION_DETECTOR] Found specific pattern for '%s'", location.LocationName)
				revealed = append(revealed, location.ID)
				break
			}
		}
	}

	// Remove duplicates
	return uniqueStrings(revealed)
}

// withinProximity checks if two strings appear within n characters of each other
func withinProximity(text, str1, str2 string, maxDistance int) bool {
	idx1 := strings.Index(text, str1)
	idx2 := strings.Index(text, str2)

	if idx1 == -1 || idx2 == -1 {
		return false
	}

	distance := idx2 - idx1
	if distance < 0 {
		distance = -distance
	}

	return distance <= maxDistance
}

// uniqueStrings removes duplicates from string slice
func uniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, s := range input {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}