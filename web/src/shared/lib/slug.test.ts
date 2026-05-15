import { describe, expect, it } from "vitest";
import { slugError, slugify } from "./slug";

describe("slugify", () => {
  it("keeps latin words and digits", () => {
    expect(slugify("Nginx 配置备忘：TLS、限流与安全头")).toBe("nginx-tls");
    expect(slugify("  Café au Lait -- 2026 ")).toBe("cafe-au-lait-2026");
  });

  it("returns an empty string for purely CJK titles", () => {
    expect(slugify("写时渲染")).toBe("");
  });
});

describe("slugError", () => {
  it("validates against the API pattern", () => {
    expect(slugError("render-on-write")).toBeUndefined();
    expect(slugError("Render")).toBeDefined();
    expect(slugError("a--b")).toBeDefined();
    expect(slugError("")).toBe("请填写链接");
  });
});
