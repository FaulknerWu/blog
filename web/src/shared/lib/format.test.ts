import { describe, expect, it } from "vitest";
import { formatDate, groupByYear, readingMinutes, yearOf } from "./format";

const shanghai = { language: "zh-CN", timezone: "Asia/Shanghai" };

describe("yearOf", () => {
  it("uses the site timezone, not UTC", () => {
    // UTC 时间仍是 2025 年最后一天，上海已是 2026 年。
    expect(yearOf("2025-12-31T16:30:00.000000Z", shanghai)).toBe(2026);
    expect(yearOf("2025-12-31T16:30:00.000000Z", { language: "zh-CN", timezone: "UTC" })).toBe(2025);
  });
});

describe("groupByYear", () => {
  it("groups consecutive posts by year in order", () => {
    const posts = [
      { slug: "a", published_at: "2026-10-12T02:00:00Z" },
      { slug: "b", published_at: "2026-01-05T02:00:00Z" },
      { slug: "c", published_at: "2025-12-20T02:00:00Z" },
    ];
    expect(groupByYear(posts, shanghai)).toEqual([
      { year: 2026, items: [posts[0], posts[1]] },
      { year: 2025, items: [posts[2]] },
    ]);
  });

  it("returns no groups for an empty list", () => {
    expect(groupByYear([], shanghai)).toEqual([]);
  });
});

describe("formatDate", () => {
  it("formats in the site language and timezone", () => {
    expect(formatDate("2026-09-30T17:00:00Z", shanghai)).toBe("2026年10月1日");
  });
});

describe("readingMinutes", () => {
  it("rounds to whole minutes with a floor of one", () => {
    expect(readingMinutes(3820)).toBe(10);
    expect(readingMinutes(50)).toBe(1);
  });
});
