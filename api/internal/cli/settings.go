package cli

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
)

func settingsCommands() []command {
	return []command{
		{name: "settings get", summary: "输出站点设置", setup: func(fs *flag.FlagSet) runFunc {
			return func(ctx context.Context, a *app, args []string) error {
				r, err := a.api.get(ctx, "/api/admin/settings", nil)
				if err != nil {
					return err
				}
				return a.out(r.body)
			}
		}},
		{name: "settings update", args: "<json|->", summary: "以 JSON 对象部分更新站点设置（需要 settings:write），如 '{\"title\":\"新标题\"}'", setup: func(fs *flag.FlagSet) runFunc {
			return func(ctx context.Context, a *app, args []string) error {
				if err := needArgs(args, 1, "<json|->"); err != nil {
					return err
				}
				data := []byte(args[0])
				if args[0] == "-" {
					var err error
					if data, err = readInput(a, "-"); err != nil {
						return err
					}
				}
				var patch map[string]json.RawMessage
				if err := json.Unmarshal(data, &patch); err != nil {
					return invalidError("设置必须是 JSON 对象：%v", err)
				}
				r, err := a.api.do(ctx, request{method: http.MethodPatch, path: "/api/admin/settings", body: patch})
				if err != nil {
					return err
				}
				return a.out(r.body)
			}
		}},
	}
}
