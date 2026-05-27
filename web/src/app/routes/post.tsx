import { PostHeader } from "~/features/post/PostHeader";
import { PostNav } from "~/features/post/PostNav";
import { getPost } from "~/shared/api/public.server";
import { ArticleLayout } from "~/shared/ui/ArticleLayout";
import { Prose } from "~/shared/ui/Prose";
import { hasToc, Toc } from "~/shared/ui/Toc";
import { seo, siteFrom, useLocale } from "../site";
import type { Route } from "./+types/post";

export async function loader({ request, params }: Route.LoaderArgs) {
  return { post: await getPost(request, params.slug) };
}

export function meta({ data, matches, location }: Route.MetaArgs) {
  if (!data) return [];
  const { post } = data;
  return seo(siteFrom(matches), {
    title: post.title,
    description: post.summary,
    path: location.pathname,
    type: "article",
    image: post.cover?.src,
    publishedTime: post.published_at,
    modifiedTime: post.updated_at,
  });
}

export default function PostRoute({ loaderData: { post } }: Route.ComponentProps) {
  const locale = useLocale();
  return (
    <ArticleLayout
      header={<PostHeader post={post} locale={locale} />}
      toc={hasToc(post.toc) ? <Toc items={post.toc} /> : undefined}
      footer={<PostNav post={post} />}
    >
      <Prose html={post.html} />
    </ArticleLayout>
  );
}
