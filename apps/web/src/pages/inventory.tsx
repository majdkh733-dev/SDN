import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createColumnHelper, flexRender, getCoreRowModel, getSortedRowModel, useReactTable, type SortingState } from "@tanstack/react-table";
import { useState } from "react";
import { Link, useOutletContext, useParams, useSearchParams } from "react-router-dom";
import { api, can, type Principal } from "../api";
import { Badge, Empty, Field, FormError, Page, QueryState, onSubmit, useT } from "../ui";

type Site = { id: string; name: string; location: string; execution_domain: string; lifecycle: string };
type Connector = {
  id: string; site_id: string; name: string; adapter: string; health: string; lifecycle: string;
  execution_mode: string; credential_ref: string; support_status: string; last_heartbeat_at?: string | null;
};
type Device = {
  id: string; site_id: string; name: string; model: string; role: string; health: string;
  data_origin: string; lifecycle: string; support_note: string; os_version: string; management_endpoint: string;
};
type Iface = { id: string; name: string; mac: string; admin_up: boolean; oper_up: boolean; speed_mbps: number; addresses: string[] };
type Capability = { capability: string; supported: boolean; evidence: string; connector_version: string };

export function InventoryPage() {
  const t = useT();
  const { principal } = useOutletContext<{ principal: Principal }>();
  const qc = useQueryClient();
  const [params, setParams] = useSearchParams();
  const q = params.get("q") ?? "";
  const site = params.get("site") ?? "";
  const [siteForm, setSiteForm] = useState({ name: "", location: "", execution_domain: "" });
  const [connForm, setConnForm] = useState({ name: "", site_id: "", adapter: "simulator" });
  const [token, setToken] = useState("");
  const [notice, setNotice] = useState("");
  const sites = useQuery({ queryKey: ["sites"], queryFn: () => api<Site[]>("/api/v1/sites") });
  const connectors = useQuery({ queryKey: ["connectors"], queryFn: () => api<Connector[]>("/api/v1/connectors") });
  const devices = useQuery({
    queryKey: ["devices", site, q],
    queryFn: () => api<Device[]>(`/api/v1/devices?site_id=${encodeURIComponent(site)}&q=${encodeURIComponent(q)}`),
  });
  const createSite = useMutation({
    mutationFn: () => api<Site>("/api/v1/sites", { method: "POST", body: JSON.stringify(siteForm) }),
    onSuccess: () => { setSiteForm({ name: "", location: "", execution_domain: "" }); void qc.invalidateQueries({ queryKey: ["sites"] }); },
  });
  const createConn = useMutation({
    mutationFn: () => api<Connector>("/api/v1/connectors", { method: "POST", body: JSON.stringify(connForm) }),
    onSuccess: () => { setConnForm({ name: "", site_id: "", adapter: "simulator" }); void qc.invalidateQueries({ queryKey: ["connectors"] }); },
  });
  const enroll = useMutation({
    mutationFn: (id: string) => api<{ token: string; notice: string }>(`/api/v1/connectors/${id}/enrollment`, { method: "POST", body: "{}" }),
    onSuccess: (data) => { setToken(data.token); setNotice(data.notice); },
  });
  const discover = useMutation({
    mutationFn: ({ id, via }: { id: string; via: string }) => api<Record<string, unknown>>(`/api/v1/connectors/${id}/discover${via}`, { method: "POST", body: "{}" }),
    onSuccess: (data) => {
      setNotice(String(data.notice || data.detail || ""));
      void qc.invalidateQueries({ queryKey: ["devices"] });
      void qc.invalidateQueries({ queryKey: ["connectors"] });
    },
  });

  function patch(key: string, value: string) {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value);
    else next.delete(key);
    setParams(next);
  }

  return (
    <Page title={t("inventory")} crumbs={[{ to: "/", label: t("command") }, { label: t("inventory") }]}>
      <p className="banner">{t("banner")}</p>
      <div className="stack">
        <section className="panel">
          <h2>{t("sites")}</h2>
          <QueryState loading={sites.isLoading} error={sites.error} onRetry={() => sites.refetch()}>
            {(sites.data ?? []).length === 0 ? <Empty text={t("empty")} /> : (
              <table className="data">
                <caption className="note">{t("sites")}</caption>
                <thead><tr><th>{t("name")}</th><th>{t("location")}</th><th>{t("domain")}</th><th>{t("lifecycle")}</th></tr></thead>
                <tbody>
                  {(sites.data ?? []).map((item) => (
                    <tr key={item.id}><td>{item.name}</td><td>{item.location}</td><td>{item.execution_domain}</td><td><Badge value={item.lifecycle} /></td></tr>
                  ))}
                </tbody>
              </table>
            )}
          </QueryState>
          {can(principal.role, "operate") ? (
            <form className="row" style={{ marginTop: 10 }} onSubmit={onSubmit(() => createSite.mutate())}>
              <Field label={t("name")}><input required value={siteForm.name} onChange={(e) => setSiteForm({ ...siteForm, name: e.target.value })} /></Field>
              <Field label={t("location")}><input value={siteForm.location} onChange={(e) => setSiteForm({ ...siteForm, location: e.target.value })} /></Field>
              <Field label={t("domain")}><input required value={siteForm.execution_domain} onChange={(e) => setSiteForm({ ...siteForm, execution_domain: e.target.value })} /></Field>
              <button type="submit" disabled={createSite.isPending}>{t("createSite")}</button>
              <FormError error={createSite.error} />
            </form>
          ) : <p className="note">{t("noPermission")}</p>}
        </section>

        <section className="panel">
          <h2>{t("connectors")}</h2>
          <QueryState loading={connectors.isLoading} error={connectors.error} onRetry={() => connectors.refetch()}>
            <table className="data">
              <thead><tr><th>{t("name")}</th><th>{t("adapter")}</th><th>{t("health")}</th><th>{t("support")}</th><th>{t("mode")}</th><th>{t("credentialRef")}</th><th></th></tr></thead>
              <tbody>
                {(connectors.data ?? []).map((item) => (
                  <tr key={item.id}>
                    <td>{item.name}</td>
                    <td>{item.adapter}</td>
                    <td><Badge value={item.health} /></td>
                    <td><Badge value={item.support_status} /></td>
                    <td>{item.execution_mode}</td>
                    <td className="mono">{item.credential_ref}</td>
                    <td>
                      <div className="row">
                        {can(principal.role, "admin") ? <button type="button" className="secondary" onClick={() => enroll.mutate(item.id)}>{t("enroll")}</button> : null}
                        {can(principal.role, "operate") ? (
                          <>
                            <button type="button" onClick={() => discover.mutate({ id: item.id, via: "" })}>{t("discoverNow")}</button>
                            <button type="button" className="secondary" onClick={() => discover.mutate({ id: item.id, via: "?via=agent" })}>{t("queueAgent")}</button>
                          </>
                        ) : null}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {token ? <p className="banner" role="status">{t("tokenOnce")} <span className="mono">{token}</span> {notice}</p> : null}
            <FormError error={enroll.error || discover.error} />
            {notice && !token ? <p className="note" role="status">{notice}</p> : null}
          </QueryState>
          {can(principal.role, "admin") ? (
            <form className="row" style={{ marginTop: 10 }} onSubmit={onSubmit(() => createConn.mutate())}>
              <Field label={t("name")}><input required value={connForm.name} onChange={(e) => setConnForm({ ...connForm, name: e.target.value })} /></Field>
              <Field label={t("site")}>
                <select required value={connForm.site_id} onChange={(e) => setConnForm({ ...connForm, site_id: e.target.value })}>
                  <option value="">{t("site")}</option>
                  {(sites.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
                </select>
              </Field>
              <Field label={t("adapter")}>
                <select value={connForm.adapter} onChange={(e) => setConnForm({ ...connForm, adapter: e.target.value })}>
                  <option value="simulator">simulator (demo)</option>
                  <option value="ovn">ovn (implemented, lab unverified)</option>
                </select>
              </Field>
              <button type="submit" disabled={createConn.isPending}>{t("createConnector")}</button>
              <FormError error={createConn.error} />
            </form>
          ) : null}
        </section>

        <section className="panel">
          <h2>{t("devices")}</h2>
          <form className="row" onSubmit={(event) => event.preventDefault()}>
            <Field label={t("search")}><input value={q} onChange={(e) => patch("q", e.target.value)} /></Field>
            <Field label={t("site")}>
              <select value={site} onChange={(e) => patch("site", e.target.value)}>
                <option value="">{t("allSites")}</option>
                {(sites.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
              </select>
            </Field>
          </form>
          <QueryState loading={devices.isLoading} error={devices.error} onRetry={() => devices.refetch()}>
            {(devices.data ?? []).length === 0 ? <Empty text={t("emptyDevices")} /> : <DeviceTable rows={devices.data ?? []} />}
          </QueryState>
        </section>
      </div>
    </Page>
  );
}

function DeviceTable({ rows }: { rows: Device[] }) {
  const t = useT();
  const [sorting, setSorting] = useState<SortingState>([{ id: "name", desc: false }]);
  const helper = createColumnHelper<Device>();
  const columns = [
    helper.accessor("name", { header: t("name"), cell: (info) => <Link to={`/inventory/devices/${info.row.original.id}`}>{info.getValue()}</Link> }),
    helper.accessor("role", { header: t("role") }),
    helper.accessor("health", { header: t("health"), cell: (info) => <Badge value={info.getValue()} /> }),
    helper.accessor("data_origin", { header: t("origin"), cell: (info) => <Badge value={info.getValue()} /> }),
    helper.accessor("model", { header: t("model") }),
  ];
  const table = useReactTable({ data: rows, columns, state: { sorting }, onSortingChange: setSorting, getCoreRowModel: getCoreRowModel(), getSortedRowModel: getSortedRowModel() });
  return (
    <table className="data">
      <thead>
        {table.getHeaderGroups().map((group) => (
          <tr key={group.id}>
            {group.headers.map((header) => (
              <th key={header.id}>
                <button type="button" onClick={header.column.getToggleSortingHandler()}>
                  {flexRender(header.column.columnDef.header, header.getContext())}
                  {header.column.getIsSorted() === "asc" ? " ↑" : header.column.getIsSorted() === "desc" ? " ↓" : ""}
                </button>
              </th>
            ))}
          </tr>
        ))}
      </thead>
      <tbody>
        {table.getRowModel().rows.map((row) => (
          <tr key={row.id}>
            {row.getVisibleCells().map((cell) => <td key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</td>)}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export function DevicePage() {
  const t = useT();
  const { id = "" } = useParams();
  const { principal } = useOutletContext<{ principal: Principal }>();
  const [faults, setFaults] = useState({ fail_next_write: false, fail_next_rollback: false, uncertain_next_write: false, uncertain_committed: false });
  const device = useQuery({
    queryKey: ["device", id],
    queryFn: () => api<{ device: Device; interfaces: Iface[]; capabilities: Capability[]; simulator_notice: string }>(`/api/v1/devices/${id}`),
  });
  const arm = useMutation({
    mutationFn: () => api(`/api/v1/devices/${id}/demo-faults`, { method: "POST", body: JSON.stringify(faults) }),
  });
  const item = device.data?.device;
  return (
    <Page title={item?.name || t("device")} crumbs={[{ to: "/", label: t("command") }, { to: "/inventory", label: t("inventory") }, { label: item?.name || t("device") }]}>
      <QueryState loading={device.isLoading} error={device.error} onRetry={() => device.refetch()}>
        {item && device.data ? (
          <div className="stack">
            {item.data_origin === "simulator" ? <p className="banner">{device.data.simulator_notice || t("simulated")}</p> : null}
            <section className="panel">
              <h2>{t("details")}</h2>
              <table className="data">
                <tbody>
                  <tr><th>{t("model")}</th><td>{item.model}</td></tr>
                  <tr><th>{t("role")}</th><td>{item.role}</td></tr>
                  <tr><th>{t("health")}</th><td><Badge value={item.health} /></td></tr>
                  <tr><th>{t("origin")}</th><td><Badge value={item.data_origin} /></td></tr>
                  <tr><th>{t("lifecycle")}</th><td><Badge value={item.lifecycle} /></td></tr>
                  <tr><th>{t("version")}</th><td>{item.os_version || "—"}</td></tr>
                </tbody>
              </table>
            </section>
            <section className="panel">
              <h2>{t("interfaces")}</h2>
              {device.data.interfaces.length === 0 ? <Empty text={t("empty")} /> : (
                <table className="data">
                  <thead><tr><th>{t("name")}</th><th>{t("mac")}</th><th>{t("adminState")}</th><th>{t("oper")}</th><th>{t("speed")}</th></tr></thead>
                  <tbody>
                    {device.data.interfaces.map((iface) => (
                      <tr key={iface.id}>
                        <td>{iface.name}</td><td className="mono">{iface.mac}</td>
                        <td>{iface.admin_up ? t("up") : t("down")}</td>
                        <td>{iface.oper_up ? t("up") : t("down")}</td>
                        <td>{iface.speed_mbps}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </section>
            <section className="panel">
              <h2>{t("capabilities")}</h2>
              {device.data.capabilities.length === 0 ? <Empty text={t("empty")} /> : (
                <table className="data">
                  <thead><tr><th>{t("name")}</th><th>{t("supported")}</th><th>{t("evidence")}</th><th>{t("version")}</th></tr></thead>
                  <tbody>
                    {device.data.capabilities.map((cap) => (
                      <tr key={cap.capability}>
                        <td>{cap.capability}</td>
                        <td>{cap.supported ? t("yes") : t("no")}</td>
                        <td>{cap.evidence}</td>
                        <td>{cap.connector_version}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </section>
            {can(principal.role, "author") && item.data_origin === "simulator" ? (
              <section className="panel">
                <h2>{t("faults")}</h2>
                <p className="note">{t("faultHint")}</p>
                <form className="stack" onSubmit={onSubmit(() => arm.mutate())}>
                  <label className="check"><input type="checkbox" checked={faults.fail_next_write} onChange={(e) => setFaults({ ...faults, fail_next_write: e.target.checked })} />{t("failWrite")}</label>
                  <label className="check"><input type="checkbox" checked={faults.fail_next_rollback} onChange={(e) => setFaults({ ...faults, fail_next_rollback: e.target.checked })} />{t("failRollback")}</label>
                  <label className="check"><input type="checkbox" checked={faults.uncertain_next_write} onChange={(e) => setFaults({ ...faults, uncertain_next_write: e.target.checked })} />{t("uncertain")}</label>
                  <label className="check"><input type="checkbox" checked={faults.uncertain_committed} onChange={(e) => setFaults({ ...faults, uncertain_committed: e.target.checked })} />{t("uncertainCommitted")}</label>
                  <button type="submit" className="secondary">{t("applyFault")}</button>
                  <FormError error={arm.error} />
                  {arm.isSuccess ? <p className="note" role="status">{t("simulated")}</p> : null}
                </form>
              </section>
            ) : null}
          </div>
        ) : null}
      </QueryState>
    </Page>
  );
}
