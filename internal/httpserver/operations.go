package httpserver

import (
	"net/http"

	"github.com/majdkh733-dev/SDN/internal/adapter"
	"github.com/majdkh733-dev/SDN/internal/domain"
)

func (s *Server) adapters(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.require(w, r, domain.CanRead); !ok {
		return
	}
	writeJSON(w, http.StatusOK, adapter.Matrix())
}

func (s *Server) proposeVLAN(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAuthor)
	if !ok {
		return
	}
	var body struct {
		Name      string `json:"name"`
		TenantID  string `json:"tenant_id"`
		SiteID    string `json:"site_id"`
		DeviceID  string `json:"device_id"`
		VID       int    `json:"vid"`
		VLANName  string `json:"vlan_name"`
		Interface string `json:"interface"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	spec := domain.PolicySpec{
		Name: body.Name, TenantID: body.TenantID, SiteIDs: []string{body.SiteID}, Operation: "vlan",
		VLAN: &domain.VLANChange{DeviceID: body.DeviceID, VID: body.VID, Name: body.VLANName, Interface: body.Interface},
	}
	item, err := s.Store.ProposeVLAN(r.Context(), p.Organization.OrganizationID, p.User.ID, spec)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) listInterfaces(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	items, err := s.Store.ListInterfaceInventory(r.Context(), p.Organization.OrganizationID, r.URL.Query().Get("device_id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) listEndpoints(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	items, err := s.Store.ListEndpoints(r.Context(), p.Organization.OrganizationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) listVLANs(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	items, err := s.Store.ListVLANs(r.Context(), p.Organization.OrganizationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) collectConfig(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanOperate)
	if !ok {
		return
	}
	item, err := s.Store.CollectConfig(r.Context(), p.Organization.OrganizationID, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) configRevisions(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	items, err := s.Store.ListConfigRevisions(r.Context(), p.Organization.OrganizationID, r.URL.Query().Get("device_id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) simulatorOverlay(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanOperate)
	if !ok {
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if err := s.Store.SetSimulatorOverlay(r.Context(), p.Organization.OrganizationID, r.PathValue("id"), body.Body); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"notice": "Recorded on the simulator only. Collect configuration to compare it with the baseline."})
}

func (s *Server) diagnostics(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanOperate)
	if !ok {
		return
	}
	var body struct {
		ConnectorID string `json:"connector_id"`
		Kind        string `json:"kind"`
		Target      string `json:"target"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	id, err := s.Store.QueueDiagnostic(r.Context(), p.Organization.OrganizationID, body.ConnectorID, body.Kind, body.Target)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"id": id, "notice": "The site connector runs this probe. The API does not contact the target."})
}

func (s *Server) availability(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	item, err := s.Store.AvailabilityReport(r.Context(), p.Organization.OrganizationID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) serviceAccounts(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAdminister)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		items, err := s.Store.ListServiceAccounts(r.Context(), p.Organization.OrganizationID)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
		return
	}
	var body struct {
		Name string `json:"name"`
		Role string `json:"role"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	item, err := s.Store.CreateServiceAccount(r.Context(), p.Organization.OrganizationID, p.User.ID, body.Name, body.Role)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) revokeServiceAccount(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAdminister)
	if !ok {
		return
	}
	if err := s.Store.RevokeServiceAccount(r.Context(), p.Organization.OrganizationID, r.PathValue("id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
