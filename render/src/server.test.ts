import type { AddressInfo } from "node:net";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import type { RenderResult } from "./protocol.ts";
import { createServer } from "./server.ts";
import { RENDERER_VERSION } from "./version.ts";

const server = createServer();
let base = "";

beforeAll(async () => {
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  base = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
});

afterAll(async () => {
  await new Promise((resolve) => server.close(resolve));
});

function render(body: unknown): Promise<Response> {
  return fetch(`${base}/render`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: typeof body === "string" ? body : JSON.stringify(body),
  });
}

describe("render server", () => {
  it("renders markdown with the given media", async () => {
    const sha = "b".repeat(64);
    const res = await render({
      markdown: "## 小节\n\n```go\nfmt.Println(1)\n```\n\n![图](/media/" + sha + ")\n",
      media: { [sha]: { sha256: sha, width: 800, height: 600, alt: "", src: `/media/${sha}`, variants: [] } },
    });
    expect(res.status).toBe(200);
    expect(res.headers.get("content-type")).toContain("application/json");
    const body = (await res.json()) as RenderResult;
    expect(body.toc).toEqual([{ depth: 2, id: "小节", text: "小节" }]);
    expect(body.html).toContain('data-lang="go"');
    expect(body.html).toContain('width="800"');
    expect(body.renderer_version).toBe(RENDERER_VERSION);
    expect(body.issues).toEqual([]);
  });

  it("accepts a request without media", async () => {
    const res = await render({ markdown: "hi" });
    expect(res.status).toBe(200);
    expect(((await res.json()) as RenderResult).html).toBe("<p>hi</p>");
  });

  it("reports math errors as issues", async () => {
    const res = await render({ markdown: "$$\n\\frac{a\n$$\n", media: {} });
    expect(res.status).toBe(200);
    const body = (await res.json()) as RenderResult;
    expect(body.issues).toMatchObject([{ line: 2, severity: "error" }]);
  });

  it("rejects malformed requests", async () => {
    for (const body of ["{", { markdown: 1 }, { markdown: "", media: { x: { sha256: "x" } } }]) {
      const res = await render(body);
      expect(res.status).toBe(400);
      expect(res.headers.get("content-type")).toContain("application/problem+json");
      expect(await res.json()).toMatchObject({ status: 400 });
    }
  });

  it("rejects oversized bodies", async () => {
    const res = await render({ markdown: "a".repeat(9 << 20) });
    expect(res.status).toBe(413);
  });

  it("rejects wrong methods and unknown paths", async () => {
    expect((await fetch(`${base}/render`)).status).toBe(405);
    expect((await fetch(`${base}/healthz`, { method: "POST" })).status).toBe(405);
    expect((await fetch(`${base}/nope`)).status).toBe(404);
  });

  it("reports health with the renderer version", async () => {
    const res = await fetch(`${base}/healthz`);
    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ status: "ok", renderer_version: RENDERER_VERSION });
  });
});
