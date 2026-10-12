package store

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/majdkh733-dev/SDN/internal/apierr"
)

func appendSiteNodes(ctx context.Context, tx pgx.Tx, orgID string, topo *Topology) error {
	rows, err := tx.Query(ctx, `
		SELECT id::text, name, location, latitude, longitude
		FROM sites WHERE organization_id = $1 AND lifecycle = 'active' ORDER BY name
	`, orgID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name, location string
		var lat, lng *float64
		if err := rows.Scan(&id, &name, &location, &lat, &lng); err != nil {
			return err
		}
		topo.Nodes = append(topo.Nodes, TopoNode{
			ID: "site:" + id, Label: name, Kind: "site", Health: "active", SiteID: id,
			Meta: map[string]any{"location": location, "latitude": lat, "longitude": lng},
		})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	erows, err := tx.Query(ctx, `
		SELECT id::text, a_site_id::text, b_site_id::text, source, status, latency_ms
		FROM site_links WHERE organization_id = $1
	`, orgID)
	if err != nil {
		return err
	}
	defer erows.Close()
	for erows.Next() {
		var edge TopoEdge
		var status string
		if err := erows.Scan(&edge.ID, &edge.Source, &edge.Target, &edge.Origin, &status, &edge.LatencyMS); err != nil {
			return err
		}
		edge.Source = "site:" + edge.Source
		edge.Target = "site:" + edge.Target
		edge.Label = status
		edge.Measured = edge.LatencyMS != nil
		if !edge.Measured {
			edge.Label = edge.Origin
		}
		topo.Edges = append(topo.Edges, edge)
	}
	return erows.Err()
}

type SiteLink struct {
	ID        string   `json:"id"`
	ASiteID   string   `json:"a_site_id"`
	BSiteID   string   `json:"b_site_id"`
	Source    string   `json:"source"`
	Status    string   `json:"status"`
	LatencyMS *float64 `json:"latency_ms,omitempty"`
}

func (s *Store) CreateSiteLink(ctx context.Context, orgID, a, b string, latency *float64) (SiteLink, error) {
	if a == b {
		return SiteLink{}, apierr.Invalid("a site link needs two different sites")
	}
	var link SiteLink
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return mapDB(tx.QueryRow(ctx, `
			INSERT INTO site_links (organization_id, a_site_id, b_site_id, source, status, latency_ms)
			VALUES ($1, $2, $3, 'manual', 'unknown', $4)
			RETURNING id::text, a_site_id::text, b_site_id::text, source, status, latency_ms
		`, orgID, a, b, latency).Scan(&link.ID, &link.ASiteID, &link.BSiteID, &link.Source, &link.Status, &link.LatencyMS), "site link")
	})
	return link, err
}

func (s *Store) UpdateSitePlace(ctx context.Context, orgID, id, location string, lat, lng *float64) (Site, error) {
	var site Site
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return mapDB(tx.QueryRow(ctx, `
			UPDATE sites
			SET location = CASE WHEN $3 = '' THEN location ELSE $3 END,
			    latitude = $4, longitude = $5
			WHERE organization_id = $1 AND id = $2
			RETURNING id::text, name, location, execution_domain, lifecycle, latitude, longitude, created_at
		`, orgID, id, location, lat, lng).Scan(
			&site.ID, &site.Name, &site.Location, &site.ExecutionDomain, &site.Lifecycle, &site.Latitude, &site.Longitude, &site.CreatedAt,
		), "site")
	})
	return site, err
}

func (s *Store) Search(ctx context.Context, orgID, query string) (map[string]any, error) {
	result := map[string]any{"devices": []any{}, "sites": []any{}, "segments": []any{}, "policies": []any{}}
	if query == "" {
		return result, nil
	}
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		devices := []map[string]string{}
		rows, err := tx.Query(ctx, `
			SELECT id::text, name, site_id::text FROM devices
			WHERE organization_id = $1 AND lifecycle = 'active'
			  AND (name ILIKE '%' || $2 || '%' OR management_endpoint ILIKE '%' || $2 || '%' OR hostname ILIKE '%' || $2 || '%')
			ORDER BY name LIMIT 8
		`, orgID, query)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id, name, site string
			if err := rows.Scan(&id, &name, &site); err != nil {
				rows.Close()
				return err
			}
			devices = append(devices, map[string]string{"id": id, "name": name, "site_id": site})
		}
		rows.Close()
		sites := []map[string]string{}
		rows, err = tx.Query(ctx, `
			SELECT id::text, name FROM sites
			WHERE organization_id = $1 AND name ILIKE '%' || $2 || '%'
			ORDER BY name LIMIT 8
		`, orgID, query)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				rows.Close()
				return err
			}
			sites = append(sites, map[string]string{"id": id, "name": name})
		}
		rows.Close()
		segments := []map[string]string{}
		rows, err = tx.Query(ctx, `
			SELECT id::text, name FROM segments
			WHERE organization_id = $1 AND lifecycle = 'active' AND (name ILIKE '%' || $2 || '%' OR cidr ILIKE '%' || $2 || '%')
			ORDER BY name LIMIT 8
		`, orgID, query)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				rows.Close()
				return err
			}
			segments = append(segments, map[string]string{"id": id, "name": name})
		}
		rows.Close()
		policies := []map[string]string{}
		rows, err = tx.Query(ctx, `
			SELECT id::text, name FROM policies
			WHERE organization_id = $1 AND name ILIKE '%' || $2 || '%'
			ORDER BY name LIMIT 8
		`, orgID, query)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				rows.Close()
				return err
			}
			policies = append(policies, map[string]string{"id": id, "name": name})
		}
		rows.Close()
		result["devices"] = devices
		result["sites"] = sites
		result["segments"] = segments
		result["policies"] = policies
		return rows.Err()
	})
	return result, err
}

func (s *Store) Incidents(ctx context.Context, orgID string) ([]map[string]any, error) {
	var out []map[string]any
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT 'connector' AS kind, c.id::text, c.name, s.name, c.last_heartbeat_at
			FROM connectors c JOIN sites s ON s.id = c.site_id
			WHERE c.organization_id = $1 AND c.lifecycle = 'active' AND c.revoked_at IS NULL
			  AND c.last_heartbeat_at IS NOT NULL AND c.last_heartbeat_at < now() - interval '90 seconds'
		`, orgID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var kind, id, name, site string
			var at any
			if err := rows.Scan(&kind, &id, &name, &site, &at); err != nil {
				rows.Close()
				return err
			}
			out = append(out, map[string]any{
				"id": id, "severity": "P1", "kind": "connector_offline", "title": name + " is offline",
				"site": site, "at": at, "href": "/connectors/" + id,
				"detail": "The connector missed its heartbeat. Device health was not changed.",
			})
		}
		rows.Close()
		rows, err = tx.Query(ctx, `
			SELECT f.id::text, d.name, f.difference::text, f.created_at
			FROM drift_findings f JOIN devices d ON d.id = f.device_id
			WHERE f.organization_id = $1 AND f.disposition = 'open'
			ORDER BY f.created_at DESC LIMIT 20
		`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id, resource, diff string
			var at any
			if err := rows.Scan(&id, &resource, &diff, &at); err != nil {
				return err
			}
			out = append(out, map[string]any{
				"id": id, "severity": "P2", "kind": "drift", "title": "Policy drift on " + resource,
				"site": "", "at": at, "href": "/assurance", "detail": diff,
			})
		}
		return rows.Err()
	})
	if out == nil {
		out = []map[string]any{}
	}
	return out, err
}
