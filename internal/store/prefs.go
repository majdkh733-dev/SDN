package store

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/majdkh733-dev/SDN/internal/apierr"
)

var pageCatalog = []string{
	"command",
	"sites", "inventory", "interfaces", "endpoints", "topology", "discovery",
	"vlans", "tenants", "subnets", "routing", "fabric", "wan",
	"policies", "groups", "qos", "validation",
	"templates", "provisioning", "backups", "drift", "changes", "firmware",
	"assurance", "incidents", "telemetry", "diagnostics",
	"reports",
	"connectors", "integrations", "credentials", "audit", "controllers", "settings",
}

func DefaultNavigation() Navigation {
	return Navigation{Items: []NavItem{
		{ID: "command-room", Type: "group", Label: "Command Room", Children: []NavItem{
			{ID: "command", Type: "page", Page: "command"},
		}},
		{ID: "infrastructure", Type: "group", Label: "Network Infrastructure", Children: []NavItem{
			{ID: "sites", Type: "page", Page: "sites"},
			{ID: "inventory", Type: "page", Page: "inventory"},
			{ID: "interfaces", Type: "page", Page: "interfaces"},
			{ID: "endpoints", Type: "page", Page: "endpoints"},
			{ID: "topology", Type: "page", Page: "topology"},
			{ID: "discovery", Type: "page", Page: "discovery"},
		}},
		{ID: "connectivity", Type: "group", Label: "Networks & Connectivity", Children: []NavItem{
			{ID: "vlans", Type: "page", Page: "vlans"},
			{ID: "tenants", Type: "page", Page: "tenants"},
			{ID: "subnets", Type: "page", Page: "subnets"},
			{ID: "routing", Type: "page", Page: "routing"},
			{ID: "fabric", Type: "page", Page: "fabric"},
			{ID: "wan", Type: "page", Page: "wan"},
		}},
		{ID: "segmentation", Type: "group", Label: "Policies & Segmentation", Children: []NavItem{
			{ID: "policies", Type: "page", Page: "policies"},
			{ID: "groups", Type: "page", Page: "groups"},
			{ID: "qos", Type: "page", Page: "qos"},
			{ID: "validation", Type: "page", Page: "validation"},
		}},
		{ID: "automation", Type: "group", Label: "Automation", Children: []NavItem{
			{ID: "templates", Type: "page", Page: "templates"},
			{ID: "provisioning", Type: "page", Page: "provisioning"},
			{ID: "backups", Type: "page", Page: "backups"},
			{ID: "drift", Type: "page", Page: "drift"},
			{ID: "changes", Type: "page", Page: "changes"},
			{ID: "firmware", Type: "page", Page: "firmware"},
		}},
		{ID: "assurance-group", Type: "group", Label: "Assurance", Children: []NavItem{
			{ID: "assurance", Type: "page", Page: "assurance"},
			{ID: "incidents", Type: "page", Page: "incidents"},
			{ID: "telemetry", Type: "page", Page: "telemetry"},
			{ID: "diagnostics", Type: "page", Page: "diagnostics"},
		}},
		{ID: "reports-group", Type: "group", Label: "Reports & Compliance", Children: []NavItem{
			{ID: "reports", Type: "page", Page: "reports"},
		}},
		{ID: "administration", Type: "group", Label: "Administration", Children: []NavItem{
			{ID: "connectors", Type: "page", Page: "connectors"},
			{ID: "integrations", Type: "page", Page: "integrations"},
			{ID: "credentials", Type: "page", Page: "credentials"},
			{ID: "audit", Type: "page", Page: "audit"},
			{ID: "controllers", Type: "page", Page: "controllers"},
			{ID: "settings", Type: "page", Page: "settings"},
		}},
	}}
}

type NavItem struct {
	ID       string    `json:"id"`
	Type     string    `json:"type"`
	Page     string    `json:"page,omitempty"`
	Label    string    `json:"label,omitempty"`
	Children []NavItem `json:"children,omitempty"`
}

type Navigation struct {
	Items []NavItem `json:"items"`
}

type Theme struct {
	Mode    string `json:"mode"`
	Preset  string `json:"preset"`
	Primary string `json:"primary"`
	Accent  string `json:"accent"`
	Sidebar string `json:"sidebar"`
	Surface string `json:"surface"`
	Border  string `json:"border"`
	Text    string `json:"text"`
	BG      string `json:"bg"`
}

func DefaultTheme() Theme {
	return Theme{
		Mode: "system", Preset: "default",
		Primary: "#2c5f8a", Accent: "#2c5f8a", Sidebar: "#ffffff", Surface: "#ffffff",
		Border: "#e4e8ee", Text: "#1c2834", BG: "#f3f5f8",
	}
}

