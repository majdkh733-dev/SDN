package config

import "testing"

func TestProductionRequiresVaultAndRejectsDevShortcuts(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("DATABASE_URL", "postgres://sdn:sdn@127.0.0.1:5432/sdn?sslmode=disable")
	t.Setenv("AUTH_MODE", "oidc")
	t.Setenv("TOKEN_SECRET", "production-token-secret")
	t.Setenv("OIDC_CLIENT_SECRET", "production-oidc-secret")
	t.Setenv("OIDC_ISSUER", "https://issuer.example")
	t.Setenv("SECURE_COOKIES", "true")
	t.Setenv("COOKIE_SAMESITE", "none")
	t.Setenv("INLINE_WORKFLOWS", "false")
	t.Setenv("SECRET_BACKEND", "memory")
	t.Setenv("VAULT_ADDR", "")
	t.Setenv("VAULT_TOKEN", "")
	if _, err := Load(); err == nil || err.Error() != "production requires SECRET_BACKEND=vault" {
		t.Fatalf("memory backend: %v", err)
	}
	t.Setenv("SECRET_BACKEND", "file")
	t.Setenv("SECRET_FILE_DIR", t.TempDir())
	if _, err := Load(); err == nil || err.Error() != "production requires SECRET_BACKEND=vault" {
		t.Fatalf("file backend: %v", err)
	}
	t.Setenv("SECRET_BACKEND", "vault")
	if _, err := Load(); err == nil || err.Error() != "production requires VAULT_ADDR and VAULT_TOKEN" {
		t.Fatalf("missing vault: %v", err)
	}
	t.Setenv("VAULT_ADDR", "https://vault.example")
	t.Setenv("VAULT_TOKEN", "replace-me")
	if _, err := Load(); err == nil {
		t.Fatal("placeholder vault token was accepted")
	}
	t.Setenv("VAULT_TOKEN", "production-vault-token")
	t.Setenv("INLINE_WORKFLOWS", "true")
	t.Setenv("TEMPORAL_ADDRESS", "temporal.example:7233")
	if _, err := Load(); err == nil || err.Error() != "production cannot run inline workflows and Temporal together" {
		t.Fatalf("inline plus temporal: %v", err)
	}
	t.Setenv("INLINE_WORKFLOWS", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SecretBackend != "vault" || cfg.TemporalAddress != "temporal.example:7233" || cfg.VaultMount != "secret" {
		t.Fatalf("%#v", cfg)
	}
	if cfg.AuthMode != "oidc" || !cfg.SecureCookies {
		t.Fatalf("production flags %#v", cfg)
	}
}

func TestDevelopmentDefaultsToMemorySecrets(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("DATABASE_URL", "postgres://sdn:sdn@127.0.0.1:5432/sdn?sslmode=disable")
	t.Setenv("AUTH_MODE", "dev")
	t.Setenv("TOKEN_SECRET", "dev-token")
	t.Setenv("OIDC_CLIENT_SECRET", "dev-oidc")
	t.Setenv("SECRET_BACKEND", "")
	t.Setenv("VAULT_ADDR", "")
	t.Setenv("VAULT_TOKEN", "")
	t.Setenv("TEMPORAL_ADDRESS", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SecretBackend != "memory" || cfg.TemporalAddress != "" {
		t.Fatalf("%#v", cfg)
	}
}
