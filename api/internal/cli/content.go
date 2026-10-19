package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// kind 描述文章与页面在 API 路径与 front matter 上的差异，两者共用同一组命令。
type kind struct {
	name   string // post | page
	plural string // posts | pages
	label  string
}

var (
	postKind = kind{name: "post", plural: "posts", label: "文章"}
	pageKind = kind{name: "page", plural: "pages", label: "页面"}
)

func (k kind) collection() string          { return "/api/admin/" + k.plural }
func (k kind) item(id int64) string        { return fmt.Sprintf("/api/admin/%s/%d", k.plural, id) }
func (k kind) publication(id int64) string { return k.item(id) + "/publication" }

func contentCommands(k kind) []command {
	cmds := []command{
		{name: k.name + " list", summary: "列出" + k.label, setup: func(fs *flag.FlagSet) runFunc {
			stale := fs.Bool("stale", false, "只列出由旧版渲染器生成的"+k.label)
			if k == pageKind {
				return func(ctx context.Context, a *app, args []string) error {
					q := url.Values{}
					if *stale {
						q.Set("stale", "true")
					}
					r, err := a.api.get(ctx, k.collection(), q)
					if err != nil {
						return err
					}
					return a.out(r.body)
				}
			}
			state := fs.String("state", "", "按状态过滤：draft、scheduled、published")
			search := fs.String("q", "", "按标题搜索")
			tag := fs.String("tag", "", "按标签（id 或 slug）过滤")
			limit := fs.Int("limit", 0, "每页条数")
			cursor := fs.String("cursor", "", "上一页返回的 next_cursor")
			all := fs.Bool("all", false, "自动翻页取回全部")
			return func(ctx context.Context, a *app, args []string) error {
				q := url.Values{}
				setNonEmpty(q, "state", *state)
				setNonEmpty(q, "q", *search)
				setNonEmpty(q, "cursor", *cursor)
				if *limit > 0 {
					q.Set("limit", strconv.Itoa(*limit))
				}
				if *stale {
					q.Set("stale", "true")
				}
				if *tag != "" {
					id, err := a.resolveTag(ctx, *tag)
					if err != nil {
						return err
					}
					q.Set("tag_id", strconv.FormatInt(id, 10))
				}
				body, err := a.api.list(ctx, k.collection(), q, *all)
				if err != nil {
					return err
				}
				return a.out(body)
			}
		}},
		{name: k.name + " get", args: "<id|slug>", summary: "输出" + k.label + "详情（含 Markdown 原文）", setup: func(fs *flag.FlagSet) runFunc {
			return func(ctx context.Context, a *app, args []string) error {
				if err := needArgs(args, 1, "<id|slug>"); err != nil {
					return err
				}
				id, err := a.resolveContent(ctx, k, args[0])
				if err != nil {
					return err
				}
				r, err := a.api.get(ctx, k.item(id), nil)
				if err != nil {
					return err
				}
				return a.out(r.body)
			}
		}},
		{name: k.name + " pull", args: "<id|slug>", summary: "把" + k.label + "写为带 front matter 的 Markdown 文件", setup: func(fs *flag.FlagSet) runFunc {
			output := fs.String("o", "", "输出文件，默认 <slug>.md；- 表示标准输出")
			return func(ctx context.Context, a *app, args []string) error {
				if err := needArgs(args, 1, "<id|slug>"); err != nil {
					return err
				}
				return a.pull(ctx, k, args[0], *output)
			}
		}},
		{name: k.name + " push", args: "<file>", summary: "创建或更新" + k.label + "：上传本地图片、提交正文并把 id 与 version 写回文件", setup: func(fs *flag.FlagSet) runFunc {
			dryRun := fs.Bool("dry-run", false, "只输出将要发送的请求，不上传、不保存、不改写文件")
			return func(ctx context.Context, a *app, args []string) error {
				if err := needArgs(args, 1, "<file>"); err != nil {
					return err
				}
				return a.push(ctx, k, args[0], *dryRun)
			}
		}},
		{name: k.name + " render", args: "<file|->", summary: "用服务端渲染器检查正文，输出 HTML、目录与问题列表；有错误时退出码 6", setup: func(fs *flag.FlagSet) runFunc {
			return func(ctx context.Context, a *app, args []string) error {
				if err := needArgs(args, 1, "<file|->"); err != nil {
					return err
				}
				return a.render(ctx, k, args[0])
			}
		}},
		{name: k.name + " publish", args: "<id|slug>", summary: "发布" + k.label + "（需要 content:publish）", setup: func(fs *flag.FlagSet) runFunc {
			version := fs.Int("version", 0, "期望的当前版本号；不指定时使用服务端最新版本")
			var at *string
			if k == postKind {
				at = fs.String("at", "", "发布时间（RFC 3339）；晚于当前时间即为定时发布")
			}
			return func(ctx context.Context, a *app, args []string) error {
				if err := needArgs(args, 1, "<id|slug>"); err != nil {
					return err
				}
				var body any
				if at != nil && *at != "" {
					t, err := time.Parse(time.RFC3339, *at)
					if err != nil {
						return usageError("--at 必须是 RFC 3339 时间，如 2026-11-01T08:00:00+08:00")
					}
					body = map[string]any{"published_at": t}
				}
				return a.changeContent(ctx, k, args[0], int32(*version), http.MethodPut, "/publication", body)
			}
		}},
		{name: k.name + " unpublish", args: "<id|slug>", summary: "撤回发布（需要 content:publish）", setup: func(fs *flag.FlagSet) runFunc {
			version := fs.Int("version", 0, "期望的当前版本号；不指定时使用服务端最新版本")
			return func(ctx context.Context, a *app, args []string) error {
				if err := needArgs(args, 1, "<id|slug>"); err != nil {
					return err
				}
				return a.changeContent(ctx, k, args[0], int32(*version), http.MethodDelete, "/publication", nil)
			}
		}},
		{name: k.name + " delete", args: "<id|slug>", summary: "删除" + k.label + "（需要 content:publish）", setup: func(fs *flag.FlagSet) runFunc {
			version := fs.Int("version", 0, "期望的当前版本号；不指定时使用服务端最新版本")
			yes := fs.Bool("yes", false, "确认删除")
			return func(ctx context.Context, a *app, args []string) error {
				if err := needArgs(args, 1, "<id|slug>"); err != nil {
					return err
				}
				if err := requireYes(*yes, "删除"+k.label); err != nil {
					return err
				}
				return a.changeContent(ctx, k, args[0], int32(*version), http.MethodDelete, "", nil)
			}
		}},
	}
	return cmds
}

