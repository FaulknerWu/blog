import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router";
import { formatShort, formatWhen, zonedParts } from "~/shared/lib/datetime";
import { formatBytes, formatMonthDay } from "~/shared/lib/format";
import { describeUserAgent } from "~/shared/lib/useragent";
import { cn } from "~/shared/lib/cn";
import { button, Status, ui } from "~/shared/ui/admin/ui";
import { isStale, RENDERER_VERSION } from "~/features/editor/render/version";
import { q } from "~/app/admin/queries";
import { useLocale } from "~/app/site";

export function meta() {
  return [{ title: "概览 · 后台" }];
}

function greeting(hour: number): string {
  if (hour < 5) return "夜深了";
  if (hour < 11) return "早上好";
  if (hour < 13) return "中午好";
  if (hour < 18) return "下午好";
  return "晚上好";
}

// 待办与列表行共用的样式：细线分隔的一行。
const todoRow = "flex flex-wrap items-baseline gap-x-5 gap-y-1 border-b border-line-soft py-3.5 text-md";
const todoKind = "flex-[0_0_96px]";
const todoText = "flex-[1_1_360px] leading-relaxed";
const statLink =
  "flex flex-col gap-1 border-l border-line-soft py-5 pl-6 text-ink first:border-l-0 first:pl-0 hover:text-accent";
const statValue = "font-serif text-6xl leading-tight font-semibold";

