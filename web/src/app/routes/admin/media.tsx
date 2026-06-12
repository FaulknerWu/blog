import { useMutation, useQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router";
import { adminApi } from "~/shared/api/admin";
import type { Media } from "~/shared/api/admin-types";
import { formatDate, formatBytes } from "~/shared/lib/format";
import { cn } from "~/shared/lib/cn";
import { button, ConfirmDialog, ui } from "~/shared/ui/admin/ui";
import { MediaGrid, UploadList } from "~/features/media/MediaGrid";
import { acceptedTypes, markdownRef, thumbnail, useUploads } from "~/features/media/uploads";
import { q, queryClient } from "~/app/admin/queries";
import { useLocale } from "~/app/site";

export function meta() {
  return [{ title: "媒体库 · 后台" }];
}

const pageSize = 60;

const group = "flex flex-col gap-2 border-b border-line-soft py-4";

export default function MediaPage() {
  const media = useQuery(q.allMedia);
  const [params, setParams] = useSearchParams();
  const [shown, setShown] = useState(pageSize);
  const [dragging, setDragging] = useState(false);
  const input = useRef<HTMLInputElement>(null);

  const unusedOnly = params.get("filter") === "unused";
  const selectedId = Number(params.get("id")) || undefined;
  const all = media.data ?? [];
  const unused = all.filter((m) => m.reference_count === 0);
  const list = unusedOnly ? unused : all;
  const selected = list.find((m) => m.id === selectedId) ?? list[0];
  const totalBytes = all.reduce((n, m) => n + m.byte_size, 0);

  function update(key: string, value: string | null) {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        if (value) next.set(key, value);
        else next.delete(key);
        return next;
      },
      { replace: true },
    );
  }

  const uploads = useUploads((m: Media) => {
    queryClient.setQueryData(q.allMedia.queryKey, (old) => (old?.some((x) => x.id === m.id) ? old : [m, ...(old ?? [])]));
    update("id", String(m.id));
  });

  return (
    <div
      className="flex flex-col gap-6"
      onDragOver={(e) => {
        if (!e.dataTransfer.types.includes("Files")) return;
        e.preventDefault();
        setDragging(true);
      }}
      onDragLeave={(e) => {
        if (e.currentTarget === e.target) setDragging(false);
      }}
      onDrop={(e) => {
        if (!e.dataTransfer.files.length) return;
        e.preventDefault();
        setDragging(false);
        void uploads.start(e.dataTransfer.files);
      }}
    >
      <header className={ui.pageHead}>
        <h1 className={ui.h1}>媒体库</h1>
        <button type="button" className={button({ variant: "primary" })} onClick={() => input.current?.click()}>
          上传图片
        </button>
        <input
          ref={input}
          type="file"
          accept={acceptedTypes.join(",")}
          multiple
          hidden
          onChange={(e) => {
            if (e.target.files) void uploads.start(e.target.files);
            e.target.value = "";
          }}
        />
      </header>

      <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2">
        <div role="tablist" aria-label="筛选" className={ui.tabs}>
          <button type="button" role="tab" className={ui.tab} aria-selected={!unusedOnly} onClick={() => update("filter", null)}>
            全部<small>{all.length}</small>
          </button>
          <button type="button" role="tab" className={ui.tab} aria-selected={unusedOnly} onClick={() => update("filter", "unused")}>
            未引用<small>{unused.length}</small>
          </button>
        </div>
        <span className="text-sm text-muted">
          JPEG / PNG / WebP，单张不超过 20 MB · 共 {all.length} 张，{formatBytes(totalBytes)}
        </span>
      </div>

      <div className="flex flex-wrap items-start gap-x-12 gap-y-8">
        <div className="flex min-w-0 flex-[999_1_480px] flex-col gap-6">
          <div
            className={cn(
              "flex flex-col gap-3 border-b border-dashed border-control-line py-4",
              dragging && "border-accent bg-accent/6",
            )}
          >
            <UploadList items={uploads.items} onDismiss={uploads.dismiss} />
            <span className="text-sm text-muted">把图片拖到这里，或在写作页直接粘贴</span>
          </div>
          {media.isPending && <p className={ui.empty}>正在加载…</p>}
          {media.isError && <p className={cn(ui.empty, "text-danger")}>{media.error.message}</p>}
          {media.isSuccess && list.length === 0 && (
            <p className={ui.empty}>{unusedOnly ? "没有未引用的图片。" : "媒体库还是空的。"}</p>
          )}
          <MediaGrid items={list.slice(0, shown)} selected={selected?.id} onSelect={(m) => update("id", String(m.id))} />
          {shown < list.length && (
            <div className="flex justify-center">
              <button type="button" className={button()} onClick={() => setShown((n) => n + pageSize)}>
                加载更多
              </button>
            </div>
          )}
        </div>
        {selected && <Detail key={selected.id} media={selected} onDeleted={() => update("id", null)} />}
      </div>
    </div>
  );
}

