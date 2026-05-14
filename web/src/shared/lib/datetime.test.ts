import { describe, expect, it } from "vitest";
import { formatWhen, fromZonedInput, toZonedInput } from "./datetime";
import { formatBytes } from "./format";

const shanghai = { language: "zh-CN", timezone: "Asia/Shanghai" };

describe("zoned input conversion", () => {
  it("round-trips a site-local time through UTC", () => {
    const at = fromZonedInput("2026-10-22", "09:00", "Asia/Shanghai");
    expect(at.toISOString()).toBe("2026-10-22T01:00:00.000Z");
    expect(toZonedInput(at, "Asia/Shanghai")).toEqual({ date: "2026-10-22", time: "09:00" });
  });

  it("handles daylight saving offsets", () => {
    expect(fromZonedInput("2026-07-01", "12:00", "America/New_York").toISOString()).toBe("2026-07-01T16:00:00.000Z");
    expect(fromZonedInput("2026-12-01", "12:00", "America/New_York").toISOString()).toBe("2026-12-01T17:00:00.000Z");
  });
});

describe("formatWhen", () => {
  const now = new Date("2026-10-19T06:00:00Z"); // 上海 14:00

  it("uses relative day names near today in the site timezone", () => {
    expect(formatWhen("2026-10-19T05:40:00Z", shanghai, now)).toBe("今天 13:40");
    // UTC 前一天 16:30，上海已是 10 月 19 日 00:30。
    expect(formatWhen("2026-10-18T16:30:00Z", shanghai, now)).toBe("今天 00:30");
    expect(formatWhen("2026-10-18T14:15:00Z", shanghai, now)).toBe("昨天 22:15");
  });

  it("falls back to month-day or full date", () => {
    expect(formatWhen("2026-10-12T02:05:00Z", shanghai, now)).toBe("10-12 10:05");
    expect(formatWhen("2025-10-12T02:05:00Z", shanghai, now)).toBe("2025-10-12");
  });
});

describe("formatBytes", () => {
  it("picks a readable unit", () => {
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(412 * 1024)).toBe("412 KB");
    expect(formatBytes(2481920)).toBe("2.4 MB");
  });
});
