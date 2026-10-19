import { useMutation, useQuery } from "@tanstack/react-query";
import { useId, useState, type KeyboardEvent } from "react";
import { adminApi, ApiError } from "~/shared/api/admin";
import type { AdminTag } from "~/shared/api/types";
import { slugError, slugify } from "~/shared/lib/slug";
import { cn } from "~/shared/lib/cn";
import { ui } from "~/shared/ui/admin/ui";
import { q, queryClient } from "~/app/admin/queries";

const option =
  "flex cursor-pointer justify-between gap-3 px-2.5 py-1.5 text-md aria-selected:bg-accent/10 aria-selected:text-accent";

const maxTags = 10;

// TagInput 选择已有标签或当场新建；新建的中文标签需要补一个英文链接。
export function TagInput({ value, onChange }: { value: number[]; onChange: (ids: number[]) => void }) {
  const tags = useQuery(q.tags).data ?? [];
  const [text, setText] = useState("");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const [pendingSlug, setPendingSlug] = useState<string | null>(null);
  const listId = useId();

  const byId = new Map(tags.map((t) => [t.id, t]));
  const needle = text.trim().toLowerCase();
  const suggestions = needle
    ? tags.filter((t) => !value.includes(t.id) && (t.name.toLowerCase().includes(needle) || t.slug.includes(needle))).slice(0, 8)
    : [];
  const exact = tags.find((t) => t.name.toLowerCase() === needle);
  const canCreate = needle.length > 0 && !exact;
  const options = suggestions.length + (canCreate ? 1 : 0);

  const create = useMutation({
    mutationFn: ({ name, slug }: { name: string; slug: string }) => adminApi.createTag(name, slug),
    onSuccess: (tag) => {
      queryClient.setQueryData(q.tags.queryKey, (old) => [...(old ?? []), tag]);
      add(tag);
    },
  });

  function add(tag: AdminTag) {
    if (!value.includes(tag.id) && value.length < maxTags) onChange([...value, tag.id]);
    setText("");
    setPendingSlug(null);
    setActive(0);
    create.reset();
  }

  function choose(index: number) {
    const tag = suggestions[index];
    if (tag) return add(tag);
    if (exact) return add(exact);
    if (!canCreate) return;
    const name = text.trim();
    const slug = slugify(name);
    if (slug) create.mutate({ name, slug });
    else setPendingSlug("");
  }

  function onKey(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "ArrowDown" && options) {
      e.preventDefault();
      setOpen(true);
      setActive((a) => (a + 1) % options);
    } else if (e.key === "ArrowUp" && options) {
      e.preventDefault();
      setActive((a) => (a - 1 + options) % options);
    } else if (e.key === "Enter") {
      e.preventDefault();
      choose(active);
    } else if (e.key === "Escape") {
      setOpen(false);
    } else if (e.key === "Backspace" && !text && value.length) {
      onChange(value.slice(0, -1));
    }
  }

  const createError =
    create.error instanceof ApiError && create.error.status === 409 ? "标签名称或链接已存在" : create.error?.message;

  return (
    <div className="flex flex-col gap-2">
      {value.length > 0 && (
        <div className="flex flex-wrap items-center gap-x-3.5 gap-y-1">
          {value.map((id) => {
            const tag = byId.get(id);
            return (
              <span key={id} className="inline-flex items-center gap-0.5 text-md">
                #{tag?.name ?? id}
                <button
                  type="button"
                  className="inline-flex size-6 items-center justify-center text-muted hover:text-danger"
                  aria-label={`移除标签 ${tag?.name ?? id}`} onClick={() => onChange(value.filter((x) => x !== id))}>
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" aria-hidden="true">
                    <path d="M6 6l12 12M18 6L6 18" />
                  </svg>
                </button>
              </span>
            );
          })}
        </div>
      )}
      {pendingSlug === null ? (
        <div className="relative">
          <input
            className={ui.input}
            role="combobox"
            aria-label="添加标签"
            aria-expanded={open && options > 0}
            aria-controls={listId}
            aria-activedescendant={open && options ? `${listId}-${active}` : undefined}
            placeholder={value.length >= maxTags ? `最多 ${maxTags} 个标签` : "搜索或新建标签…"}
            disabled={value.length >= maxTags}
            value={text}
            onChange={(e) => {
              setText(e.target.value);
              setOpen(true);
              setActive(0);
            }}
            onFocus={() => setOpen(true)}
            onBlur={() => setTimeout(() => setOpen(false), 120)}
            onKeyDown={onKey}
          />
          {open && options > 0 && (
            <ul id={listId} role="listbox" className="absolute inset-x-0 top-[calc(100%+2px)] z-5 rounded-xs border border-control-line bg-paper py-1 shadow-popover">
              {suggestions.map((t, i) => (
                <li
                  key={t.id}
                  id={`${listId}-${i}`}
                  role="option"
                  className={option}
                  aria-selected={active === i}
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => choose(i)}
                >
                  #{t.name}
                  <span className="text-xs text-muted">{t.post_count} 篇</span>
                </li>
              ))}
              {canCreate && (
                <li
                  id={`${listId}-${suggestions.length}`}
                  role="option"
                  className={option}
                  aria-selected={active === suggestions.length}
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => choose(suggestions.length)}
                >
                  新建标签「{text.trim()}」
                </li>
              )}
            </ul>
          )}
        </div>
      ) : (
        <div className="flex flex-col gap-1.5">
          <span className={ui.hint}>「{text.trim()}」的链接</span>
          <input
            className={cn(ui.input, "font-mono text-xs")}
            aria-label="新标签的链接"
            placeholder="english-slug"
            autoFocus
            value={pendingSlug}
            onChange={(e) => setPendingSlug(e.target.value.trim())}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                if (!slugError(pendingSlug)) create.mutate({ name: text.trim(), slug: pendingSlug });
              } else if (e.key === "Escape") {
                setPendingSlug(null);
              }
            }}
          />
          <span className="flex gap-4">
            <button type="button" className={cn(ui.linkBtn, "text-muted hover:text-muted")} onClick={() => setPendingSlug(null)}>
              取消
            </button>
            <button
              type="button"
              className={ui.linkBtn}
              disabled={!!slugError(pendingSlug) || create.isPending}
              onClick={() => create.mutate({ name: text.trim(), slug: pendingSlug })}
            >
              创建
            </button>
          </span>
        </div>
      )}
      {createError && <span className={ui.error}>{createError}</span>}
    </div>
  );
}
