package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Provider ProviderConfig
	Worker   WorkerConfig
	Tracing  TracingConfig
}

type ServerConfig struct {
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"ssl_mode"`
	MaxConns int    `mapstructure:"max_conns"`
}

func (d DatabaseConfig) DSN() string {
	return "postgres://" + d.User + ":" + d.Password + "@" + d.Host + ":" +
		intToStr(d.Port) + "/" + d.Name + "?sslmode=" + d.SSLMode
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type ProviderConfig struct {
	WebhookURL string        `mapstructure:"webhook_url"`
	Timeout    time.Duration `mapstructure:"timeout"`
}

type WorkerConfig struct {
	Concurrency       int           `mapstructure:"concurrency"`
	PollInterval      time.Duration `mapstructure:"poll_interval"`
	RateLimitPerSec   int           `mapstructure:"rate_limit_per_sec"`
	MaxRetries        int           `mapstructure:"max_retries"`
	RetryBaseDelay    time.Duration `mapstructure:"retry_base_delay"`
	RetryMaxDelay     time.Duration `mapstructure:"retry_max_delay"`
	SchedulerInterval time.Duration `mapstructure:"scheduler_interval"`
}

type TracingConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Endpoint string `mapstructure:"endpoint"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", "15s")
	viper.SetDefault("server.write_timeout", "15s")
	viper.SetDefault("server.shutdown_timeout", "30s")

	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("database.name", "notifications")
	viper.SetDefault("database.ssl_mode", "disable")
	viper.SetDefault("database.max_conns", 25)

	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.db", 0)

	viper.SetDefault("provider.timeout", "10s")

	viper.SetDefault("worker.concurrency", 5)
	viper.SetDefault("worker.poll_interval", "100ms")
	viper.SetDefault("worker.rate_limit_per_sec", 100)
	viper.SetDefault("worker.max_retries", 3)
	viper.SetDefault("worker.retry_base_delay", "1s")
	viper.SetDefault("worker.retry_max_delay", "5m")
	viper.SetDefault("worker.scheduler_interval", "5s")

	viper.SetDefault("tracing.enabled", true)
	viper.SetDefault("tracing.endpoint", "localhost:4318")

	viper.SetEnvPrefix("NOTIFY")
	viper.AutomaticEnv()

	// Explicit env bindings for nested keys
	bindEnvs := map[string]string{
		"server.port":              "NOTIFY_SERVER_PORT",
		"database.host":            "NOTIFY_DB_HOST",
		"database.port":            "NOTIFY_DB_PORT",
		"database.user":            "NOTIFY_DB_USER",
		"database.password":        "NOTIFY_DB_PASSWORD",
		"database.name":            "NOTIFY_DB_NAME",
		"database.ssl_mode":        "NOTIFY_DB_SSL_MODE",
		"database.max_conns":       "NOTIFY_DB_MAX_CONNS",
		"redis.addr":               "NOTIFY_REDIS_ADDR",
		"redis.password":           "NOTIFY_REDIS_PASSWORD",
		"provider.webhook_url":     "NOTIFY_PROVIDER_WEBHOOK_URL",
		"provider.timeout":         "NOTIFY_PROVIDER_TIMEOUT",
		"worker.concurrency":       "NOTIFY_WORKER_CONCURRENCY",
		"worker.rate_limit_per_sec": "NOTIFY_WORKER_RATE_LIMIT",
		"worker.max_retries":       "NOTIFY_WORKER_MAX_RETRIES",
		"tracing.enabled":          "NOTIFY_TRACING_ENABLED",
		"tracing.endpoint":         "NOTIFY_TRACING_ENDPOINT",
	}

	for key, env := range bindEnvs {
		_ = viper.BindEnv(key, env)
	}

	// Try reading config file (optional)
	_ = viper.ReadInConfig()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
