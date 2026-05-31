import { cva, type VariantProps } from "class-variance-authority";
import { useEffect, useId, useRef, type ReactNode } from "react";
import { cn } from "~/shared/lib/cn";

// 后台通用控件。沿用公开站的纸本风格：细线分隔、直角控件、衬线标题，不使用卡片容器。
// 单个元素的样式以类名配方给出，可以用在任意元素上（<button>、<Link>、<label> 都行），
// 需要局部调整时用 cn(ui.input, "w-50") 覆盖。

export const button = cva(
  "inline-flex min-h-9.5 items-center justify-center gap-2 whitespace-nowrap rounded-xs border px-4 text-md leading-tight disabled:cursor-not-allowed disabled:opacity-45",
  {
    variants: {
      variant: {
        default: "border-control-line text-ink hover:not-disabled:border-ink hover:text-ink",
        primary:
          "border-accent bg-accent text-on-accent hover:not-disabled:border-accent-strong hover:not-disabled:bg-accent-strong hover:text-on-accent",
        danger: "border-danger bg-danger text-on-accent hover:not-disabled:brightness-92 hover:text-on-accent",
      },
      size: {
        md: "",
        sm: "min-h-8 px-3 text-sm",
      },
    },
    defaultVariants: { variant: "default", size: "md" },
  },
);

export type ButtonVariants = VariantProps<typeof button>;

const input =
  "min-h-9.5 w-full min-w-0 rounded-xs border border-control-line bg-control px-2.5 text-md text-ink focus-visible:-outline-offset-1 aria-invalid:border-danger";

export const ui = {
  h1: "font-serif text-5xl font-bold",
  h2: "font-serif text-2xl font-semibold *:[small]:font-sans *:[small]:text-md *:[small]:font-normal *:[small]:text-muted",
  pageHead: "flex flex-wrap items-end justify-between gap-4",
  secHead: "flex flex-wrap items-baseline justify-between gap-x-3 gap-y-2 border-b border-line pb-2.5",

  // 文字按钮：没有边框，强调色文字，悬停加下划线。
  linkBtn:
    "inline-flex cursor-pointer items-center gap-1.5 py-2 text-md text-accent underline-offset-3 hover:not-disabled:underline disabled:cursor-not-allowed disabled:opacity-45",
  link: "text-accent underline-offset-3 hover:text-accent-strong hover:underline",
  titleLink: "font-serif text-lg font-semibold leading-normal text-ink hover:text-accent",

  input,
  textarea: cn(input, "resize-y py-2 leading-normal"),
  checkbox: "m-0 size-4 accent-accent",

  field: "flex min-w-0 flex-col gap-1.5",
  label: "text-xs font-medium tracking-wide text-muted",
  hint: "text-xs text-muted",
  error: "text-xs text-danger",
  notice: "text-sm leading-relaxed",
  empty: "py-8 text-md text-muted",
  sep: "px-2 text-control-line",
  mono: "font-mono text-xs",

  tabs: "flex flex-wrap gap-6",
  tab: "inline-flex min-h-9.5 items-center gap-1.5 text-md text-muted hover:text-ink aria-pressed:font-semibold aria-pressed:text-ink aria-selected:font-semibold aria-selected:text-ink *:[small]:text-xs *:[small]:font-normal *:[small]:text-muted",

  tableWrap: "overflow-x-auto",
  // 表格首末列贴齐版心两侧，末列右对齐（通常放操作）。
  // 单元格规则包在 :where() 里，优先级低于直接写在 <td> 上的工具类，调用处可以覆盖。
  table: cn(
    "w-full border-collapse",
    "[:where(&)_th]:border-b [:where(&)_th]:border-line [:where(&)_th]:px-3 [:where(&)_th]:py-2.5 [:where(&)_th]:text-left [:where(&)_th]:text-xs [:where(&)_th]:font-medium [:where(&)_th]:whitespace-nowrap [:where(&)_th]:text-muted",
    "[:where(&)_td]:border-b [:where(&)_td]:border-line-soft [:where(&)_td]:px-3 [:where(&)_td]:py-4 [:where(&)_td]:align-baseline [:where(&)_td]:text-md",
    "[:where(&)_:is(th,td):first-child]:pl-0 [:where(&)_:is(th,td):last-child]:pr-0 [:where(&)_:is(th,td):last-child]:text-right",
  ),
} as const;

const statusColor = {
  draft: "text-muted",
  scheduled: "text-warn",
  published: "text-ok",
  warn: "text-danger",
};

// 状态：彩色文字加小圆点，不用胶囊徽章。
export function Status({ kind, children }: { kind: keyof typeof statusColor; children: ReactNode }) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.75 text-sm whitespace-nowrap before:size-1.5 before:rounded-full before:bg-current",
        statusColor[kind],
      )}
    >
      {children}
    </span>
  );
}

interface DialogProps {
  open: boolean;
  title: ReactNode;
  // 标题下的一行补充说明，仅在需要提示后果时使用。
  lead?: ReactNode;
  onClose: () => void;
  children?: ReactNode;
  actions: ReactNode;
  wide?: boolean;
}

// Dialog 基于原生 <dialog> 的模态框，自带焦点约束、Esc 关闭与背景遮罩。
export function Dialog({ open, title, lead, onClose, children, actions, wide }: DialogProps) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (open && !el.open) el.showModal();
    if (!open && el.open) el.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      className={cn(
        "mx-auto mt-[clamp(16px,9vh,104px)] max-h-[calc(100vh-32px)] w-[calc(100%-32px)] max-w-135 rounded-xs border border-control-line bg-paper font-sans text-ink shadow-dialog",
        wide && "max-w-220",
      )}
      aria-labelledby={titleId}
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
    >
      {open && (
        <div className="flex flex-col gap-5 px-5 py-6 sm:px-9 sm:py-8">
          <header className="flex flex-col gap-1.5 border-b border-line pb-4">
            <h2 id={titleId} className="font-serif text-4xl font-bold">
              {title}
            </h2>
            {lead && <p className="text-md leading-relaxed text-muted">{lead}</p>}
          </header>
          {children}
          <footer className="flex flex-wrap items-center justify-end gap-x-4 gap-y-2 border-t border-line pt-4">{actions}</footer>
        </div>
      )}
    </dialog>
  );
}

interface ConfirmProps {
  open: boolean;
  title: ReactNode;
  lead?: ReactNode;
  confirmLabel: string;
  danger?: boolean;
  busy?: boolean;
  error?: string | null;
  onConfirm: () => void;
  onClose: () => void;
}

export function ConfirmDialog({ open, title, lead, confirmLabel, danger, busy, error, onConfirm, onClose }: ConfirmProps) {
  return (
    <Dialog
      open={open}
      title={title}
      lead={lead}
      onClose={onClose}
      actions={
        <>
          <button type="button" className={button()} onClick={onClose}>
            取消
          </button>
          <button
            type="button"
            className={button({ variant: danger ? "danger" : "primary" })}
            disabled={busy}
            onClick={onConfirm}
          >
            {confirmLabel}
          </button>
        </>
      }
    >
      {error && (
        <p role="alert" className={cn(ui.notice, "text-danger")}>
          {error}
        </p>
      )}
    </Dialog>
  );
}
