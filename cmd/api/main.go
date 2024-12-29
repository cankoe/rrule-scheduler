package main

import (
	"context"

	"event-scheduler/internal/api"
	"event-scheduler/internal/helpers"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func main() {
	components, err := helpers.InitializeCommonComponents("api")
	if err != nil {
		log.Fatal().Err(err)
	}
	defer components.CloseAll(context.Background())

	cfg := components.Config

	// Initialize Gin router
	r := gin.Default()

	// Register user routes
	api.RegisterRoutes(r, components.MongoDatabase, cfg.APIKeys.User)

	// Start server
	log.Info().Msg("API server started on port 8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}
