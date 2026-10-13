package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/majdkh733-dev/SDN/connectors/ovn"
	"github.com/majdkh733-dev/SDN/internal/adapter"
	"github.com/majdkh733-dev/SDN/internal/apierr"
	"github.com/majdkh733-dev/SDN/internal/authn"
	"github.com/majdkh733-dev/SDN/internal/config"
	"github.com/majdkh733-dev/SDN/internal/domain"
	"github.com/majdkh733-dev/SDN/internal/secrets"
	"github.com/majdkh733-dev/SDN/internal/store"
)

type ChangeStarter interface {
	StartChange(ctx context.Context, orgID, changeID string) error
}

type Server struct {
	Store     *store.Store
	Config    config.Config
	Signer    *signer
	Log       *slog.Logger
	Secrets   secrets.Manager
	Workflows ChangeStarter
}

func New(cfg config.Config, st *store.Store, log *slog.Logger) (*Server, error) {
	if log == nil {
		log = slog.Default()
	}
	sig, err := newSigner()
	if err != nil {
		return nil, err
	}
	return &Server{Store: st, Config: cfg, Signer: sig, Log: log}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /metrics", s.metrics)
	mux.HandleFunc("GET /oidc/.well-known/openid-configuration", s.oidcDiscovery)
	mux.HandleFunc("GET /oidc/jwks", func(w http.ResponseWriter, _ *http.Request) { s.Signer.jwks(w) })
	mux.HandleFunc("GET /oidc/authorize", s.oidcAuthorize)
	mux.HandleFunc("POST /oidc/authorize", s.oidcAuthorize)
	mux.HandleFunc("POST /oidc/token", s.oidcToken)
	mux.HandleFunc("GET /api/v1/auth/config", s.authConfig)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/session", s.sessionFromCode)
	mux.HandleFunc("POST /api/v1/auth/logout", s.logout)
	mux.HandleFunc("GET /api/v1/auth/me", s.me)
	mux.HandleFunc("POST /api/v1/auth/switch-organization", s.switchOrg)
	mux.HandleFunc("GET /api/v1/command-room/summary", s.summary)
	mux.HandleFunc("GET /api/v1/sites", s.listSites)
	mux.HandleFunc("POST /api/v1/sites", s.createSite)
	mux.HandleFunc("GET /api/v1/connectors", s.listConnectors)
	mux.HandleFunc("POST /api/v1/connectors", s.createConnector)
	mux.HandleFunc("GET /api/v1/connectors/{id}", s.getConnector)
	mux.HandleFunc("POST /api/v1/connectors/{id}", s.updateConnector)
	mux.HandleFunc("POST /api/v1/connectors/{id}/enrollment", s.enroll)
	mux.HandleFunc("POST /api/v1/connectors/{id}/revoke", s.revokeConnector)
	mux.HandleFunc("POST /api/v1/connectors/{id}/replace", s.replaceConnector)
	mux.HandleFunc("POST /api/v1/connectors/{id}/remove", s.removeConnector)
	mux.HandleFunc("GET /api/v1/connectors/{id}/install", s.installConnector)
	mux.HandleFunc("POST /api/v1/connectors/{id}/discover", s.discover)
	mux.HandleFunc("GET /api/v1/discovery-profiles", s.listProfiles)
	mux.HandleFunc("POST /api/v1/sites/{id}/discovery-profiles", s.createProfile)
	mux.HandleFunc("GET /api/v1/sites/{id}/discovery-profiles", s.listProfiles)
	mux.HandleFunc("POST /api/v1/discovery-profiles/{id}/run", s.runProfile)
	mux.HandleFunc("GET /api/v1/discovery-jobs", s.listJobs)
	mux.HandleFunc("GET /api/v1/discovery-jobs/{id}", s.getJob)
	mux.HandleFunc("POST /api/v1/discovery-jobs/{id}/cancel", s.cancelJob)
	mux.HandleFunc("GET /api/v1/discovery-jobs/{id}/observations", s.listObservations)
	mux.HandleFunc("POST /api/v1/discovery-jobs/{id}/import", s.importObservations)
	mux.HandleFunc("POST /api/v1/sites/{id}/assets/import", s.importCSV)
	mux.HandleFunc("POST /api/v1/sites/{id}", s.updateSite)
	mux.HandleFunc("POST /api/v1/site-links", s.createSiteLink)
	mux.HandleFunc("POST /api/v1/devices/{id}", s.updateDevice)
	mux.HandleFunc("GET /api/v1/preferences", s.preferences)
	mux.HandleFunc("POST /api/v1/preferences/navigation", s.saveNavigation)
	mux.HandleFunc("POST /api/v1/preferences/navigation/reset", s.resetNavigation)
	mux.HandleFunc("POST /api/v1/preferences/theme", s.saveTheme)
	mux.HandleFunc("POST /api/v1/preferences/theme/reset", s.resetTheme)
	mux.HandleFunc("GET /api/v1/search", s.search)
	mux.HandleFunc("GET /api/v1/command-room/incidents", s.incidents)
	mux.HandleFunc("GET /api/v1/devices", s.listDevices)
	mux.HandleFunc("GET /api/v1/devices/{id}", s.getDevice)
	mux.HandleFunc("GET /api/v1/devices/{id}/interfaces", s.interfaces)
	mux.HandleFunc("GET /api/v1/devices/{id}/capabilities", s.capabilities)
	mux.HandleFunc("POST /api/v1/devices/{id}/demo-faults", s.faults)
	mux.HandleFunc("GET /api/v1/topology", s.topology)
	mux.HandleFunc("GET /api/v1/topology-diagram", s.topologyDiagram)
	mux.HandleFunc("POST /api/v1/topology-diagram", s.saveTopologyDiagram)
	mux.HandleFunc("GET /api/v1/topology-diagram/versions", s.topologyDiagramVersions)
	mux.HandleFunc("GET /api/v1/topology-diagram/versions/{id}", s.topologyDiagramVersion)
	mux.HandleFunc("POST /api/v1/topology-diagram/bindings", s.setTopologyBinding)
	mux.HandleFunc("POST /api/v1/topology-diagram/links", s.setTopologyLink)
	mux.HandleFunc("GET /api/v1/fabrics", s.listFabrics)
	mux.HandleFunc("POST /api/v1/fabrics", s.createFabric)
	mux.HandleFunc("GET /api/v1/tenants", s.listTenants)
	mux.HandleFunc("POST /api/v1/tenants", s.createTenant)
	mux.HandleFunc("GET /api/v1/segments", s.listSegments)
	mux.HandleFunc("POST /api/v1/segments", s.createSegment)
	mux.HandleFunc("GET /api/v1/policies", s.listPolicies)
	mux.HandleFunc("POST /api/v1/policies", s.createPolicy)
	mux.HandleFunc("GET /api/v1/policies/{id}", s.getPolicy)
	mux.HandleFunc("POST /api/v1/policies/{id}/versions", s.addVersion)
	mux.HandleFunc("POST /api/v1/policies/{id}/preview", s.preview)
	mux.HandleFunc("GET /api/v1/changes", s.listChanges)
	mux.HandleFunc("POST /api/v1/changes", s.createChange)
	mux.HandleFunc("GET /api/v1/changes/{id}", s.getChange)
	mux.HandleFunc("POST /api/v1/changes/{id}/approve", s.approve)
	mux.HandleFunc("POST /api/v1/changes/{id}/execute", s.execute)
	mux.HandleFunc("POST /api/v1/changes/{id}/cancel", s.cancel)
	mux.HandleFunc("GET /api/v1/desired-states", s.desired)
	mux.HandleFunc("GET /api/v1/observed-states", s.observed)
	mux.HandleFunc("GET /api/v1/drift-findings", s.drift)
	mux.HandleFunc("POST /api/v1/drift-findings/scan", s.scanDrift)
	mux.HandleFunc("GET /api/v1/verifications", s.verifications)
	mux.HandleFunc("GET /api/v1/backups", s.backups)
	mux.HandleFunc("POST /api/v1/backups", s.createBackup)
	mux.HandleFunc("POST /api/v1/backups/{id}/restore", s.restore)
	mux.HandleFunc("GET /api/v1/restores", s.restores)
	mux.HandleFunc("GET /api/v1/audit-events", s.audit)
	mux.HandleFunc("GET /api/v1/controllers/status", s.controllers)
	mux.HandleFunc("GET /api/v1/integrations", s.integrations)
	mux.HandleFunc("GET /api/v1/adapters", s.adapters)
	mux.HandleFunc("POST /api/v1/provisioning/vlan", s.proposeVLAN)
	mux.HandleFunc("GET /api/v1/interfaces", s.listInterfaces)
	mux.HandleFunc("GET /api/v1/endpoints", s.listEndpoints)
	mux.HandleFunc("GET /api/v1/vlans", s.listVLANs)
	mux.HandleFunc("GET /api/v1/config-revisions", s.configRevisions)
	mux.HandleFunc("POST /api/v1/devices/{id}/config/collect", s.collectConfig)
	mux.HandleFunc("POST /api/v1/devices/{id}/lab/overlay", s.simulatorOverlay)
	mux.HandleFunc("POST /api/v1/diagnostics", s.diagnostics)
	mux.HandleFunc("GET /api/v1/reports/availability", s.availability)
	mux.HandleFunc("GET /api/v1/service-accounts", s.serviceAccounts)
	mux.HandleFunc("POST /api/v1/service-accounts", s.serviceAccounts)
	mux.HandleFunc("POST /api/v1/service-accounts/{id}/revoke", s.revokeServiceAccount)
	mux.HandleFunc("GET /api/v1/memberships", s.memberships)
	mux.HandleFunc("POST /api/v1/memberships", s.addMembership)
	mux.HandleFunc("POST /api/v1/maintenance/retention", s.retention)
	mux.HandleFunc("POST /api/v1/agent/enroll", s.agentEnroll)
	mux.HandleFunc("POST /api/v1/agent/heartbeat", s.agentHeartbeat)
	mux.HandleFunc("GET /api/v1/agent/tasks", s.agentLease)
	mux.HandleFunc("POST /api/v1/agent/tasks/{id}/progress", s.jobProgress)
	mux.HandleFunc("POST /api/v1/agent/tasks/{id}/complete", s.agentComplete)
	return s.wrap(mux)
}

