import { useEffect, useState, type FormEvent } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { adminApi, ApiError } from "~/shared/api/admin";
import { cn } from "~/shared/lib/cn";
import { button, ui } from "~/shared/ui/admin/ui";
import { q, queryClient } from "~/app/admin/queries";
import { useSite, type Handle } from "~/app/site";

export const handle: Handle = { admin: true };

export function meta() {
  return [{ title: "登录 · 后台" }];
}

// 登录后只允许跳回后台内部地址，避免被构造成开放重定向。
function safeNext(next: string | null): string {
  return next && /^\/admin(\/|$|\?)/.test(next) ? next : "/admin";
}

export default function Login() {
  const { settings } = useSite();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [waitUntil, setWaitUntil] = useState<number | null>(null);
  const [now, setNow] = useState(() => Date.now());

  const wait = waitUntil ? Math.max(0, Math.ceil((waitUntil - now) / 1000)) : 0;
  useEffect(() => {
    if (!waitUntil) return;
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [waitUntil]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (busy || wait > 0) return;
    setBusy(true);
    setError(null);
    try {
      const { user } = await adminApi.login(username.trim(), password);
      queryClient.setQueryData(q.me.queryKey, user);
      navigate(safeNext(params.get("next")), { replace: true });
    } catch (err) {
      setPassword("");
      if (err instanceof ApiError && err.status === 429) {
        setNow(Date.now());
        setWaitUntil(Date.now() + (err.retryAfter ?? 60) * 1000);
      } else if (err instanceof ApiError && (err.status === 401 || err.status === 422)) {
        setError("用户名或密码不正确。");
      } else {
        setError(err instanceof Error ? err.message : "登录失败，请稍后再试。");
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex min-h-screen flex-col items-center bg-paper px-4 pt-24 pb-12 font-sans text-md text-ink">
      <header className="flex w-full max-w-90 flex-col items-center gap-2.5 border-b border-line pb-9 text-center font-serif">
        <span className="text-6xl leading-tight font-bold tracking-wide">{settings.title}</span>
        <h1 className="text-lg leading-normal text-muted italic">后台登录</h1>
      </header>

      <form className="flex w-full max-w-90 flex-col gap-4.5 pt-8" onSubmit={submit}>
        {wait > 0 ? (
          <p role="alert" className={cn(ui.notice, "text-warn")}>
            尝试次数过多，请在 {wait} 秒后再试。
          </p>
        ) : (
          error && (
            <p role="alert" className={cn(ui.notice, "text-danger")}>
              {error}
            </p>
          )
        )}
        <label className={ui.field}>
          <span className={ui.label}>用户名</span>
          <input
            className={cn(ui.input, "min-h-10.5 px-3 text-base")}
            name="username"
            autoComplete="username"
            autoCapitalize="none"
            spellCheck={false}
            required
            value={username}
            onChange={(e) => setUsername(e.target.value)}
          />
        </label>
        <label className={ui.field}>
          <span className={ui.label}>密码</span>
          <input
            className={cn(ui.input, "min-h-10.5 px-3 text-base")}
            type="password"
            name="password"
            autoComplete="current-password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </label>
        <button type="submit" className={cn(button({ variant: "primary" }), "mt-1 min-h-10.5 text-base")} disabled={busy || wait > 0}>
          {busy ? "正在登录…" : "登录"}
        </button>
      </form>

      <a href="/" className="mt-10 py-2.5 text-sm text-accent">
        ← 返回站点
      </a>
    </div>
  );
}
