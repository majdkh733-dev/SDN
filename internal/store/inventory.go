package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/majdkh733-dev/SDN/internal/adapter"
	"github.com/majdkh733-dev/SDN/internal/apierr"
	"github.com/majdkh733-dev/SDN/internal/authn"
	"github.com/majdkh733-dev/SDN/internal/secrets"
)

func (s *Store) CreateOrganization(ctx context.Context, id, name, slug string, retention int) error {
	return s.WithOrg(ctx, id, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO organizations (id, name, slug, retention_days) VALUES ($1, $2, $3, $4)
		`, id, name, slug, retention)
		return err
	})
}

func (s *Store) OrgBySlug(ctx context.Context, slug string) (string, string, error) {
	var id, name string
	err := s.WithBypass(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT id::text, name FROM organizations WHERE slug = $1`, slug).Scan(&id, &name)
	})
	return id, name, mapDB(err, "organization")
}

func (s *Store) ListSites(ctx context.Context, orgID string) ([]Site, error) {
	var out []Site
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, name, location, execution_domain, lifecycle, created_at
			FROM sites WHERE organization_id = $1 ORDER BY name
		`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var site Site
			if err := rows.Scan(&site.ID, &site.Name, &site.Location, &site.ExecutionDomain, &site.Lifecycle, &site.CreatedAt); err != nil {
				return err
			}
			out = append(out, site)
		}
		return rows.Err()
	})
	if out == nil {
		out = []Site{}
	}
	return out, err
}

func (s *Store) CreateSite(ctx context.Context, orgID, name, location, domain string) (Site, error) {
	var site Site
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if domain == "" {
			domain = "default"
		}
		return tx.QueryRow(ctx, `
			INSERT INTO sites (organization_id, name, location, execution_domain)
			VALUES ($1, $2, $3, $4)
			RETURNING id::text, name, location, execution_domain, lifecycle, created_at
		`, orgID, name, location, domain).Scan(&site.ID, &site.Name, &site.Location, &site.ExecutionDomain, &site.Lifecycle, &site.CreatedAt)
	})
	return site, mapDB(err, "site")
}

func supportFor(adapterName string) string {
	switch adapterName {
	case "simulator":
		return "demo"
	case "ovn":
		return "implemented_unverified"
	default:
		return "not_implemented"
	}
}

func (s *Store) CreateConnector(ctx context.Context, orgID, siteID, name, adapterName string) (Connector, error) {
	if adapterName != "simulator" && adapterName != "ovn" {
		return Connector{}, apierr.Unsupported(adapterName + " is on the roadmap and cannot be enrolled")
	}
	var conn Connector
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		id, err := s.NewID(ctx)
		if err != nil {
			return err
		}
		ref := secrets.Reference(orgID, id)
		mode := "site_agent"
		if adapterName == "simulator" {
			mode = "inprocess"
		}
		return tx.QueryRow(ctx, `
			INSERT INTO connectors (
				id, organization_id, site_id, name, adapter, execution_mode, credential_ref, support_status, health
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'unknown')
			RETURNING id::text, site_id::text, name, adapter, version, lifecycle, health, execution_mode,
			          last_heartbeat_at, credential_ref, support_status, created_at
		`, id, orgID, siteID, name, adapterName, mode, ref, supportFor(adapterName)).Scan(
			&conn.ID, &conn.SiteID, &conn.Name, &conn.Adapter, &conn.Version, &conn.Lifecycle, &conn.Health,
			&conn.ExecutionMode, &conn.LastHeartbeat, &conn.CredentialRef, &conn.SupportStatus, &conn.CreatedAt,
		)
	})
	return conn, mapDB(err, "connector")
}

func (s *Store) ListConnectors(ctx context.Context, orgID string) ([]Connector, error) {
	var out []Connector
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, site_id::text, name, adapter, version, lifecycle, health, execution_mode,
			       last_heartbeat_at, credential_ref, support_status, created_at
			FROM connectors WHERE organization_id = $1 ORDER BY name
		`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			item, err := scanConnector(rows)
			if err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []Connector{}
	}
	return out, err
}

func scanConnector(row interface{ Scan(...any) error }) (Connector, error) {
	var conn Connector
	err := row.Scan(
		&conn.ID, &conn.SiteID, &conn.Name, &conn.Adapter, &conn.Version, &conn.Lifecycle, &conn.Health,
		&conn.ExecutionMode, &conn.LastHeartbeat, &conn.CredentialRef, &conn.SupportStatus, &conn.CreatedAt,
	)
	return conn, err
}

