package main

import (
	"context"

	"event-scheduler/internal/api"
	"event-scheduler/internal/helpers"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func main() {
	components, err := helpers.InitializeCommonComponents("dispatcher")
	if err != nil {
		log.Fatal().Err(err)
	}
	defer components.CloseAll(context.Background())

	cfg := components.Config

	// Initialize Gin router
	r := gin.Default()

	// Register user and admin routes
	api.RegisterRoutes(r, components.MongoDatabase, cfg.APIKeys.User)
	api.RegisterAdminRoutes(r, components.MongoDatabase, cfg.APIKeys.Admin)

	// Start server
	log.Info().Msg("API server started on port 8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}
