package config

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	GRPCAddr      string
	HTTPAddr      string
	APIAddr       string
	DBPath        string
	RingSize      int
	RetentionDays int
	PushURL       string
	PushInterval  int
}

func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		GRPCAddr:      "0.0.0.0:4317",
		HTTPAddr:      "0.0.0.0:4318",
		APIAddr:       "0.0.0.0:4320",
		DBPath:        filepath.Join(home, ".lookout", "traces.db"),
		RingSize:      10000,
		RetentionDays: 7,
		PushURL:       "",
		PushInterval:  60,
	}
}

func (c *Config) ApplyEnv() {
	if v := os.Getenv("LOOKOUT_GRPC_ADDR"); v != "" {
		c.GRPCAddr = v
	}
	if v := os.Getenv("LOOKOUT_HTTP_ADDR"); v != "" {
		c.HTTPAddr = v
	}
	if v := os.Getenv("LOOKOUT_API_ADDR"); v != "" {
		c.APIAddr = v
	}
	if v := os.Getenv("LOOKOUT_DB_PATH"); v != "" {
		c.DBPath = v
	}
	if v := os.Getenv("LOOKOUT_RING_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.RingSize = n
		}
	}
	if v := os.Getenv("LOOKOUT_RETENTION_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.RetentionDays = n
		}
	}
	if v := os.Getenv("LOOKOUT_PUSH_URL"); v != "" {
		c.PushURL = v
	}
	if v := os.Getenv("LOOKOUT_PUSH_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.PushInterval = n
		}
	}
}
