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

	Worker struct {
		Count      int `mapstructure:"count"`
		MaxRetries int `mapstructure:"max_retries"`
	} `mapstructure:"worker"`

	Log struct {
		Level string `mapstructure:"level"`
	} `mapstructure:"log"`
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
	v.SetDefault("worker.max_retries", 3)
	v.SetDefault("worker.count", 5)
	v.SetDefault("log.level", "info")

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
	bindEnvOrPanic(v, "worker.max_retries", "WORKER_MAX_RETRIES")
	bindEnvOrPanic(v, "worker.count", "WORKER_COUNT")
	bindEnvOrPanic(v, "log.level", "LOG_LEVEL")

	// Parse command-line flags for prequeuer
	preTicker := flag.Int("prequeuer-ticker-seconds", 0, "Override PreQueuer ticker interval in seconds")
	preTimeframe := flag.Int("prequeuer-timeframe-minutes", 0, "Override PreQueuer event timeframe in minutes")
	workerMaxRetries := flag.Int("worker-max-retries", 0, "Override Worker max retries")
	workerCount := flag.Int("worker-count", 0, "Override Worker Count")
	logLevel := flag.String("log-level", "", "Override log level")
	flag.CommandLine.Parse(args)

	// Apply command-line flags if provided
	if *preTicker > 0 {
		v.Set("prequeuer.ticker_interval_seconds", *preTicker)
	}
	if *preTimeframe > 0 {
		v.Set("prequeuer.event_timeframe_minutes", *preTimeframe)
	}
	if *workerMaxRetries > 0 {
		v.Set("worker.max_retries", *workerMaxRetries)
	}
	if *workerCount > 0 {
		v.Set("worker.count", *workerCount)
	}
	v.Set("log.level", *logLevel)

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
	// Validate Mongo settings
	if cfg.Mongo.URI == "" {
		log.Warn().Msg("MONGO_URI not provided, using default")
	}
	if cfg.Mongo.Database == "" {
		log.Warn().Msg("MONGO_DATABASE not provided, using default")
	}

	// Validate Redis settings
	if cfg.Redis.Host == "" {
		log.Warn().Msg("REDIS_HOST not provided, using default")
	}

	// Validate PreQueuer settings
	if cfg.PreQueuer.TickerIntervalSeconds <= 0 {
		return fmt.Errorf("PreQueuer ticker_interval_seconds must be > 0, got %d", cfg.PreQueuer.TickerIntervalSeconds)
	}
	if cfg.PreQueuer.EventTimeframeMinutes <= 0 {
		return fmt.Errorf("PreQueuer event_timeframe_minutes must be > 0, got %d", cfg.PreQueuer.EventTimeframeMinutes)
	}

	// Validate Worker settings
	if cfg.Worker.MaxRetries <= 0 {
		return fmt.Errorf("worker max_retries must be > 0, got %d", cfg.Worker.MaxRetries)
	}
	if cfg.Worker.Count <= 0 {
		return fmt.Errorf("worker count must be > 0, got %d", cfg.Worker.MaxRetries)
	}

	return nil
}