func (s *Server) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		origin := r.Header.Get("Origin")
		if origin != "" && s.originAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && origin != "" && !s.originAllowed(origin) {
			writeErr(w, apierr.Forbidden("origin is not allowed"))
			return
		}
		limit := int64(1 << 20)
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/topology-diagram" {
			limit = 16 << 20
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		requestID, _ := authn.NewToken()
		requestID = requestID[:12]
		w.Header().Set("X-Request-Id", requestID)
		ctx := context.WithValue(r.Context(), requestKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type requestKey struct{}

func requestID(ctx context.Context) string {
	v, _ := ctx.Value(requestKey{}).(string)
	return v
}

func (s *Server) sameSite() http.SameSite {
	if s.Config.CookieSameSite == "none" {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}

func (s *Server) originAllowed(origin string) bool {
	return originPermitted(s.Config.AllowedOrigins, s.Config.PublicAPIBase, origin)
}

func originPermitted(allowed []string, publicBase, origin string) bool {
	for _, item := range allowed {
		if origin == item {
			return true
		}
	}
	if origin == publicBase {
		return true
	}
	return sdnConsoleOrigin(origin)
}

func sdnConsoleOrigin(origin string) bool {
	host, ok := httpsHost(origin)
	if !ok {
		return false
	}
	if host == "sdn-psi.vercel.app" {
		return true
	}
	return strings.HasPrefix(host, "sdn-git-") && strings.HasSuffix(host, "-majd-6c6d.vercel.app")
}

func httpsHost(origin string) (string, bool) {
	const prefix = "https://"
	if !strings.HasPrefix(origin, prefix) {
		return "", false
	}
	host := strings.TrimPrefix(origin, prefix)
	if host == "" || strings.ContainsAny(host, "/?#@") {
		return "", false
	}
	return host, true
}

func (s *Server) allowedRedirect(uri string) bool {
	for _, allowed := range s.Config.OIDCRedirectURLs {
		if uri == allowed {
			return true
		}
	}
	return false
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.Pool().Ping(r.Context()); err != nil {
		writeErr(w, apierr.New(503, "unavailable", "database is not ready"))
		return
	}
	if pinger, ok := s.Secrets.(interface{ Ping(context.Context) error }); ok {
		if err := pinger.Ping(r.Context()); err != nil {
			writeErr(w, apierr.New(503, "unavailable", "secret backend is not ready"))
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte("sdn_up 1\n"))
}

func (s *Server) authConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"auth_mode": s.Config.AuthMode, "issuer": s.Config.PublicAPIBase + "/oidc",
		"client_id": s.Config.OIDCClientID, "authorization_endpoint": "/oidc/authorize",
	})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.Config.AuthMode != "dev" {
		writeErr(w, apierr.Forbidden("password login is disabled when AUTH_MODE=oidc"))
		return
	}
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	principal, err := s.Store.Authenticate(r.Context(), body.Email, body.Password)
	if err != nil {
		writeErr(w, apierr.Unauthorized())
		return
	}
	s.startSession(w, r, principal)
}

