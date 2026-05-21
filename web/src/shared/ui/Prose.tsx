// Prose 输出写时渲染好的正文 HTML。HTML 由作者后台生成并经 API 存储，读路径不再处理。
// 排版规则在 shared/styles/prose.css。
export function Prose({ html }: { html: string }) {
  return <div className="prose" dangerouslySetInnerHTML={{ __html: html }} />;
}
