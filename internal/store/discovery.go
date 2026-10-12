package store

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/majdkh733-dev/SDN/internal/apierr"
	"github.com/majdkh733-dev/SDN/internal/discover"
)

type DiscoveryProfile struct {
	ID              string          `json:"id"`
	SiteID          string          `json:"site_id"`
	ConnectorID     string          `json:"connector_id"`
	Name            string          `json:"name"`
	Targets         json.RawMessage `json:"targets"`
	Exclusions      json.RawMessage `json:"exclusions"`
	ScanProfile     string          `json:"scan_profile"`
	TimeoutMS       int             `json:"timeout_ms"`
	Concurrency     int             `json:"concurrency"`
	ScheduleMinutes int             `json:"schedule_minutes"`
	AutoImport      bool            `json:"auto_import"`
	CredentialRef   string          `json:"credential_ref"`
	NextRunAt       *time.Time      `json:"next_run_at"`
	CreatedAt       time.Time       `json:"created_at"`
}

type DiscoveryJob struct {
	ID              string          `json:"id"`
	SiteID          string          `json:"site_id"`
	ConnectorID     string          `json:"connector_id"`
	ProfileID       string          `json:"profile_id"`
	Status          string          `json:"status"`
	Progress        int             `json:"progress"`
	DiscoveredCount int             `json:"discovered_count"`
	Error           string          `json:"error"`
	Attempt         int             `json:"attempt"`
	Targets         json.RawMessage `json:"targets"`
	StartedAt       *time.Time      `json:"started_at"`
	FinishedAt      *time.Time      `json:"finished_at"`
	CreatedAt       time.Time       `json:"created_at"`
}

type DiscoveryObservation struct {
	ID              string          `json:"id"`
	JobID           string          `json:"job_id"`
	SiteID          string          `json:"site_id"`
	ConnectorID     string          `json:"connector_id"`
	IP              string          `json:"ip"`
	Hostname        string          `json:"hostname"`
	MAC             string          `json:"mac"`
	Vendor          string          `json:"vendor"`
	Model           string          `json:"model"`
	DeviceType      string          `json:"device_type"`
	Serial          string          `json:"serial"`
	OSVersion       string          `json:"os_version"`
	Reachable       bool            `json:"reachable"`
	Methods         json.RawMessage `json:"methods"`
	Confidence      string          `json:"confidence"`
	IdentityKey     string          `json:"identity_key"`
	MatchedDeviceID string          `json:"matched_device_id"`
	MatchStatus     string          `json:"match_status"`
	Imported        bool            `json:"imported"`
}

type ProfileInput struct {
	Name            string
	ConnectorID     string
	Targets         []string
	Exclusions      []string
	ScanProfile     string
	TimeoutMS       int
	Concurrency     int
	ScheduleMinutes int
	AutoImport      bool
	CredentialRef   string
}

func (s *Store) CreateProfile(ctx context.Context, orgID, siteID string, in ProfileInput) (DiscoveryProfile, error) {
	targets, err := discover.Expand(in.Targets, in.Exclusions)
	if err != nil {
		return DiscoveryProfile{}, apierr.Invalid(err.Error())
	}
	_ = targets
	if in.ScanProfile == "" {
		in.ScanProfile = "reachability"
	}
	if in.ScanProfile != "reachability" && in.ScanProfile != "snmp" {
		return DiscoveryProfile{}, apierr.Invalid("scan profile must be reachability or snmp")
	}
	if in.TimeoutMS == 0 {
		in.TimeoutMS = 800
	}
	if in.Concurrency == 0 {
		in.Concurrency = 8
	}
	var next any
	if in.ScheduleMinutes > 0 {
		next = time.Now()
	}
	var profile DiscoveryProfile
	err = s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var connectorSite, lifecycle string
		if err := tx.QueryRow(ctx, `
			SELECT site_id::text, lifecycle FROM connectors WHERE organization_id = $1 AND id = $2
		`, orgID, in.ConnectorID).Scan(&connectorSite, &lifecycle); err != nil {
			return mapDB(err, "connector")
		}
		if connectorSite != siteID {
			return apierr.Invalid("connector belongs to a different site")
		}
		if lifecycle != "active" {
			return apierr.Conflict("revoked", "connector is not active")
		}
		var siteOK bool
		if err := tx.QueryRow(ctx, `SELECT true FROM sites WHERE organization_id = $1 AND id = $2`, orgID, siteID).Scan(&siteOK); err != nil {
			return mapDB(err, "site")
		}
		return scanProfile(tx.QueryRow(ctx, `
			INSERT INTO discovery_profiles (
				organization_id, site_id, connector_id, name, targets, exclusions, scan_profile,
				timeout_ms, concurrency, schedule_minutes, auto_import, credential_ref, next_run_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			RETURNING id::text, site_id::text, connector_id::text, name, targets, exclusions, scan_profile,
			          timeout_ms, concurrency, schedule_minutes, auto_import, credential_ref, next_run_at, created_at
		`, orgID, siteID, in.ConnectorID, in.Name, mustJSON(in.Targets), mustJSON(in.Exclusions), in.ScanProfile,
			in.TimeoutMS, in.Concurrency, in.ScheduleMinutes, in.AutoImport, in.CredentialRef, next), &profile)
	})
	return profile, err
}

