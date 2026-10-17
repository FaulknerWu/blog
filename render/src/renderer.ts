import GithubSlugger from "github-slugger";
import type { Element, ElementContent, Root, RootContent } from "hast";
import { toText } from "hast-util-to-text";
import rehypeStringify from "rehype-stringify";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import remarkParse from "remark-parse";
import remarkRehype from "remark-rehype";
import temml from "temml";
import { unified } from "unified";
import { SKIP, visit } from "unist-util-visit";
import { prepare, resolveLang } from "./highlight.ts";
import type { Image, Issue, RenderResult, TocItem } from "./protocol.ts";
import { RENDERER_VERSION } from "./version.ts";

// 与公开站正文栏宽一致（--measure 680px，两侧各 16px 留白）。
const imageSizes = "(min-width: 712px) 680px, calc(100vw - 32px)";
const mediaPattern = /^\/media\/([0-9a-f]{64})$/;

const processor = unified().use(remarkParse).use(remarkGfm).use(remarkMath).use(remarkRehype);
const serializer = unified().use(rehypeStringify, { allowDangerousHtml: true });

// renderMarkdown 渲染正文。media 由调用方按正文引用的 sha256 查好传入，缺失的图片记为错误。
export async function renderMarkdown(markdown: string, media: Map<string, Image>): Promise<RenderResult> {
  const tree = (await processor.run(processor.parse(markdown))) as Root;
  const issues: Issue[] = [];

  const langs = new Set<string>();
  visit(tree, "element", (node) => {
    if (node.tagName === "pre") {
      const lang = codeLang(node);
      const id = lang && lang !== "math" ? resolveLang(lang) : undefined;
      if (id) langs.add(id);
    }
  });
  const highlight = await prepare(langs);

  let hasMath = false;
  const slugger = new GithubSlugger();
  const toc: TocItem[] = [];

  visit(tree, "element", (node, index, parent) => {
    if (!parent || index === undefined) return;
    const line = node.position?.start.line ?? 1;

    if (node.tagName === "pre") {
      const lang = codeLang(node);
      const code = toText(node, { whitespace: "pre" }).replace(/\n$/, "");
      if (lang === "math") {
        hasMath = true;
        // 公式块的第一行是 $$，错误定位到公式内容所在的下一行。
        parent.children[index] = math(code, true, line + 1, issues);
      } else {
        parent.children[index] = highlight(code, lang ? resolveLang(lang) : undefined);
      }
      return SKIP;
    }

    if (node.tagName === "code" && hasClass(node, "math-inline")) {
      hasMath = true;
      parent.children[index] = math(toText(node), false, line, issues);
      return SKIP;
    }

    const depth = /^h([1-6])$/.exec(node.tagName)?.[1];
    if (depth) {
      const text = toText(node).trim();
      const id = slugger.slug(text || "section");
      node.properties["id"] = id;
      // 正文的一级标题与文章标题冲突，不进入目录。
      if (depth !== "1") toc.push({ depth: Number(depth), id, text });
      return;
    }

    if (node.tagName === "p") {
      const meaningful = node.children.filter((c) => !(c.type === "text" && !c.value.trim()));
      const only = meaningful[0];
      if (meaningful.length === 1 && only?.type === "element" && only.tagName === "img") {
        parent.children[index] = figure(only, media, line, issues);
        return SKIP;
      }
      return;
    }

    if (node.tagName === "img") {
      parent.children[index] = image(node, media, line, issues);
      return SKIP;
    }
  });

  const html = serializer.stringify(tree);
  const plainText = toText(tree).replace(/\n{3,}/g, "\n\n").trim();
  return { html, toc, plain_text: plainText, has_math: hasMath, renderer_version: RENDERER_VERSION, issues };
}

function hasClass(node: Element, name: string): boolean {
  const cls = node.properties["className"];
  return Array.isArray(cls) && cls.includes(name);
}

// codeLang 返回围栏代码块的语言标记，remark-rehype 把它写成 <code class="language-xxx">。
function codeLang(pre: Element): string | undefined {
  const code = pre.children.find((c): c is Element => c.type === "element" && c.tagName === "code");
  const cls = code?.properties["className"];
  if (!Array.isArray(cls)) return undefined;
  const lang = cls.map(String).find((c) => c.startsWith("language-"));
  return lang?.slice("language-".length).toLowerCase();
}

function math(tex: string, display: boolean, line: number, issues: Issue[]): ElementContent {
  try {
    const value = temml.renderToString(tex, { displayMode: display, throwOnError: true, annotate: true });
    return { type: "raw", value } as unknown as ElementContent;
  } catch (err) {
    const reason = err instanceof Error ? err.message.replace(/^Temml parse error:\s*/i, "") : String(err);
    issues.push({ line, message: `公式无法编译：${reason}`, severity: "error" });
    return {
      type: "element",
      tagName: display ? "pre" : "code",
      properties: { dataMathError: "" },
      children: [{ type: "text", value: tex }],
    };
  }
}

function figure(img: Element, media: Map<string, Image>, line: number, issues: Issue[]): Element {
  const children: ElementContent[] = [image(img, media, line, issues)];
  const title = img.properties["title"];
  if (typeof title === "string" && title.trim()) {
    children.push({ type: "element", tagName: "figcaption", properties: {}, children: [{ type: "text", value: title }] });
  }
  return { type: "element", tagName: "figure", properties: {}, children };
}

// image 把媒体库图片展开为带多尺寸来源与原图宽高的 <picture>，外部图片保持原样并延迟加载。
function image(img: Element, media: Map<string, Image>, line: number, issues: Issue[]): Element {
  const src = String(img.properties["src"] ?? "");
  const alt = String(img.properties["alt"] ?? "");
  if (!alt.trim()) issues.push({ line, message: "图片缺少替代文本", severity: "warning" });

  const sha = mediaPattern.exec(src)?.[1];
  if (!sha) {
    return { ...img, properties: { src, alt, loading: "lazy", decoding: "async" } };
  }
  const info = media.get(sha);
  if (!info) {
    issues.push({ line, message: `媒体库中没有图片 ${src.slice(0, 15)}…`, severity: "error" });
    return { ...img, properties: { src, alt } };
  }
  const sources: RootContent[] = [...Map.groupBy(info.variants, (v) => v.format)].map(([format, variants]) => ({
    type: "element",
    tagName: "source",
    properties: {
      type: `image/${format}`,
      sizes: imageSizes,
      srcSet: variants.map((v) => `${v.url} ${v.width}w`).join(", "),
    },
    children: [],
  }));
  return {
    type: "element",
    tagName: "picture",
    properties: {},
    children: [
      ...(sources as ElementContent[]),
      {
        type: "element",
        tagName: "img",
        properties: { src: info.src, alt, width: info.width, height: info.height, loading: "lazy", decoding: "async" },
        children: [],
      },
    ],
  };
}
