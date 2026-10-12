package httpserver

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/majdkh733-dev/SDN/internal/apierr"
	"github.com/majdkh733-dev/SDN/internal/discover"
	"github.com/majdkh733-dev/SDN/internal/domain"
	"github.com/majdkh733-dev/SDN/internal/secrets"
	"github.com/majdkh733-dev/SDN/internal/store"
)

func (s *Server) getConnector(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	item, err := s.Store.GetConnector(r.Context(), p.Organization.OrganizationID, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	jobs, _ := s.Store.ListJobs(r.Context(), p.Organization.OrganizationID, item.ID, "")
	writeJSON(w, http.StatusOK, map[string]any{
		"connector": item, "jobs": jobs, "troubleshooting": troubleshoot(item),
		"network": networkRequirements(s.Config.PublicAPIBase),
	})
}

func (s *Server) updateConnector(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAdminister)
	if !ok {
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Deployment  string `json:"deployment_method"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	item, err := s.Store.UpdateConnector(r.Context(), p.Organization.OrganizationID, r.PathValue("id"), body.Name, body.Description, body.Deployment)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) revokeConnector(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAdminister)
	if !ok {
		return
	}
	if err := s.Store.RevokeConnector(r.Context(), p.Organization.OrganizationID, r.PathValue("id")); err != nil {
		writeErr(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), p.Organization.OrganizationID, p.User.ID, "connector.revoked", "connector", r.PathValue("id"), requestID(r.Context()), map[string]string{"token": "redacted"})
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (s *Server) replaceConnector(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAdminister)
	if !ok {
		return
	}
	if err := s.Store.ReplaceConnector(r.Context(), p.Organization.OrganizationID, r.PathValue("id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "not_enrolled", "notice": "Previous credentials were revoked. Generate a new enrollment token."})
}

func (s *Server) removeConnector(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAdminister)
	if !ok {
		return
	}
	if err := s.Store.RemoveConnector(r.Context(), p.Organization.OrganizationID, r.PathValue("id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "archived"})
}

func (s *Server) installConnector(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	item, err := s.Store.GetConnector(r.Context(), p.Organization.OrganizationID, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	method := r.URL.Query().Get("method")
	if method == "" {
		method = item.DeploymentMethod
	}
	writeJSON(w, http.StatusOK, installInstructions(method, s.Config.PublicAPIBase, item.Name))
}

func (s *Server) agentEnroll(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EnrollmentToken string          `json:"enrollment_token"`
		Version         string          `json:"version"`
		Host            json.RawMessage `json:"host"`
		Capabilities    json.RawMessage `json:"capabilities"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if body.EnrollmentToken == "" {
		writeErr(w, apierr.Invalid("enrollment_token is required"))
		return
	}
	result, err := s.Store.ExchangeEnrollment(r.Context(), body.EnrollmentToken, body.Version, body.Host, body.Capabilities)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"credential": result.Credential, "connector_id": result.ConnectorID, "site_id": result.SiteID,
		"organization_id": result.OrganizationID, "poll_seconds": 15,
		"notice": "Store this credential. It is not shown again.",
	})
}

