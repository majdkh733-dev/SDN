import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api";
import { ReferenceDiagram } from "./reference-topology";
import { Badge, Empty, QueryState, formatTime, useT } from "../ui";

type Summary = {
  organization: string;
  sites: number;
  devices: number;
  devices_online: number;
  devices_offline: number;
  devices_unknown: number;
  devices_stale: number;
  devices_simulated: number;
  open_drift: number;
  active_changes: number;
  failed_changes: number;
  connectors_healthy: number;
  connectors_total: number;
  discovery_active: number;
  last_collection?: string;
  notice: string;
  workloads_note: string;
  availability_note: string;
  telemetry_note: string;
  alerts: number;
  site_id?: string;
};

type Incident = { id: string; severity: string; title: string; site: string; at?: string; href: string; detail: string };
type Change = { id: string; status: string; created_at?: string; impact?: { items?: { device_id: string; device_name?: string }[] } };
type SearchHit = { devices: { id: string; name: string }[]; sites: { id: string; name: string }[]; segments: { id: string; name: string }[]; policies: { id: string; name: string }[] };

type Site = { id: string; name: string; location: string; latitude?: number; longitude?: number };

export function CommandPage() {
  const t = useT();
  const [query, setQuery] = useState("");
  const [site, setSite] = useState("");
  const [alertsOpen, setAlertsOpen] = useState(true);
  const [lens, setLens] = useState<"diagram" | "geo">("diagram");
  const sites = useQuery({ queryKey: ["sites"], queryFn: () => api<Site[]>("/api/v1/sites") });
  const summary = useQuery({
    queryKey: ["summary", site],
    queryFn: () => api<Summary>(`/api/v1/command-room/summary?site_id=${encodeURIComponent(site)}`),
    refetchInterval: 15000,
  });
  const incidents = useQuery({ queryKey: ["incidents"], queryFn: () => api<Incident[]>("/api/v1/command-room/incidents"), refetchInterval: 15000 });
  const changes = useQuery({ queryKey: ["changes", "recent"], queryFn: () => api<Change[]>("/api/v1/changes") });
  const hits = useQuery({
    queryKey: ["search", query],
    queryFn: () => api<SearchHit>(`/api/v1/search?q=${encodeURIComponent(query)}`),
    enabled: query.trim().length > 1,
  });
  const siteName = sites.data?.find((item) => item.id === site)?.name || "";
  const visibleIncidents = (incidents.data ?? []).filter((item) => !siteName || item.site === siteName);
  return (
    <div className="command-room">
      <header className="command-top">
        <div>
          <p className="eyebrow">SDN</p>
          <h1>{t("command")}</h1>
        </div>
        <label className="search">
          <span className="sr">{t("search")}</span>
          <input value={query} placeholder={t("search")} onChange={(event) => setQuery(event.target.value)} />
          {hits.data ? (
            <div className="search-results" role="listbox">
              {hits.data.devices.map((item) => <Link key={item.id} to={`/inventory/devices/${item.id}`}>{item.name}</Link>)}
              {hits.data.sites.map((item) => <Link key={item.id} to={`/sites`}>{item.name}</Link>)}
              {hits.data.policies.map((item) => <Link key={item.id} to="/policies">{item.name}</Link>)}
              {hits.data.segments.map((item) => <span key={item.id}>{item.name}</span>)}
            </div>
          ) : null}
        </label>
        <label className="field">{t("site")}
          <select value={site} onChange={(event) => setSite(event.target.value)}>
            <option value="">{t("allSites")}</option>
            {(sites.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
          </select>
        </label>
        <button type="button" className="secondary alerts-link" aria-expanded={alertsOpen} onClick={() => setAlertsOpen((open) => !open)}>{t("alerts")} {summary.data?.alerts ?? ""}</button>
      </header>
      <QueryState loading={summary.isLoading} error={summary.error} onRetry={() => summary.refetch()}>
        {summary.data ? (
          <>
            <p className="banner">{summary.data.notice}</p>
            <div className="kpi">
              <article className="card"><strong>{summary.data.devices_online}</strong><span>{t("online")}</span></article>
              <article className="card"><strong>{summary.data.devices_offline}</strong><span>{t("offline")}</span></article>
              <article className="card"><strong>{summary.data.devices_unknown}</strong><span>{t("unknown")}</span></article>
              <article className="card"><strong>{summary.data.devices_stale}</strong><span>{t("telemetryStale")}</span></article>
              <article className="card"><strong>{summary.data.devices_simulated}</strong><span>{t("simulated")}</span><p className="note">{t("simulatorSeparate")}</p></article>
              <article className="card"><strong>{summary.data.connectors_healthy}/{summary.data.connectors_total}</strong><span>{t("connectors")}</span><p className="note">{summary.data.last_collection ? formatTime(summary.data.last_collection) : t("noCollection")}</p></article>
              <article className="card"><strong>{summary.data.discovery_active}</strong><span>{t("discovery")}</span></article>
              <article className="card"><strong>{summary.data.open_drift}</strong><span>{t("drift")}</span></article>
              <article className="card"><strong>{summary.data.failed_changes}</strong><span>{t("failedChanges")}</span><p className="note">{t("orgWide")}</p></article>
            </div>
            <p className="note">{summary.data.telemetry_note} {summary.data.workloads_note} {summary.data.availability_note}</p>
          </>
        ) : null}
      </QueryState>
      <div className="command-grid">
        <section className="map-panel">
          <div className="page-title-row">
            <h2>{t("topology")}</h2>
            <div className="lenses" role="group" aria-label={t("topology")}>
              <button type="button" className="secondary" aria-pressed={lens === "diagram"} onClick={() => setLens("diagram")}>{t("referenceTopology")}</button>
              <button type="button" className="secondary" aria-pressed={lens === "geo"} onClick={() => setLens("geo")}>{t("geo")}</button>
            </div>
          </div>
          {lens === "diagram" ? <ReferenceDiagram embedded /> : <GeographicState query={sites} />}
        </section>
        <aside className="panel rail">
          <h2 id="incidents">{t("incidents")}</h2>
          {alertsOpen ? <QueryState loading={incidents.isLoading} error={incidents.error} onRetry={() => incidents.refetch()}>
            {visibleIncidents.length === 0 ? <Empty text={t("empty")} /> : (
              <ul className="incident-list">
                {visibleIncidents.map((item) => (
                  <li key={item.id}>
                    <Badge value={item.severity} />
                    <Link to={item.href}>{item.title}</Link>
                    <p className="note">{item.site} {item.at ? formatTime(item.at) : ""}</p>
                    <p className="note">{item.detail}</p>
                  </li>
                ))}
              </ul>
            )}
          </QueryState> : <p className="note">{t("alertsCollapsed")}</p>}
          <p className="note">{t("diagramDetails")}</p>
        </aside>
      </div>
      <section className="panel">
        <div className="page-title-row">
          <h2>{t("recent")}</h2>
          <Link to="/changes">{t("changes")}</Link>
        </div>
        <ol className="timeline">
          {(changes.data ?? []).slice(0, 8).map((item) => (
            <li key={item.id}><Link to={`/changes/${item.id}`}>{item.status}</Link> <span className="note">{formatTime(item.created_at)}</span></li>
          ))}
        </ol>
        {(changes.data ?? []).length === 0 ? <Empty text={t("empty")} /> : null}
      </section>
    </div>
  );
}

function GeographicState({ query }: { query: { isLoading: boolean; error: unknown; data?: Site[]; refetch: () => unknown } }) {
  const t = useT();
  const placed = (query.data ?? []).filter((site) => site.latitude != null && site.longitude != null);
  return (
    <section className="panel" aria-label={t("geo")}>
      <QueryState loading={query.isLoading} error={query.error} onRetry={() => void query.refetch()}>
        {placed.length === 0 ? (
          <>
            <Empty text={t("geoUnavailable")} />
            <p className="note">{t("geoUnavailableNote")}</p>
          </>
        ) : (
          <ul className="incident-list">
            {placed.map((site) => (
              <li key={site.id}>
                <strong>{site.name}</strong>
                <p className="note">{site.location || t("unknown")} · {site.latitude}, {site.longitude}</p>
              </li>
            ))}
          </ul>
        )}
      </QueryState>
      <p className="note">{t("telemetryUnavailable")}</p>
    </section>
  );
}
