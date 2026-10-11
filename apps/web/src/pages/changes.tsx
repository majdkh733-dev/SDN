import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link, useOutletContext, useParams, useSearchParams } from "react-router-dom";
import { api, can, type Principal } from "../api";
import { confirmationMatches } from "../confirm";
import { Badge, Empty, Field, FormError, Page, QueryState, formatTime, onSubmit, useT } from "../ui";

type Change = {
  id: string;
  policy_name: string;
  plan_hash: string;
  status: string;
  requested_by: string;
  requester_email: string;
  error_detail: string;
  cancel_requested: boolean;
  created_at: string;
  updated_at: string;
  impact: { items?: { device_id: string; device_name: string; classification: string; action: string; detail: string }[]; blocked?: boolean };
  approvals: { id: string; decision: string; reviewer_id: string; created_at: string }[] | null;
  steps: { id: string; operation_key: string; device_id?: string; status: string; attempt: number; result: unknown }[] | null;
  verifications: { id: string; scope: string; result: string; evidence: string; created_at: string }[] | null;
};

const active = new Set(["applying", "verifying", "queued", "compensating", "partial", "unknown"]);

export function ChangesPage() {
  const t = useT();
  const [params, setParams] = useSearchParams();
  const status = params.get("status") ?? "";
  const changes = useQuery({
    queryKey: ["changes", status],
    queryFn: () => api<Change[]>(`/api/v1/changes?status=${encodeURIComponent(status)}`),
  });
  return (
    <Page title={t("changes")} crumbs={[{ to: "/", label: t("command") }, { label: t("changes") }]}>
      <label className="field" style={{ maxWidth: 240, marginBottom: 12 }}>
        {t("status")}
        <select value={status} onChange={(e) => {
          const next = new URLSearchParams(params);
          if (e.target.value) next.set("status", e.target.value);
          else next.delete("status");
          setParams(next);
        }}>
          <option value="">{t("allStatuses")}</option>
          {["validated", "approved", "rejected", "queued", "applying", "verifying", "succeeded", "failed", "partial", "compensating", "compensated", "rollback_failed", "unknown", "cancelled"].map((item) => (
            <option key={item} value={item}>{item}</option>
          ))}
        </select>
      </label>
      <section className="panel">
        <QueryState loading={changes.isLoading} error={changes.error} onRetry={() => changes.refetch()}>
          {(changes.data ?? []).length === 0 ? <Empty text={t("emptyChanges")} /> : (
            <table className="data">
              <thead><tr><th>{t("name")}</th><th>{t("status")}</th><th>{t("requester")}</th><th>{t("when")}</th></tr></thead>
              <tbody>
                {(changes.data ?? []).map((item) => (
                  <tr key={item.id}>
                    <td><Link to={`/changes/${item.id}`}>{item.policy_name || item.id.slice(0, 8)}</Link></td>
                    <td><Badge value={item.status} /></td>
                    <td>{item.requester_email}</td>
                    <td>{formatTime(item.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </QueryState>
      </section>
    </Page>
  );
}

export function ChangePage() {
  const t = useT();
  const { id = "" } = useParams();
  const qc = useQueryClient();
  const { principal } = useOutletContext<{ principal: Principal }>();
  const [typed, setTyped] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const change = useQuery({
    queryKey: ["change", id],
    queryFn: () => api<Change>(`/api/v1/changes/${id}`),
    refetchInterval: (query) => (active.has(query.state.data?.status || "") ? 2000 : false),
  });
  const decide = useMutation({
    mutationFn: (decision: string) => api<Change>(`/api/v1/changes/${id}/approve`, { method: "POST", body: JSON.stringify({ decision }) }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["change", id] }),
  });
  const execute = useMutation({
    mutationFn: () => api<Change>(`/api/v1/changes/${id}/execute`, {
      method: "POST",
      body: JSON.stringify({ confirm: true, plan_hash: change.data?.plan_hash }),
    }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["change", id] }),
  });
  const cancel = useMutation({
    mutationFn: () => api<Change>(`/api/v1/changes/${id}/cancel`, { method: "POST", body: "{}" }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["change", id] }),
  });
  const item = change.data;
  const ready = item ? confirmationMatches(item.plan_hash, typed) && confirmed : false;
  return (
    <Page title={item?.policy_name || t("changes")} crumbs={[{ to: "/", label: t("command") }, { to: "/changes", label: t("changes") }, { label: item?.policy_name || id.slice(0, 8) }]}>
      <QueryState loading={change.isLoading} error={change.error} onRetry={() => change.refetch()}>
        {item ? (
          <div className="stack">
            <section className="panel">
              <div className="page-title-row">
                <h2>{t("status")}</h2>
                <Badge value={item.status} />
              </div>
              <p className="note">{t("requester")} {item.requester_email}</p>
              <p className="note">{t("plan")} <span className="mono">{item.plan_hash}</span></p>
              <p className="note">{t("when")} {formatTime(item.updated_at)}</p>
              {item.error_detail ? <p className="state error" role="alert">{t("errorDetail")}: {item.error_detail}</p> : null}
              {item.status === "partial" ? <p className="banner">{t("partialNote")}</p> : null}
              {item.status === "rollback_failed" ? <p className="banner">{t("rollbackNote")}</p> : null}
              {item.status === "unknown" ? <p className="banner">{t("unknownNote")}</p> : null}
              {item.cancel_requested ? <p className="note">{t("cancelChange")}</p> : null}
            </section>
            <section className="panel">
              <h2>{t("impact")}</h2>
              {(item.impact?.items ?? []).length === 0 ? <Empty text={t("empty")} /> : (
                <table className="data">
                  <thead><tr><th>{t("device")}</th><th>{t("classification")}</th><th>{t("action")}</th><th>{t("detail")}</th></tr></thead>
                  <tbody>
                    {(item.impact.items ?? []).map((row) => (
                      <tr key={row.device_id}><td>{row.device_name}</td><td><Badge value={row.classification} /></td><td>{row.action}</td><td>{row.detail}</td></tr>
                    ))}
                  </tbody>
                </table>
              )}
            </section>
            <section className="panel">
              <h2>{t("approvals")}</h2>
              {(item.approvals ?? []).length === 0 ? <Empty text={t("empty")} /> : (
                <table className="data">
                  <thead><tr><th>{t("decision")}</th><th>{t("when")}</th></tr></thead>
                  <tbody>{(item.approvals ?? []).map((row) => <tr key={row.id}><td><Badge value={row.decision} /></td><td>{formatTime(row.created_at)}</td></tr>)}</tbody>
                </table>
              )}
              {item.status === "validated" && can(principal.role, "approve") ? (
                <div className="row" style={{ marginTop: 8 }}>
                  <button type="button" onClick={() => decide.mutate("approved")}>{t("approve")}</button>
                  <button type="button" className="danger" onClick={() => decide.mutate("rejected")}>{t("reject")}</button>
                </div>
              ) : null}
              {item.status === "validated" && !can(principal.role, "approve") ? <p className="note">{t("noPermission")}</p> : null}
              <FormError error={decide.error} />
            </section>
            {(item.status === "approved" || item.status === "unknown") && can(principal.role, "author") ? (
              <section className="panel">
                <h2>{t("execute")}</h2>
                <p className="note">{t("hashHint")}</p>
                <form className="stack" onSubmit={onSubmit(() => { if (ready) execute.mutate(); })}>
                  <Field label={t("typeHash")}>
                    <input value={typed} onChange={(e) => setTyped(e.target.value)} autoComplete="off" spellCheck={false} aria-describedby="hash-hint" />
                  </Field>
                  <p id="hash-hint" className="note">{item.plan_hash.slice(0, 8)}</p>
                  <label className="check">
                    <input type="checkbox" checked={confirmed} onChange={(e) => setConfirmed(e.target.checked)} />
                    {t("confirm")}
                  </label>
                  <button type="submit" disabled={!ready || execute.isPending}>{t("execute")}</button>
                  <FormError error={execute.error} />
                </form>
              </section>
            ) : null}
            {can(principal.role, "author") && ["validated", "approved", "queued"].includes(item.status) ? (
              <button type="button" className="secondary" onClick={() => cancel.mutate()}>{t("cancelChange")}</button>
            ) : null}
            <FormError error={cancel.error} />
            <section className="panel">
              <h2>{t("steps")}</h2>
              {(item.steps ?? []).length === 0 ? <Empty text={t("empty")} /> : (
                <table className="data">
                  <thead><tr><th>{t("operation")}</th><th>{t("status")}</th><th>{t("attempt")}</th></tr></thead>
                  <tbody>
                    {(item.steps ?? []).map((step) => (
                      <tr key={step.id}><td className="mono">{step.operation_key}</td><td><Badge value={step.status} /></td><td>{step.attempt}</td></tr>
                    ))}
                  </tbody>
                </table>
              )}
            </section>
            <section className="panel">
              <h2>{t("verifications")}</h2>
              {(item.verifications ?? []).length === 0 ? <Empty text={t("empty")} /> : (
                <table className="data">
                  <thead><tr><th>{t("scope")}</th><th>{t("result")}</th><th>{t("evidence")}</th><th>{t("when")}</th></tr></thead>
                  <tbody>
                    {(item.verifications ?? []).map((row) => (
                      <tr key={row.id}><td>{row.scope}</td><td><Badge value={row.result} /></td><td>{row.evidence}</td><td>{formatTime(row.created_at)}</td></tr>
                    ))}
                  </tbody>
                </table>
              )}
            </section>
          </div>
        ) : null}
      </QueryState>
    </Page>
  );
}
