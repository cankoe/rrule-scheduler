package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cankoe/rrule-scheduler/internal/dispatcher"
	"github.com/cankoe/rrule-scheduler/internal/helpers"

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
		log.Info().Msgf("Received signal %s, shutting down Dispatcher gracefully...", sig)
		cancel()
	}()

	components, err := helpers.InitializeCommonComponents("dispatcher")
	if err != nil {
		log.Fatal().Err(err)
	}

	eventsCol := components.MongoDatabase.Collection("events")
	archivedEventsCol := components.MongoDatabase.Collection("archived_events")

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		log.Info().Msg("Dispatcher started.")
		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("Dispatcher is shutting down...")
				return
			case <-ticker.C:
				dispatcher.DispatchDueEvents(ctx, components.RedisClient, eventsCol, archivedEventsCol)
			}
		}
	}()

	wg.Wait()
	components.CloseAll(context.Background())
	log.Info().Msg("Dispatcher exited gracefully")
}
