import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link, useOutletContext } from "react-router-dom";
import { api, can, type Principal } from "../api";
import { Badge, Empty, Field, FormError, Page, QueryState, onSubmit, useT } from "../ui";

type Site = { id: string; name: string; location: string; execution_domain: string; lifecycle: string; latitude?: number; longitude?: number };

export function SitesPage() {
  const t = useT();
  const { principal } = useOutletContext<{ principal: Principal }>();
  const qc = useQueryClient();
  const [form, setForm] = useState({ name: "", location: "", execution_domain: "default" });
  const sites = useQuery({ queryKey: ["sites"], queryFn: () => api<Site[]>("/api/v1/sites") });
  const create = useMutation({
    mutationFn: () => api<Site>("/api/v1/sites", { method: "POST", body: JSON.stringify(form) }),
    onSuccess: () => { setForm({ name: "", location: "", execution_domain: "default" }); void qc.invalidateQueries({ queryKey: ["sites"] }); },
  });
  return (
    <Page title={t("sites")} crumbs={[{ to: "/", label: t("command") }, { label: t("sites") }]}>
      <section className="panel">
        <QueryState loading={sites.isLoading} error={sites.error} onRetry={() => sites.refetch()}>
          {(sites.data ?? []).length === 0 ? <Empty text={t("empty")} /> : (
            <table className="data">
              <thead><tr><th>{t("name")}</th><th>{t("location")}</th><th>{t("domain")}</th><th>{t("lifecycle")}</th><th></th></tr></thead>
              <tbody>
                {(sites.data ?? []).map((site) => (
                  <tr key={site.id}>
                    <td>{site.name}</td>
                    <td>{site.location}</td>
                    <td>{site.execution_domain}</td>
                    <td><Badge value={site.lifecycle} /></td>
                    <td><Link to={`/discovery?site=${site.id}`}>{t("discovery")}</Link></td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </QueryState>
        {can(principal.role, "operate") ? (
          <form className="row" onSubmit={onSubmit(() => create.mutate())}>
            <Field label={t("name")}><input required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /></Field>
            <Field label={t("location")}><input value={form.location} onChange={(e) => setForm({ ...form, location: e.target.value })} /></Field>
            <Field label={t("domain")}><input required value={form.execution_domain} onChange={(e) => setForm({ ...form, execution_domain: e.target.value })} /></Field>
            <button type="submit">{t("createSite")}</button>
            <FormError error={create.error} />
          </form>
        ) : <p className="note">{t("noPermission")}</p>}
      </section>
    </Page>
  );
}
