import type { ReactNode } from "react";
import {
  isRouteErrorResponse,
  Link,
  Links,
  Meta,
  Outlet,
  Scripts,
  ScrollRestoration,
  useMatches,
  useRouteLoaderData,
} from "react-router";
import "@fontsource-variable/source-serif-4";
import "@fontsource-variable/source-serif-4/wght-italic.css";
import "@fontsource-variable/jetbrains-mono";
import "~/shared/styles/app.css";
import { getSettings } from "~/shared/api/public.server";
import type { Route } from "./+types/root";
import { SkipLink } from "~/shared/ui/SkipLink";
import { Page } from "./layout/Page";
import { SiteFooter } from "./layout/SiteFooter";
import { SiteHeader } from "./layout/SiteHeader";
import type { Handle, SiteData } from "./site";

export async function loader({ request }: Route.LoaderArgs): Promise<SiteData> {
  const settings = await getSettings(request);
  const siteUrl = (process.env.SITE_URL ?? new URL(request.url).origin).replace(/\/+$/, "");
  return { settings, siteUrl };
}

export function Layout({ children }: { children: ReactNode }) {
  const site = useRouteLoaderData("root") as SiteData | undefined;
  const matches = useMatches();
  const masthead = matches.some((m) => (m.handle as Handle | undefined)?.masthead);
  // 后台页面自带布局，不套用公开站的页眉页脚，也不允许被收录。
  const admin = matches.some((m) => (m.handle as Handle | undefined)?.admin);
  const settings = site?.settings;

  return (
    <html lang={settings?.language ?? "zh-CN"}>
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        {(admin || settings?.noindex) && <meta name="robots" content="noindex" />}
        <Meta />
        <Links />
        {settings && !admin && (
          <>
            <link rel="alternate" type="application/atom+xml" href="/feed.xml" title={settings.title} />
            <link rel="alternate" type="application/rss+xml" href="/rss.xml" title={settings.title} />
          </>
        )}
      </head>
      <body>
        {admin ? (
          children
        ) : (
          <>
            <SkipLink href="#main">跳到正文</SkipLink>
            {settings && <SiteHeader settings={settings} masthead={masthead} />}
            <main id="main">{children}</main>
            {settings && <SiteFooter settings={settings} narrow={masthead} />}
          </>
        )}
        <ScrollRestoration />
        <Scripts />
      </body>
    </html>
  );
}

export default function App() {
  return <Outlet />;
}

export function ErrorBoundary({ error }: Route.ErrorBoundaryProps) {
  let status = 500;
  let title = "出了点问题";
  let detail = "页面暂时无法显示，请稍后再试。";

  if (isRouteErrorResponse(error)) {
    status = error.status;
    if (status === 404) {
      title = "页面不存在";
      detail = "这个地址没有内容，可能已被移动或删除。";
    } else if (status === 400) {
      title = "链接有误";
      detail = "地址中的参数无法识别。";
    }
  } else if (import.meta.env.DEV && error instanceof Error) {
    detail = error.message;
  }

  return (
    <>
      <title>{title}</title>
      <Page eyebrow={String(status)} title={title} description={detail}>
        <Link to="/">← 返回首页</Link>
      </Page>
    </>
  );
}
