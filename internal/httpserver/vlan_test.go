package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/majdkh733-dev/SDN/internal/config"
	"github.com/majdkh733-dev/SDN/internal/migrate"
	"github.com/majdkh733-dev/SDN/internal/seed"
	"github.com/majdkh733-dev/SDN/internal/store"
)

func TestVLANChangeVerifyAndDrift(t *testing.T) {
	ctx := t.Context()
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
	approver := login(t, ts.URL, "approver@nexora.demo")
	admin := login(t, ts.URL, "admin@nexora.demo")
	iso := login(t, ts.URL, "admin@isolated.demo")

	var devices []map[string]any
	getJSON(t, eng, ts.URL+"/api/v1/devices", &devices)
	var device map[string]any
	for _, item := range devices {
		if item["data_origin"] == "simulator" {
			device = item
			break
		}
	}
	if device == nil {
		t.Fatal("no simulator device")
	}
	var tenants []map[string]any
	getJSON(t, eng, ts.URL+"/api/v1/tenants", &tenants)
	postJSON(t, eng, ts.URL+"/api/v1/provisioning/vlan", map[string]any{
		"name": "vlan-users", "tenant_id": tenants[0]["id"], "site_id": device["site_id"], "device_id": device["id"],
		"vid": 250, "vlan_name": "users",
	}, http.StatusCreated, nil)
	var proposal map[string]any
	postJSON(t, eng, ts.URL+"/api/v1/provisioning/vlan", map[string]any{
		"name": "vlan-lab", "tenant_id": tenants[0]["id"], "site_id": device["site_id"], "device_id": device["id"],
		"vid": 251, "vlan_name": "lab",
	}, http.StatusCreated, &proposal)
	if proposal["generated_config"] == "" || proposal["plan_hash"] == "" {
		t.Fatalf("proposal %#v", proposal)
	}
	postJSON(t, eng, ts.URL+"/api/v1/devices/"+device["id"].(string)+"/lab/overlay", map[string]any{"body": "vlan 999\n name unmanaged\n"}, http.StatusOK, nil)
	var change map[string]any
	postJSON(t, approver, ts.URL+"/api/v1/changes/"+proposal["change_id"].(string)+"/approve", map[string]any{"decision": "approved"}, http.StatusOK, &change)
	postJSON(t, eng, ts.URL+"/api/v1/changes/"+proposal["change_id"].(string)+"/execute", map[string]any{"confirm": true, "plan_hash": proposal["plan_hash"]}, http.StatusAccepted, &change)
	if change["status"] != "failed" {
		t.Fatalf("stale baseline status %v %v", change["status"], change["error_detail"])
	}

	postJSON(t, eng, ts.URL+"/api/v1/devices/"+device["id"].(string)+"/lab/overlay", map[string]any{"body": ""}, http.StatusOK, nil)
	postJSON(t, eng, ts.URL+"/api/v1/provisioning/vlan", map[string]any{
		"name": "vlan-lab-2", "tenant_id": tenants[0]["id"], "site_id": device["site_id"], "device_id": device["id"],
		"vid": 252, "vlan_name": "lab2",
	}, http.StatusCreated, &proposal)
	postJSON(t, approver, ts.URL+"/api/v1/changes/"+proposal["change_id"].(string)+"/approve", map[string]any{"decision": "approved"}, http.StatusOK, &change)
	postJSON(t, eng, ts.URL+"/api/v1/changes/"+proposal["change_id"].(string)+"/execute", map[string]any{"confirm": true, "plan_hash": proposal["plan_hash"]}, http.StatusAccepted, &change)
	if change["status"] != "succeeded" {
		t.Fatalf("vlan apply %v %v", change["status"], change["error_detail"])
	}
	var configs []map[string]any
	getJSON(t, eng, ts.URL+"/api/v1/config-revisions?device_id="+device["id"].(string), &configs)
	if len(configs) == 0 || configs[0]["role"] != "desired" {
		t.Fatalf("missing desired config %#v", configs)
	}
	postJSON(t, eng, ts.URL+"/api/v1/devices/"+device["id"].(string)+"/lab/overlay", map[string]any{"body": "vlan 999\n name unmanaged\n"}, http.StatusOK, nil)
	var collected map[string]any
	postJSON(t, eng, ts.URL+"/api/v1/devices/"+device["id"].(string)+"/config/collect", map[string]any{}, http.StatusOK, &collected)
	if collected["body"] == "" {
		t.Fatal("empty observed config")
	}
	var drift []map[string]any
	getJSON(t, eng, ts.URL+"/api/v1/drift-findings", &drift)
	found := false
	for _, item := range drift {
		diff, _ := item["difference"].(map[string]any)
		if diff["kind"] == "config" && item["disposition"] == "open" {
			found = true
		}
	}
	if !found {
		t.Fatalf("config drift not opened: %#v", drift)
	}
	var foreign []map[string]any
	getJSON(t, iso, ts.URL+"/api/v1/vlans", &foreign)
	for _, item := range foreign {
		if item["vid"] == float64(252) {
			t.Fatal("vlan leaked across organizations")
		}
	}

	var account map[string]any
	postJSON(t, admin, ts.URL+"/api/v1/service-accounts", map[string]any{"name": "reader", "role": "viewer"}, http.StatusCreated, &account)
	token, _ := account["token"].(string)
	if token == "" {
		t.Fatal("service token missing")
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/devices", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("service account read %v %v", res, err)
	}
	res.Body.Close()
	postJSON(t, admin, ts.URL+"/api/v1/service-accounts/"+account["id"].(string)+"/revoke", map[string]any{}, http.StatusOK, nil)
	res, _ = http.DefaultClient.Do(req)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked service account status %d", res.StatusCode)
	}
	res.Body.Close()

	var matrix []map[string]any
	getJSON(t, eng, ts.URL+"/api/v1/adapters", &matrix)
	if len(matrix) < 4 {
		t.Fatalf("adapter matrix %#v", matrix)
	}
}
