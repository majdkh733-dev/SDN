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
  open_drift: number;
  active_changes: number;
  connectors_healthy: number;
  connectors_total: number;
  notice: string;
  workloads_available: boolean;
  workloads_note: string;
  availability_available: boolean;
  availability_note: string;
  alerts: number;
};

type Incident = { id: string; severity: string; title: string; site: string; at?: string; href: string; detail: string };
type Change = { id: string; status: string; created_at?: string; impact?: { items?: { device_id: string; device_name?: string }[] } };
type SearchHit = { devices: { id: string; name: string }[]; sites: { id: string; name: string }[]; segments: { id: string; name: string }[]; policies: { id: string; name: string }[] };

export function CommandPage() {
  const t = useT();
  const [query, setQuery] = useState("");
  const summary = useQuery({ queryKey: ["summary"], queryFn: () => api<Summary>("/api/v1/command-room/summary"), refetchInterval: 15000 });
  const incidents = useQuery({ queryKey: ["incidents"], queryFn: () => api<Incident[]>("/api/v1/command-room/incidents"), refetchInterval: 15000 });
  const changes = useQuery({ queryKey: ["changes", "recent"], queryFn: () => api<Change[]>("/api/v1/changes") });
  const hits = useQuery({
    queryKey: ["search", query],
    queryFn: () => api<SearchHit>(`/api/v1/search?q=${encodeURIComponent(query)}`),
    enabled: query.trim().length > 1,
  });
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
        <a className="secondary alerts-link" href="#incidents">{t("alerts")} {summary.data?.alerts ?? ""}</a>
      </header>
      <QueryState loading={summary.isLoading} error={summary.error} onRetry={() => summary.refetch()}>
        {summary.data ? (
          <>
            <p className="banner">{summary.data.notice}</p>
            <div className="kpi">
              <article className="card"><strong>{summary.data.devices}</strong><span>{t("devices")}</span></article>
              <article className="card"><strong>{t("unavailable")}</strong><span>{t("workloads")}</span><p className="note">{summary.data.workloads_note}</p></article>
              <article className="card"><strong>{t("unavailable")}</strong><span>{t("availability")}</span><p className="note">{summary.data.availability_note}</p></article>
              <article className="card"><strong>{summary.data.alerts}</strong><span>{t("alerts")}</span></article>
            </div>
          </>
        ) : null}
      </QueryState>
      <div className="command-grid">
        <section className="map-panel">
          <ReferenceDiagram embedded />
        </section>
        <aside className="panel rail">
          <h2 id="incidents">{t("incidents")}</h2>
          <QueryState loading={incidents.isLoading} error={incidents.error} onRetry={() => incidents.refetch()}>
            {(incidents.data ?? []).length === 0 ? <Empty text={t("empty")} /> : (
              <ul className="incident-list">
                {(incidents.data ?? []).map((item) => (
                  <li key={item.id}>
                    <Badge value={item.severity} />
                    <Link to={item.href}>{item.title}</Link>
                    <p className="note">{item.site} {item.at ? formatTime(item.at) : ""}</p>
                    <p className="note">{item.detail}</p>
                  </li>
                ))}
              </ul>
            )}
          </QueryState>
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
