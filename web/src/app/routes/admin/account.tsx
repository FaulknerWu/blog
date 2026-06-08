import { useMutation, useQuery } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { adminApi, ApiError } from "~/shared/api/admin";
import type { Session } from "~/shared/api/admin-types";
import { formatWhen } from "~/shared/lib/datetime";
import { formatMonthDay } from "~/shared/lib/format";
import { describeUserAgent } from "~/shared/lib/useragent";
import { cn } from "~/shared/lib/cn";
import { button, ui } from "~/shared/ui/admin/ui";
import { q, queryClient } from "~/app/admin/queries";
import { useLocale } from "~/app/site";

export function meta() {
  return [{ title: "账户与安全 · 后台" }];
}

const minPassword = 12;

// strength 粗略估计密码强度（0–4），只用于提示，不替代服务端长度校验。
function strength(pw: string): number {
  if (pw.length < minPassword) return pw.length ? 1 : 0;
  const kinds = [/[a-z]/, /[A-Z]/, /\d/, /[^A-Za-z0-9]/].filter((r) => r.test(pw)).length;
  return Math.min(4, 1 + (pw.length >= 16 ? 1 : 0) + (kinds >= 3 ? 1 : 0) + (pw.length >= 20 || kinds === 4 ? 1 : 0));
}

const strengthLabel = ["", "太短", "一般", "较强", "很强"];

export default function Account() {
  return (
    <div className="flex max-w-250 flex-col gap-12">
      <h1 className={ui.h1}>账户与安全</h1>
      <PasswordForm />
      <Sessions />
    </div>
  );
}

function PasswordForm() {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [done, setDone] = useState(false);

  const change = useMutation({
    mutationFn: () => adminApi.changePassword(current, next),
    onSuccess: () => {
      setCurrent("");
      setNext("");
      setConfirm("");
      setDone(true);
      queryClient.invalidateQueries({ queryKey: q.sessions.queryKey });
    },
  });

  const mismatch = confirm.length > 0 && confirm !== next;
  const tooShort = next.length > 0 && next.length < minPassword;
  const s = strength(next);
  const serverError =
    change.error instanceof ApiError
      ? (change.error.fieldError("current_password") ?? change.error.fieldError("new_password") ?? change.error.message)
      : change.error?.message;

  function submit(e: FormEvent) {
    e.preventDefault();
    setDone(false);
    if (!current || next.length < minPassword || next !== confirm) return;
    change.mutate();
  }

  return (
    <section className="flex flex-col gap-4.5">
      <div className={ui.secHead}>
        <h2 className={ui.h2}>修改密码</h2>
      </div>
      <form className="flex max-w-105 flex-col gap-4" onSubmit={submit}>
        <label className={ui.field}>
          <span className={ui.label}>当前密码</span>
          <input
            className={ui.input}
            type="password"
            autoComplete="current-password"
            required
            value={current}
            onChange={(e) => setCurrent(e.target.value)}
          />
        </label>
        <label className={ui.field}>
          <span className={ui.label}>新密码</span>
          <input
            className={ui.input}
            type="password"
            autoComplete="new-password"
            required
            minLength={minPassword}
            aria-invalid={tooShort || undefined}
            value={next}
            onChange={(e) => setNext(e.target.value)}
          />
          {next && (
            <span className="flex items-center gap-3">
              <span aria-hidden="true" className="grid flex-1 grid-cols-4 gap-0.75">
                {[1, 2, 3, 4].map((i) => (
                  <span key={i} className={cn("h-0.5 bg-line", i <= s && (s <= 1 ? "bg-danger" : "bg-ok"))} />
                ))}
              </span>
              <span className={ui.hint}>
                {next.length} 个字符 · {strengthLabel[s]}
              </span>
            </span>
          )}
        </label>
        <label className={ui.field}>
          <span className={ui.label}>确认新密码</span>
          <input
            className={ui.input}
            type="password"
            autoComplete="new-password"
            required
            aria-invalid={mismatch || undefined}
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
          />
          {mismatch && <span className={ui.error}>两次输入的密码不一致。</span>}
        </label>
        <span className={ui.hint}>至少 {minPassword} 个字符。修改后，除当前浏览器外的所有会话都会被注销。</span>
        {change.isError && (
          <p role="alert" className={cn(ui.notice, "text-danger")}>
            {serverError}
          </p>
        )}
        {done && (
          <p role="status" className="text-sm text-ok">
            密码已更新。
          </p>
        )}
        <div>
          <button
            type="submit"
            className={button({ variant: "primary" })}
            disabled={change.isPending || !current || next.length < minPassword || next !== confirm}
          >
            更新密码
          </button>
        </div>
      </form>
    </section>
  );
}

function Sessions() {
  const locale = useLocale();
  const sessions = useQuery(q.sessions);
  const revoke = useMutation({
    mutationFn: async (ids: number[]) => {
      for (const id of ids) await adminApi.revokeSession(id);
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: q.sessions.queryKey }),
  });

  const list = (sessions.data ?? []).toSorted((a, b) => Number(b.current) - Number(a.current) || b.last_seen_at.localeCompare(a.last_seen_at));
  const others = list.filter((s) => !s.current);

  return (
    <section className="flex flex-col gap-2">
      <div className={ui.secHead}>
        <h2 className={ui.h2}>登录会话</h2>
        {others.length > 0 && (
          <button
            type="button"
            className={ui.linkBtn}
            disabled={revoke.isPending}
            onClick={() => revoke.mutate(others.map((s) => s.id))}
          >
            注销其他所有会话
          </button>
        )}
      </div>
      {revoke.isError && (
        <p role="alert" className={cn(ui.notice, "text-danger")}>
          {revoke.error.message}
        </p>
      )}
      <ul>
        {list.map((s) => (
          <SessionRow key={s.id} session={s} locale={locale} busy={revoke.isPending} onRevoke={() => revoke.mutate([s.id])} />
        ))}
      </ul>
      {sessions.isPending && <p className={ui.empty}>正在加载…</p>}
    </section>
  );
}

function SessionRow({
  session: s,
  locale,
  busy,
  onRevoke,
}: {
  session: Session;
  locale: ReturnType<typeof useLocale>;
  busy: boolean;
  onRevoke: () => void;
}) {
  return (
    <li className="flex flex-wrap items-baseline gap-x-6 gap-y-1.5 border-b border-line-soft py-4">
      <span className="flex flex-[1_1_280px] flex-col gap-1">
        <span className="flex flex-wrap items-baseline gap-3">
          <span className="font-serif text-lg leading-normal font-semibold">{describeUserAgent(s.user_agent)}</span>
          {s.current && <span className="text-xs text-accent">当前浏览器</span>}
        </span>
        <span className="text-xs text-muted">
          <span className="font-mono">{s.ip}</span> · 登录于 {formatMonthDay(s.created_at, locale)} · 最近活动{" "}
          {formatWhen(s.last_seen_at, locale)}
        </span>
      </span>
      <span className="text-xs text-muted">{formatMonthDay(s.expires_at, locale)}失效</span>
      {!s.current && (
        <button type="button" className={cn(ui.linkBtn, "text-danger hover:text-danger")} disabled={busy} onClick={onRevoke}>
          注销
        </button>
      )}
    </li>
  );
}
