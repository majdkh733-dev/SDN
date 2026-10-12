package store

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/majdkh733-dev/SDN/internal/apierr"
)

const ReferenceDiagramName = "utec"

type DiagramVersion struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedBy string    `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	MXFile    string    `json:"mxfile,omitempty"`
}

type SymbolBinding struct {
	CellID    string    `json:"cell_id"`
	DeviceID  string    `json:"device_id,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type LinkBinding struct {
	CellID            string    `json:"cell_id"`
	SourceDeviceID    string    `json:"source_device_id,omitempty"`
	SourceInterfaceID string    `json:"source_interface_id,omitempty"`
	TargetDeviceID    string    `json:"target_device_id,omitempty"`
	TargetInterfaceID string    `json:"target_interface_id,omitempty"`
	ConnectionType    string    `json:"connection_type"`
	Evidence          string    `json:"evidence"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (s *Store) LatestDiagram(ctx context.Context, orgID, name string) (DiagramVersion, bool, error) {
	var item DiagramVersion
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT id::text, name, COALESCE(created_by::text, ''), created_at, mxfile
			FROM topology_diagrams
			WHERE organization_id = $1 AND name = $2
			ORDER BY created_at DESC
			LIMIT 1
		`, orgID, name).Scan(&item.ID, &item.Name, &item.CreatedBy, &item.CreatedAt, &item.MXFile)
	})
	if err == pgx.ErrNoRows {
		return DiagramVersion{}, false, nil
	}
	return item, err == nil, err
}

func (s *Store) ListDiagramVersions(ctx context.Context, orgID, name string) ([]DiagramVersion, error) {
	var items []DiagramVersion
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, name, COALESCE(created_by::text, ''), created_at
			FROM topology_diagrams
			WHERE organization_id = $1 AND name = $2
			ORDER BY created_at DESC
			LIMIT 20
		`, orgID, name)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item DiagramVersion
			if err := rows.Scan(&item.ID, &item.Name, &item.CreatedBy, &item.CreatedAt); err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

func (s *Store) DiagramVersion(ctx context.Context, orgID, id string) (DiagramVersion, error) {
	var item DiagramVersion
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT id::text, name, COALESCE(created_by::text, ''), created_at, mxfile
			FROM topology_diagrams WHERE organization_id = $1 AND id = $2
		`, orgID, id).Scan(&item.ID, &item.Name, &item.CreatedBy, &item.CreatedAt, &item.MXFile)
	})
	if err == pgx.ErrNoRows {
		return DiagramVersion{}, apierr.NotFound("diagram version not found")
	}
	return item, err
}

func (s *Store) SaveDiagram(ctx context.Context, orgID, actor, name, mxfile string) (DiagramVersion, error) {
	mxfile = strings.TrimSpace(mxfile)
	if len(mxfile) < 20 || len(mxfile) > 12_000_000 || (!strings.HasPrefix(mxfile, "<mxfile") && !strings.HasPrefix(mxfile, "<mxGraphModel")) {
		return DiagramVersion{}, apierr.Invalid("diagram must be the draw.io mxfile")
	}
	var item DiagramVersion
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO topology_diagrams (organization_id, name, mxfile, created_by)
			VALUES ($1, $2, $3, NULLIF($4, '')::uuid)
			RETURNING id::text, name, COALESCE(created_by::text, ''), created_at
		`, orgID, name, mxfile, actor).Scan(&item.ID, &item.Name, &item.CreatedBy, &item.CreatedAt); err != nil {
			return err
		}
		return insertAudit(ctx, tx, orgID, actor, "topology_diagram.save", "topology_diagram", item.ID, "", map[string]any{"name": name, "bytes": len(mxfile)})
	})
	return item, err
}

func (s *Store) ListBindings(ctx context.Context, orgID string) ([]SymbolBinding, error) {
	var items []SymbolBinding
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT cell_id, COALESCE(device_id::text, ''), updated_at
			FROM topology_symbol_bindings WHERE organization_id = $1 ORDER BY cell_id
		`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item SymbolBinding
			if err := rows.Scan(&item.CellID, &item.DeviceID, &item.UpdatedAt); err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

func (s *Store) SetBinding(ctx context.Context, orgID, actor, cellID, deviceID string) error {
	cellID = strings.TrimSpace(cellID)
	if cellID == "" || len(cellID) > 200 {
		return apierr.Invalid("cell id is required")
	}
	return s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if deviceID == "" {
			if _, err := tx.Exec(ctx, `DELETE FROM topology_symbol_bindings WHERE organization_id = $1 AND cell_id = $2`, orgID, cellID); err != nil {
				return err
			}
			return insertAudit(ctx, tx, orgID, actor, "topology_binding.clear", "topology_symbol", "", "", map[string]any{"cell_id": cellID})
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT true FROM devices WHERE organization_id = $1 AND id = $2`, orgID, deviceID).Scan(&exists); err != nil {
			if err == pgx.ErrNoRows {
				return apierr.Invalid("that device is not in this organization")
			}
			return err
		}
		var other string
		err := tx.QueryRow(ctx, `
			SELECT cell_id FROM topology_symbol_bindings
			WHERE organization_id = $1 AND device_id = $2 AND cell_id <> $3
		`, orgID, deviceID, cellID).Scan(&other)
		if err == nil {
			return apierr.Conflict("duplicate_binding", "that device is already linked to another symbol")
		}
		if err != pgx.ErrNoRows {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO topology_symbol_bindings (organization_id, cell_id, device_id, updated_by)
			VALUES ($1, $2, $3, NULLIF($4, '')::uuid)
			ON CONFLICT (organization_id, cell_id)
			DO UPDATE SET device_id = EXCLUDED.device_id, updated_by = EXCLUDED.updated_by, updated_at = now()
		`, orgID, cellID, deviceID, actor); err != nil {
			return err
		}
		return insertAudit(ctx, tx, orgID, actor, "topology_binding.set", "topology_symbol", "", "", map[string]any{"cell_id": cellID, "device_id": deviceID})
	})
}

