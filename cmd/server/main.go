package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/NUS-MTech-Capstone/ddm-agent/internal/config"
	"github.com/NUS-MTech-Capstone/ddm-agent/internal/execution"
	"github.com/NUS-MTech-Capstone/ddm-agent/internal/httpapi"
	"github.com/NUS-MTech-Capstone/ddm-agent/internal/observability"
	"github.com/NUS-MTech-Capstone/ddm-agent/internal/service"
	"github.com/NUS-MTech-Capstone/ddm-agent/internal/sts"
)

func main() {
	observability.ConfigureLogger("ddm-agent")
	cfg := config.Load()
	timeoutContext := func(ctx context.Context) (context.Context, context.CancelFunc) {
		return context.WithTimeout(ctx, cfg.CommandTimeout)
	}
	svc := service.New(
		sts.New(cfg.STSURL, cfg.STSTimeout),
		execution.NewRunner(cfg.Shell, cfg.MaxOutputBytes),
		timeoutContext,
		cfg.MaxCommandBytes,
		cfg.HostID,
	)
	addr := ":" + cfg.Port
	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.New(svc, cfg.MaxCommandBytes),
		ReadHeaderTimeout: 5 * time.Second,
	}
	slog.Info("server_listening", "addr", addr, "shell", cfg.Shell)
	if err := server.ListenAndServe(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
