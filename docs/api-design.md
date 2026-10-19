# 博客系统 API 设计

本文档定义博客系统 HTTP 接口的通用规范、接口清单以及各资源的请求与响应结构。字段级的完整契约以 [`api/openapi.yaml`](../api/openapi.yaml)（OpenAPI 3.1）为唯一事实来源，前端类型与 CLI 客户端均由其生成；API 的契约测试保证其中的接口清单与实际注册的路由一致。本文档侧重说明规则与语义。

---

## 1. 通用规范

### 1.1 请求与响应约定

| 项目 | 约定 |
| --- | --- |
| **路径前缀** | REST 接口统一以 `/api` 为前缀；管理端接口位于 `/api/admin` 下 |
| **数据格式** | 请求与响应为 `application/json; charset=utf-8`；图片上传为 `multipart/form-data`；错误响应为 `application/problem+json` |
| **字段命名** | `snake_case` |
| **时间格式** | RFC 3339，UTC，保留微秒精度（如 `2026-10-17T08:30:00.123456Z`） |
| **空值处理** | 可选字段无值时显式返回 `null`，不省略字段 |
| **未知字段** | 请求体包含未定义字段时返回 `400` |
| **请求体上限** | 默认 1 MB；文章与页面保存、正文渲染预览 4 MB（正文本身不超过 1 MB）；图片上传 20 MB |

### 1.2 认证与安全

- **会话 Cookie**：登录成功后通过 `Set-Cookie` 下发会话令牌，有效期为 30 天滑动过期、90 天绝对过期：
  
  ```http
  Set-Cookie: __Host-session=<token>; Path=/; Secure; HttpOnly; SameSite=Strict; Max-Age=2592000
  ```
- **API 令牌**：供 CLI 与 agent 使用的长期凭据，以 `Authorization: Bearer blog_pat_<base64url>` 提交。令牌由作者在后台签发（或在服务器上执行 `blogd token create`），明文只在签发时返回一次，数据库只保存 SHA-256。请求携带 `Authorization` 头时只按令牌鉴权，忽略会话 Cookie。
- **权限范围（scope）**：每个令牌拥有一组 scope，管理接口按下表要求 scope；浏览器会话拥有全部 scope。

  | Scope | 允许的操作 |
  | --- | --- |
  | `content:read` | 读取后台内容：文章、页面、标签、专栏、媒体、站点设置，以及 `GET /api/admin/render` |
  | `content:write` | 创建与保存文章、页面草稿，渲染预览，管理标签与专栏 |
  | `content:publish` | 发布、撤回、删除文章与页面 |
  | `media:write` | 上传、修改、删除图片 |
  | `settings:write` | 修改站点设置 |

  令牌缺少所需 scope 时返回 `403`。**仅限会话**的接口（退出登录、会话管理、修改密码、令牌管理）拒绝令牌并返回 `403`，令牌泄露后无法借此提权、续期或签发新令牌；`GET /api/auth/me` 同时接受两种凭据。
- **跨站请求校验**：只作用于以会话 Cookie 鉴权的非安全方法（`POST`、`PUT`、`PATCH`、`DELETE`）与登录接口，必须满足 `Sec-Fetch-Site: same-origin` 且 `Origin` 与站点源完全一致，否则返回 `403`。浏览器不会自动附带 `Authorization` 头，令牌请求无需此项校验。

### 1.3 游标分页

列表接口统一使用游标分页：

- **请求参数**：`limit`（每页条数）、`cursor`（上一页返回的 `next_cursor`，首页不传）。
- **响应结构**：
  ```json
  {
    "items": [],
    "next_cursor": "eyJwIjoiMjAyNi0xMC0wMVQwODowMDowMC4xMjM0NTZaIiwiaSI6NDJ9"
  }
  ```
  无下一页时 `next_cursor` 为 `null`；游标格式非法时返回 `400`。

### 1.4 条件请求与并发控制

所有可变资源（文章、页面、标签、专栏、媒体、站点设置）响应头均携带 `ETag: "v{version}"`（如 `"v7"`）：

| 场景 | 规则 |
| --- | --- |
| 文章与页面的 `PUT`、`DELETE`、发布与撤回 | 必须携带 `If-Match`，缺失返回 `428` |
| 标签、专栏、媒体、站点设置的 `PATCH`、`DELETE` | 可选携带 `If-Match`，携带时校验版本 |
| 版本不匹配 | 返回 `412`，响应体附带 `current_etag` 与 `updated_at` |
| 更新成功 | 响应头返回新 `ETag` |

### 1.5 错误响应

错误响应遵循 RFC 9457（`application/problem+json`）：

```json
{
  "type": "https://blog.example.com/problems/validation-error",
  "title": "请求参数校验失败",
  "status": 422,
  "detail": "请求中有 2 个字段不符合要求",
  "instance": "/api/admin/posts/42",
  "request_id": "7f3c9a1e2b4d4c6f",
  "errors": [
    { "pointer": "#/slug", "code": "invalid_format", "detail": "只能包含小写字母、数字和连字符" },
    { "pointer": "#/tag_ids/1", "code": "not_found", "detail": "标签不存在" }
  ]
}
```

