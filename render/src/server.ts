import { createServer as createHttpServer, type IncomingMessage, type Server, type ServerResponse } from "node:http";
import { parseRequest } from "./protocol.ts";
import { renderMarkdown } from "./renderer.ts";
import { RENDERER_VERSION } from "./version.ts";

// 与 API 保存正文的请求体上限一致。
export const maxBody = 8 << 20;

class HttpError extends Error {
  readonly status: number;
  readonly title: string;
  constructor(status: number, title: string, detail: string) {
    super(detail);
    this.status = status;
    this.title = title;
  }
}

export function createServer(): Server {
  return createHttpServer((req, res) => {
    handle(req, res).catch((err: unknown) => {
      if (err instanceof HttpError) {
        sendProblem(res, err.status, err.title, err.message);
        return;
      }
      console.error("render failed:", err);
      sendProblem(res, 500, "渲染服务内部错误", err instanceof Error ? err.message : String(err));
    });
  });
}

async function handle(req: IncomingMessage, res: ServerResponse): Promise<void> {
  const path = new URL(req.url ?? "/", "http://render").pathname;
  switch (path) {
    case "/healthz":
      allow(req, "GET", "HEAD");
      return sendJSON(res, 200, { status: "ok", renderer_version: RENDERER_VERSION });
    case "/render": {
      allow(req, "POST");
      let body: unknown;
      try {
        body = JSON.parse(await readBody(req));
      } catch (err) {
        if (err instanceof HttpError) throw err;
        throw new HttpError(400, "请求格式错误", "请求体不是合法的 JSON");
      }
      let input;
      try {
        input = parseRequest(body);
      } catch (err) {
        throw new HttpError(400, "请求格式错误", (err as Error).message);
      }
      return sendJSON(res, 200, await renderMarkdown(input.markdown, new Map(Object.entries(input.media))));
    }
    default:
      throw new HttpError(404, "资源不存在", `未知路径 ${path}`);
  }
}

function allow(req: IncomingMessage, ...methods: string[]): void {
  if (!methods.includes(req.method ?? "")) {
    throw new HttpError(405, "请求方法不受支持", `仅支持 ${methods.join("、")}`);
  }
}

async function readBody(req: IncomingMessage): Promise<string> {
  if (Number(req.headers["content-length"] ?? 0) > maxBody) throw tooLarge();
  const chunks: Buffer[] = [];
  let size = 0;
  for await (const chunk of req as AsyncIterable<Buffer>) {
    size += chunk.length;
    if (size > maxBody) throw tooLarge();
    chunks.push(chunk);
  }
  return Buffer.concat(chunks).toString("utf8");
}

function tooLarge(): HttpError {
  return new HttpError(413, "请求体过大", `请求体不能超过 ${maxBody} 字节`);
}

function sendJSON(res: ServerResponse, status: number, body: unknown): void {
  res.writeHead(status, { "Content-Type": "application/json; charset=utf-8" });
  res.end(JSON.stringify(body));
}

function sendProblem(res: ServerResponse, status: number, title: string, detail: string): void {
  if (res.headersSent) {
    res.destroy();
    return;
  }
  // 413 时请求体可能还没读完，响应后关闭连接，避免继续接收。
  if (status === 413) res.setHeader("Connection", "close");
  res.writeHead(status, { "Content-Type": "application/problem+json; charset=utf-8" });
  res.end(JSON.stringify({ title, status, detail }));
}
