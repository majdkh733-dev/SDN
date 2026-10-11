import { create } from "zustand";
import type { Locale } from "./i18n";

type UIState = {
  locale: Locale;
  menuOpen: boolean;
  setLocale: (locale: Locale) => void;
  setMenuOpen: (open: boolean) => void;
};

const stored = localStorage.getItem("sdn-locale");
const query = new URLSearchParams(window.location.search).get("lang");
const initial: Locale = query === "ar" || query === "en" ? query : stored === "ar" ? "ar" : "en";

export const useUI = create<UIState>((set) => ({
  locale: initial,
  menuOpen: false,
  setLocale: (locale) => {
    localStorage.setItem("sdn-locale", locale);
    set({ locale });
  },
  setMenuOpen: (menuOpen) => set({ menuOpen }),
}));