正文渲染失败时，`errors` 中的条目为 `{ "pointer": "#/body_md", "code": "render_error", "detail": "公式无法编译：…", "line": 12 }`，`line` 为 Markdown 原文中的行号。

| `type` 后缀 | 状态码 | 说明 |
| --- | --- | --- |
| `malformed-request` | 400 | JSON 语法错误、未知字段、查询参数或游标无法解析 |
| `unauthenticated` | 401 | 未登录、会话过期、令牌无效或已过期、登录凭据错误 |
| `forbidden` | 403 | 跨站请求校验失败、令牌缺少所需 scope，或令牌访问仅限会话的接口 |
| `not-found` | 404 | 资源不存在，或公开接口访问未发布内容 |
| `method-not-allowed` | 405 | 请求方法不受支持 |
| `conflict` | 409 | `slug` 或名称重复、删除仍被引用的图片 |
| `precondition-failed` | 412 | `If-Match` 版本不匹配 |
| `payload-too-large` | 413 | 请求体超出大小上限 |
| `unsupported-media-type` | 415 | `Content-Type` 或上传图片格式不受支持 |
| `validation-error` | 422 | 字段校验失败、业务规则不满足，或正文渲染出错（`code: render_error`） |
| `precondition-required` | 428 | 缺少 `If-Match` 请求头 |
| `rate-limited` | 429 | 触发请求限流（响应头携带 `Retry-After`） |
| `internal-error` | 500 | 服务端内部错误 |
| `service-unavailable` | 503 | 渲染服务暂不可用（保存非空正文、渲染预览、`stale` 筛选时） |

### 1.6 缓存策略

| 响应类别 | `Cache-Control` |
| --- | --- |
| 公开 JSON 接口 | `no-cache` |
| 管理与认证接口 | `no-store` |
| 订阅源、Sitemap、`robots.txt` | `public, max-age=300`（支持 `ETag` / `Last-Modified` 条件请求） |
| 媒体文件 | `public, max-age=31536000, immutable` |

---

## 2. 接口总览

认证列中，“会话”表示仅接受浏览器会话，其余为接受会话或令牌时令牌所需的 scope。

