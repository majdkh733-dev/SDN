import { describe, expect, it } from "vitest";
import { defaultNavigation, dissolveGroup, dropPage, movePage, reparent, type NavItem } from "./nav";

const tree: NavItem[] = [
  { id: "command", type: "page", page: "command" },
  { id: "net", type: "group", label: "Network", children: [
    { id: "topology", type: "page", page: "topology" },
    { id: "inventory", type: "page", page: "inventory" },
  ] },
];

describe("default navigation", () => {
  it("uses the enterprise groups and keeps topology beside the command room", () => {
    const groups = defaultNavigation();
    expect(groups.map((item) => item.label)).toEqual([
      "Dashboard",
      "Network",
      "Monitoring",
      "Configuration",
      "Policies",
      "Administration",
      "Reports",
    ]);
    const pages = groups.flatMap((item) => item.children ?? []).map((item) => item.page);
    expect(pages).toEqual([
      "command",
      "sites", "inventory", "topology", "discovery", "interfaces",
      "incidents", "telemetry", "diagnostics",
      "backups", "templates", "changes", "drift",
      "policies", "groups", "vlans", "subnets", "routing", "wan", "tenants",
      "connectors", "integrations", "credentials", "audit", "controllers", "settings",
      "reports",
    ]);
    expect(pages).not.toContain("firmware");
    expect(pages).not.toContain("qos");
  });
});

describe("navigation edits", () => {
  it("reorders, reparents, and keeps pages when a group is removed", () => {
    const moved = movePage(tree, "inventory", -1);
    expect(moved[1].children?.map((item) => item.page)).toEqual(["inventory", "topology"]);
    const top = reparent(tree, "inventory", "");
    expect(top.map((item) => item.page || item.id)).toContain("inventory");
    expect(top[1].children).toHaveLength(1);
    const flat = dissolveGroup(tree, "net");
    expect(flat.map((item) => item.page)).toEqual(["command", "topology", "inventory"]);
  });

  it("drops a page before another page or into a group", () => {
    const into = dropPage(tree, "command", "net", "into");
    expect(into[0].children?.map((item) => item.page)).toEqual(["topology", "inventory", "command"]);
    const before = dropPage(tree, "inventory", "topology", "before");
    expect(before[1].children?.map((item) => item.page)).toEqual(["inventory", "topology"]);
    expect(dropPage(tree, "command", "command", "before")).toEqual(tree);
  });
});
