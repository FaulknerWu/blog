import { Fragment } from "react";
import { Link } from "react-router";
import type { PostSummary } from "~/shared/api/types";
import { formatDate, formatMonthDay, formatNumber, groupByYear, type Locale } from "~/shared/lib/format";

interface Props {
  posts: PostSummary[];
  locale: Locale;
  // 首页与标签页按年份分组；分组后条目只显示月日。
  groupByYear?: boolean;
  emptyText?: string;
}

export function PostList({ posts, locale, groupByYear: grouped = false, emptyText = "还没有文章。" }: Props) {
  if (posts.length === 0) return <p className="py-12 text-center text-muted">{emptyText}</p>;

  if (!grouped) {
    return (
      <ol className="mt-2">
        {posts.map((p) => (
          <Entry key={p.id} post={p} locale={locale} date={formatDate(p.published_at, locale)} />
        ))}
      </ol>
    );
  }

  return groupByYear(posts, locale).map((g) => (
    <Fragment key={g.year}>
      <h2 className="mt-12 text-md leading-loose font-semibold tracking-widest text-muted first:mt-0">{g.year}</h2>
      <ol className="mt-2">
        {g.items.map((p) => (
          <Entry key={p.id} post={p} locale={locale} date={formatMonthDay(p.published_at, locale)} />
        ))}
      </ol>
    </Fragment>
  ));
}

function Entry({ post, locale, date }: { post: PostSummary; locale: Locale; date: string }) {
  return (
    <li className="flex flex-col gap-2.5 border-b border-line-soft py-7">
      <div className={meta}>
        <time dateTime={post.published_at}>{date}</time>
        {post.series && <Link to={`/series/${encodeURIComponent(post.series.slug)}`}>专栏 · {post.series.title}</Link>}
      </div>
      <h3 className="text-4xl leading-snug font-semibold">
        <Link to={`/posts/${encodeURIComponent(post.slug)}`}>{post.title}</Link>
      </h3>
      {post.summary && <p className="text-lg text-ink-soft">{post.summary}</p>}
      <div className={meta}>
        {post.tags.length > 0 && (
          <span className="inline-flex flex-wrap gap-x-[0.6em]">
            {post.tags.map((t) => (
              <Link key={t.slug} to={`/tags/${encodeURIComponent(t.slug)}`}>
                #{t.name}
              </Link>
            ))}
          </span>
        )}
        <span>{formatNumber(post.word_count, locale)} 字</span>
      </div>
    </li>
  );
}

const meta = "flex flex-wrap gap-x-4 gap-y-1.5 text-md text-muted";
