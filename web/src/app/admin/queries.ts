import { MutationCache, QueryCache, QueryClient, queryOptions } from "@tanstack/react-query";
import { adminApi, all, ApiError } from "~/shared/api/admin";

// 会话失效时整页跳转到登录页；写作页的未保存内容已暂存在本地，不会丢失。
function onError(error: unknown) {
  if (error instanceof ApiError && error.status === 401 && typeof window !== "undefined") {
    const next = window.location.pathname + window.location.search;
    window.location.assign(`/admin/login?next=${encodeURIComponent(next)}`);
  }
}

export const queryClient = new QueryClient({
  queryCache: new QueryCache({ onError }),
  mutationCache: new MutationCache({ onError }),
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
      retry: (count, error) => !(error instanceof ApiError && error.status > 0 && error.status < 500) && count < 2,
    },
  },
});

export const q = {
  me: queryOptions({ queryKey: ["me"], queryFn: adminApi.me, staleTime: Infinity }),
  // 当前渲染器版本，用来标出由旧版渲染器生成的内容；一次会话内视为不变。
  renderer: queryOptions({ queryKey: ["renderer"], queryFn: adminApi.renderer, staleTime: Infinity }),
  sessions: queryOptions({ queryKey: ["sessions"], queryFn: async () => (await adminApi.sessions()).items }),

  // 全部文章摘要，用于统计数量与概览页；单人博客的文章量很小，按 100 条一页取完。
  allPosts: queryOptions({
    queryKey: ["posts", "all"],
    queryFn: () => all((cursor) => adminApi.posts({}, cursor, 100)),
  }),
  post: (id: number) => queryOptions({ queryKey: ["post", id], queryFn: () => adminApi.post(id), staleTime: 0 }),

  pages: queryOptions({ queryKey: ["pages"], queryFn: async () => (await adminApi.pages()).items }),
  page: (id: number) => queryOptions({ queryKey: ["page", id], queryFn: () => adminApi.page(id), staleTime: 0 }),

  tags: queryOptions({ queryKey: ["tags"], queryFn: async () => (await adminApi.tags()).items }),
  seriesList: queryOptions({ queryKey: ["series"], queryFn: async () => (await adminApi.seriesList()).items }),
  series: (id: number) => queryOptions({ queryKey: ["series", id], queryFn: () => adminApi.series(id) }),

  allMedia: queryOptions({
    queryKey: ["media", "all"],
    queryFn: () => all((cursor) => adminApi.media({ cursor, limit: 100 })),
  }),
  mediaDetail: (id: number) => queryOptions({ queryKey: ["media", "detail", id], queryFn: () => adminApi.mediaDetail(id) }),

  settings: queryOptions({ queryKey: ["settings"], queryFn: adminApi.settings, staleTime: 0 }),
};