func (s *Server) sessionFromCode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code        string `json:"code"`
		Verifier    string `json:"code_verifier"`
		RedirectURI string `json:"redirect_uri"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	ac, err := s.Store.ConsumeAuthCode(r.Context(), body.Code, s.Config.OIDCClientID, body.RedirectURI)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := authn.VerifyPKCE(body.Verifier, ac.Challenge); err != nil {
		writeErr(w, apierr.Invalid("pkce verification failed"))
		return
	}
	principal, err := s.userPrincipal(r.Context(), ac.UserID, ac.OrganizationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	s.startSession(w, r, principal)
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, principal store.Principal) {
	token, err := s.Store.CreateSession(r.Context(), principal, s.Config.SessionTTL)
	if err != nil {
		writeErr(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: "sdn_session", Value: token, Path: "/", HttpOnly: true,
		SameSite: s.sameSite(), Secure: s.Config.SecureCookies, Expires: time.Now().Add(s.Config.SessionTTL),
	})
	_ = s.Store.Audit(r.Context(), principal.Organization.OrganizationID, principal.User.ID, "session.created", "user", principal.User.ID, requestID(r.Context()), map[string]string{"email": principal.User.Email})
	writeJSON(w, http.StatusOK, publicPrincipal(principal))
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("sdn_session"); err == nil {
		_ = s.Store.DeleteSession(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "sdn_session", Value: "", Path: "/", HttpOnly: true, MaxAge: -1, SameSite: s.sameSite(), Secure: s.Config.SecureCookies})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	principal, err := s.principal(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, publicPrincipal(principal))
}

func (s *Server) switchOrg(w http.ResponseWriter, r *http.Request) {
	principal, token, err := s.rawSession(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	_ = principal
	var body struct {
		OrganizationID string `json:"organization_id"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	next, err := s.Store.SwitchOrganization(r.Context(), token, body.OrganizationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, publicPrincipal(next))
}

