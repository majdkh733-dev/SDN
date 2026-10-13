import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { Link, useOutletContext } from "react-router-dom";
import { api, can, type Principal } from "../api";
import { diagramCatalog } from "../diagram/catalog";
import { classifySymbol, referenceIcon, regionAt, suggestDevices } from "../diagram/classify";
import { mountDiagram, type CellInfo, type DiagramHandle } from "../diagram/drawio";
import { diagramRegions } from "../diagram/regions";
import { Badge, Empty, Field, FormError, formatTime, message, useT } from "../ui";

type Binding = { cell_id: string; device_id?: string; updated_at: string };
type LinkBinding = {
  cell_id: string; source_device_id?: string; source_interface_id?: string; target_device_id?: string; target_interface_id?: string;
  connection_type: string; evidence: string;
};
type DiagramDoc = { source: string; version_id?: string; updated_at?: string; mxfile?: string; bindings: Binding[]; link_bindings?: LinkBinding[] };
type Version = { id: string; created_at: string };
type Device = {
  id: string; name: string; site_id: string; connector_id?: string; model: string; vendor: string; serial: string; os_version: string;
  management_endpoint: string; health: string; role: string; data_origin: string; support_note: string;
  observed_at?: string; last_seen_at?: string;
};
type SymbolRow = CellInfo & { symbolKind: string };
type DeviceDetail = { device: Device; interfaces: { id: string; name: string; oper_up: boolean; speed_mbps: number; mac: string }[]; capabilities: { capability: string; supported: boolean; evidence: string }[] };
type LiveEdge = { id: string; source: string; target: string; utilization_pct: number; label: string; origin?: string; measured?: boolean };
type Drift = { id: string; device_id?: string; disposition: string; difference?: { detail?: string; kind?: string } };
type Change = { id: string; status: string; impact?: { items?: { device_id: string; device_name?: string }[] } };

const REFERENCE = "/drawio/utec.drawio";

export function ReferenceTopologyPage() {
  return <ReferenceDiagram />;
}

