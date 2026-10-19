// 后台在浏览器中直接调用同源 API；会话 Cookie 由浏览器自动携带，写请求的 Origin 与
// Sec-Fetch-Site 头也由浏览器填写，满足 API 的跨站请求校验。
import type {
  AdminPage,
  AdminPageSummary,
  AdminPageSummaryList,
  AdminPost,
  AdminPostSummary,
  AdminPostSummaryList,
  AdminSeries,
  AdminSeriesDetail,
  AdminSeriesList,
  AdminSettings,
  AdminTag,
  AdminTagList,
  LoginResult,
  Media,
  MediaDetail,
  MediaList,
  Me,
  PageInput,
  PostInput,
  PostState,
  Problem,
  RendererInfo,
  RenderPreview,
  SeriesInput,
  SeriesPatch,
  SessionList,
  SettingsInput,
  TagPatch,
  TokenCreated,
  TokenInput,
  TokenList,
} from "./types";

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly problem: Problem | null,
    readonly retryAfter: number | null,
  ) {
    super(problem?.detail ?? problem?.title ?? (status === 0 ? "无法连接到服务器" : `请求失败（${status}）`));
  }

  // fieldError 返回指定字段（如 "slug"）的第一条校验错误。
  fieldError(field: string): string | undefined {
    return this.problem?.errors?.find((e) => e.pointer === `#/${field}` || e.parameter === field)?.detail;
  }
}

type Query = Record<string, string | number | boolean | string[] | null | undefined>;

interface Options {
  query?: Query;
  body?: unknown;
  // 资源版本号，作为 If-Match: "v{version}" 发送。
  version?: number;
  signal?: AbortSignal;
}

function url(path: string, query?: Query): string {
  const params = new URLSearchParams();
  for (const [k, v] of Object.entries(query ?? {})) {
    if (v === undefined || v === null || v === "" || v === false) continue;
    for (const item of Array.isArray(v) ? v : [v]) params.append(k, String(item));
  }
  const qs = params.toString();
  return qs ? `${path}?${qs}` : path;
}

export async function request<T>(method: string, path: string, opts: Options = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json" };
  let body: BodyInit | undefined;
  if (opts.body instanceof FormData) {
    body = opts.body;
  } else if (opts.body !== undefined) {
    headers["Content-Type"] = "application/json";
    body = JSON.stringify(opts.body);
  }
  if (opts.version !== undefined) headers["If-Match"] = `"v${opts.version}"`;

  const res = await fetch(url(path, opts.query), {
    method,
    headers,
    body,
    credentials: "same-origin",
    signal: opts.signal,
  }).catch((err: unknown) => {
    if (opts.signal?.aborted) throw err;
    throw new ApiError(0, null, null);
  });
  if (!res.ok) throw await toError(res);
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

async function toError(res: Response): Promise<ApiError> {
  let problem: Problem | null = null;
  if (res.headers.get("Content-Type")?.includes("json")) {
    problem = (await res.json().catch(() => null)) as Problem | null;
  }
  const retry = Number(res.headers.get("Retry-After"));
  return new ApiError(res.status, problem, Number.isFinite(retry) && retry > 0 ? retry : null);
}

// upload 用 XMLHttpRequest 上传以便报告进度，fetch 无法观察请求体发送进度。
export function upload(file: File, onProgress?: (ratio: number) => void): Promise<Media> {
  return new Promise((resolve, reject) => {
    const form = new FormData();
    form.append("file", file);
    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/api/admin/media");
    xhr.setRequestHeader("Accept", "application/json");
    xhr.responseType = "json";
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress?.(e.loaded / e.total);
    };
    xhr.onload = () => {
      if (xhr.status === 200 || xhr.status === 201) resolve(xhr.response as Media);
      else reject(new ApiError(xhr.status, xhr.response as Problem | null, null));
    };
    xhr.onerror = () => reject(new ApiError(0, null, null));
    xhr.send(form);
  });
}

export interface PostFilter {
  state?: PostState;
  q?: string;
  tag_id?: number;
  stale?: boolean;
}

