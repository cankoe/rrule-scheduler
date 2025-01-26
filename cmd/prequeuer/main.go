package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cankoe/rrule-scheduler/internal/helpers"
	"github.com/cankoe/rrule-scheduler/internal/prequeuer"

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
		log.Info().Msgf("Received signal %s, shutting down Prequeuer gracefully...", sig)
		cancel()
	}()

	components, err := helpers.InitializeCommonComponents("prequeuer")
	if err != nil {
		log.Fatal().Err(err)
	}

	cfg := components.Config
	eventsCol := components.MongoDatabase.Collection("events")
	schedulesCol := components.MongoDatabase.Collection("schedules")

	tickerInterval := time.Duration(cfg.PreQueuer.TickerIntervalSeconds) * time.Second
	eventTimeframe := time.Duration(cfg.PreQueuer.EventTimeframeMinutes) * time.Minute

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(tickerInterval)
		defer ticker.Stop()

		log.Info().Msg("Prequeuer started. Generating events...")
		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("Prequeuer is shutting down...")
				return
			case <-ticker.C:
				prequeuer.GenerateEvents(ctx, schedulesCol, eventsCol, components.RedisClient, eventTimeframe)
			}
		}
	}()

	wg.Wait()
	components.CloseAll(context.Background())
	log.Info().Msg("Prequeuer exited gracefully")
}
