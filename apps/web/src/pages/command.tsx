import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../api";
import { TopologyGraph, type GraphHandle, type TopoNode } from "../topology";
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

type Topology = {
  lens: string;
  notice: string;
  nodes: TopoNode[];
  edges: { id: string; source: string; target: string; utilization_pct: number; label: string; origin?: string; measured?: boolean; latency_ms?: number }[];
};

type Incident = { id: string; severity: string; title: string; site: string; at?: string; href: string; detail: string };
type Change = { id: string; status: string; created_at?: string; impact?: { items?: { device_id: string; device_name?: string }[] } };
type Audit = { id: string; action: string; resource_type: string; created_at: string };
type SearchHit = { devices: { id: string; name: string }[]; sites: { id: string; name: string }[]; segments: { id: string; name: string }[]; policies: { id: string; name: string }[] };
type DeviceDetail = {
  device: { id: string; name: string; model: string; role: string; health: string; site_id: string; management_endpoint: string; os_version: string; vendor?: string; data_origin: string; support_note: string; hostname?: string };
  interfaces: { id: string; name: string; oper_up: boolean; mac: string }[];
  capabilities: { capability: string; supported: boolean; evidence: string }[];
};

export function CommandPage({ initialView = "sites" }: { initialView?: string }) {
  const t = useT();
  const [params, setParams] = useSearchParams();
  const site = params.get("site") || "";
  const selected = params.get("node") || "";
  const view = params.get("view") || initialView;
  const [query, setQuery] = useState("");
  const [tab, setTab] = useState("overview");
  const graph = useRef<GraphHandle>(null);
  const mapPanel = useRef<HTMLElement>(null);
  const summary = useQuery({ queryKey: ["summary"], queryFn: () => api<Summary>("/api/v1/command-room/summary"), refetchInterval: 15000 });
  const sites = useQuery({ queryKey: ["sites"], queryFn: () => api<{ id: string; name: string; location: string }[]>("/api/v1/sites") });
  const topology = useQuery({
    queryKey: ["topology", site, view],
    queryFn: () => api<Topology>(`/api/v1/topology?lens=performance&site_id=${encodeURIComponent(view === "network" ? site : "")}`),
    refetchInterval: 15000,
  });
  const incidents = useQuery({ queryKey: ["incidents"], queryFn: () => api<Incident[]>("/api/v1/command-room/incidents"), refetchInterval: 15000 });
  const changes = useQuery({ queryKey: ["changes", "recent"], queryFn: () => api<Change[]>("/api/v1/changes") });
  const audit = useQuery({ queryKey: ["audit", "recent"], queryFn: () => api<Audit[]>("/api/v1/audit-events") });
  const hits = useQuery({
    queryKey: ["search", query],
    queryFn: () => api<SearchHit>(`/api/v1/search?q=${encodeURIComponent(query)}`),
    enabled: query.trim().length > 1,
  });
  const deviceId = selected.startsWith("site:") ? "" : selected;
  const device = useQuery({
    queryKey: ["device", deviceId],
    queryFn: () => api<DeviceDetail>(`/api/v1/devices/${deviceId}`),
    enabled: Boolean(deviceId),
  });
  const node = topology.data?.nodes.find((item) => item.id === selected);
  const edge = topology.data?.edges.find((item) => item.id === selected);
  const visibleNodes = useMemo(() => {
    const nodes = topology.data?.nodes ?? [];
    if (view === "sites") return nodes.filter((item) => item.kind === "site");
    if (view === "geo") return nodes.filter((item) => item.kind === "site" && item.meta?.latitude != null);
    if (site) return nodes.filter((item) => item.kind !== "site" && item.site_id === site);
    return nodes.filter((item) => item.kind !== "site");
  }, [topology.data, view, site]);
  const visibleIds = new Set(visibleNodes.map((item) => item.id));
  const visibleEdges = (topology.data?.edges ?? []).filter((item) => visibleIds.has(item.source) && visibleIds.has(item.target));
  const geoReady = (topology.data?.nodes ?? []).some((item) => item.kind === "site" && item.meta?.latitude != null);

  useEffect(() => { setTab("overview"); }, [selected]);

  function setParam(key: string, value: string) {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value);
    else next.delete(key);
    setParams(next);
  }

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
              {hits.data.sites.map((item) => <button key={item.id} type="button" className="secondary" onClick={() => { setParam("site", item.id); setQuery(""); }}>{item.name}</button>)}
              {hits.data.policies.map((item) => <Link key={item.id} to="/policies">{item.name}</Link>)}
              {hits.data.segments.map((item) => <span key={item.id}>{item.name}</span>)}
            </div>
          ) : null}
        </label>
        <a className="secondary alerts-link" href="#incidents">{t("alerts")} {summary.data?.alerts ?? ""}</a>
        <label className="field">
          {t("site")}
          <select value={site} onChange={(event) => { setParam("site", event.target.value); setParam("view", event.target.value ? "network" : "sites"); setParam("node", ""); }}>
            <option value="">{t("allSites")}</option>
            {(sites.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
          </select>
        </label>
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
        <section className="panel map-panel" ref={mapPanel}>
          <div className="page-title-row">
            <h2>{t("topology")}</h2>
            <div className="lenses" role="group" aria-label={t("topology")}>
              {([["sites", t("sitesView")], ["network", t("networkView")], ["geo", t("geo")]] as const).map(([id, label]) => (
                <button key={id} type="button" className="secondary" aria-pressed={view === id} onClick={() => setParam("view", id)}>{label}</button>
              ))}
              <button type="button" className="secondary" onClick={() => graph.current?.zoomBy(1.2)}>{t("zoomIn")}</button>
              <button type="button" className="secondary" onClick={() => graph.current?.zoomBy(0.8)}>{t("zoomOut")}</button>
              <button type="button" className="secondary" onClick={() => graph.current?.fit()}>{t("fit")}</button>
              <button type="button" className="secondary" onClick={() => {
                if (document.fullscreenElement) void document.exitFullscreen();
                else void mapPanel.current?.requestFullscreen();
              }}>{t("fullscreen")}</button>
            </div>
          </div>
          <p className="note">{topology.data?.notice} {t("noticeTopology")}</p>
          <div className="legend" aria-label={t("legend")}>
            <span><i className="dot ok" />{t("healthy")}</span>
            <span><i className="dot bad" />{t("down")}</span>
            <span><i className="dot warn" />{t("telemetryStale")}</span>
            <span>{t("notMeasured")}</span>
          </div>
          <QueryState loading={topology.isLoading} error={topology.error} onRetry={() => topology.refetch()}>
            {view === "geo" && !geoReady ? <Empty text={t("unavailable")} /> : visibleNodes.length === 0 ? <Empty text={t("empty")} /> : (
              <TopologyGraph ref={graph} nodes={visibleNodes} edges={visibleEdges} selected={selected} highlighted={[]} onSelect={(id) => setParam("node", id)} />
            )}
          </QueryState>
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
          <h2>{t("details")}</h2>
          {!node && !edge ? <p className="note">{t("selectNode")}</p> : null}
          {edge ? (
            <dl className="stack">
              <div><dt className="note">{t("kind")}</dt><dd>{edge.origin || t("notMeasured")}</dd></div>
              <div><dt className="note">{t("status")}</dt><dd>{edge.measured && edge.latency_ms != null ? `${edge.latency_ms} ms` : t("notMeasured")}</dd></div>
            </dl>
          ) : null}
          {node?.kind === "site" ? (
            <dl className="stack">
              <div><dt className="note">{t("name")}</dt><dd>{node.label}</dd></div>
              <div><dt className="note">{t("location")}</dt><dd>{String(node.meta?.location || "—")}</dd></div>
              <Link to={`/policies?site=${node.site_id || ""}`}>{t("openPolicy")}</Link>
            </dl>
          ) : null}
          {device.data ? (
            <div>
              <div className="lenses" role="tablist">
                {["overview", "interfaces", "workloads", "policies", "activity"].map((item) => (
                  <button key={item} type="button" className="secondary" aria-selected={tab === item} onClick={() => setTab(item)}>{t(item === "policies" ? "policies" : item === "interfaces" ? "interfaces" : item === "workloads" ? "workloads" : item === "activity" ? "activity" : "overview")}</button>
                ))}
              </div>
              {tab === "overview" ? (
                <dl className="stack">
                  <div><dt className="note">{t("name")}</dt><dd>{device.data.device.name}</dd></div>
                  <div><dt className="note">{t("kind")}</dt><dd>{device.data.device.role}</dd></div>
                  <div><dt className="note">{t("health")}</dt><dd><Badge value={device.data.device.health} /></dd></div>
                  <div><dt className="note">{t("model")}</dt><dd>{device.data.device.model || "—"}</dd></div>
                  <div><dt className="note">{t("origin")}</dt><dd><Badge value={device.data.device.data_origin} /></dd></div>
                  <div><dt className="note">{t("freshness")}</dt><dd>{String(node?.meta?.telemetry || "unknown")}</dd></div>
                  <p className="note">{device.data.device.support_note}</p>
                  <Link to={`/inventory/devices/${device.data.device.id}`}>{t("openDevice")}</Link>
                  <Link to={`/policies?site=${device.data.device.site_id}&device=${device.data.device.id}`}>{t("openPolicy")}</Link>
                </dl>
              ) : null}
              {tab === "interfaces" ? (
                <ul>{device.data.interfaces.map((item) => <li key={item.id}>{item.name} · {item.mac || "—"} · {item.oper_up ? t("up") : t("down")}</li>)}</ul>
              ) : null}
              {tab === "workloads" ? <p className="note">{t("noWorkloads")}</p> : null}
              {tab === "policies" ? <Link to={`/policies?site=${device.data.device.site_id}&device=${device.data.device.id}`}>{t("openPolicy")}</Link> : null}
              {tab === "activity" ? (
                <ul>{(audit.data ?? []).slice(0, 6).map((item) => <li key={item.id}>{item.action} · {formatTime(item.created_at)}</li>)}</ul>
              ) : null}
              <h2>{t("capabilities")}</h2>
              <ul>
                {device.data.capabilities.map((item) => (
                  <li key={item.capability}><Badge value={item.supported ? t("yes") : t("no")} /> {item.capability}: {item.evidence}</li>
                ))}
              </ul>
              {device.data.capabilities.find((item) => item.capability === "policy" && !item.supported) ? (
                <p className="note">{device.data.capabilities.find((item) => item.capability === "policy")?.evidence}</p>
              ) : null}
            </div>
          ) : null}
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
