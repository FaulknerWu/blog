package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// resolveMedia 把媒体 ID 或 sha256 解析为媒体 ID。
func (a *app) resolveMedia(ctx context.Context, ref string) (int64, error) {
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil {
		return id, nil
	}
	if m := mediaRef.FindStringSubmatch("/media/" + ref); m != nil {
		found, err := a.lookupMedia(ctx, []string{m[1]})
		if err != nil {
			return 0, err
		}
		if id, ok := found[m[1]]; ok {
			return id, nil
		}
		return 0, notFoundError("媒体库中没有 sha256 为 %s 的图片", ref)
	}
	return 0, usageError("图片需用媒体 ID 或 64 位 sha256 指定")
}

func mediaCommands() []command {
	return []command{
		{name: "media list", summary: "列出媒体库图片", setup: func(fs *flag.FlagSet) runFunc {
			unused := fs.Bool("unused", false, "只列出未被任何内容引用的图片")
			limit := fs.Int("limit", 0, "每页条数")
			cursor := fs.String("cursor", "", "上一页返回的 next_cursor")
			all := fs.Bool("all", false, "自动翻页取回全部")
			return func(ctx context.Context, a *app, args []string) error {
				q := url.Values{}
				if *unused {
					q.Set("unused", "true")
				}
				if *limit > 0 {
					q.Set("limit", strconv.Itoa(*limit))
				}
				setNonEmpty(q, "cursor", *cursor)
				body, err := a.api.list(ctx, "/api/admin/media", q, *all)
				if err != nil {
					return err
				}
				return a.out(body)
			}
		}},
		{name: "media get", args: "<id|sha256>", summary: "输出图片详情（含引用它的内容）", setup: func(fs *flag.FlagSet) runFunc {
			return func(ctx context.Context, a *app, args []string) error {
				if err := needArgs(args, 1, "<id|sha256>"); err != nil {
					return err
				}
				id, err := a.resolveMedia(ctx, args[0])
				if err != nil {
					return err
				}
				r, err := a.api.get(ctx, fmt.Sprintf("/api/admin/media/%d", id), nil)
				if err != nil {
					return err
				}
				return a.out(r.body)
			}
		}},
		{name: "media upload", args: "<file>...", summary: "上传图片（按内容去重，已存在时返回已有记录），在正文中以 /media/{sha256} 引用", setup: func(fs *flag.FlagSet) runFunc {
			alt := fs.String("alt", "", "默认替代文本")
			return func(ctx context.Context, a *app, args []string) error {
				if len(args) == 0 {
					return usageError("需要参数 <file>...")
				}
				items := make([]json.RawMessage, 0, len(args))
				for _, path := range args {
					r, err := a.api.upload(ctx, path, *alt)
					if err != nil {
						return err
					}
					items = append(items, r.body)
				}
				return a.out(map[string]any{"items": items})
			}
		}},
		{name: "media update", args: "<id|sha256>", summary: "修改图片的默认替代文本", setup: func(fs *flag.FlagSet) runFunc {
			alt := fs.String("alt", "", "新的替代文本")
			return func(ctx context.Context, a *app, args []string) error {
				if err := needArgs(args, 1, "<id|sha256>"); err != nil {
					return err
				}
				id, err := a.resolveMedia(ctx, args[0])
				if err != nil {
					return err
				}
				r, err := a.api.do(ctx, request{method: http.MethodPatch, path: fmt.Sprintf("/api/admin/media/%d", id),
					body: map[string]string{"alt": *alt}})
				if err != nil {
					return err
				}
				return a.out(r.body)
			}
		}},
		{name: "media delete", args: "<id|sha256>", summary: "删除图片（仍被引用时返回冲突）", setup: func(fs *flag.FlagSet) runFunc {
			yes := fs.Bool("yes", false, "确认删除")
			return func(ctx context.Context, a *app, args []string) error {
				if err := needArgs(args, 1, "<id|sha256>"); err != nil {
					return err
				}
				if err := requireYes(*yes, "删除图片"); err != nil {
					return err
				}
				id, err := a.resolveMedia(ctx, args[0])
				if err != nil {
					return err
				}
				if _, err := a.api.do(ctx, request{method: http.MethodDelete, path: fmt.Sprintf("/api/admin/media/%d", id)}); err != nil {
					return err
				}
				return a.outDeleted(id)
			}
		}},
	}
}
