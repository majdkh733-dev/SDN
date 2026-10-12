package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/majdkh733-dev/SDN/internal/discover"
)

const agentVersion = "0.2.0"

type agentLoop struct {
	base  string
	token string
	state string
	http  *http.Client
	sleep func(time.Duration)
}

func newLoop(base, token, state string) *agentLoop {
	return &agentLoop{
		base: base, token: token, state: state,
		http:  &http.Client{Timeout: 30 * time.Second},
		sleep: time.Sleep,
	}
}

func (a *agentLoop) enroll(ctx context.Context, enrollment string) error {
	host, _ := os.Hostname()
	body := map[string]any{
		"enrollment_token": enrollment,
		"version":          agentVersion,
		"host":             map[string]string{"hostname": host, "os": runtime.GOOS, "arch": runtime.GOARCH},
		"capabilities":     []string{"icmp", "tcp", "arp", "dns", "snmp"},
	}
	raw, status, err := a.request(ctx, http.MethodPost, "/api/v1/agent/enroll", "", body)
	if err != nil {
		return err
	}
	if status >= 300 {
		return fmt.Errorf("enrollment rejected")
	}
	var envelope struct {
		Data struct {
			Credential string `json:"credential"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	if envelope.Data.Credential == "" {
		return fmt.Errorf("enrollment returned no credential")
	}
	a.token = envelope.Data.Credential
	if a.state != "" {
		if err := os.WriteFile(a.state, []byte(a.token), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func (a *agentLoop) once(ctx context.Context) error {
	host, _ := os.Hostname()
	if err := a.post(ctx, "/api/v1/agent/heartbeat", map[string]any{
		"health": "healthy", "version": agentVersion,
		"host":         map[string]string{"hostname": host, "os": runtime.GOOS, "arch": runtime.GOARCH},
		"capabilities": []string{"icmp", "tcp", "arp", "dns", "snmp"},
	}); err != nil {
		return err
	}
	raw, status, err := a.request(ctx, http.MethodGet, "/api/v1/agent/tasks", a.token, nil)
	if err != nil {
		return err
	}
	if status >= 300 {
		return fmt.Errorf("task lease failed")
	}
	var envelope struct {
		Data struct {
			Found   bool            `json:"found"`
			ID      string          `json:"id"`
			Kind    string          `json:"kind"`
			Payload json.RawMessage `json:"payload"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	if !envelope.Data.Found {
		return nil
	}
	if envelope.Data.Kind == "diagnostic" {
		return a.runDiagnostic(ctx, envelope.Data.ID, envelope.Data.Payload)
	}
	if envelope.Data.Kind != "discovery" {
		return a.post(ctx, "/api/v1/agent/tasks/"+envelope.Data.ID+"/complete", map[string]any{"ok": false, "error": "unsupported task"})
	}
	return a.runDiscovery(ctx, envelope.Data.ID, envelope.Data.Payload)
}

func (a *agentLoop) runDiagnostic(ctx context.Context, id string, payload json.RawMessage) error {
	var spec struct {
		Kind   string `json:"kind"`
		Target string `json:"target"`
	}
	if err := json.Unmarshal(payload, &spec); err != nil {
		return a.post(ctx, "/api/v1/agent/tasks/"+id+"/complete", map[string]any{"ok": false, "error": "invalid diagnostic"})
	}
	switch spec.Kind {
	case "ping":
		result := discover.Ping(ctx, spec.Target, 2*time.Second)
		return a.post(ctx, "/api/v1/agent/tasks/"+id+"/complete", map[string]any{"ok": true, "result": result})
	case "dns":
		names, err := net.DefaultResolver.LookupAddr(ctx, spec.Target)
		if err != nil {
			return a.post(ctx, "/api/v1/agent/tasks/"+id+"/complete", map[string]any{"ok": true, "result": map[string]any{"ok": false, "detail": "no reverse name"}})
		}
		return a.post(ctx, "/api/v1/agent/tasks/"+id+"/complete", map[string]any{"ok": true, "result": map[string]any{"ok": true, "names": names}})
	default:
		return a.post(ctx, "/api/v1/agent/tasks/"+id+"/complete", map[string]any{"ok": false, "error": "unsupported diagnostic"})
	}
}

func (a *agentLoop) runDiscovery(ctx context.Context, id string, payload json.RawMessage) error {
	var spec struct {
		Targets     []string `json:"targets"`
		Exclusions  []string `json:"exclusions"`
		TimeoutMS   int      `json:"timeout_ms"`
		Concurrency int      `json:"concurrency"`
		Ports       []int    `json:"ports"`
		Community   string   `json:"snmp_community"`
	}
	if err := json.Unmarshal(payload, &spec); err != nil {
		return err
	}
	targets, err := discover.Expand(spec.Targets, spec.Exclusions)
	if err != nil {
		_ = a.post(ctx, "/api/v1/agent/tasks/"+id+"/complete", map[string]any{"ok": false, "error": err.Error()})
		return err
	}
	found := discover.Scan(ctx, targets, discover.Options{
		Timeout: time.Duration(spec.TimeoutMS) * time.Millisecond, Ports: spec.Ports,
		Concurrency: spec.Concurrency, Community: spec.Community,
	})
	return a.post(ctx, "/api/v1/agent/tasks/"+id+"/complete", map[string]any{
		"ok": true, "observations": found,
	})
}

func (a *agentLoop) post(ctx context.Context, path string, payload any) error {
	_, status, err := a.request(ctx, http.MethodPost, path, a.token, payload)
	if err != nil {
		return err
	}
	if status >= 300 {
		return fmt.Errorf("status %d", status)
	}
	return nil
}

func (a *agentLoop) request(ctx context.Context, method, path, token string, payload any) ([]byte, int, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.base+path, body)
	if err != nil {
		return nil, 0, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := a.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	return raw, res.StatusCode, nil
}
