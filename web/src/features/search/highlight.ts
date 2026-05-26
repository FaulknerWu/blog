export interface Segment {
  text: string;
  match: boolean;
}

// highlight 把文本按查询词（忽略大小写）切分，供渲染时标记命中部分。
export function highlight(text: string, query: string): Segment[] {
  const q = query.trim().toLowerCase();
  if (!q) return [{ text, match: false }];

  const lower = text.toLowerCase();
  const out: Segment[] = [];
  let from = 0;
  for (let i = lower.indexOf(q); i !== -1; i = lower.indexOf(q, from)) {
    if (i > from) out.push({ text: text.slice(from, i), match: false });
    out.push({ text: text.slice(i, i + q.length), match: true });
    from = i + q.length;
  }
  if (from < text.length) out.push({ text: text.slice(from), match: false });
  return out;
}
