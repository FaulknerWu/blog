import { Link } from "react-router";
import type { Post, PostRef } from "~/shared/api/types";
import { cn } from "~/shared/lib/cn";

// PostNav 在文章末尾提供翻页：属于专栏时按专栏顺序，否则按发布时间。
export function PostNav({ post }: { post: Post }) {
  const { prev, next, prevLabel, nextLabel } = post.series
    ? { prev: post.series.prev, next: post.series.next, prevLabel: "上一篇", nextLabel: "下一篇" }
    : { prev: post.adjacent.older, next: post.adjacent.newer, prevLabel: "较早", nextLabel: "较新" };

  if (!prev && !next) return null;

  return (
    <nav
      aria-label={post.series ? `专栏「${post.series.title}」导航` : "相邻文章"}
      className="flex flex-wrap justify-between gap-5 border-y border-line py-7"
    >
      {prev ? <NavLink post={prev} label={`← ${prevLabel}`} /> : <span className={spacer} />}
      {next ? <NavLink post={next} label={`${nextLabel} →`} className="text-right" /> : <span className={spacer} />}
    </nav>
  );
}

function NavLink({ post, label, className }: { post: PostRef; label: string; className?: string }) {
  return (
    <Link to={`/posts/${encodeURIComponent(post.slug)}`} className={cn("flex grow basis-55 flex-col gap-1.5", className)}>
      <span className="text-sm leading-loose tracking-wider text-muted">{label}</span>
      <span className="text-xl leading-normal font-semibold">{post.title}</span>
    </Link>
  );
}

const spacer = "grow basis-55";
