package worker

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"rrule-scheduler/internal/events"
	"time"

	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// EnsureIndexes ensures indexes needed by the worker logic.
func EnsureIndexes(eventsCol, schedulesCol *mongo.Collection) error {
	ctx := context.Background()
	if _, err := eventsCol.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "run_time", Value: 1}},
	}); err != nil {
		return fmt.Errorf("failed to create index on events.run_time: %w", err)
	}

	if _, err := schedulesCol.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "last_event_time", Value: 1}},
	}); err != nil {
		return fmt.Errorf("failed to create index on schedules.last_event_time: %w", err)
	}

	log.Info().Msg("Indexes ensured successfully")
	return nil
}

// EventWorker continuously polls worker_queue for events and performs callbacks.
func EventWorker(ctx context.Context,
	wg *sync.WaitGroup,
	redisClient *redis.Client,
	eventsCol, archivedEventsCol, schedulesCol *mongo.Collection,
	workerID, maxRetries int,
) {
	defer wg.Done()
	httpClient := &http.Client{}

	for {
		select {
		case <-ctx.Done():
			log.Info().Int("worker_id", workerID).Msg("Worker stopped by cancellation")
			return
		default:
		}

		eventID, err := redisClient.RPop(ctx, "worker_queue").Result()
		if err != nil {
			if err == redis.Nil {
				log.Debug().Int("worker_id", workerID).Msg("No events in queue, retrying...")
				time.Sleep(1 * time.Second)
			} else {
				log.Error().Err(err).Int("worker_id", workerID).Msg("Failed to fetch event from worker_queue")
			}
			continue
		}

		// Fetch the event doc
		objectID, err := primitive.ObjectIDFromHex(eventID)
		if err != nil {
			log.Error().Err(err).Int("worker_id", workerID).Str("event_id", eventID).Msg("Invalid ObjectID format")
			events.RecordErrorStatus(ctx, eventsCol, archivedEventsCol,
				eventID, "Invalid event ObjectID: "+err.Error())
			continue
		}
		var eventDoc bson.M
		if err := eventsCol.FindOne(ctx, bson.M{"_id": objectID}).Decode(&eventDoc); err != nil {
			log.Error().Err(err).Int("worker_id", workerID).Str("event_id", eventID).Msg("Failed to retrieve event")
			events.RecordErrorStatus(ctx, eventsCol, archivedEventsCol,
				eventID, "Failed to retrieve event: "+err.Error())
			continue
		}

		// Fetch schedule doc
		scheduleID, _ := eventDoc["schedule_id"].(string)
		scheduleOID, err := primitive.ObjectIDFromHex(scheduleID)
		if err != nil {
			log.Error().Err(err).Int("worker_id", workerID).Str("schedule_id", scheduleID).Msg("Invalid schedule OID")
			events.RecordErrorStatus(ctx, eventsCol, archivedEventsCol,
				eventID, "Invalid schedule ObjectID: "+err.Error())
			continue
		}
		var scheduleDoc struct {
			ID          string            `bson:"_id"`
			CallbackURL string            `bson:"callback_url"`
			Method      string            `bson:"method"`
			Headers     map[string]string `bson:"headers"`
			Body        string            `bson:"body"`
		}
		if err := schedulesCol.FindOne(ctx, bson.M{"_id": scheduleOID}).Decode(&scheduleDoc); err != nil {
			log.Error().Err(err).Int("worker_id", workerID).Str("event_id", eventID).Msg("Failed to retrieve schedule")
			events.RecordErrorStatus(ctx, eventsCol, archivedEventsCol,
				eventID, "Failed to retrieve schedule: "+err.Error())
			continue
		}

		if scheduleDoc.Method == "" {
			scheduleDoc.Method = http.MethodGet
		}

		var reqBody *bytes.Reader
		if scheduleDoc.Body != "" {
			reqBody = bytes.NewReader([]byte(scheduleDoc.Body))
		} else {
			reqBody = bytes.NewReader(nil)
		}

		// Attempt callback with retries
		var finalErr error
		for attempt := 1; attempt <= maxRetries; attempt++ {
			if attempt > 1 {
				reqBody = bytes.NewReader([]byte(scheduleDoc.Body))
			}
			req, buildErr := http.NewRequest(scheduleDoc.Method, scheduleDoc.CallbackURL, reqBody)
			if buildErr != nil {
				finalErr = buildErr
				break
			}
			for k, v := range scheduleDoc.Headers {
				req.Header.Set(k, v)
			}

			resp, callErr := httpClient.Do(req)
			if callErr == nil {
				resp.Body.Close()
				log.Info().Int("worker_id", workerID).Str("event_id", eventID).
					Msg("Marking event as completed")
				// Mark event as completed
				err := events.UpdateAndArchiveEvent(ctx, eventsCol, archivedEventsCol,
					eventID, "completed", "Event successfully processed")
				if err != nil {
					log.Error().Err(err).Int("worker_id", workerID).Str("event_id", eventID).
						Msg("Failed to mark event as completed")
					events.RecordErrorStatus(ctx, eventsCol, archivedEventsCol,
						eventID, "Failed to update status to completed: "+err.Error())
				}
				finalErr = nil
				break
			} else {
				finalErr = callErr
				if attempt < maxRetries {
					continue
				}
			}
		}

		if finalErr != nil {
			log.Error().Err(finalErr).Int("worker_id", workerID).Str("event_id", eventID).
				Msg("Callback failed after max retries")
			events.RecordErrorStatus(ctx, eventsCol, archivedEventsCol,
				eventID, "Callback failed after max retries: "+finalErr.Error())
		}
	}
}