func NavyTheme() Theme {
	return Theme{
		Mode: "dark", Preset: "navy",
		Primary: "#3ec6e0", Accent: "#7ee0f2", Sidebar: "#0a1624", Surface: "#102033",
		Border: "#1e3a52", Text: "#e8f1f8", BG: "#071018",
	}
}

type Preferences struct {
	Navigation Navigation `json:"navigation"`
	Theme      Theme      `json:"theme"`
}

func (s *Store) Preferences(ctx context.Context, orgID string) (Preferences, error) {
	prefs := Preferences{Navigation: DefaultNavigation(), Theme: DefaultTheme()}
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var nav, theme []byte
		err := tx.QueryRow(ctx, `SELECT navigation, theme FROM org_preferences WHERE organization_id = $1`, orgID).Scan(&nav, &theme)
		if err == pgx.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		if len(nav) > 0 && string(nav) != "{}" && string(nav) != "null" {
			var parsed Navigation
			if err := json.Unmarshal(nav, &parsed); err == nil && len(parsed.Items) > 0 {
				prefs.Navigation = ReconcileNavigation(parsed)
			}
		}
		if len(theme) > 0 && string(theme) != "{}" && string(theme) != "null" {
			var parsed Theme
			if err := json.Unmarshal(theme, &parsed); err == nil && parsed.Mode != "" {
				prefs.Theme = parsed
			}
		}
		return nil
	})
	return prefs, err
}

func (s *Store) SaveNavigation(ctx context.Context, orgID string, nav Navigation) (Navigation, error) {
	if err := ValidateNavigation(nav); err != nil {
		return Navigation{}, err
	}
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO org_preferences (organization_id, navigation)
			VALUES ($1, $2)
			ON CONFLICT (organization_id) DO UPDATE SET navigation = EXCLUDED.navigation, updated_at = now()
		`, orgID, mustJSON(nav))
		return err
	})
	return nav, err
}

func (s *Store) ResetNavigation(ctx context.Context, orgID string) (Navigation, error) {
	nav := DefaultNavigation()
	_, err := s.SaveNavigation(ctx, orgID, nav)
	return nav, err
}

func (s *Store) SaveTheme(ctx context.Context, orgID string, theme Theme) (Theme, error) {
	normalized, err := NormalizeTheme(theme)
	if err != nil {
		return Theme{}, err
	}
	err = s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO org_preferences (organization_id, theme)
			VALUES ($1, $2)
			ON CONFLICT (organization_id) DO UPDATE SET theme = EXCLUDED.theme, updated_at = now()
		`, orgID, mustJSON(normalized))
		return err
	})
	return normalized, err
}

func (s *Store) ResetTheme(ctx context.Context, orgID string) (Theme, error) {
	theme := DefaultTheme()
	_, err := s.SaveTheme(ctx, orgID, theme)
	return theme, err
}

func ValidateNavigation(nav Navigation) error {
	if len(nav.Items) == 0 {
		return apierr.Invalid("navigation is empty")
	}
	seen := map[string]bool{}
	var walk func(items []NavItem, depth int) error
	walk = func(items []NavItem, depth int) error {
		if depth > 1 {
			return apierr.Invalid("navigation groups cannot contain other groups")
		}
		for _, item := range items {
			if item.ID == "" {
				return apierr.Invalid("navigation item is missing an id")
			}
			if seen[item.ID] {
				return apierr.Invalid("navigation item " + item.ID + " is duplicated")
			}
			seen[item.ID] = true
			switch item.Type {
			case "page":
				if !knownPage(item.Page) {
					return apierr.Invalid("unknown page " + item.Page)
				}
				if seen["page:"+item.Page] {
					return apierr.Invalid("page " + item.Page + " is listed more than once")
				}
				seen["page:"+item.Page] = true
				if len(item.Children) > 0 {
					return apierr.Invalid("pages cannot contain children")
				}
			case "group":
				if strings.TrimSpace(item.Label) == "" {
					return apierr.Invalid("a group needs a label")
				}
				if len(item.Children) == 0 {
					return apierr.Invalid("a group must keep its pages; move them before removing the group")
				}
				if err := walk(item.Children, depth+1); err != nil {
					return err
				}
			default:
				return apierr.Invalid("navigation type must be page or group")
			}
		}
		return nil
	}
	if err := walk(nav.Items, 0); err != nil {
		return err
	}
	for _, page := range pageCatalog {
		if !seen["page:"+page] {
			return apierr.Invalid("page " + page + " is missing; removing a group must not delete its pages")
		}
	}
	return nil
}

