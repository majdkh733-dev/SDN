package main

import (
	"context"
	"log/slog"
	"os"
	"time"
)

func main() {
	base := os.Getenv("SDN_API_URL")
	token := os.Getenv("SDN_AGENT_TOKEN")
	enrollment := os.Getenv("SDN_ENROLLMENT_TOKEN")
	state := os.Getenv("SDN_AGENT_STATE")
	if base == "" {
		slog.Error("SDN_API_URL is required")
		os.Exit(1)
	}
	if token == "" && state != "" {
		if raw, err := os.ReadFile(state); err == nil {
			token = string(raw)
		}
	}
	loop := newLoop(base, token, state)
	ctx := context.Background()
	if loop.token == "" {
		if enrollment == "" {
			slog.Error("SDN_AGENT_TOKEN or SDN_ENROLLMENT_TOKEN is required")
			os.Exit(1)
		}
		if err := loop.enroll(ctx, enrollment); err != nil {
			slog.Error("enrollment failed")
			os.Exit(1)
		}
		slog.Info("connector enrolled")
	}
	delay := time.Second
	for {
		if err := loop.once(ctx); err != nil {
			slog.Error("connector poll failed")
			loop.sleep(delay)
			if delay < 30*time.Second {
				delay *= 2
			}
			continue
		}
		delay = time.Second
		loop.sleep(15 * time.Second)
	}
}
