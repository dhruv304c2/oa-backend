package db

import (
	"agent/db/models"
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func SaveFeedback(ctx context.Context, feedback *models.FeedbackDocument) (primitive.ObjectID, error) {
	feedback.CreatedAt = time.Now()

	collection := GetCollection("feedback")
	result, err := collection.InsertOne(ctx, feedback)
	if err != nil {
		return primitive.NilObjectID, err
	}

	return result.InsertedID.(primitive.ObjectID), nil
}

func GetFeedbackByStory(ctx context.Context, storyID primitive.ObjectID, limit, offset int) ([]models.FeedbackDocument, int64, error) {
	collection := GetCollection("feedback")

	total, err := collection.CountDocuments(ctx, bson.M{"story_id": storyID})
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSort(bson.D{{"created_at", -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(offset))

	cursor, err := collection.Find(ctx, bson.M{"story_id": storyID}, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var feedbacks []models.FeedbackDocument
	if err := cursor.All(ctx, &feedbacks); err != nil {
		return nil, 0, err
	}

	return feedbacks, total, nil
}

func CreateFeedbackIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{"story_id", 1}, {"created_at", -1}},
			Options: options.Index().SetBackground(true),
		},
	}

	collection := GetCollection("feedback")
	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		log.Printf("Failed to create feedback indexes: %v", err)
	}
}
