import { useMutation, useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useBeforeUnload, useRevalidator } from "react-router";
import { adminApi, ApiError } from "~/shared/api/admin";
import type { AdminSettings, SettingsInput } from "~/shared/api/types";
import { cn } from "~/shared/lib/cn";
import { button, ui } from "~/shared/ui/admin/ui";
import { q, queryClient } from "~/app/admin/queries";

export function meta() {
  return [{ title: "站点设置 · 后台" }];
}

type Form = Omit<AdminSettings, "version" | "updated_at">;
type Key = keyof Form;

const keys: Key[] = [
  "title",
  "subtitle",
  "description",
  "author",
  "language",
  "timezone",
  "footer_text",
  "navigation",
  "social_links",
  "posts_per_page",
  "feed_item_count",
  "noindex",
];

const sections = [
  { id: "basic", label: "基本信息" },
  { id: "menu", label: "导航菜单" },
  { id: "social", label: "社交链接" },
  { id: "reading", label: "阅读与订阅" },
  { id: "seo", label: "搜索引擎" },
];

const languages = [
  ["zh-CN", "简体中文"],
  ["zh-TW", "繁體中文"],
  ["en", "English"],
  ["ja", "日本語"],
];

const platforms = ["github", "x", "mastodon", "email", "rss", "weibo", "zhihu", "bilibili", "telegram", "linkedin"];
const maxItems = 10;

function toForm(s: AdminSettings): Form {
  const { version: _v, updated_at: _u, ...rest } = s;
  return structuredClone(rest);
}

const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);

export default function Settings() {
  const settings = useQuery(q.settings);
  if (settings.isPending) return <p className={ui.empty}>正在加载…</p>;
  if (settings.isError) return <p className={cn(ui.empty, "text-danger")}>{settings.error.message}</p>;
  return <SettingsForm saved={settings.data} />;
}

