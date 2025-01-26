package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cankoe/rrule-scheduler/internal/api"
	"github.com/cankoe/rrule-scheduler/internal/helpers"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Info().Msgf("Received signal %s, shutting down API gracefully...", sig)
		cancel()
	}()

	// Initialize components
	components, err := helpers.InitializeCommonComponents("api")
	if err != nil {
		log.Fatal().Err(err)
	}

	// Initialize Gin router
	r := gin.Default()
	// Register routes
	api.RegisterRoutes(r, components.MongoDatabase)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info().Msg("API server started on port 8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	log.Info().Msg("Shutting down API server...")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("API server forced to shutdown")
	}
	wg.Wait()

	components.CloseAll(context.Background())
	log.Info().Msg("API server exited gracefully")
}
