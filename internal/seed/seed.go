package seed

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/majdkh733-dev/SDN/internal/adapter"
	"github.com/majdkh733-dev/SDN/internal/store"
)

func Run(ctx context.Context, st *store.Store, password string) error {
	if password == "" {
		return fmt.Errorf("DEV_ADMIN_PASSWORD is required to seed demonstration accounts")
	}
	if _, _, err := st.OrgBySlug(ctx, "nexora-demo"); err == nil {
		return nil
	}
	demoID, err := st.NewID(ctx)
	if err != nil {
		return err
	}
	otherID, err := st.NewID(ctx)
	if err != nil {
		return err
	}
	if err := st.CreateOrganization(ctx, demoID, "Nexora Demo", "nexora-demo", 365); err != nil {
		return err
	}
	if err := st.CreateOrganization(ctx, otherID, "Isolated Labs", "isolated-labs", 90); err != nil {
		return err
	}
	type acct struct {
		email, name, org, role string
	}
	accounts := []acct{
		{"engineer@nexora.demo", "Rana Engineer", demoID, "engineer"},
		{"approver@nexora.demo", "Omar Approver", demoID, "approver"},
		{"admin@nexora.demo", "Lina Administrator", demoID, "administrator"},
		{"recovery@nexora.demo", "Hadi Recovery", demoID, "recovery_operator"},
		{"viewer@nexora.demo", "Nora Viewer", demoID, "viewer"},
		{"admin@isolated.demo", "Private Admin", otherID, "administrator"},
	}
	for _, account := range accounts {
		user, err := st.EnsureUser(ctx, account.email, account.name, password)
		if err != nil {
			return err
		}
		if err := st.AddMembership(ctx, account.org, user.ID, account.role); err != nil {
			return err
		}
	}
	riyadh, err := st.CreateSite(ctx, demoID, "Riyadh DC1", "Riyadh", "riyadh")
	if err != nil {
		return err
	}
	if _, err := st.CreateSite(ctx, demoID, "Jeddah Edge", "Jeddah", "jeddah"); err != nil {
		return err
	}
	private, err := st.CreateSite(ctx, otherID, "Private Site", "Undisclosed", "private")
	if err != nil {
		return err
	}
	conn, err := st.CreateConnector(ctx, demoID, riyadh.ID, "riyadh-simulator", "simulator", "", "")
	if err != nil {
		return err
	}
	if _, err := st.PersistDiscovery(ctx, demoID, riyadh.ID, conn.ID, adapter.SimulatorDiscovery(riyadh.Name)); err != nil {
		return err
	}
	otherConn, err := st.CreateConnector(ctx, otherID, private.ID, "private-simulator", "simulator", "", "")
	if err != nil {
		return err
	}
	if _, err := st.PersistDiscovery(ctx, otherID, private.ID, otherConn.ID, adapter.SimulatorDiscovery(private.Name)); err != nil {
		return err
	}
	fabric, err := st.CreateFabric(ctx, demoID, "Corporate", "Demonstration fabric. Not a production OVN deployment.")
	if err != nil {
		return err
	}
	finance, err := st.CreateTenant(ctx, demoID, fabric.ID, "Finance", "fin-vrf")
	if err != nil {
		return err
	}
	guest, err := st.CreateTenant(ctx, demoID, fabric.ID, "Guest", "guest-vrf")
	if err != nil {
		return err
	}
	vlan := 120
	vni := 10120
	if _, err := st.CreateSegment(ctx, demoID, finance.ID, "finance-users", "10.20.0.0/24", &vlan, &vni); err != nil {
		return err
	}
	vlan = 121
	vni = 10121
	if _, err := st.CreateSegment(ctx, demoID, finance.ID, "finance-apps", "10.20.1.0/24", &vlan, &vni); err != nil {
		return err
	}
	vlan = 200
	vni = 10200
	if _, err := st.CreateSegment(ctx, demoID, guest.ID, "guest-wifi", "10.30.0.0/24", &vlan, &vni); err != nil {
		return err
	}
	for _, item := range []store.Integration{
		{Kind: "simulator", Name: "Deterministic simulator", Status: "demo", Detail: "Verified by automated tests in this repository. Not a forwarding device."},
		{Kind: "ovn", Name: "OVN/OVS northbound", Status: "implemented_unverified", Detail: "Adapter calls ovn-nbctl. No physical lab result is recorded for this deployment."},
		{Kind: "netconf", Name: "NETCONF/YANG", Status: "not_implemented", Detail: "Write support is on the roadmap and is not available."},
		{Kind: "gnmi", Name: "gNMI/OpenConfig", Status: "not_implemented", Detail: "Telemetry and configuration are not implemented."},
		{Kind: "vendor_api", Name: "Vendor APIs", Status: "not_implemented", Detail: "No vendor write API is enrolled."},
		{Kind: "openflow", Name: "OpenFlow", Status: "not_implemented", Detail: "No confirmed target use case."},
		{Kind: "gitops", Name: "GitOps", Status: "not_implemented", Detail: "Import and review are not implemented. Nothing is deployed from Git."},
	} {
		if err := st.SeedIntegration(ctx, demoID, item); err != nil {
			return err
		}
		if err := st.SeedIntegration(ctx, otherID, item); err != nil {
			return err
		}
	}
	admin, err := st.EnsureUser(ctx, "admin@nexora.demo", "Lina Administrator", "")
	if err != nil {
		return err
	}
	if _, err := st.CreateBackup(ctx, demoID, admin.ID, "organization"); err != nil {
		return err
	}
	devices, err := st.ListDevices(ctx, demoID, riyadh.ID, "")
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		return fmt.Errorf("seed discovery produced no devices")
	}
	err = st.WithOrg(ctx, demoID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO desired_states (organization_id, scope, revision, document)
			VALUES ($1, 'fabric', 1, $2)
		`, demoID, fmt.Sprintf(`{"devices":["%s"],"spec":{"name":"seed"},"note":"Seeded desired state for drift demonstration"}`, devices[0].ID))
		return err
	})
	if err != nil {
		return err
	}
	if _, err := st.ScanDrift(ctx, demoID); err != nil {
		return err
	}
	return st.Audit(ctx, demoID, admin.ID, "organization.seeded", "organization", demoID, "", map[string]any{
		"notice": "Seeded demonstration data. No device credentials were stored.",
	})
}
