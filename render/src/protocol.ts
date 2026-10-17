// 渲染服务与 API 之间的线上格式。Image 与 API 返回的媒体结构一致。

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

export interface Issue {
  line: number;
  message: string;
  // error 阻止保存；warning 只在发布前提示。
  severity: "error" | "warning";
}

// RenderRequest 是 POST /render 的请求体；media 以 sha256 为键，只需包含正文引用到的图片。
export interface RenderRequest {
  markdown: string;
  media: Record<string, Image>;
}

// RenderResult 是 POST /render 的响应体。
export interface RenderResult {
  html: string;
  toc: TocItem[];
  plain_text: string;
  has_math: boolean;
  renderer_version: string;
  issues: Issue[];
}

// parseRequest 校验请求体结构，不合法时抛出带说明的 Error。
export function parseRequest(body: unknown): RenderRequest {
  if (!isObject(body)) throw new Error("请求体必须是 JSON 对象");
  const { markdown, media } = body;
  if (typeof markdown !== "string") throw new Error("markdown 必须是字符串");
  if (media === undefined || media === null) return { markdown, media: {} };
  if (!isObject(media)) throw new Error("media 必须是以 sha256 为键的对象");
  for (const [sha, image] of Object.entries(media)) {
    if (!isImage(image)) throw new Error(`media.${sha} 不是合法的图片信息`);
  }
  return { markdown, media: media as Record<string, Image> };
}

function isObject(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function isImage(v: unknown): v is Image {
  return (
    isObject(v) &&
    typeof v["sha256"] === "string" &&
    typeof v["width"] === "number" &&
    typeof v["height"] === "number" &&
    typeof v["alt"] === "string" &&
    typeof v["src"] === "string" &&
    Array.isArray(v["variants"]) &&
    v["variants"].every(
      (x) =>
        isObject(x) &&
        typeof x["width"] === "number" &&
        typeof x["height"] === "number" &&
        typeof x["format"] === "string" &&
        typeof x["url"] === "string",
    )
  );
}
