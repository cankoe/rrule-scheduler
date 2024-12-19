package main

import (
	"log"

	"event-scheduler/api"
	"event-scheduler/internal/config"
	"event-scheduler/internal/database"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig("config/config.yaml")

	// Initialize MongoDB
	mongoClient, err := database.NewMongoClient(cfg.Mongo.URI)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoClient.Disconnect(nil)

	db := mongoClient.Database(cfg.Mongo.Database)

	// Initialize Gin router
	r := gin.Default()

	// Register user and admin routes
	api.RegisterRoutes(r, db, cfg.APIKeys.User)
	api.RegisterAdminRoutes(r, db, cfg.APIKeys.Admin)

	// Start server
	log.Println("API server started on port 8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
