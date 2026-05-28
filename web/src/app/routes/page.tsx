import { getPage } from "~/shared/api/public.server";
import { formatDate } from "~/shared/lib/format";
import { ArticleLayout } from "~/shared/ui/ArticleLayout";
import { Prose } from "~/shared/ui/Prose";
import { hasToc, Toc } from "~/shared/ui/Toc";
import { pageEyebrow, pageTitle } from "../layout/Page";
import { seo, siteFrom, useLocale } from "../site";
import type { Route } from "./+types/page";

export async function loader({ request, params }: Route.LoaderArgs) {
  return { page: await getPage(request, params.slug) };
}

export function meta({ data, matches, location }: Route.MetaArgs) {
  if (!data) return [];
  return seo(siteFrom(matches), { title: data.page.title, description: data.page.description, path: location.pathname });
}

export default function PageRoute({ loaderData: { page } }: Route.ComponentProps) {
  const locale = useLocale();
  return (
    <ArticleLayout
      header={<h1 className={pageTitle}>{page.title}</h1>}
      toc={hasToc(page.toc) ? <Toc items={page.toc} /> : undefined}
      footer={
        <p className={pageEyebrow}>
          最后更新于 <time dateTime={page.updated_at}>{formatDate(page.updated_at, locale)}</time>
        </p>
      }
    >
      <Prose html={page.html} />
    </ArticleLayout>
  );
}
