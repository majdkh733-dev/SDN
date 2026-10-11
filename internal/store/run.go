package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/majdkh733-dev/SDN/internal/apierr"
	"github.com/majdkh733-dev/SDN/internal/domain"
)

func (s *Store) RunChange(ctx context.Context, orgID, changeID string) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtext($1))`, changeID).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return nil
	}
	defer func() { _, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock(hashtext($1))`, changeID) }()
	return s.runLocked(ctx, orgID, changeID)
}

type runnable struct {
	Status    string
	VersionID string
	PlanHash  string
	Impact    domain.Impact
	Spec      domain.PolicySpec
	Epoch     int64
	RunID     string
	Cancel    bool
}

func (s *Store) runLocked(ctx context.Context, orgID, changeID string) error {
	var prep runnable
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var status, versionID, planHash, requester string
		var epoch int64
		var cancel bool
		var impactRaw []byte
		err := tx.QueryRow(ctx, `
			SELECT status, policy_version_id::text, plan_hash, impact, requested_by::text, execution_epoch, cancel_requested
			FROM change_requests WHERE organization_id = $1 AND id = $2 FOR UPDATE
		`, orgID, changeID).Scan(&status, &versionID, &planHash, &impactRaw, &requester, &epoch, &cancel)
		if err != nil {
			return mapDB(err, "change request")
		}
		switch status {
		case "succeeded", "failed", "compensated", "rollback_failed", "cancelled", "rejected", "validated":
			return nil
		}
		if err := ensureLatestVersion(ctx, tx, versionID); err != nil {
			return forceStatus(ctx, tx, changeID, "failed", "A newer policy version superseded this plan")
		}
		impact, spec, err := impactTx(ctx, tx, orgID, versionID)
		if err != nil {
			return err
		}
		hash, err := PlanHash(spec, impact)
		if err != nil {
			return err
		}
		applied, err := countStepStatus(ctx, tx, changeID, "apply:%", "succeeded")
		if err != nil {
			return err
		}
		if hash != planHash && applied == 0 && status != "unknown" {
			return forceStatus(ctx, tx, changeID, "failed", "The plan no longer matches the approved preview")
		}
		if status == "approved" {
			if err := transition(ctx, tx, orgID, changeID, "approved", "queued", ""); err != nil {
				return err
			}
			status = "queued"
		}
		if status == "queued" {
			if err := transition(ctx, tx, orgID, changeID, "queued", "applying", ""); err != nil {
				return err
			}
			status = "applying"
		}
		if status == "unknown" {
			if err := forceStatus(ctx, tx, changeID, "applying", "retry after unknown outcome"); err != nil {
				return err
			}
			status = "applying"
		}
		var approved bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM approvals
				WHERE change_request_id = $1 AND decision = 'approved' AND plan_hash = $2
			)
		`, changeID, planHash).Scan(&approved); err != nil {
			return err
		}
		if !approved {
			return apierr.Forbidden("execution requires an approval of this exact plan hash")
		}
		if epoch == 0 {
			var domainName string
			if err := tx.QueryRow(ctx, `
				SELECT s.execution_domain
				FROM devices d JOIN sites s ON s.id = d.site_id
				WHERE d.organization_id = $1 AND d.id::text = ANY($2::text[])
				ORDER BY s.execution_domain LIMIT 1
			`, orgID, deviceIDs(impact)).Scan(&domainName); err != nil {
				return err
			}
			if err := tx.QueryRow(ctx, `
				INSERT INTO authority_epochs (organization_id, execution_domain, epoch, holder)
				VALUES ($1, $2, 1, $3)
				ON CONFLICT (organization_id, execution_domain)
				DO UPDATE SET epoch = authority_epochs.epoch + 1, holder = EXCLUDED.holder, updated_at = now()
				RETURNING epoch
			`, orgID, domainName, "worker").Scan(&epoch); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE change_requests SET execution_epoch = $2 WHERE id = $1`, changeID, epoch); err != nil {
				return err
			}
		}
		var runID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO workflow_runs (organization_id, change_request_id, status, started_at)
			VALUES ($1, $2, 'running', now())
			RETURNING id::text
		`, orgID, changeID).Scan(&runID); err != nil {
			return err
		}
		if err := ensureStep(ctx, tx, orgID, runID, changeID, "", "revalidate", epoch); err != nil {
			return err
		}
		if err := ensureStep(ctx, tx, orgID, runID, changeID, "", "backup", epoch); err != nil {
			return err
		}
		if err := ensureStep(ctx, tx, orgID, runID, changeID, "", "verify", epoch); err != nil {
			return err
		}
		for _, id := range deviceIDs(impact) {
			if err := ensureStep(ctx, tx, orgID, runID, changeID, id, "apply:"+id, epoch); err != nil {
				return err
			}
			if err := ensureStep(ctx, tx, orgID, runID, changeID, id, "compensate:"+id, epoch); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE execution_steps SET status = 'succeeded', updated_at = now(), result = '{"ok":true}' WHERE change_request_id = $1 AND operation_key = 'revalidate' AND status = 'pending'`, changeID); err != nil {
			return err
		}
		prep = runnable{Status: status, VersionID: versionID, PlanHash: planHash, Impact: impact, Spec: spec, Epoch: epoch, RunID: runID, Cancel: cancel}
		return nil
	})
	if err != nil || prep.VersionID == "" {
		return err
	}
	if err := s.backupStep(ctx, orgID, changeID, prep); err != nil {
		return err
	}
	var results []string
	for _, id := range deviceIDs(prep.Impact) {
		cancel, err := s.cancelRequested(ctx, orgID, changeID)
		if err != nil {
			return err
		}
		if cancel {
			results = append(results, "cancelled")
			break
		}
		status, err := s.applyOne(ctx, orgID, changeID, prep.VersionID, prep.Epoch, prep.Spec, id)
		if err != nil {
			return err
		}
		results = append(results, status)
		if status == "unknown" {
			return s.forceChange(ctx, orgID, changeID, "unknown", "Device outcome is unknown. Retry will inspect observed state before writing again.")
		}
	}
	return s.finishChange(ctx, orgID, changeID, prep, results)
}

