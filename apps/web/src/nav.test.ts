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
  it("includes the command room, topology, and incidents for every reader", () => {
    const pages = defaultNavigation().flatMap((item) => item.children ?? []).map((item) => item.page);
    expect(pages).toEqual(expect.arrayContaining(["command", "topology", "incidents"]));
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
