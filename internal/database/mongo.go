package database

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func NewMongoClient(uri string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Error().Err(err).Str("uri", uri).Msg("Failed to connect to MongoDB")
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping the database to ensure connectivity
	if err := client.Ping(ctx, nil); err != nil {
		log.Error().Err(err).Str("uri", uri).Msg("Failed to ping MongoDB")
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	log.Info().Str("uri", uri).Msg("Successfully connected and pinged MongoDB")
	return client, nil
}
