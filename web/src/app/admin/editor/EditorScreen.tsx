import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate } from "react-router";
import { adminApi, ApiError, upload } from "~/shared/api/admin";
import type { AdminPageSummary, AdminPostSummary, Media, RenderIssue } from "~/shared/api/types";
import { formatShort, formatWhen } from "~/shared/lib/datetime";
import { formatDate, formatNumber, readingMinutes } from "~/shared/lib/format";
import { cn } from "~/shared/lib/cn";
import { button, ConfirmDialog, Dialog, Status, ui } from "~/shared/ui/admin/ui";
import { clearDraft, loadDraft, saveDraft, type LocalDraft } from "~/features/editor/drafts";
import { MarkdownEditor, type EditorHandle } from "~/features/editor/MarkdownEditor";
import { Preview } from "~/features/editor/Preview";
import { Toolbar } from "~/features/editor/Toolbar";
import { useRendered } from "~/features/editor/useRendered";
import { MediaPicker } from "~/features/media/MediaPicker";
import { checkFile, markdownRef } from "~/features/media/uploads";
import { q, queryClient } from "~/app/admin/queries";
import { useRenderer } from "~/app/admin/status";
import { useLocale, useSite } from "~/app/site";
import { CoverField } from "./CoverField";
import {
  emptyForm,
  fieldOf,
  load,
  metaFrom,
  pageInput,
  postInput,
  sameForm,
  withDefaults,
  type EditorForm,
  type Kind,
  type Loaded,
  type Meta,
} from "./model";
import { PublishDialog } from "./PublishDialog";
import { TagInput } from "./TagInput";

// 写作页：顶栏 + 编辑 / 预览 / 属性三栏。宽屏时占满视口、各栏独立滚动，窄屏时自然堆叠。
const pane = "min-w-0 grow xl:h-full xl:border-r xl:border-line";
const bannerBar = "flex flex-wrap items-baseline gap-x-4 gap-y-1 border-b border-line px-6 py-2 text-sm text-ink-soft";
const errorBar = "flex flex-wrap items-baseline gap-x-4 gap-y-1 border-t border-line px-5 py-2.5 text-sm text-danger";
const barButton = cn(ui.linkBtn, "p-0 text-sm");
const group = "flex flex-col gap-2 border-b border-line-soft py-4.5";

type Mode = "write" | "split" | "preview";
const modes: [Mode, string][] = [
  ["write", "写作"],
  ["split", "分屏"],
  ["preview", "预览"],
];
const modeKey = "blog-admin:editor-mode";

type Banner = { kind: "restored"; savedAt: string } | { kind: "older"; draft: LocalDraft<EditorForm> };
type Confirm = "delete" | "unpublish" | null;

interface Props {
  kind: Kind;
  initial: Loaded | null;
  // 新建内容首次保存后调用，由路由替换地址。
  onCreated: (id: number) => void;
}

const labels = {
  post: { list: "文章", listPath: "/admin/posts", noun: "文章", summary: "摘要", pathPrefix: "/posts/" },
  page: { list: "页面", listPath: "/admin/pages", noun: "页面", summary: "描述", pathPrefix: "/" },
};

