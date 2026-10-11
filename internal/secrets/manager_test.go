package secrets

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFileAndMemoryKeepValuesOutOfTheReference(t *testing.T) {
	ref := Reference("org", "connector")
	if ref != "vault://org/connector/management" {
		t.Fatalf("reference %s", ref)
	}
	mem := NewMemory()
	if err := mem.Put(context.Background(), ref, "s3cret-value"); err != nil {
		t.Fatal(err)
	}
	got, err := mem.Resolve(context.Background(), ref)
	if err != nil || got != "s3cret-value" {
		t.Fatalf("memory resolve %q %v", got, err)
	}
	dir := t.TempDir()
	file, err := NewFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Put(context.Background(), ref, "file-secret"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("files %#v %v", entries, err)
	}
	info, err := os.Stat(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}
	raw, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil || string(raw) != "file-secret" {
		t.Fatalf("stored %q %v", raw, err)
	}
	if entries[0].Name() == ref {
		t.Fatal("file name is the raw reference")
	}
	got, err = file.Resolve(context.Background(), ref)
	if err != nil || got != "file-secret" {
		t.Fatalf("file resolve %q %v", got, err)
	}
}

func TestVaultKV(t *testing.T) {
	stored := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "test-token" {
			http.Error(w, "no token", http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/v1/sys/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"initialized":true}`))
		case "/v1/secret/data/org/connector/management":
			if r.Method == http.MethodPost {
				var body struct {
					Data map[string]string `json:"data"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					http.Error(w, "bad", http.StatusBadRequest)
					return
				}
				stored["value"] = body.Data["value"]
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data":{}}`))
				return
			}
			if stored["value"] == "" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"data": map[string]string{"value": stored["value"]}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	vault, err := NewVault(srv.URL, "test-token", "secret")
	if err != nil {
		t.Fatal(err)
	}
	vault.Client = srv.Client()
	if err := vault.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	ref := Reference("org", "connector")
	if _, err := vault.Resolve(context.Background(), ref); err == nil {
		t.Fatal("missing secret resolved")
	}
	if err := vault.Put(context.Background(), ref, "vault-secret"); err != nil {
		t.Fatal(err)
	}
	got, err := vault.Resolve(context.Background(), ref)
	if err != nil || got != "vault-secret" {
		t.Fatalf("resolve %q %v", got, err)
	}
	if _, err := vaultPath("vault://org/../connector/management"); err == nil {
		t.Fatal("parent segment was accepted")
	}
}
