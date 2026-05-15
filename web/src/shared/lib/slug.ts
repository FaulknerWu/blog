// slug 规则与 API 一致：小写字母、数字与单个连字符，长度不超过 100。
export const slugPattern = /^[a-z0-9]+(-[a-z0-9]+)*$/;
export const maxSlug = 100;

// slugify 从标题中提取拉丁字母与数字生成 slug；中文标题提取不到内容时返回空串，由作者手动填写。
export function slugify(text: string): string {
  return text
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, maxSlug)
    .replace(/-+$/, "");
}

export function slugError(slug: string): string | undefined {
  if (!slug) return "请填写链接";
  if (slug.length > maxSlug) return `不能超过 ${maxSlug} 个字符`;
  if (!slugPattern.test(slug)) return "只能包含小写字母、数字和单个连字符";
  return undefined;
}
