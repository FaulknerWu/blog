import { useEffect, useMemo, useState } from "react";
import type { TocItem } from "~/shared/api/types";
import { cn } from "~/shared/lib/cn";

// 目录只展示二、三级标题，更深的层级在边栏里过于拥挤。
const maxDepth = 3;

// 可展示的标题不足两个时目录没有意义。
export function hasToc(items: TocItem[]): boolean {
  return items.filter((i) => i.depth <= maxDepth).length >= 2;
}

export function Toc({ items }: { items: TocItem[] }) {
  const visible = useMemo(() => items.filter((i) => i.depth <= maxDepth), [items]);
  const active = useActiveHeading(visible);

  return (
    <nav aria-label="目录" className="text-base leading-normal">
      <div className="mb-3.5 text-sm tracking-widest text-muted">目录</div>
      <ol>
        {visible.map((item) => {
          return (
            <li key={item.id}>
              <a
                href={`#${encodeURIComponent(item.id)}`}
                className={cn(
                  "block border-l border-line py-1.5 pl-3 text-muted",
                  "aria-[current=location]:border-l-2 aria-[current=location]:border-accent aria-[current=location]:pl-2.75 aria-[current=location]:text-accent",
                  item.depth === 3 && "pl-6.5 aria-[current=location]:pl-6.25",
                )}
                aria-current={item.id === active ? "location" : undefined}
              >
                {item.text}
              </a>
            </li>
          );
        })}
      </ol>
    </nav>
  );
}

// useActiveHeading 跟踪视口上方最近经过的标题；仅在浏览器中运行，SSR 输出时没有高亮。
function useActiveHeading(items: TocItem[]): string | undefined {
  const [active, setActive] = useState<string>();

  useEffect(() => {
    const headings = items
      .map((i) => document.getElementById(i.id))
      .filter((el): el is HTMLElement => el !== null);
    if (headings.length === 0) return;

    const update = () => {
      let current: string | undefined;
      for (const h of headings) {
        if (h.getBoundingClientRect().top > 80) break;
        current = h.id;
      }
      setActive(current);
    };
    update();
    window.addEventListener("scroll", update, { passive: true });
    return () => window.removeEventListener("scroll", update);
  }, [items]);

  return active;
}
