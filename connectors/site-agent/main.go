package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/majdkh733-dev/SDN/internal/adapter"
)

func main() {
	base := os.Getenv("SDN_API_URL")
	token := os.Getenv("SDN_AGENT_TOKEN")
	siteName := os.Getenv("SDN_SITE_NAME")
	if base == "" || token == "" {
		slog.Error("SDN_API_URL and SDN_AGENT_TOKEN are required")
		os.Exit(1)
	}
	client := &http.Client{Timeout: 20 * time.Second}
	slog.Info("site agent started", "api", base)
	for {
		if err := post(client, base+"/api/v1/agent/heartbeat", token, map[string]string{"health": "healthy"}); err != nil {
			slog.Error("heartbeat", "err", err)
		}
		body, err := get(client, base+"/api/v1/agent/tasks", token)
		if err != nil {
			slog.Error("lease", "err", err)
			time.Sleep(3 * time.Second)
			continue
		}
		var envelope struct {
			Data struct {
				Found   bool            `json:"found"`
				ID      string          `json:"id"`
				Kind    string          `json:"kind"`
				Payload json.RawMessage `json:"payload"`
			} `json:"data"`
		}
		_ = json.Unmarshal(body, &envelope)
		if envelope.Data.Found && envelope.Data.Kind == "discover" {
			var payload struct {
				SiteName string `json:"site_name"`
			}
			_ = json.Unmarshal(envelope.Data.Payload, &payload)
			if payload.SiteName == "" {
				payload.SiteName = siteName
			}
			discovery := adapter.SimulatorDiscovery(payload.SiteName)
			err = post(client, base+"/api/v1/agent/tasks/"+envelope.Data.ID+"/complete", token, map[string]any{"ok": true, "discovery": discovery})
			if err != nil {
				slog.Error("complete", "err", err)
			}
		}
		time.Sleep(3 * time.Second)
	}
}

func get(client *http.Client, url, token string) ([]byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d", res.StatusCode)
	}
	return body, nil
}

func post(client *http.Client, url, token string, payload any) error {
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("status %d: %s", res.StatusCode, body)
	}
	return nil
}
