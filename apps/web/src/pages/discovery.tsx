import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Link, useOutletContext, useSearchParams } from "react-router-dom";
import { api, can, type Principal } from "../api";
import { Badge, Empty, Field, FormError, Page, QueryState, formatTime, onSubmit, useT } from "../ui";

type Profile = { id: string; name: string; site_id: string; connector_id: string; targets: string[]; auto_import: boolean; schedule_minutes: number };
type Job = { id: string; status: string; progress: number; discovered_count: number; error: string; profile_id: string; created_at?: string };
type Observation = {
  id: string; ip: string; hostname: string; mac: string; vendor: string; model: string; device_type: string;
  serial: string; os_version: string; reachable: boolean; confidence: string; match_status: string; imported: boolean; methods: { method: string; ok: boolean }[];
};

export function DiscoveryPage() {
  const t = useT();
  const { principal } = useOutletContext<{ principal: Principal }>();
  const [params] = useSearchParams();
  const qc = useQueryClient();
  const [form, setForm] = useState({
    name: "site-scan", site_id: params.get("site") || "", connector_id: params.get("connector") || "",
    targets: "", exclusions: "", timeout_ms: 800, concurrency: 8, schedule_minutes: 0, auto_import: false, scan_profile: "reachability",
  });
  const [jobId, setJobId] = useState("");
  const [picked, setPicked] = useState<string[]>([]);
  const [q, setQ] = useState("");
  const sites = useQuery({ queryKey: ["sites"], queryFn: () => api<{ id: string; name: string }[]>("/api/v1/sites") });
  const connectors = useQuery({ queryKey: ["connectors"], queryFn: () => api<{ id: string; name: string; site_id: string; status: string }[]>("/api/v1/connectors") });
  const profiles = useQuery({ queryKey: ["profiles", form.site_id], queryFn: () => api<Profile[]>(`/api/v1/discovery-profiles?site_id=${encodeURIComponent(form.site_id)}`) });
  const jobs = useQuery({ queryKey: ["jobs", form.site_id], queryFn: () => api<Job[]>(`/api/v1/discovery-jobs?site_id=${encodeURIComponent(form.site_id)}`) });
  useEffect(() => {
    if (!jobId && jobs.data && jobs.data.length > 0) setJobId(jobs.data[0].id);
  }, [jobId, jobs.data]);
  const job = useQuery({
    queryKey: ["job", jobId],
    queryFn: () => api<Job>(`/api/v1/discovery-jobs/${jobId}`),
    enabled: Boolean(jobId),
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status === "queued" || status === "running" ? 2000 : false;
    },
  });
  const observations = useQuery({
    queryKey: ["observations", jobId, q],
    queryFn: () => api<Observation[]>(`/api/v1/discovery-jobs/${jobId}/observations?q=${encodeURIComponent(q)}`),
    enabled: Boolean(jobId) && (job.data?.status === "completed" || job.data?.status === "partial" || job.data?.status === "failed"),
  });
  const create = useMutation({
    mutationFn: () => api<Profile>(`/api/v1/sites/${form.site_id}/discovery-profiles`, {
      method: "POST",
      body: JSON.stringify({
        ...form,
        targets: form.targets.split(/[\s,]+/).filter(Boolean),
        exclusions: form.exclusions.split(/[\s,]+/).filter(Boolean),
      }),
    }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["profiles"] }),
  });
  const run = useMutation({
    mutationFn: (id: string) => api<Job>(`/api/v1/discovery-profiles/${id}/run`, { method: "POST", body: "{}" }),
    onSuccess: (item) => setJobId(item.id),
  });
  const cancel = useMutation({
    mutationFn: () => api(`/api/v1/discovery-jobs/${jobId}/cancel`, { method: "POST", body: "{}" }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["job", jobId] }),
  });
  const importObs = useMutation({
    mutationFn: (all: boolean) => api(`/api/v1/discovery-jobs/${jobId}/import`, { method: "POST", body: JSON.stringify(all ? { all: true } : { ids: picked }) }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["observations", jobId] });
      void qc.invalidateQueries({ queryKey: ["devices"] });
    },
  });

  return (
    <Page title={t("discovery")} crumbs={[{ to: "/", label: t("command") }, { label: t("discovery") }]}>
      <p className="banner">{t("banner")}</p>
      <div className="stack">
        {can(principal.role, "admin") ? (
          <form className="panel stack" onSubmit={onSubmit(() => create.mutate())}>
            <h2>{t("discovery")}</h2>
            <div className="row">
              <Field label={t("name")}><input required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /></Field>
              <Field label={t("site")}>
                <select required value={form.site_id} onChange={(e) => setForm({ ...form, site_id: e.target.value, connector_id: "" })}>
                  <option value="" />
                  {(sites.data ?? []).map((site) => <option key={site.id} value={site.id}>{site.name}</option>)}
                </select>
              </Field>
              <Field label={t("connectors")}>
                <select required value={form.connector_id} onChange={(e) => setForm({ ...form, connector_id: e.target.value })}>
                  <option value="" />
                  {(connectors.data ?? []).filter((item) => item.site_id === form.site_id).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
                </select>
              </Field>
              <Field label={t("cidr")}><input required value={form.targets} onChange={(e) => setForm({ ...form, targets: e.target.value })} placeholder="10.0.0.0/28" /></Field>
              <Field label={t("destination")}><input value={form.exclusions} onChange={(e) => setForm({ ...form, exclusions: e.target.value })} placeholder={t("unavailable")} /></Field>
            </div>
            <label className="check"><input type="checkbox" checked={form.auto_import} onChange={(e) => setForm({ ...form, auto_import: e.target.checked })} />{t("autoImport")}</label>
            <button type="submit" disabled={create.isPending}>{t("save")}</button>
            <FormError error={create.error} />
          </form>
        ) : <p className="note">{t("noPermission")}</p>}
        <section className="panel">
          <h2>{t("scan")}</h2>
          <QueryState loading={profiles.isLoading} error={profiles.error} onRetry={() => profiles.refetch()}>
            {(profiles.data ?? []).length === 0 ? <Empty text={t("empty")} /> : (
              <table className="data">
                <thead><tr><th>{t("name")}</th><th>{t("connectors")}</th><th></th></tr></thead>
                <tbody>
                  {(profiles.data ?? []).map((profile) => (
                    <tr key={profile.id}>
                      <td>{profile.name}</td>
                      <td><Link to={`/connectors/${profile.connector_id}`}>{profile.connector_id.slice(0, 8)}</Link></td>
                      <td>{can(principal.role, "operate") ? <button type="button" onClick={() => run.mutate(profile.id)}>{t("scan")}</button> : null}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </QueryState>
          <h2>{t("recent")}</h2>
          <QueryState loading={jobs.isLoading} error={jobs.error} onRetry={() => jobs.refetch()}>
            {(jobs.data ?? []).length === 0 ? <Empty text={t("empty")} /> : (
              <table className="data">
                <thead><tr><th>{t("status")}</th><th>{t("devices")}</th><th>{t("created")}</th></tr></thead>
                <tbody>
                  {(jobs.data ?? []).map((item) => (
                    <tr key={item.id}>
                      <td><button type="button" className="secondary" aria-pressed={jobId === item.id} onClick={() => setJobId(item.id)}><Badge value={item.status} /></button></td>
                      <td>{item.discovered_count}</td>
                      <td className="note">{item.created_at ? formatTime(item.created_at) : ""}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </QueryState>
          <FormError error={run.error || cancel.error || importObs.error} />
          {job.data ? (
            <div>
              <p><Badge value={job.data.status} /> {job.data.progress}% · {job.data.discovered_count}</p>
              {job.data.error ? <p className="state error" role="alert">{job.data.error}</p> : null}
              {(job.data.status === "queued" || job.data.status === "running") && can(principal.role, "operate") ? (
                <button type="button" className="secondary" onClick={() => cancel.mutate()}>{t("cancel")}</button>
              ) : null}
            </div>
          ) : null}
        </section>
        <section className="panel">
          <div className="page-title-row">
            <h2>{t("devices")}</h2>
            <input aria-label={t("search")} value={q} onChange={(e) => setQ(e.target.value)} />
          </div>
          {(observations.data ?? []).length === 0 ? <Empty text={t("empty")} /> : (
            <>
              <div className="row">
                <button type="button" disabled={!can(principal.role, "operate")} onClick={() => importObs.mutate(false)}>{t("importSelected")}</button>
                <button type="button" className="secondary" disabled={!can(principal.role, "operate")} onClick={() => importObs.mutate(true)}>{t("importAll")}</button>
              </div>
              <table className="data">
                <thead><tr><th></th><th>IP</th><th>{t("name")}</th><th>MAC</th><th>{t("model")}</th><th>{t("status")}</th><th>{t("origin")}</th></tr></thead>
                <tbody>
                  {(observations.data ?? []).map((item) => (
                    <tr key={item.id}>
                      <td><input type="checkbox" checked={picked.includes(item.id)} onChange={() => setPicked((current) => current.includes(item.id) ? current.filter((id) => id !== item.id) : [...current, item.id])} /></td>
                      <td>{item.ip}</td>
                      <td>{item.hostname || "—"}</td>
                      <td>{item.mac || "—"}</td>
                      <td>{item.model || "—"}</td>
                      <td><Badge value={item.match_status} /> {item.reachable ? "" : t("unknown")}</td>
                      <td>{(item.methods ?? []).filter((method) => method.ok).map((method) => method.method).join(", ") || "—"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </>
          )}
        </section>
      </div>
    </Page>
  );
}
