package cli

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"strconv"
)

type taxon struct {
	ID   int64  `json:"id"`
	Slug string `json:"slug"`
}

// resolveTaxon 在标签或专栏的完整列表中按 id 或 slug 查找。
func (a *app) resolveTaxon(ctx context.Context, path, label, ref string) (int64, error) {
	var page struct {
		Items []taxon `json:"items"`
	}
	if err := a.api.getJSON(ctx, path, nil, &page); err != nil {
		return 0, err
	}
	id, _ := strconv.ParseInt(ref, 10, 64)
	for _, t := range page.Items {
		if t.Slug == ref || (id != 0 && t.ID == id) {
			return t.ID, nil
		}
	}
	return 0, notFoundError("没有%s %q；可先运行 blog %s create", label, ref, map[string]string{"标签": "tag", "专栏": "series"}[label])
}

func (a *app) resolveTag(ctx context.Context, ref string) (int64, error) {
	return a.resolveTaxon(ctx, "/api/admin/tags", "标签", ref)
}

func (a *app) resolveSeries(ctx context.Context, ref string) (int64, error) {
	return a.resolveTaxon(ctx, "/api/admin/series", "专栏", ref)
}

func (a *app) seriesSlug(ctx context.Context, id int64) (string, error) {
	var page struct {
		Items []taxon `json:"items"`
	}
	if err := a.api.getJSON(ctx, "/api/admin/series", nil, &page); err != nil {
		return "", err
	}
	for _, t := range page.Items {
		if t.ID == id {
			return t.Slug, nil
		}
	}
	return "", notFoundError("专栏 %d 不存在", id)
}

// optionalString 区分“未传”与“传了空字符串”，用于 PATCH 只提交显式给出的字段。
type optionalString struct {
	set   bool
	value string
}

func (o *optionalString) String() string { return o.value }
func (o *optionalString) Set(v string) error {
	o.set, o.value = true, v
	return nil
}

func patchBody(fields map[string]*optionalString) map[string]any {
	body := map[string]any{}
	for k, v := range fields {
		if v.set {
			body[k] = v.value
		}
	}
	return body
}

func (a *app) outDeleted(id int64) error {
	return a.out(map[string]any{"id": id, "deleted": true})
}

func tagCommands() []command {
	return []command{
		{name: "tag list", summary: "列出标签（含文章数）", setup: func(fs *flag.FlagSet) runFunc {
			return func(ctx context.Context, a *app, args []string) error {
				r, err := a.api.get(ctx, "/api/admin/tags", nil)
				if err != nil {
					return err
				}
				return a.out(r.body)
			}
		}},
		{name: "tag create", summary: "创建标签", setup: func(fs *flag.FlagSet) runFunc {
			name := fs.String("name", "", "标签名称")
			slug := fs.String("slug", "", "标签 slug")
			return func(ctx context.Context, a *app, args []string) error {
				if *name == "" || *slug == "" {
					return usageError("需要 --name 与 --slug")
				}
				r, err := a.api.do(ctx, request{method: http.MethodPost, path: "/api/admin/tags",
					body: map[string]string{"name": *name, "slug": *slug}})
				if err != nil {
					return err
				}
				return a.out(r.body)
			}
		}},
		{name: "tag update", args: "<id|slug>", summary: "修改标签名称或 slug", setup: func(fs *flag.FlagSet) runFunc {
			var name, slug optionalString
			fs.Var(&name, "name", "新名称")
			fs.Var(&slug, "slug", "新 slug")
			return func(ctx context.Context, a *app, args []string) error {
				if err := needArgs(args, 1, "<id|slug>"); err != nil {
					return err
				}
				id, err := a.resolveTag(ctx, args[0])
				if err != nil {
					return err
				}
				r, err := a.api.do(ctx, request{method: http.MethodPatch, path: fmt.Sprintf("/api/admin/tags/%d", id),
					body: patchBody(map[string]*optionalString{"name": &name, "slug": &slug})})
				if err != nil {
					return err
				}
				return a.out(r.body)
			}
		}},
		{name: "tag delete", args: "<id|slug>", summary: "删除标签（文章上的该标签一并移除）", setup: func(fs *flag.FlagSet) runFunc {
			yes := fs.Bool("yes", false, "确认删除")
			return func(ctx context.Context, a *app, args []string) error {
				if err := needArgs(args, 1, "<id|slug>"); err != nil {
					return err
				}
				if err := requireYes(*yes, "删除标签"); err != nil {
					return err
				}
				id, err := a.resolveTag(ctx, args[0])
				if err != nil {
					return err
				}
				if _, err := a.api.do(ctx, request{method: http.MethodDelete, path: fmt.Sprintf("/api/admin/tags/%d", id)}); err != nil {
					return err
				}
				return a.outDeleted(id)
			}
		}},
	}
}

