package config

import (
	"os"
	"strconv"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Mongo struct {
		URI      string `yaml:"uri"`
		Database string `yaml:"database"`
	} `yaml:"mongo"`

	Redis struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"redis"`

	PreQueuer struct {
		TickerIntervalSeconds int `yaml:"ticker_interval_seconds"`
		EventTimeframeMinutes int `yaml:"event_timeframe_minutes"`
	} `yaml:"prequeuer"`

	APIKeys struct {
		User  string `yaml:"user"`
		Admin string `yaml:"admin"`
	} `yaml:"api_keys"`
}

func LoadConfig(path string) *Config {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		log.Warn().Err(err).Str("config_path", path).Msg("Failed to read configuration file, will rely on defaults and environment variables")
	} else {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			log.Fatal().Err(err).Str("config_path", path).Msg("Failed to parse YAML configuration")
		}
	}

	overrideWithEnv(cfg)
	validateAndSetDefaults(cfg)
	return cfg
}

func overrideWithEnv(cfg *Config) {
	// MongoDB
	if val := os.Getenv("MONGO_URI"); val != "" {
		cfg.Mongo.URI = val
	}
	if val := os.Getenv("MONGO_DATABASE"); val != "" {
		cfg.Mongo.Database = val
	}

	// Redis
	if val := os.Getenv("REDIS_HOST"); val != "" {
		cfg.Redis.Host = val
	}
	if val := os.Getenv("REDIS_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			cfg.Redis.Port = port
		}
	}

	// PreQueuer
	if val := os.Getenv("PREQUEUER_TICKER_INTERVAL_SECONDS"); val != "" {
		if interval, err := strconv.Atoi(val); err == nil {
			cfg.PreQueuer.TickerIntervalSeconds = interval
		}
	}
	if val := os.Getenv("PREQUEUER_EVENT_TIMEFRAME_MINUTES"); val != "" {
		if timeframe, err := strconv.Atoi(val); err == nil {
			cfg.PreQueuer.EventTimeframeMinutes = timeframe
		}
	}

	// API Keys
	if val := os.Getenv("API_KEYS_USER"); val != "" {
		cfg.APIKeys.User = val
	}
	if val := os.Getenv("API_KEYS_ADMIN"); val != "" {
		cfg.APIKeys.Admin = val
	}
}

func validateAndSetDefaults(cfg *Config) {
	// Validate MongoDB URI
	if cfg.Mongo.URI == "" {
		log.Warn().Msg("MONGO_URI not provided, defaulting to mongodb://localhost:27017")
		cfg.Mongo.URI = "mongodb://localhost:27017"
	}

	if cfg.Mongo.Database == "" {
		log.Warn().Msg("MONGO_DATABASE not provided, defaulting to 'scheduler'")
		cfg.Mongo.Database = "scheduler"
	}

	// Validate Redis
	if cfg.Redis.Host == "" {
		log.Warn().Msg("REDIS_HOST not provided, defaulting to 'localhost'")
		cfg.Redis.Host = "localhost"
	}
	if cfg.Redis.Port == 0 {
		log.Warn().Msg("REDIS_PORT not provided, defaulting to 6379")
		cfg.Redis.Port = 6379
	}

	// PreQueuer defaults
	if cfg.PreQueuer.TickerIntervalSeconds == 0 {
		log.Info().Msg("TickerIntervalSeconds not set, defaulting to 30")
		cfg.PreQueuer.TickerIntervalSeconds = 30
	}
	if cfg.PreQueuer.EventTimeframeMinutes == 0 {
		log.Info().Msg("EventTimeframeMinutes not set, defaulting to 60")
		cfg.PreQueuer.EventTimeframeMinutes = 60
	}

	// API Keys checks (optional)
	if cfg.APIKeys.User == "" {
		log.Warn().Msg("No user API key provided, user API routes will be unprotected")
	}
	if cfg.APIKeys.Admin == "" {
		log.Warn().Msg("No admin API key provided, admin routes will be unprotected")
	}
}
