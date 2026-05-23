import type { ReactNode } from "react";
import { cn } from "~/shared/lib/cn";

interface Props {
  header: ReactNode;
  toc?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
}

// ArticleLayout 是文章与独立页面共用的阅读版式：窄栏正文，宽屏时目录位于右侧边栏。
export function ArticleLayout({ header, toc, footer, children }: Props) {
  return (
    <article
      className={cn(
        "mx-auto box-content grid max-w-measure grid-cols-[minmax(0,1fr)] px-4 pt-16 pb-8",
        "[grid-template-areas:'header'_'toc'_'body'_'footer']",
        // 宽屏时目录移到正文右侧留白处并随滚动停留。正文 measure + 间距 25 + 目录 50 恰好等于 wide，左右边缘与页眉对齐。
        toc &&
          "xl:max-w-wide xl:grid-cols-[minmax(0,var(--container-measure))_--spacing(50)] xl:justify-center xl:gap-x-25 xl:[grid-template-areas:'header_.'_'body_toc'_'footer_.']",
      )}
    >
      <header className="[grid-area:header]">{header}</header>
      {toc && (
        <aside className="mt-8 rounded-md border border-line px-5 pt-4 pb-3 [grid-area:toc] xl:sticky xl:top-0 xl:mt-0 xl:max-h-screen xl:self-start xl:overflow-y-auto xl:rounded-none xl:border-0 xl:px-0 xl:pt-10 xl:pb-0">
          {toc}
        </aside>
      )}
      <div className="pt-10 [grid-area:body]">{children}</div>
      {footer && <footer className="mt-12 [grid-area:footer]">{footer}</footer>}
    </article>
  );
}
