package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/majdkh733-dev/SDN/internal/apierr"
)

func (s *Store) Audit(ctx context.Context, orgID, actor, action, resourceType, resourceID, requestID string, meta any) error {
	if orgID == "" {
		return nil
	}
	return s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return insertAudit(ctx, tx, orgID, actor, action, resourceType, resourceID, requestID, meta)
	})
}

func insertAudit(ctx context.Context, tx pgx.Tx, orgID, actor, action, resourceType, resourceID, requestID string, meta any) error {
	if meta == nil {
		meta = map[string]any{}
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO audit_events (organization_id, actor_user_id, action, resource_type, resource_id, request_id, metadata)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, NULLIF($5, '')::uuid, $6, $7)
	`, orgID, actor, action, resourceType, resourceID, requestID, mustJSON(meta))
	return err
}

type AuditEvent struct {
	ID           string          `json:"id"`
	ActorUserID  string          `json:"actor_user_id,omitempty"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id,omitempty"`
	RequestID    string          `json:"request_id"`
	Metadata     json.RawMessage `json:"metadata"`
	CreatedAt    time.Time       `json:"created_at"`
}

func (s *Store) ListAudit(ctx context.Context, orgID, action string) ([]AuditEvent, error) {
	var out []AuditEvent
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, COALESCE(actor_user_id::text, ''), action, resource_type,
			       COALESCE(resource_id::text, ''), request_id, metadata, created_at
			FROM audit_events
			WHERE organization_id = $1 AND ($2 = '' OR action = $2)
			ORDER BY created_at DESC
			LIMIT 200
		`, orgID, action)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item AuditEvent
			if err := rows.Scan(&item.ID, &item.ActorUserID, &item.Action, &item.ResourceType, &item.ResourceID, &item.RequestID, &item.Metadata, &item.CreatedAt); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []AuditEvent{}
	}
	return out, err
}

type Summary struct {
	Organization          string `json:"organization"`
	Sites                 int    `json:"sites"`
	Devices               int    `json:"devices"`
	OpenDrift             int    `json:"open_drift"`
	ActiveChanges         int    `json:"active_changes"`
	ConnectorsHealthy     int    `json:"connectors_healthy"`
	ConnectorsTotal       int    `json:"connectors_total"`
	Notice                string `json:"notice"`
	WorkloadsAvailable    bool   `json:"workloads_available"`
	WorkloadsNote         string `json:"workloads_note"`
	AvailabilityAvailable bool   `json:"availability_available"`
	AvailabilityNote      string `json:"availability_note"`
	Alerts                int    `json:"alerts"`
}

func (s *Store) Summary(ctx context.Context, orgID, orgName string) (Summary, error) {
	sum := Summary{
		Organization: orgName, Notice: "Demonstration inventory is produced by the simulator unless a site agent reports an observation. Connector status is separate from device status.",
		WorkloadsNote:    "Workload collection is not implemented. No count is shown.",
		AvailabilityNote: "Availability history is not collected. No percentage is shown.",
	}
	var offline int
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT
				(SELECT COUNT(*) FROM sites WHERE organization_id = $1 AND lifecycle = 'active'),
				(SELECT COUNT(*) FROM devices WHERE organization_id = $1 AND lifecycle = 'active'),
				(SELECT COUNT(*) FROM drift_findings WHERE organization_id = $1 AND disposition = 'open'),
				(SELECT COUNT(*) FROM change_requests WHERE organization_id = $1 AND status IN ('validated','approved','queued','applying','verifying','partial','compensating','unknown')),
				(SELECT COUNT(*) FROM connectors WHERE organization_id = $1 AND health = 'healthy' AND lifecycle = 'active'),
				(SELECT COUNT(*) FROM connectors WHERE organization_id = $1 AND lifecycle <> 'archived'),
				(SELECT COUNT(*) FROM connectors WHERE organization_id = $1 AND lifecycle = 'active' AND revoked_at IS NULL AND last_heartbeat_at IS NOT NULL AND last_heartbeat_at < now() - interval '90 seconds')
		`, orgID).Scan(&sum.Sites, &sum.Devices, &sum.OpenDrift, &sum.ActiveChanges, &sum.ConnectorsHealthy, &sum.ConnectorsTotal, &offline)
	})
	sum.Alerts = sum.OpenDrift + offline
	return sum, err
}

