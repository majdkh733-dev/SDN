import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link, useLocation, useOutletContext, useSearchParams } from "react-router-dom";
import { api, can, type Principal } from "../api";
import { Badge, Empty, Field, FormError, Page, QueryState, onSubmit, useT } from "../ui";

type Adapter = { adapter: string; status: string; note: string; config_read: boolean; config_write: boolean; monitoring: boolean; discovery: boolean };

function usePrincipal() {
  return useOutletContext<{ principal: Principal }>().principal;
}

export function InterfacesPage() {
  const t = useT();
  const rows = useQuery({ queryKey: ["interfaces"], queryFn: () => api<{ id: string; device: string; name: string; mac: string; speed_mbps: number; oper_up: boolean }[]>("/api/v1/interfaces") });
  return (
    <Page title={t("interfaces")} crumbs={[{ to: "/", label: t("command") }, { label: t("interfaces") }]}>
      <QueryState loading={rows.isLoading} error={rows.error} onRetry={() => rows.refetch()}>
        {(rows.data ?? []).length === 0 ? <Empty text={t("empty")} /> : (
          <table className="data">
            <thead><tr><th>{t("devices")}</th><th>{t("name")}</th><th>MAC</th><th>{t("status")}</th></tr></thead>
            <tbody>{(rows.data ?? []).map((item) => <tr key={item.id}><td>{item.device}</td><td>{item.name}</td><td>{item.mac || "—"}</td><td>{item.oper_up ? t("up") : t("down")}</td></tr>)}</tbody>
          </table>
        )}
      </QueryState>
    </Page>
  );
}

export function EndpointsPage() {
  const t = useT();
  const rows = useQuery({ queryKey: ["endpoints"], queryFn: () => api<{ ip: string; mac: string; hostname: string; confidence: string }[]>("/api/v1/endpoints") });
  return (
    <Page title={t("endpoints")} crumbs={[{ to: "/", label: t("command") }, { label: t("endpoints") }]}>
      <p className="note">{t("labOnly")}</p>
      <QueryState loading={rows.isLoading} error={rows.error} onRetry={() => rows.refetch()}>
        {(rows.data ?? []).length === 0 ? <Empty text={t("empty")} /> : (
          <table className="data">
            <thead><tr><th>IP</th><th>MAC</th><th>{t("name")}</th><th>{t("origin")}</th></tr></thead>
            <tbody>{(rows.data ?? []).map((item) => <tr key={item.mac + item.ip}><td>{item.ip}</td><td>{item.mac}</td><td>{item.hostname || "—"}</td><td>{item.confidence}</td></tr>)}</tbody>
          </table>
        )}
      </QueryState>
    </Page>
  );
}

export function VlansPage() {
  const t = useT();
  const rows = useQuery({ queryKey: ["vlans"], queryFn: () => api<{ id: string; vid: number; name: string; status: string }[]>("/api/v1/vlans") });
  return (
    <Page title={t("vlans")} crumbs={[{ to: "/", label: t("command") }, { label: t("vlans") }]}>
      <p className="note">VLAN ids may overlap across sites. A row is intended until the simulator adapter verifies it.</p>
      <QueryState loading={rows.isLoading} error={rows.error} onRetry={() => rows.refetch()}>
        {(rows.data ?? []).length === 0 ? <Empty text={t("empty")} /> : (
          <table className="data">
            <thead><tr><th>VLAN</th><th>{t("name")}</th><th>{t("status")}</th></tr></thead>
            <tbody>{(rows.data ?? []).map((item) => <tr key={item.id}><td>{item.vid}</td><td>{item.name}</td><td><Badge value={item.status} /></td></tr>)}</tbody>
          </table>
        )}
      </QueryState>
    </Page>
  );
}

export function SubnetsPage() {
  const t = useT();
  const rows = useQuery({ queryKey: ["segments"], queryFn: () => api<{ id: string; name: string; cidr: string; vlan: number | null }[]>("/api/v1/segments") });
  return (
    <Page title={t("subnets")} crumbs={[{ to: "/", label: t("command") }, { label: t("subnets") }]}>
      <p className="note">Prefixes come from network tenants. The same private prefix may exist in another tenant or site.</p>
      <QueryState loading={rows.isLoading} error={rows.error} onRetry={() => rows.refetch()}>
        {(rows.data ?? []).length === 0 ? <Empty text={t("empty")} /> : (
          <table className="data">
            <thead><tr><th>{t("name")}</th><th>CIDR</th><th>VLAN</th></tr></thead>
            <tbody>{(rows.data ?? []).map((item) => <tr key={item.id}><td>{item.name}</td><td>{item.cidr}</td><td>{item.vlan ?? "—"}</td></tr>)}</tbody>
          </table>
        )}
      </QueryState>
    </Page>
  );
}

