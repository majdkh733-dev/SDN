package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/majdkh733-dev/SDN/internal/config"
	"github.com/majdkh733-dev/SDN/internal/migrate"
	"github.com/majdkh733-dev/SDN/internal/seed"
	"github.com/majdkh733-dev/SDN/internal/store"
)

func TestPlatformJourneys(t *testing.T) {
	ctx := context.Background()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://sdn:sdn@127.0.0.1:5432/sdn_test?sslmode=disable"
	}
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
		Environment: "test", DatabaseURL: url, HTTPAddr: ":0", AuthMode: "dev",
		TokenSecret: "test-secret", OIDCClientID: "sdn-web", OIDCClientSecret: "test-oidc",
		PublicAPIBase: "http://api.test", AllowedOrigins: []string{"http://localhost:5173"},
		OIDCRedirectURLs: []string{"http://localhost:5173/callback"}, InlineWorkflows: true, SessionTTL: time.Hour,
	}
	srv, err := New(cfg, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	eng := login(t, ts.URL, "engineer@nexora.demo")
	iso := login(t, ts.URL, "admin@isolated.demo")
	approver := login(t, ts.URL, "approver@nexora.demo")
	recovery := login(t, ts.URL, "recovery@nexora.demo")

	var devices []map[string]any
	getJSON(t, eng, ts.URL+"/api/v1/devices", &devices)
	if len(devices) < 2 {
		t.Fatalf("expected seeded devices, got %d", len(devices))
	}
	var privateDevices []map[string]any
	getJSON(t, iso, ts.URL+"/api/v1/devices", &privateDevices)
	if len(privateDevices) == 0 {
		t.Fatal("isolated org has no devices")
	}
	res := do(t, eng, http.MethodGet, ts.URL+"/api/v1/devices/"+privateDevices[0]["id"].(string), nil)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-org device status %d", res.StatusCode)
	}

	var topo map[string]any
	getJSON(t, eng, ts.URL+"/api/v1/topology?lens=performance", &topo)
	if topo["notice"] == "" {
		t.Fatal("topology notice missing")
	}
	getJSON(t, eng, ts.URL+"/api/v1/devices/"+devices[0]["id"].(string)+"/interfaces", &[]any{})

	var segments []map[string]any
	getJSON(t, eng, ts.URL+"/api/v1/segments?tenant_id=", &segments)
	finance := map[string]string{}
	var tenantID string
	for _, seg := range segments {
		if seg["name"] == "finance-users" || seg["name"] == "finance-apps" {
			finance[seg["name"].(string)] = seg["id"].(string)
			tenantID = seg["tenant_id"].(string)
		}
	}
	var sites []map[string]any
	getJSON(t, eng, ts.URL+"/api/v1/sites", &sites)
	var siteID string
	for _, site := range sites {
		if site["name"] == "Riyadh DC1" {
			siteID = site["id"].(string)
		}
	}
	spec := map[string]any{
		"name": "finance-https", "tenant_id": tenantID,
		"source":      map[string]string{"type": "segment", "id": finance["finance-users"]},
		"destination": map[string]string{"type": "segment", "id": finance["finance-apps"]},
		"protocol":    "tcp", "ports": "443", "action": "allow", "site_ids": []string{siteID},
	}
	var policy map[string]any
	postJSON(t, eng, ts.URL+"/api/v1/policies", spec, http.StatusCreated, &policy)
	var preview map[string]any
	postJSON(t, eng, ts.URL+"/api/v1/policies/"+policy["id"].(string)+"/preview", map[string]any{}, http.StatusOK, &preview)
	if preview["plan_hash"] == "" || preview["preview_token"] == "" {
		t.Fatalf("preview incomplete: %#v", preview)
	}
	var change map[string]any
	postJSON(t, eng, ts.URL+"/api/v1/changes", map[string]any{
		"policy_version_id": preview["policy_version_id"], "plan_hash": preview["plan_hash"], "preview_token": preview["preview_token"],
	}, http.StatusCreated, &change)
	res = do(t, eng, http.MethodPost, ts.URL+"/api/v1/changes/"+change["id"].(string)+"/approve", map[string]any{"decision": "approved"})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("self approval status %d", res.StatusCode)
	}
	postJSON(t, approver, ts.URL+"/api/v1/changes/"+change["id"].(string)+"/approve", map[string]any{"decision": "approved"}, http.StatusOK, &change)
	res = do(t, eng, http.MethodPost, ts.URL+"/api/v1/changes/"+change["id"].(string)+"/execute", map[string]any{"confirm": false, "plan_hash": change["plan_hash"]})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("execute without confirm %d", res.StatusCode)
	}
	postJSON(t, eng, ts.URL+"/api/v1/changes/"+change["id"].(string)+"/execute", map[string]any{"confirm": true, "plan_hash": change["plan_hash"]}, http.StatusAccepted, &change)
	if change["status"] != "succeeded" {
		t.Fatalf("status %v detail %v steps %v", change["status"], change["error_detail"], change["steps"])
	}
	for _, raw := range change["steps"].([]any) {
		step := raw.(map[string]any)
		if step["status"] == "pending" {
			t.Fatalf("pending step after success: %v", step["operation_key"])
		}
	}

	spec["name"] = "finance-https-v2"
	var policy2 map[string]any
	postJSON(t, eng, ts.URL+"/api/v1/policies", spec, http.StatusCreated, &policy2)
	var preview2 map[string]any
	postJSON(t, eng, ts.URL+"/api/v1/policies/"+policy2["id"].(string)+"/preview", map[string]any{}, http.StatusOK, &preview2)
	spec["ports"] = "8443"
	postJSON(t, eng, ts.URL+"/api/v1/policies/"+policy2["id"].(string)+"/versions", spec, http.StatusCreated, &policy2)
	res = do(t, eng, http.MethodPost, ts.URL+"/api/v1/changes", map[string]any{
		"policy_version_id": preview2["policy_version_id"], "plan_hash": preview2["plan_hash"], "preview_token": preview2["preview_token"],
	})
	if res.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("stale preview status %d body %s", res.StatusCode, body)
	}

	postJSON(t, eng, ts.URL+"/api/v1/devices/"+devices[0]["id"].(string)+"/demo-faults", map[string]any{"fail_next_rollback": true}, http.StatusOK, &map[string]any{})
	postJSON(t, eng, ts.URL+"/api/v1/devices/"+devices[1]["id"].(string)+"/demo-faults", map[string]any{"fail_next_write": true}, http.StatusOK, &map[string]any{})
	spec["name"] = "finance-rollback"
	spec["ports"] = "443"
	var policy3 map[string]any
	postJSON(t, eng, ts.URL+"/api/v1/policies", spec, http.StatusCreated, &policy3)
	var preview3 map[string]any
	postJSON(t, eng, ts.URL+"/api/v1/policies/"+policy3["id"].(string)+"/preview", map[string]any{}, http.StatusOK, &preview3)
	var change3 map[string]any
	postJSON(t, eng, ts.URL+"/api/v1/changes", map[string]any{
		"policy_version_id": preview3["policy_version_id"], "plan_hash": preview3["plan_hash"], "preview_token": preview3["preview_token"],
	}, http.StatusCreated, &change3)
	postJSON(t, approver, ts.URL+"/api/v1/changes/"+change3["id"].(string)+"/approve", map[string]any{"decision": "approved"}, http.StatusOK, &change3)
	postJSON(t, eng, ts.URL+"/api/v1/changes/"+change3["id"].(string)+"/execute", map[string]any{"confirm": true, "plan_hash": change3["plan_hash"]}, http.StatusAccepted, &change3)
	if change3["status"] != "rollback_failed" {
		t.Fatalf("expected rollback_failed, got %v %v", change3["status"], change3["error_detail"])
	}

	var backups []map[string]any
	getJSON(t, eng, ts.URL+"/api/v1/backups", &backups)
	if len(backups) == 0 {
		t.Fatal("expected backups")
	}
	var restore map[string]any
	postJSON(t, recovery, ts.URL+"/api/v1/backups/"+backups[0]["id"].(string)+"/restore", map[string]any{}, http.StatusAccepted, &restore)
	if restore["status"] != "pending_reconciliation" {
		t.Fatalf("restore %v", restore)
	}
	var drift []any
	getJSON(t, eng, ts.URL+"/api/v1/drift-findings", &drift)
	if len(drift) == 0 {
		t.Fatal("expected seeded drift")
	}
	var audit []any
	getJSON(t, eng, ts.URL+"/api/v1/audit-events", &audit)
	if len(audit) == 0 {
		t.Fatal("expected audit")
	}
}

