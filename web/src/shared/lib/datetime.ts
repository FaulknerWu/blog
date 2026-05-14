import type { Locale } from "./format";

interface Parts {
  year: number;
  month: number;
  day: number;
  hour: number;
  minute: number;
}

const partFormatters = new Map<string, Intl.DateTimeFormat>();

// zonedParts 返回某一时刻在指定时区的年月日时分。
export function zonedParts(date: Date, timeZone: string): Parts {
  let f = partFormatters.get(timeZone);
  if (!f) {
    f = new Intl.DateTimeFormat("en-US", {
      timeZone,
      hourCycle: "h23",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    });
    partFormatters.set(timeZone, f);
  }
  const get = (type: string) => Number(f.formatToParts(date).find((p) => p.type === type)?.value);
  return { year: get("year"), month: get("month"), day: get("day"), hour: get("hour"), minute: get("minute") };
}

const pad = (n: number) => String(n).padStart(2, "0");

// toZonedInput 把时刻转换为 <input type="date"> 与 <input type="time"> 使用的站点时区本地值。
export function toZonedInput(date: Date, timeZone: string): { date: string; time: string } {
  const p = zonedParts(date, timeZone);
  return { date: `${p.year}-${pad(p.month)}-${pad(p.day)}`, time: `${pad(p.hour)}:${pad(p.minute)}` };
}

// fromZonedInput 把站点时区的本地日期时间转换为 UTC 时刻。夏令时跳过的时间按偏移前的规则顺延。
export function fromZonedInput(date: string, time: string, timeZone: string): Date {
  const [y, mo, d] = date.split("-").map(Number);
  const [h, mi] = time.split(":").map(Number);
  const wall = Date.UTC(y!, mo! - 1, d!, h!, mi!);
  let guess = wall;
  for (let i = 0; i < 2; i++) {
    const p = zonedParts(new Date(guess), timeZone);
    const offset = Date.UTC(p.year, p.month - 1, p.day, p.hour, p.minute) - guess;
    guess = wall - offset;
  }
  return new Date(guess);
}

function dayNumber(p: Parts): number {
  return Date.UTC(p.year, p.month - 1, p.day) / 86_400_000;
}

// formatWhen 输出后台列表使用的紧凑时间：今天 13:40、昨天 22:15、10-12 10:05、2025-10-12。
export function formatWhen(iso: string, locale: Locale, now = new Date()): string {
  const p = zonedParts(new Date(iso), locale.timezone);
  const today = zonedParts(now, locale.timezone);
  const time = `${pad(p.hour)}:${pad(p.minute)}`;
  const diff = dayNumber(today) - dayNumber(p);
  if (diff === 0) return `今天 ${time}`;
  if (diff === 1) return `昨天 ${time}`;
  if (diff === -1) return `明天 ${time}`;
  if (p.year === today.year) return `${pad(p.month)}-${pad(p.day)} ${time}`;
  return `${p.year}-${pad(p.month)}-${pad(p.day)}`;
}

// formatDateTime 输出完整的站点时区时间：2026-10-12 10:00。
export function formatDateTime(iso: string, locale: Locale): string {
  const p = zonedParts(new Date(iso), locale.timezone);
  return `${p.year}-${pad(p.month)}-${pad(p.day)} ${pad(p.hour)}:${pad(p.minute)}`;
}

// formatShort 输出「10 月 22 日 09:00」，用于句子中。
export function formatShort(iso: string, locale: Locale): string {
  const p = zonedParts(new Date(iso), locale.timezone);
  return `${p.month} 月 ${p.day} 日 ${pad(p.hour)}:${pad(p.minute)}`;
}
