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

func TestTopologyDiagramBindingIsolation(t *testing.T) {
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
	viewer := login(t, ts.URL, "viewer@nexora.demo")
	iso := login(t, ts.URL, "admin@isolated.demo")

	var current map[string]any
	getJSON(t, eng, ts.URL+"/api/v1/topology-diagram", &current)
	if current["source"] != "reference" {
		t.Fatalf("source %#v", current["source"])
	}
	var devices []map[string]any
	getJSON(t, eng, ts.URL+"/api/v1/devices", &devices)
	if len(devices) == 0 {
		t.Fatal("no devices")
	}
	res := do(t, viewer, http.MethodPost, ts.URL+"/api/v1/topology-diagram/bindings", map[string]any{"cell_id": "symbol-1", "device_id": devices[0]["id"]})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer status %d", res.StatusCode)
	}
	res.Body.Close()
	postJSON(t, eng, ts.URL+"/api/v1/topology-diagram/bindings", map[string]any{"cell_id": "symbol-1", "device_id": devices[0]["id"]}, http.StatusOK, nil)
	postJSON(t, eng, ts.URL+"/api/v1/topology-diagram", map[string]any{"mxfile": `<mxfile host="test"><diagram name="Page-1"><mxGraphModel><root><mxCell id="0"/><mxCell id="1" parent="0"/></root></mxGraphModel></diagram></mxfile>`}, http.StatusCreated, nil)
	getJSON(t, eng, ts.URL+"/api/v1/topology-diagram", &current)
	if current["source"] != "saved" || current["mxfile"] == "" {
		t.Fatalf("saved %#v", current["source"])
	}
	var bindings []any
	switch rows := current["bindings"].(type) {
	case []any:
		bindings = rows
	default:
		t.Fatalf("bindings %#v", current["bindings"])
	}
	if len(bindings) != 1 {
		t.Fatalf("bindings %#v", bindings)
	}
	var other map[string]any
	getJSON(t, iso, ts.URL+"/api/v1/topology-diagram", &other)
	if other["source"] != "reference" {
		t.Fatalf("isolated org saw %v", other["source"])
	}
	if rows, ok := other["bindings"].([]any); ok && len(rows) != 0 {
		t.Fatalf("isolated bindings %#v", rows)
	}
	if rows, ok := other["link_bindings"].([]any); ok && len(rows) != 0 {
		t.Fatalf("isolated links %#v", rows)
	}
	res = do(t, eng, http.MethodPost, ts.URL+"/api/v1/topology-diagram/bindings", map[string]any{"cell_id": "symbol-2", "device_id": devices[0]["id"]})
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate status %d", res.StatusCode)
	}
	res.Body.Close()
	var ifaces []map[string]any
	getJSON(t, eng, ts.URL+"/api/v1/devices/"+devices[0]["id"].(string)+"/interfaces", &ifaces)
	var otherIfaces []map[string]any
	getJSON(t, eng, ts.URL+"/api/v1/devices/"+devices[1]["id"].(string)+"/interfaces", &otherIfaces)
	if len(ifaces) == 0 || len(otherIfaces) == 0 {
		t.Fatal("fixture devices have no interfaces")
	}
	res = do(t, viewer, http.MethodPost, ts.URL+"/api/v1/topology-diagram/links", map[string]any{
		"cell_id": "edge-1", "source_device_id": devices[0]["id"], "source_interface_id": ifaces[0]["id"],
	})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer link status %d", res.StatusCode)
	}
	res.Body.Close()
	postJSON(t, eng, ts.URL+"/api/v1/topology-diagram/links", map[string]any{
		"cell_id": "edge-1", "connection_type": "physical", "evidence": "manual",
		"source_device_id": devices[0]["id"], "source_interface_id": ifaces[0]["id"],
		"target_device_id": devices[1]["id"], "target_interface_id": otherIfaces[0]["id"],
	}, http.StatusOK, nil)
	getJSON(t, eng, ts.URL+"/api/v1/topology-diagram", &current)
	links, _ := current["link_bindings"].([]any)
	if len(links) != 1 {
		t.Fatalf("links %#v", current["link_bindings"])
	}
	getJSON(t, iso, ts.URL+"/api/v1/topology-diagram", &other)
	if rows, ok := other["link_bindings"].([]any); ok && len(rows) != 0 {
		t.Fatalf("isolated links after map %#v", rows)
	}
}