type TopoNode struct {
	ID     string         `json:"id"`
	Label  string         `json:"label"`
	Kind   string         `json:"kind"`
	Health string         `json:"health"`
	SiteID string         `json:"site_id,omitempty"`
	Meta   map[string]any `json:"meta"`
}

type TopoEdge struct {
	ID          string   `json:"id"`
	Source      string   `json:"source"`
	Target      string   `json:"target"`
	Utilization float64  `json:"utilization_pct"`
	Label       string   `json:"label"`
	Origin      string   `json:"origin"`
	Measured    bool     `json:"measured"`
	LatencyMS   *float64 `json:"latency_ms,omitempty"`
}

type Topology struct {
	Lens   string     `json:"lens"`
	Notice string     `json:"notice"`
	Nodes  []TopoNode `json:"nodes"`
	Edges  []TopoEdge `json:"edges"`
}

func (s *Store) Topology(ctx context.Context, orgID, lens, siteID string) (Topology, error) {
	if lens == "" {
		lens = "performance"
	}
	topo := Topology{Lens: lens, Nodes: []TopoNode{}, Edges: []TopoEdge{}}
	switch lens {
	case "application":
		topo.Notice = "Application lens shows tenant segments and policies. It is not a packet path."
	case "security":
		topo.Notice = "Security lens highlights devices in the latest policy impact. Denies are calculated, not observed drops."
	default:
		lens = "performance"
		topo.Lens = lens
		topo.Notice = "Links come from simulator blueprints, a supported discovery source, or a manual site link. Utilization and latency are omitted unless measured."
	}
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if lens == "application" {
			rows, err := tx.Query(ctx, `
				SELECT s.id::text, s.name, t.name, s.cidr
				FROM segments s JOIN tenants t ON t.id = s.tenant_id
				WHERE s.organization_id = $1 AND s.lifecycle = 'active'
				ORDER BY s.name
			`, orgID)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var id, name, tenant, cidr string
				if err := rows.Scan(&id, &name, &tenant, &cidr); err != nil {
					return err
				}
				topo.Nodes = append(topo.Nodes, TopoNode{ID: id, Label: name, Kind: "segment", Health: "active", Meta: map[string]any{"tenant": tenant, "cidr": cidr}})
			}
			return rows.Err()
		}
		rows, err := tx.Query(ctx, `
			SELECT d.id::text, d.name, d.role, d.health, d.site_id::text, d.data_origin,
			       d.observed_at, COALESCE(c.last_heartbeat_at, 'epoch'::timestamptz), c.id IS NOT NULL
			FROM devices d
			LEFT JOIN connectors c ON c.id = d.connector_id
			WHERE d.organization_id = $1 AND d.lifecycle = 'active' AND ($2 = '' OR d.site_id::text = $2)
			ORDER BY d.name
		`, orgID, siteID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id, name, role, health, site, origin string
			var observed, beat *time.Time
			var hasConnector bool
			if err := rows.Scan(&id, &name, &role, &health, &site, &origin, &observed, &beat, &hasConnector); err != nil {
				return err
			}
			freshness := "unknown"
			if hasConnector && beat != nil && beat.Year() > 1970 {
				if time.Since(*beat) <= 90*time.Second {
					freshness = "live"
				} else {
					freshness = "stale"
				}
			}
			topo.Nodes = append(topo.Nodes, TopoNode{ID: id, Label: name, Kind: role, Health: health, SiteID: site, Meta: map[string]any{
				"data_origin": origin, "telemetry": freshness, "observed_at": observed,
			}})
		}
		if err := rows.Err(); err != nil {
			return err
		}
		erows, err := tx.Query(ctx, `
			SELECT l.id::text, da.id::text, db.id::text, l.utilization_pct::float8, ia.name || '—' || ib.name, l.source
			FROM topology_links l
			JOIN interfaces ia ON ia.id = l.a_interface_id
			JOIN interfaces ib ON ib.id = l.b_interface_id
			JOIN devices da ON da.id = ia.device_id
			JOIN devices db ON db.id = ib.device_id
			WHERE l.organization_id = $1 AND ($2 = '' OR l.site_id::text = $2)
		`, orgID, siteID)
		if err != nil {
			return err
		}
		defer erows.Close()
		for erows.Next() {
			var edge TopoEdge
			if err := erows.Scan(&edge.ID, &edge.Source, &edge.Target, &edge.Utilization, &edge.Label, &edge.Origin); err != nil {
				return err
			}
			if edge.Origin != "measured" {
				edge.Utilization = 0
				edge.Measured = false
			} else {
				edge.Measured = true
			}
			topo.Edges = append(topo.Edges, edge)
		}
		if err := erows.Err(); err != nil {
			return err
		}
		if siteID == "" {
			return appendSiteNodes(ctx, tx, orgID, &topo)
		}
		return nil
	})
	return topo, err
}

