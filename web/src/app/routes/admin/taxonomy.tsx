import { useMutation, useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useSearchParams } from "react-router";
import { adminApi, ApiError } from "~/shared/api/admin";
import type { AdminSeries, AdminSeriesDetail, AdminTag } from "~/shared/api/types";
import { slugError, slugify } from "~/shared/lib/slug";
import { cn } from "~/shared/lib/cn";
import { button, ConfirmDialog, Status, ui } from "~/shared/ui/admin/ui";
import { q, queryClient } from "~/app/admin/queries";

const arrow =
  "inline-flex size-8 items-center justify-center text-ink-soft hover:not-disabled:text-accent disabled:cursor-default disabled:opacity-25";

export function meta() {
  return [{ title: "标签与专栏 · 后台" }];
}

// 标签与专栏变化会影响文章列表里的名称与计数。
function invalidateTaxonomy() {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: q.tags.queryKey }),
    queryClient.invalidateQueries({ queryKey: q.seriesList.queryKey }),
    queryClient.invalidateQueries({ queryKey: ["posts"] }),
  ]);
}

function errorText(err: unknown, field?: string): string | undefined {
  if (!err) return undefined;
  if (err instanceof ApiError) {
    if (err.status === 409) return "名称或链接已被占用";
    return (field && err.fieldError(field)) ?? err.problem?.errors?.[0]?.detail ?? err.message;
  }
  return err instanceof Error ? err.message : String(err);
}

export default function Taxonomy() {
  return (
    <div className="flex flex-col gap-8">
      <h1 className={ui.h1}>标签与专栏</h1>
      <div className="flex flex-wrap items-start gap-x-14 gap-y-12">
        <Tags />
        <SeriesSection />
      </div>
    </div>
  );
}

function Tags() {
  const tags = useQuery(q.tags);
  const [editing, setEditing] = useState<number | "new" | null>(null);
  const [deleting, setDeleting] = useState<AdminTag | null>(null);

  const remove = useMutation({
    mutationFn: (t: AdminTag) => adminApi.deleteTag(t.id, t.version),
    onSuccess: async () => {
      setDeleting(null);
      await invalidateTaxonomy();
    },
  });

  const list = (tags.data ?? []).toSorted((a, b) => b.post_count - a.post_count || a.name.localeCompare(b.name));

  return (
    <section aria-labelledby="tags-h" className="min-w-0 flex-[1_1_420px]">
      <div className={ui.secHead}>
        <h2 id="tags-h" className={ui.h2}>
          标签 <small>{tags.data?.length ?? ""}</small>
        </h2>
        <button type="button" className={cn(ui.linkBtn, "text-sm")} onClick={() => setEditing("new")}>
          + 新建标签
        </button>
      </div>
      <div className={ui.tableWrap}>
        <table className={cn(ui.table, "min-w-105 [&_td]:py-3 [&_td]:align-middle")}>
          <thead>
            <tr>
              <th scope="col">名称</th>
              <th scope="col">链接</th>
              <th scope="col">已发布 / 全部</th>
              <th scope="col">操作</th>
            </tr>
          </thead>
          <tbody>
            {editing === "new" && <TagEditor onDone={() => setEditing(null)} />}
            {list.map((t) =>
              editing === t.id ? (
                <TagEditor key={t.id} tag={t} onDone={() => setEditing(null)} />
              ) : (
                <tr key={t.id}>
                  <td className="font-serif">#{t.name}</td>
                  <td className="font-mono text-muted">/tags/{t.slug}</td>
                  <td className={t.published_post_count === 0 ? "text-warn" : "text-ink-soft"}>
                    {t.published_post_count} / {t.post_count}
                  </td>
                  <td className="whitespace-nowrap">
                    <button type="button" className={ui.linkBtn} onClick={() => setEditing(t.id)}>
                      编辑
                    </button>
                    <span className={ui.sep}>·</span>
                    <button type="button" className={cn(ui.linkBtn, "text-danger hover:text-danger")} onClick={() => setDeleting(t)}>
                      删除
                    </button>
                  </td>
                </tr>
              ),
            )}
          </tbody>
        </table>
        {tags.isPending && <p className={ui.empty}>正在加载…</p>}
        {tags.data?.length === 0 && editing !== "new" && <p className={ui.empty}>还没有标签。</p>}
      </div>
      <ConfirmDialog
        open={deleting !== null}
        title={`删除标签「${deleting?.name ?? ""}」？`}
        lead={deleting?.post_count ? `它会从 ${deleting.post_count} 篇文章上移除。` : undefined}
        confirmLabel="删除"
        danger
        busy={remove.isPending}
        error={errorText(remove.error)}
        onConfirm={() => deleting && remove.mutate(deleting)}
        onClose={() => {
          setDeleting(null);
          remove.reset();
        }}
      />
    </section>
  );
}