export function GroupsPage() {
  const t = useT();
  const rows = useQuery({ queryKey: ["segments", "groups"], queryFn: () => api<{ id: string; name: string; tenant_id: string }[]>("/api/v1/segments") });
  return (
    <Page title={t("groups")} crumbs={[{ to: "/", label: t("command") }, { label: t("groups") }]}>
      <p className="note">Policy groups are the existing segments. Free-form tags are not stored yet.</p>
      <QueryState loading={rows.isLoading} error={rows.error} onRetry={() => rows.refetch()}>
        {(rows.data ?? []).length === 0 ? <Empty text={t("empty")} /> : (
          <ul>{(rows.data ?? []).map((item) => <li key={item.id}>{item.name}</li>)}</ul>
        )}
      </QueryState>
    </Page>
  );
}

export function UnsupportedPage({ title, reason }: { title: string; reason: string }) {
  const t = useT();
  return (
    <Page title={title} crumbs={[{ to: "/", label: t("command") }, { label: title }]}>
      <p className="state" role="status">{reason}</p>
      <p className="note"><Link to="/diagnostics">{t("adapterMatrix")}</Link></p>
    </Page>
  );
}

export function AdapterTable({ rows }: { rows: Adapter[] }) {
  if (rows.length === 0) return null;
  return (
    <table className="data">
      <thead><tr><th>Adapter</th><th>Status</th><th>Discover</th><th>Monitor</th><th>Read</th><th>Write</th><th>Note</th></tr></thead>
      <tbody>
        {rows.map((item) => (
          <tr key={item.adapter}>
            <td>{item.adapter}</td>
            <td><Badge value={item.status} /></td>
            <td>{item.discovery ? "yes" : "no"}</td>
            <td>{item.monitoring ? "yes" : "no"}</td>
            <td>{item.config_read ? "yes" : "no"}</td>
            <td>{item.config_write ? "yes" : "no"}</td>
            <td className="note">{item.note}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export function ProvisionPage() {
  const t = useT();
  const principal = usePrincipal();
  const [params] = useSearchParams();
  const qc = useQueryClient();
  const sites = useQuery({ queryKey: ["sites"], queryFn: () => api<{ id: string; name: string }[]>("/api/v1/sites") });
  const devices = useQuery({ queryKey: ["devices"], queryFn: () => api<{ id: string; name: string; site_id: string; data_origin: string }[]>("/api/v1/devices") });
  const tenants = useQuery({ queryKey: ["tenants"], queryFn: () => api<{ id: string; name: string }[]>("/api/v1/tenants") });
  const [form, setForm] = useState({
    name: "vlan-change", tenant_id: "", site_id: params.get("site") || "", device_id: params.get("device") || "", vid: "120", vlan_name: "users", interface: "",
  });
  const [result, setResult] = useState<{ change_id: string; generated_config: string; plan_hash: string; note: string } | null>(null);
  const propose = useMutation({
    mutationFn: () => api<{ change_id: string; generated_config: string; plan_hash: string; note: string }>("/api/v1/provisioning/vlan", { method: "POST", body: JSON.stringify({ ...form, vid: Number(form.vid) }) }),
    onSuccess: (item) => { setResult(item); void qc.invalidateQueries({ queryKey: ["vlans"] }); },
  });
  return (
    <Page title={t("templates")} crumbs={[{ to: "/", label: t("command") }, { label: t("templates") }]}>
      <p className="note">{t("labOnly")} {t("unsupportedAdapter")}</p>
      {can(principal.role, "author") ? (
        <form className="stack" onSubmit={onSubmit(() => propose.mutate())}>
          <Field label={t("name")}><input required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /></Field>
          <Field label={t("tenant")}>
            <select required value={form.tenant_id} onChange={(e) => setForm({ ...form, tenant_id: e.target.value })}>
              <option value="" />
              {(tenants.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
            </select>
          </Field>
          <Field label={t("site")}>
            <select required value={form.site_id} onChange={(e) => setForm({ ...form, site_id: e.target.value, device_id: "" })}>
              <option value="" />
              {(sites.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
            </select>
          </Field>
          <Field label={t("devices")}>
            <select required value={form.device_id} onChange={(e) => setForm({ ...form, device_id: e.target.value })}>
              <option value="" />
              {(devices.data ?? []).filter((item) => item.site_id === form.site_id).map((item) => <option key={item.id} value={item.id}>{item.name} ({item.data_origin})</option>)}
            </select>
          </Field>
          <Field label="VLAN"><input required value={form.vid} onChange={(e) => setForm({ ...form, vid: e.target.value })} /></Field>
          <Field label={t("name")}><input required value={form.vlan_name} onChange={(e) => setForm({ ...form, vlan_name: e.target.value })} /></Field>
          <button type="submit" disabled={propose.isPending}>{t("preview")}</button>
          <FormError error={propose.error} />
        </form>
      ) : <p className="note">{t("noPermission")}</p>}
      {result ? (
        <section className="panel">
          <p className="note">{result.note}</p>
          <pre className="mono">{result.generated_config}</pre>
          <Link to={`/changes/${result.change_id}`}>{t("changes")}</Link>
        </section>
      ) : null}
    </Page>
  );
}

export function DriftPage() {
  const t = useT();
  const location = useLocation();
  const backups = location.pathname.startsWith("/backups");
  const title = backups ? t("backups") : t("drift");
  const findings = useQuery({ queryKey: ["drift"], queryFn: () => api<{ id: string; disposition: string; difference: { detail?: string; kind?: string } }[]>("/api/v1/drift-findings") });
  const configs = useQuery({ queryKey: ["configs"], queryFn: () => api<{ id: string; role: string; checksum: string; body: string; source: string }[]>("/api/v1/config-revisions") });
  return (
    <Page title={title} crumbs={[{ to: "/", label: t("command") }, { label: title }]}>
      <p className="note">Intended configuration is the baseline saved after a verified change. Observed configuration is the latest collection. A difference is drift only after both exist.</p>
      <h2>{t("drift")}</h2>
      <QueryState loading={findings.isLoading} error={findings.error} onRetry={() => findings.refetch()}>
        {(findings.data ?? []).length === 0 ? <Empty text={t("empty")} /> : (
          <ul>{(findings.data ?? []).map((item) => <li key={item.id}><Badge value={item.disposition} /> {item.difference?.detail || item.difference?.kind || ""}</li>)}</ul>
        )}
      </QueryState>
      <h2>{t("backups")}</h2>
      {(configs.data ?? []).slice(0, 6).map((item) => (
        <details key={item.id}><summary>{item.role} · {item.source} · {item.checksum.slice(0, 12)}</summary><pre className="mono">{item.body}</pre></details>
      ))}
    </Page>
  );
}

export function DiagnosticsPage() {
  const t = useT();
  const principal = usePrincipal();
  const connectors = useQuery({ queryKey: ["connectors"], queryFn: () => api<{ id: string; name: string }[]>("/api/v1/connectors") });
  const adapters = useQuery({ queryKey: ["adapters"], queryFn: () => api<Adapter[]>("/api/v1/adapters") });
  const [form, setForm] = useState({ connector_id: "", kind: "ping", target: "" });
  const [notice, setNotice] = useState("");
  const run = useMutation({
    mutationFn: () => api<{ notice: string }>("/api/v1/diagnostics", { method: "POST", body: JSON.stringify(form) }),
    onSuccess: (item) => setNotice(item.notice),
  });
  return (
    <Page title={t("diagnostics")} crumbs={[{ to: "/", label: t("command") }, { label: t("diagnostics") }]}>
      <p className="note">Ping and DNS run on the selected connector inside the site. The cloud API does not probe the address.</p>
      {can(principal.role, "operate") ? (
        <form className="stack" onSubmit={onSubmit(() => run.mutate())}>
          <Field label={t("connectors")}>
            <select required value={form.connector_id} onChange={(e) => setForm({ ...form, connector_id: e.target.value })}>
              <option value="" />
              {(connectors.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
            </select>
          </Field>
          <Field label={t("kind")}>
            <select value={form.kind} onChange={(e) => setForm({ ...form, kind: e.target.value })}>
              <option value="ping">ping</option>
              <option value="dns">dns</option>
            </select>
          </Field>
          <Field label={t("address")}><input required value={form.target} onChange={(e) => setForm({ ...form, target: e.target.value })} /></Field>
          <button type="submit">{t("run")}</button>
          <FormError error={run.error} />
          {notice ? <p className="note">{notice}</p> : null}
        </form>
      ) : null}
      <h2>{t("diagnostics")}</h2>
      <p className="note">{t("adapterMatrix")}</p>
      <QueryState loading={adapters.isLoading} error={adapters.error} onRetry={() => adapters.refetch()}>
        <AdapterTable rows={adapters.data ?? []} />
      </QueryState>
    </Page>
  );
}

export function ReportsPage() {
  const t = useT();
  const report = useQuery({ queryKey: ["availability"], queryFn: () => api<{ devices: number; healthy: number; rule: string; history: string }>("/api/v1/reports/availability") });
  return (
    <Page title={t("reports")} crumbs={[{ to: "/", label: t("command") }, { label: t("reports") }]}>
      <QueryState loading={report.isLoading} error={report.error} onRetry={() => report.refetch()}>
        {report.data ? (
          <section className="panel">
            <h2>{t("availability")}</h2>
            <p>{report.data.healthy} / {report.data.devices}</p>
            <p className="note">{report.data.rule}</p>
            <p className="note">{report.data.history}</p>
          </section>
        ) : null}
      </QueryState>
      <p className="note">Firmware end-of-support and vulnerability data are not integrated. No dates are invented.</p>
    </Page>
  );
}

export function CredentialsPage() {
  const t = useT();
  const principal = usePrincipal();
  const qc = useQueryClient();
  const rows = useQuery({ queryKey: ["service-accounts"], queryFn: () => api<{ id: string; name: string; role: string; revoked_at?: string }[]>("/api/v1/service-accounts"), enabled: can(principal.role, "admin") });
  const [form, setForm] = useState({ name: "", role: "viewer" });
  const [token, setToken] = useState("");
  const create = useMutation({
    mutationFn: () => api<{ token: string }>("/api/v1/service-accounts", { method: "POST", body: JSON.stringify(form) }),
    onSuccess: async (item) => { setToken(item.token); await qc.invalidateQueries({ queryKey: ["service-accounts"] }); },
  });
  return (
    <Page title={t("credentials")} crumbs={[{ to: "/", label: t("command") }, { label: t("credentials") }]}>
      <p className="note">The token is shown once and stored only as a hash. Revocation disables it immediately.</p>
      {!can(principal.role, "admin") ? <p className="note">{t("noPermission")}</p> : (
        <>
          <form className="row" onSubmit={onSubmit(() => create.mutate())}>
            <Field label={t("name")}><input required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /></Field>
            <Field label={t("role")}>
              <select value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value })}>
                {["viewer", "operator", "engineer", "approver", "administrator", "recovery_operator"].map((role) => <option key={role}>{role}</option>)}
              </select>
            </Field>
            <button type="submit">{t("create")}</button>
          </form>
          {token ? <p className="mono">{token}</p> : null}
          <FormError error={create.error} />
          <ul>{(rows.data ?? []).map((item) => <li key={item.id}>{item.name} · {item.role} {item.revoked_at ? t("revoked") : ""}</li>)}</ul>
        </>
      )}
    </Page>
  );
}

export function IncidentsPage() {
  const t = useT();
  const rows = useQuery({ queryKey: ["incidents"], queryFn: () => api<{ id: string; severity: string; title: string; detail: string; href: string }[]>("/api/v1/command-room/incidents") });
  return (
    <Page title={t("incidents")} crumbs={[{ to: "/", label: t("command") }, { label: t("incidents") }]}>
      <QueryState loading={rows.isLoading} error={rows.error} onRetry={() => rows.refetch()}>
        {(rows.data ?? []).length === 0 ? <Empty text={t("empty")} /> : (
          <ul>{(rows.data ?? []).map((item) => <li key={item.id}><Badge value={item.severity} /> <Link to={item.href}>{item.title}</Link> <span className="note">{item.detail}</span></li>)}</ul>
        )}
      </QueryState>
    </Page>
  );
}