function SettingsForm({ saved }: { saved: AdminSettings }) {
  const revalidator = useRevalidator();
  const [form, setForm] = useState<Form>(() => toForm(saved));
  const [justSaved, setJustSaved] = useState(false);
  const pages = useQuery(q.pages).data ?? [];

  // 保存成功或载入他处修改后的新版本时，以服务器上的值为准。
  useEffect(() => setForm(toForm(saved)), [saved]);

  const changed = useMemo(() => keys.filter((k) => !same(form[k], saved[k])), [form, saved]);
  useBeforeUnload((e) => {
    if (changed.length) e.preventDefault();
  });

  const save = useMutation({
    mutationFn: () => {
      const patch: SettingsInput = {};
      for (const k of changed) (patch as Record<string, unknown>)[k] = form[k];
      return adminApi.updateSettings(saved.version, patch);
    },
    onSuccess: (next) => {
      queryClient.setQueryData(q.settings.queryKey, next);
      setJustSaved(true);
      revalidator.revalidate();
    },
  });

  function set<K extends Key>(key: K, value: Form[K]) {
    setForm((f) => ({ ...f, [key]: value }));
    setJustSaved(false);
  }

  const err = save.error instanceof ApiError ? save.error : null;
  const invalid = (pointer: string) => err?.problem?.errors?.some((e) => e.pointer?.startsWith(`#/${pointer}`)) || undefined;
  const timezones = useMemo(() => {
    const list = Intl.supportedValuesOf("timeZone");
    return list.includes("UTC") ? list : ["UTC", ...list];
  }, []);

  const navPages = pages.filter((p) => !form.navigation.some((n) => n.url === `/${p.slug}`));

  return (
    <div className="flex flex-col gap-8">
      <header className={ui.pageHead}>
        <h1 className={ui.h1}>站点设置</h1>
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
          {changed.length > 0 ? (
            <span className="text-sm text-warn">{changed.length} 处未保存的更改</span>
          ) : (
            justSaved && (
              <span role="status" className="text-sm text-ok">
                已保存
              </span>
            )
          )}
          {changed.length > 0 && (
            <button type="button" className={cn(ui.linkBtn, "text-muted hover:text-muted")} onClick={() => setForm(toForm(saved))}>
              放弃
            </button>
          )}
          <button
            type="button"
            className={button({ variant: "primary" })}
            disabled={!changed.length || save.isPending}
            onClick={() => save.mutate()}
          >
            保存设置
          </button>
        </div>
      </header>

      {save.isError && (
        <div role="alert" className={cn(ui.notice, "text-danger")}>
          {err?.status === 412 ? (
            <>
              设置已在别处被修改，本次没有保存。
              <button
                type="button"
                className={ui.linkBtn}
                onClick={() => queryClient.invalidateQueries({ queryKey: q.settings.queryKey })}
              >
                载入最新设置
              </button>
            </>
          ) : err?.problem?.errors?.length ? (
            <ul className="list-disc pl-[1.2em]">
              {err.problem.errors.map((e, i) => (
                <li key={i}>{e.detail}</li>
              ))}
            </ul>
          ) : (
            save.error.message
          )}
        </div>
      )}

      <div className="flex flex-wrap items-start gap-x-14 gap-y-8">
        <SectionNav />
        <div className="flex max-w-190 min-w-0 flex-[1_1_520px] flex-col gap-12">
          <Section id="basic" title="基本信息">
            <div className="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-4">
              <Field label="站点标题">
                <input
                  className={cn(ui.input, "font-serif text-base")}
                  value={form.title}
                  required
                  aria-invalid={invalid("title")}
                  onChange={(e) => set("title", e.target.value)}
                />
              </Field>
              <Field label="副标题">
                <input
                  className={cn(ui.input, "font-serif text-base")}
                  value={form.subtitle}
                  aria-invalid={invalid("subtitle")}
                  onChange={(e) => set("subtitle", e.target.value)}
                />
              </Field>
            </div>
            <Field label="站点描述" hint="用作首页的 meta description 与订阅源描述。">
              <textarea
                className={cn(ui.textarea, "font-serif text-base")}
                rows={2}
                value={form.description}
                aria-invalid={invalid("description")}
                onChange={(e) => set("description", e.target.value)}
              />
            </Field>
            <div className="grid grid-cols-[repeat(auto-fit,minmax(160px,1fr))] gap-4">
              <Field label="作者">
                <input
                  className={ui.input}
                  value={form.author}
                  required
                  aria-invalid={invalid("author")}
                  onChange={(e) => set("author", e.target.value)}
                />
              </Field>
              <Field label="语言">
                <select className={ui.input} value={form.language} onChange={(e) => set("language", e.target.value)}>
                  {languages.some(([v]) => v === form.language) ? null : (
                    <option value={form.language}>{form.language}</option>
                  )}
                  {languages.map(([v, label]) => (
                    <option key={v} value={v}>
                      {v} {label}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="时区">
                <select className={ui.input} value={form.timezone} onChange={(e) => set("timezone", e.target.value)}>
                  {timezones.map((tz) => (
                    <option key={tz} value={tz}>
                      {tz}
                    </option>
                  ))}
                </select>
              </Field>
            </div>
            <span className={cn(ui.hint, "-mt-2")}>
              时区决定日期显示、归档年份和定时发布的时间。
            </span>
            <Field label="页脚文字">
              <input
                className={ui.input}
                value={form.footer_text}
                aria-invalid={invalid("footer_text")}
                onChange={(e) => set("footer_text", e.target.value)}
              />
            </Field>
            <div className="flex flex-col items-center gap-1.5 border-y border-line-soft pt-6 pb-5 text-center font-serif">
              <span className="self-start font-sans text-xs tracking-wider text-muted">首页页眉预览</span>
              <span className="text-4xl leading-tight font-bold tracking-wide">{form.title || "　"}</span>
              {form.subtitle && <span className="text-base text-muted italic">{form.subtitle}</span>}
            </div>
          </Section>

          <Section id="menu" title="导航菜单">
            <p className="-mt-1.5 text-sm leading-relaxed text-muted">文章、专栏、标签和搜索是固定入口，这里添加的链接排在它们之后。最多 {maxItems} 项。</p>
            <SortableRows
              items={form.navigation}
              onChange={(items) => set("navigation", items)}
              label={(item) => item.label || "未命名链接"}
              render={(item, i, update) => (
                <>
                  <input
                    className={cn(ui.input, "w-auto flex-[1_1_120px] font-serif text-base")}
                    aria-label="名称"
                    value={item.label}
                    aria-invalid={invalid(`navigation/${i}/label`)}
                    onChange={(e) => update({ ...item, label: e.target.value })}
                  />
                  <input
                    className={cn(ui.input, ui.mono, "w-auto flex-[2_1_200px]")}
                    aria-label="链接"
                    value={item.url}
                    aria-invalid={invalid(`navigation/${i}/url`)}
                    onChange={(e) => update({ ...item, url: e.target.value })}
                  />
                </>
              )}
            />
            <div className="-mt-2 flex flex-wrap items-center gap-x-6 gap-y-1">
              {navPages.length > 0 && (
                <select
                  className={cn(ui.linkBtn, "appearance-none")}
                  aria-label="添加页面"
                  value=""
                  disabled={form.navigation.length >= maxItems}
                  onChange={(e) => {
                    const p = pages.find((x) => String(x.id) === e.target.value);
                    if (p) set("navigation", [...form.navigation, { label: p.title, url: `/${p.slug}` }]);
                  }}
                >
                  <option value="">+ 添加页面</option>
                  {navPages.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.title}（/{p.slug}）
                    </option>
                  ))}
                </select>
              )}
              <button
                type="button"
                className={ui.linkBtn}
                disabled={form.navigation.length >= maxItems}
                onClick={() => set("navigation", [...form.navigation, { label: "", url: "https://" }])}
              >
                + 添加外部链接
              </button>
            </div>
            <span className={cn(ui.hint, "-mt-3")}>
              站内路径以 / 开头；外部链接需要 http(s)。
            </span>
          </Section>

          <Section id="social" title="社交链接">
            <p className="-mt-1.5 text-sm leading-relaxed text-muted">显示在页脚。平台名决定图标，未知平台使用通用链接图标。</p>
            <datalist id="platforms">
              {platforms.map((p) => (
                <option key={p} value={p} />
              ))}
            </datalist>
            <SortableRows
              items={form.social_links}
              onChange={(items) => set("social_links", items)}
              label={(item) => item.platform || "未命名链接"}
              render={(item, i, update) => (
                <>
                  <input
                    className={cn(ui.input, "w-auto flex-[0_1_150px]")}
                    list="platforms"
                    aria-label="平台"
                    value={item.platform}
                    aria-invalid={invalid(`social_links/${i}/platform`)}
                    onChange={(e) => update({ ...item, platform: e.target.value.trim().toLowerCase() })}
                  />
                  <input
                    className={cn(ui.input, ui.mono, "w-auto flex-[1_1_220px]")}
                    aria-label="链接"
                    value={item.url}
                    aria-invalid={invalid(`social_links/${i}/url`)}
                    onChange={(e) => update({ ...item, url: e.target.value })}
                  />
                </>
              )}
            />
            <div className="-mt-2 flex flex-wrap items-center gap-x-6 gap-y-1">
              <button
                type="button"
                className={ui.linkBtn}
                disabled={form.social_links.length >= maxItems}
                onClick={() => set("social_links", [...form.social_links, { platform: "", url: "https://" }])}
              >
                + 添加社交链接
              </button>
            </div>
          </Section>

          <Section id="reading" title="阅读与订阅">
            <div className="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-4">
              <Field label="每页文章数" hint="首页、标签页的分页条数，1–50。">
                <input
                  className={ui.input}
                  type="number"
                  min={1}
                  max={50}
                  value={form.posts_per_page}
                  aria-invalid={invalid("posts_per_page")}
                  onChange={(e) => set("posts_per_page", e.target.valueAsNumber || 0)}
                />
              </Field>
              <Field label="订阅源条数" hint="RSS / Atom 中最近的全文文章数，1–100。">
                <input
                  className={ui.input}
                  type="number"
                  min={1}
                  max={100}
                  value={form.feed_item_count}
                  aria-invalid={invalid("feed_item_count")}
                  onChange={(e) => set("feed_item_count", e.target.valueAsNumber || 0)}
                />
              </Field>
            </div>
            <p className="flex flex-wrap gap-x-5 gap-y-1.5 text-sm text-muted">
              订阅地址
              {["/feed.xml", "/rss.xml", "/sitemap.xml"].map((href) => (
                <a key={href} href={href} target="_blank" rel="noreferrer" className={cn(ui.link, ui.mono)}>
                  {href}
                </a>
              ))}
            </p>
          </Section>

          <Section id="seo" title="搜索引擎">
            <label className="flex cursor-pointer items-start gap-3">
              <input
                type="checkbox"
                className={cn(ui.checkbox, "mt-0.75 accent-danger")}
                checked={form.noindex}
                onChange={(e) => set("noindex", e.target.checked)}
              />
              <span className="flex flex-col gap-1">
                <span className="text-md font-medium">阻止搜索引擎收录</span>
                <span className={ui.hint}>开启后所有页面输出 noindex，Sitemap 返回 404，订阅源附加 X-Robots-Tag。适合上线前试运行。</span>
              </span>
            </label>
            {form.noindex && !saved.noindex && (
              <p role="alert" className={cn(ui.notice, "pl-7 text-danger")}>
                保存后，搜索引擎会逐步把本站从结果中移除；之后关闭此项，重新收录可能需要数周。
              </p>
            )}
          </Section>
        </div>
      </div>
    </div>
  );
}

