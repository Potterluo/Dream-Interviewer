"use client";

import { createContext, useCallback, useContext, useEffect, useState } from "react";

// theme-provider: two independent axes, both persisted to localStorage:
//
//   mode   — dark / light / system            → toggles the .dark class
//   accent — violet / ocean / forest / …      → sets the data-theme attribute
//
// The root layout ships a tiny inline script that applies BOTH before
// React hydrates — without it users get a white flash / accent snap on
// every hard load.

export type Theme = "dark" | "light" | "system";

export interface ThemePreset {
  id: string; // data-theme value; "violet" is the default (no attribute)
  label: string;
  swatch: string; // representative hex for the picker UI
}

// The registry the settings picker renders. Adding a preset = one entry
// here + one [data-theme] CSS pair in globals.css.
export const THEME_PRESETS: ThemePreset[] = [
  { id: "violet", label: "Violet", swatch: "#7c5cf0" },
  { id: "ocean", label: "Ocean", swatch: "#3b82f6" },
  { id: "forest", label: "Forest", swatch: "#22a06b" },
  { id: "sunset", label: "Sunset", swatch: "#f59e0b" },
  { id: "rose", label: "Rose", swatch: "#f43f5e" },
  { id: "candy", label: "Candy", swatch: "#e05fd2" },
  { id: "mono", label: "Mono", swatch: "#3f3f46" },
];

export const DEFAULT_ACCENT = "violet";

const THEME_KEY = "app-theme";
const ACCENT_KEY = "app-accent";

const ThemeContext = createContext<{
  theme: Theme;
  setTheme: (t: Theme) => void;
  resolvedTheme: "dark" | "light";
  accent: string;
  setAccent: (id: string) => void;
}>({
  theme: "system",
  setTheme: () => {},
  resolvedTheme: "light",
  accent: DEFAULT_ACCENT,
  setAccent: () => {},
});

export function useTheme() {
  return useContext(ThemeContext);
}

function readSystem(): "dark" | "light" {
  if (typeof window === "undefined") return "light";
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function applyMode(resolved: "dark" | "light") {
  document.documentElement.classList.toggle("dark", resolved === "dark");
}

// applyAccent mirrors the inline head script in layout.tsx.
function applyAccent(accent: string) {
  if (accent === DEFAULT_ACCENT) {
    document.documentElement.removeAttribute("data-theme");
  } else {
    document.documentElement.setAttribute("data-theme", accent);
  }
}

function readStoredTheme(): Theme {
  if (typeof window === "undefined") return "system";
  const stored = localStorage.getItem(THEME_KEY) as Theme | null;
  return stored === "light" || stored === "dark" || stored === "system" ? stored : "system";
}

function readStoredAccent(): string {
  if (typeof window === "undefined") return DEFAULT_ACCENT;
  const stored = localStorage.getItem(ACCENT_KEY);
  return THEME_PRESETS.some((p) => p.id === stored) ? (stored as string) : DEFAULT_ACCENT;
}

function resolveTheme(theme: Theme): "dark" | "light" {
  return theme === "system" ? readSystem() : theme;
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(readStoredTheme);
  const [resolvedTheme, setResolvedTheme] = useState<"dark" | "light">(() =>
    resolveTheme(readStoredTheme())
  );
  const [accent, setAccentState] = useState<string>(readStoredAccent);

  useEffect(() => {
    applyMode(resolvedTheme);
  }, [resolvedTheme]);

  useEffect(() => {
    applyAccent(accent);
  }, [accent]);

  // When theme=system, follow OS changes live.
  useEffect(() => {
    if (theme !== "system" || typeof window === "undefined") return;
    const mql = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => {
      const next = mql.matches ? "dark" : "light";
      setResolvedTheme(next);
      applyMode(next);
    };
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }, [theme]);

  const setTheme = useCallback((next: Theme) => {
    setThemeState(next);
    localStorage.setItem(THEME_KEY, next);
    const resolved = next === "system" ? readSystem() : next;
    setResolvedTheme(resolved);
    applyMode(resolved);
  }, []);

  const setAccent = useCallback((next: string) => {
    setAccentState(next);
    localStorage.setItem(ACCENT_KEY, next);
    applyAccent(next);
  }, []);

  return (
    <ThemeContext.Provider value={{ theme, setTheme, resolvedTheme, accent, setAccent }}>
      {children}
    </ThemeContext.Provider>
  );
}
