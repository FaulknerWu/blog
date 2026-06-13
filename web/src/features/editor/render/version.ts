// 渲染器版本随渲染结果一并保存。修改渲染管线或升级 Shiki、Temml 等依赖导致输出变化时递增，
// 后台据此找出需要重新渲染的文章。单独成文件，列表页引用它时不会带上整条渲染管线。
export const RENDERER_VERSION = "2026.10.0";

// isStale 判断内容是否由旧版渲染器生成；空正文没有渲染结果，不算过期。
export function isStale(rendererVersion: string): boolean {
  return rendererVersion !== "" && rendererVersion !== RENDERER_VERSION;
}