func setNonEmpty(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}

// resolveContent 把 id 或 slug 解析为文章或页面 ID。
func (a *app) resolveContent(ctx context.Context, k kind, ref string) (int64, error) {
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil {
		return id, nil
	}
	var page struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	if err := a.api.getJSON(ctx, k.collection(), url.Values{"slug": {ref}}, &page); err != nil {
		return 0, err
	}
	if len(page.Items) == 0 {
		return 0, notFoundError("没有 slug 为 %q 的%s", ref, k.label)
	}
	return page.Items[0].ID, nil
}

// changeContent 发送带 If-Match 的状态变更请求（发布、撤回、删除）。
func (a *app) changeContent(ctx context.Context, k kind, ref string, version int32, method, suffix string, body any) error {
	id, err := a.resolveContent(ctx, k, ref)
	if err != nil {
		return err
	}
	if version == 0 {
		r, err := a.api.get(ctx, k.item(id), nil)
		if err != nil {
			return err
		}
		version = r.version()
	}
	r, err := a.api.do(ctx, request{method: method, path: k.item(id) + suffix, body: body, ifMatch: version})
	if err != nil {
		return err
	}
	if r.status == http.StatusNoContent {
		return a.out(map[string]any{"id": id, "deleted": true})
	}
	return a.out(r.body)
}

