import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../api";
import { TopologyGraph, type TopoNode } from "../topology";
import { Badge, Empty, Page, QueryState, useT } from "../ui";

type Summary = {
  organization: string;
  sites: number;
  devices: number;
  open_drift: number;
  active_changes: number;
  connectors_healthy: number;
  connectors_total: number;
  notice: string;
};

type Topology = {
  lens: string;
  notice: string;
  nodes: TopoNode[];
  edges: { id: string; source: string; target: string; utilization_pct: number; label: string }[];
};

type Change = { impact?: { items?: { device_id: string }[] } };

export function CommandPage() {
  const t = useT();
  const [params, setParams] = useSearchParams();
  const lens = params.get("lens") || "performance";
  const site = params.get("site") || "";
  const selected = params.get("device") || "";
  const summary = useQuery({ queryKey: ["summary"], queryFn: () => api<Summary>("/api/v1/command-room/summary") });
  const sites = useQuery({ queryKey: ["sites"], queryFn: () => api<{ id: string; name: string }[]>("/api/v1/sites") });
  const topology = useQuery({
    queryKey: ["topology", lens, site],
    queryFn: () => api<Topology>(`/api/v1/topology?lens=${encodeURIComponent(lens)}&site_id=${encodeURIComponent(site)}`),
  });
  const changes = useQuery({
    queryKey: ["changes", "security"],
    queryFn: () => api<Change[]>("/api/v1/changes"),
    enabled: lens === "security",
  });
  const highlighted = useMemo(() => {
    if (lens !== "security") return [];
    const items = changes.data?.[0]?.impact?.items ?? [];
    return items.map((item) => item.device_id);
  }, [changes.data, lens]);
  const node = topology.data?.nodes.find((item) => item.id === selected);

  function setParam(key: string, value: string) {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value);
    else next.delete(key);
    if (key !== "device") next.delete("device");
    setParams(next);
  }

  return (
    <Page title={t("command")} crumbs={[{ label: t("command") }]}>
      <QueryState loading={summary.isLoading} error={summary.error} onRetry={() => summary.refetch()}>
        {summary.data && (
          <>
            <p className="banner">{summary.data.notice}</p>
            <div className="cards">
              <div className="card"><strong>{summary.data.sites}</strong><span>{t("sites")}</span></div>
              <div className="card"><strong>{summary.data.devices}</strong><span>{t("devices")}</span></div>
              <div className="card"><strong>{summary.data.open_drift}</strong><span>{t("drift")}</span></div>
              <div className="card"><strong>{summary.data.active_changes}</strong><span>{t("activeChanges")}</span></div>
              <div className="card"><strong>{summary.data.connectors_healthy}/{summary.data.connectors_total}</strong><span>{t("healthy")}</span></div>
            </div>
          </>
        )}
      </QueryState>
      <div className="panel" style={{ marginTop: 12 }}>
        <div className="page-title-row">
          <h2>{t("topology")}</h2>
          <div className="lenses" role="group" aria-label={t("topology")}>
            {(["performance", "security", "application"] as const).map((item) => (
              <button key={item} type="button" className="secondary" aria-pressed={lens === item} onClick={() => setParam("lens", item === "performance" ? "" : item)}>
                {t(item)}
              </button>
            ))}
          </div>
        </div>
        <label className="field" style={{ maxWidth: 240, margin: "8px 0" }}>
          {t("site")}
          <select value={site} onChange={(event) => setParam("site", event.target.value)} disabled={lens === "application"}>
            <option value="">{t("allSites")}</option>
            {(sites.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
          </select>
        </label>
        <QueryState loading={topology.isLoading} error={topology.error} onRetry={() => topology.refetch()}>
          <p className="note">{topology.data?.notice} {t("noticeTopology")}</p>
          {topology.data && topology.data.nodes.length === 0 ? <Empty text={t("empty")} /> : null}
          {topology.data && topology.data.nodes.length > 0 ? (
            <div className="split" style={{ marginTop: 8 }}>
              <TopologyGraph
                nodes={topology.data.nodes}
                edges={topology.data.edges ?? []}
                selected={selected}
                highlighted={highlighted}
                onSelect={(id) => setParam("device", id)}
              />
              <aside className="panel" aria-live="polite">
                <h2>{t("details")}</h2>
                {!node ? <p className="note">{t("selectNode")}</p> : (
                  <dl className="stack">
                    <div><dt className="note">{t("name")}</dt><dd>{node.label}</dd></div>
                    <div><dt className="note">{t("kind")}</dt><dd><Badge value={node.kind} /></dd></div>
                    <div><dt className="note">{t("health")}</dt><dd><Badge value={node.health} /></dd></div>
                    {node.meta?.data_origin ? <div><dt className="note">{t("origin")}</dt><dd><Badge value={String(node.meta.data_origin)} /></dd></div> : null}
                    {node.meta?.tenant ? <div><dt className="note">{t("tenant")}</dt><dd>{String(node.meta.tenant)}</dd></div> : null}
                    {node.meta?.cidr ? <div><dt className="note">{t("cidr")}</dt><dd>{String(node.meta.cidr)}</dd></div> : null}
                    {node.kind !== "segment" ? <Link to={`/inventory/devices/${node.id}`}>{t("openDevice")}</Link> : null}
                  </dl>
                )}
              </aside>
            </div>
          ) : null}
        </QueryState>
      </div>
    </Page>
  );
}
