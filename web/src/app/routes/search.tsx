import { maxQueryLength, SearchForm, SearchResults } from "~/features/search/Search";
import { search } from "~/shared/api/public.server";
import { Page } from "../layout/Page";
import { seo, siteFrom, useLocale } from "../site";
import type { Route } from "./+types/search";

export async function loader({ request }: Route.LoaderArgs) {
  const q = (new URL(request.url).searchParams.get("q") ?? "").trim();
  // 与 API 的校验一致：按字符计数，超长时截断而不是报错。
  const query = [...q].slice(0, maxQueryLength).join("");
  if (!query) return { query, hits: null };
  return { query, hits: (await search(request, query)).items };
}

export function meta({ data, matches, location }: Route.MetaArgs) {
  return [
    ...seo(siteFrom(matches), { title: data?.query ? `搜索：${data.query}` : "搜索", path: location.pathname }),
    { name: "robots", content: "noindex" },
  ];
}

export default function SearchRoute({ loaderData }: Route.ComponentProps) {
  const locale = useLocale();
  return (
    <Page title="搜索">
      <SearchForm query={loaderData.query} />
      {loaderData.hits && <SearchResults query={loaderData.query} hits={loaderData.hits} locale={locale} />}
    </Page>
  );
}
