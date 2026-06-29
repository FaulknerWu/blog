import { index, layout, prefix, route, type RouteConfig } from "@react-router/dev/routes";

// 公开站路径与 API 生成的订阅源、Sitemap 链接一致（见 api/internal/feed）。
export default [
  index("routes/home.tsx"),
  route("posts/:slug", "routes/post.tsx"),
  route("tags", "routes/tags.tsx"),
  route("tags/:slug", "routes/tag.tsx"),
  route("series", "routes/series-index.tsx"),
  route("series/:slug", "routes/series.tsx"),
  route("search", "routes/search.tsx"),

  // 管理后台在浏览器中渲染，服务端只输出加载占位。
  ...prefix("admin", [
    route("login", "routes/admin/login.tsx"),
    layout("routes/admin/session.tsx", [
      layout("routes/admin/shell.tsx", [
        index("routes/admin/dashboard.tsx"),
        route("posts", "routes/admin/posts.tsx"),
        route("pages", "routes/admin/pages.tsx"),
        route("media", "routes/admin/media.tsx"),
        route("taxonomy", "routes/admin/taxonomy.tsx"),
        route("settings", "routes/admin/settings.tsx"),
        route("account", "routes/admin/account.tsx"),
      ]),
      // :id 为 new 时新建，首次保存后原地切换到新地址，编辑器状态不重置。
      route("posts/:id", "routes/admin/post-editor.tsx"),
      route("pages/:id", "routes/admin/page-editor.tsx"),
    ]),
  ]),

  // 独立页面挂在根路径下，保留路径由 API 在保存页面时拒绝。
  route(":slug", "routes/page.tsx"),
  route("*", "routes/not-found.tsx"),
] satisfies RouteConfig;
