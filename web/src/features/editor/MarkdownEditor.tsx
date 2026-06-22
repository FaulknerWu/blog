import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { EditorSelection, EditorState, RangeSet, StateEffect, StateField, type Extension } from "@codemirror/state";
import {
  Decoration,
  drawSelection,
  EditorView,
  GutterMarker,
  gutterLineClass,
  highlightActiveLine,
  keymap,
  lineNumbers,
  placeholder,
  type DecorationSet,
} from "@codemirror/view";
import { forwardRef, useEffect, useImperativeHandle, useRef } from "react";
import { paperTheme } from "./theme";

export interface EditorHandle {
  focus(): void;
  // 用 before/after 包裹选区；选区为空时插入 fallback 并选中它。
  wrap(before: string, after: string, fallback: string): void;
  // 切换当前行的行首标记，如 "## "。
  toggleLinePrefix(prefix: string): void;
  insert(text: string): void;
  // 在指定位置（默认光标处）插入独占一行的内容，前后不在行边界时补换行。
  insertBlockAt(pos: number | null, text: string): void;
  replaceText(search: string, replacement: string): void;
  gotoLine(line: number): void;
}

interface Props {
  initialValue: string;
  errorLines: number[];
  onChange: (value: string) => void;
  onCursor: (line: number, column: number) => void;
  onSave: () => void;
  // 粘贴或拖入的图片文件，pos 为插入位置。
  onFiles: (files: File[], pos: number) => void;
  label: string;
}

const setErrors = StateEffect.define<number[]>();

const errorLinesField = StateField.define<number[]>({
  create: () => [],
  update(lines, tr) {
    for (const e of tr.effects) if (e.is(setErrors)) return e.value;
    return lines;
  },
});

class ErrorGutter extends GutterMarker {
  override elementClass = "cm-errorGutter";
}
const errorGutter = new ErrorGutter();

// 出错的行整行标红，行号同步标红。
const errorDecorations: Extension = [
  errorLinesField,
  EditorView.decorations.compute([errorLinesField], (state): DecorationSet => {
    const lines = [...new Set(state.field(errorLinesField))].filter((n) => n >= 1 && n <= state.doc.lines).sort((a, b) => a - b);
    return Decoration.set(lines.map((n) => Decoration.line({ class: "cm-errorLine" }).range(state.doc.line(n).from)));
  }),
  gutterLineClass.compute([errorLinesField], (state) => {
    const lines = [...new Set(state.field(errorLinesField))].filter((n) => n >= 1 && n <= state.doc.lines).sort((a, b) => a - b);
    return RangeSet.of(lines.map((n) => errorGutter.range(state.doc.line(n).from)));
  }),
];

function imageFiles(list: FileList | undefined | null): File[] {
  return [...(list ?? [])].filter((f) => f.type.startsWith("image/"));
}

