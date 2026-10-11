package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/majdkh733-dev/SDN/internal/apierr"
	"github.com/majdkh733-dev/SDN/internal/config"
	"github.com/majdkh733-dev/SDN/internal/migrate"
	"github.com/majdkh733-dev/SDN/internal/seed"
	"github.com/majdkh733-dev/SDN/internal/store"
)

func TestWorkflowRecovery(t *testing.T) {
	ctx := context.Background()
	st, ts, eng, approver := recoveryServer(t)
	orgID, userID := sessionIdentity(t, eng, ts.URL)
	var controllers map[string]any
	getJSON(t, eng, ts.URL+"/api/v1/controllers/status", &controllers)
	if controllers["orchestration"] != "postgresql" || controllers["temporal"] != "not_configured" || controllers["secret_backend"] != "memory" {
		t.Fatalf("controller status %#v", controllers)
	}
	base := financeSpec(t, eng, ts.URL)

	change := submitApproved(t, eng, approver, ts.URL, base, "recovery-verify")
	changeID := change["id"].(string)
	versionID := change["policy_version_id"].(string)
	planHash := change["plan_hash"].(string)
	policyName := change["policy_name"].(string)
	if change["author_confirmed"] != false {
		t.Fatalf("approval set author_confirmed: %#v", change["author_confirmed"])
	}
	res := do(t, eng, http.MethodPost, ts.URL+"/api/v1/changes/"+changeID+"/execute", map[string]any{"confirm": false, "plan_hash": planHash})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("execute without confirm %d", res.StatusCode)
	}
	res.Body.Close()
	err := st.RunChange(ctx, orgID, changeID)
	var api *apierr.Error
	if !errors.As(err, &api) || api.Code != "forbidden" {
		t.Fatalf("RunChange before confirmation: %v", err)
	}
	change = getChange(t, eng, ts.URL, changeID)
	if change["status"] != "approved" || change["author_confirmed"] != false {
		t.Fatalf("unconfirmed change changed: status %v confirmed %v", change["status"], change["author_confirmed"])
	}
	if claimed(t, st, changeID) {
		t.Fatal("worker claimed a change that the author has not confirmed")
	}

	if err := st.ConfirmExecution(ctx, orgID, userID, changeID, planHash); err != nil {
		t.Fatal(err)
	}
	originalDevices := aclDevices(change)
	if len(originalDevices) < 2 {
		t.Fatalf("expected at least two devices, got %v", originalDevices)
	}
	if err := interrupt(ctx, st, orgID, changeID, versionID, "verifying", originalDevices, nil); err != nil {
		t.Fatal(err)
	}
	supersede(t, eng, ts.URL, policyName, base)
	extraID := insertActiveDevice(t, ctx, st, orgID, base["site_id"].(string))

	if err := st.RunChange(ctx, orgID, changeID); err != nil {
		t.Fatal(err)
	}
	change = getChange(t, eng, ts.URL, changeID)
	if change["status"] != "succeeded" {
		t.Fatalf("resumed verification status %v detail %v", change["status"], change["error_detail"])
	}
	if change["policy_version_id"] != versionID {
		t.Fatalf("recovery changed policy version from %s to %v", versionID, change["policy_version_id"])
	}
	verified := verifiedDevices(t, change)
	if !sameSet(verified, originalDevices) {
		t.Fatalf("verification devices %v, stored plan %v", verified, originalDevices)
	}
	for _, id := range verified {
		if id == extraID {
			t.Fatal("recovery included a device discovered after approval")
		}
	}
	for _, raw := range change["steps"].([]any) {
		step := raw.(map[string]any)
		key := step["operation_key"].(string)
		if len(key) > 11 && key[:11] == "compensate:" && step["status"] != "skipped" {
			t.Fatalf("unused rollback step %s is %v", key, step["status"])
		}
	}
	if claimed(t, st, changeID) {
		t.Fatal("succeeded change is still claimable")
	}

	rollback := submitApproved(t, eng, approver, ts.URL, base, "recovery-rollback")
	rollbackID := rollback["id"].(string)
	rollbackVersion := rollback["policy_version_id"].(string)
	devices := aclDevices(rollback)
	if len(devices) < 3 {
		t.Fatalf("rollback plan has %d devices, want the device added after the first plan", len(devices))
	}
	if err := st.ConfirmExecution(ctx, orgID, userID, rollbackID, rollback["plan_hash"].(string)); err != nil {
		t.Fatal(err)
	}
	kept := devices[0]
	failed := devices[1]
	pending := devices[2]
	if err := interrupt(ctx, st, orgID, rollbackID, rollbackVersion, "compensating", devices, map[string]stepSeed{
		kept:    {status: "compensated", result: `{"sentinel":"keep"}`, acl: false},
		failed:  {status: "compensation_failed", result: `{"outcome":"rollback failed"}`, acl: true},
		pending: {status: "pending", result: `{}`, acl: true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.RunChange(ctx, orgID, rollbackID); err != nil {
		t.Fatal(err)
	}
	rollback = getChange(t, eng, ts.URL, rollbackID)
	if rollback["status"] != "rollback_failed" {
		t.Fatalf("resumed compensation status %v detail %v", rollback["status"], rollback["error_detail"])
	}
	if rollback["policy_version_id"] != rollbackVersion {
		t.Fatalf("compensation changed version to %v", rollback["policy_version_id"])
	}
	assertStep(t, rollback, "compensate:"+kept, "compensated", "keep")
	assertStep(t, rollback, "compensate:"+failed, "compensation_failed", "rollback failed")
	assertStep(t, rollback, "compensate:"+pending, "compensated", "")
	aclSet := aclDevicesForVersion(t, ctx, st, orgID, rollbackVersion)
	if _, ok := aclSet[kept]; ok {
		t.Fatal("completed rollback recreated an ACL")
	}
	if _, ok := aclSet[failed]; !ok {
		t.Fatal("failed rollback removed the ACL")
	}
	if _, ok := aclSet[pending]; ok {
		t.Fatal("pending rollback left the ACL in place")
	}
	if err := st.RunChange(ctx, orgID, rollbackID); err != nil {
		t.Fatal(err)
	}
	again := getChange(t, eng, ts.URL, rollbackID)
	if again["status"] != "rollback_failed" {
		t.Fatalf("second recovery rewrote rollback_failed to %v", again["status"])
	}
	assertStep(t, again, "compensate:"+failed, "compensation_failed", "rollback failed")
	if _, ok := aclDevicesForVersion(t, ctx, st, orgID, rollbackVersion)[failed]; !ok {
		t.Fatal("second recovery removed the failed rollback ACL")
	}
	if claimed(t, st, rollbackID) {
		t.Fatal("rollback_failed is claimable")
	}

	queued := submitApproved(t, eng, approver, ts.URL, base, "recovery-confirmed")
	queuedID := queued["id"].(string)
	if err := st.ConfirmExecution(ctx, orgID, userID, queuedID, queued["plan_hash"].(string)); err != nil {
		t.Fatal(err)
	}
	if !claimed(t, st, queuedID) {
		t.Fatal("confirmed approved change was not claimable after a worker restart")
	}
	if err := st.RunChange(ctx, orgID, queuedID); err != nil {
		t.Fatal(err)
	}
	queued = getChange(t, eng, ts.URL, queuedID)
	if queued["status"] != "succeeded" || queued["author_confirmed"] != true {
		t.Fatalf("confirmed execution status %v confirmed %v detail %v", queued["status"], queued["author_confirmed"], queued["error_detail"])
	}
}

type stepSeed struct {
	status string
	result string
	acl    bool
}

func recoveryServer(t *testing.T) (*store.Store, *httptest.Server, *http.Client, *http.Client) {
	t.Helper()
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
	return st, ts, login(t, ts.URL, "engineer@nexora.demo"), login(t, ts.URL, "approver@nexora.demo")
}

func sessionIdentity(t *testing.T, client *http.Client, base string) (string, string) {
	t.Helper()
	var me map[string]any
	getJSON(t, client, base+"/api/v1/auth/me", &me)
	org := me["organization"].(map[string]any)
	return org["organization_id"].(string), me["id"].(string)
}

func financeSpec(t *testing.T, client *http.Client, base string) map[string]any {
	t.Helper()
	var segments []map[string]any
	getJSON(t, client, base+"/api/v1/segments?tenant_id=", &segments)
	finance := map[string]string{}
	var tenantID string
	for _, seg := range segments {
		if seg["name"] == "finance-users" || seg["name"] == "finance-apps" {
			finance[seg["name"].(string)] = seg["id"].(string)
			tenantID = seg["tenant_id"].(string)
		}
	}
	var sites []map[string]any
	getJSON(t, client, base+"/api/v1/sites", &sites)
	var siteID string
	for _, site := range sites {
		if site["name"] == "Riyadh DC1" {
			siteID = site["id"].(string)
		}
	}
	return map[string]any{
		"name": "finance-https", "tenant_id": tenantID, "site_id": siteID,
		"source":      map[string]string{"type": "segment", "id": finance["finance-users"]},
		"destination": map[string]string{"type": "segment", "id": finance["finance-apps"]},
		"protocol":    "tcp", "ports": "443", "action": "allow", "site_ids": []string{siteID},
	}
}

func submitApproved(t *testing.T, eng, approver *http.Client, base string, spec map[string]any, name string) map[string]any {
	t.Helper()
	body := map[string]any{}
	for k, v := range spec {
		if k == "site_id" {
			continue
		}
		body[k] = v
	}
	body["name"] = name
	var policy map[string]any
	postJSON(t, eng, base+"/api/v1/policies", body, http.StatusCreated, &policy)
	var preview map[string]any
	postJSON(t, eng, base+"/api/v1/policies/"+policy["id"].(string)+"/preview", map[string]any{}, http.StatusOK, &preview)
	var change map[string]any
	postJSON(t, eng, base+"/api/v1/changes", map[string]any{
		"policy_version_id": preview["policy_version_id"], "plan_hash": preview["plan_hash"], "preview_token": preview["preview_token"],
	}, http.StatusCreated, &change)
	postJSON(t, approver, base+"/api/v1/changes/"+change["id"].(string)+"/approve", map[string]any{"decision": "approved"}, http.StatusOK, &change)
	change["policy_name"] = name
	return change
}

func getChange(t *testing.T, client *http.Client, base, id string) map[string]any {
	t.Helper()
	var change map[string]any
	getJSON(t, client, base+"/api/v1/changes/"+id, &change)
	return change
}

func supersede(t *testing.T, client *http.Client, base, name string, spec map[string]any) {
	t.Helper()
	var policies []map[string]any
	getJSON(t, client, base+"/api/v1/policies", &policies)
	var policyID string
	for _, policy := range policies {
		if policy["name"] == name {
			policyID = policy["id"].(string)
		}
	}
	if policyID == "" {
		t.Fatalf("policy %s not found", name)
	}
	body := map[string]any{}
	for k, v := range spec {
		if k == "site_id" {
			continue
		}
		body[k] = v
	}
	body["name"] = name
	body["ports"] = "8443"
	postJSON(t, client, base+"/api/v1/policies/"+policyID+"/versions", body, http.StatusCreated, &map[string]any{})
}

func aclDevices(change map[string]any) []string {
	impact := change["impact"].(map[string]any)
	var ids []string
	for _, raw := range impact["items"].([]any) {
		item := raw.(map[string]any)
		id, _ := item["device_id"].(string)
		if id != "" && item["action"] == "add_acl" && item["blocking"] != true {
			ids = append(ids, id)
		}
	}
	return ids
}

func interrupt(ctx context.Context, st *store.Store, orgID, changeID, versionID, status string, devices []string, compensate map[string]stepSeed) error {
	return st.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE change_requests
			SET status = $3, execution_epoch = 1, updated_at = now()
			WHERE organization_id = $1 AND id = $2
		`, orgID, changeID, status); err != nil {
			return err
		}
		var runID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO workflow_runs (organization_id, change_request_id, status, started_at)
			VALUES ($1, $2, 'running', now())
			RETURNING id::text
		`, orgID, changeID).Scan(&runID); err != nil {
			return err
		}
		type step struct {
			key, device, stepStatus, result string
		}
		steps := []step{
			{key: "revalidate", stepStatus: "succeeded", result: `{"ok":true}`},
			{key: "backup", stepStatus: "succeeded", result: `{"ok":true}`},
			{key: "verify", stepStatus: "pending", result: `{}`},
		}
		if status == "verifying" {
			steps[2].stepStatus = "pending"
		}
		for _, id := range devices {
			seed := compensate[id]
			applyStatus := "succeeded"
			compStatus := "pending"
			compResult := `{}`
			if seed.status != "" {
				compStatus = seed.status
				compResult = seed.result
			}
			if status == "verifying" {
				compStatus = "pending"
			}
			steps = append(steps,
				step{key: "apply:" + id, device: id, stepStatus: applyStatus, result: `{"outcome":"applied"}`},
				step{key: "compensate:" + id, device: id, stepStatus: compStatus, result: compResult},
			)
			if status == "verifying" || seed.acl {
				if _, err := tx.Exec(ctx, `
					INSERT INTO simulator_acls (organization_id, device_id, policy_version_id, spec)
					VALUES ($1, $2, $3, '{}'::jsonb)
				`, orgID, id, versionID); err != nil {
					return err
				}
			}
		}
		for _, item := range steps {
			if _, err := tx.Exec(ctx, `
				INSERT INTO execution_steps (
					organization_id, workflow_run_id, change_request_id, device_id,
					operation_key, status, authority_epoch, result
				) VALUES ($1, $2, $3, NULLIF($4, '')::uuid, $5, $6, 1, $7::jsonb)
			`, orgID, runID, changeID, item.device, item.key, item.stepStatus, item.result); err != nil {
				return err
			}
		}
		return nil
	})
}

