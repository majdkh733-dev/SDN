package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/majdkh733-dev/SDN/internal/apierr"
	"github.com/majdkh733-dev/SDN/internal/domain"
)

type VLANProposal struct {
	ChangeID  string `json:"change_id"`
	VersionID string `json:"version_id"`
	PlanHash  string `json:"plan_hash"`
	Config    string `json:"generated_config"`
	Blocked   bool   `json:"blocked"`
	Reason    string `json:"reason,omitempty"`
	Adapter   string `json:"adapter"`
	Note      string `json:"note"`
}

func (s *Store) ProposeVLAN(ctx context.Context, orgID, userID string, spec domain.PolicySpec) (VLANProposal, error) {
	spec.Operation = "vlan"
	spec.Action = "allow"
	spec.Protocol = "any"
	spec = domain.NormalizeSpec(spec)
	if errs := spec.ValidateShape(); len(errs) > 0 {
		return VLANProposal{}, apierr.Invalid(errs[0])
	}
	var proposal VLANProposal
	proposal.Adapter = "simulator"
	proposal.Note = "Generated for the simulator lab adapter. It is not a vendor configuration commit."
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.validateSpecRefs(ctx, tx, orgID, spec); err != nil {
			return err
		}
		body, err := renderDeviceConfig(ctx, tx, orgID, spec.VLAN.DeviceID, true)
		if err != nil {
			return err
		}
		spec.VLAN.Baseline = checksumText(body)
		var policyID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO policies (organization_id, tenant_id, name, created_by)
			VALUES ($1, $2, $3, $4) RETURNING id::text
		`, orgID, spec.TenantID, spec.Name, userID).Scan(&policyID); err != nil {
			return mapDB(err, "policy")
		}
		var rec PolicyRecord
		if err := insertVersion(ctx, tx, orgID, policyID, userID, 1, spec, &rec); err != nil {
			return err
		}
		impact, got, err := impactTx(ctx, tx, orgID, rec.VersionID)
		if err != nil {
			return err
		}
		if impact.Blocked {
			proposal.Blocked = true
			proposal.Reason = impact.BlockReason
			proposal.VersionID = rec.VersionID
			return apierr.Unsupported(impact.BlockReason)
		}
		hash, err := PlanHash(got, impact)
		if err != nil {
			return err
		}
		var changeID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO change_requests (organization_id, policy_version_id, plan_hash, impact, status, requested_by)
			VALUES ($1, $2, $3, $4, 'validated', $5) RETURNING id::text
		`, orgID, rec.VersionID, hash, mustJSON(impact), userID).Scan(&changeID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO vlans (organization_id, site_id, vid, name, status, provenance)
			VALUES ($1, $2, $3, $4, 'intended', 'change')
			ON CONFLICT (organization_id, site_id, vid) DO UPDATE
			SET name = EXCLUDED.name, status = 'intended', updated_at = now()
		`, orgID, spec.SiteIDs[0], spec.VLAN.VID, spec.VLAN.Name)
		if err != nil {
			return err
		}
		iface := ""
		if spec.VLAN.Interface != "" {
			iface = "\n interface " + spec.VLAN.Interface
		}
		proposal = VLANProposal{
			ChangeID: changeID, VersionID: rec.VersionID, PlanHash: hash,
			Config:  fmt.Sprintf("vlan %d\n name %s%s\n", spec.VLAN.VID, spec.VLAN.Name, iface),
			Adapter: "simulator",
			Note:    "The change is validated, not applied. Approval and author confirmation are still required. Rollback deletes the simulator VLAN row.",
		}
		return insertAudit(ctx, tx, orgID, userID, "vlan.proposed", "change_request", changeID, "", map[string]any{"vid": spec.VLAN.VID})
	})
	return proposal, err
}