func scanProfile(row interface{ Scan(...any) error }, profile *DiscoveryProfile) error {
	return row.Scan(
		&profile.ID, &profile.SiteID, &profile.ConnectorID, &profile.Name, &profile.Targets, &profile.Exclusions,
		&profile.ScanProfile, &profile.TimeoutMS, &profile.Concurrency, &profile.ScheduleMinutes, &profile.AutoImport,
		&profile.CredentialRef, &profile.NextRunAt, &profile.CreatedAt,
	)
}

func (s *Store) ListProfiles(ctx context.Context, orgID, siteID string) ([]DiscoveryProfile, error) {
	var out []DiscoveryProfile
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, site_id::text, connector_id::text, name, targets, exclusions, scan_profile,
			       timeout_ms, concurrency, schedule_minutes, auto_import, credential_ref, next_run_at, created_at
			FROM discovery_profiles
			WHERE organization_id = $1 AND ($2 = '' OR site_id::text = $2)
			ORDER BY name
		`, orgID, siteID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item DiscoveryProfile
			if err := scanProfile(rows, &item); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []DiscoveryProfile{}
	}
	return out, err
}

func (s *Store) RunProfile(ctx context.Context, orgID, profileID string) (DiscoveryJob, error) {
	var job DiscoveryJob
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var siteID, connectorID string
		var targets, exclusions []byte
		if err := tx.QueryRow(ctx, `
			SELECT site_id::text, connector_id::text, targets, exclusions
			FROM discovery_profiles WHERE organization_id = $1 AND id = $2
		`, orgID, profileID).Scan(&siteID, &connectorID, &targets, &exclusions); err != nil {
			return mapDB(err, "discovery profile")
		}
		var existing string
		err := tx.QueryRow(ctx, `
			SELECT id::text FROM discovery_jobs
			WHERE organization_id = $1 AND profile_id = $2 AND status IN ('queued', 'running')
			LIMIT 1
		`, orgID, profileID).Scan(&existing)
		if err == nil {
			return apierr.Conflict("job_active", "this profile already has a queued or running scan")
		}
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
		return scanJob(tx.QueryRow(ctx, `
			INSERT INTO discovery_jobs (organization_id, site_id, connector_id, profile_id, targets)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id::text, site_id::text, connector_id::text, COALESCE(profile_id::text, ''), status, progress,
			          discovered_count, error, attempt, targets, started_at, finished_at, created_at
		`, orgID, siteID, connectorID, profileID, targets), &job)
	})
	return job, err
}

func scanJob(row interface{ Scan(...any) error }, job *DiscoveryJob) error {
	return row.Scan(
		&job.ID, &job.SiteID, &job.ConnectorID, &job.ProfileID, &job.Status, &job.Progress,
		&job.DiscoveredCount, &job.Error, &job.Attempt, &job.Targets, &job.StartedAt, &job.FinishedAt, &job.CreatedAt,
	)
}

func (s *Store) GetJob(ctx context.Context, orgID, id string) (DiscoveryJob, error) {
	var job DiscoveryJob
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return mapDB(scanJob(tx.QueryRow(ctx, `
			SELECT id::text, site_id::text, connector_id::text, COALESCE(profile_id::text, ''), status, progress,
			       discovered_count, error, attempt, targets, started_at, finished_at, created_at
			FROM discovery_jobs WHERE organization_id = $1 AND id = $2
		`, orgID, id), &job), "discovery job")
	})
	return job, err
}

func (s *Store) ListJobs(ctx context.Context, orgID, connectorID, siteID string) ([]DiscoveryJob, error) {
	var out []DiscoveryJob
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, site_id::text, connector_id::text, COALESCE(profile_id::text, ''), status, progress,
			       discovered_count, error, attempt, targets, started_at, finished_at, created_at
			FROM discovery_jobs
			WHERE organization_id = $1
			  AND ($2 = '' OR connector_id::text = $2)
			  AND ($3 = '' OR site_id::text = $3)
			ORDER BY created_at DESC
			LIMIT 50
		`, orgID, connectorID, siteID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item DiscoveryJob
			if err := scanJob(rows, &item); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []DiscoveryJob{}
	}
	return out, err
}

