package dispatcher

import (
	"context"
	"strconv"
	"time"

	"github.com/cankoe/rrule-scheduler/internal/events"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"
)

// DispatchDueEvents fetches due events from the "ready_queue" and dispatches them.
func DispatchDueEvents(ctx context.Context,
	redisClient *redis.Client,
	eventsCollection, archivedEventsCollection *mongo.Collection,
) {
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
		removedCount, err := redisClient.ZRem(ctx, "ready_queue", eventID).Result()
		if err != nil {
			log.Error().Err(err).Str("event_id", eventID).Msg("Failed to remove event from ready_queue")
			events.RecordErrorStatus(ctx, eventsCollection, archivedEventsCollection, eventID,
				"Failed to remove from ready_queue: "+err.Error())
			continue
		}
		if removedCount == 0 {
			log.Warn().Str("event_id", eventID).Msg("Event already removed by another dispatcher, skipping")
			continue
		}

		// Update event status -> "worker_queue"
		if err := events.UpdateEventStatus(ctx, eventsCollection, eventID, "worker_queue", "Event dispatched to worker queue"); err != nil {
			log.Error().Err(err).Str("event_id", eventID).Msg("Failed to update event status to worker_queue")
			events.RecordErrorStatus(ctx, eventsCollection, archivedEventsCollection, eventID,
				"Failed to update status to worker_queue: "+err.Error())
			continue
		}

		// Put event in the worker_queue
		if err := redisClient.LPush(ctx, "worker_queue", eventID).Err(); err != nil {
			log.Error().Err(err).Str("event_id", eventID).Msg("Failed to push event to worker_queue")
			events.RecordErrorStatus(ctx, eventsCollection, archivedEventsCollection, eventID,
				"Failed to push to worker_queue: "+err.Error())
			continue
		}

		log.Info().Str("event_id", eventID).Msg("Dispatched event to worker_queue")
	}
}
