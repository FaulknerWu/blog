import { useMutation, useQuery } from "@tanstack/react-query";
import { Link, useRevalidator } from "react-router";
import { adminApi } from "~/shared/api/admin";
import type { AdminPageSummary, AdminSettings } from "~/shared/api/admin-types";
import { formatWhen } from "~/shared/lib/datetime";
import { cn } from "~/shared/lib/cn";
import { button, Status, ui } from "~/shared/ui/admin/ui";
import { isStale } from "~/features/editor/render/version";
import { q, queryClient } from "~/app/admin/queries";
import { useLocale } from "~/app/site";

export function meta() {
  return [{ title: "页面 · 后台" }];
}

const maxNavigation = 10;

export default function Pages() {
  const locale = useLocale();
  const pages = useQuery(q.pages);
  const settings = useQuery(q.settings).data;
  const revalidator = useRevalidator();

  const toggle = useMutation({
    mutationFn: ({ page, on, current }: { page: AdminPageSummary; on: boolean; current: AdminSettings }) => {
      const url = `/${page.slug}`;
      const navigation = on
        ? [...current.navigation, { label: page.title, url }]
        : current.navigation.filter((n) => n.url !== url);
      return adminApi.updateSettings(current.version, { navigation });
    },
    onSuccess: (next) => {
      queryClient.setQueryData(q.settings.queryKey, next);
      // 公开站设置来自根路由加载器，刷新后侧栏等处立即反映新导航。
      revalidator.revalidate();
    },
    onError: () => queryClient.invalidateQueries({ queryKey: q.settings.queryKey }),
  });

  const inNav = (p: AdminPageSummary) => settings?.navigation.some((n) => n.url === `/${p.slug}`) ?? false;

  return (
    <div className="flex flex-col gap-6">
      <header className={ui.pageHead}>
        <h1 className={ui.h1}>页面</h1>
        <Link to="/admin/pages/new" className={button({ variant: "primary" })}>
          新建页面
        </Link>
      </header>

      {toggle.isError && (
        <p role="alert" className={cn(ui.notice, "text-danger")}>
          导航没有更新：{toggle.error.message}
        </p>
      )}

      <div className={ui.tableWrap}>
        <table className={cn(ui.table, "min-w-150")}>
          <thead>
            <tr>
              <th scope="col">标题</th>
              <th scope="col">状态</th>
              <th scope="col">显示在导航</th>
              <th scope="col">最后编辑</th>
              <th scope="col">操作</th>
            </tr>
          </thead>
          <tbody>
            {pages.data?.map((p) => {
              const on = inNav(p);
              const full = !on && (settings?.navigation.length ?? 0) >= maxNavigation;
              return (
                <tr key={p.id}>
                  <td>
                    <div className="flex max-w-110 flex-col gap-1">
                      <Link to={`/admin/pages/${p.id}`} className={ui.titleLink}>
                        {p.title}
                      </Link>
                      <span className="flex flex-wrap gap-x-3.5 gap-y-1 text-xs text-muted">
                        <span className="font-mono">/{p.slug}</span>
                        {isStale(p.renderer_version) && <Status kind="warn">渲染器 {p.renderer_version}</Status>}
                      </span>
                    </div>
                  </td>
                  <td>
                    {p.state === "published" ? (
                      <Status kind="published">已发布</Status>
                    ) : (
                      <Status kind="draft">草稿</Status>
                    )}
                  </td>
                  <td>
                    <label className="inline-flex min-h-9.5 cursor-pointer items-center gap-2 text-sm text-ink-soft">
                      <input
                        type="checkbox"
                        className={ui.checkbox}
                        checked={on}
                        disabled={!settings || toggle.isPending || full}
                        onChange={(e) => settings && toggle.mutate({ page: p, on: e.target.checked, current: settings })}
                      />
                      <span className={on && p.state !== "published" ? "text-danger" : "text-muted"}>
                        {on ? (p.state === "published" ? "显示中" : "未发布，链接暂不可用") : full ? "导航已满" : "不显示"}
                      </span>
                    </label>
                  </td>
                  <td className="whitespace-nowrap text-muted">{formatWhen(p.updated_at, locale)}</td>
                  <td className="whitespace-nowrap">
                    <Link to={`/admin/pages/${p.id}`} className={ui.link}>
                      编辑
                    </Link>
                    {p.state === "published" && (
                      <>
                        <span className={ui.sep}>·</span>
                        <a href={`/${p.slug}`} target="_blank" rel="noreferrer" className={ui.link}>
                          查看
                        </a>
                      </>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
        {pages.isPending && <p className={ui.empty}>正在加载…</p>}
        {pages.isError && <p className={cn(ui.empty, "text-danger")}>{pages.error.message}</p>}
        {pages.data?.length === 0 && <p className={ui.empty}>还没有页面。</p>}
      </div>
    </div>
  );
}
