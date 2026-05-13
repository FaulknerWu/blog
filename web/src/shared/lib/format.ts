export interface Locale {
  language: string;
  timezone: string;
}

const formatters = new Map<string, Intl.DateTimeFormat>();

function dateFormat(locale: Locale, opts: Intl.DateTimeFormatOptions): Intl.DateTimeFormat {
  const key = `${locale.language}|${locale.timezone}|${JSON.stringify(opts)}`;
  let f = formatters.get(key);
  if (!f) {
    f = new Intl.DateTimeFormat(locale.language, { timeZone: locale.timezone, ...opts });
    formatters.set(key, f);
  }
  return f;
}

// 2026年10月1日
export function formatDate(iso: string, locale: Locale): string {
  return dateFormat(locale, { year: "numeric", month: "long", day: "numeric" }).format(new Date(iso));
}

// 10月1日，用于已按年份分组的列表
export function formatMonthDay(iso: string, locale: Locale): string {
  return dateFormat(locale, { month: "long", day: "numeric" }).format(new Date(iso));
}

export function yearOf(iso: string, locale: Locale): number {
  const part = dateFormat(locale, { year: "numeric" })
    .formatToParts(new Date(iso))
    .find((p) => p.type === "year");
  return Number(part?.value);
}

// groupByYear 按站点时区的发布年份分组，保持输入顺序（列表已按发布时间倒序）。
export function groupByYear<T extends { published_at: string }>(items: T[], locale: Locale): { year: number; items: T[] }[] {
  const groups: { year: number; items: T[] }[] = [];
  for (const item of items) {
    const year = yearOf(item.published_at, locale);
    const last = groups.at(-1);
    if (last?.year === year) last.items.push(item);
    else groups.push({ year, items: [item] });
  }
  return groups;
}

export function formatNumber(n: number, locale: Locale): string {
  return new Intl.NumberFormat(locale.language).format(n);
}

// 中文正文按每分钟约 400 字估算。
export function readingMinutes(wordCount: number): number {
  return Math.max(1, Math.round(wordCount / 400));
}

export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}
