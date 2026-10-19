---
name: blog
description: Read, write and publish content on the user's blog with the `blog` CLI — posts, standalone pages, tags, series, images and site settings. Use when asked to draft, edit, proofread, publish, schedule, tag or reorganise blog content, upload images for it, or inspect what is on the blog.
---

# blog CLI

`blog` talks to the blog's API with an API token. Every command prints JSON on stdout; on failure it prints
one line of RFC 9457 problem JSON on stderr and exits with a code that tells you what to do next.

Credentials come from `BLOG_URL` + `BLOG_TOKEN`, or from a profile saved with
`blog auth login --url https://… ` (token on stdin). Run `blog auth status` first: it shows the user and the
token's `scopes`. `blog schema` lists every command and flag as JSON; `blog <command> -h` shows one.

## Exit codes

| code | meaning | what to do |
| --- | --- | --- |
| 0 | ok | |
| 2 | usage error | fix the command line |
| 3 | not configured / token invalid / missing scope | tell the user which scope is missing (`detail` names it); do not try to work around it |
| 4 | not found | check the id or slug with a `list` command |
| 5 | version conflict or duplicate (409/412/428) | `pull` again, re-apply your edit to the fresh file, `push` again |
| 6 | input rejected or Markdown failed to render | read `errors[]` (`pointer`, `line`, `detail`) and fix the file |
| 7 | network, rate limit or server temporarily unavailable | wait and retry the same command |

## Editing content: files, not JSON

Posts and pages are edited as Markdown files with YAML front matter:

```markdown
---
id: 42                 # set by the server; absent = push creates a new post
version: 7             # optimistic lock; never edit by hand
title: 写时渲染
slug: render-on-write  # lowercase letters, digits, hyphens
summary: 一句话摘要
tags: [postgresql, go] # tag slugs; they must exist (blog tag list / blog tag create)
series: blog-engineering   # optional series slug
cover: ./cover.png     # optional: local image path or /media/<sha256>
---

## 第一节

正文。![替代文本](./images/diagram.png "可选图注")
```

Pages use `title`, `slug`, `description` instead of `summary/tags/series/cover`. Unknown front matter keys
are rejected, so a typo fails loudly.

Workflow:

1. **Existing content:** `blog post pull <id|slug> -o post.md` (default file name is `<slug>.md`). Never
   recreate an existing post from scratch — you would lose `id`/`version` and create a duplicate.
2. Edit the file. Reference local images with relative paths; `push` uploads them (deduplicated by content
   hash) and rewrites the links to `/media/<sha256>`. Never invent `/media/…` URLs yourself.
3. `blog post render post.md` — renders with the real server renderer and lists `issues` (missing alt text
   warnings, formula errors with line numbers). Exit 6 means there are errors to fix first.
4. `blog post push post.md` — saves the draft and writes the new `id`, `version` and rewritten image links back
   into the file. Use `--dry-run` to see the request without changing anything.
5. Publishing is a separate, deliberate step: `blog post publish <id|slug>` (optionally `--at 2026-11-01T08:00:00+08:00`
   to schedule), `blog post unpublish <id|slug>`. It needs the `content:publish` scope; tokens for agents often
   do not have it. **Only publish, unpublish or delete when the user explicitly asked for it.**

After a successful push the file mirrors the server, so you can keep editing and pushing it. If someone else
saved in between, push exits 5: pull into the same path and redo your change.

The body is rendered on the server (GFM, `$…$` / `$$…$$` math, fenced code with highlighting). Raw HTML in
Markdown is dropped. The first `# h1` in the body is not part of the table of contents — the post title is the
page heading, so start sections at `##`.

## Other commands

- `blog post list [--state draft|scheduled|published] [--q text] [--tag slug] [--stale] [--all]`
- `blog post get <id|slug>` — full JSON including `body_md`
- `blog page list|get|pull|push|render|publish|unpublish|delete` — same as posts
- `blog tag list|create --name N --slug S|update <ref> [--name] [--slug]|delete <ref> --yes`
- `blog series list|get <ref>|create --title T --slug S [--description D]|update <ref> …|delete <ref> --yes`
- `blog series reorder <series> <post> <post> …` — list every member in the new order
- `blog media list [--unused]|get <id|sha256>|upload <file>… [--alt text]|update <ref> --alt text|delete <ref> --yes`
- `blog settings get`, `blog settings update '{"title":"…"}'` (needs `settings:write`)

Deletes require `--yes` and cannot be undone; confirm with the user first.
