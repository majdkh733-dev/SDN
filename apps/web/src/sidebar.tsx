import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type DragEvent } from "react";
import { NavLink } from "react-router-dom";
import { api, can, type Principal } from "./api";
import { addGroup, defaultNavigation, dissolveGroup, dropPage, movePage, pageRoutes, renameGroup, reparent, type NavItem } from "./nav";
import { message, useT } from "./ui";

type Preferences = { navigation: { items: NavItem[] }; theme: { mode: string; preset: string } };

export function SidebarNav({ principal, onNavigate }: { principal: Principal; onNavigate: () => void }) {
  const t = useT();
  const qc = useQueryClient();
  const prefs = useQuery({ queryKey: ["preferences"], queryFn: () => api<Preferences>("/api/v1/preferences") });
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState<NavItem[] | null>(null);
  const [label, setLabel] = useState("");
  const [drop, setDrop] = useState<{ id: string; where: "before" | "after" | "into" } | null>(null);
  const usingDefault = !draft && !prefs.data && prefs.isError;
  const items = draft ?? prefs.data?.navigation.items ?? (usingDefault ? defaultNavigation() : []);
  const save = useMutation({
    mutationFn: (next: NavItem[]) => api("/api/v1/preferences/navigation", { method: "POST", body: JSON.stringify({ items: next }) }),
    onSuccess: async () => {
      setDraft(null);
      setEditing(false);
      await qc.invalidateQueries({ queryKey: ["preferences"] });
    },
  });
  const reset = useMutation({
    mutationFn: () => api("/api/v1/preferences/navigation/reset", { method: "POST", body: "{}" }),
    onSuccess: async () => {
      setDraft(null);
      setEditing(false);
      await qc.invalidateQueries({ queryKey: ["preferences"] });
    },
  });
  const admin = can(principal.role, "admin");
  function update(next: NavItem[]) {
    setDraft(next);
  }
  function onDragStart(event: DragEvent, page: string) {
    event.dataTransfer.setData("text/plain", page);
    event.dataTransfer.effectAllowed = "move";
  }
  function onDragOver(event: DragEvent, id: string, where: "before" | "after" | "into") {
    if (!editing) return;
    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
    setDrop({ id, where });
  }
  function onDrop(event: DragEvent, id: string, where: "before" | "after" | "into") {
    event.preventDefault();
    const page = event.dataTransfer.getData("text/plain");
    setDrop(null);
    if (page) update(dropPage(items, page, id, where));
  }
  function renderItem(item: NavItem, depth: number) {
    if (item.type === "group") {
      return (
        <details key={item.id} className={drop?.id === item.id && drop.where === "into" ? "nav-group drop-into" : "nav-group"} open
          onDragOver={(event) => onDragOver(event, item.id, "into")}
          onDrop={(event) => onDrop(event, item.id, "into")}>
          <summary>
            {editing ? (
              <input aria-label={t("groupLabel")} value={item.label || ""} onChange={(event) => update(renameGroup(items, item.id, event.target.value))} />
            ) : groupTitle(t, item.id, item.label || "")}
          </summary>
          <div className="nav-children">
            {(item.children ?? []).map((child) => renderItem(child, depth + 1))}
            {editing ? (
              <button type="button" className="secondary" onClick={() => update(dissolveGroup(items, item.id))}>{t("dissolve")}</button>
            ) : null}
          </div>
        </details>
      );
    }
    const route = pageRoutes[item.page || ""];
    if (!route) return null;
    const marker = drop?.id === item.id ? ` drop-${drop.where}` : "";
    return (
      <div key={item.id} className={`nav-row${marker}`} style={{ paddingInlineStart: depth ? 8 : 0 }}
        draggable={editing}
        onDragStart={(event) => onDragStart(event, item.page || "")}
        onDragOver={(event) => onDragOver(event, item.id, event.nativeEvent.offsetY < 14 ? "before" : "after")}
        onDrop={(event) => onDrop(event, item.id, event.nativeEvent.offsetY < 14 ? "before" : "after")}
        onDragEnd={() => setDrop(null)}>
        <NavLink to={route.to} end={route.end} className={({ isActive }) => (isActive ? "active" : undefined)} onClick={onNavigate}>
          <NavMark page={item.page || ""} />
          {t(item.page || item.id)}
        </NavLink>
        {editing ? (
          <span className="nav-moves">
            <button type="button" className="secondary" aria-label={t("moveUp")} onClick={() => update(movePage(items, item.page || "", -1))}>↑</button>
            <button type="button" className="secondary" aria-label={t("moveDown")} onClick={() => update(movePage(items, item.page || "", 1))}>↓</button>
            <select aria-label={t("navigation")} value="" onChange={(event) => update(reparent(items, item.page || "", event.target.value))}>
              <option value="" disabled>{t("navigation")}</option>
              <option value="">{t("home")}</option>
              {items.filter((entry) => entry.type === "group").map((entry) => (
                <option key={entry.id} value={entry.id}>{entry.label}</option>
              ))}
            </select>
          </span>
        ) : null}
      </div>
    );
  }
  return (
    <nav className="nav" aria-label={t("product")}>
      {usingDefault ? <p className="note">{t("navFallback")}</p> : null}
      {items.map((item) => renderItem(item, 0))}
      {admin ? (
        <div className="nav-edit">
          {editing ? (
            <>
              <form className="stack" onSubmit={(event) => { event.preventDefault(); if (label.trim()) { update(addGroup(items, label.trim())); setLabel(""); } }}>
                <input aria-label={t("groupLabel")} value={label} onChange={(event) => setLabel(event.target.value)} placeholder={t("newGroup")} />
                <button type="submit" className="secondary">{t("newGroup")}</button>
              </form>
              <button type="button" onClick={() => draft && save.mutate(draft)} disabled={!draft || save.isPending}>{t("save")}</button>
              <button type="button" className="secondary" onClick={() => { setDraft(null); setEditing(false); }}>{t("cancel")}</button>
              <button type="button" className="secondary" onClick={() => reset.mutate()}>{t("reset")}</button>
              {save.error || reset.error ? <p className="state error" role="alert">{message(save.error || reset.error, t("requestFailed"))}</p> : null}
            </>
          ) : (
            <button type="button" className="secondary" onClick={() => setEditing(true)}>{t("editNav")}</button>
          )}
        </div>
      ) : null}
    </nav>
  );
}

function NavMark({ page }: { page: string }) {
  return <span className={`nav-mark nav-mark-${page}`} aria-hidden="true" />;
}

function groupTitle(t: (key: string) => string, id: string, label: string) {
  const key = "group." + id;
  const translated = t(key);
  return translated === key ? label : translated;
}
