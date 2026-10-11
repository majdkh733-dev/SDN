package secrets

import "fmt"

type Options struct {
	Backend    string
	FileDir    string
	VaultAddr  string
	VaultToken string
	VaultMount string
}

func Open(opts Options) (Manager, error) {
	switch opts.Backend {
	case "", "memory":
		return NewMemory(), nil
	case "file":
		return NewFile(opts.FileDir)
	case "vault":
		return NewVault(opts.VaultAddr, opts.VaultToken, opts.VaultMount)
	default:
		return nil, fmt.Errorf("SECRET_BACKEND must be memory, file, or vault")
	}
}

func validate(ref, value string) error {
	if ref == "" || value == "" {
		return fmt.Errorf("secret reference and value are required")
	}
	return nil
}
