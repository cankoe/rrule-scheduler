package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"event-scheduler/internal/helpers"
	"event-scheduler/internal/models"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func main() {
	components, err := helpers.InitializeCommonComponents("dispatcher")
	if err != nil {
		log.Fatal().Err(err)
	}
	defer components.CloseAll(context.Background())

	eventsCollection := components.MongoDatabase.Collection("events")
	archivedEventsCollection := components.MongoDatabase.Collection("archived_events")
	schedulesCollection := components.MongoDatabase.Collection("schedules")

	// Ensure MongoDB Indexes
	if err := ensureIndexes(eventsCollection, schedulesCollection); err != nil {
		log.Fatal().Err(err).Msg("Failed to create necessary indexes")
	}

	workerCount := 5
	log.Info().Int("workers", workerCount).Msg("Spawning worker goroutines")
	for i := 0; i < workerCount; i++ {
		go eventWorker(components.RedisClient, eventsCollection, archivedEventsCollection, schedulesCollection, i+1, components.Config.Worker.MaxRetries)
	}

	select {}
}

func ensureIndexes(eventsCollection, schedulesCollection *mongo.Collection) error {
	ctx := context.Background()

	// Index for events: `run_time`
	_, err := eventsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "run_time", Value: 1}},
	})
	if err != nil {
		return fmt.Errorf("failed to create index on events.run_time: %w", err)
	}

	// Index for schedules: `last_event_time`
	_, err = schedulesCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "last_event_time", Value: 1}},
	})
	if err != nil {
		return fmt.Errorf("failed to create index on schedules.last_event_time: %w", err)
	}

	log.Info().Msg("Indexes ensured successfully")
	return nil
}

func eventWorker(redisClient *redis.Client, eventsCollection, archivedEventsCollection, schedulesCollection *mongo.Collection, workerID int, maxRetries int) {
	for {
		ctx := context.Background()

		eventID, err := redisClient.RPop(ctx, "worker_queue").Result()
		if err != nil {
			if err == redis.Nil {
				log.Warn().Int("worker_id", workerID).Msg("No events in queue, retrying...")
				time.Sleep(1 * time.Second)
			} else {
				log.Error().Err(err).Int("worker_id", workerID).Msg("Failed to fetch event from worker_queue")
			}
			continue
		}

		objectID, err := primitive.ObjectIDFromHex(eventID)
		if err != nil {
			log.Error().Err(err).Int("worker_id", workerID).Str("event_id", eventID).Msg("Invalid ObjectID format")
			continue
		}

		// Fetch the event document
		var event models.Event
		err = eventsCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&event)
		if err != nil {
			log.Error().Err(err).Int("worker_id", workerID).Str("event_id", eventID).Msg("Failed to retrieve event")
			recordErrorStatus(ctx, eventsCollection, archivedEventsCollection, eventID, "Failed to retrieve event: "+err.Error())
			continue
		}

		// Fetch the associated schedule document
		var schedule struct {
			ID          string `bson:"_id"`
			CallbackURL string `bson:"callback_url"`
		}
		scheduleObjectID, err := primitive.ObjectIDFromHex(event.ScheduleID)
		if err != nil {
			log.Error().Err(err).Int("worker_id", workerID).Str("schedule_id", event.ScheduleID).Msg("Invalid schedule ObjectID format")
			recordErrorStatus(ctx, eventsCollection, archivedEventsCollection, eventID, "Invalid schedule ObjectID: "+err.Error())
			continue
		}

		err = schedulesCollection.FindOne(ctx, bson.M{"_id": scheduleObjectID}).Decode(&schedule)
		if err != nil {
			log.Error().Err(err).Int("worker_id", workerID).Str("event_id", eventID).Msg("Failed to retrieve schedule")
			recordErrorStatus(ctx, eventsCollection, archivedEventsCollection, eventID, "Failed to retrieve schedule: "+err.Error())
			continue
		}

		// Try calling the callback URL up to 3 times
		var finalErr error
		for attempt := 1; attempt <= maxRetries; attempt++ {
			resp, callErr := http.Get(schedule.CallbackURL)
			if callErr == nil {
				resp.Body.Close()
				// Mark the event as completed
				log.Info().Int("worker_id", workerID).Str("event_id", eventID).Msg("Marking event as completed")
				err = helpers.UpdateAndArchiveEvent(ctx, eventsCollection, archivedEventsCollection, eventID, "completed", "Event successfully processed")
				if err != nil {
					log.Error().Err(err).Int("worker_id", workerID).Str("event_id", eventID).Msg("Failed to mark event as completed")
					recordErrorStatus(ctx, eventsCollection, archivedEventsCollection, eventID, "Failed to update status to completed: "+err.Error())
				}
				finalErr = nil
				break
			} else {
				finalErr = callErr
				if attempt < maxRetries {
					time.Sleep(1 * time.Second) // optional backoff before retry
					continue
				}
			}
		}

		if finalErr != nil {
			log.Error().Err(finalErr).Int("worker_id", workerID).Str("event_id", eventID).Msg("Failed to call callback URL after max retries")
			recordErrorStatus(ctx, eventsCollection, archivedEventsCollection, eventID, "Callback failed after max retries: "+finalErr.Error())
		}
	}
}

func recordErrorStatus(ctx context.Context, eventsCollection, archivedEventsCollection *mongo.Collection, eventID, errorMsg string) {
	err := helpers.UpdateAndArchiveEvent(ctx, eventsCollection, archivedEventsCollection, eventID, "error", errorMsg)
	if err != nil {
		log.Error().Err(err).Str("event_id", eventID).Msg("Failed to record error status")
	}
}
