package helpers

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// UpdateEventStatus adds a new status entry to the event's status array.
func UpdateEventStatus(ctx context.Context, eventsCollection *mongo.Collection, eventID string, status, message string) error {
	// Convert the eventID to an ObjectID
	objectID, err := primitive.ObjectIDFromHex(eventID)
	if err != nil {
		return fmt.Errorf("invalid ObjectID format for eventID %s: %v", eventID, err)
	}

	// Prepare the status update
	statusUpdate := bson.M{
		"time":    time.Now().UTC(),
		"status":  status,
		"message": message,
	}

	// Update the event's status array
	filter := bson.M{"_id": objectID}
	update := bson.M{
		"$push": bson.M{
			"status": statusUpdate,
		},
	}

	_, err = eventsCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update event %s: %v", eventID, err)
	}

	return nil
}
