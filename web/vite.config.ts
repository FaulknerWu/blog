import { fileURLToPath } from "node:url";
import { reactRouter } from "@react-router/dev/vite";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite";

const apiOrigin = process.env.API_ORIGIN ?? "http://127.0.0.1:8080";

// 生产环境由 Nginx 按路径分发；开发时由 Vite 把浏览器直接请求的 API 路径转发给 Go 服务。
// 后台的写请求要通过 API 的同源校验，开发时 API 的 BLOG_SITE_URL 需设为 Vite 地址（如 http://localhost:5173）。
const apiPaths = ["/api", "/media", "/feed.xml", "/rss.xml", "/sitemap.xml", "/robots.txt"];

export default defineConfig({
  plugins: process.env.VITEST ? [] : [tailwindcss(), reactRouter()],
  resolve: {
    alias: { "~": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  server: {
    proxy: Object.fromEntries(apiPaths.map((p) => [p, apiOrigin])),
  },
});
