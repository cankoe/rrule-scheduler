package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"event-scheduler/internal/config"
	"event-scheduler/internal/database"
	"event-scheduler/internal/helpers"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"
)

func main() {
	cfg := config.LoadConfig("config/config.yaml")

	log.Info().Msg("Starting dispatcher service...")

	mongoClient, err := database.NewMongoClient(cfg.Mongo.URI)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to MongoDB")
	}
	defer mongoClient.Disconnect(context.Background())

	eventsCollection := mongoClient.Database(cfg.Mongo.Database).Collection("events")

	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
	})
	defer redisClient.Close()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	log.Info().Msg("Dispatcher started.")
	for range ticker.C {
		dispatchDueEvents(redisClient, eventsCollection)
	}
}

func dispatchDueEvents(redisClient *redis.Client, eventsCollection *mongo.Collection) {
	ctx := context.Background()
	now := time.Now().UTC().Add(-400 * time.Millisecond).Unix() // sub 400ms for slight trigger delay

	eventIDs, err := redisClient.ZRangeByScore(ctx, "ready_queue", &redis.ZRangeBy{
		Min: "-inf",
		Max: strconv.FormatInt(now, 10),
	}).Result()

	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch events from ready_queue")
		return
	}

	if len(eventIDs) == 0 {
		log.Debug().Msg("No due events found in ready_queue")
		return
	}

	for _, eventID := range eventIDs {
		// Attempt to remove the event from ready_queue
		removedCount, err := redisClient.ZRem(ctx, "ready_queue", eventID).Result()
		if err != nil {
			log.Error().Err(err).Str("event_id", eventID).Msg("Failed to remove event from ready_queue")
			recordErrorStatus(ctx, eventsCollection, eventID, "Failed to remove from ready_queue: "+err.Error())
			continue
		}

		// If removedCount is 0, it means another dispatcher already processed this event
		if removedCount == 0 {
			log.Warn().Str("event_id", eventID).Msg("Event already removed by another dispatcher, skipping")
			continue
		}

		// Update event status to worker_queue
		if err := helpers.UpdateEventStatus(ctx, eventsCollection, eventID, "worker_queue", "Event dispatched to worker queue"); err != nil {
			log.Error().Err(err).Str("event_id", eventID).Msg("Failed to update event status to worker_queue")
			recordErrorStatus(ctx, eventsCollection, eventID, "Failed to update status to worker_queue: "+err.Error())
			continue
		}

		// Push the event to the worker_queue
		if err := redisClient.LPush(ctx, "worker_queue", eventID).Err(); err != nil {
			log.Error().Err(err).Str("event_id", eventID).Msg("Failed to push event to worker_queue")
			recordErrorStatus(ctx, eventsCollection, eventID, "Failed to push to worker_queue: "+err.Error())
			continue
		}

		log.Info().Str("event_id", eventID).Msg("Dispatched event to worker_queue")
	}
}

func recordErrorStatus(ctx context.Context, eventsCollection *mongo.Collection, eventID, errorMsg string) {
	err := helpers.UpdateEventStatus(ctx, eventsCollection, eventID, "error", errorMsg)
	if err != nil {
		log.Error().Err(err).Str("event_id", eventID).Msg("Failed to record error status")
	}
}