function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <label className={ui.field}>
      <span className={ui.label}>{label}</span>
      {children}
      {hint && <span className={ui.hint}>{hint}</span>}
    </label>
  );
}

function Section({ id, title, children }: { id: string; title: string; children: ReactNode }) {
  return (
    <section id={id} className="flex scroll-mt-4 flex-col gap-4.5" aria-labelledby={`${id}-h`}>
      <h2 id={`${id}-h`} className={cn(ui.h2, "border-b border-line pb-2.5")}>
        {title}
      </h2>
      {children}
    </section>
  );
}

// SectionNav 高亮当前滚动到的分区。
function SectionNav() {
  const [active, setActive] = useState(sections[0]!.id);
  useEffect(() => {
    const els = sections.map((s) => document.getElementById(s.id)).filter((el): el is HTMLElement => !!el);
    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries.filter((e) => e.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (visible[0]) setActive(visible[0].target.id);
      },
      { rootMargin: "0px 0px -60% 0px" },
    );
    els.forEach((el) => observer.observe(el));
    return () => observer.disconnect();
  }, []);

  return (
    <nav aria-label="设置分区" className="sticky top-6 hidden flex-[0_0_160px] flex-col lg:flex">
      {sections.map((s) => (
        <a
          key={s.id}
          href={`#${s.id}`}
          aria-current={active === s.id ? "true" : undefined}
          className="block border-l border-line py-1.75 pl-3.5 text-md text-muted hover:text-accent aria-[current=true]:border-l-2 aria-[current=true]:border-accent aria-[current=true]:pl-3.25 aria-[current=true]:text-accent"
        >
          {s.label}
        </a>
      ))}
    </nav>
  );
}

