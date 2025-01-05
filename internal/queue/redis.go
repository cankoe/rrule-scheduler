package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
)

func NewRedisClient(host string, port int) (*redis.Client, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	client := redis.NewClient(&redis.Options{
		Addr: addr,
		DB:   0,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Error().Err(err).Str("address", addr).Msg("Failed to connect to Redis")
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", addr, err)
	}

	log.Info().Str("address", addr).Msg("Successfully connected and pinged Redis")
	return client, nil
}
