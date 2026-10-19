import { useRef, useState } from "react";
import type { Media } from "~/shared/api/types";
import { cn } from "~/shared/lib/cn";
import { button, Dialog, ui } from "~/shared/ui/admin/ui";
import { MediaGrid, UploadList } from "./MediaGrid";
import { acceptedTypes, useUploads } from "./uploads";

interface Props {
  open: boolean;
  title: string;
  confirmLabel: string;
  items: Media[] | undefined;
  onClose: () => void;
  onPick: (media: Media) => void;
  onUploaded: (media: Media) => void;
}

// MediaPicker 从媒体库选择一张图片，也可以当场上传；上传完成的图片自动选中。
export function MediaPicker({ open, title, confirmLabel, items, onClose, onPick, onUploaded }: Props) {
  const [selected, setSelected] = useState<Media | null>(null);
  const input = useRef<HTMLInputElement>(null);
  const uploads = useUploads((m) => {
    onUploaded(m);
    setSelected(m);
  });

  const list = items && selected && !items.some((m) => m.id === selected.id) ? [selected, ...items] : (items ?? []);

  function pick(m: Media) {
    onPick(m);
    setSelected(null);
  }

  return (
    <Dialog
      open={open}
      wide
      title={title}
      onClose={onClose}
      actions={
        <>
          <button type="button" className={cn(ui.linkBtn, "mr-auto")} onClick={() => input.current?.click()}>
            上传新图片
          </button>
          <button type="button" className={button()} onClick={onClose}>
            取消
          </button>
          <button
            type="button"
            className={button({ variant: "primary" })}
            disabled={!selected}
            onClick={() => selected && pick(selected)}
          >
            {confirmLabel}
          </button>
        </>
      }
    >
      <input
        ref={input}
        type="file"
        accept={acceptedTypes.join(",")}
        multiple
        hidden
        onChange={(e) => {
          if (e.target.files) void uploads.start(e.target.files);
          e.target.value = "";
        }}
      />
      <UploadList items={uploads.items} onDismiss={uploads.dismiss} />
      <div className="-m-1 max-h-[min(56vh,560px)] overflow-y-auto p-1">
        {items === undefined ? (
          <p className={ui.empty}>正在加载…</p>
        ) : list.length === 0 ? (
          <p className={ui.empty}>媒体库还是空的。</p>
        ) : (
          <MediaGrid items={list} selected={selected?.id} onSelect={setSelected} onPick={pick} />
        )}
      </div>
    </Dialog>
  );
}
