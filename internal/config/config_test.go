package config

import (
	"testing"
	"time"
)

func TestLoadAppliesDefaultsAndParsesOverrides(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("STS_URL", "http://sts:8082")
	t.Setenv("STS_TIMEOUT", "2s")
	t.Setenv("COMMAND_TIMEOUT", "45s")
	t.Setenv("SHELL_PATH", "/bin/bash")
	t.Setenv("DDM_AGENT_HOST_ID", "host-1")
	t.Setenv("MAX_COMMAND_BYTES", "2048")
	t.Setenv("MAX_OUTPUT_BYTES", "4096")

	cfg := Load()
	if cfg.Port != "9090" || cfg.STSURL != "http://sts:8082" || cfg.Shell != "/bin/bash" || cfg.HostID != "host-1" {
		t.Fatalf("Load() = %#v", cfg)
	}
	if cfg.STSTimeout != 2*time.Second || cfg.CommandTimeout != 45*time.Second ||
		cfg.MaxCommandBytes != 2048 || cfg.MaxOutputBytes != 4096 {
		t.Fatalf("parsed values = %#v", cfg)
	}
}

func TestLoadFallsBackForInvalidDurationsAndSizes(t *testing.T) {
	t.Setenv("STS_TIMEOUT", "bad")
	t.Setenv("COMMAND_TIMEOUT", "0s")
	t.Setenv("MAX_COMMAND_BYTES", "-1")
	t.Setenv("MAX_OUTPUT_BYTES", "bad")

	cfg := Load()
	if cfg.Port != "8084" || cfg.STSURL != "http://localhost:8082" {
		t.Fatalf("defaults = %#v", cfg)
	}
	if cfg.STSTimeout != 5*time.Second || cfg.CommandTimeout != 30*time.Second ||
		cfg.MaxCommandBytes != 16*1024 || cfg.MaxOutputBytes != 1024*1024 {
		t.Fatalf("fallback values = %#v", cfg)
	}
}
