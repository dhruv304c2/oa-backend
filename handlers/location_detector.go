package handlers

import (
	"agent/config"
	"agent/models"
	"context"
	"encoding/json"
	"fmt"
	"log"

	"google.golang.org/genai"
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

// DetectRevealedLocations uses LLM to analyze dialogue and returns location IDs that are being revealed
func (d *LocationRevealDetector) DetectRevealedLocations(ctx context.Context, dialogue string) []string {
	// Build location list for the prompt
	locationInfo := "Available locations and their IDs:\n"
	for _, loc := range d.locations {
		locationInfo += fmt.Sprintf("- %s (ID: %s)\n", loc.LocationName, loc.ID)
	}

	// Construct prompt for LLM
	prompt := fmt.Sprintf(`
	You are a location reveal detector. Analyze the following character dialogue and identify which locations the character is ACTIVELY REVEALING or GRANTING ACCESS TO.
	%s

	Character's dialogue:
	"%s"

	IMPORTANT: A location is considered "revealed" when:
	1. The character is giving directions or showing how to get there
	2. The character is granting access, permission, or clearance
	3. The character is providing keys, codes, or passwords
	4. The character is scheduling a meeting there
	5. The character is sending/transferring location data or coordinates

	* Or any other explicit action that helps the investigator gain access to the location. *

	Simply mentioning a location is NOT revealing it. The character must be actively helping the investigator gain access.

	Respond ONLY with a JSON array of location IDs that are being revealed. If no locations are being revealed, return an empty array.
	Example responses:
	- ["loc_1", "loc_3"]
	- []
	- ["loc_2"]`,
		locationInfo,
		dialogue,
	)

	// Initialize Gemini client
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: config.GetGeminiAPIKey(),
	})
	if err != nil {
		log.Printf("[LOCATION_DETECTOR_ERROR] Failed to create Gemini client: %v", err)
		return []string{}
	}

	// Generate response
	genConfig := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	}

	resp, err := client.Models.GenerateContent(ctx, config.GetGeminiModel(),
		[]*genai.Content{genai.NewContentFromText(prompt, genai.RoleUser)},
		genConfig)
	if err != nil {
		log.Printf("[LOCATION_DETECTOR_ERROR] Failed to generate response: %v", err)
		return []string{}
	}

	// Parse the JSON response
	var revealedLocationIDs []string
	responseText := resp.Text()
	if err := json.Unmarshal([]byte(responseText), &revealedLocationIDs); err != nil {
		log.Printf("[LOCATION_DETECTOR_ERROR] Failed to parse LLM response: %v. Response was: %s", err, responseText)
		return []string{}
	}

	log.Printf("[LOCATION_DETECTOR] LLM detected locations: %v for dialogue: %s", revealedLocationIDs, dialogue)

	// Validate that returned IDs are valid
	validIDs := make(map[string]bool)
	for _, loc := range d.locations {
		validIDs[loc.ID] = true
	}

	filtered := []string{}
	for _, id := range revealedLocationIDs {
		if validIDs[id] {
			filtered = append(filtered, id)
		} else {
			log.Printf("[LOCATION_DETECTOR_WARNING] LLM returned invalid location ID: %s", id)
		}
	}

	return filtered
}