func ReconcileNavigation(saved Navigation) Navigation {
	filtered := filterKnown(saved.Items)
	present := map[string]bool{}
	var mark func([]NavItem)
	mark = func(items []NavItem) {
		for _, item := range items {
			if item.Type == "page" {
				present[item.Page] = true
			}
			mark(item.Children)
		}
	}
	mark(filtered)
	for _, page := range pageCatalog {
		if present[page] {
			continue
		}
		filtered = placeMissing(filtered, page)
	}
	nav := Navigation{Items: filtered}
	if err := ValidateNavigation(nav); err != nil {
		return DefaultNavigation()
	}
	return nav
}

func filterKnown(items []NavItem) []NavItem {
	var out []NavItem
	for _, item := range items {
		switch item.Type {
		case "page":
			if knownPage(item.Page) {
				out = append(out, NavItem{ID: item.ID, Type: "page", Page: item.Page})
			}
		case "group":
			children := filterKnown(item.Children)
			if len(children) == 0 {
				continue
			}
			out = append(out, NavItem{ID: item.ID, Type: "group", Label: item.Label, Children: children})
		}
	}
	return out
}

func placeMissing(items []NavItem, page string) []NavItem {
	group := defaultGroup(page)
	pageItem := NavItem{ID: page, Type: "page", Page: page}
	if group == nil {
		return append(items, pageItem)
	}
	for i, item := range items {
		if item.Type == "group" && item.ID == group.ID {
			item.Children = append(item.Children, pageItem)
			items[i] = item
			return items
		}
	}
	return append(items, NavItem{ID: group.ID, Type: "group", Label: group.Label, Children: []NavItem{pageItem}})
}

func defaultGroup(page string) *NavItem {
	for _, item := range DefaultNavigation().Items {
		if item.Type != "group" {
			continue
		}
		for _, child := range item.Children {
			if child.Page == page {
				copy := item
				return &copy
			}
		}
	}
	return nil
}

func knownPage(page string) bool {
	for _, item := range pageCatalog {
		if item == page {
			return true
		}
	}
	return false
}

func NormalizeTheme(theme Theme) (Theme, error) {
	switch theme.Preset {
	case "navy":
		base := NavyTheme()
		if theme.Mode == "light" || theme.Mode == "dark" || theme.Mode == "system" {
			base.Mode = theme.Mode
		}
		return base, nil
	case "", "default":
		if theme.Preset == "" && theme.Primary != "" {
			theme.Preset = "custom"
		} else if theme.Preset == "" {
			return DefaultTheme(), nil
		}
		if theme.Preset == "default" {
			base := DefaultTheme()
			if theme.Mode == "light" || theme.Mode == "dark" || theme.Mode == "system" {
				base.Mode = theme.Mode
			}
			return base, nil
		}
	case "custom":
	default:
		return Theme{}, apierr.Invalid("unknown theme preset")
	}
	if theme.Mode != "light" && theme.Mode != "dark" && theme.Mode != "system" {
		return Theme{}, apierr.Invalid("theme mode must be light, dark, or system")
	}
	for _, color := range []string{theme.Primary, theme.Accent, theme.Sidebar, theme.Surface, theme.Border, theme.Text, theme.BG} {
		if !hexColor(color) {
			return Theme{}, apierr.Invalid("colors must be #rrggbb")
		}
	}
	if contrast(theme.Text, theme.Surface) < 4.5 || contrast(theme.Text, theme.Sidebar) < 4.5 || contrast(theme.Text, theme.BG) < 4.5 {
		return Theme{}, apierr.Invalid("text and surface colors do not meet readable contrast")
	}
	theme.Preset = "custom"
	return theme, nil
}

func hexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, r := range value[1:] {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func contrast(a, b string) float64 {
	l1 := luminance(a)
	l2 := luminance(b)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func luminance(hex string) float64 {
	r := channel(hex[1:3])
	g := channel(hex[3:5])
	b := channel(hex[5:7])
	return 0.2126*r + 0.7152*g + 0.0722*b
}

func channel(part string) float64 {
	var n int
	for _, r := range part {
		n *= 16
		switch {
		case r >= '0' && r <= '9':
			n += int(r - '0')
		case r >= 'a' && r <= 'f':
			n += int(r-'a') + 10
		default:
			n += int(r-'A') + 10
		}
	}
	c := float64(n) / 255
	if c <= 0.03928 {
		return c / 12.92
	}
	v := (c + 0.055) / 1.055
	return v * v * v
}