| 分组 | 方法 | 路径 | 认证 | 说明 |
| --- | --- | --- | --- | --- |
| **公开内容** | `GET` | `/api/posts` | — | 已发布文章列表 |
| | `GET` | `/api/posts/{slug}` | — | 已发布文章详情 |
| | `GET` | `/api/pages/{slug}` | — | 已发布页面详情 |
| | `GET` | `/api/tags` | — | 标签列表（含文章数） |
| | `GET` | `/api/tags/{slug}` | — | 标签详情 |
| | `GET` | `/api/series` | — | 专栏列表 |
| | `GET` | `/api/series/{slug}` | — | 专栏详情（含文章目录） |
| | `GET` | `/api/search` | — | 全文检索 |
| | `GET` | `/api/settings` | — | 公开站点设置 |
| **认证** | `POST` | `/api/auth/sessions` | — | 登录 |
| | `DELETE` | `/api/auth/sessions/current` | 会话 | 退出登录 |
| | `GET` | `/api/auth/sessions` | 会话 | 活跃会话列表 |
| | `DELETE` | `/api/auth/sessions/{id}` | 会话 | 吊销指定会话 |
| | `GET` | `/api/auth/me` | 任意凭据 | 当前用户与凭据信息 |
| | `PUT` | `/api/auth/password` | 会话 | 修改密码 |
| | `GET` | `/api/auth/tokens` | 会话 | API 令牌列表 |
| | `POST` | `/api/auth/tokens` | 会话 | 签发 API 令牌 |
| | `DELETE` | `/api/auth/tokens/{id}` | 会话 | 吊销 API 令牌 |
| **正文渲染** | `GET` | `/api/admin/render` | `content:read` | 当前渲染器版本 |
| | `POST` | `/api/admin/render` | `content:write` | 渲染预览（不保存） |
| **文章管理** | `GET` | `/api/admin/posts` | `content:read` | 文章列表（含草稿与定时发布） |
| | `POST` | `/api/admin/posts` | `content:write` | 创建文章（草稿） |
| | `GET` | `/api/admin/posts/{id}` | `content:read` | 文章详情（含 Markdown 原文） |
| | `PUT` | `/api/admin/posts/{id}` | `content:write` | 保存文章内容 |
| | `DELETE` | `/api/admin/posts/{id}` | `content:publish` | 删除文章 |
| | `PUT` | `/api/admin/posts/{id}/publication` | `content:publish` | 立即发布或定时发布 |
| | `DELETE` | `/api/admin/posts/{id}/publication` | `content:publish` | 撤回发布 |
| **页面管理** | `GET` | `/api/admin/pages` | `content:read` | 页面列表（含草稿） |
| | `POST` | `/api/admin/pages` | `content:write` | 创建页面（草稿） |
| | `GET` | `/api/admin/pages/{id}` | `content:read` | 页面详情（含 Markdown 原文） |
| | `PUT` | `/api/admin/pages/{id}` | `content:write` | 保存页面内容 |
| | `DELETE` | `/api/admin/pages/{id}` | `content:publish` | 删除页面 |
| | `PUT` | `/api/admin/pages/{id}/publication` | `content:publish` | 发布页面 |
| | `DELETE` | `/api/admin/pages/{id}/publication` | `content:publish` | 撤回发布 |
| **标签管理** | `GET` | `/api/admin/tags` | `content:read` | 标签列表 |
| | `POST` | `/api/admin/tags` | `content:write` | 创建标签 |
| | `PATCH` | `/api/admin/tags/{id}` | `content:write` | 修改标签 |
| | `DELETE` | `/api/admin/tags/{id}` | `content:write` | 删除标签 |
| **专栏管理** | `GET` | `/api/admin/series` | `content:read` | 专栏列表 |
| | `POST` | `/api/admin/series` | `content:write` | 创建专栏 |
| | `GET` | `/api/admin/series/{id}` | `content:read` | 专栏详情（含全部成员文章） |
| | `PATCH` | `/api/admin/series/{id}` | `content:write` | 修改专栏 |
| | `DELETE` | `/api/admin/series/{id}` | `content:write` | 删除专栏 |
| | `PUT` | `/api/admin/series/{id}/posts` | `content:write` | 调整专栏文章顺序 |
| **媒体管理** | `GET` | `/api/admin/media` | `content:read` | 图片列表或按哈希批量查询 |
| | `POST` | `/api/admin/media` | `media:write` | 上传图片 |
| | `GET` | `/api/admin/media/{id}` | `content:read` | 图片详情（含引用文章） |
| | `PATCH` | `/api/admin/media/{id}` | `media:write` | 修改默认替代文本 |
| | `DELETE` | `/api/admin/media/{id}` | `media:write` | 删除图片 |
| **站点设置** | `GET` | `/api/admin/settings` | `content:read` | 站点设置详情 |
| | `PATCH` | `/api/admin/settings` | `settings:write` | 修改站点设置 |
| **内容分发** | `GET` | `/feed.xml` | — | Atom 1.0 全文订阅源 |
| | `GET` | `/rss.xml` | — | RSS 2.0 全文订阅源 |
| | `GET` | `/sitemap.xml` | — | Sitemap |
| | `GET` | `/robots.txt` | — | 抓取规则 |
| | `GET` | `/media/{sha256}` | — | 图片默认规格 |
| | `GET` | `/media/{sha256}/{width}.{format}` | — | 图片指定规格 |
| **健康检查** | `GET` | `/healthz` | — | 服务健康状态 |

---

## 3. 公开接口

### 3.1 公共数据结构

**`TagRef`**

```json
{ "slug": "postgresql", "name": "PostgreSQL" }
```

**`PostRef`**

```json
{ "slug": "render-on-write", "title": "写时渲染：把成本留在编辑器里" }
```

**`Image`**

```json
{
  "sha256": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
  "width": 3024,
  "height": 2016,
  "alt": "部署拓扑示意图",
  "src": "/media/9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
  "variants": [
    { "width": 480,  "height": 320,  "format": "webp", "url": "/media/9f86…0a08/480.webp" },
    { "width": 960,  "height": 640,  "format": "webp", "url": "/media/9f86…0a08/960.webp" },
    { "width": 1600, "height": 1067, "format": "webp", "url": "/media/9f86…0a08/1600.webp" }
  ]
}
```

**`PostSummary`**

```json
{
  "id": 42,
  "slug": "render-on-write",
  "title": "写时渲染：把成本留在编辑器里",
  "summary": "为什么博客系统应该在保存时而不是读取时渲染 Markdown。",
  "word_count": 3820,
  "published_at": "2026-10-01T08:00:00.000000Z",
  "updated_at": "2026-10-03T12:20:41.512300Z",
  "tags": [ { "slug": "architecture", "name": "架构" } ],
  "series": { "slug": "blog-from-scratch", "title": "从零搭建博客" },
  "cover": null
}
```

### 3.2 文章列表 `GET /api/posts`

| 查询参数 | 类型 | 说明 |
| --- | --- | --- |
| `tag` | string | 按标签 `slug` 筛选（可选） |
| `limit` | integer | 默认 20，最大 50 |
| `cursor` | string | 分页游标（可选） |

**响应 `200`**：按 `published_at` 倒序返回 `PostSummary` 的分页结构。

### 3.3 文章详情 `GET /api/posts/{slug}`

**响应 `200`**：