export default function Dashboard() {
  const locale = useLocale();
  const me = useQuery(q.me).data;
  const posts = useQuery(q.allPosts).data ?? [];
  const pages = useQuery(q.pages).data ?? [];
  const media = useQuery(q.allMedia).data ?? [];
  const sessions = useQuery(q.sessions).data ?? [];

  const now = new Date();
  const today = new Intl.DateTimeFormat(locale.language, { timeZone: locale.timezone, dateStyle: "full" }).format(now);

  const drafts = posts.filter((p) => p.state === "draft");
  const scheduled = posts
    .filter((p) => p.state === "scheduled")
    .sort((a, b) => (a.published_at ?? "").localeCompare(b.published_at ?? ""));
  const published = posts
    .filter((p) => p.state === "published")
    .sort((a, b) => (b.published_at ?? "").localeCompare(a.published_at ?? ""));
  const stalePosts = posts.filter((p) => isStale(p.renderer_version)).length;
  const stalePages = pages.filter((p) => isStale(p.renderer_version)).length;
  const unused = media.filter((m) => m.reference_count === 0);
  const unusedBytes = unused.reduce((n, m) => n + m.byte_size, 0);
  const lastLogin = sessions.toSorted((a, b) => b.created_at.localeCompare(a.created_at))[0];

  const stats = [
    { label: "已发布文章", value: published.length, to: "/admin/posts?state=published" },
    { label: "草稿", value: drafts.length, to: "/admin/posts?state=draft" },
    { label: "定时发布", value: scheduled.length, to: "/admin/posts?state=scheduled" },
  ];
  const hasTodo = stalePosts + stalePages > 0 || scheduled.length > 0 || unused.length > 0;

  return (
    <div className="flex flex-col gap-10">
      <header className={ui.pageHead}>
        <div className="flex flex-col gap-1.5">
          <h1 className={ui.h1}>
            {greeting(zonedParts(now, locale.timezone).hour)}，{me?.display_name}
          </h1>
          <p className="font-serif text-base text-muted italic">
            {today} · {locale.timezone}
          </p>
        </div>
        <Link to="/admin/posts/new" className={button({ variant: "primary" })}>
          新建文章
        </Link>
      </header>

      <section aria-label="内容统计" className="grid grid-cols-[repeat(auto-fit,minmax(150px,1fr))] border-y border-line">
        {stats.map((s) => (
          <Link key={s.label} to={s.to} className={statLink}>
            <span className="text-sm text-muted">{s.label}</span>
            <span className={statValue}>{s.value}</span>
          </Link>
        ))}
        <Link to="/admin/media?filter=unused" className={statLink}>
          <span className="text-sm text-muted">图片（未引用）</span>
          <span className={statValue}>
            {media.length}
            <small className="text-xl leading-tight font-normal text-muted">（{unused.length}）</small>
          </span>
        </Link>
      </section>

      {hasTodo && (
        <section>
          <div className={ui.secHead}>
            <h2 className={ui.h2}>需要处理</h2>
          </div>
          {stalePosts + stalePages > 0 && (
            <div className={todoRow}>
              <span className={todoKind}>
                <Status kind="warn">渲染器过期</Status>
              </span>
              <span className={todoText}>
                {[stalePosts && `${stalePosts} 篇文章`, stalePages && `${stalePages} 个页面`].filter(Boolean).join("与")}
                由旧版渲染器生成，当前为 <span className={ui.mono}>{RENDERER_VERSION}</span>。
              </span>
              <Link to={stalePosts ? "/admin/posts?stale=1" : "/admin/pages"} className={ui.link}>
                逐篇重新渲染 →
              </Link>
            </div>
          )}
          {scheduled.map((p) => (
            <div key={p.id} className={todoRow}>
              <span className={todoKind}>
                <Status kind="scheduled">即将发布</Status>
              </span>
              <span className={todoText}>
                《{p.title}》将于 {formatShort(p.published_at!, locale)} 自动发布。
              </span>
              <Link to={`/admin/posts/${p.id}`} className={ui.link}>
                检查 →
              </Link>
            </div>
          ))}
          {unused.length > 0 && (
            <div className={todoRow}>
              <span className={todoKind}>
                <Status kind="draft">可清理</Status>
              </span>
              <span className={todoText}>
                {unused.length} 张图片没有被任何文章或页面引用，共占用 {formatBytes(unusedBytes)}。
              </span>
              <Link to="/admin/media?filter=unused" className={ui.link}>
                查看未引用图片 →
              </Link>
            </div>
          )}
        </section>
      )}

      <div className="flex flex-wrap items-start gap-x-14 gap-y-10">
        <section className="min-w-0 flex-[1_1_360px]">
          <div className={ui.secHead}>
            <h2 className={ui.h2}>继续写作</h2>
            <Link to="/admin/posts?state=draft" className={cn(ui.link, "text-sm")}>
              全部草稿
            </Link>
          </div>
          <ul>
            {drafts.slice(0, 5).map((p) => (
              <li key={p.id} className="flex flex-col gap-1 border-b border-line-soft py-3.5">
                <Link to={`/admin/posts/${p.id}`} className={ui.titleLink}>
                  {p.title}
                </Link>
                <span className="text-xs text-muted">
                  {["草稿", p.series && `${p.series.title} 第 ${p.series.position} 篇`, `${formatWhen(p.updated_at, locale)} 编辑`]
                    .filter(Boolean)
                    .join(" · ")}
                </span>
              </li>
            ))}
          </ul>
          {drafts.length === 0 && <p className={ui.empty}>没有草稿。</p>}
        </section>
        <section className="min-w-0 flex-[1_1_360px]">
          <div className={ui.secHead}>
            <h2 className={ui.h2}>最近发布</h2>
            <Link to="/admin/posts?state=published" className={cn(ui.link, "text-sm")}>
              全部文章
            </Link>
          </div>
          <ul>
            {published.slice(0, 5).map((p) => (
              <li key={p.id} className="flex items-baseline gap-4 border-b border-line-soft py-3.5">
                <span className="min-w-16 flex-none text-sm text-muted">{formatMonthDay(p.published_at!, locale)}</span>
                <Link to={`/admin/posts/${p.id}`} className="min-w-0 flex-1 truncate font-serif text-lg leading-normal text-ink hover:text-accent">
                  {p.title}
                </Link>
                <a href={`/posts/${p.slug}`} target="_blank" rel="noreferrer" className={cn(ui.link, "text-sm")}>
                  查看 ↗
                </a>
              </li>
            ))}
          </ul>
          {published.length === 0 && <p className={ui.empty}>还没有发布过文章。</p>}
        </section>
      </div>

      {lastLogin && (
        <p className="border-t border-line-soft pt-4 text-sm text-muted">
          {sessions.length} 个活跃会话 · 最近一次登录：{describeUserAgent(lastLogin.user_agent)}，
          {formatWhen(lastLogin.created_at, locale)}，IP <span className={ui.mono}>{lastLogin.ip}</span> ·{" "}
          <Link to="/admin/account" className={ui.link}>
            管理会话
          </Link>
        </p>
      )}
    </div>
  );
}
