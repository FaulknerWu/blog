import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router";
import type { AdminPostSummary, PostState } from "~/shared/api/admin-types";
import type { Locale } from "~/shared/lib/format";
import { formatDateTime, formatWhen } from "~/shared/lib/datetime";
import { cn } from "~/shared/lib/cn";
import { button, Status, ui } from "~/shared/ui/admin/ui";
import { isStale } from "~/features/editor/render/version";
import { q } from "~/app/admin/queries";
import { useLocale } from "~/app/site";
import { PostStatus } from "~/app/admin/status";

export function meta() {
  return [{ title: "文章 · 后台" }];
}

const pageSize = 50;

const tabs: { id: PostState | "all"; label: string }[] = [
  { id: "all", label: "全部" },
  { id: "draft", label: "草稿" },
  { id: "scheduled", label: "定时" },
  { id: "published", label: "已发布" },
];

export default function Posts() {
  const locale = useLocale();
  const posts = useQuery(q.allPosts);
  const tags = useQuery(q.tags).data ?? [];
  const [params, setParams] = useSearchParams();
  const [shown, setShown] = useState(pageSize);

  const state = (params.get("state") ?? "all") as PostState | "all";
  const text = params.get("q") ?? "";
  const tagId = Number(params.get("tag")) || undefined;
  const stale = params.get("stale") === "1";

  function setFilter(key: string, value: string | null) {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        if (value) next.set(key, value);
        else next.delete(key);
        return next;
      },
      { replace: true },
    );
    setShown(pageSize);
  }

  const counts = useMemo(() => {
    const c = { all: 0, draft: 0, scheduled: 0, published: 0 };
    for (const p of posts.data ?? []) {
      c.all++;
      c[p.state]++;
    }
    return c;
  }, [posts.data]);

  const rows = useMemo(() => {
    const needle = text.trim().toLowerCase();
    return (posts.data ?? []).filter(
      (p) =>
        (state === "all" || p.state === state) &&
        (!needle || p.title.toLowerCase().includes(needle)) &&
        (!tagId || p.tags.some((t) => t.id === tagId)) &&
        (!stale || isStale(p.renderer_version)),
    );
  }, [posts.data, state, text, tagId, stale]);

  return (
    <div className="flex flex-col gap-6">
      <header className={ui.pageHead}>
        <h1 className={ui.h1}>文章</h1>
        <Link to="/admin/posts/new" className={button({ variant: "primary" })}>
          新建文章
        </Link>
      </header>

      <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2">
        <div role="tablist" aria-label="按状态筛选" className={ui.tabs}>
          {tabs.map((t) => (
            <button
              key={t.id}
              type="button"
              role="tab"
              className={ui.tab}
              aria-selected={state === t.id}
              onClick={() => setFilter("state", t.id === "all" ? null : t.id)}
            >
              {t.label}
              <small>{counts[t.id]}</small>
            </button>
          ))}
        </div>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2 py-1.5">
          <input
            type="search"
            className={cn(ui.input, "w-50")}
            placeholder="按标题筛选"
            aria-label="按标题筛选"
            value={text}
            onChange={(e) => setFilter("q", e.target.value)}
          />
          <select
            className={cn(ui.input, "w-auto")}
            aria-label="按标签筛选"
            value={tagId ?? ""}
            onChange={(e) => setFilter("tag", e.target.value)}
          >
            <option value="">全部标签</option>
            {tags.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </select>
          <label className="inline-flex min-h-9.5 cursor-pointer items-center gap-2 text-md text-ink-soft">
            <input
              type="checkbox"
              className={ui.checkbox}
              checked={stale}
              onChange={(e) => setFilter("stale", e.target.checked ? "1" : null)}
            />
            仅看渲染器过期
          </label>
        </div>
      </div>

      <div className={ui.tableWrap}>
        <table className={cn(ui.table, "min-w-220")}>
          <thead>
            <tr>
              <th scope="col">标题</th>
              <th scope="col">状态</th>
              <th scope="col">标签 / 专栏</th>
              <th scope="col">发布时间</th>
              <th scope="col">最后编辑</th>
              <th scope="col">操作</th>
            </tr>
          </thead>
          <tbody>
            {rows.slice(0, shown).map((p) => (
              <PostRow key={p.id} post={p} locale={locale} />
            ))}
          </tbody>
        </table>
        {posts.isPending && <p className={ui.empty}>正在加载…</p>}
        {posts.isError && <p className={cn(ui.empty, "text-danger")}>{posts.error.message}</p>}
        {posts.isSuccess && rows.length === 0 && (
          <p className={ui.empty}>{counts.all === 0 ? "还没有文章。" : "没有符合条件的文章。"}</p>
        )}
      </div>

      {rows.length > 0 && (
        <div className="flex flex-wrap items-center justify-between gap-3 text-sm text-muted">
          <span>
            按最后编辑时间排序 · 已显示 {Math.min(shown, rows.length)} / {rows.length} 篇
          </span>
          {shown < rows.length && (
            <button type="button" className={button()} onClick={() => setShown((n) => n + pageSize)}>
              加载更多
            </button>
          )}
        </div>
      )}
    </div>
  );
}

function PostRow({ post: p, locale }: { post: AdminPostSummary; locale: Locale }) {
  const edit = `/admin/posts/${p.id}`;
  return (
    <tr>
      <td>
        <div className="flex max-w-110 flex-col gap-1">
          <Link to={edit} className={ui.titleLink}>
            {p.title}
          </Link>
          <span className="flex flex-wrap gap-x-3.5 gap-y-1 text-xs text-muted">
            <span className="font-mono">/posts/{p.slug}</span>
            {isStale(p.renderer_version) && <Status kind="warn">渲染器 {p.renderer_version}</Status>}
          </span>
        </div>
      </td>
      <td>
        <PostStatus post={p} locale={locale} />
      </td>
      <td className="text-ink-soft">
        {p.tags.length > 0 && <div>{p.tags.map((t) => t.name).join("、")}</div>}
        {p.series && <div className="text-muted">{`${p.series.title} · 第 ${p.series.position} 篇`}</div>}
        {!p.tags.length && !p.series && "—"}
      </td>
      <td className="whitespace-nowrap text-ink-soft">{p.published_at ? formatDateTime(p.published_at, locale) : "—"}</td>
      <td className="whitespace-nowrap text-muted">{formatWhen(p.updated_at, locale)}</td>
      <td className="whitespace-nowrap">
        <Link to={edit} className={ui.link}>
          编辑
        </Link>
        {p.state === "published" && (
          <>
            <span className={ui.sep}>·</span>
            <a href={`/posts/${p.slug}`} target="_blank" rel="noreferrer" className={ui.link}>
              查看
            </a>
          </>
        )}
      </td>
    </tr>
  );
}
