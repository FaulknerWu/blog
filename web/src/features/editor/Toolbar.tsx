import type { RefObject } from "react";
import { cn } from "~/shared/lib/cn";
import type { EditorHandle } from "./MarkdownEditor";

interface Props {
  editor: RefObject<EditorHandle | null>;
  onImage: () => void;
}

const tool =
  "inline-flex h-8.5 min-w-8.5 items-center justify-center px-1.5 text-sm font-semibold text-ink-soft hover:text-accent";
const serifTool = cn(tool, "font-serif text-base font-normal italic");

export function Toolbar({ editor, onImage }: Props) {
  const e = () => editor.current;
  return (
    <div role="toolbar" aria-label="格式" className="flex flex-wrap items-center gap-0.5 border-b border-line-soft px-3 py-0.5">
      <button type="button" className={tool} aria-label="二级标题" onClick={() => e()?.toggleLinePrefix("## ")}>
        H2
      </button>
      <button type="button" className={cn(tool, "font-extrabold")} aria-label="加粗" onClick={() => e()?.wrap("**", "**", "粗体")}>
        B
      </button>
      <button type="button" className={serifTool} aria-label="斜体" onClick={() => e()?.wrap("*", "*", "斜体")}>
        I
      </button>
      <button type="button" className={tool} aria-label="链接" onClick={() => e()?.wrap("[", "](https://)", "链接文字")}>
        链接
      </button>
      <button type="button" className={cn(tool, "font-mono text-xs font-normal")} aria-label="行内代码" onClick={() => e()?.wrap("`", "`", "code")}>
        `code`
      </button>
      <button type="button" className={serifTool} aria-label="公式" onClick={() => e()?.wrap("$", "$", "E = mc^2")}>
        ∑
      </button>
      <button type="button" className={tool} aria-label="插入图片" onClick={onImage}>
        图片
      </button>
      <span className="ml-auto text-xs text-muted">粘贴或拖入图片即自动上传</span>
    </div>
  );
}
