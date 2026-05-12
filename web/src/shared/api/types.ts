// 公开接口的数据结构，与 docs/api-design.md 第 3 节保持一致。
// OpenAPI 契约就绪后改为由契约生成，届时删除本文件。

export type Timestamp = string;

export interface TagRef {
  slug: string;
  name: string;
}

export interface PostRef {
  slug: string;
  title: string;
}

export interface SeriesRef {
  slug: string;
  title: string;
}

export interface ImageVariant {
  width: number;
  height: number;
  format: string;
  url: string;
}

export interface Image {
  sha256: string;
  width: number;
  height: number;
  alt: string;
  src: string;
  variants: ImageVariant[];
}

export interface TocItem {
  depth: number;
  id: string;
  text: string;
}

export interface PostSummary {
  id: number;
  slug: string;
  title: string;
  summary: string;
  word_count: number;
  published_at: Timestamp;
  updated_at: Timestamp;
  tags: TagRef[];
  series: SeriesRef | null;
  cover: Image | null;
}

export interface SeriesNav extends SeriesRef {
  index: number;
  total: number;
  prev: PostRef | null;
  next: PostRef | null;
}

export interface Post extends Omit<PostSummary, "series"> {
  html: string;
  toc: TocItem[];
  has_math: boolean;
  series: SeriesNav | null;
  adjacent: { newer: PostRef | null; older: PostRef | null };
}

export interface Page {
  slug: string;
  title: string;
  description: string;
  html: string;
  toc: TocItem[];
  has_math: boolean;
  updated_at: Timestamp;
}

export interface Tag extends TagRef {
  post_count: number;
}

export interface SeriesSummary extends SeriesRef {
  description: string;
  post_count: number;
  last_published_at: Timestamp;
}

export interface SeriesDetail extends SeriesRef {
  description: string;
  posts: { index: number; slug: string; title: string; published_at: Timestamp }[];
}

export interface SearchHit {
  slug: string;
  title: string;
  summary: string;
  snippet: string;
  published_at: Timestamp;
}

export interface SiteSettings {
  title: string;
  subtitle: string;
  description: string;
  author: string;
  language: string;
  timezone: string;
  footer_text: string;
  navigation: { label: string; url: string }[];
  social_links: { platform: string; url: string }[];
  noindex: boolean;
  posts_per_page: number;
}

export interface Items<T> {
  items: T[];
}

export interface Paged<T> extends Items<T> {
  next_cursor: string | null;
}
