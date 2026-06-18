import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { EditorView } from "@codemirror/view";
import { tags as t } from "@lezer/highlight";

// CodeMirror 外观：等宽字体、细线行号栏。颜色全部取自设计令牌，深色模式随变量自动切换。
const editorTheme = EditorView.theme({
  "&": {
    height: "100%",
    backgroundColor: "transparent",
    color: "var(--color-ink)",
    fontSize: "var(--text-sm)",
  },
  "&.cm-focused": { outline: "none" },
  ".cm-scroller": {
    fontFamily: "var(--font-mono)",
    lineHeight: "24px",
    padding: "16px 0",
  },
  ".cm-content": { padding: "0", caretColor: "var(--color-ink)" },
  ".cm-line": { padding: "0 20px" },
  ".cm-gutters": {
    backgroundColor: "transparent",
    borderRight: "1px solid var(--color-line-soft)",
    color: "var(--color-muted)",
  },
  ".cm-lineNumbers .cm-gutterElement": { minWidth: "32px", padding: "0 12px 0 8px" },
  ".cm-activeLine": { backgroundColor: "color-mix(in srgb, var(--color-ink) 3%, transparent)" },
  ".cm-activeLineGutter": { backgroundColor: "transparent", color: "var(--color-ink)" },
  ".cm-selectionBackground, &.cm-focused .cm-selectionBackground": {
    backgroundColor: "color-mix(in srgb, var(--color-accent) 22%, transparent) !important",
  },
  ".cm-cursor": { borderLeftColor: "var(--color-ink)" },
  ".cm-errorLine": { backgroundColor: "var(--color-danger-soft)" },
  ".cm-errorGutter": { color: "var(--color-danger)" },
  ".cm-placeholder": { color: "var(--color-muted)" },
});

const highlight = HighlightStyle.define([
  { tag: t.heading, color: "var(--color-syntax-heading)", fontWeight: "700" },
  { tag: [t.processingInstruction, t.meta, t.contentSeparator], color: "var(--color-muted)" },
  { tag: t.monospace, color: "var(--color-ok)" },
  { tag: [t.url, t.link], color: "var(--color-accent)" },
  { tag: t.emphasis, fontStyle: "italic" },
  { tag: t.strong, fontWeight: "700" },
  { tag: t.strikethrough, textDecoration: "line-through" },
  { tag: t.quote, color: "var(--color-ink-soft)" },
]);

export const paperTheme = [editorTheme, syntaxHighlighting(highlight)];
