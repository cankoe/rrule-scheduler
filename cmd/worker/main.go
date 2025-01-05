package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"rrule-scheduler/internal/helpers"
	"rrule-scheduler/internal/worker"

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
		log.Info().Msgf("Received signal %s, shutting down Worker gracefully...", sig)
		cancel()
	}()

	components, err := helpers.InitializeCommonComponents("worker")
	if err != nil {
		log.Fatal().Err(err)
	}

	eventsCol := components.MongoDatabase.Collection("events")
	archivedEventsCol := components.MongoDatabase.Collection("archived_events")
	schedulesCol := components.MongoDatabase.Collection("schedules")

	if err := worker.EnsureIndexes(eventsCol, schedulesCol); err != nil {
		log.Fatal().Err(err).Msg("Failed to create necessary indexes")
	}

	workerCount := components.Config.Worker.Count
	log.Info().Int("workers", workerCount).Msg("Spawning worker goroutines")

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go worker.EventWorker(ctx, &wg, components.RedisClient, eventsCol,
			archivedEventsCol, schedulesCol, i+1, components.Config.Worker.MaxRetries)
	}

	wg.Wait()
	components.CloseAll(context.Background())
	log.Info().Msg("Worker service exited gracefully")
}
