package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// caseInsensitive matches emails regardless of case, so accounts created before
// emails were normalized to lowercase can still be found.
var caseInsensitive = &options.Collation{Locale: "en", Strength: 2}

func connectDB(ctx context.Context, cfg Config) (*mongo.Client, *mongo.Database, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		return nil, nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, nil, err
	}
	return client, client.Database(cfg.DBName), nil
}

// ensureIndexes creates the indexes the API relies on. Failures are logged rather
// than fatal so that pre-existing duplicate data does not stop the server booting.
func ensureIndexes(ctx context.Context, db *mongo.Database) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	indexes := map[string]mongo.IndexModel{
		"users": {
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true).SetCollation(caseInsensitive),
		},
		"blogs": {
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
		"comments": {
			Keys: bson.D{{Key: "blog_id", Value: 1}, {Key: "created_at", Value: 1}},
		},
	}
	for collection, model := range indexes {
		if _, err := db.Collection(collection).Indexes().CreateOne(ctx, model); err != nil {
			log.Printf("[WARN] could not create index on %s: %v", collection, err)
		}
	}
}
