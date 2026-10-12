import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link, useOutletContext, useParams } from "react-router-dom";
import { api, can, type Principal } from "../api";
import { Badge, Empty, Field, FormError, Page, QueryState, formatTime, onSubmit, useT } from "../ui";

type Connector = {
  id: string; site_id: string; name: string; adapter: string; description: string; deployment_method: string;
  status: string; version: string; last_heartbeat_at?: string | null; support_status: string; host_info?: unknown; capabilities?: string[];
};
type Site = { id: string; name: string };
type Install = { method: string; available: boolean; reason?: string; prerequisites?: string[]; command?: string; uninstall?: string };
type Job = { id: string; status: string; error: string; discovered_count: number; created_at: string };

const methods = ["docker", "linux", "compose", "helm", "aws", "windows"];

export function ConnectorsPage() {
  const t = useT();
  const { principal } = useOutletContext<{ principal: Principal }>();
  const qc = useQueryClient();
  const [step, setStep] = useState(1);
  const [form, setForm] = useState({ name: "", site_id: "", description: "", deployment_method: "docker", adapter: "site" });
  const [created, setCreated] = useState<Connector | null>(null);
  const [token, setToken] = useState("");
  const [install, setInstall] = useState<Install | null>(null);
  const sites = useQuery({ queryKey: ["sites"], queryFn: () => api<Site[]>("/api/v1/sites") });
  const connectors = useQuery({ queryKey: ["connectors"], queryFn: () => api<Connector[]>("/api/v1/connectors") });
  const detail = useQuery({
    queryKey: ["connector", created?.id],
    queryFn: () => api<{ connector: Connector }>(`/api/v1/connectors/${created?.id}`),
    enabled: Boolean(created) && step >= 6,
    refetchInterval: step === 6 ? 3000 : false,
  });
  const create = useMutation({
    mutationFn: () => api<Connector>("/api/v1/connectors", { method: "POST", body: JSON.stringify(form) }),
    onSuccess: async (item) => {
      setCreated(item);
      setStep(3);
      const guide = await api<Install>(`/api/v1/connectors/${item.id}/install?method=${form.deployment_method}`);
      setInstall(guide);
      void qc.invalidateQueries({ queryKey: ["connectors"] });
    },
  });
  const enroll = useMutation({
    mutationFn: () => api<{ token: string; notice: string }>(`/api/v1/connectors/${created?.id}/enrollment`, { method: "POST", body: "{}" }),
    onSuccess: (data) => { setToken(data.token); setStep(5); },
  });

  return (
    <Page title={t("connectors")} crumbs={[{ to: "/", label: t("command") }, { label: t("connectors") }]}>
      <div className="connector-layout">
        <section className="panel">
          <h2>{t("connectors")}</h2>
          <QueryState loading={connectors.isLoading} error={connectors.error} onRetry={() => connectors.refetch()}>
            {(connectors.data ?? []).length === 0 ? <Empty text={t("empty")} /> : (
              <table className="data">
                <thead><tr><th>{t("name")}</th><th>{t("site")}</th><th>{t("status")}</th><th>{t("health")}</th></tr></thead>
                <tbody>
                  {(connectors.data ?? []).map((item) => (
                    <tr key={item.id}>
                      <td><Link to={`/connectors/${item.id}`}>{item.name}</Link></td>
                      <td>{(sites.data ?? []).find((site) => site.id === item.site_id)?.name || item.site_id}</td>
                      <td><Badge value={item.status} /></td>
                      <td className="note">{formatTime(item.last_heartbeat_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </QueryState>
        </section>
        <section className="panel">
          <h2>{t("createConnector")}</h2>
          <ol className="wizard-steps">
            <li aria-current={step === 1 ? "step" : undefined}>{t("site")}</li>
            <li aria-current={step === 2 ? "step" : undefined}>{t("adapter")}</li>
            <li aria-current={step === 3 ? "step" : undefined}>{t("prerequisites")}</li>
            <li aria-current={step === 4 || step === 5 ? "step" : undefined}>{t("generate")}</li>
            <li aria-current={step === 6 ? "step" : undefined}>{t("waiting")}</li>
          </ol>
          {step <= 2 && can(principal.role, "admin") ? (
            <form className="stack" onSubmit={onSubmit(() => step === 1 ? setStep(2) : create.mutate())}>
              {step === 1 ? (
                <>
                  <Field label={t("name")}><input required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /></Field>
                  <Field label={t("site")}>
                    <select required value={form.site_id} onChange={(e) => setForm({ ...form, site_id: e.target.value })}>
                      <option value="" />
                      {(sites.data ?? []).map((site) => <option key={site.id} value={site.id}>{site.name}</option>)}
                    </select>
                  </Field>
                  <Field label={t("detail")}><input value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} /></Field>
                </>
              ) : (
                <div className="method-grid">
                  {methods.map((method) => (
                    <button key={method} type="button" className="secondary" aria-pressed={form.deployment_method === method} onClick={() => setForm({ ...form, deployment_method: method })}>
                      {method}
                      {method === "helm" || method === "aws" || method === "windows" ? <small>{t("unavailableMethod")}</small> : null}
                    </button>
                  ))}
                </div>
              )}
              <button type="submit" disabled={create.isPending}>{step === 1 ? t("confirm") : t("save")}</button>
              <FormError error={create.error} />
            </form>
          ) : null}
          {install ? (
            <div className="stack">
              <h2>{t("prerequisites")}</h2>
              {install.available ? (
                <ul>{(install.prerequisites ?? []).map((item) => <li key={item}>{item}</li>)}</ul>
              ) : <p className="note">{install.reason}</p>}
              {install.available && can(principal.role, "admin") ? (
                <button type="button" onClick={() => enroll.mutate()} disabled={enroll.isPending}>{t("generate")}</button>
              ) : null}
              <FormError error={enroll.error} />
              {token ? (
                <>
                  <p className="banner">{t("tokenOnce")}</p>
                  <pre className="mono">{token}</pre>
                  <button type="button" className="secondary" onClick={() => void navigator.clipboard.writeText(token)}>{t("copy")}</button>
                  {install.command ? <pre className="mono">{install.command}</pre> : null}
                  {install.uninstall ? <><h2>{t("uninstall")}</h2><pre className="mono">{install.uninstall}</pre></> : null}
                  <button type="button" onClick={() => setStep(6)}>{t("waiting")}</button>
                </>
              ) : null}
            </div>
          ) : null}
          {step === 6 && detail.data ? (
            <div>
              <p><Badge value={detail.data.connector.status} /></p>
              <p className="note">{detail.data.connector.status === "online" ? t("enrolled") : t("waiting")}</p>
              {detail.data.connector.status === "online" ? <Link to={`/discovery?connector=${detail.data.connector.id}&site=${detail.data.connector.site_id}`}>{t("discovery")}</Link> : null}
            </div>
          ) : null}
          {!can(principal.role, "admin") ? <p className="note">{t("noPermission")}</p> : null}
        </section>
      </div>
    </Page>
  );
}

export function ConnectorPage() {
  const t = useT();
  const { id = "" } = useParams();
  const { principal } = useOutletContext<{ principal: Principal }>();
  const qc = useQueryClient();
  const detail = useQuery({
    queryKey: ["connector", id],
    queryFn: () => api<{ connector: Connector; jobs: Job[]; troubleshooting: string; network: { outbound: string[]; inbound: string; note: string } }>(`/api/v1/connectors/${id}`),
    refetchInterval: 10000,
  });
  const act = useMutation({
    mutationFn: (path: string) => api(`/api/v1/connectors/${id}/${path}`, { method: "POST", body: "{}" }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["connector", id] }),
  });
  const item = detail.data?.connector;
  return (
    <Page title={item?.name || t("connectors")} crumbs={[{ to: "/", label: t("command") }, { to: "/connectors", label: t("connectors") }, { label: item?.name || "" }]}>
      <QueryState loading={detail.isLoading} error={detail.error} onRetry={() => detail.refetch()}>
        {detail.data && item ? (
          <div className="stack">
            <section className="panel">
              <h2>{item.name}</h2>
              <p><Badge value={item.status} /> · {item.deployment_method || "—"} · {item.version}</p>
              <p className="note">{t("lastSeen")} {formatTime(item.last_heartbeat_at)}</p>
              <p>{item.description}</p>
              <h2>{t("troubleshooting")}</h2>
              <p>{detail.data.troubleshooting}</p>
              <h2>{t("prerequisites")}</h2>
              <ul>{detail.data.network.outbound.map((line) => <li key={line}>{line}</li>)}</ul>
              <p className="note">{detail.data.network.inbound}. {detail.data.network.note}</p>
              {can(principal.role, "admin") ? (
                <div className="row">
                  <button type="button" className="secondary" onClick={() => act.mutate("replace")}>{t("enroll")}</button>
                  <button type="button" className="danger" onClick={() => act.mutate("revoke")}>{t("cancel")}</button>
                  <button type="button" className="secondary" onClick={() => act.mutate("remove")}>{t("close")}</button>
                </div>
              ) : null}
              <FormError error={act.error} />
            </section>
            <section className="panel">
              <h2>{t("recent")}</h2>
              {(detail.data.jobs ?? []).length === 0 ? <Empty text={t("empty")} /> : (
                <table className="data">
                  <thead><tr><th>{t("status")}</th><th>{t("devices")}</th><th>{t("errorDetail")}</th><th>{t("created")}</th></tr></thead>
                  <tbody>
                    {detail.data.jobs.map((job) => (
                      <tr key={job.id}><td><Badge value={job.status} /></td><td>{job.discovered_count}</td><td>{job.error || "—"}</td><td>{formatTime(job.created_at)}</td></tr>
                    ))}
                  </tbody>
                </table>
              )}
              <Link to={`/discovery?connector=${item.id}&site=${item.site_id}`}>{t("discovery")}</Link>
            </section>
          </div>
        ) : null}
      </QueryState>
    </Page>
  );
}
