import { describe, expect, it } from "vitest";
import { highlight } from "./highlight";

describe("highlight", () => {
  it("marks every case-insensitive occurrence", () => {
    expect(highlight("用 PG_TRGM 做检索，pg_trgm 很快", "pg_trgm")).toEqual([
      { text: "用 ", match: false },
      { text: "PG_TRGM", match: true },
      { text: " 做检索，", match: false },
      { text: "pg_trgm", match: true },
      { text: " 很快", match: false },
    ]);
  });

  it("handles matches at both ends", () => {
    expect(highlight("检索与检索", "检索")).toEqual([
      { text: "检索", match: true },
      { text: "与", match: false },
      { text: "检索", match: true },
    ]);
  });

  it("returns the text untouched for a blank query or no match", () => {
    expect(highlight("中文检索", "  ")).toEqual([{ text: "中文检索", match: false }]);
    expect(highlight("中文检索", "备份")).toEqual([{ text: "中文检索", match: false }]);
  });
});
