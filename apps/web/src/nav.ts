export type NavItem = {
  id: string;
  type: "page" | "group";
  page?: string;
  label?: string;
  children?: NavItem[];
};

export const pageRoutes: Record<string, { to: string; end: boolean }> = {
  command: { to: "/", end: true },
  sites: { to: "/sites", end: false },
  inventory: { to: "/inventory", end: false },
  interfaces: { to: "/interfaces", end: false },
  endpoints: { to: "/endpoints", end: false },
  topology: { to: "/topology", end: false },
  discovery: { to: "/discovery", end: false },
  vlans: { to: "/vlans", end: false },
  tenants: { to: "/tenants", end: false },
  subnets: { to: "/subnets", end: false },
  routing: { to: "/routing", end: false },
  fabric: { to: "/fabric", end: false },
  wan: { to: "/wan", end: false },
  policies: { to: "/policies", end: false },
  groups: { to: "/groups", end: false },
  qos: { to: "/qos", end: false },
  validation: { to: "/validation", end: false },
  templates: { to: "/templates", end: false },
  provisioning: { to: "/provisioning", end: false },
  backups: { to: "/backups", end: false },
  drift: { to: "/drift", end: false },
  changes: { to: "/changes", end: false },
  firmware: { to: "/firmware", end: false },
  assurance: { to: "/assurance", end: false },
  incidents: { to: "/incidents", end: false },
  telemetry: { to: "/telemetry", end: false },
  diagnostics: { to: "/diagnostics", end: false },
  reports: { to: "/reports", end: false },
  connectors: { to: "/connectors", end: false },
  integrations: { to: "/integrations", end: false },
  credentials: { to: "/credentials", end: false },
  audit: { to: "/audit", end: false },
  controllers: { to: "/controllers", end: false },
  settings: { to: "/settings", end: false },
};

function clone(items: NavItem[]): NavItem[] {
  return items.map((item) => ({ ...item, children: item.children ? clone(item.children) : undefined }));
}

function removePage(items: NavItem[], page: string): { items: NavItem[]; removed?: NavItem } {
  const next: NavItem[] = [];
  let removed: NavItem | undefined;
  for (const item of items) {
    if (item.type === "page" && item.page === page) {
      removed = item;
      continue;
    }
    if (item.type === "group" && item.children) {
      const inner = removePage(item.children, page);
      if (inner.removed) removed = inner.removed;
      next.push({ ...item, children: inner.items });
      continue;
    }
    next.push(item);
  }
  return { items: next, removed };
}

export function movePage(items: NavItem[], page: string, direction: -1 | 1): NavItem[] {
  const walk = (list: NavItem[]): NavItem[] => {
    const index = list.findIndex((item) => item.type === "page" && item.page === page);
    if (index >= 0) {
      const target = index + direction;
      if (target < 0 || target >= list.length) return list;
      const copy = list.slice();
      const [item] = copy.splice(index, 1);
      copy.splice(target, 0, item);
      return copy;
    }
    return list.map((item) => item.children ? { ...item, children: walk(item.children) } : item);
  };
  return walk(clone(items));
}

export function reparent(items: NavItem[], page: string, groupId: string): NavItem[] {
  const tree = clone(items);
  const found = removePage(tree, page);
  if (!found.removed) return items;
  const pageItem = found.removed;
  if (groupId === "") {
    return [...found.items, pageItem];
  }
  const place = (list: NavItem[]): NavItem[] => list.map((item) => {
    if (item.type === "group" && item.id === groupId) {
      return { ...item, children: [...(item.children ?? []), pageItem] };
    }
    return item;
  });
  return place(found.items);
}

export function dissolveGroup(items: NavItem[], groupId: string): NavItem[] {
  const next: NavItem[] = [];
  for (const item of items) {
    if (item.type === "group" && item.id === groupId) {
      next.push(...(item.children ?? []));
      continue;
    }
    next.push(item);
  }
  return next;
}

export function renameGroup(items: NavItem[], groupId: string, label: string): NavItem[] {
  return items.map((item) => item.id === groupId ? { ...item, label } : item);
}

export function addGroup(items: NavItem[], label: string): NavItem[] {
  const id = "group-" + Math.random().toString(36).slice(2, 8);
  return [...items, { id, type: "group", label, children: [] }];
}

export function dropPage(items: NavItem[], page: string, targetId: string, where: "before" | "after" | "into"): NavItem[] {
  if (!page || !targetId || page === targetId) return items;
  const tree = clone(items);
  const found = removePage(tree, page);
  if (!found.removed) return items;
  const pageItem = found.removed;
  if (where === "into") {
    let placed = false;
    const place = (list: NavItem[]): NavItem[] => list.map((item) => {
      if (item.type === "group" && item.id === targetId) {
        placed = true;
        return { ...item, children: [...(item.children ?? []), pageItem] };
      }
      return item;
    });
    const next = place(found.items);
    return placed ? next : items;
  }
  const insert = (list: NavItem[]): { list: NavItem[]; hit: boolean } => {
    const index = list.findIndex((item) => item.id === targetId);
    if (index >= 0) {
      const copy = list.slice();
      copy.splice(where === "before" ? index : index + 1, 0, pageItem);
      return { list: copy, hit: true };
    }
    let hit = false;
    const next = list.map((item) => {
      if (!item.children) return item;
      const inner = insert(item.children);
      if (inner.hit) hit = true;
      return { ...item, children: inner.list };
    });
    return { list: next, hit };
  };
  const result = insert(found.items);
  return result.hit ? result.list : items;
}
