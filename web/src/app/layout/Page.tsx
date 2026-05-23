import type { ReactNode } from "react";
import { Link } from "react-router";

// 页面标题与眉题的样式，独立页面（ArticleLayout 版式）也沿用。
export const pageTitle = "text-5xl leading-tight font-bold md:text-6xl";
export const pageEyebrow = "mb-3 text-md leading-loose tracking-widest text-muted";

interface PageProps {
  eyebrow?: string;
  title?: string;
  description?: ReactNode;
  children?: ReactNode;
}

// Page 是列表类页面的窄栏骨架；不传 title 时不渲染页头（首页的标题在页眉里）。
export function Page({ eyebrow, title, description, children }: PageProps) {
  return (
    <div className="mx-auto box-content max-w-measure px-4 pt-14 pb-6">
      {title && (
        <header className="mb-8 border-b border-line pb-8">
          {eyebrow && <p className={pageEyebrow}>{eyebrow}</p>}
          <h1 className={pageTitle}>{title}</h1>
          {description && <p className="mt-3.5 text-lg text-ink-soft">{description}</p>}
        </header>
      )}
      {children}
    </div>
  );
}

// Pager 链接到游标分页的下一页；不在第一页时提供回到最新的入口。
export function Pager({ basePath, nextCursor, isFirst }: { basePath: string; nextCursor: string | null; isFirst: boolean }) {
  if (!nextCursor && isFirst) return null;
  return (
    <nav aria-label="分页" className="flex justify-center gap-4 pt-10">
      {!isFirst && (
        <Link to={basePath} className={pagerLink}>
          ← 回到最新
        </Link>
      )}
      {nextCursor && (
        <Link to={`${basePath}?cursor=${encodeURIComponent(nextCursor)}`} className={pagerLink}>
          更早的文章 →
        </Link>
      )}
    </nav>
  );
}

const pagerLink = "rounded-full border border-line px-5 py-2.5 text-base leading-loose text-accent hover:border-accent";
