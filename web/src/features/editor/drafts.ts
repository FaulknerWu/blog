// 本地暂存：写作页把未保存的内容写进 localStorage，刷新、断网、会话过期或保存冲突时都不会丢失。
// 存储不可用（隐私模式、配额已满）时静默降级，只是失去暂存能力。

export interface LocalDraft<T> {
  form: T;
  // 暂存时所基于的服务器版本，新建内容为 null。
  baseVersion: number | null;
  savedAt: string;
}

const prefix = "blog-admin:draft:";

export function loadDraft<T>(key: string): LocalDraft<T> | null {
  try {
    const raw = localStorage.getItem(prefix + key);
    return raw ? (JSON.parse(raw) as LocalDraft<T>) : null;
  } catch {
    return null;
  }
}

export function saveDraft<T>(key: string, draft: LocalDraft<T>): boolean {
  try {
    localStorage.setItem(prefix + key, JSON.stringify(draft));
    return true;
  } catch {
    return false;
  }
}

export function clearDraft(key: string): void {
  try {
    localStorage.removeItem(prefix + key);
  } catch {
    // 存储不可用时没有可清理的内容。
  }
}
