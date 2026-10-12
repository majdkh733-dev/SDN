package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/majdkh733-dev/SDN/internal/apierr"
	"github.com/majdkh733-dev/SDN/internal/authn"
)

func (s *Store) UpdateConnector(ctx context.Context, orgID, id, name, description, deployment string) (Connector, error) {
	if deployment != "" && deployment != "docker" && deployment != "linux" && deployment != "compose" {
		return Connector{}, apierr.Invalid("deployment method must be docker, linux, or compose")
	}
	var conn Connector
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			UPDATE connectors
			SET name = CASE WHEN $3 = '' THEN name ELSE $3 END,
			    description = $4,
			    deployment_method = CASE WHEN $5 = '' THEN deployment_method ELSE $5 END
			WHERE organization_id = $1 AND id = $2 AND lifecycle = 'active'
			RETURNING `+connectorReturning, orgID, id, name, description, deployment)
		return mapDB(scanConnector(row, &conn), "connector")
	})
	return conn, err
}

func (s *Store) RevokeConnector(ctx context.Context, orgID, id string) error {
	return s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE connectors
			SET status = 'revoked', revoked_at = now(), health = 'disconnected', lifecycle = 'disabled'
			WHERE organization_id = $1 AND id = $2 AND lifecycle <> 'archived'
		`, orgID, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return apierr.NotFound("connector")
		}
		_, err = tx.Exec(ctx, `
			UPDATE connector_tokens SET revoked_at = now()
			WHERE organization_id = $1 AND connector_id = $2 AND revoked_at IS NULL
		`, orgID, id)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE enrollment_tokens SET used_at = COALESCE(used_at, now())
			WHERE organization_id = $1 AND connector_id = $2 AND used_at IS NULL
		`, orgID, id)
		return err
	})
}

func (s *Store) ReplaceConnector(ctx context.Context, orgID, id string) error {
	return s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE connectors
			SET status = 'not_enrolled', revoked_at = NULL, enrolled_at = NULL, health = 'unknown', lifecycle = 'active'
			WHERE organization_id = $1 AND id = $2 AND lifecycle <> 'archived'
		`, orgID, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return apierr.NotFound("connector")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE connector_tokens SET revoked_at = now()
			WHERE organization_id = $1 AND connector_id = $2 AND revoked_at IS NULL
		`, orgID, id); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE enrollment_tokens SET used_at = COALESCE(used_at, now())
			WHERE organization_id = $1 AND connector_id = $2 AND used_at IS NULL
		`, orgID, id)
		return err
	})
}

func (s *Store) RemoveConnector(ctx context.Context, orgID, id string) error {
	if err := s.RevokeConnector(ctx, orgID, id); err != nil {
		return err
	}
	return s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE connectors SET lifecycle = 'archived' WHERE organization_id = $1 AND id = $2
		`, orgID, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return apierr.NotFound("connector")
		}
		return nil
	})
}

type EnrollmentResult struct {
	Credential     string
	OrganizationID string
	ConnectorID    string
	SiteID         string
}

func (s *Store) ExchangeEnrollment(ctx context.Context, token, version string, host, capabilities []byte) (EnrollmentResult, error) {
	var result EnrollmentResult
	if len(host) == 0 {
		host = []byte("{}")
	}
	if len(capabilities) == 0 {
		capabilities = []byte("[]")
	}
	err := s.WithBypass(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var orgID, connectorID, siteID string
		var expires time.Time
		var used *time.Time
		err := tx.QueryRow(ctx, `
			SELECT t.organization_id::text, t.connector_id::text, c.site_id::text, t.expires_at, t.used_at
			FROM enrollment_tokens t
			JOIN connectors c ON c.id = t.connector_id
			WHERE t.token_hash = $1 AND c.lifecycle = 'active' AND c.revoked_at IS NULL
		`, authn.Hash(token)).Scan(&orgID, &connectorID, &siteID, &expires, &used)
		if err != nil {
			return mapDB(err, "enrollment token")
		}
		if used != nil {
			return apierr.Unauthorized()
		}
		if time.Now().After(expires) {
			return apierr.New(401, "expired", "The enrollment token has expired")
		}
		credential, err := authn.NewToken()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE enrollment_tokens SET used_at = now() WHERE token_hash = $1 AND used_at IS NULL`, authn.Hash(token)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO connector_tokens (organization_id, connector_id, token_hash) VALUES ($1, $2, $3)
		`, orgID, connectorID, authn.Hash(credential)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE connectors
			SET status = 'connecting', enrolled_at = now(), version = CASE WHEN $3 = '' THEN version ELSE $3 END,
			    host_info = $4::jsonb, capabilities = $5::jsonb, execution_mode = 'site_agent'
			WHERE id = $2 AND organization_id = $1
		`, orgID, connectorID, version, host, capabilities); err != nil {
			return err
		}
		result = EnrollmentResult{Credential: credential, OrganizationID: orgID, ConnectorID: connectorID, SiteID: siteID}
		return nil
	})
	return result, err
}