```json
{
  "id": 42,
  "slug": "render-on-write",
  "title": "写时渲染：把成本留在编辑器里",
  "summary": "为什么博客系统应该在保存时而不是读取时渲染 Markdown。",
  "html": "<h2 id=\"背景\">背景</h2><p>…</p>",
  "toc": [
    { "depth": 2, "id": "背景", "text": "背景" },
    { "depth": 2, "id": "方案", "text": "方案" }
  ],
  "has_math": false,
  "word_count": 3820,
  "published_at": "2026-10-01T08:00:00.000000Z",
  "updated_at": "2026-10-03T12:20:41.512300Z",
  "tags": [ { "slug": "architecture", "name": "架构" } ],
  "cover": null,
  "series": {
    "slug": "blog-from-scratch",
    "title": "从零搭建博客",
    "index": 2,
    "total": 5,
    "prev": { "slug": "choosing-the-stack", "title": "技术选型" },
    "next": { "slug": "pg-trgm-search", "title": "用 pg_trgm 做中文检索" }
  },
  "adjacent": {
    "newer": { "slug": "backup-strategy", "title": "单机博客的备份策略" },
    "older": null
  }
}
```

- **响应 `301`**：命中历史别名且目标文章已发布时返回，`Location: /api/posts/{current_slug}`。
- **响应 `404`**：文章不存在或未发布。

### 3.4 标签 `GET /api/tags`、`GET /api/tags/{slug}`

**列表响应 `200`**（仅含至少有一篇已发布文章的标签，按名称排序，不分页）：

```json
{
  "items": [
    { "slug": "architecture", "name": "架构", "post_count": 12 },
    { "slug": "postgresql", "name": "PostgreSQL", "post_count": 5 }
  ]
}
```

**详情响应 `200`**：返回单个标签对象（结构同列表项）；不存在或无已发布文章时返回 `404`。

### 3.5 专栏 `GET /api/series`、`GET /api/series/{slug}`

**列表响应 `200`**（仅含包含已发布文章的专栏，按 `last_published_at` 倒序，不分页）：

```json
{
  "items": [
    {
      "slug": "blog-from-scratch",
      "title": "从零搭建博客",
      "description": "记录这个博客从设计到上线的全过程。",
      "post_count": 5,
      "last_published_at": "2026-10-12T02:00:00.000000Z"
    }
  ]
}
```

**详情响应 `200`**：

```json
{
  "slug": "blog-from-scratch",
  "title": "从零搭建博客",
  "description": "记录这个博客从设计到上线的全过程。",
  "posts": [
    { "index": 1, "slug": "choosing-the-stack", "title": "技术选型", "published_at": "2026-09-20T02:00:00.000000Z" },
    { "index": 2, "slug": "render-on-write", "title": "写时渲染：把成本留在编辑器里", "published_at": "2026-10-01T08:00:00.000000Z" }
  ]
}
```

### 3.6 全文检索 `GET /api/search`

| 查询参数 | 类型 | 说明 |
| --- | --- | --- |
| `q` | string | 必填，去除首尾空白后长度 1–64 字符 |
| `limit` | integer | 默认 20，最大 50 |

**响应 `200`**（按相关度排序，不分页）：

```json
{
  "items": [
    {
      "slug": "pg-trgm-search",
      "title": "用 pg_trgm 做中文检索",
      "summary": "…",
      "snippet": "…由于中文没有天然的空格分词，三元组索引会把整段正文视为一个超长单词…",
      "published_at": "2026-10-12T02:00:00.000000Z"
    }
  ]
}
```

### 3.7 站点设置 `GET /api/settings`

供 Web 渲染布局、页面元数据与列表分页使用：

- 公开文章列表以 `limit={posts_per_page}` 请求。
- 日期按 `timezone` 与 `language` 格式化，归档页按 `timezone` 下的年月分组。
- `noindex` 为 `true` 时，全部页面输出 `<meta name="robots" content="noindex">`。

**响应 `200`**（附带弱 `ETag`，支持 `If-None-Match` 条件请求）：

```json
{
  "title": "Wu 的博客",
  "subtitle": "慢慢写，认真想",
  "description": "记录软件工程与系统设计的思考。",
  "author": "Wu",
  "language": "zh-CN",
  "timezone": "Asia/Shanghai",
  "footer_text": "© 2026 Wu",
  "navigation": [ { "label": "关于", "url": "/about" } ],
  "social_links": [ { "platform": "github", "url": "https://github.com/wu" } ],
  "noindex": false,
  "posts_per_page": 20
}
```

### 3.8 页面详情 `GET /api/pages/{slug}`

独立页面（关于、友链等）挂在站点根路径 `/{slug}` 下，没有发布时间线、标签、专栏与封面，不出现在文章列表、订阅源与搜索结果中。

**响应 `200`**：

```json
{
  "slug": "about",
  "title": "关于",
  "description": "关于本站与作者。",
  "html": "<p>…</p>",
  "toc": [],
  "has_math": false,
  "updated_at": "2026-10-18T03:00:00.000000Z"
}
```

- **响应 `301`**：命中历史别名且目标页面已发布时返回，`Location: /api/pages/{current_slug}`。
- **响应 `404`**：页面不存在或未发布。

---

## 4. 认证接口

### 4.1 登录 `POST /api/auth/sessions`

