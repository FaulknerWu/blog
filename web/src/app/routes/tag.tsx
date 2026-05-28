import { PostList } from "~/features/post/PostList";
import { getSettings, getTag, listPosts } from "~/shared/api/public.server";
import { Page, Pager } from "../layout/Page";
import { seo, siteFrom, useLocale } from "../site";
import type { Route } from "./+types/tag";

export async function loader({ request, params }: Route.LoaderArgs) {
  const cursor = new URL(request.url).searchParams.get("cursor");
  const [tag, settings] = await Promise.all([getTag(request, params.slug), getSettings(request)]);
  const page = await listPosts(request, { tag: tag.slug, limit: settings.posts_per_page, cursor });
  return { tag, posts: page.items, nextCursor: page.next_cursor, isFirst: !cursor };
}

export function meta({ data, matches, location }: Route.MetaArgs) {
  if (!data) return [];
  return seo(siteFrom(matches), { title: `#${data.tag.name}`, path: location.pathname });
}

export default function TagRoute({ loaderData }: Route.ComponentProps) {
  const locale = useLocale();
  const { tag } = loaderData;
  return (
    <Page eyebrow="标签" title={`#${tag.name}`} description={`共 ${tag.post_count} 篇文章`}>
      <PostList posts={loaderData.posts} locale={locale} groupByYear />
      <Pager
        basePath={`/tags/${encodeURIComponent(tag.slug)}`}
        nextCursor={loaderData.nextCursor}
        isFirst={loaderData.isFirst}
      />
    </Page>
  );
}