type adminContent struct {
	ID          int64  `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	BodyMD      string `json:"body_md"`
	Version     int32  `json:"version"`
	Tags        []struct {
		Slug string `json:"slug"`
	} `json:"tags"`
	Series *struct {
		ID int64 `json:"id"`
	} `json:"series"`
	Cover *struct {
		SHA256 string `json:"sha256"`
	} `json:"cover"`
}

func (a *app) pull(ctx context.Context, k kind, ref, output string) error {
	id, err := a.resolveContent(ctx, k, ref)
	if err != nil {
		return err
	}
	var c adminContent
	if err := a.api.getJSON(ctx, k.item(id), nil, &c); err != nil {
		return err
	}
	var meta any
	if k == postKind {
		pm := postMeta{ID: c.ID, Version: c.Version, Title: c.Title, Slug: c.Slug, Summary: c.Summary, Tags: flowList{}}
		for _, t := range c.Tags {
			pm.Tags = append(pm.Tags, t.Slug)
		}
		if c.Series != nil {
			if pm.Series, err = a.seriesSlug(ctx, c.Series.ID); err != nil {
				return err
			}
		}
		if c.Cover != nil {
			pm.Cover = "/media/" + c.Cover.SHA256
		}
		meta = pm
	} else {
		meta = pageMeta{ID: c.ID, Version: c.Version, Title: c.Title, Slug: c.Slug, Description: c.Description}
	}
	data, err := formatDocument(meta, c.BodyMD)
	if err != nil {
		return err
	}
	if output == "-" {
		_, err := a.env.Stdout.Write(data)
		return err
	}
	if output == "" {
		output = c.Slug + ".md"
	}
	if err := writeFileAtomic(output, data, 0o644); err != nil {
		return err
	}
	return a.out(map[string]any{"file": output, "id": c.ID, "slug": c.Slug, "version": c.Version})
}

func readInput(a *app, path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(a.env.Stdin)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, invalidError("读取 %s：%v", path, err)
	}
	return data, nil
}

// pushResult 是 push 的输出：保存后的资源摘要，以及本次上传的本地图片。
type pushResult struct {
	File     string          `json:"file"`
	Created  bool            `json:"created"`
	Uploaded []*localImage   `json:"uploaded"`
	Resource json.RawMessage `json:"resource"`
}

func (a *app) push(ctx context.Context, k kind, path string, dryRun bool) error {
	data, err := readInput(a, path)
	if err != nil {
		return err
	}
	baseDir := filepath.Dir(path)
	var pm postMeta
	var gm pageMeta
	meta := any(&gm)
	if k == postKind {
		meta = &pm
	}
	body, err := parseDocument(data, meta)
	if err != nil {
		return err
	}
	id, version := gm.ID, gm.Version
	if k == postKind {
		id, version = pm.ID, pm.Version
	}
	if id != 0 && version == 0 {
		return invalidError("front matter 有 id 却没有 version：先运行 blog %s pull 取得最新版本", k.name)
	}

	images, err := localImages(body, baseDir)
	if err != nil {
		return err
	}
	var cover *localImage
	if k == postKind && isLocal(pm.Cover) {
		cover = &localImage{Path: filepath.Join(baseDir, filepath.FromSlash(pm.Cover)), alt: pm.Title}
		covers, err := hashFiles([]*localImage{cover})
		if err != nil {
			return err
		}
		cover = covers[0]
	}

	method, target := http.MethodPost, k.collection()
	if id != 0 {
		method, target = http.MethodPut, k.item(id)
	}
	newBody := rewriteLocalImages(body, baseDir, images)
	if dryRun {
		input, err := a.contentInput(ctx, k, pm, gm, newBody, cover, true)
		if err != nil {
			return err
		}
		pending := append([]*localImage{}, images...)
		if cover != nil {
			pending = append(pending, cover)
		}
		return a.out(map[string]any{"dry_run": true, "method": method, "path": target,
			"if_match": ifMatchOrNil(version), "local_images": pending, "body": input})
	}

	all := images
	if cover != nil {
		all = append(append([]*localImage{}, images...), cover)
	}
	uploaded, err := a.ensureUploaded(ctx, all)
	if err != nil {
		return err
	}
	input, err := a.contentInput(ctx, k, pm, gm, newBody, cover, false)
	if err != nil {
		return err
	}
	r, err := a.api.do(ctx, request{method: method, path: target, body: input, ifMatch: version})
	if err != nil {
		return err
	}
	var saved struct {
		ID int64 `json:"id"`
	}
	if err := r.decode(&saved); err != nil {
		return err
	}

	// 写回：id、新版本号与改写后的图片地址，使文件与服务端一致。
	if k == postKind {
		pm.ID, pm.Version = saved.ID, r.version()
		if cover != nil {
			pm.Cover = "/media/" + cover.SHA256
		}
		meta = pm
	} else {
		gm.ID, gm.Version = saved.ID, r.version()
		meta = gm
	}
	if path != "-" {
		out, err := formatDocument(meta, newBody)
		if err != nil {
			return err
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if err := writeFileAtomic(path, out, info.Mode().Perm()); err != nil {
			return err
		}
	}
	if uploaded == nil {
		uploaded = []*localImage{}
	}
	return a.out(pushResult{File: path, Created: method == http.MethodPost, Uploaded: uploaded, Resource: r.body})
}

func ifMatchOrNil(version int32) any {
	if version == 0 {
		return nil
	}
	return ifMatch(version)
}

func hashFiles(images []*localImage) ([]*localImage, error) {
	for _, img := range images {
		data, err := os.ReadFile(img.Path)
		if err != nil {
			return nil, invalidError("封面图片无法读取：%v", err)
		}
		img.SHA256 = sha256Hex(data)
	}
	return images, nil
}

// contentInput 组装保存请求体：标签与专栏按 slug 解析为 ID，封面解析为媒体 ID。
// dryRun 时本地封面尚未上传，只给出哈希。
func (a *app) contentInput(ctx context.Context, k kind, pm postMeta, gm pageMeta, body string, cover *localImage, dryRun bool) (map[string]any, error) {
	if k == pageKind {
		return map[string]any{"title": gm.Title, "slug": gm.Slug, "description": gm.Description, "body_md": body}, nil
	}
	tagIDs := []int64{}
	for _, slug := range pm.Tags {
		id, err := a.resolveTag(ctx, slug)
		if err != nil {
			return nil, err
		}
		tagIDs = append(tagIDs, id)
	}
	in := map[string]any{"title": pm.Title, "slug": pm.Slug, "summary": pm.Summary, "body_md": body,
		"tag_ids": tagIDs, "series_id": nil, "cover_media_id": nil}
	if pm.Series != "" {
		id, err := a.resolveSeries(ctx, pm.Series)
		if err != nil {
			return nil, err
		}
		in["series_id"] = id
	}
	switch {
	case cover != nil && dryRun:
		in["cover_media_id"] = "<" + cover.SHA256 + ">"
	case cover != nil:
		in["cover_media_id"] = cover.ID
	case pm.Cover != "":
		m := mediaRef.FindStringSubmatch(pm.Cover)
		if m == nil {
			return nil, invalidError("cover 必须是本地图片路径或 /media/{sha256}")
		}
		found, err := a.lookupMedia(ctx, []string{m[1]})
		if err != nil {
			return nil, err
		}
		id, ok := found[m[1]]
		if !ok {
			return nil, notFoundError("媒体库中没有封面图片 %s", pm.Cover)
		}
		in["cover_media_id"] = id
	}
	return in, nil
}

// render 调用服务端预览渲染。本地图片尚未上传时按哈希改写后渲染，
// 这些图片所在行的"媒体库中没有图片"错误会在 push 时自动解决，因此从结果中剔除并列入 local_images。
func (a *app) render(ctx context.Context, k kind, path string) error {
	data, err := readInput(a, path)
	if err != nil {
		return err
	}
	baseDir := filepath.Dir(path)
	var body string
	if len(data) >= len(fence) && string(data[:len(fence)]) == fence {
		var meta any = &pageMeta{}
		if k == postKind {
			meta = &postMeta{}
		}
		if body, err = parseDocument(data, meta); err != nil {
			return err
		}
	} else {
		body = string(data)
	}
	images, err := localImages(body, baseDir)
	if err != nil {
		return err
	}
	found, err := a.lookupMedia(ctx, shasOf(images))
	if err != nil {
		return err
	}
	pendingLines := map[int]bool{}
	pending := []*localImage{}
	for _, img := range images {
		if _, ok := found[img.SHA256]; !ok {
			pending = append(pending, img)
			for _, l := range img.lines {
				pendingLines[l] = true
			}
		}
	}

	r, err := a.api.do(ctx, request{method: http.MethodPost, path: "/api/admin/render",
		body: map[string]string{"body_md": rewriteLocalImages(body, baseDir, images)}})
	if err != nil {
		return err
	}
	var preview map[string]any
	if err := r.decode(&preview); err != nil {
		return err
	}
	issues, _ := preview["issues"].([]any)
	kept := []any{}
	hasError := false
	for _, it := range issues {
		is, _ := it.(map[string]any)
		line, _ := is["line"].(float64)
		if is["severity"] == "error" && pendingLines[int(line)] {
			continue
		}
		if is["severity"] == "error" {
			hasError = true
		}
		kept = append(kept, it)
	}
	preview["issues"] = kept
	preview["local_images"] = pending
	if err := a.out(preview); err != nil {
		return err
	}
	if hasError {
		return &problem{Type: "blog-cli:render-error", Title: "正文渲染出错", Detail: "见 stdout 中的 issues", exit: exitInvalid}
	}
	return nil
}

func shasOf(images []*localImage) []string {
	out := make([]string, 0, len(images))
	for _, img := range images {
		out = append(out, img.SHA256)
	}
	return out
}
