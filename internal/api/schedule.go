package api

import (
	"context"
	"event-scheduler/internal/models"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/teambition/rrule-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// RegisterRoutes registers user-level routes for schedules and events.
func RegisterRoutes(r *gin.Engine, db *mongo.Database, userAPIKey string) {
	schedulesCollection := db.Collection("schedules")
	eventsCollection := db.Collection("events")
	archivedEventsCollection := db.Collection("archived_events")

	userGroup := r.Group("/api")
	{
		userGroup.POST("/schedules", createScheduleHandler(schedulesCollection))
		userGroup.PUT("/schedules/:id", updateScheduleHandler(schedulesCollection))
		userGroup.DELETE("/schedules/:id", deleteScheduleHandler(schedulesCollection, eventsCollection))
		userGroup.GET("/schedules/:id/events/pending", getEventsHandler(eventsCollection))
		userGroup.GET("/schedules/:id/events/history", getEventsHandler(archivedEventsCollection))
	}
}

func createScheduleHandler(schedulesCollection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		var schedule models.Schedule
		if err := c.ShouldBindJSON(&schedule); err != nil {
			log.Error().Err(err).Str("route", "POST /api/schedules").Msg("Invalid request body")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body. Ensure the JSON structure matches the required format."})
			return
		}

		// Clear out any pre-existing ID, let MongoDB generate a new one
		schedule.ID = ""

		// Validate the RRule string
		_, err := rrule.StrToRRule(schedule.RRule)
		if err != nil {
			log.Error().Err(err).Str("route", "POST /api/schedules").Msg("Invalid RRULE")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid RRULE. Ensure it follows the correct format."})
			return
		}

		// Validate the CallbackURL
		_, err = url.ParseRequestURI(schedule.CallbackURL)
		if err != nil {
			log.Error().Err(err).Str("route", "POST /api/schedules").Msg("Invalid CallbackURL")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CallbackURL. Ensure it is a properly formatted URL."})
			return
		}

		// Prevent the request from setting CreatedAt
		schedule.CreatedAt = time.Now().UTC() // Always set CreatedAt to the current server time

		ctx := context.TODO()
		res, err := schedulesCollection.InsertOne(ctx, schedule)
		if err != nil {
			log.Error().Err(err).Str("route", "POST /api/schedules").Msg("Failed to create schedule in database")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create schedule. Please try again later."})
			return
		}

		insertedID := res.InsertedID // This should be a primitive.ObjectID
		log.Info().Str("route", "POST /api/schedules").Str("schedule_name", schedule.Name).Msg("Schedule created successfully")

		// Convert ObjectID to hex string if needed
		objectID, ok := insertedID.(primitive.ObjectID)
		if !ok {
			log.Warn().Str("route", "POST /api/schedules").Msg("InsertedID is not an ObjectID")
			c.JSON(http.StatusCreated, gin.H{"id": insertedID})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"id": objectID.Hex()})
	}
}

func updateScheduleHandler(schedulesCollection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			log.Warn().Str("route", "PUT /api/schedules/:id").Msg("Missing schedule ID in URL")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing schedule ID in URL path"})
			return
		}

		var updates bson.M
		if err := c.ShouldBindJSON(&updates); err != nil {
			log.Error().Err(err).Str("route", "PUT /api/schedules/:id").Msg("Invalid JSON body for updates")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON body for updates"})
			return
		}

		ctx := context.TODO()
		filter := bson.M{"_id": id}
		update := bson.M{"$set": updates}

		res, err := schedulesCollection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(false))
		if err != nil {
			log.Error().Err(err).Str("route", "PUT /api/schedules/:id").Str("schedule_id", id).Msg("Database error on update")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update schedule. Please try again later."})
			return
		}

		if res.MatchedCount == 0 {
			log.Warn().Str("route", "PUT /api/schedules/:id").Str("schedule_id", id).Msg("No schedule found to update")
			c.JSON(http.StatusNotFound, gin.H{"error": "No schedule found with the specified ID."})
			return
		}

		log.Info().Str("route", "PUT /api/schedules/:id").Str("schedule_id", id).Msg("Schedule updated successfully")
		c.JSON(http.StatusOK, gin.H{"message": "Schedule updated successfully."})
	}
}

func deleteScheduleHandler(schedulesCollection, eventsCollection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			log.Warn().Str("route", "DELETE /api/schedules/:id").Msg("Missing schedule ID in URL")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing schedule ID in URL path"})
			return
		}

		ctx := context.TODO()
		filter := bson.M{"_id": id}

		res, err := schedulesCollection.DeleteOne(ctx, filter)
		if err != nil {
			log.Error().Err(err).Str("route", "DELETE /api/schedules/:id").Str("schedule_id", id).Msg("Database error on schedule deletion")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete schedule. Please try again later."})
			return
		}
		if res.DeletedCount == 0 {
			log.Warn().Str("route", "DELETE /api/schedules/:id").Str("schedule_id", id).Msg("No schedule found to delete")
			c.JSON(http.StatusNotFound, gin.H{"error": "No schedule found with the specified ID."})
			return
		}

		_, err = eventsCollection.DeleteMany(ctx, bson.M{"schedule_id": id})
		if err != nil {
			log.Error().Err(err).Str("route", "DELETE /api/schedules/:id").Str("schedule_id", id).Msg("Failed to delete associated events")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete associated events. The schedule may be partially deleted."})
			return
		}

		log.Info().Str("route", "DELETE /api/schedules/:id").Str("schedule_id", id).Msg("Schedule and associated events deleted successfully")
		c.JSON(http.StatusOK, gin.H{"message": "Schedule and associated events deleted successfully."})
	}
}

// getEventsHandler retrieves events from the specified collection with pagination.
func getEventsHandler(collection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			log.Warn().Msg("Missing schedule ID in URL")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing schedule ID in URL path"})
			return
		}

		// Pagination parameters
		limit, page := getPaginationParams(c)

		ctx := context.TODO()
		filter := bson.M{"schedule_id": id}
		opts := options.Find().
			SetSkip(int64((page - 1) * limit)).
			SetLimit(int64(limit)).
			SetSort(bson.M{"run_time": -1}) // Sort by `run_time` descending

		cursor, err := collection.Find(ctx, filter, opts)
		if err != nil {
			log.Error().Err(err).Str("schedule_id", id).Msg("Database error fetching events")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch events. Please try again later."})
			return
		}
		defer cursor.Close(ctx)

		var events []bson.M
		if err := cursor.All(ctx, &events); err != nil {
			log.Error().Err(err).Str("schedule_id", id).Msg("Failed to parse events")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse events."})
			return
		}

		if len(events) == 0 {
			log.Info().Str("schedule_id", id).Msg("No events found")
			c.JSON(http.StatusOK, gin.H{"message": "No events found."})
			return
		}

		log.Info().Str("schedule_id", id).Msg("Events retrieved successfully")
		c.JSON(http.StatusOK, gin.H{"events": events, "page": page, "limit": limit})
	}
}

// getPaginationParams parses and validates pagination parameters from the query string.
func getPaginationParams(c *gin.Context) (limit, page int) {
	const (
		defaultLimit = 10
		defaultPage  = 1
	)

	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil || limit <= 0 {
		limit = defaultLimit
	}

	page, err = strconv.Atoi(c.Query("page"))
	if err != nil || page <= 0 {
		page = defaultPage
	}

	return limit, page
}
