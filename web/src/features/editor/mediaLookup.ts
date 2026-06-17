import { adminApi } from "~/shared/api/admin";
import type { Image } from "~/shared/api/types";
import type { MediaLookup } from "./render/renderer";

const cache = new Map<string, Image>();
const batch = 100;

// rememberMedia 记录刚上传的图片，渲染时无需再查询。
export function rememberMedia(image: Image): void {
  cache.set(image.sha256, image);
}

// lookupMedia 按哈希批量查询图片信息并缓存；查不到的哈希下次渲染会重新查询，以便识别随后上传的图片。
export const lookupMedia: MediaLookup = async (shas) => {
  const missing = shas.filter((s) => !cache.has(s));
  for (let i = 0; i < missing.length; i += batch) {
    const page = await adminApi.mediaBySha(missing.slice(i, i + batch));
    for (const m of page.items) cache.set(m.sha256, m);
  }
  return new Map(shas.flatMap((s) => (cache.has(s) ? [[s, cache.get(s)!] as const] : [])));
};
