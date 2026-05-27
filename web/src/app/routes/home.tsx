import { PostList } from "~/features/post/PostList";
import { getSettings, listPosts } from "~/shared/api/public.server";
import { Page, Pager } from "../layout/Page";
import { seo, siteFrom, useLocale, type Handle } from "../site";
import type { Route } from "./+types/home";

export const handle: Handle = { masthead: true };

export async function loader({ request }: Route.LoaderArgs) {
  const cursor = new URL(request.url).searchParams.get("cursor");
  const { posts_per_page } = await getSettings(request);
  const page = await listPosts(request, { limit: posts_per_page, cursor });
  return { posts: page.items, nextCursor: page.next_cursor, isFirst: !cursor };
}

export function meta({ matches, location }: Route.MetaArgs) {
  return seo(siteFrom(matches), { path: location.pathname });
}

export default function Home({ loaderData }: Route.ComponentProps) {
  const locale = useLocale();
  return (
    <Page>
      <PostList posts={loaderData.posts} locale={locale} groupByYear />
      <Pager basePath="/" nextCursor={loaderData.nextCursor} isFirst={loaderData.isFirst} />
    </Page>
  );
}
