export type Theme = {
  mode: "light" | "dark" | "system";
  preset: string;
  primary: string;
  accent: string;
  sidebar: string;
  surface: string;
  border: string;
  text: string;
  bg: string;
};

export function applyTheme(theme: Theme) {
  const dark = theme.mode === "dark" || (theme.mode === "system" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  const root = document.documentElement;
  root.dataset.theme = dark ? "dark" : "light";
  root.style.setProperty("--bg", theme.bg);
  root.style.setProperty("--surface", theme.surface);
  root.style.setProperty("--line", theme.border);
  root.style.setProperty("--text", theme.text);
  root.style.setProperty("--blue", theme.primary);
  root.style.setProperty("--accent", theme.accent);
  root.style.setProperty("--sidebar-bg", theme.sidebar);
  root.style.setProperty("--blue-soft", dark ? "#143044" : "#e7eef5");
  root.style.setProperty("--muted", dark ? "#9bb0c3" : "#5e6c7b");
}
