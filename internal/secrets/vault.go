package secrets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Vault is an HTTP client for HashiCorp Vault KV version 2.
// A reference vault://org/connector/management is stored at
// {addr}/v1/{mount}/data/org/connector/management under the key "value".
// This client does not start a Vault server.
type Vault struct {
	Addr   string
	Token  string
	Mount  string
	Client *http.Client
}

func NewVault(addr, token, mount string) (*Vault, error) {
	if addr == "" || token == "" {
		return nil, fmt.Errorf("VAULT_ADDR and VAULT_TOKEN are required")
	}
	if mount == "" {
		mount = "secret"
	}
	if strings.Contains(mount, "/") || strings.Contains(mount, "..") {
		return nil, fmt.Errorf("VAULT_MOUNT must be a single path segment")
	}
	return &Vault{
		Addr:  strings.TrimRight(addr, "/"),
		Token: token,
		Mount: mount,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (v *Vault) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.Addr+"/v1/sys/health", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Vault-Token", v.Token)
	res, err := v.client().Do(req)
	if err != nil {
		return fmt.Errorf("vault is not reachable: %w", err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	if res.StatusCode >= 500 {
		return fmt.Errorf("vault health returned %d", res.StatusCode)
	}
	return nil
}

func (v *Vault) Put(ctx context.Context, ref, value string) error {
	if err := validate(ref, value); err != nil {
		return err
	}
	path, err := vaultPath(ref)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{"data": map[string]string{"value": value}})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.dataURL(path), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Vault-Token", v.Token)
	res, err := v.client().Do(req)
	if err != nil {
		return fmt.Errorf("vault write failed: %w", err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("vault write returned %d", res.StatusCode)
	}
	return nil
}

func (v *Vault) Resolve(ctx context.Context, ref string) (string, error) {
	path, err := vaultPath(ref)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.dataURL(path), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Vault-Token", v.Token)
	res, err := v.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("vault read failed: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("secret %s is not available to this process", ref)
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vault read returned %d", res.StatusCode)
	}
	var payload struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("vault read response was not KV v2 JSON")
	}
	value := payload.Data.Data["value"]
	if value == "" {
		return "", fmt.Errorf("secret %s is not available to this process", ref)
	}
	return value, nil
}

func (v *Vault) client() *http.Client {
	if v.Client != nil {
		return v.Client
	}
	return http.DefaultClient
}

func (v *Vault) dataURL(path string) string {
	return v.Addr + "/v1/" + v.Mount + "/data/" + path
}

func vaultPath(ref string) (string, error) {
	const prefix = "vault://"
	if !strings.HasPrefix(ref, prefix) {
		return "", fmt.Errorf("secret reference must start with vault://")
	}
	path := strings.Trim(strings.TrimPrefix(ref, prefix), "/")
	if path == "" {
		return "", fmt.Errorf("secret reference is required")
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("secret reference contains an invalid path segment")
		}
	}
	return path, nil
}