type Backup struct {
	ID        string          `json:"id"`
	Scope     string          `json:"scope"`
	Checksum  string          `json:"checksum"`
	Integrity string          `json:"integrity"`
	Manifest  json.RawMessage `json:"manifest"`
	CreatedAt time.Time       `json:"created_at"`
}

func (s *Store) CreateBackup(ctx context.Context, orgID, userID, scope string) (Backup, error) {
	var backup Backup
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if scope == "" {
			scope = "organization"
		}
		var policies, devices int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM policies WHERE organization_id = $1`, orgID).Scan(&policies); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM devices WHERE organization_id = $1`, orgID).Scan(&devices); err != nil {
			return err
		}
		manifest := map[string]any{
			"scope": scope, "policies": policies, "devices": devices,
			"note": "Manifest only. Secret values are not stored in backups.",
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO configuration_backups (organization_id, scope, manifest, checksum, created_by)
			VALUES ($1, $2, $3, 'pending', $4)
			RETURNING id::text, manifest
		`, orgID, scope, mustJSON(manifest), userID).Scan(&backup.ID, &backup.Manifest); err != nil {
			return err
		}
		sum := sha256.Sum256(backup.Manifest)
		backup.Checksum = hex.EncodeToString(sum[:])
		return tx.QueryRow(ctx, `
			UPDATE configuration_backups SET checksum = $2 WHERE id = $1
			RETURNING scope, checksum, integrity, manifest, created_at
		`, backup.ID, backup.Checksum).Scan(&backup.Scope, &backup.Checksum, &backup.Integrity, &backup.Manifest, &backup.CreatedAt)
	})
	return backup, err
}

func (s *Store) ListBackups(ctx context.Context, orgID string) ([]Backup, error) {
	var out []Backup
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, scope, checksum, integrity, manifest, created_at
			FROM configuration_backups WHERE organization_id = $1 ORDER BY created_at DESC
		`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Backup
			if err := rows.Scan(&item.ID, &item.Scope, &item.Checksum, &item.Integrity, &item.Manifest, &item.CreatedAt); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []Backup{}
	}
	return out, err
}

type Restore struct {
	ID        string    `json:"id"`
	BackupID  string    `json:"backup_id"`
	Status    string    `json:"status"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) RestoreBackup(ctx context.Context, orgID, userID, backupID string) (Restore, error) {
	var restore Restore
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var manifest []byte
		var checksum, integrity string
		err := tx.QueryRow(ctx, `
			SELECT manifest, checksum, integrity FROM configuration_backups
			WHERE organization_id = $1 AND id = $2
		`, orgID, backupID).Scan(&manifest, &checksum, &integrity)
		if err != nil {
			return mapDB(err, "backup")
		}
		sum := sha256.Sum256(manifest)
		if hex.EncodeToString(sum[:]) != checksum || integrity != "valid" {
			_, _ = tx.Exec(ctx, `UPDATE configuration_backups SET integrity = 'mismatch' WHERE id = $1`, backupID)
			return apierr.Conflict("backup_integrity", "Backup checksum does not match its manifest. Restore was rejected.")
		}
		restore.BackupID = backupID
		restore.Status = "pending_reconciliation"
		restore.Detail = "Platform records were accepted. Device writes stay disabled until a separate approved change reconciles observed state."
		return tx.QueryRow(ctx, `
			INSERT INTO restore_runs (organization_id, backup_id, requested_by, status, detail)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id::text, created_at
		`, orgID, backupID, userID, restore.Status, restore.Detail).Scan(&restore.ID, &restore.CreatedAt)
	})
	return restore, err
}

func (s *Store) ListRestores(ctx context.Context, orgID string) ([]Restore, error) {
	var out []Restore
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, backup_id::text, status, detail, created_at
			FROM restore_runs WHERE organization_id = $1 ORDER BY created_at DESC
		`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Restore
			if err := rows.Scan(&item.ID, &item.BackupID, &item.Status, &item.Detail, &item.CreatedAt); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []Restore{}
	}
	return out, err
}

