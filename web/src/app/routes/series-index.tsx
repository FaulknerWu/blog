import { SeriesIndex } from "~/features/taxonomy/Taxonomy";
import { listSeries } from "~/shared/api/public.server";
import { Page } from "../layout/Page";
import { seo, siteFrom, useLocale } from "../site";
import type { Route } from "./+types/series-index";

export async function loader({ request }: Route.LoaderArgs) {
  return { series: (await listSeries(request)).items };
}

export function meta({ matches, location }: Route.MetaArgs) {
  return seo(siteFrom(matches), { title: "专栏", path: location.pathname });
}

export default function SeriesIndexRoute({ loaderData }: Route.ComponentProps) {
  const locale = useLocale();
  return (
    <Page title="专栏" description="成系列的长篇写作，按顺序阅读效果更好。">
      <SeriesIndex series={loaderData.series} locale={locale} />
    </Page>
  );
}
