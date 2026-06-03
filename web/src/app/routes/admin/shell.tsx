import { useQuery } from "@tanstack/react-query";
import { NavLink, Outlet, useNavigate } from "react-router";
import { adminApi } from "~/shared/api/admin";
import { SkipLink } from "~/shared/ui/SkipLink";
import { q, queryClient } from "~/app/admin/queries";

export default function Shell() {
  const navigate = useNavigate();
  const me = useQuery(q.me).data;
  const posts = useQuery(q.allPosts).data;
  const pages = useQuery(q.pages).data;
  const media = useQuery(q.allMedia).data;

  const items: { to: string; label: string; count?: number; end?: boolean }[] = [
    { to: "/admin", label: "概览", end: true },
    { to: "/admin/posts", label: "文章", count: posts?.length },
    { to: "/admin/pages", label: "页面", count: pages?.length },
    { to: "/admin/media", label: "媒体库", count: media?.length },
    { to: "/admin/taxonomy", label: "标签与专栏" },
    { to: "/admin/settings", label: "站点设置" },
    { to: "/admin/account", label: "账户与安全" },
  ];

  async function logout() {
    await adminApi.logout().catch(() => undefined);
    // 先离开后台页面再清缓存，避免仍挂载的查询用失效的会话重新请求。
    await navigate("/admin/login", { replace: true });
    queryClient.clear();
  }

  return (
    // 宽屏时左侧细线导航 + 主区域；窄屏（< lg）导航折到顶部横排。
    <div className="flex min-h-screen flex-wrap content-start bg-paper font-sans text-md text-ink lg:content-stretch">
      <SkipLink href="#admin-main">跳到主要内容</SkipLink>
      <aside className="flex flex-[1_1_200px] flex-col gap-3 border-b border-line px-4 pt-6 pb-2 lg:gap-7 lg:border-r lg:border-b-0 lg:px-6 lg:pt-10 lg:pb-6">
        <nav aria-label="后台导航" className="flex flex-row flex-wrap gap-x-5 lg:flex-col lg:gap-0">
          {items.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className="flex min-h-9 items-center gap-1.5 text-md text-ink-soft hover:text-accent aria-[current=page]:font-semibold aria-[current=page]:text-accent lg:min-h-10 lg:gap-0 lg:border-l lg:border-line lg:pl-3.5 lg:aria-[current=page]:border-l-2 lg:aria-[current=page]:border-accent lg:aria-[current=page]:pl-3.25"
            >
              {item.label}
              {item.count !== undefined && (
                <span className="text-xs font-normal text-muted lg:ml-auto">{item.count}</span>
              )}
            </NavLink>
          ))}
        </nav>
        <div className="flex flex-row gap-5 text-sm lg:mt-auto lg:flex-col lg:items-start lg:gap-0">
          <a href="/" target="_blank" rel="noreferrer" className="py-2 text-accent">
            查看站点 ↗
          </a>
          <button type="button" onClick={logout} className="py-2 text-left text-muted hover:text-ink">
            {me?.display_name} · 退出登录
          </button>
        </div>
      </aside>
      <main id="admin-main" className="min-w-0 flex-[999_1_640px] px-4 pt-10 pb-16 md:px-8 xl:px-14">
        <Outlet />
      </main>
    </div>
  );
}