function TagEditor({ tag, onDone }: { tag?: AdminTag; onDone: () => void }) {
  const [name, setName] = useState(tag?.name ?? "");
  const [slug, setSlug] = useState(tag?.slug ?? "");
  const [slugTouched, setSlugTouched] = useState(!!tag);
  const [showErrors, setShowErrors] = useState(false);

  const save = useMutation({
    mutationFn: () => {
      if (!tag) return adminApi.createTag(name.trim(), slug);
      const patch: { name?: string; slug?: string } = {};
      if (name.trim() !== tag.name) patch.name = name.trim();
      if (slug !== tag.slug) patch.slug = slug;
      return adminApi.updateTag(tag.id, tag.version, patch);
    },
    onSuccess: async () => {
      await invalidateTaxonomy();
      onDone();
    },
  });

  const localError = !name.trim() ? "请填写名称" : slugError(slug);
  const message = (showErrors && localError) || errorText(save.error, "slug");

  function submit() {
    setShowErrors(true);
    if (!localError) save.mutate();
  }

  return (
    <tr className="*:align-top!">
      <td>
        <input
          className={cn(ui.input, "w-35 font-serif text-base")}
          aria-label="名称"
          autoFocus
          value={name}
          onChange={(e) => {
            setName(e.target.value);
            if (!slugTouched) setSlug(slugify(e.target.value));
          }}
          onKeyDown={(e) => e.key === "Enter" && submit()}
        />
      </td>
      <td>
        <input
          className={cn(ui.input, ui.mono, "w-35")}
          aria-label="链接"
          placeholder="english-slug"
          value={slug}
          aria-invalid={(showErrors && !!slugError(slug)) || undefined}
          onChange={(e) => {
            setSlugTouched(true);
            setSlug(e.target.value.trim());
          }}
          onKeyDown={(e) => e.key === "Enter" && submit()}
        />
        {message && <div className={ui.error}>{message}</div>}
      </td>
      <td className="text-ink-soft">{tag ? `${tag.published_post_count} / ${tag.post_count}` : "—"}</td>
      <td className="whitespace-nowrap">
        <button type="button" className={cn(ui.linkBtn, "mr-3 text-muted hover:text-muted")} onClick={onDone}>
          取消
        </button>
        <button type="button" className={button({ variant: "primary", size: "sm" })} disabled={save.isPending} onClick={submit}>
          保存
        </button>
      </td>
    </tr>
  );
}

