package store

import "testing"

func TestNavigationRejectsMissingSettings(t *testing.T) {
	nav := DefaultNavigation()
	nav.Items = nav.Items[:1]
	if err := ValidateNavigation(nav); err == nil {
		t.Fatal("expected missing pages to be rejected")
	}
	if err := ValidateNavigation(DefaultNavigation()); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileKeepsCustomOrder(t *testing.T) {
	saved := Navigation{Items: []NavItem{
		{ID: "command", Type: "page", Page: "command"},
		{ID: "network", Type: "group", Label: "Custom network", Children: []NavItem{
			{ID: "inventory", Type: "page", Page: "inventory"},
			{ID: "topology", Type: "page", Page: "topology"},
			{ID: "desired", Type: "page", Page: "desired"},
		}},
	}}
	got := ReconcileNavigation(saved)
	if err := ValidateNavigation(got); err != nil {
		t.Fatal(err)
	}
	if got.Items[1].Label != "Custom network" {
		t.Fatalf("custom group label lost: %#v", got.Items[1])
	}
	if got.Items[1].Children[0].Page != "inventory" || got.Items[1].Children[1].Page != "topology" {
		t.Fatalf("custom order lost: %#v", got.Items[1].Children)
	}
}

func TestDefaultNavigationOmitsUnimplementedPages(t *testing.T) {
	seen := map[string]bool{}
	var walk func([]NavItem)
	walk = func(items []NavItem) {
		for _, item := range items {
			if item.Type == "page" {
				seen[item.Page] = true
			}
			walk(item.Children)
		}
	}
	walk(DefaultNavigation().Items)
	for _, page := range []string{"firmware", "qos", "endpoints", "fabric", "provisioning", "validation", "assurance"} {
		if seen[page] {
			t.Fatalf("default navigation still lists %s", page)
		}
	}
	if err := ValidateNavigation(DefaultNavigation()); err != nil {
		t.Fatal(err)
	}
}

func TestPreviousDefaultBecomesDashboardNavigation(t *testing.T) {
	saved := Navigation{Items: []NavItem{
		{ID: "command-room", Type: "group", Label: "Command Room", Children: []NavItem{{ID: "command", Type: "page", Page: "command"}}},
		{ID: "infrastructure", Type: "group", Label: "Network Infrastructure", Children: []NavItem{{ID: "sites", Type: "page", Page: "sites"}}},
		{ID: "connectivity", Type: "group", Label: "Networks & Connectivity", Children: []NavItem{{ID: "vlans", Type: "page", Page: "vlans"}}},
		{ID: "segmentation", Type: "group", Label: "Policies & Segmentation", Children: []NavItem{{ID: "policies", Type: "page", Page: "policies"}}},
		{ID: "automation", Type: "group", Label: "Automation", Children: []NavItem{{ID: "changes", Type: "page", Page: "changes"}}},
		{ID: "assurance-group", Type: "group", Label: "Assurance", Children: []NavItem{{ID: "incidents", Type: "page", Page: "incidents"}}},
		{ID: "reports-group", Type: "group", Label: "Reports & Compliance", Children: []NavItem{{ID: "reports", Type: "page", Page: "reports"}}},
		{ID: "administration", Type: "group", Label: "Administration", Children: []NavItem{{ID: "settings", Type: "page", Page: "settings"}}},
	}}
	got := ReconcileNavigation(saved)
	if got.Items[0].Label != "Dashboard" || got.Items[1].Label != "Network" {
		t.Fatalf("previous catalog was not replaced: %#v", got.Items[0])
	}
}

func TestThemeContrast(t *testing.T) {
	theme := DefaultTheme()
	theme.Preset = "custom"
	theme.Text = "#ffffff"
	theme.Surface = "#ffffff"
	theme.Sidebar = "#ffffff"
	theme.BG = "#ffffff"
	if _, err := NormalizeTheme(theme); err == nil {
		t.Fatal("expected low contrast to be rejected")
	}
	navy, err := NormalizeTheme(Theme{Preset: "navy", Mode: "dark"})
	if err != nil || navy.Primary != "#3ec6e0" {
		t.Fatalf("%v %#v", err, navy)
	}
}