func deviceIDs(impact domain.Impact) []string {
	ids := []string{}
	for _, item := range impact.Items {
		if item.DeviceID != "" && !item.Blocking && item.Action == "add_acl" {
			ids = append(ids, item.DeviceID)
		}
	}
	return ids
}

func countStepStatus(ctx context.Context, tx pgx.Tx, changeID, pattern, status string) (int, error) {
	var n int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM execution_steps
		WHERE change_request_id = $1 AND operation_key LIKE $2 AND status = $3
	`, changeID, pattern, status).Scan(&n)
	return n, err
}

func transition(ctx context.Context, tx pgx.Tx, orgID, changeID, from, to, detail string) error {
	if from == to {
		return nil
	}
	if !domain.CanTransition(from, to) {
		return fmt.Errorf("illegal change transition %s -> %s", from, to)
	}
	_, err := tx.Exec(ctx, `
		UPDATE change_requests SET status = $3, error_detail = $4, updated_at = now()
		WHERE organization_id = $1 AND id = $2 AND status = $5
	`, orgID, changeID, to, detail, from)
	return err
}

func forceStatus(ctx context.Context, tx pgx.Tx, changeID, status, detail string) error {
	_, err := tx.Exec(ctx, `UPDATE change_requests SET status = $2, error_detail = $3, updated_at = now() WHERE id = $1`, changeID, status, detail)
	return err
}

func ensureStep(ctx context.Context, tx pgx.Tx, orgID, runID, changeID, deviceID, key string, epoch int64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO execution_steps (organization_id, workflow_run_id, change_request_id, device_id, operation_key, status, authority_epoch)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid, $5, 'pending', $6)
		ON CONFLICT (change_request_id, operation_key) DO NOTHING
	`, orgID, runID, changeID, deviceID, key, epoch)
	return err
}

