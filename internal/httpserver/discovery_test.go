package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/majdkh733-dev/SDN/internal/authn"
	"github.com/majdkh733-dev/SDN/internal/config"
	"github.com/majdkh733-dev/SDN/internal/migrate"
	"github.com/majdkh733-dev/SDN/internal/seed"
	"github.com/majdkh733-dev/SDN/internal/store"
)

func TestConnectorDiscoveryLifecycle(t *testing.T) {
	ctx := context.Background()
	url := "postgres://sdn:sdn@127.0.0.1:5432/sdn_test?sslmode=disable"
	st, err := store.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	if err := migrate.Apply(ctx, st.Pool()); err != nil {
		t.Fatal(err)
	}
	if err := resetDB(ctx, st); err != nil {
		t.Fatal(err)
	}
	if err := seed.Run(ctx, st, "test-password"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Environment: "test", DatabaseURL: url, AuthMode: "dev", TokenSecret: "test-secret",
		OIDCClientID: "sdn-web", OIDCClientSecret: "test-oidc", PublicAPIBase: "http://127.0.0.1:8080",
		AllowedOrigins: []string{"http://localhost:5173"}, OIDCRedirectURLs: []string{"http://localhost:5173/callback"},
		InlineWorkflows: true, SessionTTL: time.Hour,
	}
	srv, err := New(cfg, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv.Secrets = nil
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	admin := login(t, ts.URL, "admin@nexora.demo")
	other := login(t, ts.URL, "admin@isolated.demo")

	var site map[string]any
	postJSON(t, admin, ts.URL+"/api/v1/sites", map[string]string{"name": "Lab", "location": "Riyadh", "execution_domain": "lab"}, http.StatusCreated, &site)
	var conn map[string]any
	postJSON(t, admin, ts.URL+"/api/v1/connectors", map[string]string{
		"name": "lab-edge", "site_id": site["id"].(string), "adapter": "site", "deployment_method": "docker", "description": "Lab connector",
	}, http.StatusCreated, &conn)
	if conn["status"] != "not_enrolled" {
		t.Fatalf("status %#v", conn["status"])
	}

	var hidden []map[string]any
	getJSON(t, other, ts.URL+"/api/v1/connectors", &hidden)
	for _, item := range hidden {
		if item["name"] == "lab-edge" {
			t.Fatal("connector leaked across organizations")
		}
	}

	var enrolled map[string]string
	postJSON(t, admin, ts.URL+"/api/v1/connectors/"+conn["id"].(string)+"/enrollment", map[string]any{}, http.StatusCreated, &enrolled)
	pgConn, err := st.Pool().Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pgConn.Exec(ctx, `SELECT set_config('app.bypass', 'on', false)`); err != nil {
		t.Fatal(err)
	}
	tag, err := pgConn.Exec(ctx, `UPDATE enrollment_tokens SET expires_at = now() - interval '1 minute' WHERE token_hash = $1`, authn.Hash(enrolled["token"]))
	pgConn.Release()
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("expire token %v %v", tag, err)
	}
	expiredReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agent/enroll", bytes.NewReader([]byte(`{"enrollment_token":"`+enrolled["token"]+`"}`)))
	expiredReq.Header.Set("Content-Type", "application/json")
	expiredRes, err := http.DefaultClient.Do(expiredReq)
	if err != nil || expiredRes.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expired token status %v %v", expiredRes, err)
	}
	expiredRes.Body.Close()

	postJSON(t, admin, ts.URL+"/api/v1/connectors/"+conn["id"].(string)+"/enrollment", map[string]any{}, http.StatusCreated, &enrolled)
	var credential struct {
		Data struct {
			Credential string `json:"credential"`
		} `json:"data"`
	}
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agent/enroll", bytes.NewReader([]byte(`{"enrollment_token":"`+enrolled["token"]+`","version":"0.2.0"}`)))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(res.Body).Decode(&credential); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if credential.Data.Credential == "" {
		t.Fatal("missing credential")
	}
	reuse, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agent/enroll", bytes.NewReader([]byte(`{"enrollment_token":"`+enrolled["token"]+`"}`)))
	reuse.Header.Set("Content-Type", "application/json")
	reuseRes, err := http.DefaultClient.Do(reuse)
	if err != nil || reuseRes.StatusCode != http.StatusUnauthorized {
		t.Fatalf("reused token %v", reuseRes)
	}
	reuseRes.Body.Close()

	beat, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agent/heartbeat", bytes.NewReader([]byte(`{"health":"healthy","version":"0.2.0"}`)))
	beat.Header.Set("Authorization", "Bearer "+credential.Data.Credential)
	beat.Header.Set("Content-Type", "application/json")
	beatRes, err := http.DefaultClient.Do(beat)
	if err != nil || beatRes.StatusCode != http.StatusOK {
		t.Fatalf("heartbeat %v %v", beatRes, err)
	}
	beatRes.Body.Close()

	var profile map[string]any
	postJSON(t, admin, ts.URL+"/api/v1/sites/"+site["id"].(string)+"/discovery-profiles", map[string]any{
		"name": "lab", "connector_id": conn["id"], "targets": []string{"127.0.0.1"}, "exclusions": []string{},
		"scan_profile": "reachability", "timeout_ms": 200, "concurrency": 1,
	}, http.StatusCreated, &profile)
	postJSON(t, admin, ts.URL+"/api/v1/sites/"+site["id"].(string)+"/discovery-profiles", map[string]any{
		"name": "too-wide", "connector_id": conn["id"], "targets": []string{"10.0.0.0/16"},
	}, http.StatusBadRequest, nil)

	var job map[string]any
	postJSON(t, admin, ts.URL+"/api/v1/discovery-profiles/"+profile["id"].(string)+"/run", map[string]any{}, http.StatusAccepted, &job)
	leaseReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/agent/tasks", nil)
	leaseReq.Header.Set("Authorization", "Bearer "+credential.Data.Credential)
	leaseRes, err := http.DefaultClient.Do(leaseReq)
	if err != nil {
		t.Fatal(err)
	}
	var leased struct {
		Data struct {
			Found bool   `json:"found"`
			ID    string `json:"id"`
			Kind  string `json:"kind"`
		} `json:"data"`
	}
	if err := json.NewDecoder(leaseRes.Body).Decode(&leased); err != nil {
		t.Fatal(err)
	}
	leaseRes.Body.Close()
	if !leased.Data.Found || leased.Data.Kind != "discovery" || leased.Data.ID != job["id"] {
		t.Fatalf("lease %#v", leased.Data)
	}
	obs := map[string]any{"ok": true, "observations": []map[string]any{{
		"ip": "127.0.0.1", "hostname": "lab-sw", "mac": "aa:bb:cc:dd:ee:01", "reachable": true,
		"methods": []map[string]any{{"method": "tcp", "ok": true, "port": 22}}, "confidence": "low", "explicit": true,
	}}}
	raw, _ := json.Marshal(obs)
	complete := func() *http.Response {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agent/tasks/"+leased.Data.ID+"/complete", bytes.NewReader(raw))
		req.Header.Set("Authorization", "Bearer "+credential.Data.Credential)
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	first := complete()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("complete %d", first.StatusCode)
	}
	first.Body.Close()
	second := complete()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("duplicate complete %d", second.StatusCode)
	}
	second.Body.Close()
	var observations []map[string]any
	getJSON(t, admin, ts.URL+"/api/v1/discovery-jobs/"+job["id"].(string)+"/observations", &observations)
	if len(observations) != 1 {
		t.Fatalf("observations %d", len(observations))
	}
	var imported []map[string]any
	postJSON(t, admin, ts.URL+"/api/v1/discovery-jobs/"+job["id"].(string)+"/import", map[string]any{"all": true}, http.StatusOK, &imported)
	postJSON(t, admin, ts.URL+"/api/v1/discovery-jobs/"+job["id"].(string)+"/import", map[string]any{"all": true}, http.StatusOK, &imported)
	var devices []map[string]any
	getJSON(t, admin, ts.URL+"/api/v1/devices?q=lab-sw", &devices)
	if len(devices) != 1 || devices[0]["data_origin"] != "discovery" {
		t.Fatalf("devices %#v", devices)
	}
	deviceID := devices[0]["id"].(string)
	var renamed map[string]any
	postJSON(t, admin, ts.URL+"/api/v1/devices/"+deviceID, map[string]any{"name": "Manual Name", "lock": []string{"name"}}, http.StatusOK, &renamed)

	var again map[string]any
	postJSON(t, admin, ts.URL+"/api/v1/discovery-profiles/"+profile["id"].(string)+"/run", map[string]any{}, http.StatusAccepted, &again)
	leaseReq, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/v1/agent/tasks", nil)
	leaseReq.Header.Set("Authorization", "Bearer "+credential.Data.Credential)
	leaseRes, _ = http.DefaultClient.Do(leaseReq)
	_ = json.NewDecoder(leaseRes.Body).Decode(&leased)
	leaseRes.Body.Close()
	raw, _ = json.Marshal(map[string]any{"ok": true, "observations": []map[string]any{{
		"ip": "127.0.0.1", "hostname": "renamed-by-scan", "mac": "aa:bb:cc:dd:ee:01", "reachable": true, "explicit": true,
	}}})
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agent/tasks/"+leased.Data.ID+"/complete", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+credential.Data.Credential)
	req.Header.Set("Content-Type", "application/json")
	res, _ = http.DefaultClient.Do(req)
	res.Body.Close()
	postJSON(t, admin, ts.URL+"/api/v1/discovery-jobs/"+again["id"].(string)+"/import", map[string]any{"all": true}, http.StatusOK, &imported)
	getJSON(t, admin, ts.URL+"/api/v1/devices?q=Manual", &devices)
	if len(devices) != 1 || devices[0]["name"] != "Manual Name" {
		t.Fatalf("manual name was overwritten: %#v", devices)
	}

	var revoked map[string]any
	postJSON(t, admin, ts.URL+"/api/v1/connectors/"+conn["id"].(string)+"/revoke", map[string]any{}, http.StatusOK, &revoked)
	beat, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agent/heartbeat", bytes.NewReader([]byte(`{"health":"healthy"}`)))
	beat.Header.Set("Authorization", "Bearer "+credential.Data.Credential)
	beat.Header.Set("Content-Type", "application/json")
	beatRes, _ = http.DefaultClient.Do(beat)
	if beatRes.StatusCode != http.StatusUnauthorized && beatRes.StatusCode != http.StatusNotFound {
		t.Fatalf("revoked credential still accepted: %d", beatRes.StatusCode)
	}
	beatRes.Body.Close()
	getJSON(t, admin, ts.URL+"/api/v1/devices?q=Manual", &devices)
	if devices[0]["health"] == "down" {
		t.Fatal("revoking a connector marked the device down")
	}

	postJSON(t, admin, ts.URL+"/api/v1/preferences/navigation", map[string]any{"items": []any{}}, http.StatusBadRequest, nil)
	var prefs map[string]any
	postJSON(t, admin, ts.URL+"/api/v1/preferences/navigation/reset", map[string]any{}, http.StatusOK, &prefs)
	postJSON(t, admin, ts.URL+"/api/v1/preferences/theme", map[string]any{"preset": "navy", "mode": "dark"}, http.StatusOK, &prefs)
	postJSON(t, admin, ts.URL+"/api/v1/preferences/theme", map[string]any{
		"preset": "custom", "mode": "light", "primary": "#ffffff", "accent": "#ffffff",
		"sidebar": "#ffffff", "surface": "#ffffff", "border": "#ffffff", "text": "#ffffff", "bg": "#ffffff",
	}, http.StatusBadRequest, nil)
}