function SeriesSection() {
  const list = useQuery(q.seriesList);
  const [params, setParams] = useSearchParams();
  const param = params.get("series");
  const selected = param === "new" ? "new" : Number(param) || list.data?.[0]?.id;

  function select(id: number | "new") {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        next.set("series", String(id));
        return next;
      },
      { replace: true },
    );
  }

  return (
    <section aria-labelledby="series-h" className="min-w-0 flex-[1_1_520px]">
      <div className={ui.secHead}>
        <h2 id="series-h" className={ui.h2}>
          专栏 <small>{list.data?.length ?? ""}</small>
        </h2>
        <button type="button" className={cn(ui.linkBtn, "text-sm")} onClick={() => select("new")}>
          + 新建专栏
        </button>
      </div>
      <div className="flex flex-wrap gap-x-8 gap-y-6 pt-5">
        {list.data && list.data.length > 0 && (
          <nav aria-label="专栏列表" className="flex flex-[1_1_160px] flex-col">
            {list.data?.map((s) => (
              <a
                key={s.id}
                href={`?series=${s.id}`}
                aria-current={selected === s.id ? "true" : undefined}
                className="flex flex-col gap-0.5 border-l border-line py-2 pl-3.5 text-ink hover:text-accent aria-[current=true]:border-l-2 aria-[current=true]:border-accent aria-[current=true]:pl-3.25 aria-[current=true]:text-accent"
                onClick={(e) => {
                  e.preventDefault();
                  select(s.id);
                }}
              >
                <span className="font-serif text-base leading-normal font-semibold">{s.title}</span>
                <span className="text-xs text-muted">
                  已发布 {s.published_post_count} / 全部 {s.post_count}
                </span>
              </a>
            ))}
          </nav>
        )}
        <div className="min-w-0 flex-[999_1_280px]">
          {selected === "new" ? (
            <SeriesEditor key="new" onSaved={(s) => select(s.id)} onDeleted={() => undefined} />
          ) : selected ? (
            <SeriesLoader key={selected} id={selected} onDeleted={() => setParams({}, { replace: true })} />
          ) : (
            list.isSuccess && <p className={ui.hint}>还没有专栏。</p>
          )}
        </div>
      </div>
    </section>
  );
}

function SeriesLoader({ id, onDeleted }: { id: number; onDeleted: () => void }) {
  const detail = useQuery(q.series(id));
  if (detail.isPending) return <p className={ui.hint}>正在加载…</p>;
  if (detail.isError) return <p className={cn(ui.notice, "text-danger")}>{detail.error.message}</p>;
  return <SeriesEditor key={detail.data.version} series={detail.data} onSaved={() => undefined} onDeleted={onDeleted} />;
}