func (s *Store) CancelJob(ctx context.Context, orgID, id string) (DiscoveryJob, error) {
	var job DiscoveryJob
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		err := scanJob(tx.QueryRow(ctx, `
			UPDATE discovery_jobs
			SET status = 'cancelled', finished_at = now(), updated_at = now(), error = 'cancelled by operator'
			WHERE organization_id = $1 AND id = $2 AND status IN ('queued', 'running')
			RETURNING id::text, site_id::text, connector_id::text, COALESCE(profile_id::text, ''), status, progress,
			          discovered_count, error, attempt, targets, started_at, finished_at, created_at
		`, orgID, id), &job)
		if err != nil {
			return mapDB(err, "discovery job")
		}
		return nil
	})
	return job, err
}

func (s *Store) EnqueueDueScans(ctx context.Context) (int, error) {
	var n int
	err := s.WithBypass(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT organization_id::text, id::text
			FROM discovery_profiles
			WHERE schedule_minutes > 0 AND (next_run_at IS NULL OR next_run_at <= now())
			ORDER BY next_run_at NULLS FIRST
			LIMIT 20
			FOR UPDATE SKIP LOCKED
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		type due struct{ org, id string }
		var items []due
		for rows.Next() {
			var item due
			if err := rows.Scan(&item.org, &item.id); err != nil {
				return err
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, item := range items {
			var active bool
			_ = tx.QueryRow(ctx, `
				SELECT true FROM discovery_jobs
				WHERE profile_id = $1 AND status IN ('queued', 'running') LIMIT 1
			`, item.id).Scan(&active)
			if !active {
				if _, err := tx.Exec(ctx, `
					INSERT INTO discovery_jobs (organization_id, site_id, connector_id, profile_id, targets)
					SELECT organization_id, site_id, connector_id, id, targets
					FROM discovery_profiles WHERE id = $1
				`, item.id); err != nil {
					return err
				}
				n++
			}
			if _, err := tx.Exec(ctx, `
				UPDATE discovery_profiles
				SET next_run_at = now() + (schedule_minutes || ' minutes')::interval, updated_at = now()
				WHERE id = $1
			`, item.id); err != nil {
				return err
			}
		}
		return nil
	})
	return n, err
}

func (s *Store) leaseDiscovery(ctx context.Context, tx pgx.Tx, orgID, connectorID string) (string, json.RawMessage, bool, error) {
	if _, err := tx.Exec(ctx, `
		UPDATE discovery_jobs
		SET status = 'failed', error = 'lease expired', finished_at = now(), updated_at = now()
		WHERE organization_id = $1 AND connector_id = $2 AND status = 'running' AND lease_until < now() AND attempt >= 3
	`, orgID, connectorID); err != nil {
		return "", nil, false, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE discovery_jobs
		SET status = 'queued', lease_until = NULL, updated_at = now()
		WHERE organization_id = $1 AND connector_id = $2 AND status = 'running' AND lease_until < now() AND attempt < 3
	`, orgID, connectorID); err != nil {
		return "", nil, false, err
	}
	var id string
	var targets, exclusions []byte
	var timeout, concurrency int
	var ref string
	err := tx.QueryRow(ctx, `
		UPDATE discovery_jobs SET
			status = 'running',
			attempt = attempt + 1,
			lease_until = now() + interval '2 minutes',
			started_at = COALESCE(started_at, now()),
			updated_at = now()
		WHERE id = (
			SELECT id FROM discovery_jobs
			WHERE organization_id = $1 AND connector_id = $2 AND status = 'queued'
			ORDER BY created_at
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id::text, targets,
			COALESCE((SELECT exclusions FROM discovery_profiles p WHERE p.id = discovery_jobs.profile_id), '[]'::jsonb),
			COALESCE((SELECT timeout_ms FROM discovery_profiles p WHERE p.id = discovery_jobs.profile_id), 800),
			COALESCE((SELECT concurrency FROM discovery_profiles p WHERE p.id = discovery_jobs.profile_id), 8),
			COALESCE((SELECT credential_ref FROM discovery_profiles p WHERE p.id = discovery_jobs.profile_id), '')
	`, orgID, connectorID).Scan(&id, &targets, &exclusions, &timeout, &concurrency, &ref)
	if err != nil {
		return "", nil, false, nil
	}
	payload := map[string]any{
		"targets": json.RawMessage(targets), "exclusions": json.RawMessage(exclusions),
		"timeout_ms": timeout, "concurrency": concurrency,
		"ports": []int{22, 80, 443, 161}, "credential_ref": ref,
	}
	return id, mustJSON(payload), true, nil
}

func (s *Store) JobProgress(ctx context.Context, orgID, connectorID, jobID string, progress, count int) (bool, error) {
	cancelled := false
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var status string
		err := tx.QueryRow(ctx, `
			SELECT status FROM discovery_jobs
			WHERE organization_id = $1 AND id = $2 AND connector_id = $3
		`, orgID, jobID, connectorID).Scan(&status)
		if err != nil {
			return mapDB(err, "discovery job")
		}
		if status == "cancelled" {
			cancelled = true
			return nil
		}
		if status != "running" {
			return nil
		}
		_, err = tx.Exec(ctx, `
			UPDATE discovery_jobs
			SET progress = $4, discovered_count = $5, lease_until = now() + interval '2 minutes', updated_at = now()
			WHERE id = $2
		`, orgID, jobID, connectorID, progress, count)
		return err
	})
	return cancelled, err
}

type InboundObservation struct {
	IP         string          `json:"ip"`
	Hostname   string          `json:"hostname"`
	MAC        string          `json:"mac"`
	Vendor     string          `json:"vendor"`
	Model      string          `json:"model"`
	DeviceType string          `json:"device_type"`
	Serial     string          `json:"serial"`
	OSVersion  string          `json:"os_version"`
	Reachable  bool            `json:"reachable"`
	Methods    json.RawMessage `json:"methods"`
	Confidence string          `json:"confidence"`
	Explicit   bool            `json:"explicit"`
}

func (s *Store) CompleteDiscovery(ctx context.Context, orgID, connectorID, jobID string, observations []InboundObservation, partial bool, errText string) error {
	return s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var status, siteID, profileID string
		var auto bool
		err := tx.QueryRow(ctx, `
			SELECT j.status, j.site_id::text, COALESCE(j.profile_id::text, ''), COALESCE(p.auto_import, false)
			FROM discovery_jobs j
			LEFT JOIN discovery_profiles p ON p.id = j.profile_id
			WHERE j.organization_id = $1 AND j.id = $2 AND j.connector_id = $3
		`, orgID, jobID, connectorID).Scan(&status, &siteID, &profileID, &auto)
		if err != nil {
			return mapDB(err, "discovery job")
		}
		if status == "completed" || status == "partial" || status == "failed" || status == "cancelled" {
			return nil
		}
		for _, obs := range observations {
			key := IdentityKey(siteID, obs.Serial, obs.MAC, obs.IP)
			if key == "" {
				continue
			}
			if len(obs.Methods) == 0 {
				obs.Methods = []byte("[]")
			}
			if obs.Confidence == "" {
				obs.Confidence = "low"
			}
			matchID, matchStatus, err := matchAsset(ctx, tx, orgID, siteID, key, obs)
			if err != nil {
				return err
			}
			var id string
			err = tx.QueryRow(ctx, `
				INSERT INTO discovery_observations (
					organization_id, job_id, site_id, connector_id, ip, hostname, mac, vendor, model, device_type,
					serial, os_version, reachable, methods, confidence, identity_key, matched_device_id, match_status
				) VALUES (
					$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16, NULLIF($17, '')::uuid, $18
				)
				ON CONFLICT (job_id, identity_key) DO NOTHING
				RETURNING id::text
			`, orgID, jobID, siteID, connectorID, obs.IP, obs.Hostname, strings.ToLower(obs.MAC), obs.Vendor, obs.Model, obs.DeviceType,
				obs.Serial, obs.OSVersion, obs.Reachable, obs.Methods, obs.Confidence, key, matchID, matchStatus).Scan(&id)
			if err == pgx.ErrNoRows {
				continue
			}
			if err != nil {
				return err
			}
			if auto && matchStatus != "ambiguous" {
				if _, err := importObservationTx(ctx, tx, orgID, "", id); err != nil {
					return err
				}
			}
		}
		final := "completed"
		if partial {
			final = "partial"
		}
		if errText != "" && len(observations) == 0 {
			final = "failed"
		}
		_, err = tx.Exec(ctx, `
			UPDATE discovery_jobs
			SET status = $4, error = $5, progress = 100, discovered_count = (
				SELECT COUNT(*) FROM discovery_observations WHERE job_id = $2
			), finished_at = now(), lease_until = NULL, updated_at = now()
			WHERE organization_id = $1 AND id = $2 AND connector_id = $3
		`, orgID, jobID, connectorID, final, errText)
		return err
	})
}

func matchAsset(ctx context.Context, tx pgx.Tx, orgID, siteID, key string, obs InboundObservation) (string, string, error) {
	rows, err := tx.Query(ctx, `
		SELECT id::text, identity_key, mac, serial, management_endpoint
		FROM devices
		WHERE organization_id = $1 AND site_id = $2 AND lifecycle <> 'archived'
		  AND (
		    identity_key = $3
		    OR ($4 <> '' AND lower(serial) = lower($4))
		    OR ($5 <> '' AND lower(mac) = lower($5))
		    OR ($6 <> '' AND management_endpoint = $6)
		  )
	`, orgID, siteID, key, obs.Serial, obs.MAC, obs.IP)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()
	type hit struct{ id, key, mac, serial, ip string }
	var hits []hit
	for rows.Next() {
		var item hit
		if err := rows.Scan(&item.id, &item.key, &item.mac, &item.serial, &item.ip); err != nil {
			return "", "", err
		}
		hits = append(hits, item)
	}
	if err := rows.Err(); err != nil {
		return "", "", err
	}
	if len(hits) == 0 {
		return "", "new", nil
	}
	if len(hits) > 1 {
		return "", "ambiguous", nil
	}
	item := hits[0]
	if obs.MAC != "" && item.mac != "" && !strings.EqualFold(obs.MAC, item.mac) {
		return item.id, "ambiguous", nil
	}
	if obs.Serial != "" && item.serial != "" && !strings.EqualFold(obs.Serial, item.serial) {
		return item.id, "ambiguous", nil
	}
	return item.id, "matched", nil
}

func IdentityKey(siteID, serial, mac, ip string) string {
	serial = strings.ToLower(strings.TrimSpace(serial))
	mac = strings.ToLower(strings.TrimSpace(mac))
	ip = strings.TrimSpace(ip)
	switch {
	case serial != "":
		return "site:" + siteID + ":serial:" + serial
	case mac != "":
		return "site:" + siteID + ":mac:" + mac
	case ip != "":
		return "site:" + siteID + ":ip:" + ip
	default:
		return ""
	}
}

func (s *Store) ListObservations(ctx context.Context, orgID, jobID, query, match string) ([]DiscoveryObservation, error) {
	var out []DiscoveryObservation
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, job_id::text, site_id::text, connector_id::text, ip, hostname, mac, vendor, model,
			       device_type, serial, os_version, reachable, methods, confidence, identity_key,
			       COALESCE(matched_device_id::text, ''), match_status, imported
			FROM discovery_observations
			WHERE organization_id = $1 AND job_id = $2
			  AND ($3 = '' OR ip ILIKE '%' || $3 || '%' OR hostname ILIKE '%' || $3 || '%' OR mac ILIKE '%' || $3 || '%')
			  AND ($4 = '' OR match_status = $4)
			ORDER BY ip
		`, orgID, jobID, query, match)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item DiscoveryObservation
			if err := rows.Scan(
				&item.ID, &item.JobID, &item.SiteID, &item.ConnectorID, &item.IP, &item.Hostname, &item.MAC,
				&item.Vendor, &item.Model, &item.DeviceType, &item.Serial, &item.OSVersion, &item.Reachable,
				&item.Methods, &item.Confidence, &item.IdentityKey, &item.MatchedDeviceID, &item.MatchStatus, &item.Imported,
			); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if out == nil {
		out = []DiscoveryObservation{}
	}
	return out, err
}

type ImportResult struct {
	DeviceID string `json:"device_id"`
	Action   string `json:"action"`
	Detail   string `json:"detail"`
}

func (s *Store) ImportObservations(ctx context.Context, orgID, userID, jobID string, ids []string, all bool) ([]ImportResult, error) {
	if ids == nil {
		ids = []string{}
	}
	var out []ImportResult
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		query := `
			SELECT id::text FROM discovery_observations
			WHERE organization_id = $1 AND job_id = $2 AND match_status <> 'ambiguous'`
		args := []any{orgID, jobID}
		if !all {
			query += ` AND id::text = ANY($3::text[])`
			args = append(args, ids)
		}
		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		var chosen []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			chosen = append(chosen, id)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, id := range chosen {
			result, err := importObservationTx(ctx, tx, orgID, userID, id)
			if err != nil {
				return err
			}
			out = append(out, result)
		}
		return nil
	})
	if out == nil {
		out = []ImportResult{}
	}
	return out, err
}