func (s *Store) IssueEnrollment(ctx context.Context, orgID, connectorID string) (string, error) {
	token, err := authn.NewToken()
	if err != nil {
		return "", err
	}
	err = s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT true FROM connectors WHERE organization_id = $1 AND id = $2`, orgID, connectorID).Scan(&exists); err != nil {
			return mapDB(err, "connector")
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO connector_tokens (organization_id, connector_id, token_hash) VALUES ($1, $2, $3)
		`, orgID, connectorID, authn.Hash(token))
		return err
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

type AgentIdentity struct {
	OrganizationID string
	ConnectorID    string
	SiteID         string
	Adapter        string
}

func (s *Store) AgentFromToken(ctx context.Context, token string) (AgentIdentity, error) {
	var ident AgentIdentity
	err := s.WithBypass(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT c.organization_id::text, c.id::text, c.site_id::text, c.adapter
			FROM connector_tokens t
			JOIN connectors c ON c.id = t.connector_id
			WHERE t.token_hash = $1 AND t.revoked_at IS NULL AND c.lifecycle = 'active'
		`, authn.Hash(token)).Scan(&ident.OrganizationID, &ident.ConnectorID, &ident.SiteID, &ident.Adapter)
	})
	return ident, mapDB(err, "connector token")
}

func (s *Store) Heartbeat(ctx context.Context, orgID, connectorID, health string) error {
	if health == "" {
		health = "healthy"
	}
	return s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE connectors
			SET health = $3, last_heartbeat_at = now(), execution_mode = 'site_agent'
			WHERE organization_id = $1 AND id = $2
		`, orgID, connectorID, health)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return apierr.NotFound("connector")
		}
		return nil
	})
}

