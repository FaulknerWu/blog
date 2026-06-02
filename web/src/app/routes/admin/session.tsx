import { QueryClientProvider } from "@tanstack/react-query";
import { isRouteErrorResponse, Link, Outlet, redirect } from "react-router";
import { ApiError } from "~/shared/api/admin";
import { cn } from "~/shared/lib/cn";
import { ui } from "~/shared/ui/admin/ui";
import { q, queryClient } from "~/app/admin/queries";
import type { Handle } from "~/app/site";
import type { Route } from "./+types/session";

export const handle: Handle = { admin: true };

// 进入后台前确认会话有效，未登录时带上原地址跳转到登录页。
export async function clientLoader({ request }: Route.ClientLoaderArgs) {
  try {
    return await queryClient.ensureQueryData(q.me);
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      const url = new URL(request.url);
      throw redirect(`/admin/login?next=${encodeURIComponent(url.pathname + url.search)}`);
    }
    throw err;
  }
}

export function HydrateFallback() {
  return <div className="min-h-screen bg-paper" aria-busy="true" />;
}

export default function Session() {
  return (
    <QueryClientProvider client={queryClient}>
      <Outlet />
    </QueryClientProvider>
  );
}

export function ErrorBoundary({ error }: Route.ErrorBoundaryProps) {
  let title = "后台出了点问题";
  let detail = error instanceof Error ? error.message : "请稍后重试。";
  if (isRouteErrorResponse(error) && error.status === 404) {
    title = "没有这个页面";
    detail = "地址可能有误，或者内容已被删除。";
  } else if (error instanceof ApiError && error.status === 404) {
    title = "内容不存在";
    detail = "它可能已在别处被删除。";
  } else if (error instanceof ApiError && error.status === 0) {
    detail = "无法连接到服务器，请检查网络后刷新页面。";
  }
  return (
    <div className="mx-auto flex max-w-140 flex-col items-start gap-3 px-4 py-24 font-sans">
      <h1 className={ui.h1}>{title}</h1>
      <p className={cn(ui.notice, "text-muted")}>{detail}</p>
      <Link to="/admin" className={ui.link}>
        ← 返回概览
      </Link>
    </div>
  );
}
