import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { Link, useOutletContext } from "react-router-dom";
import { api, can, type Principal } from "../api";
import { diagramCatalog } from "../diagram/catalog";
import { mountDiagram, type CellInfo, type DiagramHandle } from "../diagram/drawio";
import { diagramRegions } from "../diagram/regions";
import { Badge, Empty, Field, FormError, formatTime, message, useT } from "../ui";

type Binding = { cell_id: string; device_id?: string; updated_at: string };
type DiagramDoc = { source: string; version_id?: string; updated_at?: string; mxfile?: string; bindings: Binding[] };
type Device = {
  id: string; name: string; site_id: string; model: string; vendor: string; serial: string; os_version: string;
  management_endpoint: string; health: string; role: string; data_origin: string; support_note: string;
  observed_at?: string; last_seen_at?: string;
};
type DeviceDetail = { device: Device; interfaces: { id: string; name: string; oper_up: boolean; speed_mbps: number; mac: string }[]; capabilities: { capability: string; supported: boolean; evidence: string }[] };
type LiveEdge = { id: string; source: string; target: string; utilization_pct: number; label: string; origin?: string; measured?: boolean };
type Drift = { id: string; device_id?: string; disposition: string; difference?: { detail?: string; kind?: string } };
type Change = { id: string; status: string; impact?: { items?: { device_id: string; device_name?: string }[] } };

const REFERENCE = "/drawio/utec.drawio";