type Drift struct {
	ID          string          `json:"id"`
	DeviceID    string          `json:"device_id"`
	DeviceName  string          `json:"device_name"`
	Difference  json.RawMessage `json:"difference"`
	Disposition string          `json:"disposition"`
	CreatedAt   time.Time       `json:"created_at"`
}

func (s *Store) ListDrift(ctx context.Context, orgID string) ([]Drift, error) {
	var out []Drift
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT f.id::text, f.device_id::text, d.name, f.difference, f.disposition, f.created_at
			FROM drift_findings f JOIN devices d ON d.id = f.device_id
			WHERE f.organization_id = $1 ORDER BY f.created_at DESC
		`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Drift
			if err := rows.Scan(&item.ID, &item.DeviceID, &item.DeviceName, &item.Difference, &item.Disposition, &item.CreatedAt); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []Drift{}
	}
	return out, err
}

func (s *Store) ScanDrift(ctx context.Context, orgID string) ([]Drift, error) {
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT d.id::text, d.name,
			       COALESCE((
			           SELECT document FROM observed_states o
			           WHERE o.device_id = d.id ORDER BY observed_at DESC LIMIT 1
			       ), '{"acls":[]}'::jsonb)
			FROM devices d WHERE d.organization_id = $1 AND d.lifecycle = 'active'
		`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		type obs struct {
			id, name string
			doc      []byte
		}
		var items []obs
		for rows.Next() {
			var item obs
			if err := rows.Scan(&item.id, &item.name, &item.doc); err != nil {
				return err
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		var desired []byte
		err = tx.QueryRow(ctx, `
			SELECT document FROM desired_states
			WHERE organization_id = $1 AND scope = 'fabric'
			ORDER BY revision DESC LIMIT 1
		`, orgID).Scan(&desired)
		if err != nil {
			desired = []byte(`{"devices":[]}`)
		}
		var desiredDoc struct {
			Devices []string        `json:"devices"`
			Spec    json.RawMessage `json:"spec"`
		}
		_ = json.Unmarshal(desired, &desiredDoc)
		wanted := map[string]bool{}
		for _, id := range desiredDoc.Devices {
			wanted[id] = true
		}
		for _, item := range items {
			var doc struct {
				ACLs []struct {
					Spec json.RawMessage `json:"spec"`
				} `json:"acls"`
			}
			_ = json.Unmarshal(item.doc, &doc)
			match := true
			if wanted[item.id] && len(doc.ACLs) == 0 {
				match = false
			}
			diff := map[string]any{"device_id": item.id, "desired": wanted[item.id], "observed_acls": len(doc.ACLs), "data_origin": "simulator"}
			if !match {
				var exists bool
				_ = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM drift_findings WHERE organization_id = $1 AND device_id = $2 AND disposition = 'open')`, orgID, item.id).Scan(&exists)
				if !exists {
					if _, err := tx.Exec(ctx, `
						INSERT INTO drift_findings (organization_id, device_id, difference)
						VALUES ($1, $2, $3)
					`, orgID, item.id, mustJSON(diff)); err != nil {
						return err
					}
				}
			} else if wanted[item.id] {
				_, _ = tx.Exec(ctx, `UPDATE drift_findings SET disposition = 'remediated' WHERE organization_id = $1 AND device_id = $2 AND disposition = 'open'`, orgID, item.id)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.ListDrift(ctx, orgID)
}

type Integration struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

func (s *Store) ListIntegrations(ctx context.Context, orgID string) ([]Integration, error) {
	var out []Integration
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT kind, name, status, detail FROM integrations WHERE organization_id = $1 ORDER BY name`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Integration
			if err := rows.Scan(&item.Kind, &item.Name, &item.Status, &item.Detail); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []Integration{}
	}
	return out, err
}

func (s *Store) SeedIntegration(ctx context.Context, orgID string, item Integration) error {
	return s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO integrations (organization_id, kind, name, status, detail)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (organization_id, kind) DO UPDATE SET status = EXCLUDED.status, detail = EXCLUDED.detail
		`, orgID, item.Kind, item.Name, item.Status, item.Detail)
		return err
	})
}

type ControllerStatus struct {
	Coordination   string           `json:"coordination"`
	Etcd           string           `json:"etcd"`
	WriteAuthority string           `json:"write_authority"`
	Orchestration  string           `json:"orchestration"`
	Temporal       string           `json:"temporal"`
	SecretBackend  string           `json:"secret_backend"`
	Epochs         []map[string]any `json:"epochs"`
	Workers        []map[string]any `json:"workers"`
	Notice         string           `json:"notice"`
}

func (s *Store) Controllers(ctx context.Context, orgID string) (ControllerStatus, error) {
	status := ControllerStatus{
		Coordination:   "postgresql_fencing",
		Etcd:           "not_deployed",
		WriteAuthority: "epoch_per_execution_domain",
		Epochs:         []map[string]any{},
		Workers:        []map[string]any{},
		Notice:         "etcd is not part of this release. Fencing uses a PostgreSQL epoch so a stale writer can be rejected. A database lease does not by itself fence an external device session.",
	}
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT execution_domain, epoch, holder, updated_at FROM authority_epochs WHERE organization_id = $1 ORDER BY execution_domain`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var domain, holder string
			var epoch int64
			var updated time.Time
			if err := rows.Scan(&domain, &epoch, &holder, &updated); err != nil {
				return err
			}
			status.Epochs = append(status.Epochs, map[string]any{"execution_domain": domain, "epoch": epoch, "holder": holder, "updated_at": updated})
		}
		return rows.Err()
	})
	if err != nil {
		return status, err
	}
	_ = s.WithBypass(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, kind, beat_at FROM worker_heartbeats ORDER BY id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id, kind string
			var beat time.Time
			if err := rows.Scan(&id, &kind, &beat); err != nil {
				return err
			}
			status.Workers = append(status.Workers, map[string]any{"id": id, "kind": kind, "beat_at": beat, "healthy": time.Since(beat) < 30*time.Second})
		}
		return rows.Err()
	})
	return status, nil
}