func (s *Store) backupStep(ctx context.Context, orgID, changeID string, prep runnable) error {
	return s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM execution_steps WHERE change_request_id = $1 AND operation_key = 'backup'`, changeID).Scan(&status); err != nil {
			return err
		}
		if status == "succeeded" {
			return nil
		}
		manifest := map[string]any{
			"change_id":         changeID,
			"policy_version_id": prep.VersionID,
			"plan_hash":         prep.PlanHash,
			"spec":              prep.Spec,
			"note":              "Platform configuration backup. Device credentials are not included.",
		}
		var backupID string
		var stored []byte
		if err := tx.QueryRow(ctx, `
			INSERT INTO configuration_backups (organization_id, scope, manifest, checksum, created_by)
			SELECT $1, $2, $3, 'pending', requested_by FROM change_requests WHERE id = $4
			RETURNING id::text, manifest
		`, orgID, "change:"+changeID, mustJSON(manifest), changeID).Scan(&backupID, &stored); err != nil {
			return err
		}
		sum := sha256.Sum256(stored)
		checksum := hex.EncodeToString(sum[:])
		if _, err := tx.Exec(ctx, `UPDATE configuration_backups SET checksum = $2 WHERE id = $1`, backupID, checksum); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE execution_steps SET status = 'succeeded', result = $2, updated_at = now()
			WHERE change_request_id = $1 AND operation_key = 'backup'
		`, changeID, mustJSON(map[string]string{"checksum": checksum}))
		return err
	})
}

func (s *Store) cancelRequested(ctx context.Context, orgID, changeID string) (bool, error) {
	var cancel bool
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT cancel_requested FROM change_requests WHERE organization_id = $1 AND id = $2`, orgID, changeID).Scan(&cancel)
	})
	return cancel, err
}

func (s *Store) applyOne(ctx context.Context, orgID, changeID, versionID string, epoch int64, spec domain.PolicySpec, deviceID string) (string, error) {
	status := "failed"
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		key := "apply:" + deviceID
		var current string
		var attempt int
		err := tx.QueryRow(ctx, `
			SELECT status, attempt FROM execution_steps
			WHERE change_request_id = $1 AND operation_key = $2 FOR UPDATE
		`, changeID, key).Scan(&current, &attempt)
		if err != nil {
			return err
		}
		if current == "succeeded" {
			status = "succeeded"
			return nil
		}
		var present bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM simulator_acls WHERE device_id = $1 AND policy_version_id = $2)
		`, deviceID, versionID).Scan(&present); err != nil {
			return err
		}
		if current == "unknown" && present {
			status = "succeeded"
			_, err = tx.Exec(ctx, `UPDATE execution_steps SET status = 'succeeded', attempt = attempt + 1, updated_at = now(), result = '{"inspected":true}' WHERE change_request_id = $1 AND operation_key = $2`, changeID, key)
			return err
		}
		var failWrite, failRollback, uncertain, committed bool
		err = tx.QueryRow(ctx, `
			SELECT fail_next_write, fail_next_rollback, uncertain_next_write, uncertain_committed
			FROM devices WHERE organization_id = $1 AND id = $2 FOR UPDATE
		`, orgID, deviceID).Scan(&failWrite, &failRollback, &uncertain, &committed)
		if err != nil {
			return mapDB(err, "device")
		}
		_ = failRollback
		next := "succeeded"
		detail := map[string]any{"device_id": deviceID, "epoch": epoch}
		switch {
		case uncertain:
			if _, err := tx.Exec(ctx, `UPDATE devices SET uncertain_next_write = false, uncertain_committed = false WHERE id = $1`, deviceID); err != nil {
				return err
			}
			if committed {
				if err := insertACL(ctx, tx, orgID, deviceID, versionID, spec); err != nil {
					return err
				}
			}
			next = "unknown"
			detail["outcome"] = "unknown"
		case failWrite:
			if _, err := tx.Exec(ctx, `UPDATE devices SET fail_next_write = false WHERE id = $1`, deviceID); err != nil {
				return err
			}
			next = "failed"
			detail["outcome"] = "write rejected by simulator"
		default:
			if err := insertACL(ctx, tx, orgID, deviceID, versionID, spec); err != nil {
				return err
			}
			if err := writeObserved(ctx, tx, orgID, deviceID); err != nil {
				return err
			}
			detail["outcome"] = "applied"
		}
		_, err = tx.Exec(ctx, `
			UPDATE execution_steps
			SET status = $3, attempt = $4, authority_epoch = $5, result = $6, updated_at = now()
			WHERE change_request_id = $1 AND operation_key = $2
		`, changeID, key, next, attempt+1, epoch, mustJSON(detail))
		status = next
		return err
	})
	return status, err
}