function SeriesEditor({
  series,
  onSaved,
  onDeleted,
}: {
  series?: AdminSeriesDetail;
  onSaved: (s: AdminSeries) => void;
  onDeleted: () => void;
}) {
  const [title, setTitle] = useState(series?.title ?? "");
  const [slug, setSlug] = useState(series?.slug ?? "");
  const [slugTouched, setSlugTouched] = useState(!!series);
  const [description, setDescription] = useState(series?.description ?? "");
  const [order, setOrder] = useState(() => (series?.posts ?? []).toSorted((a, b) => a.position - b.position));
  const [showErrors, setShowErrors] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [saved, setSaved] = useState(false);

  useEffect(() => setSaved(false), [title, slug, description, order]);

  const orderChanged = series ? order.some((p, i) => p.position !== i + 1) : false;
  const localError = !title.trim() ? "请填写名称" : slugError(slug);

  const save = useMutation({
    mutationFn: async () => {
      if (!series) return adminApi.createSeries({ title: title.trim(), slug, description });
      const patch: { title?: string; slug?: string; description?: string } = {};
      if (title.trim() !== series.title) patch.title = title.trim();
      if (slug !== series.slug) patch.slug = slug;
      if (description !== series.description) patch.description = description;
      let result: AdminSeries = series;
      if (Object.keys(patch).length) result = await adminApi.updateSeries(series.id, series.version, patch);
      if (orderChanged) result = await adminApi.reorderSeries(series.id, order.map((p) => p.id));
      return result;
    },
    onSuccess: async (s) => {
      setSaved(true);
      await Promise.all([invalidateTaxonomy(), queryClient.invalidateQueries({ queryKey: q.series(s.id).queryKey })]);
      onSaved(s);
    },
  });

  const remove = useMutation({
    mutationFn: () => adminApi.deleteSeries(series!.id, series!.version),
    onSuccess: async () => {
      setConfirmDelete(false);
      await invalidateTaxonomy();
      onDeleted();
    },
  });

  function move(i: number, d: number) {
    const next = order.slice();
    [next[i], next[i + d]] = [next[i + d]!, next[i]!];
    setOrder(next);
  }

  function submit() {
    setShowErrors(true);
    if (!localError) save.mutate();
  }

  const message = (showErrors && localError) || errorText(save.error, "slug");

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-3">
        <label className={ui.field}>
          <span className={ui.label}>名称</span>
          <input
            className={cn(ui.input, "font-serif text-base")}
            value={title}
            autoFocus={!series}
            onChange={(e) => {
              setTitle(e.target.value);
              if (!slugTouched) setSlug(slugify(e.target.value));
            }}
          />
        </label>
        <label className={ui.field}>
          <span className={ui.label}>链接</span>
          <input
            className={cn(ui.input, ui.mono)}
            placeholder="english-slug"
            value={slug}
            aria-invalid={(showErrors && !!slugError(slug)) || undefined}
            onChange={(e) => {
              setSlugTouched(true);
              setSlug(e.target.value.trim());
            }}
          />
        </label>
      </div>
      <label className={ui.field}>
        <span className={ui.label}>简介</span>
        <textarea className={cn(ui.textarea, "font-serif text-base")} rows={2} value={description} onChange={(e) => setDescription(e.target.value)} />
      </label>
      {series && (
        <div className={ui.field}>
          <span className={ui.label}>文章顺序</span>
          {order.length === 0 ? (
            <p className={cn(ui.hint, "border-t border-line py-2.5")}>还没有文章加入这个专栏。</p>
          ) : (
            <ol className="border-t border-line">
              {order.map((p, i) => (
                <li key={p.id} className="flex items-center gap-3 border-b border-line-soft py-1.5">
                  <span className="flex-[0_0_24px] font-serif text-md text-muted">{i + 1}</span>
                  <span className="min-w-0 flex-1 truncate font-serif text-base">{p.title}</span>
                  {p.state === "published" ? (
                    <Status kind="published">已发布</Status>
                  ) : p.state === "scheduled" ? (
                    <Status kind="scheduled">定时</Status>
                  ) : (
                    <Status kind="draft">草稿</Status>
                  )}
                  <span className="inline-flex">
                    <button type="button" className={arrow} aria-label={`上移 ${p.title}`} disabled={i === 0} onClick={() => move(i, -1)}>
                      ↑
                    </button>
                    <button
                      type="button"
                      className={arrow}
                      aria-label={`下移 ${p.title}`}
                      disabled={i === order.length - 1}
                      onClick={() => move(i, 1)}
                    >
                      ↓
                    </button>
                  </span>
                </li>
              ))}
            </ol>
          )}
        </div>
      )}
      {message && (
        <p role="alert" className={ui.error}>
          {message}
        </p>
      )}
      <div className="flex flex-wrap items-center justify-between gap-2">
        {series ? (
          <button type="button" className={cn(ui.linkBtn, "text-danger hover:text-danger")} onClick={() => setConfirmDelete(true)}>
            删除专栏
          </button>
        ) : (
          <span />
        )}
        <span className="flex items-center gap-4">
          {saved && !save.isPending && (
            <span role="status" className="text-sm text-ok">
              已保存
            </span>
          )}
          <button type="button" className={button({ variant: "primary" })} disabled={save.isPending} onClick={submit}>
            {series ? "保存专栏" : "创建专栏"}
          </button>
        </span>
      </div>
      <ConfirmDialog
        open={confirmDelete}
        title={`删除专栏「${series?.title ?? ""}」？`}
        lead={series?.post_count ? `其中的 ${series.post_count} 篇文章会移出专栏，文章本身不受影响。` : undefined}
        confirmLabel="删除"
        danger
        busy={remove.isPending}
        error={errorText(remove.error)}
        onConfirm={() => remove.mutate()}
        onClose={() => {
          setConfirmDelete(false);
          remove.reset();
        }}
      />
    </div>
  );
}
