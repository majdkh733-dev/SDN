package authn

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

func TestPKCE(t *testing.T) {
	verifier := strings.Repeat("a", 50)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	if err := VerifyPKCE(verifier, challenge); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPKCE(verifier, "not-the-challenge-value-pad-pad-pad"); err == nil {
		t.Fatal("expected mismatch")
	}
}

func TestHashStable(t *testing.T) {
	if Hash("abc") != Hash("abc") || Hash("abc") == Hash("abd") {
		t.Fatal("token hash mismatch")
	}
}
