// Markdown — the public entry point for rendering LLM/user markdown.
// The concrete implementation (full: Shiki highlighting + CJK, or lite:
// plain Streamdown) is chosen at BUILD TIME via the MARKDOWN_FULL env —
// next.config.ts aliases @/components/markdown-impl to the right file so
// the excluded implementation is never bundled. AGENTS.md convention 8b:
// always import from here, never configure Streamdown by hand.
export { Markdown } from "@/components/markdown-impl";