export function ReferenceTopologyPage() {
  const t = useT();
  const principal = useOutletContext<{ principal: Principal }>().principal;
  const author = can(principal.role, "author");
  const qc = useQueryClient();
  const stage = useRef<HTMLElement>(null);
  const canvas = useRef<HTMLDivElement>(null);
  const handle = useRef<DiagramHandle | null>(null);
  const [query, setQuery] = useState("");
  const [region, setRegion] = useState("");
  const [selected, setSelected] = useState("");
  const [info, setInfo] = useState<CellInfo | null>(null);
  const [editing, setEditing] = useState(false);
  const [notice, setNotice] = useState("");
  const [icon, setIcon] = useState(diagramCatalog.find((item) => item.kind === "device")?.id || "");
  const [linkDevice, setLinkDevice] = useState("");
  const doc = useQuery({ queryKey: ["topology-diagram"], queryFn: () => api<DiagramDoc>("/api/v1/topology-diagram") });
  const xml = useQuery({
    queryKey: ["topology-xml", doc.data?.source, doc.data?.version_id],
    queryFn: async () => doc.data?.mxfile || await fetch(REFERENCE).then((res) => res.text()),
    enabled: Boolean(doc.data),
  });
  const devices = useQuery({ queryKey: ["devices"], queryFn: () => api<Device[]>("/api/v1/devices") });
  const sites = useQuery({ queryKey: ["sites"], queryFn: () => api<{ id: string; name: string }[]>("/api/v1/sites") });
  const live = useQuery({ queryKey: ["topology", "reference"], queryFn: () => api<{ edges: LiveEdge[] }>("/api/v1/topology?lens=performance") });
  const binding = (doc.data?.bindings || []).find((item) => item.cell_id === selected);
  const deviceId = binding?.device_id || "";
  const detail = useQuery({
    queryKey: ["device", deviceId],
    queryFn: () => api<DeviceDetail>(`/api/v1/devices/${deviceId}`),
    enabled: Boolean(deviceId),
  });
  const drifts = useQuery({ queryKey: ["drift"], queryFn: () => api<Drift[]>("/api/v1/drift-findings"), enabled: Boolean(deviceId) });
  const changes = useQuery({ queryKey: ["changes", "diagram"], queryFn: () => api<Change[]>("/api/v1/changes"), enabled: Boolean(deviceId) });
  const save = useMutation({
    mutationFn: (mxfile: string) => api("/api/v1/topology-diagram", { method: "POST", body: JSON.stringify({ mxfile }) }),
    onSuccess: async () => {
      setNotice(t("diagramSaved"));
      await qc.invalidateQueries({ queryKey: ["topology-diagram"] });
    },
  });
  const bind = useMutation({
    mutationFn: (body: { cell_id: string; device_id: string }) => api("/api/v1/topology-diagram/bindings", { method: "POST", body: JSON.stringify(body) }),
    onSuccess: async () => {
      setNotice(t("bindingSaved"));
      await qc.invalidateQueries({ queryKey: ["topology-diagram"] });
    },
  });

  useEffect(() => {
    const host = canvas.current;
    if (!host || !xml.data) return;
    let dead = false;
    setNotice("");
    void mountDiagram(host, xml.data, (id) => {
      setSelected(id);
    }).then((next) => {
      if (dead) return;
      handle.current = next;
      next.setEditing(editing);
    }).catch((error: unknown) => setNotice(message(error, "The diagram could not be shown")));
    return () => {
      dead = true;
      handle.current = null;
    };
  }, [xml.data]);

  useEffect(() => {
    handle.current?.setEditing(editing);
  }, [editing]);

  useEffect(() => {
    if (!selected || !handle.current) {
      setInfo(null);
      return;
    }
    setInfo(handle.current.info(selected));
    setLinkDevice(binding?.device_id || "");
  }, [selected, binding?.device_id, xml.data]);

  const boundIds = new Set((doc.data?.bindings || []).map((item) => item.device_id).filter(Boolean));
  const unplaced = (devices.data || []).filter((item) => !boundIds.has(item.id));
  const siteName = sites.data?.find((item) => item.id === detail.data?.device.site_id)?.name;
  const relatedChanges = (changes.data || []).filter((item) => (item.impact?.items || []).some((row) => row.device_id === deviceId));
  const relatedDrift = (drifts.data || []).filter((item) => item.device_id === deviceId);
  const sourceDevice = info?.sourceId ? (doc.data?.bindings || []).find((item) => item.cell_id === info.sourceId)?.device_id : "";
  const targetDevice = info?.targetId ? (doc.data?.bindings || []).find((item) => item.cell_id === info.targetId)?.device_id : "";
  const discovered = (live.data?.edges || []).find((item) => sourceDevice && targetDevice && ((item.source === sourceDevice && item.target === targetDevice) || (item.source === targetDevice && item.target === sourceDevice)));

  function persist() {
    const mxfile = handle.current?.serialize();
    if (mxfile) save.mutate(mxfile);
  }

  return (
    <div className="ref-topology">
      <div className="ref-toolbar">
        <div>
          <h1>{t("referenceTopology")}</h1>
          <p className="note">{doc.data?.source === "saved" ? t("diagramSavedCopy") : t("diagramReference")}</p>
        </div>
        <label className="field">{t("site")}
          <select value={region} onChange={(event) => {
            const id = event.target.value;
            setRegion(id);
            const item = diagramRegions.find((region) => region.id === id);
            if (!item) handle.current?.reset();
            else handle.current?.focus(item);
          }}>
            <option value="">{t("fullDiagram")}</option>
            {diagramRegions.map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
          </select>
        </label>
        <form className="search" onSubmit={(event) => {
          event.preventDefault();
          const id = handle.current?.find(query) || "";
          setSelected(id);
          if (!id) setNotice(t("diagramNoMatch"));
        }}>
          <input value={query} placeholder={t("search")} onChange={(event) => setQuery(event.target.value)} />
        </form>
        <div className="lenses">
          <button type="button" className="secondary" onClick={() => handle.current?.zoomBy(1.25)}>{t("zoomIn")}</button>
          <button type="button" className="secondary" onClick={() => handle.current?.zoomBy(0.8)}>{t("zoomOut")}</button>
          <button type="button" className="secondary" onClick={() => handle.current?.fit()}>{t("fit")}</button>
          <button type="button" className="secondary" onClick={() => { setRegion(""); handle.current?.reset(); }}>{t("resetView")}</button>
          <button type="button" className="secondary" onClick={() => {
            if (document.fullscreenElement) void document.exitFullscreen();
            else void stage.current?.requestFullscreen();
          }}>{t("fullscreen")}</button>
          <button type="button" className="secondary" onClick={() => void detail.refetch()}>{t("refreshTelemetry")}</button>
          {author ? <button type="button" className="secondary" aria-pressed={editing} onClick={() => setEditing((value) => !value)}>{t("editDiagram")}</button> : null}
        </div>
      </div>
      {notice ? <p className="note" role="status">{notice}</p> : null}
      {editing ? (
        <div className="ref-edit">
          <button type="button" className="secondary" onClick={() => handle.current?.undo()}>{t("undo")}</button>
          <button type="button" className="secondary" onClick={() => handle.current?.redo()}>{t("redo")}</button>
          <Field label={t("icon")}>
            <select value={icon} onChange={(event) => setIcon(event.target.value)}>
              {diagramCatalog.filter((item) => item.kind === "device").map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
            </select>
          </Field>
          <button type="button" onClick={() => persist()}>{t("save")}</button>
          <button type="button" className="secondary" onClick={() => { setEditing(false); void qc.invalidateQueries({ queryKey: ["topology-xml"] }); }}>{t("cancel")}</button>
          <FormError error={save.error} />
        </div>
      ) : null}
      <div className="ref-stage">
        <section className="ref-canvas-wrap" ref={stage}>
          {xml.isLoading || doc.isLoading ? <p className="state" role="status">{t("loading")}</p> : null}
          {xml.isError || doc.isError ? <p className="state error" role="alert">{t("requestFailed")}</p> : null}
          <div ref={canvas} className="ref-canvas" />
        </section>
        <aside className="panel ref-drawer">
          {!info ? <Empty text={t("selectSymbol")} /> : (
            <>
              <h2>{info.label || t("unlabeled")}</h2>
              <p className="note">{info.kind === "edge" ? t("documentedLink") : t("diagramSymbol")} · {info.id}</p>
              {info.kind === "edge" ? (
                <>
                  <p>{t("endpoints")} {info.sourceLabel || t("unknown")} → {info.targetLabel || t("unknown")}</p>
                  <p className="note">{t("portsUnknown")}</p>
                  {discovered ? (
                    <p>{t("discoveredLink")} {discovered.measured ? `${discovered.utilization_pct}%` : t("notMeasured")} {discovered.origin || ""}</p>
                  ) : <p className="note">{t("notVerified")}</p>}
                </>
              ) : null}
              {info.kind === "vertex" && !deviceId ? (
                <p className="state" role="status">{t("unmapped")}</p>
              ) : null}
              {info.kind === "vertex" && deviceId ? (
                detail.data ? (
                  <DeviceFacts device={detail.data.device} siteName={siteName} interfaces={detail.data.interfaces} capabilities={detail.data.capabilities} changes={relatedChanges} drifts={relatedDrift} />
                ) : detail.isLoading ? <p className="note">{t("loading")}</p> : <p className="note">{t("missingDevice")}</p>
              ) : null}
              {author && info.kind === "vertex" ? (
                <form className="stack" onSubmit={(event) => { event.preventDefault(); bind.mutate({ cell_id: info.id, device_id: linkDevice }); }}>
                  <Field label={t("linkDevice")}>
                    <select value={linkDevice} onChange={(event) => setLinkDevice(event.target.value)}>
                      <option value="">{t("unmapped")}</option>
                      {(devices.data || []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
                    </select>
                  </Field>
                  <button type="submit">{t("linkDevice")}</button>
                  {deviceId ? <button type="button" className="secondary" onClick={() => handle.current?.setLabel(info.id, detail.data?.device.name || info.label)}>{t("syncLabel")}</button> : null}
                  <FormError error={bind.error} />
                </form>
              ) : null}
            </>
          )}
        </aside>
      </div>
      <section className="panel">
        <h2>{t("unplaced")}</h2>
        <p className="note">{t("unplacedNote")}</p>
        {unplaced.length === 0 ? <Empty text={t("empty")} /> : (
          <ul className="incident-list">
            {unplaced.map((item) => (
              <li key={item.id}>
                <strong>{item.name}</strong>
                <span className="note"> {item.management_endpoint || t("unknown")}</span>
                {author && editing ? <button type="button" className="secondary" onClick={() => {
                  const style = diagramCatalog.find((entry) => entry.id === icon);
                  if (!style || !handle.current) return;
                  const id = handle.current.placeDevice(item.name, style.style, style.width || 100, style.height || 100);
                  bind.mutate({ cell_id: id, device_id: item.id });
                  setSelected(id);
                  persist();
                }}>{t("placeDevice")}</button> : null}
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function DeviceFacts({ device, siteName, interfaces, capabilities, changes, drifts }: {
  device: Device;
  siteName?: string;
  interfaces: DeviceDetail["interfaces"];
  capabilities: DeviceDetail["capabilities"];
  changes: Change[];
  drifts: Drift[];
}) {
  const t = useT();
  const seen = device.last_seen_at || device.observed_at;
  const fresh = seen ? Date.now() - new Date(seen).getTime() < 90_000 : false;
  return (
    <div className="stack">
      <p><Link to={`/inventory/devices/${device.id}`}>{t("openDevice")}</Link></p>
      <p>{t("name")} {device.name}</p>
      <p>{t("address")} {device.management_endpoint || t("unknown")}</p>
      <p>{t("site")} {siteName || t("unknown")}</p>
      <p>{t("kind")} {device.role || t("unknown")}</p>
      <p>{t("vendor")} {device.vendor || t("unknown")} · {device.model || t("unknown")}</p>
      <p>{t("serial")} {device.serial || t("unknown")}</p>
      <p>{t("firmwareVersion")} {device.os_version || t("unknown")}</p>
      <p>{t("health")} <Badge value={device.health || "unknown"} /> {fresh ? formatTime(seen || "") : t("telemetryStale")}</p>
      <p className="note">{t("iconNotHealth")}</p>
      <h3>{t("interfaces")}</h3>
      {interfaces.length === 0 ? <p className="note">{t("unknown")}</p> : (
        <ul>{interfaces.map((item) => <li key={item.id}>{item.name} · {item.oper_up ? t("up") : t("down")} · {item.speed_mbps ? `${item.speed_mbps} Mb` : t("notMeasured")}</li>)}</ul>
      )}
      <h3>{t("capabilities")}</h3>
      {capabilities.length === 0 ? <p className="note">{t("unknown")}</p> : (
        <ul>{capabilities.map((item) => <li key={item.capability}>{item.capability}: {item.supported ? t("supported") : t("unsupported")} · {item.evidence}</li>)}</ul>
      )}
      <p><Link to="/drift">{t("backups")}</Link> · <Link to="/changes">{t("changes")}</Link> · <Link to="/policies">{t("policies")}</Link></p>
      <h3>{t("drift")}</h3>
      {drifts.length === 0 ? <p className="note">{t("empty")}</p> : drifts.map((item) => <p key={item.id}><Badge value={item.disposition} /> {item.difference?.detail || item.difference?.kind}</p>)}
      <h3>{t("changes")}</h3>
      {changes.length === 0 ? <p className="note">{t("empty")}</p> : changes.slice(0, 5).map((item) => <p key={item.id}><Link to={`/changes/${item.id}`}>{item.status}</Link></p>)}
      <p className="note">{device.support_note}</p>
    </div>
  );
}
