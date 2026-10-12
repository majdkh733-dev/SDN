import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Link, useNavigate, useOutletContext, useSearchParams } from "react-router-dom";
import { api, can, type Principal } from "../api";
import { Badge, Empty, Field, FormError, Page, QueryState, formatTime, onSubmit, useT } from "../ui";

type Policy = {
  id: string;
  name: string;
  tenant_id: string;
  revision: number;
  validation_status: string;
  validation_errors: string[];
  version_id: string;
  spec: {
    name: string;
    tenant_id: string;
    source: { type: string; id: string };
    destination: { type: string; id: string };
    protocol: string;
    ports: string;
    action: string;
    site_ids: string[];
  };
};

type Preview = {
  impact: {
    items: { device_id: string; device_name: string; classification: string; action: string; detail: string; blocking: boolean }[];
    blocked: boolean;
    block_reason?: string;
    calculated: number;
    inferred: number;
    verified: number;
    unknown: number;
  };
  plan_hash: string;
  preview_token: string;
  expires_at: string;
  policy_version_id: string;
  notice: string;
};

const emptySpec = { name: "", tenant_id: "", source_id: "", destination_id: "", protocol: "tcp", ports: "443", action: "allow", site_ids: [] as string[] };

export function PoliciesPage() {
  const t = useT();
  const nav = useNavigate();
  const qc = useQueryClient();
  const { principal } = useOutletContext<{ principal: Principal }>();
  const [params] = useSearchParams();
  const scopedSite = params.get("site") || "";
  const scopedDevice = params.get("device") || "";
  const [form, setForm] = useState({ ...emptySpec, site_ids: scopedSite ? [scopedSite] : [] });
  const [selected, setSelected] = useState("");
  const [preview, setPreview] = useState<Preview | null>(null);
  const policies = useQuery({ queryKey: ["policies"], queryFn: () => api<Policy[]>("/api/v1/policies") });
  const tenants = useQuery({ queryKey: ["tenants"], queryFn: () => api<{ id: string; name: string }[]>("/api/v1/tenants") });
  const segments = useQuery({ queryKey: ["segments"], queryFn: () => api<{ id: string; name: string; tenant_id: string }[]>("/api/v1/segments") });
  const sites = useQuery({ queryKey: ["sites"], queryFn: () => api<{ id: string; name: string }[]>("/api/v1/sites") });
  const tenantSegments = (segments.data ?? []).filter((item) => !form.tenant_id || item.tenant_id === form.tenant_id);

  const create = useMutation({
    mutationFn: () => api<Policy>("/api/v1/policies", {
      method: "POST",
      body: JSON.stringify({
        name: form.name,
        tenant_id: form.tenant_id,
        source: { type: "segment", id: form.source_id },
        destination: { type: "segment", id: form.destination_id },
        protocol: form.protocol,
        ports: form.ports,
        action: form.action,
        site_ids: form.site_ids,
      }),
    }),
    onSuccess: (policy) => {
      setSelected(policy.id);
      setPreview(null);
      void qc.invalidateQueries({ queryKey: ["policies"] });
    },
  });
  const previewMut = useMutation({
    mutationFn: (id: string) => api<Preview>(`/api/v1/policies/${id}/preview`, { method: "POST", body: "{}" }),
    onSuccess: (data) => setPreview(data),
  });
  const submit = useMutation({
    mutationFn: () => {
      if (!preview) throw new Error("preview");
      return api<{ id: string }>("/api/v1/changes", {
        method: "POST",
        headers: { "Idempotency-Key": crypto.randomUUID() },
        body: JSON.stringify({
          policy_version_id: preview.policy_version_id,
          plan_hash: preview.plan_hash,
          preview_token: preview.preview_token,
        }),
      });
    },
    onSuccess: (change) => nav(`/changes/${change.id}`),
  });

  useEffect(() => {
    if (scopedSite) setForm((current) => ({ ...current, site_ids: current.site_ids.includes(scopedSite) ? current.site_ids : [...current.site_ids, scopedSite] }));
  }, [scopedSite]);

  function toggleSite(id: string) {
    setForm((current) => ({
      ...current,
      site_ids: current.site_ids.includes(id) ? current.site_ids.filter((item) => item !== id) : [...current.site_ids, id],
    }));
    setPreview(null);
  }

  return (
    <Page title={t("policies")} crumbs={[{ to: "/", label: t("command") }, { label: t("policies") }]}>
      <div className="stack">
        {scopedSite || scopedDevice ? <p className="banner">{t("scopeHint")} {scopedDevice ? <Link to={`/inventory/devices/${scopedDevice}`}>{t("device")}</Link> : null}</p> : null}
        <section className="panel">
          <h2>{t("policies")}</h2>
          <QueryState loading={policies.isLoading} error={policies.error} onRetry={() => policies.refetch()}>
            {(policies.data ?? []).length === 0 ? <Empty text={t("emptyPolicies")} /> : (
              <table className="data">
                <thead><tr><th>{t("name")}</th><th>{t("revision")}</th><th>{t("status")}</th><th></th></tr></thead>
                <tbody>
                  {(policies.data ?? []).map((policy) => (
                    <tr key={policy.id}>
                      <td>{policy.name}</td>
                      <td>{policy.revision}</td>
                      <td><Badge value={policy.validation_status} /></td>
                      <td>
                        <button type="button" className="secondary" onClick={() => { setSelected(policy.id); setPreview(null); previewMut.mutate(policy.id); }} disabled={!can(principal.role, "author")}>
                          {t("preview")}
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </QueryState>
        </section>

        {can(principal.role, "author") ? (
          <section className="panel">
            <h2>{t("createPolicy")}</h2>
            <form className="stack" onSubmit={onSubmit(() => create.mutate())}>
              <div className="row">
                <Field label={t("name")}><input required value={form.name} onChange={(e) => { setForm({ ...form, name: e.target.value }); setPreview(null); }} /></Field>
                <Field label={t("tenant")}>
                  <select required value={form.tenant_id} onChange={(e) => { setForm({ ...form, tenant_id: e.target.value, source_id: "", destination_id: "" }); setPreview(null); }}>
                    <option value="" />
                    {(tenants.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
                  </select>
                </Field>
                <Field label={t("source")}>
                  <select required value={form.source_id} onChange={(e) => setForm({ ...form, source_id: e.target.value })}>
                    <option value="" />
                    {tenantSegments.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
                  </select>
                </Field>
                <Field label={t("destination")}>
                  <select required value={form.destination_id} onChange={(e) => setForm({ ...form, destination_id: e.target.value })}>
                    <option value="" />
                    {tenantSegments.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
                  </select>
                </Field>
                <Field label={t("protocol")}>
                  <select value={form.protocol} onChange={(e) => setForm({ ...form, protocol: e.target.value })}>
                    <option value="tcp">tcp</option><option value="udp">udp</option><option value="icmp">icmp</option><option value="any">any</option>
                  </select>
                </Field>
                <Field label={t("ports")}><input value={form.ports} onChange={(e) => setForm({ ...form, ports: e.target.value })} /></Field>
                <Field label={t("action")}>
                  <select value={form.action} onChange={(e) => setForm({ ...form, action: e.target.value })}>
                    <option value="allow">{t("allow")}</option>
                    <option value="deny">{t("deny")}</option>
                  </select>
                </Field>
              </div>
              <fieldset>
                <legend>{t("sites")}</legend>
                <div className="row">
                  {(sites.data ?? []).map((site) => (
                    <label className="check" key={site.id}>
                      <input type="checkbox" checked={form.site_ids.includes(site.id)} onChange={() => toggleSite(site.id)} />
                      {site.name}
                    </label>
                  ))}
                </div>
              </fieldset>
              <button type="submit" disabled={create.isPending}>{t("createPolicy")}</button>
              <FormError error={create.error} />
            </form>
          </section>
        ) : <p className="note">{t("noPermission")}</p>}

        <section className="panel">
          <h2>{t("impact")}</h2>
          <FormError error={previewMut.error || submit.error} />
          {!preview ? <p className="note">{selected ? t("loading") : t("empty")}</p> : (
            <>
              <p className="banner">{preview.notice}</p>
              <p className="note">{t("plan")} <span className="mono">{preview.plan_hash}</span></p>
              <p className="note">{t("expires")} {formatTime(preview.expires_at)} · calculated {preview.impact.calculated} · inferred {preview.impact.inferred} · verified {preview.impact.verified} · unknown {preview.impact.unknown}</p>
              {preview.impact.blocked ? <p className="state error" role="alert">{t("blocked")} {preview.impact.block_reason}</p> : null}
              {preview.impact.items.length === 0 ? <Empty text={t("empty")} /> : (
                <table className="data">
                  <thead><tr><th>{t("device")}</th><th>{t("classification")}</th><th>{t("action")}</th><th>{t("detail")}</th></tr></thead>
                  <tbody>
                    {preview.impact.items.map((item) => (
                      <tr key={item.device_id}>
                        <td><Link to={`/inventory/devices/${item.device_id}`}>{item.device_name}</Link></td>
                        <td><Badge value={item.classification} /></td>
                        <td>{item.action}</td>
                        <td>{item.detail}{item.blocking ? " · blocking" : ""}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
              <button type="button" style={{ marginTop: 8 }} disabled={preview.impact.blocked || submit.isPending} onClick={() => submit.mutate()}>{t("submit")}</button>
            </>
          )}
        </section>
      </div>
    </Page>
  );
}
