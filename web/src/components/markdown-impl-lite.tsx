"use client";

// LITE markdown implementation: plain Streamdown, no plugins. Chosen at
// build time when MARKDOWN_FULL=0 — trades code highlighting and CJK
// line breaking for ~11MB less in the shipped binary. Same props as the
// full implementation (see markdown-impl.tsx); swap back anytime.
import { type MouseEvent } from "react";
import { Streamdown } from "streamdown";

const PROSE_CLASS =
  "text-sm leading-relaxed [&>*:first-child]:mt-0 [&>*:last-child]:mb-0 " +
  "[&_p]:my-2 [&_ul]:my-2 [&_ol]:my-2 [&_ul]:list-disc [&_ol]:list-decimal [&_li]:ml-5 " +
  "[&_a]:text-primary [&_a]:underline [&_a]:underline-offset-2";

export function Markdown({
  children,
  streaming = false,
  className,
}: {
  children: string;
  streaming?: boolean;
  className?: string;
}) {
  return (
    <div
      className={`${PROSE_CLASS} ${className ?? ""}`}
      onClick={linkGuard}
    >
      <Streamdown parseIncompleteMarkdown={streaming}>{children}</Streamdown>
    </div>
  );
}

// External links open in a new tab.
function linkGuard(e: MouseEvent<HTMLDivElement>) {
  const anchor = (e.target as HTMLElement).closest("a");
  if (!anchor) return;
  const href = anchor.getAttribute("href") ?? "";
  if (/^https?:\/\//.test(href)) {
    anchor.setAttribute("target", "_blank");
    anchor.setAttribute("rel", "noreferrer noopener");
  }
}
