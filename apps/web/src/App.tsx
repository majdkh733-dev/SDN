import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { Outlet, Route, Routes, useNavigate } from "react-router-dom";
import { api, type Principal } from "./api";
import { ChangePage, ChangesPage } from "./pages/changes";
import { CommandPage } from "./pages/command";
import { ConnectorsPage, ConnectorPage } from "./pages/connectors";
import { DiscoveryPage } from "./pages/discovery";
import { DevicePage, InventoryPage } from "./pages/inventory";
import { CallbackPage, LoginPage } from "./pages/login";
import { AssurancePage, AuditPage, ControllersPage, DesiredPage, FabricPage, IntegrationsPage, SettingsPage } from "./pages/operate";
import { PoliciesPage } from "./pages/policies";
import { SitesPage } from "./pages/sites";
import { useUI } from "./session";
import { SidebarNav } from "./sidebar";
import { applyTheme, type Theme } from "./theme";
import { useDirection, useT } from "./ui";

function Shell() {
  const t = useT();
  const nav = useNavigate();
  const qc = useQueryClient();
  const menuOpen = useUI((s) => s.menuOpen);
  const setMenuOpen = useUI((s) => s.setMenuOpen);
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<Principal>("/api/v1/auth/me"), retry: false });
  const prefs = useQuery({
    queryKey: ["preferences"],
    queryFn: () => api<{ theme: Theme }>("/api/v1/preferences"),
    enabled: Boolean(me.data),
  });
  useDirection();
  useEffect(() => {
    if (prefs.data?.theme) applyTheme(prefs.data.theme);
  }, [prefs.data]);
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
        <SidebarNav principal={principal} onNavigate={() => setMenuOpen(false)} />
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
        <Route path="/topology" element={<CommandPage initialView="network" />} />
        <Route path="/sites" element={<SitesPage />} />
        <Route path="/connectors" element={<ConnectorsPage />} />
        <Route path="/connectors/:id" element={<ConnectorPage />} />
        <Route path="/discovery" element={<DiscoveryPage />} />
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
