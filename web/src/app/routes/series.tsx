import { SeriesChapters } from "~/features/taxonomy/Taxonomy";
import { getSeries } from "~/shared/api/public.server";
import { Page } from "../layout/Page";
import { seo, siteFrom, useLocale } from "../site";
import type { Route } from "./+types/series";

export async function loader({ request, params }: Route.LoaderArgs) {
  return { series: await getSeries(request, params.slug) };
}

export function meta({ data, matches, location }: Route.MetaArgs) {
  if (!data) return [];
  return seo(siteFrom(matches), {
    title: data.series.title,
    description: data.series.description,
    path: location.pathname,
  });
}

export default function SeriesRoute({ loaderData: { series } }: Route.ComponentProps) {
  const locale = useLocale();
  return (
    <Page eyebrow={`专栏 · ${series.posts.length} 篇`} title={series.title} description={series.description || undefined}>
      <SeriesChapters series={series} locale={locale} />
    </Page>
  );
}
