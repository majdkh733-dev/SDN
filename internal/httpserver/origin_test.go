package httpserver

import "testing"

func TestSDNConsoleOrigins(t *testing.T) {
	allowed := []string{"https://sdn-psi.vercel.app"}
	publicBase := "https://sdn-api.84-8-104-48.sslip.io"
	ok := []string{
		"https://sdn-psi.vercel.app",
		"https://sdn-git-cursor-sdn-platform-9880-majd-6c6d.vercel.app",
		"https://sdn-git-cursor-site-connectors-command-room-9880-majd-6c6d.vercel.app",
		"https://sdn-git-cursor-sdn-operations-platform-9880-majd-6c6d.vercel.app",
		publicBase,
	}
	for _, origin := range ok {
		if !originPermitted(allowed, publicBase, origin) {
			t.Fatalf("expected %s to be allowed", origin)
		}
	}
	blocked := []string{
		"http://sdn-psi.vercel.app",
		"https://evil.vercel.app",
		"https://sdn-psi.vercel.app.evil.com",
		"https://sdn-git-cursor-sdn-platform-9880-majd-6c6d.vercel.app.evil.com",
		"https://sdn-git-other.vercel.app",
		"https://sdn-psi.vercel.app/callback",
	}
	for _, origin := range blocked {
		if originPermitted(nil, publicBase, origin) {
			t.Fatalf("expected %s to be blocked", origin)
		}
	}
}
