package httpserver

import (
	"net/http"

	"github.com/majdkh733-dev/SDN/internal/domain"
	"github.com/majdkh733-dev/SDN/internal/store"
)

func (s *Server) topologyDiagram(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	org := p.Organization.OrganizationID
	item, found, err := s.Store.LatestDiagram(r.Context(), org, store.ReferenceDiagramName)
	if err != nil {
		writeErr(w, err)
		return
	}
	bindings, err := s.Store.ListBindings(r.Context(), org)
	if err != nil {
		writeErr(w, err)
		return
	}
	if bindings == nil {
		bindings = []store.SymbolBinding{}
	}
	links, err := s.Store.ListLinkBindings(r.Context(), org)
	if err != nil {
		writeErr(w, err)
		return
	}
	if links == nil {
		links = []store.LinkBinding{}
	}
	if !found {
		writeJSON(w, http.StatusOK, map[string]any{
			"source": "reference", "name": store.ReferenceDiagramName, "bindings": bindings, "link_bindings": links,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"source": "saved", "name": item.Name, "version_id": item.ID, "updated_at": item.CreatedAt,
		"updated_by": item.CreatedBy, "mxfile": item.MXFile, "bindings": bindings, "link_bindings": links,
	})
}

func (s *Server) saveTopologyDiagram(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAuthor)
	if !ok {
		return
	}
	var body struct {
		MXFile string `json:"mxfile"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	item, err := s.Store.SaveDiagram(r.Context(), p.Organization.OrganizationID, p.User.ID, store.ReferenceDiagramName, body.MXFile)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) topologyDiagramVersions(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	items, err := s.Store.ListDiagramVersions(r.Context(), p.Organization.OrganizationID, store.ReferenceDiagramName)
	if err != nil {
		writeErr(w, err)
		return
	}
	if items == nil {
		items = []store.DiagramVersion{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) topologyDiagramVersion(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanRead)
	if !ok {
		return
	}
	item, err := s.Store.DiagramVersion(r.Context(), p.Organization.OrganizationID, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) setTopologyBinding(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAuthor)
	if !ok {
		return
	}
	var body struct {
		CellID   string `json:"cell_id"`
		DeviceID string `json:"device_id"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if err := s.Store.SetBinding(r.Context(), p.Organization.OrganizationID, p.User.ID, body.CellID, body.DeviceID); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"cell_id": body.CellID, "device_id": body.DeviceID})
}

func (s *Server) setTopologyLink(w http.ResponseWriter, r *http.Request) {
	p, ok := s.require(w, r, domain.CanAuthor)
	if !ok {
		return
	}
	var body store.LinkBinding
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if err := s.Store.SetLinkBinding(r.Context(), p.Organization.OrganizationID, p.User.ID, body); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, body)
}
