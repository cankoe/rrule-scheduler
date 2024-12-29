package main

import (
	"context"
	"strconv"
	"time"

	"event-scheduler/internal/helpers"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"
)

func main() {
	components, err := helpers.InitializeCommonComponents("dispatcher")
	if err != nil {
		log.Fatal().Err(err)
	}
	defer components.CloseAll(context.Background())

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	eventsCollection := components.MongoDatabase.Collection("events")
	archivedEventsCollection := components.MongoDatabase.Collection("archived_events")

	log.Info().Msg("Dispatcher started.")
	for range ticker.C {
		dispatchDueEvents(components.RedisClient, eventsCollection, archivedEventsCollection)
	}
}

func dispatchDueEvents(redisClient *redis.Client, eventsCollection, archivedEventsCollection *mongo.Collection) {
	ctx := context.Background()
	now := time.Now().UTC().Unix()

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
			recordErrorStatus(ctx, eventsCollection, archivedEventsCollection, eventID, "Failed to remove from ready_queue: "+err.Error())
			continue
		}

		// If removedCount is 0, it means another dispatcher already processed this event
		if removedCount == 0 {
			log.Warn().Str("event_id", eventID).Msg("Event already removed by another dispatcher, skipping")
			continue
		}

		// Update event status in mongodb
		if err := helpers.UpdateEventStatus(ctx, eventsCollection, eventID, "worker_queue", "Event dispatched to worker queue"); err != nil {
			log.Error().Err(err).Str("event_id", eventID).Msg("Failed to update event status to worker_queue")
			recordErrorStatus(ctx, eventsCollection, archivedEventsCollection, eventID, "Failed to update status to worker_queue: "+err.Error())
			continue
		}

		// Push the event to the worker_queue
		if err := redisClient.LPush(ctx, "worker_queue", eventID).Err(); err != nil {
			log.Error().Err(err).Str("event_id", eventID).Msg("Failed to push event to worker_queue")
			recordErrorStatus(ctx, eventsCollection, archivedEventsCollection, eventID, "Failed to push to worker_queue: "+err.Error())
			continue
		}

		log.Info().Str("event_id", eventID).Msg("Dispatched event to worker_queue")
	}
}

func recordErrorStatus(ctx context.Context, eventsCollection, archivedEventsCollection *mongo.Collection, eventID, errorMsg string) {
	err := helpers.UpdateAndArchiveEvent(ctx, eventsCollection, archivedEventsCollection, eventID, "error", errorMsg)
	if err != nil {
		log.Error().Err(err).Str("event_id", eventID).Msg("Failed to record error status")
	}
}
