package helpers

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// UpdateEventStatus adds a new status entry to the event's status array.
func UpdateEventStatus(ctx context.Context, eventsCollection *mongo.Collection, eventID string, status, message string) error {
	objectID, err := convertToObjectID(eventID)
	if err != nil {
		return err
	}

	statusUpdate := bson.M{
		"time":    time.Now().UTC(),
		"status":  status,
		"message": message,
	}

	filter := bson.M{"_id": objectID}
	update := bson.M{"$push": bson.M{"status": statusUpdate}}

	if _, err := eventsCollection.UpdateOne(ctx, filter, update); err != nil {
		log.Error().Err(err).Str("event_id", eventID).Msg("Failed to update event status")
		return err
	}

	log.Info().Str("event_id", eventID).Str("status", status).Msg("Event status updated successfully")
	return nil
}

// UpdateAndArchiveEvent updates the event's status and moves it to the archivedEventsCollection.
func UpdateAndArchiveEvent(ctx context.Context, eventsCollection, archivedEventsCollection *mongo.Collection, eventID string, status, message string) error {
	// Update the event's status
	if err := UpdateEventStatus(ctx, eventsCollection, eventID, status, message); err != nil {
		return err
	}

	objectID, err := convertToObjectID(eventID)
	if err != nil {
		return err
	}

	// Find the event
	event, err := findEventByID(ctx, eventsCollection, objectID)
	if err != nil {
		return err
	}

	// Archive the event
	if err := archiveEvent(ctx, archivedEventsCollection, eventsCollection, objectID, event); err != nil {
		return err
	}

	log.Info().Str("event_id", eventID).Msg("Event archived and deleted successfully")
	return nil
}

// convertToObjectID converts a string ID to a MongoDB ObjectID.
func convertToObjectID(eventID string) (primitive.ObjectID, error) {
	objectID, err := primitive.ObjectIDFromHex(eventID)
	if err != nil {
		log.Error().Err(err).Str("event_id", eventID).Msg("Invalid ObjectID format")
		return primitive.NilObjectID, err
	}
	return objectID, nil
}

// findEventByID retrieves an event by its ObjectID.
func findEventByID(ctx context.Context, eventsCollection *mongo.Collection, objectID primitive.ObjectID) (bson.M, error) {
	var event bson.M
	if err := eventsCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&event); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Warn().Str("event_id", objectID.Hex()).Msg("Event not found")
		} else {
			log.Error().Err(err).Str("event_id", objectID.Hex()).Msg("Failed to retrieve event")
		}
		return nil, err
	}
	return event, nil
}

// archiveEvent moves an event to the archivedEventsCollection and deletes it from the original collection.
func archiveEvent(ctx context.Context, archivedEventsCollection, eventsCollection *mongo.Collection, objectID primitive.ObjectID, event bson.M) error {
	// Insert into the archive
	if _, err := archivedEventsCollection.InsertOne(ctx, event); err != nil {
		log.Error().Err(err).Str("event_id", objectID.Hex()).Msg("Failed to archive event")
		return err
	}

	// Delete from the original collection
	if _, err := eventsCollection.DeleteOne(ctx, bson.M{"_id": objectID}); err != nil {
		// Rollback: Remove from archive if deletion fails
		if _, rollbackErr := archivedEventsCollection.DeleteOne(ctx, bson.M{"_id": objectID}); rollbackErr != nil {
			log.Error().Err(err).Err(rollbackErr).Str("event_id", objectID.Hex()).Msg("Failed to delete event and rollback archive")
			return rollbackErr
		}

		log.Error().Err(err).Str("event_id", objectID.Hex()).Msg("Failed to delete event from eventsCollection")
		return err
	}

	return nil
}
