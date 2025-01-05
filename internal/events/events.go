package events

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// UpdateEventStatus adds a new status entry to the event's status array.
func UpdateEventStatus(ctx context.Context, eventsCollection *mongo.Collection, eventID, status, message string) error {
	objectID, err := primitive.ObjectIDFromHex(eventID)
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
func UpdateAndArchiveEvent(ctx context.Context,
	eventsCollection, archivedCollection *mongo.Collection,
	eventID, status, message string,
) error {
	if err := UpdateEventStatus(ctx, eventsCollection, eventID, status, message); err != nil {
		return err
	}
	oid, err := primitive.ObjectIDFromHex(eventID)
	if err != nil {
		return err
	}

	// Find the event
	var eventDoc bson.M
	if err := eventsCollection.FindOne(ctx, bson.M{"_id": oid}).Decode(&eventDoc); err != nil {
		log.Error().Err(err).Str("event_id", eventID).Msg("Failed to retrieve event for archiving")
		return err
	}

	// Insert into the archive
	if _, err := archivedCollection.InsertOne(ctx, eventDoc); err != nil {
		log.Error().Err(err).Str("event_id", eventID).Msg("Failed to archive event")
		return err
	}

	// Delete from original
	if _, err := eventsCollection.DeleteOne(ctx, bson.M{"_id": oid}); err != nil {
		// Attempt rollback
		if _, rbErr := archivedCollection.DeleteOne(ctx, bson.M{"_id": oid}); rbErr != nil {
			log.Error().Err(rbErr).Str("event_id", eventID).Msg("Rollback failed after deleteOne error")
		}
		return err
	}
	log.Info().Str("event_id", eventID).Msg("Event archived and deleted successfully")
	return nil
}

// RecordErrorStatus is a small helper to set status="error" & message.
func RecordErrorStatus(ctx context.Context,
	eventsCol, archivedCol *mongo.Collection,
	eventID, errorMsg string,
) {
	if err := UpdateAndArchiveEvent(ctx, eventsCol, archivedCol, eventID, "error", errorMsg); err != nil {
		log.Error().Err(err).Str("event_id", eventID).Msg("Failed to record error status")
	}
}