func importObservationTx(ctx context.Context, tx pgx.Tx, orgID, userID, observationID string) (ImportResult, error) {
	var obs DiscoveryObservation
	var methods []byte
	err := tx.QueryRow(ctx, `
		SELECT id::text, job_id::text, site_id::text, connector_id::text, ip, hostname, mac, vendor, model,
		       device_type, serial, os_version, reachable, methods, confidence, identity_key,
		       COALESCE(matched_device_id::text, ''), match_status, imported
		FROM discovery_observations
		WHERE organization_id = $1 AND id = $2
	`, orgID, observationID).Scan(
		&obs.ID, &obs.JobID, &obs.SiteID, &obs.ConnectorID, &obs.IP, &obs.Hostname, &obs.MAC, &obs.Vendor, &obs.Model,
		&obs.DeviceType, &obs.Serial, &obs.OSVersion, &obs.Reachable, &methods, &obs.Confidence, &obs.IdentityKey,
		&obs.MatchedDeviceID, &obs.MatchStatus, &obs.Imported,
	)
	if err != nil {
		return ImportResult{}, mapDB(err, "observation")
	}
	if obs.MatchStatus == "ambiguous" {
		return ImportResult{Action: "skipped", Detail: "ambiguous match requires review"}, nil
	}
	role := obs.DeviceType
	if role == "" {
		role = "unknown"
	}
	health := "unknown"
	note := "Discovered by a site connector. Reachability is not continuous monitoring, and empty fields were not identified."
	if !obs.Reachable {
		note = "Explicit target did not answer. Absence is not assumed."
	}
	var deviceID, action string
	if obs.MatchedDeviceID != "" {
		deviceID = obs.MatchedDeviceID
		action = "updated"
		var locked []string
		if err := tx.QueryRow(ctx, `SELECT locked_fields FROM devices WHERE organization_id = $1 AND id = $2`, orgID, deviceID).Scan(&locked); err != nil {
			return ImportResult{}, err
		}
		lockedSet := map[string]bool{}
		for _, field := range locked {
			lockedSet[field] = true
		}
		_, err = tx.Exec(ctx, `
			UPDATE devices SET
				hostname = CASE WHEN $3 = '' OR 'hostname' = ANY(locked_fields) THEN hostname ELSE $3 END,
				mac = CASE WHEN $4 = '' OR 'mac' = ANY(locked_fields) THEN mac ELSE $4 END,
				vendor = CASE WHEN $5 = '' OR 'vendor' = ANY(locked_fields) THEN vendor ELSE $5 END,
				model = CASE WHEN $6 = '' OR 'model' = ANY(locked_fields) THEN model ELSE $6 END,
				serial = CASE WHEN $7 = '' OR 'serial' = ANY(locked_fields) THEN serial ELSE $7 END,
				os_version = CASE WHEN $8 = '' OR 'os_version' = ANY(locked_fields) THEN os_version ELSE $8 END,
				name = CASE WHEN $9 = '' OR 'name' = ANY(locked_fields) THEN name ELSE $9 END,
				role = CASE WHEN 'role' = ANY(locked_fields) THEN role ELSE $10 END,
				management_endpoint = CASE WHEN $11 = '' THEN management_endpoint ELSE $11 END,
				identity_key = $12,
				connector_id = $13,
				last_seen_at = now(),
				observed_at = now(),
				discovery_source = 'discovery',
				support_note = $14
			WHERE organization_id = $1 AND id = $2
		`, orgID, deviceID, obs.Hostname, obs.MAC, obs.Vendor, obs.Model, obs.Serial, obs.OSVersion,
			preferredName(obs.Hostname, obs.IP), role, obs.IP, obs.IdentityKey, obs.ConnectorID, note)
		if err != nil {
			return ImportResult{}, err
		}
		_ = lockedSet
	} else {
		action = "created"
		name := preferredName(obs.Hostname, obs.IP)
		err = tx.QueryRow(ctx, `
			INSERT INTO devices (
				organization_id, site_id, connector_id, name, model, os_version, role, management_endpoint,
				health, data_origin, support_note, observed_at, hostname, mac, vendor, serial, identity_key,
				first_seen_at, last_seen_at, discovery_source
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,'discovery',$10,now(),$11,$12,$13,$14,$15,now(),now(),'discovery'
			)
			ON CONFLICT (organization_id, identity_key) WHERE identity_key <> '' DO UPDATE SET
				last_seen_at = now(), observed_at = now()
			RETURNING id::text
		`, orgID, obs.SiteID, obs.ConnectorID, name, obs.Model, obs.OSVersion, role, obs.IP, health, note,
			obs.Hostname, obs.MAC, obs.Vendor, obs.Serial, obs.IdentityKey).Scan(&deviceID)
		if err != nil {
			return ImportResult{}, err
		}
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM capability_snapshots
		WHERE organization_id = $1 AND device_id = $2
		  AND capability IN ('inventory', 'interfaces', 'topology', 'health', 'policy')
	`, orgID, deviceID); err != nil {
		return ImportResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO capability_snapshots (organization_id, device_id, capability, supported, evidence, connector_version)
		VALUES
			($1, $2, 'inventory', $3, 'Address was observed by the site connector.', 'site'),
			($1, $2, 'interfaces', false, 'Interface collection is not available for this observation.', 'site'),
			($1, $2, 'topology', false, 'No LLDP, CDP, or manual link was reported.', 'site'),
			($1, $2, 'health', false, 'A scan-time probe is not continuous health monitoring.', 'site'),
			($1, $2, 'policy', false, 'No management adapter is enrolled for configuration changes.', 'site')
	`, orgID, deviceID, obs.Reachable || obs.IP != ""); err != nil {
		return ImportResult{}, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE discovery_observations
		SET imported = true, matched_device_id = $3, match_status = CASE WHEN match_status = 'new' THEN 'matched' ELSE match_status END
		WHERE organization_id = $1 AND id = $2
	`, orgID, observationID, deviceID)
	if err != nil {
		return ImportResult{}, err
	}
	var actor any
	if userID != "" {
		actor = userID
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO asset_imports (organization_id, device_id, job_id, observation_id, action, detail)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, orgID, deviceID, obs.JobID, observationID, action, "identity "+obs.IdentityKey)
	_ = actor
	return ImportResult{DeviceID: deviceID, Action: action, Detail: obs.IdentityKey}, err
}

func preferredName(hostname, ip string) string {
	if hostname != "" {
		return hostname
	}
	if ip != "" {
		return ip
	}
	return "unknown"
}

func (s *Store) UpdateDeviceFields(ctx context.Context, orgID, id, name string, lock []string) (Device, error) {
	allowed := map[string]bool{"name": true, "hostname": true, "model": true, "vendor": true, "role": true, "os_version": true, "mac": true}
	var clean []string
	for _, field := range lock {
		if allowed[field] {
			clean = append(clean, field)
		}
	}
	if clean == nil {
		clean = []string{}
	}
	var device Device
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE devices
			SET name = CASE WHEN $3 = '' THEN name ELSE $3 END,
			    locked_fields = (
			      SELECT ARRAY(SELECT DISTINCT unnest(locked_fields || $4::text[]))
			    )
			WHERE organization_id = $1 AND id = $2
		`, orgID, id, name, clean)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return apierr.NotFound("device")
		}
		row := tx.QueryRow(ctx, deviceSelect+` WHERE organization_id = $1 AND id = $2`, orgID, id)
		item, err := scanDevice(row)
		device = item
		return err
	})
	return device, err
}

