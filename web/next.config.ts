import type { NextConfig } from "next";

// The frontend is a pure static export (output: "export"): `next build`
// emits plain HTML/JS/CSS into web/out, which the Go binary embeds and
// serves same-origin. Consequences worth knowing:
//
//   - No server-side Next features: no SSR, no API routes, no next
//     middleware, no rewrites. All pages are client components or
//     statically prerendered shells.
//   - Same-origin with the API: apiFetch uses relative URLs and cookie
//     sessions work with zero CORS setup.
//   - Development: run the Go server with APP_DEV_PROXY=http://localhost:3000
//     and `pnpm dev` — the Go process proxies non-API traffic to the dev
//     server, so HMR works against the real backend.
//
// Build options (read at CONFIG time, so pass them on the command line
// or in web/.env.local — both reach this file):
//
//   MARKDOWN_FULL=1 (default) — Shiki code highlighting + CJK breaks.
//   MARKDOWN_FULL=0           — lite markdown, ~11MB less in the binary.
//
// The switch is a resolve.alias: the bundler literally never sees the
// excluded implementation, so the savings are real.
const markdownLite = process.env.MARKDOWN_FULL === "0";

const nextConfig: NextConfig = {
  output: "export",
  trailingSlash: true,
  images: {
    unoptimized: true,
  },
  turbopack: markdownLite
    ? {
        resolveAlias: {
          // Exact module id → lite implementation (relative to web/).
          "@/components/markdown-impl": "./src/components/markdown-impl-lite.tsx",
        },
      }
    : {},
};

export default nextConfig;
