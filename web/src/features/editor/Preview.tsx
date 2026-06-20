import { cn } from "~/shared/lib/cn";

// 预览直接复用公开站的正文样式（.prose），这里只补充版心。
const preview = "mx-auto max-w-measure px-4 pt-6 pb-10 font-serif sm:px-8 sm:pt-9 sm:pb-14";

// Preview 展示与保存时完全相同的渲染结果，并套用公开站的正文样式。
export function Preview({ html }: { html: string | undefined }) {
  if (html === undefined) return <div className={preview} aria-busy="true" />;
  if (!html) {
    return (
      <div className={preview}>
        <p className="font-sans text-md text-muted">正文还是空的。</p>
      </div>
    );
  }
  return <div className={cn(preview, "prose")} dangerouslySetInnerHTML={{ __html: html }} />;
}