func (s *Store) ListLinkBindings(ctx context.Context, orgID string) ([]LinkBinding, error) {
	var items []LinkBinding
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT cell_id, COALESCE(source_device_id::text, ''), COALESCE(source_interface_id::text, ''),
			       COALESCE(target_device_id::text, ''), COALESCE(target_interface_id::text, ''),
			       connection_type, evidence, updated_at
			FROM topology_link_bindings WHERE organization_id = $1 ORDER BY cell_id
		`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item LinkBinding
			if err := rows.Scan(&item.CellID, &item.SourceDeviceID, &item.SourceInterfaceID, &item.TargetDeviceID, &item.TargetInterfaceID, &item.ConnectionType, &item.Evidence, &item.UpdatedAt); err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

func (s *Store) SetLinkBinding(ctx context.Context, orgID, actor string, item LinkBinding) error {
	item.CellID = strings.TrimSpace(item.CellID)
	if item.CellID == "" || len(item.CellID) > 200 {
		return apierr.Invalid("cell id is required")
	}
	if item.ConnectionType == "" {
		item.ConnectionType = "documented"
	}
	if item.Evidence == "" {
		item.Evidence = "manual"
	}
	switch item.ConnectionType {
	case "documented", "physical", "logical", "wan":
	default:
		return apierr.Invalid("connection type must be documented, physical, logical, or wan")
	}
	switch item.Evidence {
	case "manual", "discovered", "documented":
	default:
		return apierr.Invalid("evidence must be manual, discovered, or documented")
	}
	if item.SourceInterfaceID != "" && item.TargetInterfaceID != "" && item.SourceInterfaceID == item.TargetInterfaceID {
		return apierr.Invalid("the two ends cannot be the same interface")
	}
	return s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if item.SourceDeviceID == "" && item.TargetDeviceID == "" {
			if _, err := tx.Exec(ctx, `DELETE FROM topology_link_bindings WHERE organization_id = $1 AND cell_id = $2`, orgID, item.CellID); err != nil {
				return err
			}
			return insertAudit(ctx, tx, orgID, actor, "topology_link.clear", "topology_symbol", "", "", map[string]any{"cell_id": item.CellID})
		}
		if err := interfaceOnDevice(ctx, tx, orgID, item.SourceDeviceID, item.SourceInterfaceID); err != nil {
			return err
		}
		if err := interfaceOnDevice(ctx, tx, orgID, item.TargetDeviceID, item.TargetInterfaceID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO topology_link_bindings (
				organization_id, cell_id, source_device_id, source_interface_id, target_device_id, target_interface_id,
				connection_type, evidence, updated_by)
			VALUES ($1, $2, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, NULLIF($5, '')::uuid, NULLIF($6, '')::uuid, $7, $8, NULLIF($9, '')::uuid)
			ON CONFLICT (organization_id, cell_id) DO UPDATE SET
				source_device_id = EXCLUDED.source_device_id,
				source_interface_id = EXCLUDED.source_interface_id,
				target_device_id = EXCLUDED.target_device_id,
				target_interface_id = EXCLUDED.target_interface_id,
				connection_type = EXCLUDED.connection_type,
				evidence = EXCLUDED.evidence,
				updated_by = EXCLUDED.updated_by,
				updated_at = now()
		`, orgID, item.CellID, item.SourceDeviceID, item.SourceInterfaceID, item.TargetDeviceID, item.TargetInterfaceID, item.ConnectionType, item.Evidence, actor); err != nil {
			return err
		}
		return insertAudit(ctx, tx, orgID, actor, "topology_link.set", "topology_symbol", "", "", map[string]any{
			"cell_id": item.CellID, "connection_type": item.ConnectionType, "evidence": item.Evidence,
		})
	})
}

func interfaceOnDevice(ctx context.Context, tx pgx.Tx, orgID, deviceID, interfaceID string) error {
	if deviceID == "" && interfaceID == "" {
		return nil
	}
	if deviceID == "" || interfaceID == "" {
		return apierr.Invalid("a mapped end needs both a device and an interface")
	}
	var ok bool
	err := tx.QueryRow(ctx, `
		SELECT true FROM interfaces WHERE organization_id = $1 AND device_id = $2 AND id = $3
	`, orgID, deviceID, interfaceID).Scan(&ok)
	if err == pgx.ErrNoRows {
		return apierr.Invalid("that interface is not on the selected device")
	}
	return err
}
