package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config holds all media-gateway configuration.
type Config struct {
	Listen  string       `env:"MG_LISTEN"          envDefault:":8190"`
	Server  ServerConfig `envPrefix:"MG_SERVER_"`
	S3      S3Config     `envPrefix:"MG_S3_"`
	FFmpeg  FFmpegConfig `envPrefix:"MG_FFMPEG_"`
	Task    TaskConfig   `envPrefix:"MG_TASK_"`
	Log     LogConfig    `envPrefix:"MG_"`
	Auth    AuthConfig   `envPrefix:"MG_AUTH_"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	ReadTimeout           int `env:"READ_TIMEOUT"   envDefault:"60"`
	WriteTimeout          int `env:"WRITE_TIMEOUT"   envDefault:"330"`
	IdleTimeout           int `env:"IDLE_TIMEOUT"    envDefault:"120"`
	MaxConcurrentRequests int `env:"MAX_CONCURRENT"  envDefault:"0"`
}

// S3Config holds default S3 storage configuration.
type S3Config struct {
	Endpoint       string `env:"ENDPOINT"`
	AccessKey      string `env:"ACCESS_KEY"`
	SecretKey      string `env:"SECRET_KEY"`
	Bucket         string `env:"BUCKET"`
	Region         string `env:"REGION"          envDefault:"auto"`
	ForcePathStyle bool   `env:"FORCE_PATH_STYLE" envDefault:"false"`
}

// FFmpegConfig holds ffmpeg/ffprobe configuration.
type FFmpegConfig struct {
	FFprobePath    string `env:"FFPROBE_PATH"      envDefault:"ffprobe"`
	FFmpegPath     string `env:"FFMPEG_PATH"       envDefault:"ffmpeg"`
	MaxSourceSize  int64  `env:"MAX_SOURCE_SIZE"   envDefault:"524288000"` // 500MB
	TempDir        string `env:"TEMP_DIR"          envDefault:"/tmp/media-gateway"`
	ConvertTimeout int    `env:"CONVERT_TIMEOUT"   envDefault:"300"`
}

// TaskConfig holds worker pool configuration.
type TaskConfig struct {
	MaxConcurrency int `env:"MAX_CONCURRENCY"   envDefault:"4"`
	QueueSize      int `env:"QUEUE_SIZE"        envDefault:"100"`
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level string `env:"LOG_LEVEL"   envDefault:"info"`
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	Secret string `env:"SECRET"`
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parsing environment variables: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

// Validate checks configuration for consistency.
func (c *Config) Validate() error {
	if c.Listen == "" {
		return fmt.Errorf("listen address is required")
	}
	if c.Task.MaxConcurrency < 1 {
		return fmt.Errorf("task max_concurrency must be >= 1")
	}
	if c.Task.QueueSize < 1 {
		return fmt.Errorf("task queue_size must be >= 1")
	}
	if c.FFmpeg.MaxSourceSize < 1 {
		return fmt.Errorf("ffmpeg max_source_size must be >= 1")
	}
	if c.FFmpeg.ConvertTimeout < 1 {
		return fmt.Errorf("ffmpeg convert_timeout must be >= 1")
	}
	return nil
}
