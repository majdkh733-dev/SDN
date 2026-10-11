package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/majdkh733-dev/SDN/internal/apierr"
	"github.com/majdkh733-dev/SDN/internal/domain"
)

type Approval struct {
	ID         string    `json:"id"`
	PlanHash   string    `json:"plan_hash"`
	ReviewerID string    `json:"reviewer_id"`
	Decision   string    `json:"decision"`
	CreatedAt  time.Time `json:"created_at"`
}

type Step struct {
	ID             string          `json:"id"`
	OperationKey   string          `json:"operation_key"`
	DeviceID       string          `json:"device_id,omitempty"`
	Status         string          `json:"status"`
	Attempt        int             `json:"attempt"`
	AuthorityEpoch int64           `json:"authority_epoch"`
	Result         json.RawMessage `json:"result"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type Verification struct {
	ID         string          `json:"id"`
	Scope      string          `json:"scope"`
	Result     string          `json:"result"`
	Evidence   string          `json:"evidence"`
	Assertions json.RawMessage `json:"assertions"`
	CreatedAt  time.Time       `json:"created_at"`
}

type Change struct {
	ID              string         `json:"id"`
	PolicyVersionID string         `json:"policy_version_id"`
	PolicyName      string         `json:"policy_name"`
	PlanHash        string         `json:"plan_hash"`
	Status          string         `json:"status"`
	Impact          domain.Impact  `json:"impact"`
	RequestedBy     string         `json:"requested_by"`
	RequesterEmail  string         `json:"requester_email"`
	ExecutionEpoch  int64          `json:"execution_epoch"`
	CancelRequested bool           `json:"cancel_requested"`
	ErrorDetail     string         `json:"error_detail"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	Approvals       []Approval     `json:"approvals"`
	Steps           []Step         `json:"steps"`
	Verifications   []Verification `json:"verifications"`
}

func (s *Store) CreateChange(ctx context.Context, orgID, userID, versionID, planHash, idempotencyKey string) (Change, error) {
	var change Change
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if err := ensureLatestVersion(ctx, tx, versionID); err != nil {
			return err
		}
		impact, spec, err := impactTx(ctx, tx, orgID, versionID)
		if err != nil {
			return err
		}
		hash, err := PlanHash(spec, impact)
		if err != nil {
			return err
		}
		if hash != planHash {
			return apierr.StalePlan()
		}
		if impact.Blocked {
			return apierr.Unsupported(impact.BlockReason)
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO change_requests (
				organization_id, policy_version_id, idempotency_key, plan_hash, impact, status, requested_by
			) VALUES ($1, $2, NULLIF($3, ''), $4, $5, 'validated', $6)
			RETURNING id::text
		`, orgID, versionID, idempotencyKey, planHash, mustJSON(impact), userID).Scan(&change.ID)
		return mapDB(err, "change request")
	})
	if err != nil {
		return Change{}, err
	}
	return s.GetChange(ctx, orgID, change.ID)
}

func (s *Store) GetChange(ctx context.Context, orgID, id string) (Change, error) {
	var change Change
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		change, err = loadChange(ctx, tx, orgID, id)
		return err
	})
	return change, err
}

func (s *Store) ListChanges(ctx context.Context, orgID, status string) ([]Change, error) {
	var ids []string
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text FROM change_requests
			WHERE organization_id = $1 AND ($2 = '' OR status = $2)
			ORDER BY created_at DESC
		`, orgID, status)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	out := make([]Change, 0, len(ids))
	for _, id := range ids {
		item, err := s.GetChange(ctx, orgID, id)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func ensureLatestVersion(ctx context.Context, tx pgx.Tx, versionID string) error {
	var latest string
	err := tx.QueryRow(ctx, `
		SELECT id::text FROM policy_versions
		WHERE policy_id = (SELECT policy_id FROM policy_versions WHERE id = $1)
		ORDER BY revision DESC LIMIT 1
	`, versionID).Scan(&latest)
	if err != nil {
		return mapDB(err, "policy version")
	}
	if latest != versionID {
		return apierr.StalePlan()
	}
	return nil
}

func loadChange(ctx context.Context, tx pgx.Tx, orgID, id string) (Change, error) {
	var change Change
	var impactRaw []byte
	err := tx.QueryRow(ctx, `
		SELECT c.id::text, c.policy_version_id::text, p.name, c.plan_hash, c.status, c.impact,
		       c.requested_by::text, u.email, c.execution_epoch, c.cancel_requested, c.error_detail,
		       c.created_at, c.updated_at
		FROM change_requests c
		JOIN policy_versions v ON v.id = c.policy_version_id
		JOIN policies p ON p.id = v.policy_id
		JOIN users u ON u.id = c.requested_by
		WHERE c.organization_id = $1 AND c.id = $2
	`, orgID, id).Scan(
		&change.ID, &change.PolicyVersionID, &change.PolicyName, &change.PlanHash, &change.Status, &impactRaw,
		&change.RequestedBy, &change.RequesterEmail, &change.ExecutionEpoch, &change.CancelRequested, &change.ErrorDetail,
		&change.CreatedAt, &change.UpdatedAt,
	)
	if err != nil {
		return Change{}, mapDB(err, "change request")
	}
	if err := json.Unmarshal(impactRaw, &change.Impact); err != nil {
		return Change{}, err
	}
	arows, err := tx.Query(ctx, `
		SELECT id::text, plan_hash, reviewer_id::text, decision, created_at
		FROM approvals WHERE organization_id = $1 AND change_request_id = $2 ORDER BY created_at
	`, orgID, id)
	if err != nil {
		return Change{}, err
	}
	defer arows.Close()
	for arows.Next() {
		var item Approval
		if err := arows.Scan(&item.ID, &item.PlanHash, &item.ReviewerID, &item.Decision, &item.CreatedAt); err != nil {
			return Change{}, err
		}
		change.Approvals = append(change.Approvals, item)
	}
	if change.Approvals == nil {
		change.Approvals = []Approval{}
	}
	if err := arows.Err(); err != nil {
		return Change{}, err
	}
	srows, err := tx.Query(ctx, `
		SELECT id::text, operation_key, COALESCE(device_id::text, ''), status, attempt, authority_epoch, result, updated_at
		FROM execution_steps WHERE organization_id = $1 AND change_request_id = $2 ORDER BY created_at, operation_key
	`, orgID, id)
	if err != nil {
		return Change{}, err
	}
	defer srows.Close()
	for srows.Next() {
		var item Step
		if err := srows.Scan(&item.ID, &item.OperationKey, &item.DeviceID, &item.Status, &item.Attempt, &item.AuthorityEpoch, &item.Result, &item.UpdatedAt); err != nil {
			return Change{}, err
		}
		change.Steps = append(change.Steps, item)
	}
	if change.Steps == nil {
		change.Steps = []Step{}
	}
	vrows, err := tx.Query(ctx, `
		SELECT id::text, scope, result, evidence, assertions, created_at
		FROM verification_results
		WHERE organization_id = $1 AND change_request_id = $2
		ORDER BY created_at
	`, orgID, id)
	if err != nil {
		return Change{}, err
	}
	defer vrows.Close()
	for vrows.Next() {
		var item Verification
		if err := vrows.Scan(&item.ID, &item.Scope, &item.Result, &item.Evidence, &item.Assertions, &item.CreatedAt); err != nil {
			return Change{}, err
		}
		change.Verifications = append(change.Verifications, item)
	}
	if change.Verifications == nil {
		change.Verifications = []Verification{}
	}
	return change, vrows.Err()
}

func (s *Store) DecideChange(ctx context.Context, orgID, changeID, reviewerID, decision string) (Change, error) {
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var status, hash, requester string
		err := tx.QueryRow(ctx, `
			SELECT status, plan_hash, requested_by::text
			FROM change_requests WHERE organization_id = $1 AND id = $2 FOR UPDATE
		`, orgID, changeID).Scan(&status, &hash, &requester)
		if err != nil {
			return mapDB(err, "change request")
		}
		if reviewerID == requester {
			return apierr.Forbidden("The requester cannot approve their own change")
		}
		if status != "validated" {
			return apierr.Conflict("conflict", "Only a validated change can be reviewed")
		}
		var versionID string
		if err := tx.QueryRow(ctx, `SELECT policy_version_id::text FROM change_requests WHERE id = $1`, changeID).Scan(&versionID); err != nil {
			return err
		}
		if err := ensureLatestVersion(ctx, tx, versionID); err != nil {
			return err
		}
		impact, spec, err := impactTx(ctx, tx, orgID, versionID)
		if err != nil {
			return err
		}
		current, err := PlanHash(spec, impact)
		if err != nil {
			return err
		}
		if current != hash {
			return apierr.StalePlan()
		}
		next := "approved"
		if decision == "rejected" {
			next = "rejected"
		} else if decision != "approved" {
			return apierr.Invalid("decision must be approved or rejected")
		}
		if !domain.CanTransition(status, next) {
			return apierr.Conflict("conflict", "change cannot move to "+next)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO approvals (organization_id, change_request_id, plan_hash, reviewer_id, decision)
			VALUES ($1, $2, $3, $4, $5)
		`, orgID, changeID, hash, reviewerID, decision); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE change_requests SET status = $3, updated_at = now() WHERE organization_id = $1 AND id = $2
		`, orgID, changeID, next)
		return err
	})
	if err != nil {
		return Change{}, err
	}
	return s.GetChange(ctx, orgID, changeID)
}