function Detail({ media, onDeleted }: { media: Media; onDeleted: () => void }) {
  const locale = useLocale();
  const detail = useQuery(q.mediaDetail(media.id));
  const [alt, setAlt] = useState(media.alt);
  const [copied, setCopied] = useState(false);
  const [confirm, setConfirm] = useState(false);

  useEffect(() => setAlt(media.alt), [media.alt]);

  const saveAlt = useMutation({
    mutationFn: () => adminApi.updateMedia(media.id, media.version, alt.trim()),
    onSuccess: (next) => {
      queryClient.setQueryData(q.allMedia.queryKey, (old) => old?.map((m) => (m.id === next.id ? next : m)));
      queryClient.invalidateQueries({ queryKey: q.mediaDetail(media.id).queryKey });
    },
  });

  const remove = useMutation({
    mutationFn: () => adminApi.deleteMedia(media.id, media.version),
    onSuccess: () => {
      setConfirm(false);
      queryClient.setQueryData(q.allMedia.queryKey, (old) => old?.filter((m) => m.id !== media.id));
      onDeleted();
    },
  });

  async function copy() {
    await navigator.clipboard.writeText(markdownRef({ sha256: media.sha256, alt }));
    setCopied(true);
    setTimeout(() => setCopied(false), 1600);
  }

  const refs = detail.data?.referenced_by ?? [];
  const formats = [...new Set(media.variants.map((v) => v.format))].join(" / ");

  return (
    <aside aria-label="图片详情" className="min-w-0 flex-[1_1_280px] lg:sticky lg:top-6 lg:border-l lg:border-line lg:pl-7">
      <a href={media.src} target="_blank" rel="noreferrer" className="block bg-line-soft">
        <img
          src={thumbnail(media, 960)}
          alt={media.alt}
          width={media.width}
          height={media.height}
          className="h-auto max-h-80 w-full object-contain"
        />
      </a>
      <div className={group}>
        <span className="font-serif text-lg leading-normal font-semibold wrap-anywhere">{media.original_filename}</span>
        <dl className="grid grid-cols-[auto_1fr] gap-x-3.5 gap-y-1.5 [&>dd]:text-sm [&>dt]:text-xs [&>dt]:text-muted">
          <dt>尺寸</dt>
          <dd>
            {media.width}×{media.height}
          </dd>
          <dt>大小</dt>
          <dd>{formatBytes(media.byte_size)}</dd>
          <dt>衍生</dt>
          <dd>
            {media.variants.map((v) => v.width).join(" · ")} {formats}
          </dd>
          <dt>哈希</dt>
          <dd className="font-mono wrap-anywhere">{media.sha256}</dd>
          <dt>上传</dt>
          <dd>{formatDate(media.created_at, locale)}</dd>
        </dl>
      </div>
      <div className={group}>
        <label htmlFor="media-alt" className={ui.label}>
          默认替代文本
        </label>
        <textarea
          id="media-alt"
          rows={2}
          className={cn(ui.textarea, "font-serif text-base")}
          value={alt}
          maxLength={500}
          onChange={(e) => setAlt(e.target.value)}
        />
        <span className={ui.hint}>插入文章时作为默认 alt，文章里可单独改写。</span>
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
          {alt.trim() !== media.alt && (
            <button type="button" className={button({ variant: "primary", size: "sm" })} disabled={saveAlt.isPending} onClick={() => saveAlt.mutate()}>
              保存替代文本
            </button>
          )}
          <button type="button" className={ui.linkBtn} onClick={copy}>
            {copied ? "已复制" : "复制 Markdown 引用"}
          </button>
        </div>
        {saveAlt.isError && <span className={ui.error}>{saveAlt.error.message}</span>}
      </div>
      <div className={cn(group, "border-b-0")}>
        <span className={ui.label}>被引用</span>
        {detail.isPending ? (
          <span className={ui.hint}>正在加载…</span>
        ) : refs.length > 0 ? (
          <ul className="font-serif text-base *:py-1">
            {refs.map((r) => (
              <li key={`${r.type}-${r.id}`}>
                <Link to={`/admin/${r.type === "post" ? "posts" : "pages"}/${r.id}`} className={ui.link}>
                  {r.type === "page" ? `页面：${r.title}` : r.title}
                </Link>
              </li>
            ))}
          </ul>
        ) : (
          <>
            <span className={ui.hint}>没有文章或页面引用这张图片。</span>
            <button type="button" className={cn(ui.linkBtn, "self-start text-danger hover:text-danger")} onClick={() => setConfirm(true)}>
              删除图片
            </button>
          </>
        )}
      </div>
      <ConfirmDialog
        open={confirm}
        title="删除这张图片？"
        lead={`${media.original_filename} 及其全部衍生尺寸会被永久删除。`}
        confirmLabel="删除"
        danger
        busy={remove.isPending}
        error={remove.error?.message}
        onConfirm={() => remove.mutate()}
        onClose={() => {
          setConfirm(false);
          remove.reset();
        }}
      />
    </aside>
  );
}
