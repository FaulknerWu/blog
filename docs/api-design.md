# 博客系统 API 设计

本文档定义博客系统 HTTP 接口的通用规范、接口清单以及各资源的请求与响应结构。

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
| **请求体上限** | 默认 1 MB；文章与页面保存 8 MB；图片上传 20 MB |

### 1.2 认证与安全

- **会话 Cookie**：登录成功后通过 `Set-Cookie` 下发会话令牌，有效期为 30 天滑动过期、90 天绝对过期：
  
  ```http
  Set-Cookie: __Host-session=<token>; Path=/; Secure; HttpOnly; SameSite=Strict; Max-Age=2592000
  ```
- **跨站请求校验**：所有非安全方法（`POST`、`PUT`、`PATCH`、`DELETE`）必须满足 `Sec-Fetch-Site: same-origin` 与站点源完全一致，否则返回 `403`。

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

| `type` 后缀 | 状态码 | 说明 |
| --- | --- | --- |
| `malformed-request` | 400 | JSON 语法错误、未知字段、查询参数或游标无法解析 |
| `unauthenticated` | 401 | 未登录、会话过期或登录凭据错误 |
| `forbidden` | 403 | 跨站请求校验失败 |
| `not-found` | 404 | 资源不存在，或公开接口访问未发布内容 |
| `method-not-allowed` | 405 | 请求方法不受支持 |
| `conflict` | 409 | `slug` 或名称重复、删除仍被引用的图片 |
| `precondition-failed` | 412 | `If-Match` 版本不匹配 |
| `payload-too-large` | 413 | 请求体超出大小上限 |
| `unsupported-media-type` | 415 | `Content-Type` 或上传图片格式不受支持 |
| `validation-error` | 422 | 字段校验失败或业务规则不满足 |
| `precondition-required` | 428 | 缺少 `If-Match` 请求头 |
| `rate-limited` | 429 | 触发请求限流（响应头携带 `Retry-After`） |
| `internal-error` | 500 | 服务端内部错误 |

### 1.6 缓存策略

| 响应类别 | `Cache-Control` |
| --- | --- |
| 公开 JSON 接口 | `no-cache` |
| 管理与认证接口 | `no-store` |
| 订阅源、Sitemap、`robots.txt` | `public, max-age=300`（支持 `ETag` / `Last-Modified` 条件请求） |
| 媒体文件 | `public, max-age=31536000, immutable` |

---

## 2. 接口总览

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
| | `DELETE` | `/api/auth/sessions/current` | ✓ | 退出登录 |
| | `GET` | `/api/auth/sessions` | ✓ | 活跃会话列表 |
| | `DELETE` | `/api/auth/sessions/{id}` | ✓ | 吊销指定会话 |
| | `GET` | `/api/auth/me` | ✓ | 当前用户信息 |
| | `PUT` | `/api/auth/password` | ✓ | 修改密码 |
| **文章管理** | `GET` | `/api/admin/posts` | ✓ | 文章列表（含草稿与定时发布） |
| | `POST` | `/api/admin/posts` | ✓ | 创建文章（草稿） |
| | `GET` | `/api/admin/posts/{id}` | ✓ | 文章详情（含 Markdown 原文） |
| | `PUT` | `/api/admin/posts/{id}` | ✓ | 保存文章内容 |
| | `DELETE` | `/api/admin/posts/{id}` | ✓ | 删除文章 |
| | `PUT` | `/api/admin/posts/{id}/publication` | ✓ | 立即发布或定时发布 |
| | `DELETE` | `/api/admin/posts/{id}/publication` | ✓ | 撤回发布 |
| **页面管理** | `GET` | `/api/admin/pages` | ✓ | 页面列表（含草稿） |
| | `POST` | `/api/admin/pages` | ✓ | 创建页面（草稿） |
| | `GET` | `/api/admin/pages/{id}` | ✓ | 页面详情（含 Markdown 原文） |
| | `PUT` | `/api/admin/pages/{id}` | ✓ | 保存页面内容 |
| | `DELETE` | `/api/admin/pages/{id}` | ✓ | 删除页面 |
| | `PUT` | `/api/admin/pages/{id}/publication` | ✓ | 发布页面 |
| | `DELETE` | `/api/admin/pages/{id}/publication` | ✓ | 撤回发布 |
| **标签管理** | `GET` | `/api/admin/tags` | ✓ | 标签列表 |
| | `POST` | `/api/admin/tags` | ✓ | 创建标签 |
| | `PATCH` | `/api/admin/tags/{id}` | ✓ | 修改标签 |
| | `DELETE` | `/api/admin/tags/{id}` | ✓ | 删除标签 |
| **专栏管理** | `GET` | `/api/admin/series` | ✓ | 专栏列表 |
| | `POST` | `/api/admin/series` | ✓ | 创建专栏 |
| | `GET` | `/api/admin/series/{id}` | ✓ | 专栏详情（含全部成员文章） |
| | `PATCH` | `/api/admin/series/{id}` | ✓ | 修改专栏 |
| | `DELETE` | `/api/admin/series/{id}` | ✓ | 删除专栏 |
| | `PUT` | `/api/admin/series/{id}/posts` | ✓ | 调整专栏文章顺序 |
| **媒体管理** | `GET` | `/api/admin/media` | ✓ | 图片列表或按哈希批量查询 |
| | `POST` | `/api/admin/media` | ✓ | 上传图片 |
| | `GET` | `/api/admin/media/{id}` | ✓ | 图片详情（含引用文章） |
| | `PATCH` | `/api/admin/media/{id}` | ✓ | 修改默认替代文本 |
| | `DELETE` | `/api/admin/media/{id}` | ✓ | 删除图片 |
| **站点设置** | `GET` | `/api/admin/settings` | ✓ | 站点设置详情 |
| | `PATCH` | `/api/admin/settings` | ✓ | 修改站点设置 |
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

