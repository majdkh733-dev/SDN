package store

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/majdkh733-dev/SDN/internal/apierr"
	"github.com/majdkh733-dev/SDN/internal/domain"
)

type PolicyRecord struct {
	ID               string            `json:"id"`
	TenantID         string            `json:"tenant_id"`
	Name             string            `json:"name"`
	Lifecycle        string            `json:"lifecycle"`
	VersionID        string            `json:"version_id"`
	Revision         int               `json:"revision"`
	ValidationStatus string            `json:"validation_status"`
	ValidationErrors []string          `json:"validation_errors"`
	Spec             domain.PolicySpec `json:"spec"`
	CreatedAt        time.Time         `json:"created_at"`
}

func (s *Store) CreatePolicy(ctx context.Context, orgID, userID string, spec domain.PolicySpec) (PolicyRecord, error) {
	spec = domain.NormalizeSpec(spec)
	if errs := spec.ValidateShape(); len(errs) > 0 {
		return PolicyRecord{}, apierr.Invalid(errs[0])
	}
	var rec PolicyRecord
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.validateSpecRefs(ctx, tx, orgID, spec); err != nil {
			return err
		}
		var policyID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO policies (organization_id, tenant_id, name, created_by)
			VALUES ($1, $2, $3, $4)
			RETURNING id::text
		`, orgID, spec.TenantID, spec.Name, userID).Scan(&policyID); err != nil {
			return mapDB(err, "policy")
		}
		return insertVersion(ctx, tx, orgID, policyID, userID, 1, spec, &rec)
	})
	return rec, err
}

func (s *Store) AddPolicyVersion(ctx context.Context, orgID, userID, policyID string, spec domain.PolicySpec) (PolicyRecord, error) {
	spec = domain.NormalizeSpec(spec)
	if errs := spec.ValidateShape(); len(errs) > 0 {
		return PolicyRecord{}, apierr.Invalid(errs[0])
	}
	var rec PolicyRecord
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.validateSpecRefs(ctx, tx, orgID, spec); err != nil {
			return err
		}
		var revision int
		var name string
		err := tx.QueryRow(ctx, `
			SELECT name, COALESCE((SELECT MAX(revision) FROM policy_versions WHERE policy_id = policies.id), 0)
			FROM policies WHERE organization_id = $1 AND id = $2
		`, orgID, policyID).Scan(&name, &revision)
		if err != nil {
			return mapDB(err, "policy")
		}
		spec.Name = name
		return insertVersion(ctx, tx, orgID, policyID, userID, revision+1, spec, &rec)
	})
	return rec, err
}

func insertVersion(ctx context.Context, tx pgx.Tx, orgID, policyID, userID string, revision int, spec domain.PolicySpec, rec *PolicyRecord) error {
	raw := mustJSON(spec)
	err := tx.QueryRow(ctx, `
		INSERT INTO policy_versions (organization_id, policy_id, revision, spec, validation_status, validation_errors, created_by)
		VALUES ($1, $2, $3, $4, 'valid', '[]'::jsonb, $5)
		RETURNING id::text, created_at
	`, orgID, policyID, revision, raw, userID).Scan(&rec.VersionID, &rec.CreatedAt)
	if err != nil {
		return err
	}
	rec.ID = policyID
	rec.TenantID = spec.TenantID
	rec.Name = spec.Name
	rec.Lifecycle = "active"
	rec.Revision = revision
	rec.ValidationStatus = "valid"
	rec.ValidationErrors = []string{}
	rec.Spec = spec
	return nil
}

func (s *Store) validateSpecRefs(ctx context.Context, tx pgx.Tx, orgID string, spec domain.PolicySpec) error {
	if spec.Operation == "vlan" {
		return validateVLANRefs(ctx, tx, orgID, spec)
	}
	var tenantOK bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM tenants WHERE organization_id = $1 AND id = $2 AND lifecycle = 'active')
	`, orgID, spec.TenantID).Scan(&tenantOK); err != nil {
		return err
	}
	if !tenantOK {
		return apierr.Invalid("tenant was not found in this organization")
	}
	for _, sel := range []domain.Selector{spec.Source, spec.Destination} {
		var ok bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM segments WHERE organization_id = $1 AND tenant_id = $2 AND id = $3 AND lifecycle = 'active'
			)
		`, orgID, spec.TenantID, sel.ID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return apierr.Invalid("segment " + sel.ID + " is not in the selected tenant")
		}
	}
	for _, siteID := range spec.SiteIDs {
		var ok bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM sites WHERE organization_id = $1 AND id = $2 AND lifecycle = 'active')
		`, orgID, siteID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return apierr.Invalid("site " + siteID + " is not active in this organization")
		}
	}
	return nil
}

