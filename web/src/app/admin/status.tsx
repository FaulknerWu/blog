import { useQuery } from "@tanstack/react-query";
import { useCallback } from "react";
import type { AdminPostSummary } from "~/shared/api/types";
import { formatWhen } from "~/shared/lib/datetime";
import type { Locale } from "~/shared/lib/format";
import { Status } from "~/shared/ui/admin/ui";
import { q } from "./queries";

export function PostStatus({ post, locale }: { post: AdminPostSummary; locale: Locale }) {
  if (post.state === "published") return <Status kind="published">已发布</Status>;
  if (post.state === "scheduled" && post.published_at) {
    return <Status kind="scheduled">定时 {formatWhen(post.published_at, locale)}</Status>;
  }
  return <Status kind="draft">草稿</Status>;
}

// useRenderer 返回服务端当前的渲染器版本，以及判断内容是否由旧版渲染器生成的函数。
// 空版本表示正文为空、没有渲染结果；当前版本未知时一律不视为过期。
export function useRenderer() {
  const current = useQuery(q.renderer).data?.renderer_version;
  const isStale = useCallback(
    (rendererVersion: string) => current !== undefined && rendererVersion !== "" && rendererVersion !== current,
    [current],
  );
  return { current, isStale };
}