func seriesCommands() []command {
	return []command{
		{name: "series list", summary: "列出专栏", setup: func(fs *flag.FlagSet) runFunc {
			return func(ctx context.Context, a *app, args []string) error {
				r, err := a.api.get(ctx, "/api/admin/series", nil)
				if err != nil {
					return err
				}
				return a.out(r.body)
			}
		}},
		{name: "series get", args: "<id|slug>", summary: "输出专栏详情（含全部成员文章及顺序）", setup: func(fs *flag.FlagSet) runFunc {
			return func(ctx context.Context, a *app, args []string) error {
				if err := needArgs(args, 1, "<id|slug>"); err != nil {
					return err
				}
				id, err := a.resolveSeries(ctx, args[0])
				if err != nil {
					return err
				}
				r, err := a.api.get(ctx, fmt.Sprintf("/api/admin/series/%d", id), nil)
				if err != nil {
					return err
				}
				return a.out(r.body)
			}
		}},
		{name: "series create", summary: "创建专栏", setup: func(fs *flag.FlagSet) runFunc {
			title := fs.String("title", "", "专栏标题")
			slug := fs.String("slug", "", "专栏 slug")
			desc := fs.String("description", "", "专栏简介")
			return func(ctx context.Context, a *app, args []string) error {
				if *title == "" || *slug == "" {
					return usageError("需要 --title 与 --slug")
				}
				r, err := a.api.do(ctx, request{method: http.MethodPost, path: "/api/admin/series",
					body: map[string]string{"title": *title, "slug": *slug, "description": *desc}})
				if err != nil {
					return err
				}
				return a.out(r.body)
			}
		}},
		{name: "series update", args: "<id|slug>", summary: "修改专栏标题、slug 或简介", setup: func(fs *flag.FlagSet) runFunc {
			var title, slug, desc optionalString
			fs.Var(&title, "title", "新标题")
			fs.Var(&slug, "slug", "新 slug")
			fs.Var(&desc, "description", "新简介")
			return func(ctx context.Context, a *app, args []string) error {
				if err := needArgs(args, 1, "<id|slug>"); err != nil {
					return err
				}
				id, err := a.resolveSeries(ctx, args[0])
				if err != nil {
					return err
				}
				r, err := a.api.do(ctx, request{method: http.MethodPatch, path: fmt.Sprintf("/api/admin/series/%d", id),
					body: patchBody(map[string]*optionalString{"title": &title, "slug": &slug, "description": &desc})})
				if err != nil {
					return err
				}
				return a.out(r.body)
			}
		}},
		{name: "series delete", args: "<id|slug>", summary: "删除专栏（成员文章移出专栏）", setup: func(fs *flag.FlagSet) runFunc {
			yes := fs.Bool("yes", false, "确认删除")
			return func(ctx context.Context, a *app, args []string) error {
				if err := needArgs(args, 1, "<id|slug>"); err != nil {
					return err
				}
				if err := requireYes(*yes, "删除专栏"); err != nil {
					return err
				}
				id, err := a.resolveSeries(ctx, args[0])
				if err != nil {
					return err
				}
				if _, err := a.api.do(ctx, request{method: http.MethodDelete, path: fmt.Sprintf("/api/admin/series/%d", id)}); err != nil {
					return err
				}
				return a.outDeleted(id)
			}
		}},
		{name: "series reorder", args: "<id|slug> <post>...", summary: "按给定顺序排列专栏文章（必须列出全部成员，文章用 id 或 slug）", setup: func(fs *flag.FlagSet) runFunc {
			return func(ctx context.Context, a *app, args []string) error {
				if len(args) < 2 {
					return usageError("需要参数 <id|slug> <post>...")
				}
				id, err := a.resolveSeries(ctx, args[0])
				if err != nil {
					return err
				}
				postIDs := make([]int64, 0, len(args)-1)
				for _, ref := range args[1:] {
					pid, err := a.resolveContent(ctx, postKind, ref)
					if err != nil {
						return err
					}
					postIDs = append(postIDs, pid)
				}
				r, err := a.api.do(ctx, request{method: http.MethodPut, path: fmt.Sprintf("/api/admin/series/%d/posts", id),
					body: map[string]any{"post_ids": postIDs}})
				if err != nil {
					return err
				}
				return a.out(r.body)
			}
		}},
	}
}