func insertActiveDevice(t *testing.T, ctx context.Context, st *store.Store, orgID, siteID string) string {
	t.Helper()
	var id string
	err := st.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO devices (
				organization_id, site_id, name, model, role, management_endpoint,
				lifecycle, health, data_origin, support_note, observed_at
			) VALUES ($1, $2, 'recovery-extra-sw', 'SDN-SIM-LEAF', 'leaf', 'sim://recovery-extra-sw',
			          'active', 'healthy', 'simulator', 'Added after the approved plan.', now())
			RETURNING id::text
		`, orgID, siteID).Scan(&id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO capability_snapshots (organization_id, device_id, capability, supported, evidence, connector_version)
			VALUES ($1, $2, 'acl.v1', true, 'Test device that must stay out of an in-flight plan.', '0.1.0')
		`, orgID, id)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func verifiedDevices(t *testing.T, change map[string]any) []string {
	t.Helper()
	raw, ok := change["verifications"].([]any)
	if !ok || len(raw) == 0 {
		t.Fatalf("no verification results: %#v", change["verifications"])
	}
	last := raw[len(raw)-1].(map[string]any)
	payload, err := json.Marshal(last["assertions"])
	if err != nil {
		t.Fatal(err)
	}
	var assertions []struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.Unmarshal(payload, &assertions); err != nil {
		t.Fatalf("assertions %s: %v", payload, err)
	}
	ids := make([]string, 0, len(assertions))
	for _, item := range assertions {
		ids = append(ids, item.DeviceID)
	}
	return ids
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, id := range a {
		seen[id]++
	}
	for _, id := range b {
		seen[id]--
		if seen[id] < 0 {
			return false
		}
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

func assertStep(t *testing.T, change map[string]any, key, status, snippet string) {
	t.Helper()
	for _, raw := range change["steps"].([]any) {
		step := raw.(map[string]any)
		if step["operation_key"] != key {
			continue
		}
		if step["status"] != status {
			t.Fatalf("step %s status %v, want %s", key, step["status"], status)
		}
		if snippet != "" && !strings.Contains(string(mustRaw(step["result"])), snippet) {
			t.Fatalf("step %s result %v lost %q", key, step["result"], snippet)
		}
		return
	}
	t.Fatalf("missing step %s", key)
}

func mustRaw(v any) []byte {
	raw, _ := json.Marshal(v)
	return raw
}

func aclDevicesForVersion(t *testing.T, ctx context.Context, st *store.Store, orgID, versionID string) map[string]struct{} {
	t.Helper()
	out := map[string]struct{}{}
	err := st.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT device_id::text FROM simulator_acls
			WHERE organization_id = $1 AND policy_version_id = $2
		`, orgID, versionID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			out[id] = struct{}{}
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func claimed(t *testing.T, st *store.Store, changeID string) bool {
	t.Helper()
	pairs, err := st.ClaimedChanges(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range pairs {
		if pair[1] == changeID {
			return true
		}
	}
	return false
}
