package handlers

import (
	"agent/db"
	"agent/models"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// fetchEvidenceDetails retrieves the full evidence documents for the requested IDs
func fetchEvidenceDetails(storyID string, evidenceIDs []string) ([]models.Evidence, error) {
	storyObjID, err := primitive.ObjectIDFromHex(storyID)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var story models.Story
	collection := db.GetCollection("stories")
	if err := collection.FindOne(ctx, bson.M{"_id": storyObjID}).Decode(&story); err != nil {
		return nil, err
	}

	evidenceMap := make(map[string]bool, len(evidenceIDs))
	for _, id := range evidenceIDs {
		evidenceMap[id] = true
	}

	var evidenceDetails []models.Evidence
	for _, character := range story.Story.Characters {
		for _, evidence := range character.HoldsEvidence {
			if evidenceMap[evidence.ID] {
				evidenceDetails = append(evidenceDetails, evidence)
			}
		}
	}

	// Also search evidence inside location containers
	for _, location := range story.Story.Locations {
		for _, container := range location.Containers {
			for _, evidence := range container.ContainsEvidence {
				if evidenceMap[evidence.ID] {
					evidenceDetails = append(evidenceDetails, evidence)
				}
			}
		}
	}

	return evidenceDetails, nil
}