登录接口执行跨站请求校验，并按客户端 IP 限流（超出时返回 `429` 与 `Retry-After`）。

**请求体**：

```json
{ "username": "wu", "password": "••••••••" }
```

**响应 `201`**（同时下发会话 Cookie）：

```json
{
  "user": { "id": 1, "username": "wu", "display_name": "Wu" },
  "session": { "id": 17, "expires_at": "2026-11-16T08:30:00.000000Z" }
}
```

### 4.2 会话管理

- **`DELETE /api/auth/sessions/current`**：吊销当前会话并清除 Cookie，返回 `204`。
- **`GET /api/auth/sessions`**：返回当前用户的全部活跃会话（`200`）：
  ```json
  {
    "items": [
      {
        "id": 17,
        "current": true,
        "user_agent": "Mozilla/5.0 (X11; Linux x86_64) …",
        "ip": "203.0.113.8",
        "created_at": "2026-10-17T08:30:00.000000Z",
        "last_seen_at": "2026-10-17T09:12:00.000000Z",
        "expires_at": "2026-11-16T09:12:00.000000Z"
      }
    ]
  }
  ```
- **`DELETE /api/auth/sessions/{id}`**：吊销指定会话，返回 `204`。

### 4.3 当前用户 `GET /api/auth/me`

接受会话或令牌，额外返回本次请求所用凭据的类型（`via` 为 `session` 或 `token`）与权限（会话为全部 scope），供 CLI 自检。

**响应 `200`**：

```json
{ "id": 1, "username": "wu", "display_name": "Wu", "via": "token", "scopes": ["content:read", "content:write"] }
```

### 4.4 修改密码 `PUT /api/auth/password`

**请求体**（`new_password` 长度 ≥ 12；`current_password` 错误返回 `422`）：

```json
{ "current_password": "••••••••", "new_password": "••••••••••••" }
```

**响应 `204`**：更新密码并吊销当前会话外的其余全部会话；API 令牌不受影响（服务器上执行 `blogd user reset-password` 时则同时吊销全部会话与令牌）。

### 4.5 API 令牌

以下接口仅接受浏览器会话。

**`Token`**：

```json
{
  "id": 4,
  "name": "claude-code",
  "scopes": ["content:read", "content:write", "media:write"],
  "created_at": "2026-10-19T08:00:00.000000Z",
  "last_used_at": "2026-10-19T09:12:00.000000Z",
  "expires_at": null
}
```

| 接口 | 请求体 | 响应 |
| --- | --- | --- |
| `GET /api/auth/tokens` | — | `200`，`{ "items": [Token] }`（含已过期令牌，按创建时间倒序） |
| `POST /api/auth/tokens` | `{ "name", "scopes", "expires_at"? }` | `201`，`{ "token": Token, "secret": "blog_pat_…" }`；`secret` 只在此时返回 |
| `DELETE /api/auth/tokens/{id}` | — | `204` |

| 字段 | 约束 |
| --- | --- |
| `name` | 1–100 字符（去除首尾空白后），同一用户内唯一，重复返回 `409` |
| `scopes` | 至少一项，取值见 1.2 节 scope 表 |
| `expires_at` | 可省略或为 `null`（不过期）；须晚于当前时间 |

`last_used_at` 最多每分钟更新一次。

---

## 5. 管理接口

### 5.1 文章数据结构

**`AdminPostSummary`**（列表项与写操作响应体，其中 `state` 取值为 `draft` / `scheduled` / `published`）：

```json
{
  "id": 42,
  "slug": "render-on-write",
  "title": "写时渲染：把成本留在编辑器里",
  "state": "scheduled",
  "published_at": "2026-10-20T01:00:00.000000Z",
  "renderer_version": "2026.10.0",
  "tags": [ { "id": 3, "slug": "architecture", "name": "架构" } ],
  "series": { "id": 1, "title": "从零搭建博客", "position": 2 },
  "version": 8,
  "created_at": "2026-09-28T14:02:11.000000Z",
  "updated_at": "2026-10-17T09:30:05.104221Z"
}
```

**`AdminPost`**（`GET /api/admin/posts/{id}` 响应体）：在 `AdminPostSummary` 基础上增加 `summary`（string）、`body_md`（string）、`cover`（`Image` 或 `null`）。

### 5.2 文章列表 `GET /api/admin/posts`

| 查询参数 | 类型 | 说明 |
| --- | --- | --- |
| `state` | string | `draft` / `scheduled` / `published`（可选） |
| `q` | string | 按标题子串筛选（可选） |
| `tag_id` | integer | 按标签 ID 筛选（可选） |
| `slug` | string | 按 `slug` 精确匹配（可选），CLI 据此按 slug 定位文章 |
| `stale` | boolean | 为 `true` 时只返回由非当前渲染器版本渲染的文章（空正文不算过期，需要访问渲染服务） |
| `limit` | integer | 默认 50，最大 100 |
| `cursor` | string | 分页游标（可选） |

**响应 `200`**：按 `updated_at` 倒序返回 `AdminPostSummary` 的分页结构。

