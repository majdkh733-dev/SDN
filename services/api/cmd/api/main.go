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
	"github.com/majdkh733-dev/SDN/internal/orchestration"
	"github.com/majdkh733-dev/SDN/internal/secrets"
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
		srv.Secrets = mgr
		if cfg.TemporalAddress != "" && !cfg.InlineWorkflows {
			temporalClient, err := orchestration.Dial(cfg.TemporalAddress, cfg.TemporalNamespace)
			if err != nil {
				slog.Error("temporal", "err", err)
				os.Exit(1)
			}
			defer temporalClient.Close()
			srv.Workflows = temporalClient
			slog.Info("temporal client configured", "address", cfg.TemporalAddress, "namespace", cfg.TemporalNamespace)
		} else if cfg.TemporalAddress != "" {
			slog.Warn("TEMPORAL_ADDRESS is set and INLINE_WORKFLOWS is true; execution stays in the API process")
		} else {
			slog.Info("workflow driver", "driver", "postgresql")
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