**响应 `200`**：

```json
{ "id": 1, "username": "wu", "display_name": "Wu" }
```

### 4.4 修改密码 `PUT /api/auth/password`

**请求体**（`new_password` 长度 ≥ 12；`current_password` 错误返回 `422`）：

```json
{ "current_password": "••••••••", "new_password": "••••••••••••" }
```

**响应 `204`**：更新密码并吊销当前会话外的其余全部会话。

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
| `renderer_version_not` | string | 筛选 `renderer_version` 不等于该值的文章（可选） |
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
  "rendered": {
    "html": "<h2 id=\"背景\">背景</h2><p>…</p>",
    "toc": [ { "depth": 2, "id": "背景", "text": "背景" } ],
    "plain_text": "背景 …",
    "has_math": false,
    "renderer_version": "2026.10.0"
  },
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
| `body_md` | ≤ 1 MB；正文中引用的 `/media/{sha256}` 必须已存在 |
| `rendered` | `body_md` 非空时必填；`html` ≤ 6 MB；`toc` ≤ 200 项且 `depth` 为 2–6 |
| `tag_ids` | ≤ 10 个，必须均已存在 |
| `series_id` | 可空，必须已存在 |
| `cover_media_id` | 可空，必须已存在 |

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

- **详情 `GET /api/admin/media/{id}`**：返回媒体对象，并附加 `referenced_by: [{ "type", "id", "title" }]`，其中 `type` 为 `post` 或 `page`。`reference_count` 为引用该图片的文章与页面数（文章正文与封面同时引用只计一次）。
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

- **列表 `GET /api/admin/pages`**：返回 `{ "items": [AdminPageSummary] }`，按 `updated_at` 倒序，不分页；可选查询参数 `renderer_version_not`（含义同文章列表）。
- **创建 `POST /api/admin/pages` 与保存 `PUT /api/admin/pages/{id}`**：请求体为 `{ "title", "slug", "description", "body_md", "rendered" }`，`rendered` 结构与约束同文章。`POST` 返回 `201`、`Location` 与 `ETag`；`PUT` 要求 `If-Match`，返回 `200` 与新 `ETag`。

  | 字段 | 约束 |
  | --- | --- |
  | `title` | 必填，1–200 字符 |
  | `slug` | 必填，`^[a-z0-9]+(-[a-z0-9]+)*$`，≤ 100 字符；不能为站点保留路径（`admin`、`api`、`archive`、`archives`、`assets`、`feed`、`healthz`、`login`、`logout`、`media`、`page`、`pages`、`posts`、`problems`、`rss`、`search`、`series`、`sitemap`、`static`、`tags`），否则返回 `422`（`code: reserved`）；与其他页面重复返回 `409`。页面与文章的 `slug` 相互独立 |
  | `description` | ≤ 500 字符，用作 meta description |
  | `body_md` / `rendered` | 同文章 |

- **删除 `DELETE /api/admin/pages/{id}`**：要求 `If-Match`，物理删除页面及其媒体引用与历史别名，返回 `204`。
- **发布 `PUT /api/admin/pages/{id}/publication`**：要求 `If-Match`；请求体须为空或 `{}`（页面没有定时发布）；正文为空时返回 `422`；已发布时不修改、版本号不变。返回 `200`、`AdminPageSummary` 与 `ETag`。
- **撤回 `DELETE /api/admin/pages/{id}/publication`**：要求 `If-Match`，状态恢复为 `draft`；已是草稿时不修改。返回 `200`、`AdminPageSummary` 与 `ETag`。

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
| `GET /healthz` | `application/json` | 正常返回 `200`（`{"status":"ok"}`），异常返回 `503` |
