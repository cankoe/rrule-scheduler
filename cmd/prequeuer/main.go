package main

import (
	"context"
	"fmt"
	"time"

	"event-scheduler/internal/config"
	"event-scheduler/internal/database"
	"event-scheduler/internal/models"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
	"github.com/teambition/rrule-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func main() {
	cfg := config.LoadConfig("config/config.yaml")

	log.Info().Msg("Starting prequeuer service...")

	mongoClient, err := database.NewMongoClient(cfg.Mongo.URI)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to MongoDB")
	}
	defer mongoClient.Disconnect(context.Background())

	schedulesCollection := mongoClient.Database(cfg.Mongo.Database).Collection("schedules")
	eventsCollection := mongoClient.Database(cfg.Mongo.Database).Collection("events")

	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
	})
	defer redisClient.Close()

	tickerInterval := time.Duration(cfg.PreQueuer.TickerIntervalSeconds) * time.Second
	eventTimeframe := time.Duration(cfg.PreQueuer.EventTimeframeMinutes) * time.Minute

	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	log.Info().Msg("Prequeuer started. Generating events...")
	for range ticker.C {
		generateEvents(schedulesCollection, eventsCollection, redisClient, eventTimeframe)
	}
}

func generateEvents(schedulesCollection, eventsCollection *mongo.Collection, redisClient *redis.Client, eventTimeframe time.Duration) {
	ctx := context.Background()
	now := time.Now().UTC()
	endTime := now.Add(eventTimeframe)

	log.Info().Time("start_time", now).Time("end_time", endTime).Msg("Generating events for timeframe")

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
			// Check if an event for this schedule and run_time already exists
			existing := eventsCollection.FindOne(ctx, bson.M{
				"schedule_id": schedule.ID,
				"run_time":    occurrence,
			})

			if existing.Err() == nil {
				// Event already exists, skip
				log.Warn().Str("schedule_id", schedule.ID).Time("run_time", occurrence).Msg("Event already exists, skipping")
				continue
			}

			// Create a new event
			event := models.Event{
				ScheduleID: schedule.ID,
				RunTime:    occurrence,
				Status: []models.StatusEntry{
					{
						Time:    now,
						Status:  "ready_queue",
						Message: "Event pre-queued for ready queue",
					},
				},
				CreatedAt: now,
			}

			insertResult, err := eventsCollection.InsertOne(ctx, event)
			if err != nil {
				log.Error().Err(err).Str("schedule_id", schedule.ID).Time("occurrence", occurrence).Msg("Failed to insert event")
				continue
			}

			eventID := insertResult.InsertedID.(primitive.ObjectID).Hex()
			err = redisClient.ZAdd(ctx, "ready_queue", &redis.Z{
				Score:  float64(occurrence.Unix()),
				Member: eventID,
			}).Err()
			if err != nil {
				log.Error().Err(err).Str("event_id", eventID).Msg("Failed to enqueue event in ready_queue")
				continue
			}

			log.Info().Str("event_id", eventID).Str("schedule_id", schedule.ID).Time("run_time", occurrence).Msg("Pre-queued event")
		}
	}
}
