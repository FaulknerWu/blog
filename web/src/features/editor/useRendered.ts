import { useEffect, useState } from "react";
import { adminApi } from "~/shared/api/admin";
import type { RenderPreview } from "~/shared/api/types";

// useRendered 在输入停顿后请求服务端渲染正文；首次渲染不等待。新的输入会中止仍在进行的旧请求。
export function useRendered(markdown: string, delay = 250) {
  const [state, setState] = useState<{ source: string; result: RenderPreview } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const first = state === null;

  useEffect(() => {
    const controller = new AbortController();
    const timer = setTimeout(
      () => {
        adminApi.render(markdown, controller.signal).then(
          (result) => {
            setState({ source: markdown, result });
            setError(null);
          },
          (err: unknown) => {
            if (!controller.signal.aborted) setError(err instanceof Error ? err.message : String(err));
          },
        );
      },
      first ? 0 : delay,
    );
    return () => {
      controller.abort();
      clearTimeout(timer);
    };
    // first 只决定首次是否延迟，不触发重新渲染。
  }, [markdown]);

  return { result: state?.result, current: state?.source === markdown, error };
}
