import { apiGet } from "./client.server";
import type {
  Items,
  Page,
  Paged,
  Post,
  PostSummary,
  SearchHit,
  SeriesDetail,
  SeriesSummary,
  SiteSettings,
  Tag,
} from "./types";

const seg = encodeURIComponent;

// 根路由与页面路由的加载器并行执行且都需要站点设置，同一请求内只取一次。
const settingsByRequest = new WeakMap<Request, Promise<SiteSettings>>();

export function getSettings(req: Request): Promise<SiteSettings> {
  let p = settingsByRequest.get(req);
  if (!p) {
    p = apiGet<SiteSettings>(req, "/api/settings");
    settingsByRequest.set(req, p);
  }
  return p;
}

export const listPosts = (req: Request, q: { tag?: string; limit: number; cursor?: string | null }) =>
  apiGet<Paged<PostSummary>>(req, "/api/posts", { query: q });

export const getPost = (req: Request, slug: string) =>
  apiGet<Post>(req, `/api/posts/${seg(slug)}`, { onMoved: (s) => `/posts/${seg(s)}` });

export const getPage = (req: Request, slug: string) =>
  apiGet<Page>(req, `/api/pages/${seg(slug)}`, { onMoved: (s) => `/${seg(s)}` });

export const listTags = (req: Request) => apiGet<Items<Tag>>(req, "/api/tags");

export const getTag = (req: Request, slug: string) => apiGet<Tag>(req, `/api/tags/${seg(slug)}`);

export const listSeries = (req: Request) => apiGet<Items<SeriesSummary>>(req, "/api/series");

export const getSeries = (req: Request, slug: string) => apiGet<SeriesDetail>(req, `/api/series/${seg(slug)}`);

export const search = (req: Request, q: string) => apiGet<Items<SearchHit>>(req, "/api/search", { query: { q } });