func (s *Store) GetConnector(ctx context.Context, orgID, id string) (Connector, error) {
	var conn Connector
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id::text, site_id::text, name, adapter, version, lifecycle, health, execution_mode,
			       last_heartbeat_at, credential_ref, support_status, created_at
			FROM connectors WHERE organization_id = $1 AND id = $2
		`, orgID, id)
		var err error
		conn, err = scanConnector(row)
		return mapDB(err, "connector")
	})
	return conn, err
}

func (s *Store) SiteName(ctx context.Context, orgID, siteID string) (string, error) {
	var name string
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return mapDB(tx.QueryRow(ctx, `SELECT name FROM sites WHERE organization_id = $1 AND id = $2`, orgID, siteID).Scan(&name), "site")
	})
	return name, err
}

func (s *Store) PersistDiscovery(ctx context.Context, orgID, siteID, connectorID string, found adapter.Discovery) ([]Device, error) {
	var devices []Device
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		now := time.Now()
		var ids []string
		for i, blueprint := range found.Devices {
			var id string
			err := tx.QueryRow(ctx, `
				INSERT INTO devices (
					organization_id, site_id, connector_id, name, model, os_version, role,
					management_endpoint, health, data_origin, support_note, observed_at
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'healthy',$9,$10,$11)
				ON CONFLICT (organization_id, name) DO UPDATE
				SET model = EXCLUDED.model,
				    os_version = EXCLUDED.os_version,
				    health = 'healthy',
				    observed_at = EXCLUDED.observed_at,
				    support_note = EXCLUDED.support_note,
				    connector_id = EXCLUDED.connector_id
				RETURNING id::text
			`, orgID, siteID, connectorID, blueprint.Name, blueprint.Model, blueprint.OSVersion, blueprint.Role,
				"sim://"+blueprint.Name, found.DataOrigin, found.SupportNote, now).Scan(&id)
			if err != nil {
				return err
			}
			ids = append(ids, id)
			if _, err := tx.Exec(ctx, `DELETE FROM capability_snapshots WHERE organization_id = $1 AND device_id = $2`, orgID, id); err != nil {
				return err
			}
			for _, cap := range found.Capabilities {
				if _, err := tx.Exec(ctx, `
					INSERT INTO capability_snapshots (organization_id, device_id, capability, supported, evidence, connector_version)
					VALUES ($1, $2, $3, true, $4, '0.1.0')
				`, orgID, id, cap, "Reported by the "+found.DataOrigin+" adapter. Not a physical lab result."); err != nil {
					return err
				}
			}
			for n, iface := range blueprint.Interfaces {
				mac := fmt.Sprintf("02:00:00:%02x:%02x:%02x", i+1, n+1, len(blueprint.Name))
				speed := 10000
				if _, err := tx.Exec(ctx, `
					INSERT INTO interfaces (organization_id, device_id, name, mac, speed_mbps, addresses)
					VALUES ($1, $2, $3, $4, $5, $6::jsonb)
					ON CONFLICT (device_id, name) DO UPDATE SET mac = EXCLUDED.mac, oper_up = true
				`, orgID, id, iface, mac, speed, fmt.Sprintf(`["10.%d.%d.1/24"]`, i+1, n+1)); err != nil {
					return err
				}
			}
		}
		if len(ids) >= 2 {
			var aID, bID string
			_ = tx.QueryRow(ctx, `SELECT id::text FROM interfaces WHERE device_id = $1 AND name = 'eth2'`, ids[0]).Scan(&aID)
			_ = tx.QueryRow(ctx, `SELECT id::text FROM interfaces WHERE device_id = $1 AND name = 'eth1'`, ids[1]).Scan(&bID)
			if aID != "" && bID != "" {
				_, err := tx.Exec(ctx, `
					INSERT INTO topology_links (organization_id, site_id, a_interface_id, b_interface_id, source, utilization_pct)
					SELECT $1, $2, $3, $4, 'simulator', 18.5
					WHERE NOT EXISTS (
						SELECT 1 FROM topology_links
						WHERE organization_id = $1 AND a_interface_id = $3 AND b_interface_id = $4
					)
				`, orgID, siteID, aID, bID)
				if err != nil {
					return err
				}
			}
		}
		_, err := tx.Exec(ctx, `
			UPDATE connectors
			SET health = 'healthy', last_heartbeat_at = now()
			WHERE organization_id = $1 AND id = $2
		`, orgID, connectorID)
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, deviceSelect+` WHERE organization_id = $1 AND connector_id = $2 ORDER BY name`, orgID, connectorID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			item, err := scanDevice(rows)
			if err != nil {
				return err
			}
			devices = append(devices, item)
		}
		return rows.Err()
	})
	if devices == nil {
		devices = []Device{}
	}
	return devices, err
}

const deviceSelect = `
	SELECT id::text, site_id::text, COALESCE(connector_id::text, ''), name, model, os_version, role,
	       management_endpoint, lifecycle, health, data_origin, support_note, observed_at, created_at,
	       fail_next_write, fail_next_rollback, uncertain_next_write, uncertain_committed
	FROM devices`

func scanDevice(row interface{ Scan(...any) error }) (Device, error) {
	var d Device
	err := row.Scan(
		&d.ID, &d.SiteID, &d.ConnectorID, &d.Name, &d.Model, &d.OSVersion, &d.Role,
		&d.ManagementEndpoint, &d.Lifecycle, &d.Health, &d.DataOrigin, &d.SupportNote, &d.ObservedAt, &d.CreatedAt,
		&d.FailNextWrite, &d.FailNextRollback, &d.UncertainNextWrite, &d.UncertainCommitted,
	)
	return d, err
}

func (s *Store) ListDevices(ctx context.Context, orgID, siteID, query string) ([]Device, error) {
	var out []Device
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, deviceSelect+`
			WHERE organization_id = $1
			  AND ($2 = '' OR site_id::text = $2)
			  AND ($3 = '' OR name ILIKE '%' || $3 || '%' OR model ILIKE '%' || $3 || '%')
			ORDER BY name
		`, orgID, siteID, query)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			item, err := scanDevice(rows)
			if err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []Device{}
	}
	return out, err
}

func (s *Store) GetDevice(ctx context.Context, orgID, id string) (Device, error) {
	var d Device
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		d, err = scanDevice(tx.QueryRow(ctx, deviceSelect+` WHERE organization_id = $1 AND id = $2`, orgID, id))
		return mapDB(err, "device")
	})
	return d, err
}

func (s *Store) ListInterfaces(ctx context.Context, orgID, deviceID string) ([]Interface, error) {
	var out []Interface
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.deviceExists(ctx, tx, orgID, deviceID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT id::text, device_id::text, name, mac, admin_up, oper_up, speed_mbps, addresses
			FROM interfaces WHERE organization_id = $1 AND device_id = $2 ORDER BY name
		`, orgID, deviceID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Interface
			var raw []byte
			if err := rows.Scan(&item.ID, &item.DeviceID, &item.Name, &item.MAC, &item.AdminUp, &item.OperUp, &item.SpeedMbps, &raw); err != nil {
				return err
			}
			_ = jsonUnmarshal(raw, &item.Addresses)
			if item.Addresses == nil {
				item.Addresses = []string{}
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []Interface{}
	}
	return out, err
}

func (s *Store) deviceExists(ctx context.Context, tx pgx.Tx, orgID, deviceID string) (string, error) {
	var name string
	err := tx.QueryRow(ctx, `SELECT name FROM devices WHERE organization_id = $1 AND id = $2`, orgID, deviceID).Scan(&name)
	return name, mapDB(err, "device")
}

