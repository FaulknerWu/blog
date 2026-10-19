// Command blog 是博客的命令行客户端：以 API 令牌远程管理文章、页面、标签、专栏、媒体与站点设置，
// 输出 JSON 与稳定的退出码，便于 agent 调用。服务端运维见 cmd/blogd。
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"blog/api/internal/cli"
)

// version 在构建时通过 -ldflags "-X main.version=…" 注入。
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Run(ctx, os.Args[1:], cli.Env{
		Stdin:   os.Stdin,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Getenv:  os.Getenv,
		Version: version,
	})
	stop()
	os.Exit(code)
}
