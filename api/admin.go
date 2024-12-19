package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// RegisterAdminRoutes registers admin-specific routes
func RegisterAdminRoutes(r *gin.Engine, db *mongo.Database, adminAPIKey string) {
	schedulesCollection := db.Collection("schedules")
	eventsCollection := db.Collection("events")

	adminGroup := r.Group("/admin", APIKeyMiddleware(adminAPIKey, true))
	{
		adminGroup.GET("/schedules", getAllSchedulesHandler(schedulesCollection))
		adminGroup.GET("/events", getAllEventsHandler(eventsCollection))
		adminGroup.DELETE("/schedules", deleteAllSchedulesHandler(schedulesCollection, eventsCollection))
		adminGroup.DELETE("/events", deleteAllEventsHandler(eventsCollection))
	}
}

func getAllSchedulesHandler(schedulesCollection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.TODO()
		cursor, err := schedulesCollection.Find(ctx, bson.M{})
		if err != nil {
			log.Error().Err(err).Str("route", "GET /admin/schedules").Msg("Failed to fetch schedules from database")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch schedules. Please try again later."})
			return
		}
		defer cursor.Close(ctx)

		var schedules []bson.M
		if err := cursor.All(ctx, &schedules); err != nil {
			log.Error().Err(err).Str("route", "GET /admin/schedules").Msg("Failed to parse schedules")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse schedules."})
			return
		}

		if len(schedules) == 0 {
			log.Info().Str("route", "GET /admin/schedules").Msg("No schedules found")
			c.JSON(http.StatusOK, gin.H{"message": "No schedules found."})
			return
		}

		log.Info().Str("route", "GET /admin/schedules").Msg("Schedules retrieved successfully")
		c.JSON(http.StatusOK, schedules)
	}
}

func getAllEventsHandler(eventsCollection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.TODO()
		cursor, err := eventsCollection.Find(ctx, bson.M{})
		if err != nil {
			log.Error().Err(err).Str("route", "GET /admin/events").Msg("Failed to fetch events from database")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch events. Please try again later."})
			return
		}
		defer cursor.Close(ctx)

		var events []bson.M
		if err := cursor.All(ctx, &events); err != nil {
			log.Error().Err(err).Str("route", "GET /admin/events").Msg("Failed to parse events")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse events."})
			return
		}

		if len(events) == 0 {
			log.Info().Str("route", "GET /admin/events").Msg("No events found")
			c.JSON(http.StatusOK, gin.H{"message": "No events found."})
			return
		}

		log.Info().Str("route", "GET /admin/events").Msg("Events retrieved successfully")
		c.JSON(http.StatusOK, events)
	}
}

func deleteAllSchedulesHandler(schedulesCollection, eventsCollection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.TODO()
		_, err := schedulesCollection.DeleteMany(ctx, bson.M{})
		if err != nil {
			log.Error().Err(err).Str("route", "DELETE /admin/schedules").Msg("Failed to delete schedules")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete schedules."})
			return
		}

		_, err = eventsCollection.DeleteMany(ctx, bson.M{})
		if err != nil {
			log.Error().Err(err).Str("route", "DELETE /admin/schedules").Msg("Failed to delete events after schedules deletion")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete events after schedules deletion."})
			return
		}

		log.Info().Str("route", "DELETE /admin/schedules").Msg("All schedules and events deleted successfully")
		c.JSON(http.StatusOK, gin.H{"message": "All schedules and events deleted successfully."})
	}
}

func deleteAllEventsHandler(eventsCollection *mongo.Collection) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.TODO()
		_, err := eventsCollection.DeleteMany(ctx, bson.M{})
		if err != nil {
			log.Error().Err(err).Str("route", "DELETE /admin/events").Msg("Failed to delete events")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete events."})
			return
		}

		log.Info().Str("route", "DELETE /admin/events").Msg("All events deleted successfully")
		c.JSON(http.StatusOK, gin.H{"message": "All events deleted successfully."})
	}
}