func (s *Store) ListCapabilities(ctx context.Context, orgID, deviceID string) ([]Capability, error) {
	var out []Capability
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.deviceExists(ctx, tx, orgID, deviceID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT DISTINCT ON (capability) capability, supported, evidence, connector_version, discovered_at
			FROM capability_snapshots
			WHERE organization_id = $1 AND device_id = $2
			ORDER BY capability, discovered_at DESC
		`, orgID, deviceID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Capability
			if err := rows.Scan(&item.Capability, &item.Supported, &item.Evidence, &item.ConnectorVersion, &item.DiscoveredAt); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []Capability{}
	}
	return out, err
}

type Faults struct {
	FailNextWrite      bool `json:"fail_next_write"`
	FailNextRollback   bool `json:"fail_next_rollback"`
	UncertainNextWrite bool `json:"uncertain_next_write"`
	UncertainCommitted bool `json:"uncertain_committed"`
}

func (s *Store) SetFaults(ctx context.Context, orgID, deviceID string, faults Faults) error {
	return s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE devices
			SET fail_next_write = $3, fail_next_rollback = $4, uncertain_next_write = $5, uncertain_committed = $6
			WHERE organization_id = $1 AND id = $2 AND data_origin = 'simulator'
		`, orgID, deviceID, faults.FailNextWrite, faults.FailNextRollback, faults.UncertainNextWrite, faults.UncertainCommitted)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return apierr.NotFound("simulator device")
		}
		return nil
	})
}

func (s *Store) CreateFabric(ctx context.Context, orgID, name, description string) (Fabric, error) {
	var item Fabric
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO fabrics (organization_id, name, description)
			VALUES ($1, $2, $3)
			RETURNING id::text, name, description, lifecycle, created_at
		`, orgID, name, description).Scan(&item.ID, &item.Name, &item.Description, &item.Lifecycle, &item.CreatedAt)
	})
	return item, mapDB(err, "fabric")
}

func (s *Store) ListFabrics(ctx context.Context, orgID string) ([]Fabric, error) {
	return queryFabrics(ctx, s, orgID)
}

func queryFabrics(ctx context.Context, s *Store, orgID string) ([]Fabric, error) {
	var out []Fabric
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id::text, name, description, lifecycle, created_at FROM fabrics WHERE organization_id = $1 ORDER BY name`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Fabric
			if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Lifecycle, &item.CreatedAt); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []Fabric{}
	}
	return out, err
}

func (s *Store) CreateTenant(ctx context.Context, orgID, fabricID, name, isolation string) (Tenant, error) {
	var item Tenant
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO tenants (organization_id, fabric_id, name, isolation_domain)
			VALUES ($1, $2, $3, $4)
			RETURNING id::text, fabric_id::text, name, isolation_domain, lifecycle, created_at
		`, orgID, fabricID, name, isolation).Scan(&item.ID, &item.FabricID, &item.Name, &item.IsolationDomain, &item.Lifecycle, &item.CreatedAt)
	})
	return item, mapDB(err, "tenant")
}

func (s *Store) ListTenants(ctx context.Context, orgID string) ([]Tenant, error) {
	var out []Tenant
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, fabric_id::text, name, isolation_domain, lifecycle, created_at
			FROM tenants WHERE organization_id = $1 ORDER BY name
		`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Tenant
			if err := rows.Scan(&item.ID, &item.FabricID, &item.Name, &item.IsolationDomain, &item.Lifecycle, &item.CreatedAt); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []Tenant{}
	}
	return out, err
}

func (s *Store) CreateSegment(ctx context.Context, orgID, tenantID, name, cidr string, vlan, vni *int) (Segment, error) {
	var item Segment
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO segments (organization_id, tenant_id, name, cidr, vlan, vni)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id::text, tenant_id::text, name, cidr, vlan, vni, lifecycle, created_at
		`, orgID, tenantID, name, cidr, vlan, vni).Scan(&item.ID, &item.TenantID, &item.Name, &item.CIDR, &item.VLAN, &item.VNI, &item.Lifecycle, &item.CreatedAt)
	})
	return item, mapDB(err, "segment")
}

func (s *Store) ListSegments(ctx context.Context, orgID, tenantID string) ([]Segment, error) {
	var out []Segment
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, tenant_id::text, name, cidr, vlan, vni, lifecycle, created_at
			FROM segments
			WHERE organization_id = $1 AND ($2 = '' OR tenant_id::text = $2)
			ORDER BY name
		`, orgID, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Segment
			if err := rows.Scan(&item.ID, &item.TenantID, &item.Name, &item.CIDR, &item.VLAN, &item.VNI, &item.Lifecycle, &item.CreatedAt); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []Segment{}
	}
	return out, err
}
