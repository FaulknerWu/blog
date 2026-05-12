import { data, redirect } from "react-router";

const apiOrigin = (process.env.API_ORIGIN ?? "http://127.0.0.1:8080").replace(/\/+$/, "");
const timeoutMs = 10_000;

export type Query = Record<string, string | number | null | undefined>;

export interface GetOptions {
  query?: Query;
  // 命中历史别名（API 返回 301）时，把新 slug 映射为站内路径并重定向读者。
  onMoved?: (slug: string) => string;
}

// apiGet 在 SSR 加载器中请求 Go API。400/404 抛出同状态码的路由响应，其他失败抛出异常交给错误边界。
export async function apiGet<T>(request: Request, path: string, opts: GetOptions = {}): Promise<T> {
  const url = new URL(apiOrigin + path);
  for (const [k, v] of Object.entries(opts.query ?? {})) {
    if (v != null && v !== "") url.searchParams.set(k, String(v));
  }

  const headers = new Headers({ Accept: "application/json" });
  const realIP = request.headers.get("x-real-ip");
  if (realIP) headers.set("X-Real-IP", realIP);

  const res = await fetch(url, {
    headers,
    redirect: "manual",
    signal: AbortSignal.any([request.signal, AbortSignal.timeout(timeoutMs)]),
  });

  if (res.ok) return (await res.json()) as T;

  if (res.status === 301 && opts.onMoved) {
    const location = res.headers.get("location") ?? "";
    const slug = decodeURIComponent(location.slice(location.lastIndexOf("/") + 1));
    if (slug) throw redirect(opts.onMoved(slug), 301);
  }
  // 400 多由读者手改的查询参数（如游标）引起，与 404 一样交给错误页而不是记为服务端错误。
  if (res.status === 400 || res.status === 404) throw data(null, { status: res.status });

  const problem = await res.json().catch(() => null);
  throw new Error(`API ${res.status} ${path}: ${problem?.title ?? res.statusText}`);
}
