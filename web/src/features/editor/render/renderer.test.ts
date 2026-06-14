import { describe, expect, it } from "vitest";
import type { Image } from "~/shared/api/types";
import { RENDERER_VERSION, renderMarkdown, type MediaLookup } from "./renderer";
import { countWords } from "./wordcount";

const sha = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08";
const topology: Image = {
  sha256: sha,
  width: 3024,
  height: 2016,
  alt: "",
  src: `/media/${sha}`,
  variants: [
    { width: 480, height: 320, format: "webp", url: `/media/${sha}/480.webp` },
    { width: 960, height: 640, format: "webp", url: `/media/${sha}/960.webp` },
  ],
};
const lookup: MediaLookup = async (shas) => new Map(shas.filter((s) => s === sha).map((s) => [s, topology]));
const none: MediaLookup = async () => new Map();

describe("renderMarkdown", () => {
  it("builds heading ids and a toc from h2–h6", async () => {
    const { rendered } = await renderMarkdown("# 标题\n\n## 背景\n\n### 读路径\n\n## 背景\n", none);
    expect(rendered.toc).toEqual([
      { depth: 2, id: "背景", text: "背景" },
      { depth: 3, id: "读路径", text: "读路径" },
      { depth: 2, id: "背景-1", text: "背景" },
    ]);
    expect(rendered.html).toContain('<h2 id="背景">背景</h2>');
    expect(rendered.renderer_version).toBe(RENDERER_VERSION);
  });

  it("highlights code with dual-theme variables", async () => {
    const { rendered } = await renderMarkdown("```ts\nconst a = 1\n```\n\n```unknown\nplain\n```\n", none);
    expect(rendered.html).toContain('class="shiki shiki-themes github-light github-dark"');
    expect(rendered.html).toContain("--shiki-dark:");
    expect(rendered.html).toContain('data-lang="typescript"');
    expect(rendered.html).toContain(">plain</span>");
  });

  it("compiles math to MathML and reports errors with line numbers", async () => {
    const ok = await renderMarkdown("质能方程 $E = mc^2$\n\n$$\n\\frac{a}{b}\n$$\n", none);
    expect(ok.rendered.has_math).toBe(true);
    expect(ok.rendered.html).toContain("<math");
    expect(ok.rendered.html).toContain('display="block"');
    expect(ok.rendered.plain_text).not.toContain("frac");
    expect(ok.issues).toEqual([]);

    const bad = await renderMarkdown("段落\n\n$$\n\\frac{a}{b\n$$\n", none);
    expect(bad.issues).toHaveLength(1);
    expect(bad.issues[0]).toMatchObject({ line: 4, severity: "error" });
  });

  it("expands media images into responsive figures", async () => {
    const { rendered, issues } = await renderMarkdown(`![部署拓扑](/media/${sha} "图 1")\n`, lookup);
    expect(rendered.html).toBe(
      "<figure><picture>" +
        `<source type="image/webp" sizes="(min-width: 712px) 680px, calc(100vw - 32px)" srcset="/media/${sha}/480.webp 480w, /media/${sha}/960.webp 960w">` +
        `<img src="/media/${sha}" alt="部署拓扑" width="3024" height="2016" loading="lazy" decoding="async">` +
        "</picture><figcaption>图 1</figcaption></figure>",
    );
    expect(issues).toEqual([]);
  });

  it("flags unknown media as an error and missing alt as a warning", async () => {
    const { issues } = await renderMarkdown(`文字\n\n![](/media/${"a".repeat(64)})\n`, none);
    expect(issues.map((i) => [i.line, i.severity])).toEqual([
      [3, "warning"],
      [3, "error"],
    ]);
  });

  it("drops raw HTML", async () => {
    const { rendered } = await renderMarkdown('<script>alert(1)</script>\n\n<b onclick="x">hi</b>\n', none);
    expect(rendered.html).not.toContain("script");
    expect(rendered.html).not.toContain("onclick");
  });

  it("supports GFM tables and task lists", async () => {
    const { rendered } = await renderMarkdown("| a | b |\n| - | - |\n| 1 | 2 |\n\n- [x] done\n", none);
    expect(rendered.html).toContain("<table>");
    expect(rendered.html).toContain('type="checkbox"');
  });
});

describe("countWords", () => {
  it("matches the API's mixed CJK and Latin counting", () => {
    expect(countWords("写时渲染 render-on-write, don't 2026")).toBe(7);
  });
});
