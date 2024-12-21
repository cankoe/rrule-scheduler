package config

import (
	"flag"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type Config struct {
	Mongo struct {
		URI      string `mapstructure:"uri"`
		Database string `mapstructure:"database"`
	} `mapstructure:"mongo"`

	Redis struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
	} `mapstructure:"redis"`

	PreQueuer struct {
		TickerIntervalSeconds int `mapstructure:"ticker_interval_seconds"`
		EventTimeframeMinutes int `mapstructure:"event_timeframe_minutes"`
	} `mapstructure:"prequeuer"`

	APIKeys struct {
		User  string `mapstructure:"user"`
		Admin string `mapstructure:"admin"`
	} `mapstructure:"api_keys"`
}

// LoadConfig loads the configuration from file, environment variables, and command-line arguments.
// Order of precedence: defaults < config file < env vars < cmd flags.
func LoadConfig(configPath string, args []string) (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("mongo.uri", "mongodb://localhost:27017")
	v.SetDefault("mongo.database", "scheduler")
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("prequeuer.ticker_interval_seconds", 30)
	v.SetDefault("prequeuer.event_timeframe_minutes", 60)

	// Read from config file if present
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		log.Warn().Err(err).Str("config_path", configPath).Msg("Failed to read config file, relying on defaults, env, and flags")
	}

	// Explicitly bind environment variables
	bindEnvOrPanic(v, "mongo.uri", "MONGO_URI")
	bindEnvOrPanic(v, "mongo.database", "MONGO_DATABASE")
	bindEnvOrPanic(v, "redis.host", "REDIS_HOST")
	bindEnvOrPanic(v, "redis.port", "REDIS_PORT")
	bindEnvOrPanic(v, "prequeuer.ticker_interval_seconds", "PREQUEUER_TICKER_INTERVAL_SECONDS")
	bindEnvOrPanic(v, "prequeuer.event_timeframe_minutes", "PREQUEUER_EVENT_TIMEFRAME_MINUTES")
	bindEnvOrPanic(v, "api_keys.user", "API_KEYS_USER")
	bindEnvOrPanic(v, "api_keys.admin", "API_KEYS_ADMIN")

	// Parse command-line flags for prequeuer
	preTicker := flag.Int("prequeuer-ticker-seconds", 0, "Override PreQueuer ticker interval in seconds")
	preTimeframe := flag.Int("prequeuer-timeframe-minutes", 0, "Override PreQueuer event timeframe in minutes")
	flag.CommandLine.Parse(args)

	// Apply command-line flags if provided
	if *preTicker > 0 {
		v.Set("prequeuer.ticker_interval_seconds", *preTicker)
	}
	if *preTimeframe > 0 {
		v.Set("prequeuer.event_timeframe_minutes", *preTimeframe)
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func bindEnvOrPanic(v *viper.Viper, key, env string) {
	if err := v.BindEnv(key, env); err != nil {
		log.Fatal().Err(err).Msgf("Failed to bind environment variable %s to key %s", env, key)
	}
}

func validateConfig(cfg *Config) error {
	if cfg.Mongo.URI == "" {
		log.Warn().Msg("MONGO_URI not provided, using default")
	}
	if cfg.Mongo.Database == "" {
		log.Warn().Msg("MONGO_DATABASE not provided, using default")
	}
	if cfg.APIKeys.User == "" {
		log.Warn().Msg("No user API key provided, user API routes will be unprotected")
	}
	if cfg.APIKeys.Admin == "" {
		log.Warn().Msg("No admin API key provided, admin routes will be unprotected")
	}

	// Validate PreQueuer settings
	if cfg.PreQueuer.TickerIntervalSeconds <= 0 {
		return fmt.Errorf("PreQueuer ticker_interval_seconds must be > 0, got %d", cfg.PreQueuer.TickerIntervalSeconds)
	}
	if cfg.PreQueuer.EventTimeframeMinutes <= 0 {
		return fmt.Errorf("PreQueuer event_timeframe_minutes must be > 0, got %d", cfg.PreQueuer.EventTimeframeMinutes)
	}

	return nil
}