function readMode(): Mode {
  try {
    const m = localStorage.getItem(modeKey);
    if (m === "write" || m === "split" || m === "preview") return m;
  } catch {
    // 读不到偏好时按屏幕宽度决定。
  }
  return typeof window !== "undefined" && window.innerWidth < 1100 ? "write" : "split";
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

export function EditorScreen({ kind, initial, onCreated }: Props) {
  const L = labels[kind];
  const locale = useLocale();
  const site = useSite();
  const navigate = useNavigate();
  const editor = useRef<EditorHandle>(null);

  const [meta, setMeta] = useState<Meta | null>(initial?.meta ?? null);
  const [saved, setSaved] = useState<EditorForm>(() => initial?.form ?? emptyForm);
  const [form, setForm] = useState<EditorForm>(saved);
  const [editorKey, setEditorKey] = useState(0);
  const [banner, setBanner] = useState<Banner | null>(null);
  const [mode, setMode] = useState<Mode>("split");
  const [cursor, setCursor] = useState({ line: 1, column: 1 });
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Partial<Record<keyof EditorForm, string>>>({});
  const [conflict, setConflict] = useState<{ version: number; updatedAt: string | null } | null>(null);
  const [publishOpen, setPublishOpen] = useState(false);
  const [publishError, setPublishError] = useState<string | null>(null);
  const [confirm, setConfirm] = useState<Confirm>(null);
  const [confirmError, setConfirmError] = useState<string | null>(null);
  const [picker, setPicker] = useState<"insert" | "cover" | null>(null);
  const [uploadErrors, setUploadErrors] = useState<string[]>([]);
  const [notice, setNotice] = useState<string | null>(null);
  // 保存时服务端报告的渲染错误，只对提交时的正文有效。
  const [rejected, setRejected] = useState<{ body: string; issues: RenderIssue[] } | null>(null);

  const series = useQuery({ ...q.seriesList, enabled: kind === "post" }).data;
  const media = useQuery({ ...q.allMedia, enabled: picker !== null }).data;

  const draftKey = `${kind}:${meta?.id ?? "new"}`;
  const dirty = !sameForm(form, saved);
  const { isStale } = useRenderer();
  const stale = meta ? isStale(meta.rendererVersion) : false;
  const { result } = useRendered(form.body);
  const issues = result?.issues;
  const errors = useMemo(() => {
    const previewErrors = issues?.filter((i) => i.severity === "error") ?? [];
    if (previewErrors.length || rejected?.body !== form.body) return previewErrors;
    return rejected.issues;
  }, [issues, rejected, form.body]);
  const errorLines = useMemo(() => errors.map((e) => e.line), [errors]);

  // 首次进入：读取视图偏好，并检查本地是否有未保存的内容。
  useEffect(() => {
    setMode(readMode());
    const local = loadDraft<EditorForm>(draftKey);
    if (!local) return;
    if (sameForm(local.form, saved)) {
      clearDraft(draftKey);
    } else if (local.baseVersion === (meta?.version ?? null)) {
      setForm(local.form);
      setEditorKey((k) => k + 1);
      setBanner({ kind: "restored", savedAt: local.savedAt });
    } else {
      setBanner({ kind: "older", draft: local });
    }
    // 只在挂载时检查一次。
  }, []);

  // 未保存的更改随时暂存到本地。
  useEffect(() => {
    if (!dirty) {
      if (!banner || banner.kind === "restored") clearDraft(draftKey);
      return;
    }
    const timer = setTimeout(
      () => saveDraft(draftKey, { form, baseVersion: meta?.version ?? null, savedAt: new Date().toISOString() }),
      400,
    );
    return () => clearTimeout(timer);
  }, [form, dirty, draftKey, meta?.version, banner]);

  function update<K extends keyof EditorForm>(key: K, value: EditorForm[K]) {
    setForm((f) => ({ ...f, [key]: value }));
    setFieldErrors((e) => ({ ...e, [key]: undefined }));
    setNotice(null);
  }

  function chooseMode(m: Mode) {
    setMode(m);
    try {
      localStorage.setItem(modeKey, m);
    } catch {
      // 偏好保存失败不影响使用。
    }
  }

  function replaceAll(next: EditorForm) {
    setForm(next);
    setEditorKey((k) => k + 1);
  }

  function afterWrite(summary: AdminPostSummary | AdminPageSummary) {
    setMeta(metaFrom(summary));
    queryClient.invalidateQueries({ queryKey: kind === "post" ? ["posts"] : ["pages"] });
    queryClient.invalidateQueries({ queryKey: ["media"] });
    if (kind === "post") {
      queryClient.invalidateQueries({ queryKey: ["series"] });
      queryClient.invalidateQueries({ queryKey: q.tags.queryKey });
    }
  }

  // save 提交当前正文，由服务端渲染；version 用于冲突后强制覆盖。成功时返回最新的服务器状态。
  async function save(version = meta?.version): Promise<Meta | null> {
    if (saving) return null;
    setSaving(true);
    setSaveError(null);
    setFieldErrors({});
    const submitted = withDefaults(kind, form);
    try {
      let summary: AdminPostSummary | AdminPageSummary;
      if (kind === "post") {
        const input = postInput(submitted);
        summary = meta ? await adminApi.updatePost(meta.id, version!, input) : await adminApi.createPost(input);
      } else {
        const input = pageInput(submitted);
        summary = meta ? await adminApi.updatePage(meta.id, version!, input) : await adminApi.createPage(input);
      }
      const created = !meta;
      afterWrite(summary);
      setSaved(submitted);
      // 保存期间可能还在输入；只把自动补全的标题与链接合并进当前表单。
      setForm((cur) => ({
        ...cur,
        title: cur.title === form.title ? submitted.title : cur.title,
        slug: cur.slug === form.slug ? submitted.slug : cur.slug,
      }));
      setBanner(null);
      setConflict(null);
      clearDraft(draftKey);
      if (created) onCreated(summary.id);
      return metaFrom(summary);
    } catch (err) {
      handleWriteError(err, submitted.body);
      return null;
    } finally {
      setSaving(false);
    }
  }

  function handleWriteError(err: unknown, body = form.body) {
    if (err instanceof ApiError && err.status === 412) {
      const version = Number(/\d+/.exec(err.problem?.current_etag ?? "")?.[0]);
      setConflict({ version: Number.isFinite(version) ? version : (meta?.version ?? 0) + 1, updatedAt: err.problem?.updated_at ?? null });
      return;
    }
    if (err instanceof ApiError && err.status === 409) {
      setFieldErrors({ slug: "链接已被占用" });
      setSaveError("链接已被占用，换一个再保存。");
      return;
    }
    // 正文渲染错误与预览中的错误一样标在对应行上，由错误栏提示。
    const renderErrors = err instanceof ApiError ? (err.problem?.errors?.filter((e) => e.code === "render_error") ?? []) : [];
    if (renderErrors.length) {
      setRejected({ body, issues: renderErrors.map((e) => ({ line: e.line ?? 1, message: e.detail, severity: "error" })) });
      return;
    }
    if (err instanceof ApiError && err.problem?.errors?.length) {
      const next: Partial<Record<keyof EditorForm, string>> = {};
      const other: string[] = [];
      for (const e of err.problem.errors) {
        const f = fieldOf(e.pointer);
        if (f === "other") other.push(e.detail);
        else next[f] ??= e.detail;
      }
      setFieldErrors(next);
      setSaveError(other[0] ?? Object.values(next)[0] ?? err.message);
      return;
    }
    setSaveError(errorMessage(err));
  }

  async function overwrite() {
    if (!meta) return;
    try {
      const latest = await load(kind, meta.id);
      await save(latest.meta.version);
    } catch (err) {
      setSaveError(errorMessage(err));
    }
  }

  async function loadLatest() {
    if (!meta) return;
    try {
      // 当前内容先完整暂存到本地，载入后通过横幅可以随时恢复。
      saveDraft(draftKey, { form, baseVersion: meta.version, savedAt: new Date().toISOString() });
      const latest = await load(kind, meta.id);
      setMeta(latest.meta);
      setSaved(latest.form);
      replaceAll(latest.form);
      setConflict(null);
      setBanner({ kind: "older", draft: { form, baseVersion: meta.version, savedAt: new Date().toISOString() } });
    } catch (err) {
      setSaveError(errorMessage(err));
    }
  }

  async function publish(at: string | null) {
    setPublishError(null);
    let current = meta;
    if (!current || dirty || stale) {
      current = await save();
      if (!current) {
        setPublishOpen(false);
        return;
      }
    }
    setSaving(true);
    try {
      const summary =
        kind === "post"
          ? await adminApi.publishPost(current.id, current.version, at)
          : await adminApi.publishPage(current.id, current.version);
      afterWrite(summary);
      setPublishOpen(false);
      setNotice(summary.state === "scheduled" ? `已定时，将于 ${formatShort(summary.published_at!, locale)} 发布` : "已发布");
    } catch (err) {
      if (err instanceof ApiError && err.status === 412) {
        setPublishOpen(false);
        handleWriteError(err);
      } else {
        setPublishError(errorMessage(err));
      }
    } finally {
      setSaving(false);
    }
  }

  async function runConfirm() {
    if (!meta) return;
    setConfirmError(null);
    setSaving(true);
    try {
      if (confirm === "delete") {
        await (kind === "post" ? adminApi.deletePost(meta.id, meta.version) : adminApi.deletePage(meta.id, meta.version));
        clearDraft(draftKey);
        queryClient.invalidateQueries({ queryKey: kind === "post" ? ["posts"] : ["pages"] });
        queryClient.invalidateQueries({ queryKey: ["media"] });
        navigate(L.listPath, { replace: true });
        return;
      }
      const summary =
        kind === "post" ? await adminApi.unpublishPost(meta.id, meta.version) : await adminApi.unpublishPage(meta.id, meta.version);
      afterWrite(summary);
      setConfirm(null);
      setNotice("已撤回，现在是草稿");
    } catch (err) {
      if (err instanceof ApiError && err.status === 412) {
        setConfirm(null);
        handleWriteError(err);
      } else {
        setConfirmError(errorMessage(err));
      }
    } finally {
      setSaving(false);
    }
  }

  const uploadSeq = useRef(0);
  function onFiles(files: File[], pos: number) {
    for (const file of files) {
      const problem = checkFile(file);
      if (problem) {
        setUploadErrors((e) => [...e, `${file.name}：${problem}`]);
        continue;
      }
      const marker = `![上传中 ${++uploadSeq.current}：${file.name.replace(/[[\]]/g, "")}…]()`;
      editor.current?.insertBlockAt(pos, marker);
      upload(file)
        .then((m) => {
          onUploaded(m);
          editor.current?.replaceText(marker, markdownRef(m));
        })
        .catch((err: unknown) => {
          editor.current?.replaceText(marker, "");
          setUploadErrors((e) => [...e, `${file.name}：${errorMessage(err)}`]);
        });
    }
  }

  function onUploaded(m: Media) {
    queryClient.setQueryData(q.allMedia.queryKey, (old) => (old?.some((x) => x.id === m.id) ? old : [m, ...(old ?? [])]));
  }

  const showEditor = mode !== "preview";
  const showPreview = mode !== "write";
  const saveDisabled = saving || errors.length > 0 || (!!meta && !dirty && !stale);
  const seriesInfo = series?.find((s) => s.id === form.seriesId);
  const firstError = errors[0];
  const siteHost = site.siteUrl.replace(/^https?:\/\//, "");
  const path = `${L.pathPrefix}${form.slug}`;

  let status: { text: string; tone?: "warn" } = { text: "尚未保存" };
  if (saving) status = { text: "正在保存…" };
  else if (dirty) status = { text: meta ? `已暂存在本地 · 服务器版本 v${meta.version}（${formatWhen(meta.updatedAt, locale)}）` : "已暂存在本地 · 尚未保存到服务器" };
  else if (stale) status = { text: `由旧版渲染器 ${meta!.rendererVersion} 生成，保存即可重新渲染`, tone: "warn" };
  else if (meta) status = { text: notice ?? `已保存 · v${meta.version}（${formatWhen(meta.updatedAt, locale)}）` };

  // 写作或预览单栏时，正文栏加宽到可读宽度，属性栏靠右。
  const split = mode === "split";

  return (
    <div className="flex min-h-screen flex-col bg-paper font-sans text-md text-ink xl:h-screen">
      <header className="flex flex-wrap items-center gap-x-6 gap-y-2 border-b border-line px-6 py-2.5">
        <Link to={L.listPath} className="py-2 text-md text-accent">
          ← {L.list}
        </Link>
        <div className="flex min-w-0 grow basis-80 items-center gap-3.5">
          <StateBadge meta={meta} locale={locale} />
          <label htmlFor="doc-title" className="sr-only">
            标题
          </label>
          <input
            id="doc-title"
            className="h-10 min-w-0 flex-1 bg-transparent font-serif text-2xl font-semibold text-ink outline-none placeholder:font-normal placeholder:text-muted aria-invalid:underline aria-invalid:decoration-danger aria-invalid:decoration-wavy sm:text-3xl"
            placeholder={kind === "post" ? "文章标题" : "页面标题"}
            value={form.title}
            aria-invalid={!!fieldErrors.title || undefined}
            onChange={(e) => update("title", e.target.value)}
          />
        </div>
        <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
          <span role="status" className={cn("text-xs text-muted", status.tone === "warn" && "text-warn")}>
            {status.text}
          </span>
          <div role="group" aria-label="视图" className="flex gap-4">
            {modes.map(([id, label]) => (
              <button key={id} type="button" className="min-h-9.5 border-b border-transparent text-sm text-muted hover:not-aria-pressed:text-ink aria-pressed:border-accent aria-pressed:text-accent"
                aria-pressed={mode === id} onClick={() => chooseMode(id)}>
                {label}
              </button>
            ))}
          </div>
          <span className="flex gap-2">
            {meta?.state === "published" ? (
              <button type="button" className={button({ variant: "primary" })} disabled={saveDisabled} onClick={() => void save()}>
                保存
              </button>
            ) : (
              <>
                <button
                  type="button"
                  className={button()}
                  disabled={saveDisabled}
                  title={errors.length ? `修复第 ${firstError!.line} 行的错误后才能保存` : undefined}
                  onClick={() => void save()}
                >
                  保存
                </button>
                <button
                  type="button"
                  className={button({ variant: "primary" })}
                  disabled={saving}
                  onClick={() => {
                    setPublishError(null);
                    setPublishOpen(true);
                  }}
                >
                  {meta?.state === "scheduled" ? "修改定时…" : "发布…"}
                </button>
              </>
            )}
          </span>
        </div>
      </header>

      {banner && (
        <div className={bannerBar} role="status">
          {banner.kind === "restored" ? (
            <>
              <span>已恢复本地未保存的更改（{formatWhen(banner.savedAt, locale)}）。</span>
              <button
                type="button"
                className={barButton}
                onClick={() => {
                  replaceAll(saved);
                  clearDraft(draftKey);
                  setBanner(null);
                }}
              >
                丢弃这些更改
              </button>
            </>
          ) : (
            <>
              <span>
                本地有一份{banner.draft.baseVersion ? `基于 v${banner.draft.baseVersion} ` : ""}的未保存版本（
                {formatWhen(banner.draft.savedAt, locale)}），与服务器上的 v{meta?.version} 不同。
              </span>
              <button
                type="button"
                className={barButton}
                onClick={() => {
                  replaceAll(banner.draft.form);
                  setBanner(null);
                }}
              >
                恢复本地版本
              </button>
              <button
                type="button"
                className={cn(barButton, "text-muted hover:text-muted")}
                onClick={() => {
                  clearDraft(draftKey);
                  setBanner(null);
                }}
              >
                丢弃
              </button>
            </>
          )}
        </div>
      )}

      {saveError && (
        <div className={cn(bannerBar, "text-danger")} role="alert">
          <span>{saveError}</span>
          {firstError && saveError.startsWith("第") && (
            <button type="button" className={cn(barButton, "text-danger hover:text-danger")} onClick={() => editor.current?.gotoLine(firstError.line)}>
              跳到错误
            </button>
          )}
        </div>
      )}

      <div className="flex min-h-0 flex-1 flex-wrap xl:flex-nowrap xl:overflow-hidden">
        {showEditor && (
          <section aria-label="Markdown 编辑器" className={cn(pane, split ? "basis-105" : "basis-160", "flex flex-col xl:overflow-hidden")}>
            <Toolbar editor={editor} onImage={() => setPicker("insert")} />
            <div className="min-h-[60vh] flex-1 xl:min-h-0 xl:overflow-hidden">
              <MarkdownEditor
                key={editorKey}
                ref={editor}
                label="正文（Markdown）"
                initialValue={form.body}
                errorLines={errorLines}
                onChange={(body) => update("body", body)}
                onCursor={(line, column) => setCursor({ line, column })}
                onSave={() => void save()}
                onFiles={onFiles}
              />
            </div>
            {uploadErrors.length > 0 && (
              <div role="alert" className={errorBar}>
                <span className="grow basis-65">{uploadErrors.at(-1)}</span>
                <button type="button" className={cn(barButton, "text-danger hover:text-danger")} onClick={() => setUploadErrors([])}>
                  关闭
                </button>
              </div>
            )}
            {firstError && (
              <div role="status" className={errorBar}>
                <span className="grow basis-65">
                  第 {firstError.line} 行：{firstError.message}
                  {errors.length > 1 ? `（共 ${errors.length} 处）` : ""}，修复前无法保存。
                </span>
                <button type="button" className={cn(barButton, "text-danger hover:text-danger")} onClick={() => editor.current?.gotoLine(firstError.line)}>
                  跳到错误
                </button>
              </div>
            )}
            <div className="flex flex-wrap justify-between gap-2 border-t border-line-soft px-5 py-1.5 text-xs text-muted">
              <span>
                第 {cursor.line} 行，第 {cursor.column} 列
              </span>
              <span>
                {formatNumber(result?.word_count ?? 0, locale)} 字 · 约 {readingMinutes(result?.word_count ?? 0)} 分钟 · 渲染器{" "}
                {result?.renderer_version ?? "…"}
              </span>
            </div>
          </section>
        )}
        {showPreview && (
          <section aria-label="预览" className={cn(pane, split ? "basis-105" : "basis-160", "xl:overflow-auto")}>
            <Preview html={result?.html} />
          </section>
        )}
        <aside aria-label={`${L.noun}属性`} className="min-w-65 shrink grow-0 basis-full border-t border-line px-6 pt-1 pb-6 xl:h-full xl:basis-75 xl:overflow-auto xl:border-t-0">
          <div className={group}>
            <label htmlFor="doc-slug" className={ui.label}>
              链接
            </label>
            <div className={cn(
                "flex items-center rounded-xs border border-control-line bg-control focus-within:outline-2 focus-within:-outline-offset-1 focus-within:outline-accent",
                fieldErrors.slug && "border-danger",
              )}>
              <span className="pr-0.5 pl-2.5 font-mono text-xs text-muted">{L.pathPrefix}</span>
              <input
                id="doc-slug"
                className="h-9 min-w-0 flex-1 bg-transparent font-mono text-xs text-ink outline-none"
                placeholder={kind === "post" ? "english-slug" : "about"}
                value={form.slug}
                aria-invalid={!!fieldErrors.slug || undefined}
                onChange={(e) => update("slug", e.target.value.trim().toLowerCase())}
              />
            </div>
            {fieldErrors.slug && <span className={ui.error}>{fieldErrors.slug}</span>}
          </div>

          <div className={group}>
            <div className="flex justify-between">
              <label htmlFor="doc-summary" className={ui.label}>
                {L.summary}
              </label>
              <span className={ui.hint}>{form.summary.length} / 500</span>
            </div>
            <textarea
              id="doc-summary"
              className={cn(ui.textarea, "font-serif text-base")}
              rows={3}
              maxLength={500}
              value={form.summary}
              aria-invalid={!!fieldErrors.summary || undefined}
              onChange={(e) => update("summary", e.target.value)}
            />
            {fieldErrors.summary && <span className={ui.error}>{fieldErrors.summary}</span>}
          </div>

          {kind === "post" && (
            <>
              <div className={group}>
                <span className={ui.label}>标签</span>
                <TagInput value={form.tagIds} onChange={(ids) => update("tagIds", ids)} />
                {fieldErrors.tagIds && <span className={ui.error}>{fieldErrors.tagIds}</span>}
              </div>
              <div className={group}>
                <label htmlFor="doc-series" className={ui.label}>
                  专栏
                </label>
                <select
                  id="doc-series"
                  className={ui.input}
                  value={form.seriesId ?? ""}
                  onChange={(e) => update("seriesId", e.target.value ? Number(e.target.value) : null)}
                >
                  <option value="">不属于专栏</option>
                  {series?.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.title}
                    </option>
                  ))}
                </select>
                {seriesInfo && (
                  <span className={ui.hint}>
                    {meta?.series?.id === seriesInfo.id
                      ? `第 ${meta.series.position} 篇，共 ${seriesInfo.post_count} 篇`
                      : `保存后排在第 ${seriesInfo.post_count + 1} 篇`}
                  </span>
                )}
              </div>
              <div className={group}>
                <span className={ui.label}>封面</span>
                <CoverField
                  cover={form.cover}
                  onChange={(cover) => update("cover", cover)}
                  onBrowse={() => setPicker("cover")}
                  onUploaded={onUploaded}
                />
              </div>
            </>
          )}

          <div className={group}>
            <span className={ui.label}>搜索结果预览</span>
            <span className="font-mono text-xs wrap-anywhere text-ok">{[siteHost, ...path.split("/").filter(Boolean)].join(" › ")}</span>
            <span className="-mt-1 text-base text-accent">
              {form.title || "（无标题）"} - {site.settings.title}
            </span>
            <span className="-mt-1 line-clamp-3 text-sm text-ink-soft">{form.summary || site.settings.description}</span>
          </div>

          {(result?.toc.length ?? 0) > 0 && (
            <div className={group}>
              <span className={ui.label}>目录</span>
              <TocOutline items={result!.toc} />
            </div>
          )}

          {meta && (
            <div className="flex flex-col items-start pt-4.5">
              {meta.state !== "draft" && (
                <button type="button" className={cn(ui.linkBtn, "text-warn hover:text-warn")} onClick={() => setConfirm("unpublish")}>
                  {meta.state === "scheduled" ? "取消定时发布" : "撤回发布"}
                </button>
              )}
              <button type="button" className={cn(ui.linkBtn, "text-danger hover:text-danger")} onClick={() => setConfirm("delete")}>
                删除这{kind === "post" ? "篇文章" : "个页面"}
              </button>
              <span className={ui.hint}>
                创建于 {formatDate(meta.createdAt, locale)} · 版本 v{meta.version}
              </span>
            </div>
          )}
        </aside>
      </div>

      {publishOpen && (
        <PublishDialog
          open
          kind={kind}
          meta={meta}
          form={withDefaults(kind, form)}
          preview={result}
          seriesTitle={seriesInfo?.title}
          locale={locale}
          busy={saving}
          error={publishError ?? saveError}
          onClose={() => setPublishOpen(false)}
          onConfirm={(at) => void publish(at)}
        />
      )}

      <ConfirmDialog
        open={confirm !== null}
        title={confirm === "delete" ? `删除「${form.title || "未命名"}」？` : meta?.state === "scheduled" ? "取消定时发布？" : `撤回这${kind === "post" ? "篇文章" : "个页面"}？`}
        lead={
          confirm === "delete"
            ? "删除后无法恢复。"
            : meta?.state === "scheduled"
              ? "它会回到草稿状态，不会自动发布。"
              : "读者将无法再访问它，订阅源与 Sitemap 也会移除它。"
        }
        confirmLabel={confirm === "delete" ? "删除" : meta?.state === "scheduled" ? "取消定时" : "撤回"}
        danger={confirm === "delete"}
        busy={saving}
        error={confirmError}
        onConfirm={() => void runConfirm()}
        onClose={() => {
          setConfirm(null);
          setConfirmError(null);
        }}
      />

      <Dialog
        open={conflict !== null}
        title={`这${kind === "post" ? "篇文章" : "个页面"}在别处被修改过`}
        lead="本次保存没有执行。"
        onClose={() => setConflict(null)}
        actions={
          <>
            <button type="button" className={button()} disabled={saving} onClick={() => void overwrite()}>
              保留我的版本并覆盖
            </button>
            <button type="button" className={button({ variant: "primary" })} disabled={saving} onClick={() => void loadLatest()}>
              载入 v{conflict?.version} 后继续
            </button>
          </>
        }
      >
        <p className="font-serif text-lg leading-normal text-ink-soft [&_strong]:font-semibold">
          你正在编辑 v{meta?.version}，但服务器上已经是 <strong>v{conflict?.version}</strong>
          {conflict?.updatedAt ? `（${formatWhen(conflict.updatedAt, locale)} 保存）` : ""}。为避免互相覆盖，请先决定保留哪个版本。
        </p>
        <p className={ui.hint}>你的改动已暂存在本地，载入最新版本后不会丢失。</p>
      </Dialog>

      {picker && (
        <MediaPicker
          open
          title={picker === "cover" ? "选择封面" : "插入图片"}
          confirmLabel={picker === "cover" ? "设为封面" : "插入"}
          items={media}
          onClose={() => setPicker(null)}
          onUploaded={onUploaded}
          onPick={(m) => {
            if (picker === "cover") update("cover", { id: m.id, image: m });
            else editor.current?.insertBlockAt(null, markdownRef(m));
            setPicker(null);
          }}
        />
      )}
    </div>
  );
}

function StateBadge({ meta, locale }: { meta: Meta | null; locale: ReturnType<typeof useLocale> }) {
  if (!meta || meta.state === "draft") return <Status kind="draft">草稿</Status>;
  if (meta.state === "scheduled") return <Status kind="scheduled">定时 {formatWhen(meta.publishedAt!, locale)}</Status>;
  return <Status kind="published">已发布</Status>;
}

function TocOutline({ items }: { items: { depth: number; text: string; id: string }[] }) {
  const top = Math.min(...items.map((i) => i.depth));
  return (
    <ol className="font-serif text-md leading-[1.9] text-ink-soft">
      {items.map((i) => (
        <li key={i.id} style={{ paddingLeft: (i.depth - top) * 16 }}>
          {i.text}
        </li>
      ))}
    </ol>
  );
}
