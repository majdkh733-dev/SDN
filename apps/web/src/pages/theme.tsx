import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { api, can } from "../api";
import { applyTheme, type Theme } from "../theme";
import { FormError, useT } from "../ui";

const navy: Theme = {
  mode: "dark", preset: "navy", primary: "#3ec6e0", accent: "#7ee0f2",
  sidebar: "#0a1624", surface: "#102033", border: "#1e3a52", text: "#e8f1f8", bg: "#071018",
};

export function ThemePanel({ role }: { role: string }) {
  const t = useT();
  const qc = useQueryClient();
  const prefs = useQuery({ queryKey: ["preferences"], queryFn: () => api<{ theme: Theme }>("/api/v1/preferences") });
  const [draft, setDraft] = useState<Theme | null>(null);
  const theme = draft ?? prefs.data?.theme;
  useEffect(() => { if (theme) applyTheme(theme); }, [theme]);
  const save = useMutation({
    mutationFn: (next: Theme) => api<Theme>("/api/v1/preferences/theme", { method: "POST", body: JSON.stringify(next) }),
    onSuccess: async (saved) => { setDraft(null); applyTheme(saved); await qc.invalidateQueries({ queryKey: ["preferences"] }); },
  });
  const reset = useMutation({
    mutationFn: () => api<Theme>("/api/v1/preferences/theme/reset", { method: "POST", body: "{}" }),
    onSuccess: async (saved) => { setDraft(null); applyTheme(saved); await qc.invalidateQueries({ queryKey: ["preferences"] }); },
  });
  if (!theme) return null;
  function choose(next: Theme) {
    setDraft(next);
    if (can(role, "admin")) save.mutate(next);
  }
  function patch(partial: Partial<Theme>) {
    setDraft({ ...theme!, ...partial, preset: partial.preset || "custom" });
  }
  return (
    <section className="panel">
      <h2>{t("appearance")}</h2>
      <div className="theme-preview" style={{ background: theme.bg, color: theme.text, borderColor: theme.border }}>
        <div style={{ background: theme.sidebar, padding: 8 }}>{t("navigation")}</div>
        <div style={{ background: theme.surface, padding: 8, border: `1px solid ${theme.border}` }}>
          <strong style={{ color: theme.primary }}>{t("command")}</strong>
          <p>{t("themeNavy")}</p>
        </div>
      </div>
      <div className="lenses">
        <button type="button" className="secondary" aria-pressed={theme.mode === "light"} onClick={() => choose({ ...theme, mode: "light" })}>{t("light")}</button>
        <button type="button" className="secondary" aria-pressed={theme.mode === "dark"} onClick={() => choose({ ...theme, mode: "dark" })}>{t("dark")}</button>
        <button type="button" className="secondary" aria-pressed={theme.mode === "system"} onClick={() => choose({ ...theme, mode: "system" })}>{t("system")}</button>
        <button type="button" className="secondary" onClick={() => choose(navy)}>{t("themeNavy")}</button>
        <button type="button" className="secondary" onClick={() => choose({ mode: "system", preset: "default", primary: "#2c5f8a", accent: "#2c5f8a", sidebar: "#ffffff", surface: "#ffffff", border: "#e4e8ee", text: "#1c2834", bg: "#f3f5f8" })}>{t("themeDefault")}</button>
      </div>
      <div className="row">
        {(["primary", "accent", "sidebar", "surface", "border", "text", "bg"] as const).map((key) => (
          <label key={key} className="field">{key}<input type="color" value={theme[key]} onChange={(event) => patch({ [key]: event.target.value })} /></label>
        ))}
      </div>
      {can(role, "admin") ? (
        <div className="row">
          <button type="button" onClick={() => save.mutate(theme)} disabled={save.isPending}>{t("save")}</button>
          <button type="button" className="secondary" onClick={() => reset.mutate()}>{t("reset")}</button>
        </div>
      ) : <p className="note">{t("noPermission")}</p>}
      <FormError error={save.error || reset.error} />
    </section>
  );
}
