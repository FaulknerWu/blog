import type { Element } from "hast";
import { createHighlighterCore, type HighlighterCore, type LanguageRegistration } from "shiki/core";
import { createJavaScriptRegexEngine } from "shiki/engine/javascript";

type LangModule = { default: LanguageRegistration[] };

// 按需加载的语法，只列常用语言以控制启动与内存开销；其余语言按纯文本输出。
const loaders: Record<string, () => Promise<LangModule>> = {
  bash: () => import("shiki/langs/bash.mjs"),
  c: () => import("shiki/langs/c.mjs"),
  cpp: () => import("shiki/langs/cpp.mjs"),
  csharp: () => import("shiki/langs/csharp.mjs"),
  css: () => import("shiki/langs/css.mjs"),
  diff: () => import("shiki/langs/diff.mjs"),
  docker: () => import("shiki/langs/docker.mjs"),
  go: () => import("shiki/langs/go.mjs"),
  graphql: () => import("shiki/langs/graphql.mjs"),
  html: () => import("shiki/langs/html.mjs"),
  http: () => import("shiki/langs/http.mjs"),
  ini: () => import("shiki/langs/ini.mjs"),
  java: () => import("shiki/langs/java.mjs"),
  javascript: () => import("shiki/langs/javascript.mjs"),
  json: () => import("shiki/langs/json.mjs"),
  jsx: () => import("shiki/langs/jsx.mjs"),
  kotlin: () => import("shiki/langs/kotlin.mjs"),
  lua: () => import("shiki/langs/lua.mjs"),
  make: () => import("shiki/langs/make.mjs"),
  markdown: () => import("shiki/langs/markdown.mjs"),
  nginx: () => import("shiki/langs/nginx.mjs"),
  php: () => import("shiki/langs/php.mjs"),
  powershell: () => import("shiki/langs/powershell.mjs"),
  proto: () => import("shiki/langs/proto.mjs"),
  python: () => import("shiki/langs/python.mjs"),
  ruby: () => import("shiki/langs/ruby.mjs"),
  rust: () => import("shiki/langs/rust.mjs"),
  scss: () => import("shiki/langs/scss.mjs"),
  sql: () => import("shiki/langs/sql.mjs"),
  swift: () => import("shiki/langs/swift.mjs"),
  toml: () => import("shiki/langs/toml.mjs"),
  tsx: () => import("shiki/langs/tsx.mjs"),
  typescript: () => import("shiki/langs/typescript.mjs"),
  xml: () => import("shiki/langs/xml.mjs"),
  yaml: () => import("shiki/langs/yaml.mjs"),
};

const aliases: Record<string, string> = {
  sh: "bash",
  shell: "bash",
  zsh: "bash",
  console: "bash",
  "c++": "cpp",
  cs: "csharp",
  dockerfile: "docker",
  golang: "go",
  gql: "graphql",
  js: "javascript",
  mjs: "javascript",
  cjs: "javascript",
  kt: "kotlin",
  makefile: "make",
  md: "markdown",
  ps1: "powershell",
  protobuf: "proto",
  py: "python",
  rb: "ruby",
  rs: "rust",
  ts: "typescript",
  mts: "typescript",
  yml: "yaml",
  svg: "xml",
  conf: "ini",
};

export function resolveLang(lang: string): string | undefined {
  const id = aliases[lang] ?? lang;
  return id in loaders ? id : undefined;
}

let highlighter: Promise<HighlighterCore> | undefined;

function getHighlighter(): Promise<HighlighterCore> {
  highlighter ??= createHighlighterCore({
    themes: [import("shiki/themes/github-light.mjs"), import("shiki/themes/github-dark.mjs")],
    langs: [],
    engine: createJavaScriptRegexEngine(),
  });
  return highlighter;
}

// prepare 加载代码块用到的语法，返回同步的高亮函数。
export async function prepare(langs: Iterable<string>): Promise<(code: string, lang: string | undefined) => Element> {
  const hl = await getHighlighter();
  const loaded = new Set(hl.getLoadedLanguages());
  for (const lang of new Set(langs)) {
    if (!loaded.has(lang)) {
      const loader = loaders[lang];
      if (loader) await hl.loadLanguage((await loader()).default);
    }
  }
  return (code, lang) => {
    const root = hl.codeToHast(code, {
      lang: lang ?? "text",
      // 只输出 --shiki-light / --shiki-dark 变量，由公开站样式按系统配色切换。
      themes: { light: "github-light", dark: "github-dark" },
      defaultColor: false,
    });
    const pre = root.children[0] as Element;
    if (lang) pre.properties["dataLang"] = lang;
    return pre;
  };
}
