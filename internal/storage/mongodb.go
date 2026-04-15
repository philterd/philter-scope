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

	// Default collection
	collName := "audits"
	if u.Query().Has("collection") {
		collName = u.Query().Get("collection")
	}

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
		{Key: "_id", Value: 1},
		{Key: "timestamp", Value: 1},
		{Key: "precision", Value: 1},
		{Key: "recall", Value: 1},
		{Key: "f1_score", Value: 1},
		{Key: "policy", Value: 1},
	}).SetSort(bson.D{{Key: "timestamp", Value: -1}})

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

	if history == nil {
		return []model.HistoryEntry{}, nil
	}

	return history, nil
}

func (s *MongoDBStorage) GetAuditResult(ctx context.Context, id string) (*model.AuditResult, error) {
	coll := s.client.Database(s.database).Collection(s.collection)

	objID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("invalid audit ID: %w", err)
	}

	var res model.AuditResult
	err = coll.FindOne(ctx, bson.D{{Key: "_id", Value: objID}}).Decode(&res)
	if err != nil {
		return nil, fmt.Errorf("failed to find audit result: %w", err)
	}

	return &res, nil
}

func (s *MongoDBStorage) DeleteAuditResult(ctx context.Context, id string) error {
	coll := s.client.Database(s.database).Collection(s.collection)

	objID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid audit ID: %w", err)
	}

	_, err = coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: objID}})
	if err != nil {
		return fmt.Errorf("failed to delete audit result: %w", err)
	}

	return nil
}

func (s *MongoDBStorage) ResolveRecommendation(ctx context.Context, auditID string, entity string) error {
	coll := s.client.Database(s.database).Collection(s.collection)

	objID, err := bson.ObjectIDFromHex(auditID)
	if err != nil {
		return fmt.Errorf("invalid audit ID: %w", err)
	}

	// Update the recommendation in the array that matches the entity
	filter := bson.D{
		{Key: "_id", Value: objID},
		{Key: "recommendations.entity", Value: entity},
	}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "recommendations.$.resolved", Value: true},
		}},
	}

	_, err = coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to resolve recommendation: %w", err)
	}

	return nil
}

func (s *MongoDBStorage) DismissRecommendation(ctx context.Context, auditID string, entity string) error {
	coll := s.client.Database(s.database).Collection(s.collection)

	objID, err := bson.ObjectIDFromHex(auditID)
	if err != nil {
		return fmt.Errorf("invalid audit ID: %w", err)
	}

	// Update the recommendation in the array that matches the entity
	filter := bson.D{
		{Key: "_id", Value: objID},
		{Key: "recommendations.entity", Value: entity},
	}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "recommendations.$.dismissed", Value: true},
		}},
	}

	_, err = coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to dismiss recommendation: %w", err)
	}

	return nil
}

func (s *MongoDBStorage) SaveAuditNotes(ctx context.Context, id string, notes string) error {
	coll := s.client.Database(s.database).Collection(s.collection)

	objID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid audit ID: %w", err)
	}

	filter := bson.D{{Key: "_id", Value: objID}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "notes", Value: notes},
		}},
	}

	_, err = coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update audit notes: %w", err)
	}

	return nil
}

// SaveRecommendations updates the recommendations for an audit.
func (s *MongoDBStorage) SaveRecommendations(ctx context.Context, id string, recs []model.Recommendation) error {
	coll := s.client.Database(s.database).Collection(s.collection)

	objID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid audit ID: %w", err)
	}

	filter := bson.D{{Key: "_id", Value: objID}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "recommendations", Value: recs},
		}},
	}

	_, err = coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update recommendations: %w", err)
	}

	return nil
}

func (s *MongoDBStorage) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}