func TestSiteAgentDiscover(t *testing.T) {
	ctx := context.Background()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://sdn:sdn@127.0.0.1:5432/sdn_test?sslmode=disable"
	}
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
		OIDCClientID: "sdn-web", OIDCClientSecret: "test-oidc", PublicAPIBase: "http://api.test",
		AllowedOrigins: []string{"http://localhost:5173"}, OIDCRedirectURLs: []string{"http://localhost:5173/callback"},
		InlineWorkflows: true, SessionTTL: time.Hour,
	}
	srv, err := New(cfg, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	admin := login(t, ts.URL, "admin@nexora.demo")
	var site map[string]any
	postJSON(t, admin, ts.URL+"/api/v1/sites", map[string]string{"name": "Agent Site", "location": "Lab", "execution_domain": "agent"}, http.StatusCreated, &site)
	var conn map[string]any
	postJSON(t, admin, ts.URL+"/api/v1/connectors", map[string]string{"name": "agent-sim", "site_id": site["id"].(string), "adapter": "simulator"}, http.StatusCreated, &conn)
	var enrolled map[string]string
	postJSON(t, admin, ts.URL+"/api/v1/connectors/"+conn["id"].(string)+"/enrollment", map[string]any{}, http.StatusCreated, &enrolled)
	if enrolled["token"] == "" {
		t.Fatal("missing enrollment token")
	}
	var credential struct {
		Data struct {
			Credential string `json:"credential"`
		} `json:"data"`
	}
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agent/enroll", bytes.NewReader([]byte(`{"enrollment_token":"`+enrolled["token"]+`","version":"test"}`)))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(res.Body).Decode(&credential); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusCreated || credential.Data.Credential == "" {
		t.Fatalf("enroll status %d", res.StatusCode)
	}
	var queued map[string]any
	postJSON(t, admin, ts.URL+"/api/v1/connectors/"+conn["id"].(string)+"/discover?via=agent", map[string]any{}, http.StatusAccepted, &queued)
	if queued["task_id"] == "" {
		t.Fatal("task was not queued")
	}
	agent := &http.Client{}
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agent/heartbeat", bytes.NewReader([]byte(`{"health":"healthy"}`)))
	req.Header.Set("Authorization", "Bearer "+credential.Data.Credential)
	req.Header.Set("Content-Type", "application/json")
	res, err = agent.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("heartbeat %v %v", res, err)
	}
	res.Body.Close()
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/v1/agent/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+credential.Data.Credential)
	res, err = agent.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var leased struct {
		Data struct {
			Found   bool            `json:"found"`
			ID      string          `json:"id"`
			Kind    string          `json:"kind"`
			Payload json.RawMessage `json:"payload"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&leased); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if !leased.Data.Found || leased.Data.Kind != "discover" {
		t.Fatalf("lease %#v", leased.Data)
	}
	var payload map[string]string
	if err := json.Unmarshal(leased.Data.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["site_name"] != "Agent Site" || payload["site_id"] == "" {
		t.Fatalf("payload %#v", payload)
	}
	body := map[string]any{"ok": true, "discovery": map[string]any{
		"data_origin": "simulator", "support_note": "agent",
		"capabilities": []string{"inventory.v1"},
		"devices":      []map[string]any{{"name": "agent-site-sw-1", "model": "SDN-SIM-LEAF", "os_version": "sim-1.0.0", "role": "leaf", "interfaces": []string{"eth1"}}},
	}}
	raw, _ := json.Marshal(body)
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agent/tasks/"+leased.Data.ID+"/complete", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+credential.Data.Credential)
	req.Header.Set("Content-Type", "application/json")
	res, err = agent.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(res.Body)
		t.Fatalf("complete %v %v %s", res, err, msg)
	}
	res.Body.Close()
	var devices []map[string]any
	getJSON(t, admin, ts.URL+"/api/v1/devices?q=agent-site-sw-1", &devices)
	if len(devices) != 1 || devices[0]["data_origin"] != "simulator" {
		t.Fatalf("discovered %#v", devices)
	}
}

func resetDB(ctx context.Context, st *store.Store) error {
	conn, err := st.Pool().Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SELECT set_config('app.bypass', 'on', false)`); err != nil {
		return err
	}
	_, err = conn.Exec(ctx, `
		TRUNCATE audit_events, restore_runs, configuration_backups, verification_results,
		         execution_steps, workflow_runs, approvals, change_requests, simulator_acls,
		         drift_findings, observed_states, desired_states, policy_versions, policies,
		         segments, tenants, fabrics, capability_snapshots, topology_links, interfaces,
		         devices, agent_tasks, connector_tokens, connectors, sites, idempotency_keys,
		         integrations, authority_epochs, memberships, sessions, oidc_auth_codes,
		         users, organizations, worker_heartbeats
		RESTART IDENTITY CASCADE`)
	_, _ = conn.Exec(ctx, `SELECT set_config('app.bypass', '', false)`)
	return err
}

