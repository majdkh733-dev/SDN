import { useEffect, type FormEvent, type ReactNode } from "react";
import { Link } from "react-router-dom";
import { ApiError } from "./api";
import { direction, translate } from "./i18n";
import { useUI } from "./session";

export function useT() {
  const locale = useUI((s) => s.locale);
  return (key: string) => translate(locale, key);
}

export function useDirection() {
  const locale = useUI((s) => s.locale);
  useEffect(() => {
    document.documentElement.lang = locale;
    document.documentElement.dir = direction(locale);
  }, [locale]);
}

export function message(error: unknown, fallback: string) {
  return error instanceof ApiError ? error.message : fallback;
}

export function tone(status: string): "ok" | "bad" | "warn" | "info" | "neutral" {
  if (["succeeded", "healthy", "approved", "active", "valid", "verified", "remediated", "demo", "compensated", "online", "completed", "matched"].includes(status)) return "ok";
  if (["failed", "rollback_failed", "rejected", "down", "not_implemented", "offline", "revoked", "ambiguous"].includes(status)) return "bad";
  if (["partial", "unknown", "compensating", "pending_reconciliation", "implemented_unverified", "open", "validated", "queued", "connecting", "degraded", "not_enrolled", "cancelled"].includes(status)) return "warn";
  if (["applying", "verifying", "running"].includes(status)) return "info";
  return "neutral";
}

export function Badge({ value }: { value: string }) {
  return <span className={`badge`} data-tone={tone(value)}>{value}</span>;
}

export function formatTime(value?: string | null) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleString();
}

export function Page({ title, crumbs, actions, children }: { title: string; crumbs: { to?: string; label: string }[]; actions?: ReactNode; children: ReactNode }) {
  useEffect(() => {
    document.title = `${title} · SDN`;
  }, [title]);
  return (
    <>
      <header className="page-head">
        <nav className="crumbs" aria-label="Breadcrumb">
          <ol>
            {crumbs.map((crumb) => (
              <li key={crumb.label}>{crumb.to ? <Link to={crumb.to}>{crumb.label}</Link> : <span aria-current="page">{crumb.label}</span>}</li>
            ))}
          </ol>
        </nav>
        <div className="page-title-row">
          <h1>{title}</h1>
          {actions}
        </div>
      </header>
      {children}
    </>
  );
}

export function QueryState({ loading, error, onRetry, children }: { loading: boolean; error: unknown; onRetry: () => void; children: ReactNode }) {
  const t = useT();
  if (loading) return <p className="state" role="status">{t("loading")}</p>;
  if (error) {
    return (
      <div className="state error" role="alert">
        <p>{message(error, t("requestFailed"))}</p>
        <button type="button" onClick={onRetry}>{t("retry")}</button>
      </div>
    );
  }
  return <>{children}</>;
}

export function Empty({ text }: { text: string }) {
  return <p className="state">{text}</p>;
}

export function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="field grow">
      {label}
      {children}
    </label>
  );
}

export function FormError({ error }: { error: unknown }) {
  const t = useT();
  if (!error) return null;
  return <p className="state error" role="alert">{message(error, t("requestFailed"))}</p>;
}

export function onSubmit(handler: () => void) {
  return (event: FormEvent) => {
    event.preventDefault();
    handler();
  };
}
