package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Environment       string
	DatabaseURL       string
	HTTPAddr          string
	AuthMode          string
	TokenSecret       string
	OIDCIssuer        string
	OIDCClientID      string
	OIDCClientSecret  string
	OIDCRedirectURLs  []string
	AllowedOrigins    []string
	SecureCookies     bool
	CookieSameSite    string
	InlineWorkflows   bool
	PublicAPIBase     string
	DevPassword       string
	SessionTTL        time.Duration
	SecretBackend     string
	SecretFileDir     string
	VaultAddr         string
	VaultToken        string
	VaultMount        string
	TemporalAddress   string
	TemporalNamespace string
}

func Load() (Config, error) {
	cfg := Config{
		Environment:       getenv("ENVIRONMENT", "development"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		HTTPAddr:          getenv("HTTP_ADDR", ":8080"),
		AuthMode:          getenv("AUTH_MODE", "dev"),
		TokenSecret:       os.Getenv("TOKEN_SECRET"),
		OIDCIssuer:        os.Getenv("OIDC_ISSUER"),
		OIDCClientID:      getenv("OIDC_CLIENT_ID", "sdn-web"),
		OIDCClientSecret:  os.Getenv("OIDC_CLIENT_SECRET"),
		PublicAPIBase:     strings.TrimRight(getenv("PUBLIC_API_BASE", "http://localhost:8080"), "/"),
		DevPassword:       os.Getenv("DEV_ADMIN_PASSWORD"),
		SecureCookies:     os.Getenv("SECURE_COOKIES") == "true",
		CookieSameSite:    getenv("COOKIE_SAMESITE", "lax"),
		InlineWorkflows:   getenv("INLINE_WORKFLOWS", "true") != "false",
		SessionTTL:        12 * time.Hour,
		SecretBackend:     getenv("SECRET_BACKEND", "memory"),
		SecretFileDir:     os.Getenv("SECRET_FILE_DIR"),
		VaultAddr:         strings.TrimRight(os.Getenv("VAULT_ADDR"), "/"),
		VaultToken:        os.Getenv("VAULT_TOKEN"),
		VaultMount:        getenv("VAULT_MOUNT", "secret"),
		TemporalAddress:   os.Getenv("TEMPORAL_ADDRESS"),
		TemporalNamespace: getenv("TEMPORAL_NAMESPACE", "default"),
	}
	cfg.AllowedOrigins = splitCSV(getenv("ALLOWED_ORIGINS", "http://localhost:5173"))
	cfg.OIDCRedirectURLs = splitCSV(getenv("OIDC_REDIRECT_URLS", "http://localhost:5173/callback"))
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.TokenSecret == "" {
		return Config{}, fmt.Errorf("TOKEN_SECRET is required")
	}
	if cfg.AuthMode == "dev" && cfg.OIDCClientSecret == "" {
		return Config{}, fmt.Errorf("OIDC_CLIENT_SECRET is required")
	}
	if cfg.Environment == "production" {
		if cfg.AuthMode != "oidc" {
			return Config{}, fmt.Errorf("production requires AUTH_MODE=oidc")
		}
		if cfg.TokenSecret == "replace-me" || cfg.OIDCClientSecret == "replace-me" || cfg.OIDCClientSecret == "" {
			return Config{}, fmt.Errorf("production secrets must be replaced")
		}
		if cfg.OIDCIssuer == "" {
			return Config{}, fmt.Errorf("production requires OIDC_ISSUER")
		}
		if !cfg.SecureCookies {
			return Config{}, fmt.Errorf("production requires SECURE_COOKIES=true")
		}
		if cfg.CookieSameSite != "lax" && cfg.CookieSameSite != "none" {
			return Config{}, fmt.Errorf("COOKIE_SAMESITE must be lax or none")
		}
		if cfg.SecretBackend != "vault" {
			return Config{}, fmt.Errorf("production requires SECRET_BACKEND=vault")
		}
		if cfg.VaultAddr == "" || cfg.VaultToken == "" || cfg.VaultToken == "replace-me" {
			return Config{}, fmt.Errorf("production requires VAULT_ADDR and VAULT_TOKEN")
		}
		if cfg.InlineWorkflows && cfg.TemporalAddress != "" {
			return Config{}, fmt.Errorf("production cannot run inline workflows and Temporal together")
		}
	}
	switch cfg.SecretBackend {
	case "memory", "file", "vault":
	default:
		return Config{}, fmt.Errorf("SECRET_BACKEND must be memory, file, or vault")
	}
	if cfg.SecretBackend == "file" && cfg.SecretFileDir == "" {
		return Config{}, fmt.Errorf("SECRET_FILE_DIR is required for the file backend")
	}
	if cfg.SecretBackend == "vault" && (cfg.VaultAddr == "" || cfg.VaultToken == "") {
		return Config{}, fmt.Errorf("VAULT_ADDR and VAULT_TOKEN are required for the vault backend")
	}
	if cfg.CookieSameSite != "lax" && cfg.CookieSameSite != "none" {
		return Config{}, fmt.Errorf("COOKIE_SAMESITE must be lax or none")
	}
	if cfg.CookieSameSite == "none" && !cfg.SecureCookies {
		return Config{}, fmt.Errorf("COOKIE_SAMESITE=none requires SECURE_COOKIES=true")
	}
	if cfg.AuthMode != "dev" && cfg.AuthMode != "oidc" {
		return Config{}, fmt.Errorf("AUTH_MODE must be dev or oidc")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
