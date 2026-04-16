package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FeedbackDocument struct {
	ID                 primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	StoryID            primitive.ObjectID `bson:"story_id" json:"story_id"`
	PlayerID           string             `bson:"player_id,omitempty" json:"player_id,omitempty"`
	QuestStoryRating   int                `bson:"quest_story_rating" json:"quest_story_rating"`
	NPCDialogueRating  int                `bson:"npc_dialogue_rating" json:"npc_dialogue_rating"`
	GameplayRating     int                `bson:"gameplay_rating" json:"gameplay_rating"`
	OverallRating      int                `bson:"overall_rating" json:"overall_rating"`
	WouldPlayAgain     string             `bson:"would_play_again" json:"would_play_again"` // "yes", "maybe", "no"
	LikedText          string             `bson:"liked_text,omitempty" json:"liked_text,omitempty"`
	DislikedText       string             `bson:"disliked_text,omitempty" json:"disliked_text,omitempty"`
	CreatedAt          time.Time          `bson:"created_at" json:"created_at"`
}
