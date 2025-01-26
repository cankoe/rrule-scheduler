package helpers

import (
	"context"
	"fmt"
	"os"

	"github.com/cankoe/rrule-scheduler/internal/config"
	"github.com/cankoe/rrule-scheduler/internal/database"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"
)

type AppComponents struct {
	Config        *config.Config
	MongoClient   *mongo.Client
	RedisClient   *redis.Client
	MongoDatabase *mongo.Database
}

func InitializeCommonComponents(serviceName string) (*AppComponents, error) {
	cfg, err := config.LoadConfig("config/config.yaml", os.Args[1:])
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Set log level
	level, err := zerolog.ParseLevel(cfg.Log.Level)
	if err != nil {
		log.Warn().Msgf("Invalid log level '%s', defaulting to info", cfg.Log.Level)
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	log.Info().Msgf("Starting %s service with log level %s...", serviceName, level.String())

	mongoClient, err := database.NewMongoClient(cfg.Mongo.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	db := mongoClient.Database(cfg.Mongo.Database)

	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
	})

	return &AppComponents{
		Config:        cfg,
		MongoClient:   mongoClient,
		RedisClient:   redisClient,
		MongoDatabase: db,
	}, nil
}

func (c *AppComponents) CloseAll(ctx context.Context) {
	if err := c.MongoClient.Disconnect(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to disconnect MongoDB client")
	}
	if err := c.RedisClient.Close(); err != nil {
		log.Error().Err(err).Msg("Failed to close Redis client")
	}
}
