package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAgentEnrollAndDiscover(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	defer ln.Close()

	var completed []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/agent/enroll":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"data":{"credential":"cred-1","connector_id":"c","site_id":"s"}}`))
		case "/api/v1/agent/heartbeat":
			if r.Header.Get("Authorization") != "Bearer cred-1" {
				http.Error(w, "no", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"status":"ok"}}`))
		case "/api/v1/agent/tasks":
			payload, _ := json.Marshal(map[string]any{
				"targets": []string{"127.0.0.1"}, "exclusions": []string{},
				"timeout_ms": 300, "concurrency": 1, "ports": []int{port},
			})
			raw, _ := json.Marshal(map[string]any{"data": map[string]any{"found": true, "id": "job-1", "kind": "discovery", "payload": json.RawMessage(payload)}})
			_, _ = w.Write(raw)
		default:
			body, _ := io.ReadAll(r.Body)
			completed = body
			_, _ = w.Write([]byte(`{"data":{"ok":true}}`))
		}
	}))
	defer srv.Close()

	loop := newLoop(srv.URL, "", t.TempDir()+"/credential")
	if err := loop.enroll(context.Background(), "one-time"); err != nil {
		t.Fatal(err)
	}
	if loop.token != "cred-1" {
		t.Fatalf("token %s", loop.token)
	}
	if err := loop.once(context.Background()); err != nil {
		t.Fatal(err)
	}
	var body struct {
		Observations []struct {
			IP        string `json:"ip"`
			Reachable bool   `json:"reachable"`
			OSVersion string `json:"os_version"`
		} `json:"observations"`
	}
	if err := json.Unmarshal(completed, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Observations) != 1 || !body.Observations[0].Reachable || body.Observations[0].OSVersion != "" {
		t.Fatalf("observations %#v", body.Observations)
	}
	_ = time.Second
}
