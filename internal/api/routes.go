package api

import (
	"rrule-scheduler/internal/schedules"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

// RegisterRoutes registers all top-level domain routes.
func RegisterRoutes(r *gin.Engine, db *mongo.Database) {
	// Serve Swagger UI
	r.Static("/swagger-ui", "./swagger-ui")
	r.StaticFile("/docs/openapi.yml", "./docs/openapi.yml")

	// Schedules & related events
	schedules.RegisterScheduleRoutes(r, db)
}