func publicPrincipal(p store.Principal) map[string]any {
	return map[string]any{
		"id": p.User.ID, "email": p.User.Email, "display_name": p.User.DisplayName,
		"organization": p.Organization, "role": p.Organization.Role, "memberships": p.Memberships,
	}
}

func (s *Server) principal(r *http.Request) (store.Principal, error) {
	if header := r.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
		token := strings.TrimPrefix(header, "Bearer ")
		if token != "" && r.URL.Path != "/api/v1/agent/enroll" && !strings.HasPrefix(r.URL.Path, "/api/v1/agent/") {
			if p, err := s.Store.PrincipalFromServiceToken(r.Context(), token); err == nil {
				return p, nil
			}
		}
	}
	p, _, err := s.rawSession(r)
	return p, err
}

func (s *Server) rawSession(r *http.Request) (store.Principal, string, error) {
	cookie, err := r.Cookie("sdn_session")
	if err != nil || cookie.Value == "" {
		return store.Principal{}, "", apierr.Unauthorized()
	}
	p, err := s.Store.PrincipalFromSession(r.Context(), cookie.Value)
	if err != nil {
		var ae *apierr.Error
		if errors.As(err, &ae) && ae.Code == "not_found" {
			return store.Principal{}, "", apierr.Unauthorized()
		}
		return store.Principal{}, "", err
	}
	return p, cookie.Value, nil
}

func (s *Server) userPrincipal(ctx context.Context, userID, orgID string) (store.Principal, error) {
	return s.Store.PrincipalByID(ctx, userID, orgID)
}

func (s *Server) require(w http.ResponseWriter, r *http.Request, allow func(domain.Role) bool) (store.Principal, bool) {
	p, err := s.principal(r)
	if err != nil {
		writeErr(w, err)
		return store.Principal{}, false
	}
	if !allow(domain.Role(p.Organization.Role)) {
		writeErr(w, apierr.Forbidden(""))
		return store.Principal{}, false
	}
	return p, true
}

func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	sum, err := s.Store.Summary(r.Context(), p.Organization.OrganizationID, p.Organization.OrganizationName, strings.TrimSpace(r.URL.Query().Get("site_id")))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

func (s *Server) listSites(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	items, err := s.Store.ListSites(r.Context(), p.Organization.OrganizationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createSite(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanOperate)
	if !ok {
		return
	}
	var body struct {
		Name            string `json:"name"`
		Location        string `json:"location"`
		ExecutionDomain string `json:"execution_domain"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		writeErr(w, apierr.Invalid("name is required"))
		return
	}
	item, err := s.Store.CreateSite(r.Context(), p.Organization.OrganizationID, strings.TrimSpace(body.Name), body.Location, body.ExecutionDomain)
	if err != nil {
		writeErr(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), p.Organization.OrganizationID, p.User.ID, "site.created", "site", item.ID, requestID(r.Context()), map[string]string{"name": item.Name})
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) listConnectors(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	items, err := s.Store.ListConnectors(r.Context(), p.Organization.OrganizationID, r.URL.Query().Get("site_id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createConnector(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAdminister)
	if !ok {
		return
	}
	var body struct {
		Name        string `json:"name"`
		SiteID      string `json:"site_id"`
		Adapter     string `json:"adapter"`
		Credential  string `json:"credential"`
		Description string `json:"description"`
		Deployment  string `json:"deployment_method"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if body.Credential != "" {
		writeErr(w, apierr.Invalid("send a credential reference through the secret manager, not a secret value"))
		return
	}
	if body.Adapter == "" {
		body.Adapter = "site"
	}
	item, err := s.Store.CreateConnector(r.Context(), p.Organization.OrganizationID, body.SiteID, body.Name, body.Adapter, body.Description, body.Deployment)
	if err != nil {
		writeErr(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), p.Organization.OrganizationID, p.User.ID, "connector.created", "connector", item.ID, requestID(r.Context()), map[string]string{"adapter": item.Adapter, "credential_ref": item.CredentialRef})
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) enroll(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAdminister)
	if !ok {
		return
	}
	token, expires, err := s.Store.IssueEnrollment(r.Context(), p.Organization.OrganizationID, r.PathValue("id"), p.User.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), p.Organization.OrganizationID, p.User.ID, "connector.enrolled", "connector", r.PathValue("id"), requestID(r.Context()), map[string]string{"token": "redacted"})
	writeJSON(w, http.StatusCreated, map[string]any{
		"token": token, "expires_at": expires, "notice": "Copy this enrollment token now. It is stored only as a hash, expires in 15 minutes, and works once.",
	})
}

