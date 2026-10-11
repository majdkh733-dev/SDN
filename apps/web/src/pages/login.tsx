import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { api, ApiError, type Principal } from "../api";
import { useT } from "../ui";

function randomToken(bytes = 32) {
  const buf = new Uint8Array(bytes);
  crypto.getRandomValues(buf);
  return Array.from(buf, (b) => b.toString(16).padStart(2, "0")).join("");
}

async function challenge(verifier: string) {
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier));
  const bytes = new Uint8Array(digest);
  let binary = "";
  bytes.forEach((b) => { binary += String.fromCharCode(b); });
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

export function LoginPage() {
  const t = useT();
  const nav = useNavigate();
  const qc = useQueryClient();
  const [email, setEmail] = useState("engineer@nexora.demo");
  const [password, setPassword] = useState("");
  const config = useQuery({ queryKey: ["auth-config"], queryFn: () => api<{ auth_mode: string; client_id: string }>("/api/v1/auth/config") });
  const login = useMutation({
    mutationFn: () => api<Principal>("/api/v1/auth/login", { method: "POST", body: JSON.stringify({ email, password }) }),
    onSuccess: async () => { await qc.invalidateQueries({ queryKey: ["me"] }); nav("/"); },
  });
  async function startOidc() {
    const verifier = randomToken();
    const state = randomToken(16);
    sessionStorage.setItem("sdn-verifier", verifier);
    sessionStorage.setItem("sdn-state", state);
    const params = new URLSearchParams({
      client_id: config.data?.client_id || "sdn-web",
      redirect_uri: `${window.location.origin}/callback`,
      response_type: "code",
      scope: "openid",
      code_challenge: await challenge(verifier),
      code_challenge_method: "S256",
      state,
    });
    window.location.assign(`/oidc/authorize?${params.toString()}`);
  }
  return (
    <main className="auth-wrap">
      <section className="auth-card">
        <h1>{t("product")}</h1>
        <p className="note">{t("tagline")}</p>
        <p className="note">{t("demoHint")}</p>
        <ul className="note">
          <li>engineer@nexora.demo</li>
          <li>approver@nexora.demo</li>
          <li>admin@nexora.demo</li>
          <li>recovery@nexora.demo</li>
          <li>viewer@nexora.demo</li>
        </ul>
        {config.data?.auth_mode !== "oidc" ? (
          <form onSubmit={(event) => { event.preventDefault(); login.mutate(); }}>
            <label className="field">{t("email")}<input type="email" autoComplete="username" required value={email} onChange={(e) => setEmail(e.target.value)} /></label>
            <label className="field">{t("password")}<input type="password" autoComplete="current-password" required value={password} onChange={(e) => setPassword(e.target.value)} /></label>
            <button type="submit" disabled={login.isPending}>{login.isPending ? t("working") : t("signIn")}</button>
            {login.error ? <p className="state error" role="alert">{login.error instanceof ApiError ? login.error.message : t("requestFailed")}</p> : null}
          </form>
        ) : <p className="note">{t("devLogin")}</p>}
        <button type="button" className="secondary" style={{ marginTop: 12 }} onClick={() => void startOidc()}>{t("oidc")}</button>
      </section>
    </main>
  );
}

export function CallbackPage() {
  const t = useT();
  const nav = useNavigate();
  const qc = useQueryClient();
  const [params] = useSearchParams();
  const [error, setError] = useState("");
  useEffect(() => {
    const code = params.get("code") || "";
    const state = params.get("state") || "";
    const verifier = sessionStorage.getItem("sdn-verifier") || "";
    if (!code || !verifier || state !== sessionStorage.getItem("sdn-state")) {
      setError(t("callbackError"));
      return;
    }
    const lock = `sdn-callback-${code}`;
    if (sessionStorage.getItem(lock)) return;
    sessionStorage.setItem(lock, "1");
    api<Principal>("/api/v1/auth/session", {
      method: "POST",
      body: JSON.stringify({ code, code_verifier: verifier, redirect_uri: `${window.location.origin}/callback` }),
    }).then(async () => {
      sessionStorage.removeItem("sdn-verifier");
      await qc.invalidateQueries({ queryKey: ["me"] });
      nav("/");
    }).catch((err: unknown) => setError(err instanceof ApiError ? err.message : t("requestFailed")));
  }, [nav, params, qc, t]);
  return (
    <main className="auth-wrap">
      <section className="auth-card">
        <h1>{t("product")}</h1>
        {error ? <p className="state error" role="alert">{error}</p> : <p role="status">{t("loading")}</p>}
      </section>
    </main>
  );
}