func (s *Store) ListPolicies(ctx context.Context, orgID string) ([]PolicyRecord, error) {
	var out []PolicyRecord
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT p.id::text, p.tenant_id::text, p.name, p.lifecycle,
			       v.id::text, v.revision, v.validation_status, v.validation_errors, v.spec, v.created_at
			FROM policies p
			JOIN LATERAL (
				SELECT * FROM policy_versions WHERE policy_id = p.id ORDER BY revision DESC LIMIT 1
			) v ON true
			WHERE p.organization_id = $1
			ORDER BY p.name
		`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			rec, err := scanPolicy(rows)
			if err != nil {
				return err
			}
			out = append(out, rec)
		}
		return rows.Err()
	})
	if out == nil {
		out = []PolicyRecord{}
	}
	return out, err
}

func (s *Store) GetPolicy(ctx context.Context, orgID, policyID string) (PolicyRecord, error) {
	var rec PolicyRecord
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT p.id::text, p.tenant_id::text, p.name, p.lifecycle,
			       v.id::text, v.revision, v.validation_status, v.validation_errors, v.spec, v.created_at
			FROM policies p
			JOIN LATERAL (
				SELECT * FROM policy_versions WHERE policy_id = p.id ORDER BY revision DESC LIMIT 1
			) v ON true
			WHERE p.organization_id = $1 AND p.id = $2
		`, orgID, policyID)
		var err error
		rec, err = scanPolicy(row)
		return mapDB(err, "policy")
	})
	return rec, err
}

func scanPolicy(row interface{ Scan(...any) error }) (PolicyRecord, error) {
	var rec PolicyRecord
	var errs, specRaw []byte
	err := row.Scan(&rec.ID, &rec.TenantID, &rec.Name, &rec.Lifecycle, &rec.VersionID, &rec.Revision, &rec.ValidationStatus, &errs, &specRaw, &rec.CreatedAt)
	if err != nil {
		return PolicyRecord{}, err
	}
	_ = jsonUnmarshal(errs, &rec.ValidationErrors)
	if rec.ValidationErrors == nil {
		rec.ValidationErrors = []string{}
	}
	if err := json.Unmarshal(specRaw, &rec.Spec); err != nil {
		return PolicyRecord{}, err
	}
	return rec, nil
}