func insertACL(ctx context.Context, tx pgx.Tx, orgID, deviceID, versionID string, spec domain.PolicySpec) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO simulator_acls (organization_id, device_id, policy_version_id, spec)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (device_id, policy_version_id) DO NOTHING
	`, orgID, deviceID, versionID, mustJSON(spec))
	return err
}

func writeObserved(ctx context.Context, tx pgx.Tx, orgID, deviceID string) error {
	rows, err := tx.Query(ctx, `SELECT policy_version_id::text, spec FROM simulator_acls WHERE organization_id = $1 AND device_id = $2 ORDER BY policy_version_id`, orgID, deviceID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type ace struct {
		PolicyVersionID string          `json:"policy_version_id"`
		Spec            json.RawMessage `json:"spec"`
	}
	var aces []ace
	for rows.Next() {
		var item ace
		if err := rows.Scan(&item.PolicyVersionID, &item.Spec); err != nil {
			return err
		}
		aces = append(aces, item)
	}
	if aces == nil {
		aces = []ace{}
	}
	doc := map[string]any{"acls": aces, "collector": "simulator", "note": "Observed state comes from the simulator, not a device poll."}
	_, err = tx.Exec(ctx, `
		INSERT INTO observed_states (organization_id, device_id, document, freshness, collector)
		VALUES ($1, $2, $3, 'fresh', 'simulator')
	`, orgID, deviceID, mustJSON(doc))
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE devices SET observed_at = now(), health = 'healthy' WHERE id = $1 AND organization_id = $2`, deviceID, orgID)
	return err
}

func (s *Store) forceChange(ctx context.Context, orgID, changeID, status, detail string) error {
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE change_requests SET status = $3, error_detail = $4, updated_at = now() WHERE organization_id = $1 AND id = $2`, orgID, changeID, status, detail)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE workflow_runs SET status = $2, last_error = $3, finished_at = now() WHERE change_request_id = $1`, changeID, status, detail)
		return err
	})
	if err != nil {
		return err
	}
	return s.Audit(ctx, orgID, "", "change."+status, "change_request", changeID, "", map[string]string{"detail": detail})
}

func (s *Store) finishChange(ctx context.Context, orgID, changeID string, prep runnable, results []string) error {
	succeeded, failed, cancelled := 0, 0, 0
	for _, result := range results {
		switch result {
		case "succeeded":
			succeeded++
		case "failed":
			failed++
		case "cancelled":
			cancelled++
		}
	}
	expected := len(deviceIDs(prep.Impact))
	if cancelled > 0 && succeeded == 0 {
		return s.forceChange(ctx, orgID, changeID, "cancelled", "Cancelled before device writes")
	}
	if failed > 0 || (cancelled > 0 && succeeded > 0) {
		return s.compensate(ctx, orgID, changeID, prep)
	}
	if succeeded != expected {
		return s.forceChange(ctx, orgID, changeID, "failed", "Not every device write succeeded")
	}
	return s.verifySuccess(ctx, orgID, changeID, prep)
}

func (s *Store) compensate(ctx context.Context, orgID, changeID string, prep runnable) error {
	if err := s.forceChange(ctx, orgID, changeID, "partial", "One or more devices did not accept the change"); err != nil {
		return err
	}
	if err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return forceStatus(ctx, tx, changeID, "compensating", "Removing simulator ACLs from devices that were updated")
	}); err != nil {
		return err
	}
	rollbackFailed := false
	for _, id := range deviceIDs(prep.Impact) {
		var applyStatus string
		err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT status FROM execution_steps WHERE change_request_id = $1 AND operation_key = $2`, changeID, "apply:"+id).Scan(&applyStatus)
		})
		if err != nil {
			return err
		}
		if applyStatus != "succeeded" {
			_ = s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `UPDATE execution_steps SET status = 'skipped', updated_at = now() WHERE change_request_id = $1 AND operation_key = $2 AND status = 'pending'`, changeID, "compensate:"+id)
				return err
			})
			continue
		}
		err = s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
			var fail bool
			if err := tx.QueryRow(ctx, `SELECT fail_next_rollback FROM devices WHERE id = $1 FOR UPDATE`, id).Scan(&fail); err != nil {
				return err
			}
			key := "compensate:" + id
			if fail {
				if _, err := tx.Exec(ctx, `UPDATE devices SET fail_next_rollback = false WHERE id = $1`, id); err != nil {
					return err
				}
				_, err := tx.Exec(ctx, `UPDATE execution_steps SET status = 'compensation_failed', updated_at = now(), result = '{"outcome":"rollback failed"}' WHERE change_request_id = $1 AND operation_key = $2`, changeID, key)
				rollbackFailed = true
				return err
			}
			if _, err := tx.Exec(ctx, `DELETE FROM simulator_acls WHERE organization_id = $1 AND device_id = $2 AND policy_version_id = $3`, orgID, id, prep.VersionID); err != nil {
				return err
			}
			if err := writeObserved(ctx, tx, orgID, id); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE execution_steps SET status = 'compensated', updated_at = now(), result = '{"outcome":"removed"}' WHERE change_request_id = $1 AND operation_key = $2`, changeID, key)
			return err
		})
		if err != nil {
			return err
		}
	}
	if rollbackFailed {
		return s.forceChange(ctx, orgID, changeID, "rollback_failed", "Compensation failed on at least one device. Inspect steps before retrying.")
	}
	return s.forceChange(ctx, orgID, changeID, "compensated", "Applied device changes were removed after a partial failure")
}

