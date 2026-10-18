// Command blogd 是博客的服务端程序：运行 HTTP 服务，以及只能在服务器本地执行的运维命令（直连数据库）。
// 内容操作不在这里，远程客户端见 cmd/blog。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

type command struct {
	name    string // 子命令用空格分隔，如 "user create"
	summary string
	run     func(ctx context.Context, args []string) error
}

var commands = []command{
	{"serve", "启动 HTTP 服务", serve},
	{"migrate", "执行数据库迁移", migrate},
	{"rerender", "用当前渲染器重新渲染过期的文章与页面", rerender},
	{"user create", "创建作者账号（密码读取自 BLOG_PASSWORD 或标准输入）", userCreate},
	{"user reset-password", "重置密码，并吊销全部会话与 API 令牌", userResetPassword},
	{"token create", "签发 API 令牌（明文只输出一次）", tokenCreate},
	{"token list", "列出 API 令牌", tokenList},
	{"token revoke", "吊销 API 令牌", tokenRevoke},
	{"healthcheck", "请求 /healthz，供容器健康检查使用", healthcheck},
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:])
	stop()
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printUsage(os.Stdout)
		if len(args) == 0 {
			return errors.New("missing command")
		}
		return nil
	}
	// 优先匹配两级子命令（如 "user create"），再匹配一级命令。
	for _, n := range []int{2, 1} {
		if len(args) < n {
			continue
		}
		name := strings.Join(args[:n], " ")
		for _, c := range commands {
			if c.name == name {
				return c.run(ctx, args[n:])
			}
		}
	}
	printUsage(os.Stderr)
	return fmt.Errorf("unknown command %q", strings.Join(args, " "))
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: blogd <command> [flags]\n\ncommands:")
	for _, c := range commands {
		fmt.Fprintf(w, "  %-22s %s\n", c.name, c.summary)
	}
	fmt.Fprintln(w, "\n各命令的参数见 blogd <command> -h")
}

// newFlags 创建子命令的参数集，-h 时打印用法并返回 flag.ErrHelp。
func newFlags(name string) *flag.FlagSet {
	return flag.NewFlagSet("blogd "+name, flag.ContinueOnError)
}
