import { adminApi } from "~/shared/api/admin";
import type { AdminPageSummary, AdminPostSummary, Image, PageInput, PostInput } from "~/shared/api/types";
import { slugify } from "~/shared/lib/slug";

export type Kind = "post" | "page";

// EditorForm 是写作页可编辑的全部字段；页面只使用其中的标题、链接、描述与正文。
export interface EditorForm {
  title: string;
  slug: string;
  // 文章的摘要或页面的描述。
  summary: string;
  body: string;
  tagIds: number[];
  seriesId: number | null;
  cover: { id: number; image: Image } | null;
}

// Meta 是服务器上的状态，只在加载、保存与发布后更新。
export interface Meta {
  id: number;
  version: number;
  state: "draft" | "scheduled" | "published";
  publishedAt: string | null;
  rendererVersion: string;
  createdAt: string;
  updatedAt: string;
  series: { id: number; position: number } | null;
}

export interface Loaded {
  meta: Meta;
  form: EditorForm;
}

export const emptyForm: EditorForm = { title: "", slug: "", summary: "", body: "", tagIds: [], seriesId: null, cover: null };

export const untitled = "未命名草稿";

export function metaFrom(s: AdminPostSummary | AdminPageSummary): Meta {
  return {
    id: s.id,
    version: s.version,
    state: s.state,
    publishedAt: "published_at" in s ? s.published_at : null,
    rendererVersion: s.renderer_version,
    createdAt: s.created_at,
    updatedAt: s.updated_at,
    series: "series" in s && s.series ? { id: s.series.id, position: s.series.position } : null,
  };
}

export async function load(kind: Kind, id: number): Promise<Loaded> {
  if (kind === "page") {
    const p = await adminApi.page(id);
    return { meta: metaFrom(p), form: { ...emptyForm, title: p.title, slug: p.slug, summary: p.description, body: p.body_md } };
  }
  const p = await adminApi.post(id);
  let cover: EditorForm["cover"] = null;
  if (p.cover) {
    // 文章详情里的封面不带媒体 ID，保存时需要它，按哈希查一次。
    const found = (await adminApi.mediaBySha([p.cover.sha256])).items[0];
    if (found) cover = { id: found.id, image: p.cover };
  }
  return {
    meta: metaFrom(p),
    form: {
      title: p.title,
      slug: p.slug,
      summary: p.summary,
      body: p.body_md,
      tagIds: p.tags.map((t) => t.id),
      seriesId: p.series?.id ?? null,
      cover,
    },
  };
}

// withDefaults 补全首次保存所需的标题与链接：中文标题提取不出 slug 时用日期生成一个，作者随后可以改。
export function withDefaults(kind: Kind, form: EditorForm, now = new Date()): EditorForm {
  const title = form.title.trim() || untitled;
  let slug = form.slug.trim();
  if (!slug) {
    const stamp = now.toISOString().slice(0, 16).replace(/[-:T]/g, "");
    slug = (form.title.trim() && slugify(form.title)) || `${kind}-${stamp}`;
  }
  return { ...form, title, slug };
}

// 只提交 Markdown 原文与元数据，渲染由服务端完成。
export function postInput(form: EditorForm): PostInput {
  return {
    title: form.title,
    slug: form.slug,
    summary: form.summary.trim(),
    body_md: form.body,
    tag_ids: form.tagIds,
    series_id: form.seriesId,
    cover_media_id: form.cover?.id ?? null,
  };
}

export function pageInput(form: EditorForm): PageInput {
  return { title: form.title, slug: form.slug, description: form.summary.trim(), body_md: form.body };
}

export const sameForm = (a: EditorForm, b: EditorForm) => JSON.stringify(a) === JSON.stringify(b);

// fieldOf 把 Problem 的 JSON Pointer 映射到表单字段名。
export function fieldOf(pointer: string | undefined): keyof EditorForm | "other" {
  const key = pointer?.replace(/^#\//, "").split("/")[0];
  switch (key) {
    case "title":
    case "slug":
    case "summary":
      return key;
    case "description":
      return "summary";
    case "body_md":
      return "body";
    case "tag_ids":
      return "tagIds";
    case "series_id":
      return "seriesId";
    case "cover_media_id":
      return "cover";
    default:
      return "other";
  }
}
