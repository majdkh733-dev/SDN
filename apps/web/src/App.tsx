import { useQuery, useQueryClient } from "@tanstack/react-query";
import { NavLink, Outlet, Route, Routes, useNavigate } from "react-router-dom";
import { api, type Principal } from "./api";
import { ChangePage, ChangesPage } from "./pages/changes";
import { CommandPage } from "./pages/command";
import { DevicePage, InventoryPage } from "./pages/inventory";
import { CallbackPage, LoginPage } from "./pages/login";
import { AssurancePage, AuditPage, ControllersPage, DesiredPage, FabricPage, IntegrationsPage, SettingsPage } from "./pages/operate";
import { PoliciesPage } from "./pages/policies";
import { useUI } from "./session";
import { useDirection, useT } from "./ui";

const links = [
  { to: "/", key: "command", end: true },
  { to: "/policies", key: "policies", end: false },
  { to: "/fabric", key: "fabric", end: false },
  { to: "/controllers", key: "controllers", end: false },
  { to: "/changes", key: "changes", end: false },
  { to: "/desired", key: "desired", end: false },
  { to: "/assurance", key: "assurance", end: false },
  { to: "/inventory", key: "inventory", end: false },
  { to: "/integrations", key: "integrations", end: false },
  { to: "/audit", key: "audit", end: false },
  { to: "/settings", key: "settings", end: false },
];

function Shell() {
  const t = useT();
  const nav = useNavigate();
  const qc = useQueryClient();
  const menuOpen = useUI((s) => s.menuOpen);
  const setMenuOpen = useUI((s) => s.setMenuOpen);
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<Principal>("/api/v1/auth/me"), retry: false });
  useDirection();
  if (me.isLoading) return <p className="state" role="status">{t("loading")}</p>;
  if (me.isError || !me.data) return <LoginPage />;
  const principal = me.data;
  async function logout() {
    await api("/api/v1/auth/logout", { method: "POST", body: "{}" });
    await qc.invalidateQueries({ queryKey: ["me"] });
    nav("/login");
  }
  return (
    <>
      <a className="skip" href="#content">{t("skip")}</a>
      <aside className={menuOpen ? "sidebar open" : "sidebar"}>
        <div className="brand"><strong>{t("product")}</strong><span>{t("tagline")}</span></div>
        <nav className="nav" aria-label={t("product")}>
          {links.map((link) => (
            <NavLink key={link.to} to={link.to} end={link.end} className={({ isActive }) => (isActive ? "active" : undefined)} onClick={() => setMenuOpen(false)}>
              {t(link.key)}
            </NavLink>
          ))}
        </nav>
        <div className="side-user">
          <p>{principal.display_name}</p>
          <p className="muted">{principal.organization.organization_name}</p>
          <p className="muted">{principal.role}</p>
          <button type="button" className="secondary" onClick={() => void logout()}>{t("signOut")}</button>
        </div>
      </aside>
      <div className={menuOpen ? "scrim show" : "scrim"} onClick={() => setMenuOpen(false)} />
      <main className="main" id="content">
        <button type="button" className="menu-btn secondary" aria-expanded={menuOpen} onClick={() => setMenuOpen(!menuOpen)}>{menuOpen ? t("close") : t("menu")}</button>
        <Outlet context={{ principal }} />
      </main>
    </>
  );
}

export function App() {
  useDirection();
  return (
    <Routes>
      <Route path="/callback" element={<CallbackPage />} />
      <Route path="/login" element={<LoginPage />} />
      <Route element={<Shell />}>
        <Route path="/" element={<CommandPage />} />
        <Route path="/policies" element={<PoliciesPage />} />
        <Route path="/fabric" element={<FabricPage />} />
        <Route path="/controllers" element={<ControllersPage />} />
        <Route path="/changes" element={<ChangesPage />} />
        <Route path="/changes/:id" element={<ChangePage />} />
        <Route path="/desired" element={<DesiredPage />} />
        <Route path="/assurance" element={<AssurancePage />} />
        <Route path="/inventory" element={<InventoryPage />} />
        <Route path="/inventory/devices/:id" element={<DevicePage />} />
        <Route path="/integrations" element={<IntegrationsPage />} />
        <Route path="/audit" element={<AuditPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="*" element={<CommandPage />} />
      </Route>
    </Routes>
  );
}
