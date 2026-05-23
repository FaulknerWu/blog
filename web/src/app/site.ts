import { useRouteLoaderData, type MetaDescriptor } from "react-router";
import type { SiteSettings } from "~/shared/api/types";
import type { Locale } from "~/shared/lib/format";

// SiteData 是根路由加载器的返回值，所有页面共享。
export interface SiteData {
  settings: SiteSettings;
  siteUrl: string;
}

// Handle 是路由模块可导出的 handle 字段。
export interface Handle {
  // 首页使用居中的大标题页眉。
  masthead?: boolean;
  // 后台页面：使用后台自己的布局。
  admin?: boolean;
}

export function useSite(): SiteData {
  const site = useRouteLoaderData("root") as SiteData | undefined;
  if (!site) throw new Error("root loader data is unavailable");
  return site;
}

export function useLocale(): Locale {
  const { language, timezone } = useSite().settings;
  return { language, timezone };
}

type Match = { id: string; data?: unknown } | undefined;

export function siteFrom(matches: readonly Match[]): SiteData | undefined {
  return matches.find((m) => m?.id === "root")?.data as SiteData | undefined;
}

interface SeoOptions {
  title?: string;
  description?: string;
  path: string;
  type?: "website" | "article";
  image?: string;
  publishedTime?: string;
  modifiedTime?: string;
}

// seo 组装页面标题、描述、规范链接与 Open Graph 元数据。
export function seo(site: SiteData | undefined, o: SeoOptions): MetaDescriptor[] {
  if (!site) return o.title ? [{ title: o.title }] : [];
  const { settings, siteUrl } = site;
  const title = o.title
    ? `${o.title} - ${settings.title}`
    : [settings.title, settings.subtitle].filter(Boolean).join(" - ");
  const description = o.description || settings.description;
  const url = siteUrl + o.path;

  const tags: MetaDescriptor[] = [
    { title },
    { name: "description", content: description },
    { tagName: "link", rel: "canonical", href: url },
    { property: "og:site_name", content: settings.title },
    { property: "og:title", content: o.title ?? settings.title },
    { property: "og:description", content: description },
    { property: "og:type", content: o.type ?? "website" },
    { property: "og:url", content: url },
    { property: "og:locale", content: settings.language.replace("-", "_") },
  ];
  if (o.image) tags.push({ property: "og:image", content: siteUrl + o.image });
  if (o.publishedTime) tags.push({ property: "article:published_time", content: o.publishedTime });
  if (o.modifiedTime) tags.push({ property: "article:modified_time", content: o.modifiedTime });
  return tags;
}
