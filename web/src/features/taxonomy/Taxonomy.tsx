import { Link } from "react-router";
import type { SeriesDetail, SeriesSummary, Tag } from "~/shared/api/types";
import { formatDate, type Locale } from "~/shared/lib/format";

export function TagIndex({ tags }: { tags: Tag[] }) {
  return (
    <ul className="flex flex-wrap gap-x-7 gap-y-3 text-xl leading-loose">
      {tags.map((t) => (
        <li key={t.slug}>
          <Link to={`/tags/${encodeURIComponent(t.slug)}`} className="inline-flex items-baseline gap-1.5 py-1.5">
            #{t.name}
            <span className="text-sm leading-loose text-muted">{t.post_count}</span>
          </Link>
        </li>
      ))}
    </ul>
  );
}

export function SeriesIndex({ series, locale }: { series: SeriesSummary[]; locale: Locale }) {
  return (
    <ul>
      {series.map((s) => (
        <li key={s.slug} className="flex flex-col gap-2.5 border-b border-line-soft py-7 first:pt-0">
          <h2 className="text-4xl leading-snug font-semibold">
            <Link to={`/series/${encodeURIComponent(s.slug)}`}>{s.title}</Link>
          </h2>
          {s.description && <p className="text-lg text-ink-soft">{s.description}</p>}
          <span className="text-md leading-loose text-muted">
            共 {s.post_count} 篇 · 最近更新于 <time dateTime={s.last_published_at}>{formatDate(s.last_published_at, locale)}</time>
          </span>
        </li>
      ))}
    </ul>
  );
}

// SeriesChapters 按专栏顺序列出已发布的文章。
export function SeriesChapters({ series, locale }: { series: SeriesDetail; locale: Locale }) {
  return (
    <ol>
      {series.posts.map((p) => (
        <li key={p.slug} className="flex flex-wrap items-baseline gap-x-4 gap-y-1 border-b border-line-soft py-4.5 sm:flex-nowrap sm:gap-5">
          <span className="flex-[0_0_2em] text-base leading-loose text-muted tabular-nums">{String(p.index).padStart(2, "0")}</span>
          <Link to={`/posts/${encodeURIComponent(p.slug)}`} className="flex-1 text-2xl leading-normal font-semibold">
            {p.title}
          </Link>
          <time dateTime={p.published_at} className="basis-full pl-[calc(2em+--spacing(4))] text-md leading-loose text-muted sm:shrink-0 sm:basis-auto sm:pl-0">
            {formatDate(p.published_at, locale)}
          </time>
        </li>
      ))}
    </ol>
  );
}
