// 管理与认证接口的数据结构，与 docs/api-design.md 第 4、5 节保持一致。
// OpenAPI 契约就绪后改为由契约生成，届时删除本文件。
import type { Image, SiteSettings, Timestamp, TocItem } from "./types";

export interface User {
  id: number;
  username: string;
  display_name: string;
}

export interface Session {
  id: number;
  current: boolean;
  user_agent: string;
  ip: string;
  created_at: Timestamp;
  last_seen_at: Timestamp;
  expires_at: Timestamp;
}

export type PostState = "draft" | "scheduled" | "published";
export type PageState = "draft" | "published";

export interface AdminTagRef {
  id: number;
  slug: string;
  name: string;
}

export interface AdminPostSummary {
  id: number;
  slug: string;
  title: string;
  state: PostState;
  published_at: Timestamp | null;
  renderer_version: string;
  tags: AdminTagRef[];
  series: { id: number; title: string; position: number } | null;
  version: number;
  created_at: Timestamp;
  updated_at: Timestamp;
}

export interface AdminPost extends AdminPostSummary {
  summary: string;
  body_md: string;
  cover: Image | null;
}

export interface RenderedInput {
  html: string;
  toc: TocItem[];
  plain_text: string;
  has_math: boolean;
  renderer_version: string;
}

export interface PostInput {
  title: string;
  slug: string;
  summary: string;
  body_md: string;
  rendered: RenderedInput | null;
  tag_ids: number[];
  series_id: number | null;
  cover_media_id: number | null;
}

export interface AdminPageSummary {
  id: number;
  slug: string;
  title: string;
  state: PageState;
  renderer_version: string;
  version: number;
  created_at: Timestamp;
  updated_at: Timestamp;
}

export interface AdminPage extends AdminPageSummary {
  description: string;
  body_md: string;
}

export interface PageInput {
  title: string;
  slug: string;
  description: string;
  body_md: string;
  rendered: RenderedInput | null;
}

export interface AdminTag {
  id: number;
  slug: string;
  name: string;
  post_count: number;
  published_post_count: number;
  version: number;
  created_at: Timestamp;
  updated_at: Timestamp;
}

export interface AdminSeries {
  id: number;
  slug: string;
  title: string;
  description: string;
  post_count: number;
  published_post_count: number;
  version: number;
  created_at: Timestamp;
  updated_at: Timestamp;
}

export interface AdminSeriesDetail extends AdminSeries {
  posts: { id: number; slug: string; title: string; state: PostState; position: number }[];
}

export interface Media extends Image {
  id: number;
  byte_size: number;
  original_filename: string;
  reference_count: number;
  version: number;
  created_at: Timestamp;
  updated_at: Timestamp;
}

export interface MediaReference {
  type: "post" | "page";
  id: number;
  title: string;
}

export interface MediaDetail extends Media {
  referenced_by: MediaReference[];
}

export interface AdminSettings extends SiteSettings {
  feed_item_count: number;
  version: number;
  updated_at: Timestamp;
}

export type SettingsInput = Partial<Omit<AdminSettings, "version" | "updated_at">>;
