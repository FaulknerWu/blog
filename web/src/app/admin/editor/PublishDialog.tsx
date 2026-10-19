import { useState, type ReactNode } from "react";
import { formatDateTime, formatShort, fromZonedInput, toZonedInput } from "~/shared/lib/datetime";
import type { Locale } from "~/shared/lib/format";
import { slugError } from "~/shared/lib/slug";
import { cn } from "~/shared/lib/cn";
import { button, Dialog, ui } from "~/shared/ui/admin/ui";
import type { RenderPreview } from "~/shared/api/types";
import type { EditorForm, Kind, Meta } from "./model";

const option = "flex cursor-pointer items-start gap-3 border-b border-line-soft py-3.5";
const radio = "mt-1 size-4 accent-accent";
const levelColor = { ok: "text-ok", warn: "text-warn", fail: "text-danger" };

interface Props {
  open: boolean;
  kind: Kind;
  meta: Meta | null;
  form: EditorForm;
  preview: RenderPreview | undefined;
  seriesTitle: string | undefined;
  locale: Locale;
  busy: boolean;
  error: string | null;
  onClose: () => void;
  // at 为 null 表示立即发布（曾发布过的文章沿用原发布时间）。
  onConfirm: (at: string | null) => void;
}

type Check = { level: "ok" | "warn" | "fail"; text: ReactNode };

function defaultSchedule(meta: Meta | null, timeZone: string) {
  if (meta?.state === "scheduled" && meta.publishedAt) return toZonedInput(new Date(meta.publishedAt), timeZone);
  const tomorrow = toZonedInput(new Date(Date.now() + 86_400_000), timeZone);
  return { date: tomorrow.date, time: "09:00" };
}

export function PublishDialog({ open, kind, meta, form, preview, seriesTitle, locale, busy, error, onClose, onConfirm }: Props) {
  const [when, setWhen] = useState<"now" | "later">(meta?.state === "scheduled" ? "later" : "now");
  const [schedule, setSchedule] = useState(() => defaultSchedule(meta, locale.timezone));

  const issues = preview?.issues;
  const errors = issues?.filter((i) => i.severity === "error") ?? [];
  const missingAlt = issues?.filter((i) => i.message === "图片缺少替代文本").length ?? 0;
  const path = kind === "post" ? `/posts/${form.slug}` : `/${form.slug}`;
  const at = when === "later" && schedule.date && schedule.time ? fromZonedInput(schedule.date, schedule.time, locale.timezone) : null;
  const pastSchedule = when === "later" && (!at || at.getTime() <= Date.now());
  // 撤回后重新发布时，API 沿用原发布时间。
  const republish = meta?.state === "draft" && meta.publishedAt;

  const checks: Check[] = [];
  if (!form.body.trim()) checks.push({ level: "fail", text: "正文为空，无法发布" });
  else if (errors.length) checks.push({ level: "fail", text: `正文有 ${errors.length} 处错误，修复后才能发布` });
  else
    checks.push({
      level: "ok",
      text: (
        <>
          正文已用渲染器 <span className="font-mono text-xs">{preview?.renderer_version}</span> 渲染，无公式或语法错误
        </>
      ),
    });
  const slugProblem = form.slug ? slugError(form.slug) : undefined;
  checks.push(
    slugProblem
      ? { level: "fail", text: `链接不可用：${slugProblem}` }
      : {
          level: "ok",
          text: form.slug ? (
            <>
              链接 <span className="font-mono text-xs">{path}</span>
            </>
          ) : (
            "保存时自动生成链接"
          ),
        },
  );
  if (kind === "post") {
    const series = seriesTitle ? `，属于专栏「${seriesTitle}」` : "";
    checks.push(
      form.summary.trim()
        ? { level: "ok", text: `已填写摘要${series}` }
        : { level: "warn", text: `还没有填写摘要，列表与订阅源中将不显示简介${series}` },
    );
  } else if (!form.summary.trim()) {
    checks.push({ level: "warn", text: "还没有填写描述，搜索引擎将自行截取正文" });
  }
  if (missingAlt) checks.push({ level: "warn", text: `${missingAlt} 张图片缺少替代文本，读屏用户将无法理解它` });

  const blocked = checks.some((c) => c.level === "fail") || pastSchedule;
  const confirmLabel =
    when === "later" && at && !pastSchedule ? `定时发布于 ${formatShort(at.toISOString(), locale)}` : "立即发布";

  return (
    <Dialog
      open={open}
      title={kind === "post" ? (meta?.state === "scheduled" ? "修改定时发布" : "发布文章") : "发布页面"}
      lead={kind === "post" ? "发布后出现在首页、标签页、RSS / Atom 订阅源与 Sitemap 中。" : undefined}
      onClose={onClose}
      actions={
        <>
          <button type="button" className={button()} onClick={onClose}>
            取消
          </button>
          <button
            type="button"
            className={button({ variant: "primary" })}
            disabled={blocked || busy}
            onClick={() => onConfirm(when === "later" && at ? at.toISOString() : null)}
          >
            {busy ? "正在发布…" : confirmLabel}
          </button>
        </>
      }
    >
      {kind === "post" && (
        <fieldset>
          <legend className="sr-only">发布时间</legend>
          <label className={option}>
            <input type="radio" className={radio} name="when" checked={when === "now"} onChange={() => setWhen("now")} />
            <span className="flex flex-col gap-0.5">
              <span className="text-base font-medium">立即发布</span>
              <span className={ui.hint}>
                {republish ? `沿用原发布时间 ${formatDateTime(meta.publishedAt!, locale)}` : "发布时间记为现在"}
              </span>
            </span>
          </label>
          <label className={option}>
            <input type="radio" className={radio} name="when" checked={when === "later"} onChange={() => setWhen("later")} />
            <span className="flex flex-col gap-0.5">
              <span className="text-base font-medium">定时发布</span>
              <span className={ui.hint}>到点后对读者可见，之前可随时撤回</span>
            </span>
          </label>
          {when === "later" && (
            <div className="flex flex-wrap items-end gap-x-3.5 gap-y-2.5 border-b border-line-soft py-3.5 pl-7">
              <label className={ui.field}>
                <span className={ui.label}>日期</span>
                <input
                  className={ui.input}
                  type="date"
                  value={schedule.date}
                  onChange={(e) => setSchedule((s) => ({ ...s, date: e.target.value }))}
                />
              </label>
              <label className={ui.field}>
                <span className={ui.label}>时间</span>
                <input
                  className={ui.input}
                  type="time"
                  value={schedule.time}
                  onChange={(e) => setSchedule((s) => ({ ...s, time: e.target.value }))}
                />
              </label>
              <span className={cn(ui.hint, "pb-2.5")}>站点时区 {locale.timezone}</span>
              {pastSchedule && <span className={cn(ui.error, "basis-full")}>定时发布的时间需要晚于现在。</span>}
            </div>
          )}
        </fieldset>
      )}
      <div>
        <p className="mb-1 text-xs font-medium tracking-wide text-muted">发布前检查</p>
        {checks.map((c, i) => (
          <div key={i} className="flex items-baseline gap-2.5 py-1.5 text-md">
            <span role="img" aria-label={c.level === "ok" ? "通过" : c.level === "warn" ? "提醒" : "未通过"} className={levelColor[c.level]}>
              {c.level === "ok" ? "✓" : c.level === "warn" ? "!" : "✕"}
            </span>
            <span>{c.text}</span>
          </div>
        ))}
      </div>
      {error && (
        <p role="alert" className={cn(ui.notice, "text-danger")}>
          {error}
        </p>
      )}
    </Dialog>
  );
}
