import { useMutation, useQuery } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { adminApi, ApiError } from "~/shared/api/admin";
import type { Scope, Session, Token, TokenCreated } from "~/shared/api/types";
import { formatWhen, fromZonedInput } from "~/shared/lib/datetime";
import { formatMonthDay } from "~/shared/lib/format";
import { describeUserAgent } from "~/shared/lib/useragent";
import { cn } from "~/shared/lib/cn";
import { button, ConfirmDialog, ui } from "~/shared/ui/admin/ui";
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
      <Tokens />
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
          {s.ip && (
            <>
              <span className="font-mono">{s.ip}</span> ·{" "}
            </>
          )}
          登录于 {formatMonthDay(s.created_at, locale)} · 最近活动{" "}
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

const scopes: { scope: Scope; label: string }[] = [
  { scope: "content:read", label: "读取内容" },
  { scope: "content:write", label: "撰写草稿、管理标签与专栏" },
  { scope: "content:publish", label: "发布、撤回与删除" },
  { scope: "media:write", label: "上传与管理图片" },
  { scope: "settings:write", label: "修改站点设置" },
];

// Tokens 管理供 blog CLI 与 agent 使用的 API 令牌；令牌明文只在创建后显示一次。
function Tokens() {
  const locale = useLocale();
  const tokens = useQuery(q.tokens);
  const [creating, setCreating] = useState(false);
  const [created, setCreated] = useState<TokenCreated | null>(null);
  const [revoking, setRevoking] = useState<Token | null>(null);

  const revoke = useMutation({
    mutationFn: (id: number) => adminApi.revokeToken(id),
    onSuccess: () => setRevoking(null),
    onSettled: () => queryClient.invalidateQueries({ queryKey: q.tokens.queryKey }),
  });

  return (
    <section className="flex flex-col gap-2">
      <div className={ui.secHead}>
        <h2 className={ui.h2}>API 令牌</h2>
        {!creating && (
          <button
            type="button"
            className={ui.linkBtn}
            onClick={() => {
              setCreated(null);
              setCreating(true);
            }}
          >
            新建令牌
          </button>
        )}
      </div>
      {creating && (
        <TokenForm
          onCancel={() => setCreating(false)}
          onCreated={(t) => {
            setCreating(false);
            setCreated(t);
          }}
        />
      )}
      {created && <NewSecret created={created} onDismiss={() => setCreated(null)} />}
      <ul>
        {(tokens.data ?? []).map((t) => (
          <li key={t.id} className="flex flex-wrap items-baseline gap-x-6 gap-y-1.5 border-b border-line-soft py-4">
            <span className="flex flex-[1_1_280px] flex-col gap-1">
              <span className="font-serif text-lg leading-normal font-semibold">{t.name}</span>
              <span className="text-xs text-muted">
                <span className="font-mono">{t.scopes.join(" ")}</span> · 创建于 {formatMonthDay(t.created_at, locale)} ·{" "}
                {t.last_used_at ? `最近使用 ${formatWhen(t.last_used_at, locale)}` : "从未使用"}
              </span>
            </span>
            <span className="text-xs text-muted">{t.expires_at ? `${formatMonthDay(t.expires_at, locale)}失效` : "长期有效"}</span>
            <button type="button" className={cn(ui.linkBtn, "text-danger hover:text-danger")} onClick={() => setRevoking(t)}>
              吊销
            </button>
          </li>
        ))}
      </ul>
      {tokens.isPending && <p className={ui.empty}>正在加载…</p>}
      {tokens.data?.length === 0 && !creating && !created && <p className={ui.empty}>还没有令牌。</p>}
      <ConfirmDialog
        open={revoking !== null}
        title={`吊销令牌「${revoking?.name ?? ""}」？`}
        lead="使用它的 CLI 与 agent 将立即无法访问。"
        confirmLabel="吊销"
        danger
        busy={revoke.isPending}
        error={revoke.error?.message}
        onConfirm={() => revoking && revoke.mutate(revoking.id)}
        onClose={() => {
          setRevoking(null);
          revoke.reset();
        }}
      />
    </section>
  );
}