func (s *Store) ImportCSV(ctx context.Context, orgID, siteID, connectorID string, rows []InboundObservation) ([]ImportResult, error) {
	var out []ImportResult
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT true FROM sites WHERE organization_id = $1 AND id = $2`, orgID, siteID).Scan(&ok); err != nil {
			return mapDB(err, "site")
		}
		for _, obs := range rows {
			key := IdentityKey(siteID, obs.Serial, obs.MAC, obs.IP)
			if key == "" {
				out = append(out, ImportResult{Action: "skipped", Detail: "row has no ip, mac, or serial"})
				continue
			}
			obsMatch := obs
			matchID, status, err := matchAsset(ctx, tx, orgID, siteID, key, obsMatch)
			if err != nil {
				return err
			}
			if status == "ambiguous" {
				out = append(out, ImportResult{Action: "skipped", Detail: "ambiguous match for " + obs.IP})
				continue
			}
			name := preferredName(obs.Hostname, obs.IP)
			role := obs.DeviceType
			if role == "" {
				role = "unknown"
			}
			var deviceID string
			action := "created"
			if matchID != "" {
				action = "updated"
				deviceID = matchID
				if _, err := tx.Exec(ctx, `
					UPDATE devices SET
						hostname = CASE WHEN 'hostname' = ANY(locked_fields) OR $3 = '' THEN hostname ELSE $3 END,
						mac = CASE WHEN 'mac' = ANY(locked_fields) OR $4 = '' THEN mac ELSE $4 END,
						vendor = CASE WHEN 'vendor' = ANY(locked_fields) OR $5 = '' THEN vendor ELSE $5 END,
						model = CASE WHEN 'model' = ANY(locked_fields) OR $6 = '' THEN model ELSE $6 END,
						serial = CASE WHEN 'serial' = ANY(locked_fields) OR $7 = '' THEN serial ELSE $7 END,
						os_version = CASE WHEN 'os_version' = ANY(locked_fields) OR $8 = '' THEN os_version ELSE $8 END,
						name = CASE WHEN 'name' = ANY(locked_fields) THEN name ELSE $9 END,
						management_endpoint = CASE WHEN $10 = '' THEN management_endpoint ELSE $10 END,
						identity_key = $11,
						last_seen_at = now(),
						discovery_source = 'csv'
					WHERE organization_id = $1 AND id = $2
				`, orgID, deviceID, obs.Hostname, obs.MAC, obs.Vendor, obs.Model, obs.Serial, obs.OSVersion, name, obs.IP, key); err != nil {
					return err
				}
			} else {
				if err := tx.QueryRow(ctx, `
					INSERT INTO devices (
						organization_id, site_id, connector_id, name, model, os_version, role, management_endpoint,
						health, data_origin, support_note, hostname, mac, vendor, serial, identity_key,
						first_seen_at, last_seen_at, discovery_source
					) VALUES (
						$1,$2, NULLIF($3, '')::uuid, $4,$5,$6,$7,$8,'unknown','csv',
						'Imported from CSV. Empty fields were not supplied.',
						$9,$10,$11,$12,$13, now(), now(), 'csv'
					)
					ON CONFLICT (organization_id, identity_key) WHERE identity_key <> '' DO UPDATE SET last_seen_at = now()
					RETURNING id::text
				`, orgID, siteID, connectorID, name, obs.Model, obs.OSVersion, role, obs.IP, obs.Hostname, obs.MAC, obs.Vendor, obs.Serial, key).Scan(&deviceID); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO asset_imports (organization_id, device_id, action, detail) VALUES ($1, $2, $3, $4)
			`, orgID, deviceID, action, key); err != nil {
				return err
			}
			out = append(out, ImportResult{DeviceID: deviceID, Action: action, Detail: key})
		}
		return nil
	})
	return out, err
}

func (s *Store) ListImports(ctx context.Context, orgID, deviceID string) ([]map[string]any, error) {
	var out []map[string]any
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, COALESCE(device_id::text, ''), COALESCE(job_id::text, ''), action, detail, created_at
			FROM asset_imports
			WHERE organization_id = $1 AND ($2 = '' OR device_id::text = $2)
			ORDER BY created_at DESC
			LIMIT 50
		`, orgID, deviceID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id, device, job, action, detail string
			var at time.Time
			if err := rows.Scan(&id, &device, &job, &action, &detail, &at); err != nil {
				return err
			}
			out = append(out, map[string]any{"id": id, "device_id": device, "job_id": job, "action": action, "detail": detail, "created_at": at})
		}
		return rows.Err()
	})
	if out == nil {
		out = []map[string]any{}
	}
	return out, err
}
