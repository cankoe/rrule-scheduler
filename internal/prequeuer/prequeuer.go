package prequeuer

import (
	"context"
	"time"

	"github.com/cankoe/rrule-scheduler/internal/models"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
	"github.com/teambition/rrule-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// GenerateEvents finds schedules that have occurrences in [now, now+timeframe)
// and creates events in "events" + pushes them into "ready_queue".
func GenerateEvents(ctx context.Context,
	schedulesCollection, eventsCollection *mongo.Collection,
	redisClient *redis.Client,
	eventTimeframe time.Duration,
) {
	now := time.Now().UTC()
	endTime := now.Add(eventTimeframe)

	log.Info().Time("start", now).Time("end", endTime).Msg("Generating events for timeframe")
	cursor, err := schedulesCollection.Find(ctx, bson.M{})
	if err != nil {
		log.Error().Err(err).Msg("Error fetching schedules")
		return
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var schedule models.Schedule
		if err := cursor.Decode(&schedule); err != nil {
			log.Error().Err(err).Msg("Error decoding schedule")
			continue
		}
		rule, err := rrule.StrToRRule(schedule.RRule)
		if err != nil {
			log.Error().Err(err).Str("schedule_id", schedule.ID).Msg("Invalid RRULE")
			continue
		}
		occurrences := rule.Between(now, endTime, false)
		if len(occurrences) == 0 {
			continue
		}

		for _, occurrence := range occurrences {
			existing := eventsCollection.FindOne(ctx, bson.M{
				"schedule_id": schedule.ID,
				"run_time":    occurrence,
			})
			if existing.Err() == nil {
				log.Debug().Str("schedule_id", schedule.ID).Time("run_time", occurrence).Msg("Event already exists, skipping")
				continue
			}

			event := bson.M{
				"schedule_id": schedule.ID,
				"run_time":    occurrence,
				"status": []bson.M{{
					"time":    now,
					"status":  "ready_queue",
					"message": "Event pre-queued for ready queue",
				}},
				"created_at": now,
			}
			insertResult, err := eventsCollection.InsertOne(ctx, event)
			if err != nil {
				log.Error().Err(err).Str("schedule_id", schedule.ID).Time("occurrence", occurrence).
					Msg("Failed to insert event")
				continue
			}

			// Push into Redis ZSET with score = run_time (epoch)
			eventID := insertResult.InsertedID.(primitive.ObjectID).Hex()
			if err := redisClient.ZAdd(ctx, "ready_queue", &redis.Z{
				Score:  float64(occurrence.Unix()),
				Member: eventID,
			}).Err(); err != nil {
				log.Error().Err(err).Str("event_id", eventID).Msg("Failed to enqueue event in ready_queue")
				continue
			}

			log.Info().Str("event_id", eventID).Str("schedule_id", schedule.ID).
				Time("run_time", occurrence).Msg("Pre-queued event")
		}
	}
}
