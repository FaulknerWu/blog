import type { AdminPostSummary } from "~/shared/api/admin-types";
import { formatWhen } from "~/shared/lib/datetime";
import type { Locale } from "~/shared/lib/format";
import { Status } from "~/shared/ui/admin/ui";

export function PostStatus({ post, locale }: { post: AdminPostSummary; locale: Locale }) {
  if (post.state === "published") return <Status kind="published">已发布</Status>;
  if (post.state === "scheduled" && post.published_at) {
    return <Status kind="scheduled">定时 {formatWhen(post.published_at, locale)}</Status>;
  }
  return <Status kind="draft">草稿</Status>;
}
