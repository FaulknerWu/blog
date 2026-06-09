import { useCallback, useState } from "react";
import { upload, ApiError } from "~/shared/api/admin";
import type { Media } from "~/shared/api/admin-types";

export const acceptedTypes = ["image/jpeg", "image/png", "image/webp"];
export const maxUploadBytes = 20 << 20;

export interface UploadItem {
  key: string;
  name: string;
  size: number;
  progress: number;
  error?: string;
}

// checkFile 在上传前做与服务端一致的格式与大小检查，省去一次注定失败的请求。
export function checkFile(file: File): string | undefined {
  if (!acceptedTypes.includes(file.type)) return "只支持 JPEG、PNG 与 WebP";
  if (file.size > maxUploadBytes) return "单张不能超过 20 MB";
  return undefined;
}

function uploadError(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 413) return "文件超过 20 MB";
    if (err.status === 415) return "不支持的图片格式";
    if (err.status === 0) return "网络中断";
    return err.problem?.errors?.[0]?.detail ?? err.message;
  }
  return "上传失败";
}

// useUploads 管理一组并发上传及其进度；失败项保留在列表中直到被关闭。
export function useUploads(onUploaded: (media: Media) => void) {
  const [items, setItems] = useState<UploadItem[]>([]);

  const patch = (key: string, p: Partial<UploadItem>) =>
    setItems((list) => list.map((i) => (i.key === key ? { ...i, ...p } : i)));

  const start = useCallback(
    async (files: Iterable<File>): Promise<Media[]> => {
      const jobs = [...files].map(async (file) => {
        const key = `${file.name}-${file.size}-${Math.random().toString(36).slice(2)}`;
        const problem = checkFile(file);
        setItems((list) => [...list, { key, name: file.name, size: file.size, progress: 0, error: problem }]);
        if (problem) return null;
        try {
          const media = await upload(file, (progress) => patch(key, { progress }));
          setItems((list) => list.filter((i) => i.key !== key));
          onUploaded(media);
          return media;
        } catch (err) {
          patch(key, { error: uploadError(err) });
          return null;
        }
      });
      return (await Promise.all(jobs)).filter((m): m is Media => m !== null);
    },
    [onUploaded],
  );

  const dismiss = (key: string) => setItems((list) => list.filter((i) => i.key !== key));

  return { items, start, dismiss };
}

// thumbnail 选取不小于目标宽度的最小衍生图，没有衍生图时用原图默认规格。
export function thumbnail(media: { src: string; variants: { width: number; url: string }[] }, width = 480): string {
  const fit = media.variants.filter((v) => v.width >= width).sort((a, b) => a.width - b.width)[0];
  return fit?.url ?? media.variants.at(-1)?.url ?? media.src;
}

export function markdownRef(media: { sha256: string; alt: string }): string {
  return `![${media.alt.replace(/[[\]]/g, "")}](/media/${media.sha256})`;
}
