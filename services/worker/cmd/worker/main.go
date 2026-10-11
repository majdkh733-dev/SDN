package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/majdkh733-dev/SDN/internal/config"
	"github.com/majdkh733-dev/SDN/internal/migrate"
	"github.com/majdkh733-dev/SDN/internal/orchestration"
	"github.com/majdkh733-dev/SDN/internal/secrets"
	"github.com/majdkh733-dev/SDN/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration", "err", err)
		os.Exit(1)
	}
	ctx := context.Background()
	st, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database", "err", err)
		os.Exit(1)
	}
	defer st.Close()
	if err := migrate.Apply(ctx, st.Pool()); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}
	mgr, err := secrets.Open(secrets.Options{
		Backend: cfg.SecretBackend, FileDir: cfg.SecretFileDir,
		VaultAddr: cfg.VaultAddr, VaultToken: cfg.VaultToken, VaultMount: cfg.VaultMount,
	})
	if err != nil {
		slog.Error("secrets", "err", err)
		os.Exit(1)
	}
	if pinger, ok := mgr.(interface{ Ping(context.Context) error }); ok {
		if err := pinger.Ping(ctx); err != nil {
			slog.Error("secrets", "err", err)
			os.Exit(1)
		}
	}
	slog.Info("secrets", "backend", cfg.SecretBackend)
	if cfg.TemporalAddress != "" {
		slog.Info("temporal worker", "address", cfg.TemporalAddress, "namespace", cfg.TemporalNamespace)
		if err := orchestration.RunWorker(ctx, cfg.TemporalAddress, cfg.TemporalNamespace, st.RunChange); err != nil {
			slog.Error("temporal", "err", err)
			os.Exit(1)
		}
		return
	}
	id := os.Getenv("WORKER_ID")
	if id == "" {
		id = "worker-1"
	}
	slog.Info("postgresql worker", "id", id, "temporal", "not_configured")
	for {
		if err := st.Beat(ctx, id, "workflow"); err != nil {
			slog.Error("heartbeat", "err", err)
		}
		claimed, err := st.ClaimedChanges(ctx)
		if err != nil {
			slog.Error("claim", "err", err)
		}
		for _, pair := range claimed {
			if err := st.RunChange(ctx, pair[0], pair[1]); err != nil {
				slog.Error("run", "change", pair[1], "err", err)
			}
		}
		time.Sleep(2 * time.Second)
	}
}