### 5.3 创建与保存文章

`POST /api/admin/posts` 与 `PUT /api/admin/posts/{id}` 请求体（`PostInput`）：

```json
{
  "title": "写时渲染：把成本留在编辑器里",
  "slug": "render-on-write",
  "summary": "为什么博客系统应该在保存时而不是读取时渲染 Markdown。",
  "body_md": "## 背景\n\n…\n\n![部署拓扑](/media/9f86d081…0a08)\n",
  "tag_ids": [3, 7],
  "series_id": 1,
  "cover_media_id": null
}
```

| 字段 | 约束 |
| --- | --- |
| `title` | 必填，1–200 字符 |
| `slug` | 必填，`^[a-z0-9]+(-[a-z0-9]+)*$`，≤ 100 字符；重复返回 `409` |
| `summary` | ≤ 500 字符 |
| `body_md` | ≤ 1 MB；正文中以图片引用的 `/media/{sha256}` 必须已存在 |
| `tag_ids` | ≤ 10 个，必须均已存在 |
| `series_id` | 可空，必须已存在 |
| `cover_media_id` | 可空，必须已存在 |

请求体只包含 Markdown 原文，客户端不提交渲染结果（包含 `rendered` 等未定义字段时返回 `400`）。服务端在事务之外调用渲染服务，生成 HTML、目录、搜索纯文本、字数与 `renderer_version` 后与原文一并保存：

- 渲染出错（如公式无法编译、引用的图片不在媒体库中）时返回 `422`，错误条目为 `code: render_error` 并带 `line`；警告（如图片缺少替代文本）不影响保存，可通过渲染预览获取。
- 渲染服务不可用时返回 `503`；`body_md` 为空时不调用渲染服务，`renderer_version` 为空字符串。

- **`POST` 响应 `201`**：以草稿状态创建，返回 `AdminPostSummary`，响应头含 `Location` 与 `ETag`。
- **`PUT` 响应 `200`**：要求 `If-Match`，返回 `AdminPostSummary` 与新 `ETag`。

### 5.4 删除文章 `DELETE /api/admin/posts/{id}`

要求 `If-Match`，物理删除文章及关联记录，返回 `204`。

### 5.5 发布与撤回

- **发布 `PUT /api/admin/posts/{id}/publication`**（要求 `If-Match`，正文为空时返回 `422`）：
  ```json
  { "published_at": "2026-10-20T01:00:00Z" }
  ```
  `published_at` 可省略或为 `null`（曾发布过则沿用原时间，否则取当前时间）；晚于当前时间即为定时发布。返回 `200`、`AdminPostSummary` 与新 `ETag`。
- **撤回 `DELETE /api/admin/posts/{id}/publication`**（要求 `If-Match`）：状态恢复为 `draft`（保留 `published_at`），返回 `200`、`AdminPostSummary` 与新 `ETag`。

### 5.6 标签管理

**标签对象结构**：

```json
{
  "id": 3,
  "slug": "postgresql",
  "name": "PostgreSQL",
  "post_count": 6,
  "published_post_count": 5,
  "version": 1,
  "created_at": "2026-09-20T02:00:00.000000Z",
  "updated_at": "2026-09-20T02:00:00.000000Z"
}
```

| 接口 | 请求体 | 响应 |
| --- | --- | --- |
| `GET /api/admin/tags` | — | `200`，`{ "items": [ ... ] }`（全部标签，不分页） |
| `POST /api/admin/tags` | `{ "name", "slug" }` | `201`，标签对象（名称或 `slug` 重复返回 `409`） |
| `PATCH /api/admin/tags/{id}` | `{ "name"?, "slug"? }` | `200`，标签对象 |
| `DELETE /api/admin/tags/{id}` | — | `204` |

### 5.7 专栏管理

**专栏对象结构**：

```json
{
  "id": 1,
  "slug": "blog-from-scratch",
  "title": "从零搭建博客",
  "description": "记录这个博客从设计到上线的全过程。",
  "post_count": 5,
  "published_post_count": 4,
  "version": 2,
  "created_at": "2026-09-20T02:00:00.000000Z",
  "updated_at": "2026-10-12T02:00:00.000000Z"
}
```

| 接口 | 请求体 | 响应 |
| --- | --- | --- |
| `GET /api/admin/series` | — | `200`，`{ "items": [ ... ] }`（全部专栏，不分页） |
| `POST /api/admin/series` | `{ "title", "slug", "description"? }` | `201`，专栏对象 |
| `GET /api/admin/series/{id}` | — | `200`，专栏对象附加 `posts: [{ "id", "slug", "title", "state", "position" }]` |
| `PATCH /api/admin/series/{id}` | `{ "title"?, "slug"?, "description"? }` | `200`，专栏对象 |
| `DELETE /api/admin/series/{id}` | — | `204`（成员文章自动移出专栏） |
| `PUT /api/admin/series/{id}/posts` | `{ "post_ids": [12, 5, 9] }` | `200`，专栏详情（`post_ids` 须与当前成员集合完全一致，否则返回 `422`） |

