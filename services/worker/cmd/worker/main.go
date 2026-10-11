package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/majdkh733-dev/SDN/internal/config"
	"github.com/majdkh733-dev/SDN/internal/migrate"
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
	id := os.Getenv("WORKER_ID")
	if id == "" {
		id = "worker-1"
	}
	slog.Info("worker loop", "id", id)
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
