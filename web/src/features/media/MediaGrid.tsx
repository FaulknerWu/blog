import type { Media } from "~/shared/api/types";
import { formatBytes } from "~/shared/lib/format";
import { cn } from "~/shared/lib/cn";
import { thumbnail, type UploadItem } from "./uploads";

interface GridProps {
  items: Media[];
  selected?: number;
  onSelect: (media: Media) => void;
  // 双击或回车时触发，用于选择器里直接确认。
  onPick?: (media: Media) => void;
}

export function MediaGrid({ items, selected, onSelect, onPick }: GridProps) {
  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(170px,1fr))] gap-x-5 gap-y-7">
      {items.map((m) => (
        <button
          key={m.id}
          type="button"
          className="group flex min-w-0 flex-col gap-2 text-left text-ink"
          aria-pressed={selected === m.id}
          onClick={() => onSelect(m)}
          onDoubleClick={() => onPick?.(m)}
        >
          <span className="block aspect-3/2 bg-line-soft outline outline-offset-3 outline-transparent group-hover:outline-control-line group-aria-pressed:outline-2 group-aria-pressed:outline-accent">
            <img src={thumbnail(m)} alt="" loading="lazy" decoding="async" className="size-full object-cover" />
          </span>
          <span className="flex min-w-0 flex-col gap-0.5">
            <span className="truncate text-sm">{m.original_filename}</span>
            <span className={cn("text-xs text-muted", m.reference_count === 0 && "text-warn")}>
              {m.reference_count ? `被 ${m.reference_count} 处引用` : "未引用"}
            </span>
          </span>
        </button>
      ))}
    </div>
  );
}

export function UploadList({ items, onDismiss }: { items: UploadItem[]; onDismiss: (key: string) => void }) {
  if (items.length === 0) return null;
  return (
    <ul className="flex flex-col gap-3" aria-live="polite">
      {items.map((u) => (
        <li key={u.key} className="flex flex-col gap-2 text-md">
          <span>
            {u.name} <span className="text-muted">· {formatBytes(u.size)}</span>
            {u.error ? (
              <span className="text-danger"> · {u.error}</span>
            ) : (
              <span className="text-muted">
                {" "}
                · {u.progress < 1 ? `上传中 ${Math.round(u.progress * 100)}%` : "正在去除 EXIF 并生成多尺寸版本…"}
              </span>
            )}
          </span>
          {u.error ? (
            <button type="button" className="self-start text-sm text-accent" onClick={() => onDismiss(u.key)}>
              关闭
            </button>
          ) : (
            <span className="block h-0.5 bg-line">
              <span
                className="block h-0.5 bg-accent transition-[width] duration-200"
                style={{ width: `${Math.round(u.progress * 100)}%` }}
              />
            </span>
          )}
        </li>
      ))}
    </ul>
  );
}