### 5.8 媒体管理

**媒体对象结构**：

```json
{
  "id": 23,
  "sha256": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
  "width": 3024,
  "height": 2016,
  "byte_size": 2481920,
  "original_filename": "topology.png",
  "alt": "部署拓扑示意图",
  "src": "/media/9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
  "variants": [
    { "width": 480, "height": 320, "format": "webp", "url": "/media/9f86…0a08/480.webp" }
  ],
  "reference_count": 0,
  "version": 1,
  "created_at": "2026-10-17T09:50:00.000000Z",
  "updated_at": "2026-10-17T09:50:00.000000Z"
}
```

- **上传 `POST /api/admin/media`**：`multipart/form-data`，包含 `file`（必填，≤ 20 MB，仅支持 JPEG / PNG / WebP，总像素 ≤ 4000 万）与 `alt`（可选，≤ 500 字符）。新文件返回 `201`，重复文件去重返回 `200`，响应体均为媒体对象。
- **列表 `GET /api/admin/media`**：

  | 查询参数 | 类型 | 说明 |
  | --- | --- | --- |
  | `sha256` | string[] | 按哈希批量查询（≤ 100 个），传入时忽略分页参数 |
  | `unused` | boolean | 为 `true` 时仅返回 `reference_count = 0` 的图片 |
  | `limit` / `cursor` | integer / string | 默认 50，最大 100，按上传时间倒序 |

- **详情 `GET /api/admin/media/{id}`**：返回媒体对象，并附加 `referenced_by: [{ "type", "id", "title" }]`（没有引用时省略该字段），其中 `type` 为 `post` 或 `page`。`reference_count` 为引用该图片的文章与页面数（文章正文与封面同时引用只计一次）。
- **修改 `PATCH /api/admin/media/{id}`**：请求体 `{ "alt": "..." }`，返回 `200` 与媒体对象。
- **删除 `DELETE /api/admin/media/{id}`**：无引用时返回 `204`；仍被文章正文、文章封面或页面正文引用时返回 `409`（附带 `referenced_by`）。

### 5.9 站点设置管理

**站点设置对象结构**（在公开站点设置基础上增加以下字段）：

```json
{
  "title": "Wu 的博客",
  "subtitle": "慢慢写，认真想",
  "description": "记录软件工程与系统设计的思考。",
  "author": "Wu",
  "language": "zh-CN",
  "timezone": "Asia/Shanghai",
  "footer_text": "© 2026 Wu",
  "navigation": [ { "label": "关于", "url": "/about" } ],
  "social_links": [ { "platform": "github", "url": "https://github.com/wu" } ],
  "noindex": false,
  "posts_per_page": 20,
  "feed_item_count": 20,
  "version": 3,
  "updated_at": "2026-10-18T03:00:00.000000Z"
}
```

| 接口 | 请求体 | 响应 |
| --- | --- | --- |
| `GET /api/admin/settings` | — | `200`，站点设置对象与 `ETag` |
| `PATCH /api/admin/settings` | 上述任意可写字段的子集 | `200`，站点设置对象与新 `ETag`（请求体为 `{}` 时不递增版本） |

| 字段 | 约束 |
| --- | --- |
| `title` | 1–100 字符（去除首尾空白后） |
| `subtitle` | ≤ 200 字符 |
| `description` | ≤ 500 字符 |
| `author` | 1–100 字符（去除首尾空白后） |
| `language` | BCP 47 语言标签（如 `zh-CN`、`en`），≤ 35 字符 |
| `timezone` | IANA 时区名称（如 `Asia/Shanghai`、`UTC`），不接受 `Local` |
| `footer_text` | ≤ 500 字符，纯文本 |
| `navigation` | ≤ 10 项，整体替换；`label` 1–50 字符；`url` ≤ 500 字符，须为以 `/` 开头的站内路径（不含 `//`）或 `http(s)` 绝对地址 |
| `social_links` | ≤ 10 项，整体替换；`platform` 为 `^[a-z0-9]+(-[a-z0-9]+)*$`、≤ 30 字符（如 `github`、`x`、`email`，前端据此选择图标，未知平台使用通用图标）；`url` ≤ 500 字符，须为 `http(s)` 绝对地址或 `mailto:` 邮箱地址 |
| `noindex` | 布尔值 |
| `posts_per_page` | 1–50 |
| `feed_item_count` | 1–100 |

站点设置在每次请求时从数据库读取，修改后立即作用于订阅源与公开接口，无需重启服务。部署相关参数（数据库地址、站点源 `BLOG_SITE_URL`、媒体目录等）仍通过环境变量配置，不在此接口范围内。

### 5.10 页面管理

**`AdminPageSummary`**（列表项与写操作响应体，`state` 取值为 `draft` / `published`）：

```json
{
  "id": 3,
  "slug": "about",
  "title": "关于",
  "state": "published",
  "renderer_version": "2026.10.0",
  "version": 4,
  "created_at": "2026-10-01T08:00:00.000000Z",
  "updated_at": "2026-10-18T03:00:00.000000Z"
}
```

