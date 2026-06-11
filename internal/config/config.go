package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port            string
	STSURL          string
	STSTimeout      time.Duration
	CommandTimeout  time.Duration
	Shell           string
	HostID          string
	MaxCommandBytes int
	MaxOutputBytes  int
}

func Load() Config {
	return Config{
		Port:            env("PORT", "8084"),
		STSURL:          env("STS_URL", "http://localhost:8082"),
		STSTimeout:      durationEnv("STS_TIMEOUT", 5*time.Second),
		CommandTimeout:  durationEnv("COMMAND_TIMEOUT", 30*time.Second),
		Shell:           env("SHELL_PATH", "/bin/sh"),
		HostID:          os.Getenv("DDM_AGENT_HOST_ID"),
		MaxCommandBytes: intEnv("MAX_COMMAND_BYTES", 16*1024),
		MaxOutputBytes:  intEnv("MAX_OUTPUT_BYTES", 1024*1024),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(env(key, fallback.String()))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func intEnv(key string, fallback int) int {
	value, err := strconv.Atoi(env(key, strconv.Itoa(fallback)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
