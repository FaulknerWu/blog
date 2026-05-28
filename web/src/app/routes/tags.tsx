import { TagIndex } from "~/features/taxonomy/Taxonomy";
import { listTags } from "~/shared/api/public.server";
import { Page } from "../layout/Page";
import { seo, siteFrom } from "../site";
import type { Route } from "./+types/tags";

export async function loader({ request }: Route.LoaderArgs) {
  return { tags: (await listTags(request)).items };
}

export function meta({ matches, location }: Route.MetaArgs) {
  return seo(siteFrom(matches), { title: "标签", path: location.pathname });
}

export default function TagsRoute({ loaderData }: Route.ComponentProps) {
  return (
    <Page title="标签" description={`共 ${loaderData.tags.length} 个标签`}>
      <TagIndex tags={loaderData.tags} />
    </Page>
  );
}
