// internal/helpers/bootstrap.go
package helpers

import (
	"context"
	"fmt"
	"os"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"

	"event-scheduler/internal/config"
	"event-scheduler/internal/database"
)

type AppComponents struct {
	Config        *config.Config
	MongoClient   *mongo.Client
	RedisClient   *redis.Client
	MongoDatabase *mongo.Database
}

// InitializeCommonComponents sets up configuration, MongoDB, Redis, etc.
func InitializeCommonComponents(serviceName string) (*AppComponents, error) {
	cfg, err := config.LoadConfig("config/config.yaml", os.Args[1:])
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	log.Info().Msgf("Starting %s service...", serviceName)

	mongoClient, err := database.NewMongoClient(cfg.Mongo.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	mongoDatabase := mongoClient.Database(cfg.Mongo.Database)

	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
	})

	return &AppComponents{
		Config:        cfg,
		MongoClient:   mongoClient,
		RedisClient:   redisClient,
		MongoDatabase: mongoDatabase,
	}, nil
}

// CloseAll cleans up resources.
func (c *AppComponents) CloseAll(ctx context.Context) {
	if err := c.MongoClient.Disconnect(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to disconnect MongoDB client")
	}
	if err := c.RedisClient.Close(); err != nil {
		log.Error().Err(err).Msg("Failed to close Redis client")
	}
}