export function ReferenceDiagram({ embedded = false }: { embedded?: boolean }) {
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
  const [deviceQuery, setDeviceQuery] = useState("");
  const [deviceSite, setDeviceSite] = useState("");
  const [symbols, setSymbols] = useState<SymbolRow[]>([]);
  const [mapQuery, setMapQuery] = useState("");
  const [linkForm, setLinkForm] = useState({ sourceDevice: "", sourceInterface: "", targetDevice: "", targetInterface: "", connectionType: "documented", evidence: "manual" });
  const doc = useQuery({ queryKey: ["topology-diagram"], queryFn: () => api<DiagramDoc>("/api/v1/topology-diagram") });
  const xml = useQuery({
    queryKey: ["topology-xml", doc.data?.source, doc.data?.version_id, doc.isError],
    queryFn: async () => {
      if (doc.data?.mxfile) return doc.data.mxfile;
      const response = await fetch(REFERENCE);
      if (!response.ok) throw new Error(String(response.status));
      return response.text();
    },
    enabled: doc.isFetched,
  });
  const devices = useQuery({ queryKey: ["devices"], queryFn: () => api<Device[]>("/api/v1/devices") });
  const sites = useQuery({ queryKey: ["sites"], queryFn: () => api<{ id: string; name: string }[]>("/api/v1/sites") });
  const live = useQuery({ queryKey: ["topology", "reference"], queryFn: () => api<{ edges: LiveEdge[] }>("/api/v1/topology?lens=performance") });
  const binding = (doc.data?.bindings || []).find((item) => item.cell_id === selected);
  const savedLink = (doc.data?.link_bindings || []).find((item) => item.cell_id === selected);
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
  const versions = useQuery({ queryKey: ["topology-versions"], queryFn: () => api<Version[]>("/api/v1/topology-diagram/versions"), enabled: author });
  const vlans = useQuery({ queryKey: ["vlans", "diagram"], queryFn: () => api<{ id: string; site_id: string; vid: number; name: string }[]>("/api/v1/vlans"), enabled: Boolean(deviceId) });
  const backups = useQuery({ queryKey: ["backups", "diagram"], queryFn: () => api<{ id: string; scope: string; checksum: string; integrity: string; created_at: string }[]>("/api/v1/backups"), enabled: Boolean(deviceId) });
  const policies = useQuery({ queryKey: ["policies", "diagram"], queryFn: () => api<{ id: string; name: string; spec?: { site_ids?: string[] } }[]>("/api/v1/policies"), enabled: Boolean(deviceId) });
  const sourceIfaces = useQuery({
    queryKey: ["interfaces", linkForm.sourceDevice],
    queryFn: () => api<{ id: string; name: string; oper_up: boolean; speed_mbps: number }[]>(`/api/v1/devices/${linkForm.sourceDevice}/interfaces`),
    enabled: Boolean(linkForm.sourceDevice),
  });
  const targetIfaces = useQuery({
    queryKey: ["interfaces", linkForm.targetDevice],
    queryFn: () => api<{ id: string; name: string; oper_up: boolean; speed_mbps: number }[]>(`/api/v1/devices/${linkForm.targetDevice}/interfaces`),
    enabled: Boolean(linkForm.targetDevice),
  });
  const bind = useMutation({
    mutationFn: (body: { cell_id: string; device_id: string }) => api("/api/v1/topology-diagram/bindings", { method: "POST", body: JSON.stringify(body) }),
    onSuccess: async () => {
      setNotice(t("bindingSaved"));
      await qc.invalidateQueries({ queryKey: ["topology-diagram"] });
    },
  });
  const mapLink = useMutation({
    mutationFn: (body: LinkBinding) => api("/api/v1/topology-diagram/links", { method: "POST", body: JSON.stringify(body) }),
    onSuccess: async () => {
      setNotice(t("linkMapped"));
      await qc.invalidateQueries({ queryKey: ["topology-diagram"] });
    },
  });

  useEffect(() => {
    const host = canvas.current;
    if (!host || !xml.data) return;
    let dead = false;
    void mountDiagram(host, xml.data, (id) => {
      setSelected(id);
    }).then((next) => {
      if (dead) return;
      handle.current = next;
      next.setEditing(editing);
      setSymbols(next.symbols());
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
    setLinkForm({
      sourceDevice: savedLink?.source_device_id || "",
      sourceInterface: savedLink?.source_interface_id || "",
      targetDevice: savedLink?.target_device_id || "",
      targetInterface: savedLink?.target_interface_id || "",
      connectionType: savedLink?.connection_type || "documented",
      evidence: savedLink?.evidence || "manual",
    });
  }, [selected, binding?.device_id, savedLink?.source_device_id, savedLink?.source_interface_id, savedLink?.target_device_id, savedLink?.target_interface_id, savedLink?.connection_type, savedLink?.evidence, xml.data]);

  const boundIds = new Set((doc.data?.bindings || []).map((item) => item.device_id).filter(Boolean));
  const unplaced = (devices.data || []).filter((item) => !boundIds.has(item.id));
  const siteName = sites.data?.find((item) => item.id === detail.data?.device.site_id)?.name;
  const relatedChanges = (changes.data || []).filter((item) => (item.impact?.items || []).some((row) => row.device_id === deviceId));
  const relatedDrift = (drifts.data || []).filter((item) => item.device_id === deviceId);
  const sourceDevice = info?.sourceId ? (doc.data?.bindings || []).find((item) => item.cell_id === info.sourceId)?.device_id : "";
  const targetDevice = info?.targetId ? (doc.data?.bindings || []).find((item) => item.cell_id === info.targetId)?.device_id : "";
  const discovered = (live.data?.edges || []).find((item) => sourceDevice && targetDevice && ((item.source === sourceDevice && item.target === targetDevice) || (item.source === targetDevice && item.target === sourceDevice)));
  const duplicateIds = new Set((doc.data?.bindings || []).map((item) => item.device_id).filter((id, index, all) => id && all.indexOf(id) !== index));
  const deviceSymbols = symbols.filter((item) => item.symbolKind === "device");
  const linkSymbols = symbols.filter((item) => item.symbolKind === "link");
  const mappedDevices = deviceSymbols.filter((item) => (doc.data?.bindings || []).some((binding) => binding.cell_id === item.id && binding.device_id));
  const verifiedLinks = linkSymbols.filter((item) => {
    const row = (doc.data?.link_bindings || []).find((binding) => binding.cell_id === item.id);
    return Boolean(row?.source_interface_id && row?.target_interface_id);
  });
  const filteredDevices = (devices.data || []).filter((item) => (!deviceSite || item.site_id === deviceSite) && (!deviceQuery || `${item.name} ${item.management_endpoint} ${item.serial}`.toLowerCase().includes(deviceQuery.toLowerCase())));
  const preview = (devices.data || []).find((item) => item.id === linkDevice);
  const suggestions = info?.kind === "vertex" ? suggestDevices(info.label, devices.data || []) : [];
  const selectedKind = info ? classifySymbol({ vertex: info.kind === "vertex", style: info.style, label: info.label, width: info.width, height: info.height }) : "";
  const siteVlans = (vlans.data || []).filter((item) => item.site_id === detail.data?.device.site_id);
  const sitePolicies = (policies.data || []).filter((item) => (item.spec?.site_ids || []).includes(detail.data?.device.site_id || ""));
  const neighbors = (live.data?.edges || []).filter((item) => item.source === deviceId || item.target === deviceId);

  function persist() {
    const mxfile = handle.current?.serialize();
    if (mxfile) save.mutate(mxfile);
  }

  function rememberSymbols() {
    setSymbols(handle.current?.symbols() || []);
  }

  return (
    <div className={embedded ? "ref-topology embedded" : "ref-topology"}>
      <div className="ref-toolbar">
        <div>
          {embedded ? <h2>{t("referenceTopology")}</h2> : <h1>{t("referenceTopology")}</h1>}
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
          <button type="button" className="secondary" onClick={() => { void detail.refetch(); void live.refetch(); }}>{t("refreshTelemetry")}</button>
          {author ? <button type="button" className="secondary" aria-pressed={editing} onClick={() => setEditing((value) => !value)}>{t("editDiagram")}</button> : null}
        </div>
      </div>
      {doc.isError ? (
        <p className="state error" role="alert">
          {t("diagramRequestFailed")}
          <button type="button" className="secondary" onClick={() => void doc.refetch()}>{t("retry")}</button>
        </p>
      ) : null}
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
          <button type="button" className="secondary" onClick={() => {
            const style = diagramCatalog.find((entry) => entry.id === "building-group");
            if (!style || !handle.current) return;
            handle.current.placeDevice("Group", style.style, 420, 240);
            rememberSymbols();
          }}>{t("addGroup")}</button>
          <button type="button" className="secondary" onClick={() => {
            const frame = diagramCatalog.find((entry) => entry.id === "building-group");
            const badge = diagramCatalog.find((entry) => entry.id === "site-badge");
            if (!frame || !badge || !handle.current) return;
            handle.current.placeDevice("Site", frame.style, 760, 520);
            ["WAN", "Core Layer", "Distribution layer", "Access Layer"].forEach((label, index) => {
              handle.current?.placeDevice(label, badge.style, 180, 36, -240, -180 + index * 90);
            });
            rememberSymbols();
          }}>{t("siteTemplate")}</button>
          <button type="button" onClick={() => { rememberSymbols(); persist(); }}>{t("save")}</button>
          <button type="button" className="secondary" onClick={() => { setEditing(false); void qc.invalidateQueries({ queryKey: ["topology-xml"] }); }}>{t("cancel")}</button>
          {(versions.data || []).length > 0 ? (
            <Field label={t("versions")}>
              <select defaultValue="" onChange={(event) => {
                const id = event.target.value;
                if (!id) return;
                void api<Version & { mxfile: string }>(`/api/v1/topology-diagram/versions/${id}`).then((item) => save.mutate(item.mxfile));
              }}>
                <option value="">{t("restoreVersion")}</option>
                {versions.data?.map((item) => <option key={item.id} value={item.id}>{formatTime(item.created_at)}</option>)}
              </select>
            </Field>
          ) : null}
          <FormError error={save.error} />
        </div>
      ) : null}
      <div className="ref-stage">
        <section className="ref-canvas-wrap" ref={stage}>
          {xml.isLoading || doc.isLoading ? <p className="state" role="status">{t("loading")}</p> : null}
          {xml.isError ? <p className="state error" role="alert">{t("diagramFileFailed")}</p> : null}
          <div ref={canvas} className="ref-canvas" />
        </section>
        <aside className={info ? "panel ref-drawer" : "panel ref-drawer empty"}>
          {!info ? <Empty text={t("selectSymbol")} /> : (
            <>
              <h2>{info.label || referenceIcon(info.style) || t("unlabeled")}</h2>
              <p className="note">{info.kind === "edge" ? t("documentedLink") : t("diagramSymbol")} · {info.id}</p>
              {info.kind === "edge" ? (
                <>
                  <p>{t("endpoints")} {info.sourceLabel || t("unknown")} → {info.targetLabel || t("unknown")}</p>
                  {savedLink?.source_interface_id && savedLink.target_interface_id ? (
                    <p>{t("mappedLink")} {savedLink.connection_type} · {savedLink.evidence}</p>
                  ) : <p className="note">{t("linkUnresolved")}</p>}
                  {([
                    { label: t("sourceInterface"), id: linkForm.sourceInterface, rows: sourceIfaces.data },
                    { label: t("targetInterface"), id: linkForm.targetInterface, rows: targetIfaces.data },
                  ] as const).map((end) => {
                    const item = end.rows?.find((row) => row.id === end.id);
                    if (!item) return null;
                    return <p key={end.label}>{end.label} {item.name} · {item.oper_up ? t("up") : t("down")} · {item.speed_mbps ? `${item.speed_mbps} Mb` : t("notMeasured")} · {discovered?.measured ? `${discovered.utilization_pct}%` : t("notMeasured")} · {t("errorsUnknown")}</p>;
                  })}
                  <p className="note">{t("portsUnknown")}</p>
                  {discovered ? (
                    <p>{t("discoveredLink")} {discovered.measured ? `${discovered.utilization_pct}%` : t("notMeasured")} {discovered.origin || ""}</p>
                  ) : <p className="note">{t("notVerified")}</p>}
                  <p className="note">{t("errorsUnknown")}</p>
                  {selectedKind !== "link" ? <p className="note">{t("notDeviceLink")}</p> : null}
                  {author && selectedKind === "link" ? (
                    <form className="stack" onSubmit={(event) => {
                      event.preventDefault();
                      mapLink.mutate({
                        cell_id: info.id,
                        source_device_id: linkForm.sourceDevice,
                        source_interface_id: linkForm.sourceInterface,
                        target_device_id: linkForm.targetDevice,
                        target_interface_id: linkForm.targetInterface,
                        connection_type: linkForm.connectionType,
                        evidence: linkForm.evidence,
                      });
                    }}>
                      <Field label={t("sourceDevice")}>
                        <select value={linkForm.sourceDevice} onChange={(event) => setLinkForm({ ...linkForm, sourceDevice: event.target.value, sourceInterface: "" })}>
                          <option value="">{t("unresolved")}</option>
                          {(devices.data || []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
                        </select>
                      </Field>
                      <Field label={t("sourceInterface")}>
                        <select value={linkForm.sourceInterface} onChange={(event) => setLinkForm({ ...linkForm, sourceInterface: event.target.value })}>
                          <option value="">{t("unresolved")}</option>
                          {(sourceIfaces.data || []).map((item) => <option key={item.id} value={item.id}>{item.name} · {item.oper_up ? t("up") : t("down")} · {item.speed_mbps || t("notMeasured")}</option>)}
                        </select>
                      </Field>
                      <Field label={t("targetDevice")}>
                        <select value={linkForm.targetDevice} onChange={(event) => setLinkForm({ ...linkForm, targetDevice: event.target.value, targetInterface: "" })}>
                          <option value="">{t("unresolved")}</option>
                          {(devices.data || []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
                        </select>
                      </Field>
                      <Field label={t("targetInterface")}>
                        <select value={linkForm.targetInterface} onChange={(event) => setLinkForm({ ...linkForm, targetInterface: event.target.value })}>
                          <option value="">{t("unresolved")}</option>
                          {(targetIfaces.data || []).map((item) => <option key={item.id} value={item.id}>{item.name} · {item.oper_up ? t("up") : t("down")} · {item.speed_mbps || t("notMeasured")}</option>)}
                        </select>
                      </Field>
                      <Field label={t("connectionType")}>
                        <select value={linkForm.connectionType} onChange={(event) => setLinkForm({ ...linkForm, connectionType: event.target.value })}>
                          <option value="documented">{t("connDocumented")}</option>
                          <option value="physical">{t("connPhysical")}</option>
                          <option value="logical">{t("connLogical")}</option>
                          <option value="wan">{t("connWan")}</option>
                        </select>
                      </Field>
                      <Field label={t("evidence")}>
                        <select value={linkForm.evidence} onChange={(event) => setLinkForm({ ...linkForm, evidence: event.target.value })}>
                          <option value="manual">{t("evidenceManual")}</option>
                          <option value="discovered">{t("evidenceDiscovered")}</option>
                          <option value="documented">{t("connDocumented")}</option>
                        </select>
                      </Field>
                      <button type="submit">{t("mapLink")}</button>
                      <FormError error={mapLink.error} />
                    </form>
                  ) : null}
                </>
              ) : null}
              {info.kind === "vertex" && !deviceId ? (
                <p className="state" role="status">{t("unmapped")}</p>
              ) : null}
              {info.kind === "vertex" && deviceId ? (
                detail.data ? (
                  <DeviceFacts device={detail.data.device} siteName={siteName} interfaces={detail.data.interfaces} capabilities={detail.data.capabilities} changes={relatedChanges} drifts={relatedDrift} vlans={siteVlans} policies={sitePolicies} neighbors={neighbors} devices={devices.data || []} backups={backups.data || []} />
                ) : detail.isLoading ? <p className="note">{t("loading")}</p> : <p className="note">{t("missingDevice")}</p>
              ) : null}
              {author && info.kind === "vertex" ? (
                <form className="stack" onSubmit={(event) => { event.preventDefault(); bind.mutate({ cell_id: info.id, device_id: linkDevice }); }}>
                  <Field label={t("inventorySearch")}>
                    <input value={deviceQuery} onChange={(event) => setDeviceQuery(event.target.value)} />
                  </Field>
                  <Field label={t("site")}>
                    <select value={deviceSite} onChange={(event) => setDeviceSite(event.target.value)}>
                      <option value="">{t("allSites")}</option>
                      {(sites.data || []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
                    </select>
                  </Field>
                  <Field label={t("linkDevice")}>
                    <select value={linkDevice} onChange={(event) => setLinkDevice(event.target.value)}>
                      <option value="">{t("unmapped")}</option>
                      {filteredDevices.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
                    </select>
                  </Field>
                  {preview ? <p className="note">{preview.name} · {preview.management_endpoint || t("unknown")} · {preview.vendor || t("unknown")} · {preview.serial || t("unknown")}</p> : null}
                  {suggestions.length > 0 ? (
                    <div>
                      <p className="note">{t("reviewRequired")}</p>
                      {suggestions.map((item) => {
                        const device = (devices.data || []).find((row) => row.id === item.deviceId);
                        const evidence = item.evidence === "management-address" ? t("evidenceAddress") : item.evidence === "serial" ? t("evidenceSerial") : t("evidenceName");
                        return <button key={item.deviceId} type="button" className="secondary" onClick={() => setLinkDevice(item.deviceId)}>{device?.name} · {evidence}{item.ambiguous ? ` · ${t("ambiguous")}` : ""}</button>;
                      })}
                    </div>
                  ) : <p className="note">{t("noSuggestion")}</p>}
                  <button type="submit">{linkDevice ? t("linkDevice") : t("removeBinding")}</button>
                  {deviceId ? <button type="button" className="secondary" onClick={() => handle.current?.setLabel(info.id, detail.data?.device.name || info.label)}>{t("syncLabel")}</button> : null}
                  <FormError error={bind.error} />
                </form>
              ) : null}
            </>
          )}
        </aside>
      </div>
      {!embedded ? <><section className="panel">
        <h2>{t("mappingTable")}</h2>
        <p className="note">{t("deviceSymbols")} {mappedDevices.length}/{deviceSymbols.length || "—"} · {t("verifiedLinks")} {verifiedLinks.length}/{linkSymbols.length || "—"} · {t("unresolvedLinks")} {Math.max(linkSymbols.length - verifiedLinks.length, 0)}</p>
        <input value={mapQuery} placeholder={t("search")} onChange={(event) => setMapQuery(event.target.value)} />
        <div className="map-table-wrap">
          <table className="map-table">
            <thead>
              <tr>
                <th>{t("diagramSymbol")}</th>
                <th>{t("site")}</th>
                <th>{t("linkDevice")}</th>
                <th>{t("address")}</th>
                <th>{t("mappingStatus")}</th>
                <th>{t("suggestion")}</th>
              </tr>
            </thead>
            <tbody>
              {deviceSymbols.filter((item) => !mapQuery || `${item.label} ${item.id}`.toLowerCase().includes(mapQuery.toLowerCase())).map((item) => {
                const bound = (doc.data?.bindings || []).find((row) => row.cell_id === item.id);
                const device = (devices.data || []).find((row) => row.id === bound?.device_id);
                const ideas = suggestDevices(item.label, devices.data || []);
                const status = !bound?.device_id ? t("unmapped") : duplicateIds.has(bound.device_id) ? t("duplicateBinding") : device ? t("mapped") : t("missingDevice");
                return (
                  <tr key={item.id}>
                    <td><button type="button" className="secondary" onClick={() => { handle.current?.select(item.id); setSelected(item.id); }}>{item.label || referenceIcon(item.style) || t("unlabeled")}</button><span className="note"> {item.id}</span></td>
                    <td>{regionAt(item.x, item.y)?.label || t("unknown")}</td>
                    <td>{device?.name || t("unmapped")}</td>
                    <td>{device?.management_endpoint || t("unknown")}</td>
                    <td>{status}</td>
                    <td>{ideas.length === 0 ? t("noSuggestion") : ideas.map((idea) => (devices.data || []).find((row) => row.id === idea.deviceId)?.name).filter(Boolean).join(", ")}{ideas.some((idea) => idea.ambiguous) ? ` · ${t("reviewRequired")}` : ""}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </section>
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
                  rememberSymbols();
                  persist();
                }}>{t("placeDevice")}</button> : null}
              </li>
            ))}
          </ul>
        )}
      </section></> : null}
    </div>
  );
}

function DeviceFacts({ device, siteName, interfaces, capabilities, changes, drifts, vlans, policies, neighbors, devices, backups }: {
  device: Device;
  siteName?: string;
  interfaces: DeviceDetail["interfaces"];
  capabilities: DeviceDetail["capabilities"];
  changes: Change[];
  drifts: Drift[];
  vlans: { id: string; vid: number; name: string }[];
  policies: { id: string; name: string }[];
  neighbors: LiveEdge[];
  devices: Device[];
  backups: { id: string; scope: string; checksum: string; integrity: string; created_at: string }[];
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
      <p>{t("health")} <Badge value={device.health || "unknown"} /> {device.connector_id ? (fresh ? formatTime(seen || "") : t("telemetryStale")) : t("noConnector")}</p>
      <p className="note">{t("iconNotHealth")}</p>
      <h3>{t("neighbors")}</h3>
      {neighbors.length === 0 ? <p className="note">{t("unknown")}</p> : neighbors.map((item) => {
        const other = devices.find((row) => row.id === (item.source === device.id ? item.target : item.source));
        return <p key={item.id}>{other?.name || t("unknown")} · {item.label || t("unknown")} · {item.measured ? `${item.utilization_pct}%` : t("notMeasured")}</p>;
      })}
      <h3>{t("vlans")}</h3>
      {vlans.length === 0 ? <p className="note">{t("unknown")}</p> : vlans.map((item) => <p key={item.id}>{item.vid} {item.name}</p>)}
      <p className="note">{t("routingUnsupported")}</p>
      <h3>{t("policies")}</h3>
      {policies.length === 0 ? <p className="note">{t("unknown")}</p> : policies.map((item) => <p key={item.id}><Link to="/policies">{item.name}</Link></p>)}
      <h3>{t("interfaces")}</h3>
      {interfaces.length === 0 ? <p className="note">{t("unknown")}</p> : (
        <ul>{interfaces.map((item) => <li key={item.id}>{item.name} · {item.oper_up ? t("up") : t("down")} · {item.speed_mbps ? `${item.speed_mbps} Mb` : t("notMeasured")}</li>)}</ul>
      )}
      <h3>{t("capabilities")}</h3>
      {capabilities.length === 0 ? <p className="note">{t("unknown")}</p> : (
        <ul>{capabilities.map((item) => <li key={item.capability}>{item.capability}: {item.supported ? t("supported") : t("unsupported")} · {item.evidence}</li>)}</ul>
      )}
      <h3>{t("backups")}</h3>
      {backups.length === 0 ? <p className="note">{t("unknown")}</p> : (
        <p>{backups[0].scope} · {backups[0].integrity || t("unknown")} · {formatTime(backups[0].created_at)}</p>
      )}
      <p className="note">{t("backupManifest")}</p>
      <p><Link to="/drift">{t("backups")}</Link> · <Link to="/changes">{t("changes")}</Link> · <Link to="/policies">{t("policies")}</Link></p>
      <h3>{t("drift")}</h3>
      {drifts.length === 0 ? <p className="note">{t("empty")}</p> : drifts.map((item) => <p key={item.id}><Badge value={item.disposition} /> {item.difference?.detail || item.difference?.kind}</p>)}
      <h3>{t("changes")}</h3>
      {changes.length === 0 ? <p className="note">{t("empty")}</p> : changes.slice(0, 5).map((item) => <p key={item.id}><Link to={`/changes/${item.id}`}>{item.status}</Link></p>)}
      <p className="note">{device.support_note}</p>
    </div>
  );
}
