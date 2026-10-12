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
