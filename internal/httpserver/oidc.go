package httpserver

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/majdkh733-dev/SDN/internal/apierr"
	"github.com/majdkh733-dev/SDN/internal/authn"
	"github.com/majdkh733-dev/SDN/internal/store"
)

type signer struct {
	key *rsa.PrivateKey
	kid string
}

func newSigner() (*signer, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return &signer{key: key, kid: "sdn-dev"}, nil
}

func (s *signer) jwks(w http.ResponseWriter) {
	pub := s.key.PublicKey
	writeJSON(w, http.StatusOK, map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "alg": "RS256", "use": "sig", "kid": s.kid,
		"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}}})
}

func (s *signer) token(sub, email, audience, issuer string) (string, error) {
	header := mustB64(map[string]string{"alg": "RS256", "typ": "JWT", "kid": s.kid})
	payload := mustB64(map[string]any{
		"iss": issuer, "sub": sub, "aud": audience, "email": email,
		"exp": time.Now().Add(15 * time.Minute).Unix(), "iat": time.Now().Unix(),
	})
	signingInput := header + "." + payload
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func mustB64(v any) string {
	raw, _ := json.Marshal(v)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func (s *Server) oidcDiscovery(w http.ResponseWriter, _ *http.Request) {
	base := s.Config.PublicAPIBase
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer": base + "/oidc", "authorization_endpoint": base + "/oidc/authorize",
		"token_endpoint": base + "/oidc/token", "jwks_uri": base + "/oidc/jwks", "userinfo_endpoint": base + "/oidc/userinfo",
		"response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"code_challenge_methods_supported":      []string{"S256"},
	})
}

func (s *Server) oidcAuthorize(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeErr(w, apierr.Invalid("invalid form"))
		return
	}
	q := r.Form
	redirectURI := q.Get("redirect_uri")
	if !s.allowedRedirect(redirectURI) || q.Get("client_id") != s.Config.OIDCClientID || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		writeErr(w, apierr.Invalid("authorization request is not allowed"))
		return
	}
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		page := authorizePage
		replacer := strings.NewReplacer(
			"{{CLIENT}}", htmlEscape(q.Get("client_id")),
			"{{REDIRECT}}", htmlEscape(redirectURI),
			"{{CHALLENGE}}", htmlEscape(q.Get("code_challenge")),
			"{{STATE}}", htmlEscape(q.Get("state")),
			"{{SCOPE}}", htmlEscape(q.Get("scope")),
		)
		_, _ = w.Write([]byte(replacer.Replace(page)))
		return
	}
	principal, err := s.Store.Authenticate(r.Context(), q.Get("email"), q.Get("password"))
	if err != nil {
		http.Redirect(w, r, r.URL.Path+"?"+r.URL.RawQuery+"&error=1", http.StatusSeeOther)
		return
	}
	code, err := authn.NewToken()
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := s.Store.SaveAuthCode(r.Context(), code, store.AuthCode{
		UserID: principal.User.ID, OrganizationID: principal.Organization.OrganizationID,
		Challenge: q.Get("code_challenge"), RedirectURI: redirectURI, ClientID: q.Get("client_id"),
	}, 5*time.Minute); err != nil {
		writeErr(w, err)
		return
	}
	sep := "?"
	if strings.Contains(redirectURI, "?") {
		sep = "&"
	}
	http.Redirect(w, r, redirectURI+sep+"code="+code+"&state="+q.Get("state"), http.StatusSeeOther)
}

func (s *Server) oidcToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeErr(w, apierr.Invalid("invalid form"))
		return
	}
	if r.PostForm.Get("client_secret") != s.Config.OIDCClientSecret {
		writeErr(w, apierr.Unauthorized())
		return
	}
	ac, err := s.Store.ConsumeAuthCode(r.Context(), r.PostForm.Get("code"), r.PostForm.Get("client_id"), r.PostForm.Get("redirect_uri"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := authn.VerifyPKCE(r.PostForm.Get("code_verifier"), ac.Challenge); err != nil {
		writeErr(w, apierr.Invalid("pkce verification failed"))
		return
	}
	principal, err := s.userPrincipal(r.Context(), ac.UserID, ac.OrganizationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	raw, err := s.Signer.token(principal.User.ID, principal.User.Email, s.Config.OIDCClientID, s.Config.PublicAPIBase+"/oidc")
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access_token": raw, "id_token": raw, "token_type": "Bearer", "expires_in": 900})
}

func htmlEscape(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(value)
}

const authorizePage = `<!doctype html>
<meta charset="utf-8">
<title>SDN sign in</title>
<style>
body{font-family:Segoe UI,sans-serif;background:#f3f5f8;color:#1c2834;margin:0;display:grid;place-items:center;min-height:100vh}
form{background:#fff;border:1px solid #e4e8ee;border-radius:6px;padding:24px;width:320px}
h1{font-size:18px;margin:0 0 8px}p{color:#5e6c7b;font-size:13px}label{display:block;font-size:13px;margin:12px 0 4px}
input{width:100%;box-sizing:border-box;padding:8px;border:1px solid #e4e8ee;border-radius:6px}
button{margin-top:16px;background:#2c5f8a;color:#fff;border:0;border-radius:6px;padding:8px 12px;width:100%}
</style>
<h1>SDN</h1>
<p>Development identity provider. Passwords are checked on the server and are not written to the audit log.</p>
<form method="post">
<input type="hidden" name="client_id" value="{{CLIENT}}">
<input type="hidden" name="redirect_uri" value="{{REDIRECT}}">
<input type="hidden" name="code_challenge" value="{{CHALLENGE}}">
<input type="hidden" name="code_challenge_method" value="S256">
<input type="hidden" name="state" value="{{STATE}}">
<input type="hidden" name="scope" value="{{SCOPE}}">
<label for="email">Email</label><input id="email" name="email" type="email" required autocomplete="username">
<label for="password">Password</label><input id="password" name="password" type="password" required autocomplete="current-password">
<button type="submit">Continue</button>
</form>`
