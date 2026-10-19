package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

func authCommands() []command {
	return []command{
		{name: "auth login", summary: "保存站点地址与 API 令牌（令牌读取自 BLOG_TOKEN 或标准输入）并校验", offline: true,
			setup: func(fs *flag.FlagSet) runFunc {
				siteURL := fs.String("url", "", "站点地址，如 https://blog.example.com")
				return func(ctx context.Context, a *app, args []string) error {
					if *siteURL == "" {
						return usageError("需要 --url")
					}
					token, err := a.readToken()
					if err != nil {
						return err
					}
					p := profile{URL: strings.TrimRight(*siteURL, "/"), Token: token}
					a.api = &client{base: p.URL, token: p.Token, http: a.env.HTTP}
					me, err := a.me(ctx)
					if err != nil {
						return err
					}
					cfg, err := a.loadConfig()
					if err != nil {
						return err
					}
					name := a.profileName(cfg)
					cfg.Profiles[name] = p
					if cfg.Current == "" {
						cfg.Current = name
					}
					if err := a.saveConfig(cfg); err != nil {
						return err
					}
					return a.out(map[string]any{"profile": name, "url": p.URL, "me": me})
				}
			}},
		{name: "auth status", summary: "输出当前凭据对应的用户与权限", setup: func(fs *flag.FlagSet) runFunc {
			return func(ctx context.Context, a *app, args []string) error {
				me, err := a.me(ctx)
				if err != nil {
					return err
				}
				cfg, err := a.loadConfig()
				if err != nil {
					return err
				}
				return a.out(map[string]any{"profile": a.profileName(cfg), "url": a.api.base, "me": me})
			}
		}},
		{name: "auth logout", summary: "从本地配置删除 profile（令牌本身需在后台吊销）", offline: true,
			setup: func(fs *flag.FlagSet) runFunc {
				return func(ctx context.Context, a *app, args []string) error {
					cfg, err := a.loadConfig()
					if err != nil {
						return err
					}
					name := a.profileName(cfg)
					if _, ok := cfg.Profiles[name]; !ok {
						return notFoundError("没有 profile %q", name)
					}
					delete(cfg.Profiles, name)
					if cfg.Current == name {
						cfg.Current = ""
					}
					if err := a.saveConfig(cfg); err != nil {
						return err
					}
					return a.out(map[string]any{"profile": name, "removed": true,
						"note": "令牌仍然有效，如不再使用请在后台账户页吊销"})
				}
			}},
	}
}

func (a *app) me(ctx context.Context) (json.RawMessage, error) {
	r, err := a.api.get(ctx, "/api/auth/me", nil)
	if err != nil {
		return nil, err
	}
	return r.body, nil
}

// readToken 优先读取 BLOG_TOKEN；否则从标准输入读一行，是终端时不回显。
func (a *app) readToken() (string, error) {
	if t := a.env.Getenv("BLOG_TOKEN"); t != "" {
		return t, nil
	}
	if f, ok := a.env.Stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		fmt.Fprint(a.env.Stderr, "token: ")
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(a.env.Stderr)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	line, err := bufio.NewReader(a.env.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", usageError("从标准输入读取令牌失败：%v", err)
	}
	if t := strings.TrimSpace(line); t != "" {
		return t, nil
	}
	return "", usageError("令牌为空")
}