**`AdminPage`**（`GET /api/admin/pages/{id}` 响应体）：在 `AdminPageSummary` 基础上增加 `description`（string）、`body_md`（string）。

- **列表 `GET /api/admin/pages`**：返回 `{ "items": [AdminPageSummary] }`，按 `updated_at` 倒序，不分页；可选查询参数 `slug`、`stale`（含义同文章列表）。
- **创建 `POST /api/admin/pages` 与保存 `PUT /api/admin/pages/{id}`**：请求体为 `{ "title", "slug", "description", "body_md" }`，服务端渲染规则与错误同文章。`POST` 返回 `201`、`Location` 与 `ETag`；`PUT` 要求 `If-Match`，返回 `200` 与新 `ETag`。

  | 字段 | 约束 |
  | --- | --- |
  | `title` | 必填，1–200 字符 |
  | `slug` | 必填，`^[a-z0-9]+(-[a-z0-9]+)*$`，≤ 100 字符；不能为站点保留路径（`admin`、`api`、`archive`、`archives`、`assets`、`feed`、`healthz`、`login`、`logout`、`media`、`page`、`pages`、`posts`、`problems`、`rss`、`search`、`series`、`sitemap`、`static`、`tags`），否则返回 `422`（`code: reserved`）；与其他页面重复返回 `409`。页面与文章的 `slug` 相互独立 |
  | `description` | ≤ 500 字符，用作 meta description |
  | `body_md` | 同文章 |

- **删除 `DELETE /api/admin/pages/{id}`**：要求 `If-Match`，物理删除页面及其媒体引用与历史别名，返回 `204`。
- **发布 `PUT /api/admin/pages/{id}/publication`**：要求 `If-Match`；请求体须为空或 `{}`（页面没有定时发布）；正文为空时返回 `422`；已发布时不修改、版本号不变。返回 `200`、`AdminPageSummary` 与 `ETag`。
- **撤回 `DELETE /api/admin/pages/{id}/publication`**：要求 `If-Match`，状态恢复为 `draft`；已是草稿时不修改。返回 `200`、`AdminPageSummary` 与 `ETag`。

### 5.11 正文渲染

正文由独立的渲染服务完成 Markdown 解析、代码高亮与公式编译，API 只在写入时调用它（写时渲染）。以下接口供后台编辑器预览与 CLI 自检：

- **渲染器版本 `GET /api/admin/render`**：返回 `{ "renderer_version": "2026.10.0" }`。`renderer_version` 不等于该值的非空内容即为过期内容；渲染器升级后 API 启动时会自动重新渲染过期内容，也可在服务器上执行 `blogd rerender`。重新渲染会递增版本号，使公开接口的 `ETag` 失效。
- **渲染预览 `POST /api/admin/render`**：请求体 `{ "body_md": "..." }`，不保存。渲染问题随结果返回而不作为错误响应：

  ```json
  {
    "html": "<h2 id=\"背景\">背景</h2><p>…</p>",
    "toc": [ { "depth": 2, "id": "背景", "text": "背景" } ],
    "has_math": true,
    "word_count": 3820,
    "renderer_version": "2026.10.0",
    "issues": [
      { "line": 12, "message": "公式无法编译：…", "severity": "error" },
      { "line": 20, "message": "图片缺少替代文本", "severity": "warning" }
    ]
  }
  ```

  `severity` 为 `error` 的问题会使保存返回 `422`；`warning` 仅作提示。渲染服务不可用时返回 `503`。

---

## 6. 内容分发与健康检查

| 接口 | 响应格式 | 说明 |
| --- | --- | --- |
| `GET /feed.xml` | `application/atom+xml` | Atom 1.0 订阅源，包含最近 `feed_item_count` 篇已发布文章全文（站内相对路径转为绝对 URL）；标题、副标题、作者与语言取自站点设置 |
| `GET /rss.xml` | `application/rss+xml` | RSS 2.0 订阅源，内容同 `/feed.xml` |
| `GET /sitemap.xml` | `application/xml` | 包含首页、全部已发布文章、已发布页面（`/{slug}`）、标签页与专栏页；站点设置 `noindex` 为 `true` 时返回 `404` |
| `GET /robots.txt` | `text/plain` | 允许抓取全部路径并声明 Sitemap 地址；`noindex` 时仅省略 Sitemap 声明，不使用 `Disallow`（抓取器须能访问页面才能读到 `noindex` 标记），订阅源同时附加 `X-Robots-Tag: noindex` |
| `GET /media/{sha256}` | 对应图片 MIME | 返回图片默认规格（不超过 1600 宽的最大档位） |
| `GET /media/{sha256}/{width}.{format}` | 对应图片 MIME | 返回指定衍生规格，不存在时返回 `404` |
| `GET /healthz` | `application/json` | 只检查数据库连接：正常返回 `200`（`{"status":"ok"}`），异常返回 `503`（`{"status":"unavailable"}`）；不依赖渲染服务 |
