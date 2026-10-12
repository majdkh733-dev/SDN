package store

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/majdkh733-dev/SDN/internal/apierr"
	"github.com/majdkh733-dev/SDN/internal/authn"
	"github.com/majdkh733-dev/SDN/internal/domain"
)

type ServiceAccount struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Role      string     `json:"role"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	Token     string     `json:"token,omitempty"`
}

func (s *Store) CreateServiceAccount(ctx context.Context, orgID, userID, name, role string) (ServiceAccount, error) {
	if !domain.CanRead(domain.Role(role)) {
		return ServiceAccount{}, apierr.Invalid("unknown role")
	}
	token, err := authn.NewToken()
	if err != nil {
		return ServiceAccount{}, err
	}
	var item ServiceAccount
	err = s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO service_accounts (organization_id, name, role, token_hash, created_by)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id::text, name, role, created_at
		`, orgID, name, role, authn.Hash(token), userID).Scan(&item.ID, &item.Name, &item.Role, &item.CreatedAt)
		return mapDB(err, "service account")
	})
	item.Token = token
	return item, err
}

func (s *Store) ListServiceAccounts(ctx context.Context, orgID string) ([]ServiceAccount, error) {
	var out []ServiceAccount
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id::text, name, role, revoked_at, created_at FROM service_accounts WHERE organization_id = $1 ORDER BY name`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item ServiceAccount
			if err := rows.Scan(&item.ID, &item.Name, &item.Role, &item.RevokedAt, &item.CreatedAt); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []ServiceAccount{}
	}
	return out, err
}

func (s *Store) RevokeServiceAccount(ctx context.Context, orgID, id string) error {
	return s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE service_accounts SET revoked_at = now() WHERE organization_id = $1 AND id = $2 AND revoked_at IS NULL`, orgID, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return apierr.NotFound("service account")
		}
		return nil
	})
}

func (s *Store) PrincipalFromServiceToken(ctx context.Context, token string) (Principal, error) {
	var principal Principal
	err := s.WithBypass(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var orgID, orgName, slug, role, name, creator string
		err := tx.QueryRow(ctx, `
			SELECT sa.organization_id::text, o.name, o.slug, sa.role, sa.name, sa.created_by::text
			FROM service_accounts sa JOIN organizations o ON o.id = sa.organization_id
			WHERE sa.token_hash = $1 AND sa.revoked_at IS NULL
		`, authn.Hash(token)).Scan(&orgID, &orgName, &slug, &role, &name, &creator)
		if err != nil {
			return apierr.Unauthorized()
		}
		principal = Principal{
			User:         User{ID: creator, Email: name + "@service.local", DisplayName: "service:" + name},
			Organization: Membership{OrganizationID: orgID, OrganizationName: orgName, OrganizationSlug: slug, Role: role},
			Memberships:  []Membership{{OrganizationID: orgID, OrganizationName: orgName, OrganizationSlug: slug, Role: role}},
		}
		return nil
	})
	return principal, err
}

type VLANRow struct {
	ID     string `json:"id"`
	SiteID string `json:"site_id"`
	VID    int    `json:"vid"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (s *Store) ListVLANs(ctx context.Context, orgID string) ([]VLANRow, error) {
	var out []VLANRow
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id::text, site_id::text, vid, name, status FROM vlans WHERE organization_id = $1 ORDER BY vid`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item VLANRow
			if err := rows.Scan(&item.ID, &item.SiteID, &item.VID, &item.Name, &item.Status); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []VLANRow{}
	}
	return out, err
}

type Endpoint struct {
	IP         string `json:"ip"`
	MAC        string `json:"mac"`
	Hostname   string `json:"hostname"`
	SiteID     string `json:"site_id"`
	Confidence string `json:"confidence"`
}

