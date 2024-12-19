package main

import (
	"event-scheduler/internal/config"
	"event-scheduler/internal/database"
	"event-scheduler/internal/queue"
	"log"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig("config/config.yaml")

	// Test MongoDB connection
	mongoClient, err := database.NewMongoClient(cfg.Mongo.URI)
	if err != nil {
		log.Fatalf("MongoDB connection failed: %v", err)
	}
	log.Println("MongoDB connected successfully!")

	// Ping Redis
	redisClient := queue.NewRedisClient(cfg.Redis.Host, cfg.Redis.Port)
	if err := redisClient.Ping(queue.Ctx).Err(); err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}
	log.Println("Redis connected successfully!")

	// Cleanup
	defer mongoClient.Disconnect(queue.Ctx)
	defer redisClient.Close()
}