function TokenForm({ onCancel, onCreated }: { onCancel: () => void; onCreated: (t: TokenCreated) => void }) {
  const locale = useLocale();
  const [name, setName] = useState("");
  const [chosen, setChosen] = useState<Scope[]>(["content:read", "content:write"]);
  const [expires, setExpires] = useState("");

  const create = useMutation({
    mutationFn: () =>
      adminApi.createToken({
        name: name.trim(),
        scopes: chosen,
        // 过期日期按站点时区取当天结束。
        expires_at: expires ? fromZonedInput(expires, "23:59", locale.timezone).toISOString() : null,
      }),
    onSuccess: (t) => {
      queryClient.invalidateQueries({ queryKey: q.tokens.queryKey });
      onCreated(t);
    },
  });
  const nameError = create.error instanceof ApiError ? create.error.fieldError("name") : undefined;

  function toggle(scope: Scope, on: boolean) {
    setChosen((cur) => (on ? [...cur, scope] : cur.filter((s) => s !== scope)));
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    if (!name.trim() || chosen.length === 0) return;
    create.mutate();
  }

  return (
    <form className="flex max-w-105 flex-col gap-4 border-b border-line-soft py-4.5" onSubmit={submit}>
      <label className={ui.field}>
        <span className={ui.label}>名称</span>
        <input
          className={ui.input}
          required
          maxLength={100}
          placeholder="claude-code"
          aria-invalid={!!nameError || undefined}
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        {nameError && <span className={ui.error}>{nameError}</span>}
      </label>
      <fieldset className={ui.field}>
        <legend className={cn(ui.label, "mb-1.5")}>权限</legend>
        {scopes.map(({ scope, label }) => (
          <label key={scope} className="flex cursor-pointer items-baseline gap-2.5 py-1 text-md">
            <input
              type="checkbox"
              className={cn(ui.checkbox, "translate-y-0.5")}
              checked={chosen.includes(scope)}
              onChange={(e) => toggle(scope, e.target.checked)}
            />
            <span>{label}</span>
            <span className={cn(ui.mono, "text-muted")}>{scope}</span>
          </label>
        ))}
      </fieldset>
      <label className={ui.field}>
        <span className={ui.label}>过期日期（可选）</span>
        <input className={cn(ui.input, "w-50")} type="date" value={expires} onChange={(e) => setExpires(e.target.value)} />
      </label>
      {create.isError && !nameError && (
        <p role="alert" className={cn(ui.notice, "text-danger")}>
          {create.error.message}
        </p>
      )}
      <div className="flex gap-2">
        <button type="submit" className={button({ variant: "primary" })} disabled={create.isPending || !name.trim() || chosen.length === 0}>
          创建
        </button>
        <button type="button" className={button()} onClick={onCancel}>
          取消
        </button>
      </div>
    </form>
  );
}

function NewSecret({ created, onDismiss }: { created: TokenCreated; onDismiss: () => void }) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    await navigator.clipboard.writeText(created.secret);
    setCopied(true);
  }

  return (
    <div role="status" className="flex flex-col gap-2 border-b border-line-soft py-4.5">
      <span className={ui.label}>令牌「{created.token.name}」· 只显示这一次</span>
      <span className="flex max-w-160 gap-2">
        <input
          className={cn(ui.input, "font-mono text-sm")}
          readOnly
          value={created.secret}
          aria-label="令牌"
          onFocus={(e) => e.currentTarget.select()}
        />
        <button type="button" className={button()} onClick={() => void copy()}>
          {copied ? "已复制" : "复制"}
        </button>
      </span>
      <span>
        <button type="button" className={ui.linkBtn} onClick={onDismiss}>
          完成
        </button>
      </span>
    </div>
  );
}
