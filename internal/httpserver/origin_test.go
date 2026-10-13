package httpserver

import "testing"

func TestSDNConsoleOrigins(t *testing.T) {
	allowed := []string{"https://sdn-psi.vercel.app"}
	publicBase := "https://sdn-api.84-8-104-48.sslip.io"
	for _, origin := range []string{
		"https://sdn-psi.vercel.app",
		"https://sdn-git-cursor-sdn-platform-9880-majd-6c6d.vercel.app",
		"https://sdn-git-cursor-sdn-operations-platform-9880-majd-6c6d.vercel.app",
		publicBase,
	} {
		if !originPermitted(allowed, publicBase, origin) {
			t.Fatalf("expected %s to be allowed", origin)
		}
	}
	for _, origin := range []string{
		"http://sdn-psi.vercel.app",
		"https://evil.vercel.app",
		"https://sdn-git-other.vercel.app",
		"https://sdn-psi.vercel.app/callback",
	} {
		if originPermitted(nil, publicBase, origin) {
			t.Fatalf("expected %s to be blocked", origin)
		}
	}
}
