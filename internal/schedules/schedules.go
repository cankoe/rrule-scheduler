package schedules

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/cankoe/rrule-scheduler/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/teambition/rrule-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	ErrCodeInvalidRequest   = "invalid_request"
	ErrCodeNotFound         = "not_found"
	ErrCodeDatabaseError    = "database_error"
	ErrCodeValidationFailed = "validation_failed"
)

type ApiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ApiError) Error() string {
	return e.Message
}

// RegisterScheduleRoutes defines HTTP routes for schedules & their events.
func RegisterScheduleRoutes(r *gin.Engine, db *mongo.Database) {
	schedulesCol := db.Collection("schedules")
	eventsCol := db.Collection("events")
	archivedEventsCol := db.Collection("archived_events")

	group := r.Group("/api")

	group.GET("/schedules/:id", func(c *gin.Context) {
		scheduleID := c.Param("id")
		schedule, err := getScheduleByID(c.Request.Context(), schedulesCol, scheduleID)
		if err != nil {
			statusCode, apiErr := mapErrorToStatusCode(err)
			c.JSON(statusCode, gin.H{"error": apiErr})
			return
		}
		c.JSON(http.StatusOK, schedule)
	})

	group.POST("/schedules", func(c *gin.Context) {
		var schedule models.Schedule
		if err := c.ShouldBindJSON(&schedule); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		objID, err := createSchedule(c.Request.Context(), schedulesCol, &schedule)
		if err != nil {
			statusCode, apiErr := mapErrorToStatusCode(err)
			c.JSON(statusCode, gin.H{"error": apiErr})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": objID.Hex()})
	})

	group.PUT("/schedules/:id", func(c *gin.Context) {
		scheduleID := c.Param("id")
		var updates bson.M
		if err := c.ShouldBindJSON(&updates); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON body for updates"})
			return
		}
		if err := updateSchedule(c.Request.Context(), schedulesCol, scheduleID, updates); err != nil {
			statusCode, apiErr := mapErrorToStatusCode(err)
			c.JSON(statusCode, gin.H{"error": apiErr})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Schedule updated successfully."})
	})

	group.DELETE("/schedules/:id", func(c *gin.Context) {
		scheduleID := c.Param("id")
		if err := deleteScheduleAndEvents(c.Request.Context(), schedulesCol, eventsCol, scheduleID); err != nil {
			statusCode, apiErr := mapErrorToStatusCode(err)
			c.JSON(statusCode, gin.H{"error": apiErr})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Schedule and associated events deleted."})
	})

	// GET events (pending or history)
	group.GET("/schedules/:id/events/pending", func(c *gin.Context) {
		handleGetEvents(c, eventsCol)
	})
	group.GET("/schedules/:id/events/history", func(c *gin.Context) {
		handleGetEvents(c, archivedEventsCol)
	})
}

/**************************************************************************/
/*                           DB & Validation                              */
/**************************************************************************/

func getScheduleByID(ctx context.Context, col *mongo.Collection, scheduleHexID string) (*models.Schedule, error) {
	oid, err := primitive.ObjectIDFromHex(scheduleHexID)
	if err != nil {
		return nil, &ApiError{
			Code:    ErrCodeInvalidRequest,
			Message: "Invalid schedule ID format",
		}
	}

	var schedule models.Schedule
	err = col.FindOne(ctx, bson.M{"_id": oid}).Decode(&schedule)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, &ApiError{
				Code:    ErrCodeNotFound,
				Message: "Schedule not found",
			}
		}
		return nil, &ApiError{
			Code:    ErrCodeDatabaseError,
			Message: "Failed to retrieve schedule",
		}
	}

	// Convert ObjectID to hex string for response
	schedule.ID = oid.Hex()
	return &schedule, nil
}

func createSchedule(ctx context.Context, col *mongo.Collection, s *models.Schedule) (primitive.ObjectID, error) {
	// Clear out any provided ID to let Mongo generate it
	s.ID = ""
	if err := validateSchedule(s); err != nil {
		return primitive.NilObjectID, err
	}

	res, err := col.InsertOne(ctx, s)
	if err != nil {
		return primitive.NilObjectID, &ApiError{
			Code:    ErrCodeDatabaseError,
			Message: "Failed to create schedule in database",
		}
	}
	oid, ok := res.InsertedID.(primitive.ObjectID)
	if !ok {
		return primitive.NilObjectID, errors.New("inserted ID is not an ObjectID")
	}
	return oid, nil
}

