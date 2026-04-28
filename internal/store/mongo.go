package store

import (
	"context"
	"time"

	"notification-service/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoStore struct {
	db *mongo.Database
}

func NewMongoStore(db *mongo.Database) *MongoStore {
	// Create indexes
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	col := db.Collection("notifications")
	col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "type", Value: 1}}},
		{Keys: bson.D{{Key: "campaign_id", Value: 1}}},
		{Keys: bson.D{{Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "recipient", Value: 1}, {Key: "type", Value: 1}, {Key: "created_at", Value: -1}}},
	})

	return &MongoStore{db: db}
}

func (s *MongoStore) Create(ctx context.Context, n *models.Notification) error {
	_, err := s.db.Collection("notifications").InsertOne(ctx, n)
	return err
}

func (s *MongoStore) CreateMany(ctx context.Context, notifications []models.Notification) error {
	docs := make([]interface{}, len(notifications))
	for i, n := range notifications {
		docs[i] = n
	}
	_, err := s.db.Collection("notifications").InsertMany(ctx, docs)
	return err
}

func (s *MongoStore) GetByID(ctx context.Context, id string) (*models.Notification, error) {
	var n models.Notification
	err := s.db.Collection("notifications").FindOne(ctx, bson.M{"_id": id}).Decode(&n)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (s *MongoStore) UpdateStatus(ctx context.Context, id string, status models.Status, errMsg string) error {
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}
	if errMsg != "" {
		update["$set"].(bson.M)["error"] = errMsg
	}
	if status == models.StatusSent {
		now := time.Now()
		update["$set"].(bson.M)["sent_at"] = now
	}
	_, err := s.db.Collection("notifications").UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

func (s *MongoStore) IncrementRetry(ctx context.Context, id string) error {
	_, err := s.db.Collection("notifications").UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{
			"$inc": bson.M{"retry_count": 1},
			"$set": bson.M{"updated_at": time.Now()},
		},
	)
	return err
}

func (s *MongoStore) GetByCampaign(ctx context.Context, campaignID string, limit, offset int64) ([]models.Notification, int64, error) {
	col := s.db.Collection("notifications")
	filter := bson.M{"campaign_id": campaignID}

	total, err := col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSort(bson.M{"created_at": -1}).
		SetSkip(offset).
		SetLimit(limit)

	cursor, err := col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var results []models.Notification
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, err
	}
	return results, total, nil
}

// CheckDuplicate checks if a similar notification was sent recently (dedup)
func (s *MongoStore) CheckDuplicate(ctx context.Context, recipient string, nType models.NotificationType, window time.Duration) bool {
	cutoff := time.Now().Add(-window)
	count, err := s.db.Collection("notifications").CountDocuments(ctx, bson.M{
		"recipient":  recipient,
		"type":       nType,
		"status":     bson.M{"$in": []string{string(models.StatusTriggered), string(models.StatusSent)}},
		"created_at": bson.M{"$gte": cutoff},
	})
	return err == nil && count > 0
}
