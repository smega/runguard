// mongo.go
package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func InitMongo(cfg *Config) (*mongo.Database, error) {
	clientOpts := options.Client().ApplyURI(cfg.Mongo.URI)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	db := client.Database(cfg.Mongo.Database)

	// users.email unique index
	usersColl := db.Collection("users")
	userIndexes := usersColl.Indexes()
	if _, err := userIndexes.CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "email", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetBackground(true).
			SetName("uniq_email"),
	}); err != nil {
		log.Printf("failed to ensure users.email index: %v", err)
	}

	// tokens.token unique
	tokensColl := db.Collection("tokens")
	tokenIndexes := tokensColl.Indexes()
	if _, err := tokenIndexes.CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "token", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetBackground(true).
			SetName("uniq_token"),
	}); err != nil {
		log.Printf("failed to ensure tokens.token index: %v", err)
	}
	if _, err := tokenIndexes.CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "type", Value: 1}},
		Options: options.Index().
			SetBackground(true).
			SetName("idx_type"),
	}); err != nil {
		log.Printf("failed to ensure tokens.type index: %v", err)
	}

	// jobs indexes: unique key, jobs by user
	jobsColl := db.Collection("jobs")
	jobIndexes := jobsColl.Indexes()
	if _, err := jobIndexes.CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "key", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetBackground(true).
			SetName("uniq_job_key"),
	}); err != nil {
		log.Printf("failed to ensure jobs.key index: %v", err)
	}
	if _, err := jobIndexes.CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "user_id", Value: 1}},
		Options: options.Index().
			SetBackground(true).
			SetName("idx_job_user"),
	}); err != nil {
		log.Printf("failed to ensure jobs.user_id index: %v", err)
	}

	return db, nil
}

// somewhere in your startup (e.g. internal/db/indexes.go)
func ensureWebhookLogIndexes(db *mongo.Database) error {
	coll := db.Collection("webhook_logs")

	_, err := coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "job_id", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index().SetBackground(true),
		},
	})
	return err
}