func updateSchedule(ctx context.Context, col *mongo.Collection, scheduleHexID string, updates bson.M) error {
	oid, err := primitive.ObjectIDFromHex(scheduleHexID)
	if err != nil {
		return errors.New("invalid schedule ID format")
	}
	stripReadOnlyFields(updates)

	filter := bson.M{"_id": oid}
	update := bson.M{"$set": updates}

	res, err := col.UpdateOne(ctx, filter, update, options.Update().SetUpsert(false))
	if err != nil {
		return errors.New("database error on schedule update")
	}
	if res.MatchedCount == 0 {
		return errors.New("no schedule found with the specified ID")
	}
	return nil
}

func deleteScheduleAndEvents(ctx context.Context, schedulesCol, eventsCol *mongo.Collection, scheduleHexID string) error {
	oid, err := primitive.ObjectIDFromHex(scheduleHexID)
	if err != nil {
		return errors.New("invalid schedule ID format")
	}

	filter := bson.M{"_id": oid}
	res, err := schedulesCol.DeleteOne(ctx, filter)
	if err != nil {
		return errors.New("failed to delete schedule from DB")
	}
	if res.DeletedCount == 0 {
		return errors.New("no schedule found with the specified ID")
	}

	// Remove events that belong to this schedule
	if _, err := eventsCol.DeleteMany(ctx, bson.M{"schedule_id": scheduleHexID}); err != nil {
		return errors.New("failed to delete associated events")
	}
	return nil
}

/**************************************************************************/
/*                           EVENT LISTING                                */
/**************************************************************************/

func handleGetEvents(c *gin.Context, col *mongo.Collection) {
	scheduleID := c.Param("id")
	if scheduleID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing schedule ID in URL path"})
		return
	}
	limit, page := getPaginationParams(c)

	ctx := c.Request.Context()
	filter := bson.M{"schedule_id": scheduleID}
	opts := options.Find().
		SetSkip(int64((page - 1) * limit)).
		SetLimit(int64(limit)).
		SetSort(bson.M{"run_time": -1})

	cursor, err := col.Find(ctx, filter, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch events"})
		return
	}
	defer cursor.Close(ctx)

	var events []bson.M
	if err := cursor.All(ctx, &events); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse events"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": events, "page": page, "limit": limit})
}

/**************************************************************************/
/*                          Helper Utilities                              */
/**************************************************************************/

func validateSchedule(s *models.Schedule) error {
	if s.Name == "" {
		return &ApiError{
			Code:    ErrCodeValidationFailed,
			Message: "Schedule name cannot be empty",
		}
	}
	if s.RRule == "" {
		return &ApiError{
			Code:    ErrCodeValidationFailed,
			Message: "RRULE cannot be empty",
		}
	}
	if _, err := rrule.StrToRRule(s.RRule); err != nil {
		return &ApiError{
			Code:    ErrCodeValidationFailed,
			Message: "Invalid RRULE format",
		}
	}
	if _, err := url.ParseRequestURI(s.CallbackURL); err != nil {
		return &ApiError{
			Code:    ErrCodeValidationFailed,
			Message: "Invalid callback URL format",
		}
	}
	return nil
}

func stripReadOnlyFields(updates bson.M) {
	delete(updates, "_id")
	delete(updates, "created_at")
}

func getPaginationParams(c *gin.Context) (int, int) {
	const defaultLimit = 10
	const defaultPage = 1

	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil || limit <= 0 {
		limit = defaultLimit
	}
	page, err := strconv.Atoi(c.Query("page"))
	if err != nil || page <= 0 {
		page = defaultPage
	}
	return limit, page
}

func mapErrorToStatusCode(err error) (int, *ApiError) {
	var apiErr *ApiError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case ErrCodeInvalidRequest:
			return http.StatusBadRequest, apiErr
		case ErrCodeNotFound:
			return http.StatusNotFound, apiErr
		case ErrCodeValidationFailed:
			return http.StatusUnprocessableEntity, apiErr
		case ErrCodeDatabaseError:
			return http.StatusInternalServerError, apiErr
		}
	}

	// Default unknown error
	return http.StatusInternalServerError, &ApiError{
		Code:    "internal_error",
		Message: "An unexpected error occurred",
	}
}