func (s *Store) verifySuccess(ctx context.Context, orgID, changeID string, prep runnable) error {
	return s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if err := forceStatus(ctx, tx, changeID, "verifying", ""); err != nil {
			return err
		}
		missing := 0
		assertions := []map[string]any{}
		for _, id := range deviceIDs(prep.Impact) {
			var ok bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM simulator_acls WHERE device_id = $1 AND policy_version_id = $2)`, id, prep.VersionID).Scan(&ok); err != nil {
				return err
			}
			assertions = append(assertions, map[string]any{"device_id": id, "acl_present": ok, "kind": "simulator"})
			if !ok {
				missing++
			}
		}
		result := "passed"
		if missing > 0 {
			result = "failed"
		}
		evidence := "Verification compared simulator ACL rows with the approved policy version. This is not a traffic probe."
		if _, err := tx.Exec(ctx, `
			INSERT INTO verification_results (organization_id, change_request_id, scope, assertions, result, evidence)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, orgID, changeID, "change", mustJSON(assertions), result, evidence); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE execution_steps SET status = $2, updated_at = now(), result = $3 WHERE change_request_id = $1 AND operation_key = 'verify'`, changeID, map[bool]string{true: "succeeded", false: "failed"}[result == "passed"], mustJSON(map[string]string{"result": result}))
		if err != nil {
			return err
		}
		if result != "passed" {
			return forceStatus(ctx, tx, changeID, "failed", "Verification did not observe the approved ACL")
		}
		doc := map[string]any{"policy_version_id": prep.VersionID, "plan_hash": prep.PlanHash, "spec": prep.Spec, "devices": deviceIDs(prep.Impact)}
		if _, err := tx.Exec(ctx, `
			INSERT INTO desired_states (organization_id, scope, revision, document, source_change_id)
			VALUES ($1, 'fabric', COALESCE((SELECT MAX(revision) FROM desired_states WHERE organization_id = $1 AND scope = 'fabric'), 0) + 1, $2, $3)
		`, orgID, mustJSON(doc), changeID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE execution_steps
			SET status = 'skipped', updated_at = now(), result = '{"reason":"rollback was not required"}'
			WHERE change_request_id = $1 AND operation_key LIKE 'compensate:%' AND status = 'pending'
		`, changeID); err != nil {
			return err
		}
		if err := forceStatus(ctx, tx, changeID, "succeeded", ""); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE workflow_runs SET status = 'succeeded', finished_at = now() WHERE change_request_id = $1`, changeID)
		if err != nil {
			return err
		}
		return insertAudit(ctx, tx, orgID, "", "change.succeeded", "change_request", changeID, "", map[string]string{"plan_hash": prep.PlanHash})
	})
}

func (s *Store) ClaimedChanges(ctx context.Context) ([][2]string, error) {
	var out [][2]string
	err := s.WithBypass(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT organization_id::text, id::text FROM change_requests
			WHERE status IN ('queued', 'applying', 'unknown')
			ORDER BY updated_at
			LIMIT 20
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var pair [2]string
			if err := rows.Scan(&pair[0], &pair[1]); err != nil {
				return err
			}
			out = append(out, pair)
		}
		return rows.Err()
	})
	return out, err
}