func (s *Server) discover(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanOperate)
	if !ok {
		return
	}
	conn, err := s.Store.GetConnector(r.Context(), p.Organization.OrganizationID, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if conn.Adapter == "ovn" {
		runner := ovn.Adapter{Runner: ovn.ExecRunner{}}
		if err := runner.Available(r.Context()); err != nil {
			writeErr(w, apierr.New(409, "implemented_unverified", "OVN adapter is implemented and has not been verified in a lab: "+err.Error()))
			return
		}
		switches, err := runner.ListLogicalSwitches(r.Context())
		if err != nil {
			writeErr(w, apierr.New(502, "connector", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"adapter": "ovn", "status": "implemented_unverified", "logical_switches": switches})
		return
	}
	if conn.Adapter != "simulator" {
		writeErr(w, apierr.Unsupported(conn.Adapter+" cannot discover devices"))
		return
	}
	if r.URL.Query().Get("via") == "agent" || conn.ExecutionMode == "site_agent" {
		siteName, nameErr := s.Store.SiteName(r.Context(), p.Organization.OrganizationID, conn.SiteID)
		if nameErr != nil {
			writeErr(w, nameErr)
			return
		}
		id, err := s.Store.EnqueueTask(r.Context(), p.Organization.OrganizationID, conn.ID, "discover", map[string]string{"site_id": conn.SiteID, "site_name": siteName})
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"task_id": id, "status": "pending", "notice": "The site agent must pull this task over its outbound connection."})
		return
	}
	siteName, err := s.Store.SiteName(r.Context(), p.Organization.OrganizationID, conn.SiteID)
	if err != nil {
		writeErr(w, err)
		return
	}
	devices, err := s.Store.PersistDiscovery(r.Context(), p.Organization.OrganizationID, conn.SiteID, conn.ID, adapter.SimulatorDiscovery(siteName))
	if err != nil {
		writeErr(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), p.Organization.OrganizationID, p.User.ID, "connector.discovered", "connector", conn.ID, requestID(r.Context()), map[string]any{"devices": len(devices), "data_origin": "simulator"})
	writeJSON(w, http.StatusOK, map[string]any{"data_origin": "simulator", "notice": "In-process simulator discovery. A site agent uses the same blueprint over an outbound connection.", "devices": devices})
}

