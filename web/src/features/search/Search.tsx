import { Form, Link } from "react-router";
import type { SearchResult } from "~/shared/api/types";
import { formatDate, type Locale } from "~/shared/lib/format";
import { highlight } from "./highlight";

export const maxQueryLength = 64;

export function SearchForm({ query }: { query: string }) {
  return (
    <Form method="get" action="/search" role="search" className="mt-7 flex gap-3">
      <label htmlFor="search-q" className="sr-only">
        搜索文章
      </label>
      <input
        id="search-q"
        type="search"
        name="q"
        defaultValue={query}
        maxLength={maxQueryLength}
        required
        placeholder="标题、摘要或正文中的词句"
        className="h-12 min-w-0 flex-1 rounded-md border border-line bg-transparent px-4 text-lg text-ink focus:border-accent focus:outline-none"
        key={query}
      />
      <button type="submit" className="h-12 rounded-md border border-accent bg-accent px-5.5 text-base text-paper">
        搜索
      </button>
    </Form>
  );
}

export function SearchResults({ query, hits, locale }: { query: string; hits: SearchResult[]; locale: Locale }) {
  return (
    <>
      <p className="mt-8 text-md leading-loose text-muted" role="status">
        {hits.length > 0 ? `找到 ${hits.length} 篇与「${query}」相关的文章` : `没有找到与「${query}」相关的文章`}
      </p>
      <ol className="mt-2">
        {hits.map((hit) => (
          <li key={hit.slug} className="flex flex-col gap-2 border-b border-line-soft py-6">
            <h2 className="text-3xl leading-snug font-semibold">
              <Link to={`/posts/${encodeURIComponent(hit.slug)}`}>
                <Marked text={hit.title} query={query} />
              </Link>
            </h2>
            <p className="text-lg text-ink-soft">
              <Marked text={hit.snippet || hit.summary} query={query} />
            </p>
            <time dateTime={hit.published_at} className="text-md leading-loose text-muted">
              {formatDate(hit.published_at, locale)}
            </time>
          </li>
        ))}
      </ol>
    </>
  );
}

function Marked({ text, query }: { text: string; query: string }) {
  return highlight(text, query).map((s, i) => (s.match ? <mark key={i} className="rounded-xs bg-mark text-inherit">{s.text}</mark> : s.text));
}