func (s *Store) RequestCancel(ctx context.Context, orgID, changeID string) (Change, error) {
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var status string
		err := tx.QueryRow(ctx, `SELECT status FROM change_requests WHERE organization_id = $1 AND id = $2 FOR UPDATE`, orgID, changeID).Scan(&status)
		if err != nil {
			return mapDB(err, "change request")
		}
		switch status {
		case "validated", "approved", "queued":
			if !domain.CanTransition(status, "cancelled") && status != "validated" {
				return apierr.Conflict("conflict", "change cannot be cancelled")
			}
			_, err = tx.Exec(ctx, `UPDATE change_requests SET status = 'cancelled', updated_at = now() WHERE id = $1`, changeID)
			return err
		case "applying":
			_, err = tx.Exec(ctx, `UPDATE change_requests SET cancel_requested = true, updated_at = now() WHERE id = $1`, changeID)
			return err
		default:
			return apierr.Conflict("conflict", "change can no longer be cancelled")
		}
	})
	if err != nil {
		return Change{}, err
	}
	return s.GetChange(ctx, orgID, changeID)
}

type Idempotency struct {
	RequestHash string
	StatusCode  int
	Response    json.RawMessage
	Found       bool
}

func (s *Store) GetIdempotency(ctx context.Context, orgID, key string) (Idempotency, error) {
	var item Idempotency
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT request_hash, status_code, response FROM idempotency_keys
			WHERE organization_id = $1 AND key = $2
		`, orgID, key).Scan(&item.RequestHash, &item.StatusCode, &item.Response)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		item.Found = true
		return nil
	})
	return item, err
}

func (s *Store) PutIdempotency(ctx context.Context, orgID, key, requestHash string, status int, response any) error {
	return s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO idempotency_keys (organization_id, key, request_hash, status_code, response)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (organization_id, key) DO NOTHING
		`, orgID, key, requestHash, status, mustJSON(response))
		return err
	})
}
