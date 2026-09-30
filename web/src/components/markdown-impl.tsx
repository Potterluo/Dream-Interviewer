"use client";

import { type MouseEvent } from "react";
import { Streamdown } from "streamdown";
import { createCodePlugin } from "@streamdown/code";
import { cjk } from "@streamdown/cjk";
import remarkBreaks from "remark-breaks";

// FULL markdown implementation: Streamdown + Shiki code highlighting
// (via @streamdown/code) + CJK line breaking. Selected at build time by
// the MARKDOWN_FULL env (next.config.ts aliases @/components/markdown-impl
// to this file or markdown-impl-lite.tsx). Adding ~11MB to the binary —
// set MARKDOWN_FULL=0 for the lite build.
//
//   code — Shiki syntax highlighting, github-light/github-dark themes
//          (the surrounding .dark context picks the side), line numbers,
//          copy/download buttons on every code block
//   cjk  — proper CJK line breaking, with remark-breaks injected AFTER
//          gfm (single newlines become <br>; injecting before would
//          rewrite newlines inside tables and break them)
//
// Streaming-safe: pass streaming while tokens are arriving so incomplete
// markdown (unclosed fences/bold) renders cleanly.
//
// Layout note: code blocks render as full-width cards — put <Markdown> in
// a full-width container, not inside a tight bubble.

const code = createCodePlugin({ themes: ["github-light", "github-dark"] });
const cjkWithBreaks = { ...cjk, remarkPluginsAfter: [...cjk.remarkPluginsAfter, remarkBreaks] };

const PROSE_CLASS =
  "text-sm leading-relaxed [&>*:first-child]:mt-0 [&>*:last-child]:mb-0 " +
  "[&_h1]:text-lg [&_h1]:font-semibold [&_h2]:text-base [&_h2]:font-semibold [&_h3]:text-sm [&_h3]:font-semibold " +
  "[&_p]:my-2 [&_ul]:my-2 [&_ol]:my-2 [&_ul]:list-disc [&_ol]:list-decimal [&_li]:ml-5 [&_li]:my-0.5 " +
  "[&_blockquote]:border-l-2 [&_blockquote]:border-border [&_blockquote]:pl-3 [&_blockquote]:text-muted-foreground " +
  "[&_a]:text-primary [&_a]:underline [&_a]:underline-offset-2 " +
  "[&_hr]:my-3 [&_table]:my-2 [&_th]:text-left";

export function Markdown({
  children,
  streaming = false,
  className,
}: {
  children: string;
  /** true while tokens are still arriving — heals unclosed fences/bold */
  streaming?: boolean;
  className?: string;
}) {
  const onClick = useCallbackLinkGuard();
  return (
    <div className={`${PROSE_CLASS} ${className ?? ""}`} onClick={onClick}>
      <Streamdown
        parseIncompleteMarkdown={streaming}
        plugins={{ code, cjk: cjkWithBreaks }}
      >
        {children}
      </Streamdown>
    </div>
  );
}

// Anchor clicks inside rendered markdown: external URLs open in a new
// tab; relative paths fall back to normal navigation.
function useCallbackLinkGuard() {
  return (e: MouseEvent<HTMLDivElement>) => {
    const anchor = (e.target as HTMLElement).closest("a");
    if (!anchor) return;
    const href = anchor.getAttribute("href") ?? "";
    if (/^https?:\/\//.test(href)) {
      anchor.setAttribute("target", "_blank");
      anchor.setAttribute("rel", "noreferrer noopener");
    }
  };
}
