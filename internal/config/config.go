package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	ServiceName string `mapstructure:"service_name"`
	Env         string `mapstructure:"env"`

	Server struct {
		HTTPPort    int           `mapstructure:"http_port"`
		HTTPTimeout time.Duration `mapstructure:"http_timeout"`
	}

	Database struct {
		Username string `mapstructure:"username"`
		Password string `mapstructure:"password"`
		Database string `mapstructure:"database"`
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
		SSLMode  string `mapstructure:"ssl_mode"`
	}

	Worker struct {
		PollInterval    time.Duration `mapstructure:"poll_interval"`
		Workers         int           `mapstructure:"workers"`
		QueueSize       int           `mapstructure:"queue_size"`
		ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	}

	MaxAttempts int `mapstructure:"max_attempts"`
}

func NewConfig() (*Config, error) {
	_ = godotenv.Load()

	viper.AutomaticEnv()

	viper.SetEnvKeyReplacer(
		strings.NewReplacer(".", "_"),
	)

	viper.BindEnv("server.http_port", "SERVER_HTTP_PORT")
	viper.BindEnv("server.http_timeout", "SERVER_HTTP_TIMEOUT")

	viper.BindEnv("service_name", "SERVICE_NAME")
	viper.BindEnv("env", "ENV")
	viper.SetDefault("env", "local")

	viper.BindEnv("database.host", "DB_SOURCE_HOST")
	viper.BindEnv("database.port", "DB_SOURCE_PORT")
	viper.BindEnv("database.database", "DB_SOURCE_DATABASE")
	viper.BindEnv("database.username", "DB_SOURCE_USERNAME")
	viper.BindEnv("database.password", "DB_SOURCE_PASSWORD")
	viper.BindEnv("database.ssl_mode", "DB_SOURCE_SSL_MODE")

	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.ssl_mode", "disable")

	viper.BindEnv("max_attempts", "MAX_ATTEMPTS")
	viper.SetDefault("max_attempts", 5)

	viper.BindEnv("worker.poll_interval", "WORKER_POLL_INTERVAL")
	viper.SetDefault("worker.poll_interval", "5s")

	viper.BindEnv("worker.workers", "WORKER_COUNT")
	viper.SetDefault("worker.workers", 4)

	viper.BindEnv("worker.queue_size", "WORKER_QUEUE_SIZE")
	viper.SetDefault("worker.queue_size", 8)

	viper.BindEnv("worker.shutdown_timeout", "WORKER_SHUTDOWN_TIMEOUT")
	viper.SetDefault("worker.shutdown_timeout", "10s")

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &cfg, nil
}
