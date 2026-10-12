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