func (s *Store) Beat(ctx context.Context, id, kind string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO worker_heartbeats (id, kind, beat_at) VALUES ($1, $2, now())
		ON CONFLICT (id) DO UPDATE SET beat_at = now(), kind = EXCLUDED.kind
	`, id, kind)
	return err
}

type RetentionResult struct {
	AuditRemoved    int `json:"audit_events_removed"`
	ObservedRemoved int `json:"observed_states_removed"`
	BackupsRemoved  int `json:"backups_removed"`
}

func (s *Store) ApplyRetention(ctx context.Context, orgID string) (RetentionResult, error) {
	var result RetentionResult
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var days int
		if err := tx.QueryRow(ctx, `SELECT retention_days FROM organizations WHERE id = $1`, orgID).Scan(&days); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			DELETE FROM audit_events
			WHERE organization_id = $1 AND created_at < now() - ($2::text || ' days')::interval
		`, orgID, days)
		if err != nil {
			return err
		}
		result.AuditRemoved = int(tag.RowsAffected())
		tag, err = tx.Exec(ctx, `
			DELETE FROM observed_states
			WHERE organization_id = $1 AND observed_at < now() - ($2::text || ' days')::interval
		`, orgID, days)
		if err != nil {
			return err
		}
		result.ObservedRemoved = int(tag.RowsAffected())
		tag, err = tx.Exec(ctx, `
			DELETE FROM configuration_backups
			WHERE organization_id = $1
			  AND created_at < now() - ($2::text || ' days')::interval
			  AND id NOT IN (
			    SELECT id FROM configuration_backups
			    WHERE organization_id = $1
			    ORDER BY created_at DESC
			    LIMIT 5
			  )
			  AND id NOT IN (SELECT backup_id FROM restore_runs WHERE organization_id = $1)
		`, orgID, days)
		if err != nil {
			return err
		}
		result.BackupsRemoved = int(tag.RowsAffected())
		return nil
	})
	return result, err
}

