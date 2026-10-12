import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link, useNavigate, useOutletContext, useSearchParams } from "react-router-dom";
import { api, can, type Principal } from "../api";
import { confirmationMatches } from "../confirm";
import { ThemePanel } from "./theme";
import { Badge, Empty, Field, FormError, Page, QueryState, formatTime, onSubmit, useT } from "../ui";
import { useUI } from "../session";

type Fabric = { id: string; name: string; description: string; lifecycle: string };
type Tenant = { id: string; fabric_id: string; name: string; isolation_domain: string; lifecycle: string };
type Segment = { id: string; tenant_id: string; name: string; cidr: string; vlan: number | null; vni: number | null; lifecycle: string };

export function FabricPage() {
  const t = useT();
  const { principal } = useOutletContext<{ principal: Principal }>();
  const qc = useQueryClient();
  const fabrics = useQuery({ queryKey: ["fabrics"], queryFn: () => api<Fabric[]>("/api/v1/fabrics") });
  const tenants = useQuery({ queryKey: ["tenants"], queryFn: () => api<Tenant[]>("/api/v1/tenants") });
  const segments = useQuery({ queryKey: ["segments"], queryFn: () => api<Segment[]>("/api/v1/segments") });
  const [fabric, setFabric] = useState({ name: "", description: "" });
  const [tenant, setTenant] = useState({ fabric_id: "", name: "", isolation_domain: "" });
  const [segment, setSegment] = useState({ tenant_id: "", name: "", cidr: "", vlan: "", vni: "" });
  const createFabric = useMutation({
    mutationFn: () => api("/api/v1/fabrics", { method: "POST", body: JSON.stringify(fabric) }),
    onSuccess: () => { setFabric({ name: "", description: "" }); void qc.invalidateQueries({ queryKey: ["fabrics"] }); },
  });
  const createTenant = useMutation({
    mutationFn: () => api("/api/v1/tenants", { method: "POST", body: JSON.stringify(tenant) }),
    onSuccess: () => { setTenant({ fabric_id: "", name: "", isolation_domain: "" }); void qc.invalidateQueries({ queryKey: ["tenants"] }); },
  });
  const createSegment = useMutation({
    mutationFn: () => api("/api/v1/segments", {
      method: "POST",
      body: JSON.stringify({
        tenant_id: segment.tenant_id, name: segment.name, cidr: segment.cidr,
        vlan: segment.vlan ? Number(segment.vlan) : null,
        vni: segment.vni ? Number(segment.vni) : null,
      }),
    }),
    onSuccess: () => { setSegment({ tenant_id: "", name: "", cidr: "", vlan: "", vni: "" }); void qc.invalidateQueries({ queryKey: ["segments"] }); },
  });
  const author = can(principal.role, "author");
  return (
    <Page title={t("fabric")} crumbs={[{ to: "/", label: t("command") }, { label: t("fabric") }]}>
      <div className="stack">
        <section className="panel">
          <h2>{t("fabric")}</h2>
          <QueryState loading={fabrics.isLoading} error={fabrics.error} onRetry={() => fabrics.refetch()}>
            {(fabrics.data ?? []).length === 0 ? <Empty text={t("empty")} /> : (
              <table className="data">
                <thead><tr><th>{t("name")}</th><th>{t("description")}</th><th>{t("lifecycle")}</th></tr></thead>
                <tbody>{(fabrics.data ?? []).map((item) => <tr key={item.id}><td>{item.name}</td><td>{item.description}</td><td><Badge value={item.lifecycle} /></td></tr>)}</tbody>
              </table>
            )}
          </QueryState>
          {author ? (
            <form className="row" onSubmit={onSubmit(() => createFabric.mutate())}>
              <Field label={t("name")}><input required value={fabric.name} onChange={(e) => setFabric({ ...fabric, name: e.target.value })} /></Field>
              <Field label={t("description")}><input value={fabric.description} onChange={(e) => setFabric({ ...fabric, description: e.target.value })} /></Field>
              <button type="submit">{t("createFabric")}</button>
            </form>
          ) : null}
          <FormError error={createFabric.error} />
        </section>
        <section className="panel">
          <h2>{t("tenant")}</h2>
          <QueryState loading={tenants.isLoading} error={tenants.error} onRetry={() => tenants.refetch()}>
            <table className="data">
              <thead><tr><th>{t("name")}</th><th>{t("isolation")}</th><th>{t("lifecycle")}</th></tr></thead>
              <tbody>{(tenants.data ?? []).map((item) => <tr key={item.id}><td>{item.name}</td><td>{item.isolation_domain}</td><td><Badge value={item.lifecycle} /></td></tr>)}</tbody>
            </table>
          </QueryState>
          {author ? (
            <form className="row" onSubmit={onSubmit(() => createTenant.mutate())}>
              <Field label={t("fabric")}>
                <select required value={tenant.fabric_id} onChange={(e) => setTenant({ ...tenant, fabric_id: e.target.value })}>
                  <option value="" />
                  {(fabrics.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
                </select>
              </Field>
              <Field label={t("name")}><input required value={tenant.name} onChange={(e) => setTenant({ ...tenant, name: e.target.value })} /></Field>
              <Field label={t("isolation")}><input required value={tenant.isolation_domain} onChange={(e) => setTenant({ ...tenant, isolation_domain: e.target.value })} /></Field>
              <button type="submit">{t("createTenant")}</button>
            </form>
          ) : null}
          <FormError error={createTenant.error} />
        </section>
        <section className="panel">
          <h2>{t("segment")}</h2>
          <QueryState loading={segments.isLoading} error={segments.error} onRetry={() => segments.refetch()}>
            <table className="data">
              <thead><tr><th>{t("name")}</th><th>{t("cidr")}</th><th>{t("vlan")}</th><th>{t("vni")}</th></tr></thead>
              <tbody>{(segments.data ?? []).map((item) => <tr key={item.id}><td>{item.name}</td><td className="mono">{item.cidr}</td><td>{item.vlan ?? "—"}</td><td>{item.vni ?? "—"}</td></tr>)}</tbody>
            </table>
          </QueryState>
          {author ? (
            <form className="row" onSubmit={onSubmit(() => createSegment.mutate())}>
              <Field label={t("tenant")}>
                <select required value={segment.tenant_id} onChange={(e) => setSegment({ ...segment, tenant_id: e.target.value })}>
                  <option value="" />
                  {(tenants.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
                </select>
              </Field>
              <Field label={t("name")}><input required value={segment.name} onChange={(e) => setSegment({ ...segment, name: e.target.value })} /></Field>
              <Field label={t("cidr")}><input required value={segment.cidr} onChange={(e) => setSegment({ ...segment, cidr: e.target.value })} placeholder="10.40.0.0/24" /></Field>
              <Field label={t("vlan")}><input value={segment.vlan} onChange={(e) => setSegment({ ...segment, vlan: e.target.value })} /></Field>
              <Field label={t("vni")}><input value={segment.vni} onChange={(e) => setSegment({ ...segment, vni: e.target.value })} /></Field>
              <button type="submit">{t("createSegment")}</button>
            </form>
          ) : null}
          <FormError error={createSegment.error} />
        </section>
      </div>
    </Page>
  );
}

type Controllers = {
  coordination: string; etcd: string; write_authority: string; notice: string;
  orchestration: string; temporal: string; secret_backend: string;
  epochs: { execution_domain: string; epoch: number; holder: string; updated_at: string }[];
  workers: { id?: string; kind?: string; beat_at?: string }[];
};

export function ControllersPage() {
  const t = useT();
  const status = useQuery({ queryKey: ["controllers"], queryFn: () => api<Controllers>("/api/v1/controllers/status") });
  return (
    <Page title={t("controllers")} crumbs={[{ to: "/", label: t("command") }, { label: t("controllers") }]}>
      <QueryState loading={status.isLoading} error={status.error} onRetry={() => status.refetch()}>
        {status.data ? (
          <div className="stack">
            <p className="banner">{status.data.notice}</p>
            <div className="cards status-cards">
              <div className="card"><strong>{status.data.coordination}</strong><span>{t("coordination")}</span></div>
              <div className="card"><strong>{status.data.etcd}</strong><span>{t("etcd")}</span></div>
              <div className="card"><strong>{status.data.write_authority}</strong><span>{t("domain")}</span></div>
              <div className="card"><strong>{status.data.orchestration}</strong><span>{t("orchestration")}</span></div>
              <div className="card"><strong>{status.data.temporal}</strong><span>{t("temporal")}</span></div>
              <div className="card"><strong>{status.data.secret_backend}</strong><span>{t("secretBackend")}</span></div>
            </div>
            <section className="panel">
              <h2>{t("epochs")}</h2>
              {status.data.epochs.length === 0 ? <Empty text={t("empty")} /> : (
                <table className="data">
                  <thead><tr><th>{t("domain")}</th><th>{t("revision")}</th><th>{t("current")}</th><th>{t("when")}</th></tr></thead>
                  <tbody>{status.data.epochs.map((row) => <tr key={row.execution_domain}><td>{row.execution_domain}</td><td>{row.epoch}</td><td>{row.holder || "—"}</td><td>{formatTime(row.updated_at)}</td></tr>)}</tbody>
                </table>
              )}
            </section>
            <section className="panel">
              <h2>{t("workers")}</h2>
              {status.data.workers.length === 0 ? <Empty text={t("empty")} /> : (
                <table className="data">
                  <thead><tr><th>{t("name")}</th><th>{t("kind")}</th><th>{t("lastSeen")}</th></tr></thead>
                  <tbody>{status.data.workers.map((row, index) => <tr key={row.id || index}><td>{row.id}</td><td>{row.kind}</td><td>{formatTime(row.beat_at)}</td></tr>)}</tbody>
                </table>
              )}
            </section>
          </div>
        ) : null}
      </QueryState>
    </Page>
  );
}

export function DesiredPage() {
  const t = useT();
  const desired = useQuery({ queryKey: ["desired"], queryFn: () => api<{ desired: unknown; revision: number; scope: string }>("/api/v1/desired-states") });
  const observed = useQuery({ queryKey: ["observed"], queryFn: () => api<{ device_id: string; device_name: string; freshness: string; collector: string; observed_at: string; document: unknown }[]>("/api/v1/observed-states") });
  return (
    <Page title={t("desired")} crumbs={[{ to: "/", label: t("command") }, { label: t("desired") }]} actions={<Link to="/assurance">{t("drift")}</Link>}>
      <div className="stack">
        <section className="panel">
          <h2>{t("desiredDoc")}</h2>
          <QueryState loading={desired.isLoading} error={desired.error} onRetry={() => desired.refetch()}>
            {desired.data ? (
              <>
                <p className="note">{t("scope")} {desired.data.scope} · {t("revision")} {desired.data.revision}</p>
                <pre className="mono">{JSON.stringify(desired.data.desired, null, 2)}</pre>
              </>
            ) : <Empty text={t("empty")} />}
          </QueryState>
        </section>
        <section className="panel">
          <h2>{t("observed")}</h2>
          <QueryState loading={observed.isLoading} error={observed.error} onRetry={() => observed.refetch()}>
            {(observed.data ?? []).length === 0 ? <Empty text={t("empty")} /> : (
              <table className="data">
                <thead><tr><th>{t("device")}</th><th>{t("freshness")}</th><th>{t("collector")}</th><th>{t("when")}</th></tr></thead>
                <tbody>
                  {(observed.data ?? []).map((row) => (
                    <tr key={row.device_id}>
                      <td><Link to={`/inventory/devices/${row.device_id}`}>{row.device_name}</Link></td>
                      <td><Badge value={row.freshness} /></td>
                      <td>{row.collector}</td>
                      <td>{formatTime(row.observed_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </QueryState>
        </section>
      </div>
    </Page>
  );
}

type Drift = { id: string; device_id: string; device_name: string; difference: unknown; disposition: string; created_at: string };

export function AssurancePage() {
  const t = useT();
  const { principal } = useOutletContext<{ principal: Principal }>();
  const qc = useQueryClient();
  const drift = useQuery({ queryKey: ["drift"], queryFn: () => api<Drift[]>("/api/v1/drift-findings") });
  const verifications = useQuery({ queryKey: ["verifications"], queryFn: () => api<{ id: string; scope: string; result: string; evidence: string; created_at: string }[]>("/api/v1/verifications") });
  const scan = useMutation({
    mutationFn: () => api<Drift[]>("/api/v1/drift-findings/scan", { method: "POST", body: "{}" }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["drift"] }),
  });
  return (
    <Page title={t("assurance")} crumbs={[{ to: "/", label: t("command") }, { label: t("assurance") }]} actions={can(principal.role, "operate") ? <button type="button" onClick={() => scan.mutate()}>{t("scan")}</button> : undefined}>
      <div className="stack">
        <p className="banner">{t("compare")}. {t("simulated")}</p>
        <FormError error={scan.error} />
        <section className="panel">
          <h2>{t("drift")}</h2>
          <QueryState loading={drift.isLoading} error={drift.error} onRetry={() => drift.refetch()}>
            {(drift.data ?? []).length === 0 ? <Empty text={t("emptyDrift")} /> : (
              <table className="data">
                <thead><tr><th>{t("device")}</th><th>{t("disposition")}</th><th>{t("difference")}</th><th>{t("when")}</th></tr></thead>
                <tbody>
                  {(drift.data ?? []).map((row) => (
                    <tr key={row.id}>
                      <td><Link to={`/inventory/devices/${row.device_id}`}>{row.device_name}</Link></td>
                      <td><Badge value={row.disposition} /></td>
                      <td className="mono">{JSON.stringify(row.difference)}</td>
                      <td>{formatTime(row.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </QueryState>
        </section>
        <section className="panel">
          <h2>{t("verifications")}</h2>
          <QueryState loading={verifications.isLoading} error={verifications.error} onRetry={() => verifications.refetch()}>
            {(verifications.data ?? []).length === 0 ? <Empty text={t("empty")} /> : (
              <table className="data">
                <thead><tr><th>{t("scope")}</th><th>{t("result")}</th><th>{t("evidence")}</th><th>{t("when")}</th></tr></thead>
                <tbody>{(verifications.data ?? []).map((row) => <tr key={row.id}><td>{row.scope}</td><td><Badge value={row.result} /></td><td>{row.evidence}</td><td>{formatTime(row.created_at)}</td></tr>)}</tbody>
              </table>
            )}
          </QueryState>
        </section>
      </div>
    </Page>
  );
}

export function IntegrationsPage() {
  const t = useT();
  const items = useQuery({ queryKey: ["integrations"], queryFn: () => api<{ kind: string; name: string; status: string; detail: string }[]>("/api/v1/integrations") });
  return (
    <Page title={t("integrations")} crumbs={[{ to: "/", label: t("command") }, { label: t("integrations") }]}>
      <section className="panel">
        <h2>{t("catalog")}</h2>
        <QueryState loading={items.isLoading} error={items.error} onRetry={() => items.refetch()}>
          <table className="data">
            <thead><tr><th>{t("kind")}</th><th>{t("name")}</th><th>{t("status")}</th><th>{t("detail")}</th></tr></thead>
            <tbody>
              {(items.data ?? []).map((item) => (
                <tr key={item.kind}><td>{item.kind}</td><td>{item.name}</td><td><Badge value={item.status} /></td><td>{item.detail}</td></tr>
              ))}
            </tbody>
          </table>
        </QueryState>
      </section>
    </Page>
  );
}

export function AuditPage() {
  const t = useT();
  const [params, setParams] = useSearchParams();
  const action = params.get("action") ?? "";
  const events = useQuery({
    queryKey: ["audit", action],
    queryFn: () => api<{ id: string; action: string; actor_user_id?: string; resource_type: string; request_id: string; created_at: string }[]>(`/api/v1/audit-events?action=${encodeURIComponent(action)}`),
  });
  return (
    <Page title={t("audit")} crumbs={[{ to: "/", label: t("command") }, { label: t("audit") }]}>
      <form className="row" onSubmit={(event) => event.preventDefault()}>
        <Field label={t("filterAction")}>
          <input value={action} onChange={(e) => {
            const next = new URLSearchParams(params);
            if (e.target.value) next.set("action", e.target.value);
            else next.delete("action");
            setParams(next);
          }} />
        </Field>
      </form>
      <section className="panel">
        <QueryState loading={events.isLoading} error={events.error} onRetry={() => events.refetch()}>
          {(events.data ?? []).length === 0 ? <Empty text={t("emptyAudit")} /> : (
            <table className="data">
              <thead><tr><th>{t("when")}</th><th>{t("action")}</th><th>{t("resource")}</th><th>{t("actor")}</th></tr></thead>
              <tbody>
                {(events.data ?? []).map((row) => (
                  <tr key={row.id}><td>{formatTime(row.created_at)}</td><td>{row.action}</td><td>{row.resource_type}</td><td className="mono">{row.actor_user_id || "—"}</td></tr>
                ))}
              </tbody>
            </table>
          )}
        </QueryState>
      </section>
    </Page>
  );
}

type Backup = { id: string; scope: string; checksum: string; integrity: string; created_at: string };
type Restore = { id: string; backup_id: string; status: string; detail: string; created_at: string };
type Member = { user_id: string; email: string; display_name: string; role: string };

export function SettingsPage() {
  const t = useT();
  const nav = useNavigate();
  const qc = useQueryClient();
  const { principal } = useOutletContext<{ principal: Principal }>();
  const locale = useUI((s) => s.locale);
  const setLocale = useUI((s) => s.setLocale);
  const [member, setMember] = useState({ email: "", display_name: "", role: "viewer" });
  const [restoreFor, setRestoreFor] = useState("");
  const [typed, setTyped] = useState("");
  const config = useQuery({ queryKey: ["auth-config"], queryFn: () => api<{ auth_mode: string; issuer: string; client_id: string }>("/api/v1/auth/config") });
  const members = useQuery({ queryKey: ["members"], queryFn: () => api<Member[]>("/api/v1/memberships") });
  const backups = useQuery({ queryKey: ["backups"], queryFn: () => api<Backup[]>("/api/v1/backups") });
  const restores = useQuery({ queryKey: ["restores"], queryFn: () => api<Restore[]>("/api/v1/restores") });
  const add = useMutation({
    mutationFn: () => api("/api/v1/memberships", { method: "POST", body: JSON.stringify(member) }),
    onSuccess: () => { setMember({ email: "", display_name: "", role: "viewer" }); void qc.invalidateQueries({ queryKey: ["members"] }); },
  });
  const backup = useMutation({
    mutationFn: () => api("/api/v1/backups", { method: "POST", body: JSON.stringify({ scope: "organization" }) }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["backups"] }),
  });
  const restore = useMutation({
    mutationFn: (id: string) => api(`/api/v1/backups/${id}/restore`, { method: "POST", body: "{}" }),
    onSuccess: () => { setRestoreFor(""); setTyped(""); void qc.invalidateQueries({ queryKey: ["restores"] }); },
  });
  const retention = useMutation({ mutationFn: () => api<{ audit_events_removed: number; observed_states_removed: number; backups_removed: number }>("/api/v1/maintenance/retention", { method: "POST", body: "{}" }) });
  const switchOrg = useMutation({
    mutationFn: (organization_id: string) => api("/api/v1/auth/switch-organization", { method: "POST", body: JSON.stringify({ organization_id }) }),
    onSuccess: async () => { await qc.invalidateQueries(); nav("/"); },
  });
  const chosen = (backups.data ?? []).find((item) => item.id === restoreFor);
  return (
    <Page title={t("settings")} crumbs={[{ to: "/", label: t("command") }, { label: t("settings") }]}>
      <div className="stack">
        <section className="panel">
          <h2>{t("language")}</h2>
          <div className="lenses">
            <button type="button" className="secondary" aria-pressed={locale === "en"} onClick={() => setLocale("en")}>{t("english")}</button>
            <button type="button" className="secondary" aria-pressed={locale === "ar"} onClick={() => setLocale("ar")}>{t("arabic")}</button>
          </div>
          <p className="note">{t("authMode")}: {config.data?.auth_mode || "—"} · {config.data?.issuer}</p>
        </section>
        <ThemePanel role={principal.role} />
        <section className="panel">
          <h2>{t("organization")}</h2>
          <p>{principal.organization.organization_name} · <Badge value={principal.role} /></p>
          {principal.memberships.length > 1 ? (
            <div className="row">
              {principal.memberships.map((item) => (
                <button key={item.organization_id} type="button" className="secondary" disabled={item.organization_id === principal.organization.organization_id} onClick={() => switchOrg.mutate(item.organization_id)}>
                  {t("switchOrg")}: {item.organization_name}
                </button>
              ))}
            </div>
          ) : null}
          <FormError error={switchOrg.error} />
        </section>
        <section className="panel">
          <h2>{t("members")}</h2>
          <QueryState loading={members.isLoading} error={members.error} onRetry={() => members.refetch()}>
            <table className="data">
              <thead><tr><th>{t("email")}</th><th>{t("displayName")}</th><th>{t("role")}</th></tr></thead>
              <tbody>{(members.data ?? []).map((item) => <tr key={item.user_id}><td>{item.email}</td><td>{item.display_name}</td><td><Badge value={item.role} /></td></tr>)}</tbody>
            </table>
          </QueryState>
          {can(principal.role, "admin") ? (
            <form className="row" onSubmit={onSubmit(() => add.mutate())}>
              <Field label={t("email")}><input type="email" required value={member.email} onChange={(e) => setMember({ ...member, email: e.target.value })} /></Field>
              <Field label={t("displayName")}><input required value={member.display_name} onChange={(e) => setMember({ ...member, display_name: e.target.value })} /></Field>
              <Field label={t("role")}>
                <select value={member.role} onChange={(e) => setMember({ ...member, role: e.target.value })}>
                  {["viewer", "operator", "engineer", "approver", "administrator", "recovery_operator"].map((role) => <option key={role}>{role}</option>)}
                </select>
              </Field>
              <button type="submit">{t("addMember")}</button>
            </form>
          ) : null}
          <FormError error={add.error} />
          {can(principal.role, "admin") ? (
            <button type="button" className="secondary" onClick={() => retention.mutate()}>{t("retention")}</button>
          ) : null}
          {retention.data ? <p className="note" role="status">{t("audit")} {retention.data.audit_events_removed} · {t("observed")} {retention.data.observed_states_removed} · {t("backups")} {retention.data.backups_removed}</p> : null}
          <FormError error={retention.error} />
        </section>
        <section className="panel">
          <h2>{t("recovery")}</h2>
          <p className="note">{t("restoreHint")}</p>
          {can(principal.role, "operate") ? <button type="button" onClick={() => backup.mutate()}>{t("createBackup")}</button> : null}
          <FormError error={backup.error} />
          <QueryState loading={backups.isLoading} error={backups.error} onRetry={() => backups.refetch()}>
            {(backups.data ?? []).length === 0 ? <Empty text={t("emptyBackups")} /> : (
              <table className="data">
                <thead><tr><th>{t("when")}</th><th>{t("scope")}</th><th>{t("checksum")}</th><th>{t("integrity")}</th><th></th></tr></thead>
                <tbody>
                  {(backups.data ?? []).map((item) => (
                    <tr key={item.id}>
                      <td>{formatTime(item.created_at)}</td>
                      <td>{item.scope}</td>
                      <td className="mono">{item.checksum.slice(0, 12)}</td>
                      <td><Badge value={item.integrity} /></td>
                      <td>{can(principal.role, "restore") ? <button type="button" className="secondary" onClick={() => { setRestoreFor(item.id); setTyped(""); }}>{t("restore")}</button> : null}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </QueryState>
          {chosen && can(principal.role, "restore") ? (
            <form className="stack" onSubmit={onSubmit(() => restore.mutate(chosen.id))}>
              <Field label={t("typeChecksum")}><input value={typed} onChange={(e) => setTyped(e.target.value)} autoComplete="off" /></Field>
              <p className="note">{chosen.checksum.slice(0, 8)}</p>
              <button type="submit" disabled={!confirmationMatches(chosen.checksum, typed) || restore.isPending}>{t("restore")}</button>
            </form>
          ) : null}
          <FormError error={restore.error} />
          <h2 style={{ marginTop: 12 }}>{t("restore")}</h2>
          {(restores.data ?? []).length === 0 ? <Empty text={t("empty")} /> : (
            <table className="data">
              <thead><tr><th>{t("when")}</th><th>{t("status")}</th><th>{t("detail")}</th></tr></thead>
              <tbody>{(restores.data ?? []).map((item) => <tr key={item.id}><td>{formatTime(item.created_at)}</td><td><Badge value={item.status} /></td><td>{item.detail}</td></tr>)}</tbody>
            </table>
          )}
        </section>
      </div>
    </Page>
  );
}
