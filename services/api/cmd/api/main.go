package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/majdkh733-dev/SDN/internal/config"
	"github.com/majdkh733-dev/SDN/internal/httpserver"
	"github.com/majdkh733-dev/SDN/internal/migrate"
	"github.com/majdkh733-dev/SDN/internal/seed"
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
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "migrate":
		return
	case "seed":
		if err := seed.Run(ctx, st, cfg.DevPassword); err != nil {
			slog.Error("seed", "err", err)
			os.Exit(1)
		}
	default:
		srv, err := httpserver.New(cfg, st, slog.Default())
		if err != nil {
			slog.Error("server", "err", err)
			os.Exit(1)
		}
		httpServer := &http.Server{
			Addr: cfg.HTTPAddr, Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second,
		}
		slog.Info("api listening", "addr", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen", "err", err)
			os.Exit(1)
		}
	}
}
