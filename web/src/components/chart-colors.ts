"use client";

import { useEffect, useState } from "react";
import { useTheme } from "@/components/theme-provider";

// Chart colors: SVG presentation attributes do NOT resolve CSS var(), so
// the theme tokens (--chart-1..5, --border, --muted-foreground) are read
// from the computed style in JS (AGENTS.md convention 9). Recharts HTML
// tooltips may still use var().
//
// The initial read happens one frame after mount (requestAnimationFrame)
// and every later read is driven by a MutationObserver on <html>'s
// class/style — a theme flip rewrites both the `.dark` class and the
// `data-theme` attribute, so the charts follow the accent preset. Keeping
// the reads inside callbacks (rather than setting state synchronously in
// the effect body) also satisfies react-hooks/set-state-in-effect.

export interface ChartColors {
  c1: string;
  c2: string;
  c3: string;
  c4: string;
  c5: string;
  grid: string;
  axis: string;
}

const FALLBACK: ChartColors = {
  c1: "#8b5cf6",
  c2: "#06b6d4",
  c3: "#64748b",
  c4: "#d97706",
  c5: "#f97316",
  grid: "#27272a",
  axis: "#a1a1aa",
};

function readChartColors(): ChartColors {
  const s = getComputedStyle(document.documentElement);
  const read = (name: string, fallback: string) => s.getPropertyValue(name).trim() || fallback;
  return {
    c1: read("--chart-1", FALLBACK.c1),
    c2: read("--chart-2", FALLBACK.c2),
    c3: read("--chart-3", FALLBACK.c3),
    c4: read("--chart-4", FALLBACK.c4),
    c5: read("--chart-5", FALLBACK.c5),
    grid: read("--border", FALLBACK.grid),
    axis: read("--muted-foreground", FALLBACK.axis),
  };
}

export function useChartColors(): ChartColors {
  const { resolvedTheme } = useTheme();
  const [colors, setColors] = useState<ChartColors>(FALLBACK);

  useEffect(() => {
    let frame = window.requestAnimationFrame(() => setColors(readChartColors()));
    const observer = new MutationObserver(() => {
      window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(() => setColors(readChartColors()));
    });
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class", "data-theme"],
    });
    return () => {
      window.cancelAnimationFrame(frame);
      observer.disconnect();
    };
  }, [resolvedTheme]);

  return colors;
}

// Shared recharts <Tooltip contentStyle> — the tooltip is HTML, so the CSS
// vars work here.
export const CHART_TOOLTIP = {
  background: "var(--popover)",
  border: "1px solid var(--border)",
  borderRadius: 8,
  color: "var(--popover-foreground)",
} as const;
