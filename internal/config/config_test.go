package config

import (
	"testing"
)

func validConfig() *Config {
	return &Config{
		Listen: ":8190",
		Task: TaskConfig{
			MaxConcurrency: 4,
			QueueSize:      100,
		},
		FFmpeg: FFmpegConfig{
			MaxSourceSize:  524288000,
			ConvertTimeout: 300,
		},
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := validConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("valid config should pass: %v", err)
	}
}

func TestValidate_EmptyListen(t *testing.T) {
	cfg := validConfig()
	cfg.Listen = ""
	if err := cfg.Validate(); err == nil {
		t.Error("empty listen should fail")
	}
}

func TestValidate_ZeroConcurrency(t *testing.T) {
	cfg := validConfig()
	cfg.Task.MaxConcurrency = 0
	if err := cfg.Validate(); err == nil {
		t.Error("zero max concurrency should fail")
	}
}

func TestValidate_NegativeConcurrency(t *testing.T) {
	cfg := validConfig()
	cfg.Task.MaxConcurrency = -1
	if err := cfg.Validate(); err == nil {
		t.Error("negative max concurrency should fail")
	}
}

func TestValidate_ZeroQueueSize(t *testing.T) {
	cfg := validConfig()
	cfg.Task.QueueSize = 0
	if err := cfg.Validate(); err == nil {
		t.Error("zero queue size should fail")
	}
}

func TestValidate_ZeroMaxSourceSize(t *testing.T) {
	cfg := validConfig()
	cfg.FFmpeg.MaxSourceSize = 0
	if err := cfg.Validate(); err == nil {
		t.Error("zero max source size should fail")
	}
}

func TestValidate_ZeroConvertTimeout(t *testing.T) {
	cfg := validConfig()
	cfg.FFmpeg.ConvertTimeout = 0
	if err := cfg.Validate(); err == nil {
		t.Error("zero convert timeout should fail")
	}
}

func TestValidate_MinimalValid(t *testing.T) {
	cfg := &Config{
		Listen: ":1",
		Task:   TaskConfig{MaxConcurrency: 1, QueueSize: 1},
		FFmpeg: FFmpegConfig{MaxSourceSize: 1, ConvertTimeout: 1},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("minimal valid config should pass: %v", err)
	}
}