func (s *Store) GetVersion(ctx context.Context, orgID, versionID string) (PolicyRecord, error) {
	var rec PolicyRecord
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT p.id::text, p.tenant_id::text, p.name, p.lifecycle,
			       v.id::text, v.revision, v.validation_status, v.validation_errors, v.spec, v.created_at
			FROM policy_versions v
			JOIN policies p ON p.id = v.policy_id
			WHERE v.organization_id = $1 AND v.id = $2
		`, orgID, versionID)
		var err error
		rec, err = scanPolicy(row)
		return mapDB(err, "policy version")
	})
	return rec, err
}

type deviceCap struct {
	ID     string
	Name   string
	SiteID string
	Domain string
	ACL    bool
	HasACL bool
}

func (s *Store) BuildImpact(ctx context.Context, orgID, versionID string) (domain.Impact, domain.PolicySpec, error) {
	var impact domain.Impact
	var spec domain.PolicySpec
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		impact, spec, err = impactTx(ctx, tx, orgID, versionID)
		return err
	})
	return impact, spec, err
}

func validateVLANRefs(ctx context.Context, tx pgx.Tx, orgID string, spec domain.PolicySpec) error {
	var tenantOK bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tenants WHERE organization_id = $1 AND id = $2 AND lifecycle = 'active')`, orgID, spec.TenantID).Scan(&tenantOK); err != nil {
		return err
	}
	if !tenantOK {
		return apierr.Invalid("tenant was not found in this organization")
	}
	var siteOK bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM sites WHERE organization_id = $1 AND id = $2)`, orgID, spec.SiteIDs[0]).Scan(&siteOK); err != nil {
		return err
	}
	if !siteOK {
		return apierr.Invalid("site was not found in this organization")
	}
	var deviceSite string
	var origin string
	err := tx.QueryRow(ctx, `SELECT site_id::text, data_origin FROM devices WHERE organization_id = $1 AND id = $2 AND lifecycle = 'active'`, orgID, spec.VLAN.DeviceID).Scan(&deviceSite, &origin)
	if err != nil {
		return apierr.Invalid("device was not found in this organization")
	}
	if deviceSite != spec.SiteIDs[0] {
		return apierr.Invalid("device is not in the selected site")
	}
	if origin != "simulator" {
		return apierr.Unsupported("VLAN configuration write is implemented for the simulator lab adapter only. This device has no config.write adapter.")
	}
	return nil
}

func impactTx(ctx context.Context, tx pgx.Tx, orgID, versionID string) (domain.Impact, domain.PolicySpec, error) {
	var spec domain.PolicySpec
	var raw []byte
	var status string
	err := tx.QueryRow(ctx, `
		SELECT spec, validation_status FROM policy_versions WHERE organization_id = $1 AND id = $2
	`, orgID, versionID).Scan(&raw, &status)
	if err != nil {
		return domain.Impact{}, domain.PolicySpec{}, mapDB(err, "policy version")
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		return domain.Impact{}, domain.PolicySpec{}, err
	}
	spec = domain.NormalizeSpec(spec)
	if spec.Operation == "vlan" {
		return vlanImpact(ctx, tx, orgID, versionID, status, spec)
	}
	impact := domain.Impact{PolicyVersionID: versionID}
	if status != "valid" {
		impact.Blocked = true
		impact.BlockReason = "policy version is not valid"
		return impact, spec, nil
	}
	rows, err := tx.Query(ctx, `
		SELECT d.id::text, d.name, d.site_id::text, s.execution_domain,
		       EXISTS (
		           SELECT 1 FROM capability_snapshots c
		           WHERE c.device_id = d.id AND c.capability = 'acl.v1' AND c.supported
		             AND c.discovered_at = (
		                 SELECT MAX(discovered_at) FROM capability_snapshots
		                 WHERE device_id = d.id AND capability = 'acl.v1'
		             )
		       )
		FROM devices d
		JOIN sites s ON s.id = d.site_id
		WHERE d.organization_id = $1 AND d.lifecycle = 'active' AND d.site_id::text = ANY($2::text[])
		ORDER BY d.id
	`, orgID, spec.SiteIDs)
	if err != nil {
		return domain.Impact{}, spec, err
	}
	defer rows.Close()
	domains := map[string]struct{}{}
	var devices []deviceCap
	for rows.Next() {
		var d deviceCap
		if err := rows.Scan(&d.ID, &d.Name, &d.SiteID, &d.Domain, &d.ACL); err != nil {
			return domain.Impact{}, spec, err
		}
		d.HasACL = true
		devices = append(devices, d)
		domains[d.Domain] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return domain.Impact{}, spec, err
	}
	if len(devices) == 0 {
		impact.Blocked = true
		impact.BlockReason = "no active devices were discovered in the selected sites"
		impact.Items = []domain.ImpactItem{}
		return impact, spec, nil
	}
	if len(domains) > 1 {
		impact.Blocked = true
		impact.BlockReason = "a change cannot span execution domains"
	}
	for _, d := range devices {
		item := domain.ImpactItem{
			DeviceID:       d.ID,
			DeviceName:     d.Name,
			Classification: "calculated",
			Action:         "add_acl",
			Detail:         spec.Action + " " + spec.Protocol + "/" + spec.Ports + " from segment " + spec.Source.ID + " to " + spec.Destination.ID,
		}
		if !d.ACL {
			item.Classification = "calculated"
			item.Blocking = true
			item.Action = "blocked"
			item.Detail = "device has no supported acl.v1 capability"
			impact.Blocked = true
			impact.BlockReason = "one or more devices do not support acl.v1"
		}
		impact.Items = append(impact.Items, item)
		impact.Calculated++
	}
	impact.Items = append(impact.Items, domain.ImpactItem{
		DeviceID:       "",
		DeviceName:     "",
		Classification: "inferred",
		Action:         "dependency",
		Detail:         "Links between selected devices are assumed in scope. This is not a measured path.",
	})
	impact.Inferred++
	impact.Items = append(impact.Items, domain.ImpactItem{
		DeviceID:       "",
		DeviceName:     "",
		Classification: "unknown",
		Action:         "performance",
		Detail:         "Performance impact is unknown. The simulator does not measure packet flow.",
	})
	impact.Unknown++
	impact.Items = append(impact.Items, domain.ImpactItem{
		DeviceID:       "",
		DeviceName:     "",
		Classification: "verified",
		Action:         "pending",
		Detail:         "No post-change verification has run for this preview.",
	})
	sort.Slice(impact.Items, func(i, j int) bool {
		if impact.Items[i].DeviceID == impact.Items[j].DeviceID {
			return impact.Items[i].Classification < impact.Items[j].Classification
		}
		return impact.Items[i].DeviceID < impact.Items[j].DeviceID
	})
	return impact, spec, nil
}

func vlanImpact(ctx context.Context, tx pgx.Tx, orgID, versionID, status string, spec domain.PolicySpec) (domain.Impact, domain.PolicySpec, error) {
	impact := domain.Impact{PolicyVersionID: versionID}
	if status != "valid" {
		impact.Blocked = true
		impact.BlockReason = "policy version is not valid"
		return impact, spec, nil
	}
	var name, origin, domainName string
	err := tx.QueryRow(ctx, `
		SELECT d.name, d.data_origin, s.execution_domain
		FROM devices d JOIN sites s ON s.id = d.site_id
		WHERE d.organization_id = $1 AND d.id = $2 AND d.lifecycle = 'active'
	`, orgID, spec.VLAN.DeviceID).Scan(&name, &origin, &domainName)
	if err != nil {
		impact.Blocked = true
		impact.BlockReason = "device was not found"
		impact.Items = []domain.ImpactItem{}
		return impact, spec, nil
	}
	item := domain.ImpactItem{
		DeviceID: spec.VLAN.DeviceID, DeviceName: name, Classification: "calculated", Action: "set_vlan",
		Detail: "Create VLAN " + strconv.Itoa(spec.VLAN.VID) + " " + spec.VLAN.Name + " on the simulator lab adapter",
	}
	if origin != "simulator" {
		item.Blocking = true
		item.Action = "blocked"
		item.Detail = "no adapter can write VLANs on this device"
		impact.Blocked = true
		impact.BlockReason = "VLAN write is available on the simulator lab adapter only"
	}
	_ = domainName
	impact.Items = []domain.ImpactItem{item, {
		Classification: "unknown", Action: "performance",
		Detail: "Performance impact is unknown. The simulator does not forward traffic.",
	}}
	impact.Calculated = 1
	impact.Unknown = 1
	return impact, spec, nil
}

func PlanHash(spec domain.PolicySpec, impact domain.Impact) (string, error) {
	return domain.CanonicalHash(struct {
		Spec   domain.PolicySpec `json:"spec"`
		Impact domain.Impact     `json:"impact"`
	}{Spec: spec, Impact: impact})
}
