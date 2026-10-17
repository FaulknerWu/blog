import { createServer } from "./server.ts";

// RENDER_LISTEN_ADDR 形如 ":8090" 或 "127.0.0.1:8090"，省略主机时监听全部地址。
const addr = process.env["RENDER_LISTEN_ADDR"] || ":8090";
const sep = addr.lastIndexOf(":");
const host = addr.slice(0, sep).replace(/^\[(.*)\]$/, "$1") || undefined;
const port = Number(addr.slice(sep + 1));
if (sep < 0 || !Number.isInteger(port) || port < 0 || port > 65535) {
  console.error(`RENDER_LISTEN_ADDR must look like :8090 or host:8090, got ${JSON.stringify(addr)}`);
  process.exit(1);
}

const server = createServer();
server.listen(port, host, () => {
  console.error(`render listening on ${addr}`);
});

for (const signal of ["SIGTERM", "SIGINT"] as const) {
  process.once(signal, () => {
    server.close((err) => process.exit(err ? 1 : 0));
    server.closeIdleConnections();
  });
}