interface SortableProps<T> {
  items: T[];
  onChange: (items: T[]) => void;
  label: (item: T) => string;
  render: (item: T, index: number, update: (item: T) => void) => ReactNode;
}

// SortableRows 支持拖动把手排序，把手获得焦点时也可以用上下方向键移动。
function SortableRows<T>({ items, onChange, label, render }: SortableProps<T>) {
  const [dragging, setDragging] = useState<number | null>(null);
  const [armed, setArmed] = useState<number | null>(null);
  const handles = useRef<(HTMLButtonElement | null)[]>([]);

  function move(from: number, to: number) {
    if (to < 0 || to >= items.length || from === to) return;
    const next = items.slice();
    const [item] = next.splice(from, 1);
    next.splice(to, 0, item!);
    onChange(next);
    return to;
  }

  if (items.length === 0) return <p className={cn(ui.hint, "border-y border-line-soft py-2.5")}>还没有添加。</p>;

  return (
    <ol className="border-t border-line-soft">
      {items.map((item, i) => (
        <li
          key={i}
          className={cn(
            "flex flex-wrap items-center gap-x-3 gap-y-2 border-b border-line-soft bg-paper py-2.5",
            dragging === i && "opacity-50",
          )}
          draggable={armed === i}
          onDragStart={(e) => {
            setDragging(i);
            e.dataTransfer.effectAllowed = "move";
          }}
          onDragOver={(e) => {
            if (dragging === null) return;
            e.preventDefault();
            if (dragging !== i) setDragging(move(dragging, i) ?? dragging);
          }}
          onDragEnd={() => {
            setDragging(null);
            setArmed(null);
          }}
        >
          <button
            ref={(el) => {
              handles.current[i] = el;
            }}
            type="button"
            className="inline-flex h-8.5 w-6 cursor-grab items-center justify-center text-muted active:cursor-grabbing"
            aria-label={`调整顺序：${label(item)}（上下方向键移动）`}
            onPointerDown={() => setArmed(i)}
            onPointerUp={() => setArmed(null)}
            onKeyDown={(e) => {
              const to = e.key === "ArrowUp" ? i - 1 : e.key === "ArrowDown" ? i + 1 : null;
              if (to === null) return;
              e.preventDefault();
              if (move(i, to) !== undefined) requestAnimationFrame(() => handles.current[to]?.focus());
            }}
          >
            <svg width="10" height="16" viewBox="0 0 10 16" fill="currentColor" aria-hidden="true">
              {[3, 8, 13].flatMap((y) => [2, 8].map((x) => <circle key={`${x}-${y}`} cx={x} cy={y} r="1.4" />))}
            </svg>
          </button>
          {render(item, i, (next) => onChange(items.map((x, j) => (j === i ? next : x))))}
          <button
            type="button"
            className="inline-flex size-8.5 flex-none items-center justify-center text-muted hover:text-danger"
            aria-label={`移除 ${label(item)}`}
            onClick={() => onChange(items.filter((_, j) => j !== i))}
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden="true">
              <path d="M6 6l12 12M18 6L6 18" />
            </svg>
          </button>
        </li>
      ))}
    </ol>
  );
}