func (s *Server) createProfile(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAdminister)
	if !ok {
		return
	}
	var body struct {
		Name            string   `json:"name"`
		ConnectorID     string   `json:"connector_id"`
		Targets         []string `json:"targets"`
		Exclusions      []string `json:"exclusions"`
		ScanProfile     string   `json:"scan_profile"`
		TimeoutMS       int      `json:"timeout_ms"`
		Concurrency     int      `json:"concurrency"`
		ScheduleMinutes int      `json:"schedule_minutes"`
		AutoImport      bool     `json:"auto_import"`
		SNMPCommunity   string   `json:"snmp_community"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	ref := ""
	if body.SNMPCommunity != "" {
		if body.ScanProfile == "" {
			body.ScanProfile = "snmp"
		}
		ref = secrets.Reference(p.Organization.OrganizationID, "discovery-"+body.Name)
		if s.Secrets == nil {
			writeErr(w, apierr.New(503, "secrets", "secret backend is not configured"))
			return
		}
		if err := s.Secrets.Put(r.Context(), ref, body.SNMPCommunity); err != nil {
			writeErr(w, apierr.New(502, "secrets", "the read-only credential could not be stored"))
			return
		}
	}
	item, err := s.Store.CreateProfile(r.Context(), p.Organization.OrganizationID, r.PathValue("id"), store.ProfileInput{
		Name: body.Name, ConnectorID: body.ConnectorID, Targets: body.Targets, Exclusions: body.Exclusions,
		ScanProfile: body.ScanProfile, TimeoutMS: body.TimeoutMS, Concurrency: body.Concurrency,
		ScheduleMinutes: body.ScheduleMinutes, AutoImport: body.AutoImport, CredentialRef: ref,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) listProfiles(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	siteID := r.PathValue("id")
	if siteID == "" {
		siteID = r.URL.Query().Get("site_id")
	}
	items, err := s.Store.ListProfiles(r.Context(), p.Organization.OrganizationID, siteID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) runProfile(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanOperate)
	if !ok {
		return
	}
	job, err := s.Store.RunProfile(r.Context(), p.Organization.OrganizationID, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	job, err := s.Store.GetJob(r.Context(), p.Organization.OrganizationID, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	items, err := s.Store.ListJobs(r.Context(), p.Organization.OrganizationID, r.URL.Query().Get("connector_id"), r.URL.Query().Get("site_id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) cancelJob(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanOperate)
	if !ok {
		return
	}
	job, err := s.Store.CancelJob(r.Context(), p.Organization.OrganizationID, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) listObservations(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	items, err := s.Store.ListObservations(r.Context(), p.Organization.OrganizationID, r.PathValue("id"), r.URL.Query().Get("q"), r.URL.Query().Get("match"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) importObservations(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanOperate)
	if !ok {
		return
	}
	var body struct {
		IDs []string `json:"ids"`
		All bool     `json:"all"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	items, err := s.Store.ImportObservations(r.Context(), p.Organization.OrganizationID, p.User.ID, r.PathValue("id"), body.IDs, body.All)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) importCSV(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanOperate)
	if !ok {
		return
	}
	var body struct {
		CSV         string `json:"csv"`
		ConnectorID string `json:"connector_id"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	rows, err := parseAssetCSV(body.CSV)
	if err != nil {
		writeErr(w, err)
		return
	}
	items, err := s.Store.ImportCSV(r.Context(), p.Organization.OrganizationID, r.PathValue("id"), body.ConnectorID, rows)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) updateDevice(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanOperate)
	if !ok {
		return
	}
	var body struct {
		Name string   `json:"name"`
		Lock []string `json:"lock"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	item, err := s.Store.UpdateDeviceFields(r.Context(), p.Organization.OrganizationID, r.PathValue("id"), body.Name, body.Lock)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) preferences(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	item, err := s.Store.Preferences(r.Context(), p.Organization.OrganizationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) saveNavigation(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAdminister)
	if !ok {
		return
	}
	var body store.Navigation
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	item, err := s.Store.SaveNavigation(r.Context(), p.Organization.OrganizationID, body)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) resetNavigation(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAdminister)
	if !ok {
		return
	}
	item, err := s.Store.ResetNavigation(r.Context(), p.Organization.OrganizationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) saveTheme(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAdminister)
	if !ok {
		return
	}
	var body store.Theme
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	item, err := s.Store.SaveTheme(r.Context(), p.Organization.OrganizationID, body)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) resetTheme(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAdminister)
	if !ok {
		return
	}
	item, err := s.Store.ResetTheme(r.Context(), p.Organization.OrganizationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	item, err := s.Store.Search(r.Context(), p.Organization.OrganizationID, strings.TrimSpace(r.URL.Query().Get("q")))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) incidents(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	items, err := s.Store.Incidents(r.Context(), p.Organization.OrganizationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) updateSite(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanOperate)
	if !ok {
		return
	}
	var body struct {
		Location  string   `json:"location"`
		Latitude  *float64 `json:"latitude"`
		Longitude *float64 `json:"longitude"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	item, err := s.Store.UpdateSitePlace(r.Context(), p.Organization.OrganizationID, r.PathValue("id"), body.Location, body.Latitude, body.Longitude)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) createSiteLink(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanOperate)
	if !ok {
		return
	}
	var body struct {
		A         string   `json:"a_site_id"`
		B         string   `json:"b_site_id"`
		LatencyMS *float64 `json:"latency_ms"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	item, err := s.Store.CreateSiteLink(r.Context(), p.Organization.OrganizationID, body.A, body.B, body.LatencyMS)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) jobProgress(w http.ResponseWriter, r *http.Request) {
	ident, err := s.agent(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		Progress int `json:"progress"`
		Count    int `json:"discovered_count"`
	}
	_ = decode(r, &body)
	cancelled, err := s.Store.JobProgress(r.Context(), ident.OrganizationID, ident.ConnectorID, r.PathValue("id"), body.Progress, body.Count)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cancelled": cancelled})
}

func troubleshoot(item store.Connector) string {
	switch item.Status {
	case "not_enrolled":
		return "Generate an enrollment token and start the connector with outbound HTTPS to the API. The browser does not scan the site."
	case "connecting":
		return "A token was issued or a credential exists, and no recent heartbeat has arrived. Check that the connector process is running and can reach the API."
	case "offline":
		return "The last heartbeat is older than 90 seconds. Device health is left unchanged. Restart the connector or restore outbound HTTPS."
	case "degraded":
		return "The connector is checking in and reported degraded health. Read the host information and the latest job error."
	case "revoked":
		return "Credentials were revoked. Replace the connector to enroll it again. Existing assets stay in inventory."
	case "online":
		return "Heartbeat is current. Discovery runs only on this connector's jobs."
	default:
		return "Check enrollment, outbound HTTPS, and the latest discovery job."
	}
}

func networkRequirements(apiBase string) map[string]any {
	if apiBase == "" {
		apiBase = "the API origin in PUBLIC_API_BASE"
	}
	return map[string]any{
		"outbound": []string{
			"HTTPS to " + apiBase + " for enrollment, heartbeat, and job transfer",
			"DNS to the site resolver when reverse lookup is enabled",
			"ICMP echo, TCP probe ports, and optional SNMP UDP/161 only toward addresses in the discovery profile",
		},
		"inbound": "none",
		"note":    "Scan targets are limited to the profile allow-list, at most 256 IPv4 addresses, and no prefix shorter than /24.",
	}
}

func installInstructions(method, apiBase, name string) map[string]any {
	if apiBase == "" {
		apiBase = "http://127.0.0.1:8080"
	}
	unavailable := map[string]any{"method": method, "available": false, "reason": "This deployment method is not implemented. Use Docker or Linux."}
	switch method {
	case "docker", "":
		return map[string]any{
			"method": "docker", "available": true, "api": apiBase, "connector": name,
			"prerequisites": []string{"Docker Engine on a host inside the site network", "Outbound HTTPS to " + apiBase, "Host networking if ARP or ICMP must see the LAN"},
			"command": "docker build -f deploy/Dockerfile.connector -t sdn-connector:local .\n" +
				"docker run -d --name sdn-connector --restart unless-stopped --network host \\\n" +
				"  -v sdn-connector:/var/lib/sdn-connector \\\n" +
				"  -e SDN_API_URL='" + apiBase + "' \\\n" +
				"  -e SDN_ENROLLMENT_TOKEN='paste-the-one-time-token' \\\n" +
				"  -e SDN_AGENT_STATE=/var/lib/sdn-connector/credential \\\n" +
				"  sdn-connector:local",
			"uninstall": "docker rm -f sdn-connector\ndocker volume rm sdn-connector",
		}
	case "compose":
		return map[string]any{
			"method": "compose", "available": true, "api": apiBase,
			"prerequisites": []string{"Docker Compose on a host inside the site network", "Outbound HTTPS to " + apiBase},
			"command":       "SDN_API_URL='" + apiBase + "' SDN_ENROLLMENT_TOKEN='paste-the-one-time-token' docker compose -f deploy/connector-compose.yml up -d --build",
			"uninstall":     "docker compose -f deploy/connector-compose.yml down -v",
		}
	case "linux":
		return map[string]any{
			"method": "linux", "available": true, "api": apiBase,
			"prerequisites": []string{"Linux host inside the site network", "Outbound HTTPS to " + apiBase, "A binary built with go build -o sdn-connector ./connectors/site-agent"},
			"command": "sudo install -d -m 0700 /var/lib/sdn-connector /etc/sdn\n" +
				"sudo install -m 0755 sdn-connector /usr/local/bin/sdn-connector\n" +
				"sudo tee /etc/sdn/connector.env >/dev/null <<EOF\nSDN_API_URL=" + apiBase + "\nSDN_ENROLLMENT_TOKEN=paste-the-one-time-token\nSDN_AGENT_STATE=/var/lib/sdn-connector/credential\nEOF\n" +
				"sudo tee /etc/systemd/system/sdn-connector.service >/dev/null <<'EOF'\n" + systemdUnit + "EOF\n" +
				"sudo systemctl daemon-reload && sudo systemctl enable --now sdn-connector",
			"uninstall": "sudo systemctl disable --now sdn-connector\nsudo rm -f /etc/systemd/system/sdn-connector.service /usr/local/bin/sdn-connector\nsudo rm -rf /var/lib/sdn-connector /etc/sdn/connector.env\nsudo systemctl daemon-reload",
		}
	default:
		return unavailable
	}
}

const systemdUnit = `[Unit]
Description=SDN site connector
After=network-online.target
Wants=network-online.target

[Service]
EnvironmentFile=/etc/sdn/connector.env
ExecStart=/usr/local/bin/sdn-connector
Restart=on-failure
RestartSec=5
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
`

func parseAssetCSV(text string) ([]store.InboundObservation, error) {
	if strings.TrimSpace(text) == "" {
		return nil, apierr.Invalid("csv is empty")
	}
	reader := csv.NewReader(strings.NewReader(text))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, apierr.Invalid("csv could not be read")
	}
	if len(records) < 2 {
		return nil, apierr.Invalid("csv needs a header and one row")
	}
	index := map[string]int{}
	for i, column := range records[0] {
		index[strings.ToLower(strings.TrimSpace(column))] = i
	}
	var out []store.InboundObservation
	for _, row := range records[1:] {
		cell := func(name string) string {
			i, ok := index[name]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		out = append(out, store.InboundObservation{
			IP: cell("ip"), Hostname: firstNonEmpty(cell("hostname"), cell("name")), MAC: cell("mac"),
			Vendor: cell("vendor"), Model: cell("model"), DeviceType: cell("role"), Serial: cell("serial"),
			OSVersion: cell("os_version"), Reachable: false, Confidence: "low", Explicit: true,
		})
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func scrubJobPayload(ctx interface{ Deadline() (time.Time, bool) }, raw json.RawMessage, resolve func(string) (string, error)) json.RawMessage {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return raw
	}
	ref, _ := payload["credential_ref"].(string)
	delete(payload, "credential_ref")
	if ref != "" && resolve != nil {
		if value, err := resolve(ref); err == nil && value != "" {
			payload["snmp_community"] = value
		}
	}
	_ = ctx
	return jsonRaw(payload)
}

func jsonRaw(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return raw
}

// TargetsForJob expands the stored allow-list for an agent that received raw CIDR strings.
func TargetsForJob(raw json.RawMessage) ([]discover.Target, error) {
	var payload struct {
		Targets    []string `json:"targets"`
		Exclusions []string `json:"exclusions"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if len(payload.Targets) > 0 && !strings.Contains(payload.Targets[0], ".") && len(payload.Targets[0]) > 0 && payload.Targets[0][0] == '[' {
		return nil, apierr.Invalid("targets were not expanded")
	}
	return discover.Expand(payload.Targets, payload.Exclusions)
}
