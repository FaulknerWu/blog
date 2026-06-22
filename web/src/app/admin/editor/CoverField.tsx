import { useState } from "react";
import type { Media } from "~/shared/api/admin-types";
import { cn } from "~/shared/lib/cn";
import { ui } from "~/shared/ui/admin/ui";
import { thumbnail, useUploads } from "~/features/media/uploads";
import type { EditorForm } from "./model";

interface Props {
  cover: EditorForm["cover"];
  onChange: (cover: EditorForm["cover"]) => void;
  onBrowse: () => void;
  onUploaded: (media: Media) => void;
}

// CoverField 显示封面缩略图；没有封面时可以从媒体库选择或直接拖入图片。
export function CoverField({ cover, onChange, onBrowse, onUploaded }: Props) {
  const [over, setOver] = useState(false);
  const uploads = useUploads((m) => {
    onUploaded(m);
    onChange({ id: m.id, image: m });
  });
  const failed = uploads.items.find((i) => i.error);
  const busy = uploads.items.some((i) => !i.error);

  if (cover) {
    return (
      <div className="flex flex-col gap-1">
        <img src={thumbnail(cover.image)} alt={cover.image.alt} className="aspect-3/2 w-full bg-line-soft object-cover" />
        <span className="flex items-center">
          <button type="button" className={ui.linkBtn} onClick={onBrowse}>
            更换
          </button>
          <span className={ui.sep}>·</span>
          <button type="button" className={cn(ui.linkBtn, "text-muted hover:text-muted")} onClick={() => onChange(null)}>
            移除
          </button>
        </span>
      </div>
    );
  }

  return (
    <>
      <button
        type="button"
        className={cn(
          "h-24 border border-dashed border-control-line text-sm text-muted hover:border-accent hover:text-accent",
          over && "border-accent text-accent",
        )}
        onClick={onBrowse}
        onDragOver={(e) => {
          if (!e.dataTransfer.types.includes("Files")) return;
          e.preventDefault();
          setOver(true);
        }}
        onDragLeave={() => setOver(false)}
        onDrop={(e) => {
          e.preventDefault();
          setOver(false);
          const file = e.dataTransfer.files[0];
          if (file) void uploads.start([file]);
        }}
      >
        {busy ? "正在上传…" : "从媒体库选择，或拖入图片"}
      </button>
      {failed && <span className={ui.error}>{failed.error}</span>}
    </>
  );
}
