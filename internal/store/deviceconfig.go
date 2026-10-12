package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/majdkh733-dev/SDN/internal/apierr"
	"github.com/majdkh733-dev/SDN/internal/domain"
)

func checksumText(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func renderDeviceConfig(ctx context.Context, tx pgx.Tx, orgID, deviceID string, overlay bool) (string, error) {
	var name, origin string
	if err := tx.QueryRow(ctx, `SELECT name, data_origin FROM devices WHERE organization_id = $1 AND id = $2`, orgID, deviceID).Scan(&name, &origin); err != nil {
		return "", mapDB(err, "device")
	}
	if origin != "simulator" {
		return "", apierr.Unsupported("configuration retrieval is implemented for the simulator lab adapter. This device has no config.read adapter.")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "hostname %s\n! origin simulator\n", name)
	rows, err := tx.Query(ctx, `SELECT name FROM interfaces WHERE organization_id = $1 AND device_id = $2 ORDER BY name`, orgID, deviceID)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var iface string
		if err := rows.Scan(&iface); err != nil {
			rows.Close()
			return "", err
		}
		fmt.Fprintf(&b, "interface %s\n", iface)
	}
	rows.Close()
	vrows, err := tx.Query(ctx, `SELECT vid, name, interface_name FROM simulator_vlans WHERE organization_id = $1 AND device_id = $2 ORDER BY vid`, orgID, deviceID)
	if err != nil {
		return "", err
	}
	for vrows.Next() {
		var vid int
		var vlanName, iface string
		if err := vrows.Scan(&vid, &vlanName, &iface); err != nil {
			vrows.Close()
			return "", err
		}
		fmt.Fprintf(&b, "vlan %d\n name %s\n", vid, vlanName)
		if iface != "" {
			fmt.Fprintf(&b, " interface %s\n", iface)
		}
	}
	vrows.Close()
	if overlay {
		var extra string
		_ = tx.QueryRow(ctx, `SELECT body FROM simulator_overlays WHERE organization_id = $1 AND device_id = $2`, orgID, deviceID).Scan(&extra)
		if strings.TrimSpace(extra) != "" {
			b.WriteString(extra)
			if !strings.HasSuffix(extra, "\n") {
				b.WriteByte('\n')
			}
		}
	}
	return b.String(), nil
}

func insertVLAN(ctx context.Context, tx pgx.Tx, orgID, deviceID, versionID string, spec domain.PolicySpec) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO simulator_vlans (organization_id, device_id, policy_version_id, vid, name, interface_name)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (device_id, policy_version_id) DO NOTHING
	`, orgID, deviceID, versionID, spec.VLAN.VID, spec.VLAN.Name, spec.VLAN.Interface)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO vlans (organization_id, site_id, vid, name, status, provenance)
		VALUES ($1, $2, $3, $4, 'observed', 'simulator')
		ON CONFLICT (organization_id, site_id, vid) DO UPDATE SET status = 'observed', name = EXCLUDED.name, updated_at = now()
	`, orgID, spec.SiteIDs[0], spec.VLAN.VID, spec.VLAN.Name)
	return err
}

func insertConfigRevision(ctx context.Context, tx pgx.Tx, orgID, deviceID, role, body, source, changeID string) error {
	var change any
	if changeID != "" {
		change = changeID
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO device_config_revisions (organization_id, device_id, role, body, checksum, source, change_request_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, orgID, deviceID, role, body, checksumText(body), source, change)
	return err
}

type ConfigRevision struct {
	ID        string `json:"id"`
	DeviceID  string `json:"device_id"`
	Role      string `json:"role"`
	Body      string `json:"body"`
	Checksum  string `json:"checksum"`
	Source    string `json:"source"`
	Collected string `json:"collected_at"`
}

func (s *Store) ListConfigRevisions(ctx context.Context, orgID, deviceID string) ([]ConfigRevision, error) {
	var out []ConfigRevision
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, device_id::text, role, body, checksum, source, collected_at::text
			FROM device_config_revisions
			WHERE organization_id = $1 AND ($2 = '' OR device_id::text = $2)
			ORDER BY collected_at DESC LIMIT 50
		`, orgID, deviceID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item ConfigRevision
			if err := rows.Scan(&item.ID, &item.DeviceID, &item.Role, &item.Body, &item.Checksum, &item.Source, &item.Collected); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []ConfigRevision{}
	}
	return out, err
}

func (s *Store) CollectConfig(ctx context.Context, orgID, deviceID string) (ConfigRevision, error) {
	var item ConfigRevision
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		body, err := renderDeviceConfig(ctx, tx, orgID, deviceID, true)
		if err != nil {
			return err
		}
		if err := insertConfigRevision(ctx, tx, orgID, deviceID, "observed", body, "simulator", ""); err != nil {
			return err
		}
		item = ConfigRevision{DeviceID: deviceID, Role: "observed", Body: body, Checksum: checksumText(body), Source: "simulator"}
		var desired string
		err = tx.QueryRow(ctx, `
			SELECT body FROM device_config_revisions
			WHERE organization_id = $1 AND device_id = $2 AND role = 'desired'
			ORDER BY collected_at DESC LIMIT 1
		`, orgID, deviceID).Scan(&desired)
		if err == pgx.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		if desired == body {
			_, err = tx.Exec(ctx, `UPDATE drift_findings SET disposition = 'remediated' WHERE organization_id = $1 AND device_id = $2 AND disposition = 'open' AND difference->>'kind' = 'config'`, orgID, deviceID)
			return err
		}
		diff := configDiff(desired, body)
		_, err = tx.Exec(ctx, `
			INSERT INTO drift_findings (organization_id, device_id, difference)
			SELECT $1, $2, $3::jsonb
			WHERE NOT EXISTS (
				SELECT 1 FROM drift_findings
				WHERE organization_id = $1 AND device_id = $2 AND disposition = 'open' AND difference->>'kind' = 'config'
			)
		`, orgID, deviceID, mustJSON(map[string]string{"kind": "config", "detail": diff}))
		return err
	})
	return item, err
}

func configDiff(desired, observed string) string {
	have := map[string]bool{}
	for _, line := range strings.Split(desired, "\n") {
		have[line] = true
	}
	var extra []string
	for _, line := range strings.Split(observed, "\n") {
		if strings.TrimSpace(line) == "" || have[line] {
			continue
		}
		extra = append(extra, "+ "+line)
	}
	if len(extra) == 0 {
		return "observed configuration differs from the saved baseline"
	}
	return strings.Join(extra, "\n")
}

func (s *Store) SetSimulatorOverlay(ctx context.Context, orgID, deviceID, body string) error {
	return s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var origin string
		if err := tx.QueryRow(ctx, `SELECT data_origin FROM devices WHERE organization_id = $1 AND id = $2`, orgID, deviceID).Scan(&origin); err != nil {
			return mapDB(err, "device")
		}
		if origin != "simulator" {
			return apierr.Unsupported("unmanaged configuration can be recorded only on the simulator lab adapter")
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO simulator_overlays (organization_id, device_id, body)
			VALUES ($1, $2, $3)
			ON CONFLICT (organization_id, device_id) DO UPDATE SET body = EXCLUDED.body, updated_at = now()
		`, orgID, deviceID, body)
		return err
	})
}
