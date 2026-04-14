// Copyright 2026 Philterd, LLC.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/philterd/philterscope/pkg/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoDBStorage struct {
	client     *mongo.Client
	database   string
	collection string
}

func NewMongoDBStorage(ctx context.Context) (*MongoDBStorage, error) {
	connStr := os.Getenv("PHILTERSCOPE_MONGODB_CONNECTION_STRING")
	if connStr == "" {
		return nil, fmt.Errorf("PHILTERSCOPE_MONGODB_CONNECTION_STRING environment variable is not set")
	}

	u, err := url.Parse(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Extract database name from path (e.g., /dbname)
	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		dbName = "philterscope"
	}

	// Always use collection name audits
	collName := "audits"

	client, err := mongo.Connect(options.Client().ApplyURI(connStr))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	return &MongoDBStorage{
		client:     client,
		database:   dbName,
		collection: collName,
	}, nil
}

func (s *MongoDBStorage) SaveAuditResult(ctx context.Context, res model.AuditResult) error {
	coll := s.client.Database(s.database).Collection(s.collection)
	_, err := coll.InsertOne(ctx, res)
	if err != nil {
		return fmt.Errorf("failed to insert audit result: %w", err)
	}
	return nil
}

func (s *MongoDBStorage) GetHistory(ctx context.Context) ([]model.HistoryEntry, error) {
	coll := s.client.Database(s.database).Collection(s.collection)

	opts := options.Find().SetProjection(bson.D{
		{Key: "timestamp", Value: 1},
		{Key: "precision", Value: 1},
		{Key: "recall", Value: 1},
		{Key: "f1_score", Value: 1},
		{Key: "policy", Value: 1},
	})

	cursor, err := coll.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find history entries: %w", err)
	}
	defer cursor.Close(ctx)

	var history []model.HistoryEntry
	for cursor.Next(ctx) {
		var entry model.HistoryEntry
		if err := cursor.Decode(&entry); err != nil {
			return nil, fmt.Errorf("failed to decode history entry: %w", err)
		}
		history = append(history, entry)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return history, nil
}

func (s *MongoDBStorage) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}