export const adminApi = {
  login: (username: string, password: string) =>
    request<LoginResult>("POST", "/api/auth/sessions", { body: { username, password } }),
  logout: () => request<void>("DELETE", "/api/auth/sessions/current"),
  me: () => request<Me>("GET", "/api/auth/me"),
  sessions: () => request<SessionList>("GET", "/api/auth/sessions"),
  revokeSession: (id: number) => request<void>("DELETE", `/api/auth/sessions/${id}`),
  changePassword: (current_password: string, new_password: string) =>
    request<void>("PUT", "/api/auth/password", { body: { current_password, new_password } }),
  tokens: () => request<TokenList>("GET", "/api/auth/tokens"),
  createToken: (input: TokenInput) => request<TokenCreated>("POST", "/api/auth/tokens", { body: input }),
  revokeToken: (id: number) => request<void>("DELETE", `/api/auth/tokens/${id}`),

  renderer: () => request<RendererInfo>("GET", "/api/admin/render"),
  render: (body_md: string, signal?: AbortSignal) =>
    request<RenderPreview>("POST", "/api/admin/render", { body: { body_md }, signal }),

  posts: (filter: PostFilter, cursor?: string, limit = 50) =>
    request<AdminPostSummaryList>("GET", "/api/admin/posts", { query: { ...filter, cursor, limit } }),
  post: (id: number) => request<AdminPost>("GET", `/api/admin/posts/${id}`),
  createPost: (input: PostInput) => request<AdminPostSummary>("POST", "/api/admin/posts", { body: input }),
  updatePost: (id: number, version: number, input: PostInput) =>
    request<AdminPostSummary>("PUT", `/api/admin/posts/${id}`, { body: input, version }),
  deletePost: (id: number, version: number) => request<void>("DELETE", `/api/admin/posts/${id}`, { version }),
  publishPost: (id: number, version: number, publishedAt: string | null) =>
    request<AdminPostSummary>("PUT", `/api/admin/posts/${id}/publication`, {
      body: { published_at: publishedAt },
      version,
    }),
  unpublishPost: (id: number, version: number) =>
    request<AdminPostSummary>("DELETE", `/api/admin/posts/${id}/publication`, { version }),

  pages: (stale?: boolean) => request<AdminPageSummaryList>("GET", "/api/admin/pages", { query: { stale } }),
  page: (id: number) => request<AdminPage>("GET", `/api/admin/pages/${id}`),
  createPage: (input: PageInput) => request<AdminPageSummary>("POST", "/api/admin/pages", { body: input }),
  updatePage: (id: number, version: number, input: PageInput) =>
    request<AdminPageSummary>("PUT", `/api/admin/pages/${id}`, { body: input, version }),
  deletePage: (id: number, version: number) => request<void>("DELETE", `/api/admin/pages/${id}`, { version }),
  publishPage: (id: number, version: number) =>
    request<AdminPageSummary>("PUT", `/api/admin/pages/${id}/publication`, { body: {}, version }),
  unpublishPage: (id: number, version: number) =>
    request<AdminPageSummary>("DELETE", `/api/admin/pages/${id}/publication`, { version }),

  tags: () => request<AdminTagList>("GET", "/api/admin/tags"),
  createTag: (name: string, slug: string) => request<AdminTag>("POST", "/api/admin/tags", { body: { name, slug } }),
  updateTag: (id: number, version: number, patch: TagPatch) =>
    request<AdminTag>("PATCH", `/api/admin/tags/${id}`, { body: patch, version }),
  deleteTag: (id: number, version: number) => request<void>("DELETE", `/api/admin/tags/${id}`, { version }),

  seriesList: () => request<AdminSeriesList>("GET", "/api/admin/series"),
  series: (id: number) => request<AdminSeriesDetail>("GET", `/api/admin/series/${id}`),
  createSeries: (input: SeriesInput) =>
    request<AdminSeries>("POST", "/api/admin/series", { body: input }),
  updateSeries: (id: number, version: number, patch: SeriesPatch) =>
    request<AdminSeries>("PATCH", `/api/admin/series/${id}`, { body: patch, version }),
  deleteSeries: (id: number, version: number) => request<void>("DELETE", `/api/admin/series/${id}`, { version }),
  reorderSeries: (id: number, postIds: number[]) =>
    request<AdminSeriesDetail>("PUT", `/api/admin/series/${id}/posts`, { body: { post_ids: postIds } }),

  media: (opts: { unused?: boolean; cursor?: string; limit?: number }) =>
    request<MediaList>("GET", "/api/admin/media", { query: { limit: 50, ...opts } }),
  mediaBySha: (sha256: string[]) => request<MediaList>("GET", "/api/admin/media", { query: { sha256 } }),
  mediaDetail: (id: number) => request<MediaDetail>("GET", `/api/admin/media/${id}`),
  updateMedia: (id: number, version: number, alt: string) =>
    request<Media>("PATCH", `/api/admin/media/${id}`, { body: { alt }, version }),
  deleteMedia: (id: number, version: number) => request<void>("DELETE", `/api/admin/media/${id}`, { version }),

  settings: () => request<AdminSettings>("GET", "/api/admin/settings"),
  updateSettings: (version: number, patch: SettingsInput) =>
    request<AdminSettings>("PATCH", "/api/admin/settings", { body: patch, version }),
};

// all 依次翻页取回全部条目，用于统计数量等需要完整集合的场景。
export async function all<T>(
  fetchPage: (cursor?: string) => Promise<{ items: T[]; next_cursor: string | null }>,
): Promise<T[]> {
  const items: T[] = [];
  let cursor: string | undefined;
  do {
    const page = await fetchPage(cursor);
    items.push(...page.items);
    cursor = page.next_cursor ?? undefined;
  } while (cursor);
  return items;
}