func (s *Server) listDevices(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	items, err := s.Store.ListDevices(r.Context(), p.Organization.OrganizationID, r.URL.Query().Get("site_id"), r.URL.Query().Get("q"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getDevice(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	item, err := s.Store.GetDevice(r.Context(), p.Organization.OrganizationID, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	ifaces, _ := s.Store.ListInterfaces(r.Context(), p.Organization.OrganizationID, item.ID)
	caps, _ := s.Store.ListCapabilities(r.Context(), p.Organization.OrganizationID, item.ID)
	writeJSON(w, http.StatusOK, map[string]any{"device": item, "interfaces": ifaces, "capabilities": caps, "simulator_notice": item.SupportNote})
}

func (s *Server) interfaces(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	items, err := s.Store.ListInterfaces(r.Context(), p.Organization.OrganizationID, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) capabilities(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	items, err := s.Store.ListCapabilities(r.Context(), p.Organization.OrganizationID, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) faults(w http.ResponseWriter, r *http.Request) {
	if s.Config.AuthMode != "dev" {
		writeErr(w, apierr.Forbidden("fault injection is only available in the development simulator"))
		return
	}
	p, ok := s.require(w, r, domain.CanAuthor)
	if !ok {
		return
	}
	var body store.Faults
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if err := s.Store.SetFaults(r.Context(), p.Organization.OrganizationID, r.PathValue("id"), body); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notice": "Simulator fault armed. This does not affect a real device.", "faults": body})
}

func (s *Server) topology(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	item, err := s.Store.Topology(r.Context(), p.Organization.OrganizationID, r.URL.Query().Get("lens"), r.URL.Query().Get("site_id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) listFabrics(w http.ResponseWriter, r *http.Request) {
	s.readList(w, r, func(org string) (any, error) { return s.Store.ListFabrics(r.Context(), org) })
}
func (s *Server) listTenants(w http.ResponseWriter, r *http.Request) {
	s.readList(w, r, func(org string) (any, error) { return s.Store.ListTenants(r.Context(), org) })
}
func (s *Server) listSegments(w http.ResponseWriter, r *http.Request) {
	s.readList(w, r, func(org string) (any, error) {
		return s.Store.ListSegments(r.Context(), org, r.URL.Query().Get("tenant_id"))
	})
}

func (s *Server) readList(w http.ResponseWriter, r *http.Request, fn func(string) (any, error)) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	items, err := fn(p.Organization.OrganizationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createFabric(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAuthor)
	if !ok {
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	item, err := s.Store.CreateFabric(r.Context(), p.Organization.OrganizationID, body.Name, body.Description)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) createTenant(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAuthor)
	if !ok {
		return
	}
	var body struct {
		FabricID        string `json:"fabric_id"`
		Name            string `json:"name"`
		IsolationDomain string `json:"isolation_domain"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	item, err := s.Store.CreateTenant(r.Context(), p.Organization.OrganizationID, body.FabricID, body.Name, body.IsolationDomain)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) createSegment(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAuthor)
	if !ok {
		return
	}
	var body struct {
		TenantID string `json:"tenant_id"`
		Name     string `json:"name"`
		CIDR     string `json:"cidr"`
		VLAN     *int   `json:"vlan"`
		VNI      *int   `json:"vni"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	item, err := s.Store.CreateSegment(r.Context(), p.Organization.OrganizationID, body.TenantID, body.Name, body.CIDR, body.VLAN, body.VNI)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) listPolicies(w http.ResponseWriter, r *http.Request) {
	s.readList(w, r, func(org string) (any, error) { return s.Store.ListPolicies(r.Context(), org) })
}

func (s *Server) getPolicy(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	item, err := s.Store.GetPolicy(r.Context(), p.Organization.OrganizationID, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) createPolicy(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAuthor)
	if !ok {
		return
	}
	var spec domain.PolicySpec
	if err := decode(r, &spec); err != nil {
		writeErr(w, err)
		return
	}
	item, err := s.Store.CreatePolicy(r.Context(), p.Organization.OrganizationID, p.User.ID, spec)
	if err != nil {
		writeErr(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), p.Organization.OrganizationID, p.User.ID, "policy.created", "policy", item.ID, requestID(r.Context()), map[string]any{"revision": item.Revision})
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) addVersion(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAuthor)
	if !ok {
		return
	}
	var spec domain.PolicySpec
	if err := decode(r, &spec); err != nil {
		writeErr(w, err)
		return
	}
	item, err := s.Store.AddPolicyVersion(r.Context(), p.Organization.OrganizationID, p.User.ID, r.PathValue("id"), spec)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) preview(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAuthor)
	if !ok {
		return
	}
	policy, err := s.Store.GetPolicy(r.Context(), p.Organization.OrganizationID, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	impact, spec, err := s.Store.BuildImpact(r.Context(), p.Organization.OrganizationID, policy.VersionID)
	if err != nil {
		writeErr(w, err)
		return
	}
	hash, err := store.PlanHash(spec, impact)
	if err != nil {
		writeErr(w, err)
		return
	}
	exp := time.Now().Add(15 * time.Minute)
	token := domain.SignPreview(s.Config.TokenSecret, policy.VersionID, hash, exp)
	_ = s.Store.Audit(r.Context(), p.Organization.OrganizationID, p.User.ID, "policy.previewed", "policy_version", policy.VersionID, requestID(r.Context()), map[string]string{"plan_hash": hash})
	writeJSON(w, http.StatusOK, map[string]any{
		"impact": impact, "plan_hash": hash, "preview_token": token, "expires_at": exp,
		"policy_version_id": policy.VersionID,
		"notice":            "This preview is not authorization to change the network. Submit it for approval.",
	})
}

func (s *Server) listChanges(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	items, err := s.Store.ListChanges(r.Context(), p.Organization.OrganizationID, r.URL.Query().Get("status"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getChange(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	item, err := s.Store.GetChange(r.Context(), p.Organization.OrganizationID, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) createChange(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAuthor)
	if !ok {
		return
	}
	var body struct {
		PolicyVersionID string `json:"policy_version_id"`
		PlanHash        string `json:"plan_hash"`
		PreviewToken    string `json:"preview_token"`
	}
	raw, err := decodeRaw(r, &body)
	if err != nil {
		writeErr(w, err)
		return
	}
	versionID, hash, err := domain.VerifyPreview(s.Config.TokenSecret, body.PreviewToken, time.Now())
	if err != nil || versionID != body.PolicyVersionID || hash != body.PlanHash {
		writeErr(w, apierr.StalePlan())
		return
	}
	key := r.Header.Get("Idempotency-Key")
	reqHash := hex.EncodeToString(sha256Sum(raw))
	if key != "" {
		existing, err := s.Store.GetIdempotency(r.Context(), p.Organization.OrganizationID, key)
		if err != nil {
			writeErr(w, err)
			return
		}
		if existing.Found {
			if existing.RequestHash != reqHash {
				writeErr(w, apierr.Conflict("idempotency", "This idempotency key was used with a different request"))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(existing.StatusCode)
			_, _ = w.Write(existing.Response)
			return
		}
	}
	item, err := s.Store.CreateChange(r.Context(), p.Organization.OrganizationID, p.User.ID, body.PolicyVersionID, body.PlanHash, key)
	if err != nil {
		writeErr(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), p.Organization.OrganizationID, p.User.ID, "change.submitted", "change_request", item.ID, requestID(r.Context()), map[string]string{"plan_hash": item.PlanHash})
	payload := map[string]any{"data": item}
	if key != "" {
		_ = s.Store.PutIdempotency(r.Context(), p.Organization.OrganizationID, key, reqHash, http.StatusCreated, payload)
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) approve(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanApprove)
	if !ok {
		return
	}
	var body struct {
		Decision string `json:"decision"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if body.Decision == "" {
		body.Decision = "approved"
	}
	item, err := s.Store.DecideChange(r.Context(), p.Organization.OrganizationID, r.PathValue("id"), p.User.ID, body.Decision)
	if err != nil {
		writeErr(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), p.Organization.OrganizationID, p.User.ID, "change."+body.Decision, "change_request", item.ID, requestID(r.Context()), map[string]string{"plan_hash": item.PlanHash})
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) execute(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAuthor)
	if !ok {
		return
	}
	var body struct {
		Confirm  bool   `json:"confirm"`
		PlanHash string `json:"plan_hash"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if !body.Confirm {
		writeErr(w, apierr.Invalid("confirm must be true before a network change is queued"))
		return
	}
	current, err := s.Store.GetChange(r.Context(), p.Organization.OrganizationID, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if current.PlanHash != body.PlanHash {
		writeErr(w, apierr.StalePlan())
		return
	}
	switch current.Status {
	case "approved", "unknown", "queued", "applying", "verifying", "partial", "compensating":
	default:
		writeErr(w, apierr.Conflict("conflict", "Only an approved change can be executed"))
		return
	}
	orgID := p.Organization.OrganizationID
	changeID := current.ID
	if err := s.Store.ConfirmExecution(r.Context(), orgID, p.User.ID, changeID, body.PlanHash); err != nil {
		writeErr(w, err)
		return
	}
	if s.Config.InlineWorkflows {
		if err := s.Store.RunChange(r.Context(), orgID, changeID); err != nil {
			writeErr(w, err)
			return
		}
	} else if s.Workflows != nil {
		if err := s.Workflows.StartChange(r.Context(), orgID, changeID); err != nil {
			writeErr(w, err)
			return
		}
	} else {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			_ = s.Store.RunChange(ctx, orgID, changeID)
		}()
	}
	item, err := s.Store.GetChange(r.Context(), orgID, changeID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, item)
}

func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAuthor)
	if !ok {
		return
	}
	item, err := s.Store.RequestCancel(r.Context(), p.Organization.OrganizationID, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), p.Organization.OrganizationID, p.User.ID, "change.cancel_requested", "change_request", item.ID, requestID(r.Context()), nil)
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) desired(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	item, err := s.Store.LatestDesired(r.Context(), p.Organization.OrganizationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) observed(w http.ResponseWriter, r *http.Request) {
	s.readList(w, r, func(org string) (any, error) { return s.Store.ListObserved(r.Context(), org) })
}

func (s *Server) drift(w http.ResponseWriter, r *http.Request) {
	s.readList(w, r, func(org string) (any, error) { return s.Store.ListDrift(r.Context(), org) })
}

func (s *Server) scanDrift(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanOperate)
	if !ok {
		return
	}
	items, err := s.Store.ScanDrift(r.Context(), p.Organization.OrganizationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) verifications(w http.ResponseWriter, r *http.Request) {
	s.readList(w, r, func(org string) (any, error) { return s.Store.ListVerifications(r.Context(), org) })
}

func (s *Server) backups(w http.ResponseWriter, r *http.Request) {
	s.readList(w, r, func(org string) (any, error) { return s.Store.ListBackups(r.Context(), org) })
}

func (s *Server) restores(w http.ResponseWriter, r *http.Request) {
	s.readList(w, r, func(org string) (any, error) { return s.Store.ListRestores(r.Context(), org) })
}

func (s *Server) createBackup(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanOperate)
	if !ok {
		return
	}
	var body struct {
		Scope string `json:"scope"`
	}
	_ = decode(r, &body)
	item, err := s.Store.CreateBackup(r.Context(), p.Organization.OrganizationID, p.User.ID, body.Scope)
	if err != nil {
		writeErr(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), p.Organization.OrganizationID, p.User.ID, "backup.created", "configuration_backup", item.ID, requestID(r.Context()), map[string]string{"checksum": item.Checksum})
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) restore(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRestore)
	if !ok {
		return
	}
	item, err := s.Store.RestoreBackup(r.Context(), p.Organization.OrganizationID, p.User.ID, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), p.Organization.OrganizationID, p.User.ID, "backup.restore_requested", "configuration_backup", r.PathValue("id"), requestID(r.Context()), map[string]string{"status": item.Status})
	writeJSON(w, http.StatusAccepted, item)
}

func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	items, err := s.Store.ListAudit(r.Context(), p.Organization.OrganizationID, r.URL.Query().Get("action"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) controllers(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	item, err := s.Store.Controllers(r.Context(), p.Organization.OrganizationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	item.SecretBackend = s.Config.SecretBackend
	if item.SecretBackend == "" {
		item.SecretBackend = "memory"
	}
	item.Orchestration = "postgresql"
	item.Temporal = "not_configured"
	if s.Config.TemporalAddress != "" {
		item.Orchestration = "temporal"
		item.Temporal = "address_set_client_missing"
		if s.Workflows != nil {
			item.Temporal = "client_configured"
		}
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) integrations(w http.ResponseWriter, r *http.Request) {
	s.readList(w, r, func(org string) (any, error) { return s.Store.ListIntegrations(r.Context(), org) })
}

func (s *Server) memberships(w http.ResponseWriter, r *http.Request) {
	s.readList(w, r, func(org string) (any, error) { return s.Store.ListMemberships(r.Context(), org) })
}

func (s *Server) addMembership(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAdminister)
	if !ok {
		return
	}
	var body struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	user, err := s.Store.EnsureUser(r.Context(), body.Email, body.DisplayName, "")
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := s.Store.AddMembership(r.Context(), p.Organization.OrganizationID, user.ID, body.Role); err != nil {
		writeErr(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), p.Organization.OrganizationID, p.User.ID, "membership.added", "user", user.ID, requestID(r.Context()), map[string]string{"role": body.Role})
	writeJSON(w, http.StatusCreated, map[string]string{"user_id": user.ID, "role": body.Role})
}

func (s *Server) retention(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAdminister)
	if !ok {
		return
	}
	result, err := s.Store.ApplyRetention(r.Context(), p.Organization.OrganizationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) agent(r *http.Request) (store.AgentIdentity, error) {
	header := r.Header.Get("Authorization")
	token := strings.TrimPrefix(header, "Bearer ")
	if token == "" || token == header {
		return store.AgentIdentity{}, apierr.Unauthorized()
	}
	return s.Store.AgentFromToken(r.Context(), token)
}

func (s *Server) agentHeartbeat(w http.ResponseWriter, r *http.Request) {
	ident, err := s.agent(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		Health       string          `json:"health"`
		Version      string          `json:"version"`
		Host         json.RawMessage `json:"host"`
		Capabilities json.RawMessage `json:"capabilities"`
	}
	_ = decode(r, &body)
	if err := s.Store.ReportHeartbeat(r.Context(), ident.OrganizationID, ident.ConnectorID, store.HeartbeatInput{
		Health: body.Health, Version: body.Version, Host: body.Host, Capabilities: body.Capabilities,
	}); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) agentLease(w http.ResponseWriter, r *http.Request) {
	ident, err := s.agent(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	id, kind, payload, found, err := s.Store.LeaseTask(r.Context(), ident.OrganizationID, ident.ConnectorID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if kind == "discovery" && s.Secrets != nil {
		payload = scrubJobPayload(r.Context(), payload, func(ref string) (string, error) {
			return s.Secrets.Resolve(r.Context(), ref)
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"found": found, "id": id, "kind": kind, "payload": payload})
}

func (s *Server) agentComplete(w http.ResponseWriter, r *http.Request) {
	ident, err := s.agent(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		OK           bool                       `json:"ok"`
		Partial      bool                       `json:"partial"`
		Error        string                     `json:"error"`
		Discovery    adapter.Discovery          `json:"discovery"`
		Observations []store.InboundObservation `json:"observations"`
		Result       json.RawMessage            `json:"result"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if len(body.Observations) > 0 || body.Partial || body.Error != "" {
		if err := s.Store.CompleteDiscovery(r.Context(), ident.OrganizationID, ident.ConnectorID, r.PathValue("id"), body.Observations, body.Partial, body.Error); err == nil {
			writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
			return
		}
	}
	kind, connectorID, payload, err := s.Store.GetTask(r.Context(), ident.OrganizationID, r.PathValue("id"))
	if err != nil {
		if len(body.Observations) > 0 || !body.OK {
			if discErr := s.Store.CompleteDiscovery(r.Context(), ident.OrganizationID, ident.ConnectorID, r.PathValue("id"), body.Observations, body.Partial, body.Error); discErr == nil {
				writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
				return
			}
		}
		writeErr(w, err)
		return
	}
	if connectorID != ident.ConnectorID {
		writeErr(w, apierr.Forbidden("task belongs to another connector"))
		return
	}
	if kind == "discover" && body.OK {
		var meta struct {
			SiteID string `json:"site_id"`
		}
		_ = json.Unmarshal(payload, &meta)
		if _, err := s.Store.PersistDiscovery(r.Context(), ident.OrganizationID, meta.SiteID, connectorID, body.Discovery); err != nil {
			writeErr(w, err)
			return
		}
	}
	stored := map[string]any{"ok": body.OK, "error": body.Error}
	if len(body.Result) > 0 {
		stored["result"] = json.RawMessage(body.Result)
	}
	if err := s.Store.CompleteTask(r.Context(), ident.OrganizationID, r.PathValue("id"), stored, body.OK); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func decode(r *http.Request, dest any) error {
	_, err := decodeRaw(r, dest)
	return err
}

func decodeRaw(r *http.Request, dest any) ([]byte, error) {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dest); err != nil {
		return nil, apierr.Invalid("request body must be JSON")
	}
	raw, err := json.Marshal(dest)
	return raw, err
}

func sha256Sum(raw []byte) []byte {
	sum := sha256.Sum256(raw)
	return sum[:]
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func writeErr(w http.ResponseWriter, err error) {
	var ae *apierr.Error
	if !errors.As(err, &ae) {
		slog.Error("request failed", "err", err)
		ae = apierr.New(500, "internal", "The request could not be completed")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(ae.Status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": ae.Code, "message": ae.Message}})
}