func login(t *testing.T, base, email string) *http.Client {
	t.Helper()
	client := &http.Client{}
	res := do(t, client, http.MethodPost, base+"/api/v1/auth/login", map[string]string{"email": email, "password": "test-password"})
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("login %s: %d %s", email, res.StatusCode, body)
	}
	return client
}

func getJSON(t *testing.T, client *http.Client, url string, dest any) {
	t.Helper()
	res := do(t, client, http.MethodGet, url, nil)
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("GET %s: %d %s", url, res.StatusCode, body)
	}
	readData(t, res, dest)
}

func postJSON(t *testing.T, client *http.Client, url string, payload any, status int, dest any) {
	t.Helper()
	res := do(t, client, http.MethodPost, url, payload)
	if res.StatusCode != status {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("POST %s: %d %s", url, res.StatusCode, body)
	}
	if dest != nil {
		readData(t, res, dest)
	}
}

func readData(t *testing.T, res *http.Response, dest any) {
	t.Helper()
	defer res.Body.Close()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if dest != nil {
		if err := json.Unmarshal(envelope.Data, dest); err != nil {
			t.Fatalf("decode data: %v %s", err, envelope.Data)
		}
	}
}

func do(t *testing.T, client *http.Client, method, url string, payload any) *http.Response {
	t.Helper()
	var body io.Reader
	if payload != nil {
		raw, _ := json.Marshal(payload)
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	jar, _ := client.Jar, error(nil)
	_ = jar
	if client.Jar == nil {
		client.Jar = newJar()
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}