func (s *Store) ListEndpoints(ctx context.Context, orgID string) ([]Endpoint, error) {
	var out []Endpoint
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT DISTINCT ON (site_id, mac) ip, mac, hostname, site_id::text, confidence
			FROM discovery_observations
			WHERE organization_id = $1 AND mac <> ''
			ORDER BY site_id, mac, id DESC
		`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Endpoint
			if err := rows.Scan(&item.IP, &item.MAC, &item.Hostname, &item.SiteID, &item.Confidence); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []Endpoint{}
	}
	return out, err
}

type InterfaceRow struct {
	ID       string `json:"id"`
	DeviceID string `json:"device_id"`
	Device   string `json:"device"`
	Name     string `json:"name"`
	MAC      string `json:"mac"`
	Speed    int    `json:"speed_mbps"`
	OperUp   bool   `json:"oper_up"`
}

func (s *Store) ListInterfaceInventory(ctx context.Context, orgID, deviceID string) ([]InterfaceRow, error) {
	var out []InterfaceRow
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT i.id::text, i.device_id::text, d.name, i.name, i.mac, i.speed_mbps, i.oper_up
			FROM interfaces i JOIN devices d ON d.id = i.device_id
			WHERE i.organization_id = $1 AND ($2 = '' OR i.device_id::text = $2)
			ORDER BY d.name, i.name
		`, orgID, deviceID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item InterfaceRow
			if err := rows.Scan(&item.ID, &item.DeviceID, &item.Device, &item.Name, &item.MAC, &item.Speed, &item.OperUp); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []InterfaceRow{}
	}
	return out, err
}

func (s *Store) QueueDiagnostic(ctx context.Context, orgID, connectorID, kind, target string) (string, error) {
	if kind != "ping" && kind != "dns" {
		return "", apierr.Invalid("diagnostic must be ping or dns")
	}
	var id string
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var siteID string
		if err := tx.QueryRow(ctx, `SELECT site_id::text FROM connectors WHERE organization_id = $1 AND id = $2 AND status <> 'revoked'`, orgID, connectorID).Scan(&siteID); err != nil {
			return mapDB(err, "connector")
		}
		return tx.QueryRow(ctx, `
			INSERT INTO agent_tasks (organization_id, connector_id, kind, payload)
			VALUES ($1, $2, 'diagnostic', $3) RETURNING id::text
		`, orgID, connectorID, mustJSON(map[string]string{"kind": kind, "target": target, "site_id": siteID})).Scan(&id)
	})
	return id, err
}

type AvailabilityReport struct {
	Devices      int    `json:"devices"`
	Healthy      int    `json:"healthy"`
	SnapshotOnly bool   `json:"snapshot_only"`
	Rule         string `json:"rule"`
	History      string `json:"history"`
}

func (s *Store) AvailabilityReport(ctx context.Context, orgID string) (AvailabilityReport, error) {
	var report AvailabilityReport
	report.SnapshotOnly = true
	report.Rule = "A device counts as healthy only when its current health value is healthy or up. This is the current row, not an uptime percentage."
	report.History = "Availability history is not collected, so no percentage over time is calculated."
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*), COUNT(*) FILTER (WHERE health IN ('healthy', 'up'))
			FROM devices WHERE organization_id = $1 AND lifecycle = 'active'
		`, orgID).Scan(&report.Devices, &report.Healthy)
	})
	return report, err
}

func (s *Store) DeliverWebhooks(ctx context.Context, orgID, event, resourceID string) error {
	type hook struct {
		id, url string
	}
	var hooks []hook
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id::text, url FROM webhooks WHERE organization_id = $1 AND enabled AND $2 = ANY(events)`, orgID, event)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item hook
			if err := rows.Scan(&item.id, &item.url); err != nil {
				return err
			}
			hooks = append(hooks, item)
		}
		return rows.Err()
	})
	if err != nil || len(hooks) == 0 {
		return err
	}
	payload, _ := json.Marshal(map[string]string{"event": event, "resource_id": resourceID})
	client := &http.Client{Timeout: 3 * time.Second}
	for _, hook := range hooks {
		status := 0
		detail := ""
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, hook.url, bytes.NewReader(payload))
		if reqErr != nil {
			detail = "invalid url"
		} else {
			req.Header.Set("Content-Type", "application/json")
			res, doErr := client.Do(req)
			if doErr != nil {
				detail = "delivery failed"
			} else {
				status = res.StatusCode
				res.Body.Close()
			}
		}
		_ = s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `INSERT INTO webhook_deliveries (organization_id, webhook_id, event, status_code, error) VALUES ($1,$2,$3,$4,$5)`, orgID, hook.id, event, status, detail)
			return err
		})
	}
	return nil
}
