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
