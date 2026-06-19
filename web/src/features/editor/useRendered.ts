import { useEffect, useState } from "react";
import { renderMarkdown, type MediaLookup, type RenderResult } from "./render/renderer";

// useRendered 在输入停顿后重新渲染正文；首次渲染不等待。
export function useRendered(markdown: string, lookup: MediaLookup, delay = 250) {
  const [state, setState] = useState<{ source: string; result: RenderResult } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const first = state === null;

  useEffect(() => {
    let cancelled = false;
    const timer = setTimeout(
      () => {
        renderMarkdown(markdown, lookup).then(
          (result) => {
            if (cancelled) return;
            setState({ source: markdown, result });
            setError(null);
          },
          (err: unknown) => {
            if (!cancelled) setError(err instanceof Error ? err.message : String(err));
          },
        );
      },
      first ? 0 : delay,
    );
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
    // lookup 视为稳定；first 只决定首次是否延迟，都不触发重新渲染。
  }, [markdown]);

  return { result: state?.result, current: state?.source === markdown, error };
}