func (s *Store) EnqueueTask(ctx context.Context, orgID, connectorID, kind string, payload any) (string, error) {
	var id string
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO agent_tasks (organization_id, connector_id, kind, payload)
			VALUES ($1, $2, $3, $4) RETURNING id::text
		`, orgID, connectorID, kind, mustJSON(payload)).Scan(&id)
	})
	return id, err
}

func (s *Store) LeaseTask(ctx context.Context, orgID, connectorID string) (string, string, json.RawMessage, bool, error) {
	var id, kind string
	var payload json.RawMessage
	found := false
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		jobID, jobPayload, jobFound, err := s.leaseDiscovery(ctx, tx, orgID, connectorID)
		if err != nil {
			return err
		}
		if jobFound {
			id, kind, payload, found = jobID, "discovery", jobPayload, true
			return nil
		}
		err = tx.QueryRow(ctx, `
			UPDATE agent_tasks SET status = 'leased', updated_at = now()
			WHERE id = (
				SELECT id FROM agent_tasks
				WHERE organization_id = $1 AND connector_id = $2 AND status = 'pending'
				ORDER BY created_at LIMIT 1
				FOR UPDATE SKIP LOCKED
			)
			RETURNING id::text, kind, payload
		`, orgID, connectorID).Scan(&id, &kind, &payload)
		if err != nil {
			return nil
		}
		found = true
		return nil
	})
	return id, kind, payload, found, err
}

func (s *Store) GetTask(ctx context.Context, orgID, taskID string) (string, string, json.RawMessage, error) {
	var kind, connectorID string
	var payload json.RawMessage
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return mapDB(tx.QueryRow(ctx, `
			SELECT kind, connector_id::text, payload FROM agent_tasks WHERE organization_id = $1 AND id = $2
		`, orgID, taskID).Scan(&kind, &connectorID, &payload), "agent task")
	})
	return kind, connectorID, payload, err
}

func (s *Store) CompleteTask(ctx context.Context, orgID, taskID string, result any, ok bool) error {
	status := "succeeded"
	if !ok {
		status = "failed"
	}
	return s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE agent_tasks SET status = $3, result = $4, updated_at = now()
			WHERE organization_id = $1 AND id = $2
		`, orgID, taskID, status, mustJSON(result))
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return apierr.NotFound("agent task")
		}
		return nil
	})
}

type DesiredObserved struct {
	Desired  json.RawMessage `json:"desired"`
	Revision int             `json:"revision"`
	Scope    string          `json:"scope"`
}

func (s *Store) LatestDesired(ctx context.Context, orgID string) (DesiredObserved, error) {
	var item DesiredObserved
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT scope, revision, document FROM desired_states
			WHERE organization_id = $1 ORDER BY revision DESC LIMIT 1
		`, orgID).Scan(&item.Scope, &item.Revision, &item.Desired)
		if err != nil {
			item = DesiredObserved{Scope: "fabric", Revision: 0, Desired: json.RawMessage(`{}`)}
			return nil
		}
		return nil
	})
	return item, err
}

func (s *Store) ListObserved(ctx context.Context, orgID string) ([]map[string]any, error) {
	var out []map[string]any
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT DISTINCT ON (o.device_id) o.device_id::text, d.name, o.document, o.freshness, o.collector, o.observed_at
			FROM observed_states o JOIN devices d ON d.id = o.device_id
			WHERE o.organization_id = $1
			ORDER BY o.device_id, o.observed_at DESC
		`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id, name, freshness, collector string
			var doc []byte
			var at time.Time
			if err := rows.Scan(&id, &name, &doc, &freshness, &collector, &at); err != nil {
				return err
			}
			out = append(out, map[string]any{"device_id": id, "device_name": name, "document": json.RawMessage(doc), "freshness": freshness, "collector": collector, "observed_at": at})
		}
		return rows.Err()
	})
	if out == nil {
		out = []map[string]any{}
	}
	return out, err
}

func (s *Store) ListVerifications(ctx context.Context, orgID string) ([]Verification, error) {
	var out []Verification
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, scope, result, evidence, assertions, created_at
			FROM verification_results WHERE organization_id = $1 ORDER BY created_at DESC
		`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Verification
			if err := rows.Scan(&item.ID, &item.Scope, &item.Result, &item.Evidence, &item.Assertions, &item.CreatedAt); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []Verification{}
	}
	return out, err
}
