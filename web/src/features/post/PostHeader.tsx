import { Link } from "react-router";
import type { Post } from "~/shared/api/types";
import { formatDate, formatNumber, readingMinutes, type Locale } from "~/shared/lib/format";
import { ResponsiveImage } from "~/shared/ui/ResponsiveImage";

export function PostHeader({ post, locale }: { post: Post; locale: Locale }) {
  const published = formatDate(post.published_at, locale);
  const updated = formatDate(post.updated_at, locale);
  const showUpdated = updated !== published && post.updated_at > post.published_at;

  return (
    <>
      {post.series && (
        <div className="mb-4.5 text-md leading-loose tracking-wider text-muted *:[a]:text-accent">
          <Link to={`/series/${encodeURIComponent(post.series.slug)}`}>{post.series.title}</Link> · 第 {post.series.index} 篇
        </div>
      )}
      <h1 className="text-5xl leading-tight font-bold md:text-6xl xl:text-7xl">{post.title}</h1>
      {post.summary && <p className="mt-4.5 text-2xl leading-relaxed text-ink-soft italic">{post.summary}</p>}
      <div className="mt-6 flex flex-wrap gap-x-4 gap-y-1.5 border-b border-line-soft pb-8 text-md text-muted">
        <time dateTime={post.published_at}>{published}</time>
        {showUpdated && (
          <span>
            更新于 <time dateTime={post.updated_at}>{updated}</time>
          </span>
        )}
        <span>
          {formatNumber(post.word_count, locale)} 字 · 约 {readingMinutes(post.word_count)} 分钟
        </span>
        {post.tags.map((t) => (
          <Link key={t.slug} to={`/tags/${encodeURIComponent(t.slug)}`}>
            #{t.name}
          </Link>
        ))}
      </div>
      {post.cover && (
        <ResponsiveImage image={post.cover} sizes="(min-width: 712px) 680px, 100vw" className="mt-8 block w-full rounded-sm" priority />
      )}
    </>
  );
}