export const MarkdownEditor = forwardRef<EditorHandle, Props>(function MarkdownEditor(props, ref) {
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView | null>(null);
  // 回调可能每次渲染都变化，CodeMirror 扩展只在创建时读取一次，因此经由 ref 转发。
  const cb = useRef(props);
  cb.current = props;

  useEffect(() => {
    const v = new EditorView({
      parent: host.current!,
      state: EditorState.create({
        doc: props.initialValue,
        extensions: [
          lineNumbers(),
          history(),
          drawSelection(),
          highlightActiveLine(),
          EditorView.lineWrapping,
          markdown({ base: markdownLanguage }),
          paperTheme,
          placeholder("从这里开始写…"),
          errorDecorations,
          EditorView.contentAttributes.of({ "aria-label": props.label, spellcheck: "false" }),
          keymap.of([
            { key: "Mod-s", preventDefault: true, run: () => (cb.current.onSave(), true) },
            { key: "Mod-b", run: (v) => (wrapIn(v, "**", "**", "粗体"), true) },
            { key: "Mod-i", run: (v) => (wrapIn(v, "*", "*", "斜体"), true) },
            { key: "Mod-k", run: (v) => (wrapIn(v, "[", "](https://)", "链接文字"), true) },
            ...defaultKeymap,
            ...historyKeymap,
            indentWithTab,
          ]),
          EditorView.updateListener.of((u) => {
            if (u.docChanged) cb.current.onChange(u.state.doc.toString());
            if (u.docChanged || u.selectionSet) {
              const head = u.state.selection.main.head;
              const line = u.state.doc.lineAt(head);
              cb.current.onCursor(line.number, head - line.from + 1);
            }
          }),
          EditorView.domEventHandlers({
            paste(e, v) {
              const files = imageFiles(e.clipboardData?.files);
              if (!files.length) return false;
              e.preventDefault();
              cb.current.onFiles(files, v.state.selection.main.head);
              return true;
            },
            drop(e, v) {
              const files = imageFiles(e.dataTransfer?.files);
              if (!files.length) return false;
              e.preventDefault();
              const pos = v.posAtCoords({ x: e.clientX, y: e.clientY }) ?? v.state.selection.main.head;
              cb.current.onFiles(files, pos);
              return true;
            },
          }),
        ],
      }),
    });
    view.current = v;
    return () => {
      v.destroy();
      view.current = null;
    };
    // 初始内容只在挂载时使用；需要整体替换时由上层更换 key 重建编辑器。
  }, []);

  useEffect(() => {
    view.current?.dispatch({ effects: setErrors.of(props.errorLines) });
  }, [props.errorLines]);

  useImperativeHandle(ref, () => ({
    focus: () => view.current?.focus(),
    wrap: (before, after, fallback) => view.current && wrapIn(view.current, before, after, fallback),
    toggleLinePrefix(prefix) {
      const v = view.current;
      if (!v) return;
      const line = v.state.doc.lineAt(v.state.selection.main.head);
      const existing = /^#{1,6} /.exec(line.text)?.[0] ?? "";
      const insert = existing === prefix ? "" : prefix;
      v.dispatch({ changes: { from: line.from, to: line.from + existing.length, insert } });
      v.focus();
    },
    insert(text) {
      const v = view.current;
      if (!v) return;
      v.dispatch(v.state.replaceSelection(text));
      v.focus();
    },
    insertBlockAt(at, text) {
      const v = view.current;
      if (!v) return;
      const doc = v.state.doc;
      const pos = at ?? v.state.selection.main.head;
      const before = pos > 0 && doc.sliceString(pos - 1, pos) !== "\n" ? "\n" : "";
      const after = pos < doc.length && doc.sliceString(pos, pos + 1) !== "\n" ? "\n" : "";
      const insert = before + text + after;
      v.dispatch({ changes: { from: pos, insert }, selection: { anchor: pos + insert.length } });
      v.focus();
    },
    replaceText(search, replacement) {
      const v = view.current;
      if (!v) return;
      const at = v.state.doc.toString().indexOf(search);
      if (at >= 0) v.dispatch({ changes: { from: at, to: at + search.length, insert: replacement } });
    },
    gotoLine(n) {
      const v = view.current;
      if (!v) return;
      const line = v.state.doc.line(Math.min(Math.max(1, n), v.state.doc.lines));
      v.dispatch({ selection: { anchor: line.from }, effects: EditorView.scrollIntoView(line.from, { y: "center" }) });
      v.focus();
    },
  }));

  return <div ref={host} className="h-full" />;
});

function wrapIn(v: EditorView, before: string, after: string, fallback: string) {
  v.dispatch(
    v.state.changeByRange((range) => {
      const text = v.state.sliceDoc(range.from, range.to) || fallback;
      const insert = before + text + after;
      return {
        changes: { from: range.from, to: range.to, insert },
        range: EditorSelection.range(range.from + before.length, range.from + before.length + text.length),
      };
    }),
  );
  v.focus();
}
