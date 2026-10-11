package secrets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// File stores secret values outside PostgreSQL. Production startup refuses this
// backend. The database still holds only the vault:// reference.
type File struct {
	Dir string
}

func NewFile(dir string) (*File, error) {
	if dir == "" {
		return nil, fmt.Errorf("SECRET_FILE_DIR is required for the file backend")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &File{Dir: dir}, nil
}

func (f *File) Put(_ context.Context, ref, value string) error {
	if err := validate(ref, value); err != nil {
		return err
	}
	path := f.path(ref)
	tmp, err := os.CreateTemp(f.Dir, "secret-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(value); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	ok = true
	return os.Chmod(path, 0o600)
}

func (f *File) Resolve(_ context.Context, ref string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("secret reference is required")
	}
	raw, err := os.ReadFile(f.path(ref))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("secret %s is not available to this process", ref)
		}
		return "", err
	}
	if len(raw) == 0 {
		return "", fmt.Errorf("secret %s is not available to this process", ref)
	}
	return string(raw), nil
}

func (f *File) path(ref string) string {
	sum := sha256.Sum256([]byte(ref))
	return filepath.Join(f.Dir, hex.EncodeToString(sum[:]))
}
