package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"blog/api/internal/app"
	"blog/api/internal/auth"
	"blog/api/internal/store"
)

const usage = `usage: blog-api <command> [flags]

commands:
  serve                              启动 HTTP 服务（默认）
  migrate                            执行数据库迁移
  user create -username U -display-name N
                                     创建作者账号（密码读取自 BLOG_PASSWORD 或标准输入）
  user reset-password -username U    重置密码并吊销全部会话
  healthcheck [-url URL]             请求 /healthz，供容器健康检查使用
`

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cmd := "serve"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	switch cmd {
	case "serve":
		return serve()
	case "migrate":
		return withStore(func(ctx context.Context, s *store.Store) error { return s.Migrate(ctx) })
	case "user":
		return userCmd(args)
	case "healthcheck":
		return healthcheck(args)
	case "-h", "--help", "help":
		fmt.Print(usage)
		return nil
	default:
		fmt.Fprint(os.Stderr, usage)
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func serve() error {
	cfg, err := app.LoadConfig()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	s, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer s.Close()
	if cfg.MigrateOnStart {
		if err := s.Migrate(ctx); err != nil {
			return err
		}
	}
	a, err := app.New(cfg, s)
	if err != nil {
		return err
	}
	return a.Run(ctx)
}

func withStore(fn func(context.Context, *store.Store) error) error {
	dsn := os.Getenv("BLOG_DATABASE_URL")
	if dsn == "" {
		return errors.New("BLOG_DATABASE_URL is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	s, err := store.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer s.Close()
	return fn(ctx, s)
}

func readPassword() (string, error) {
	if pw := os.Getenv("BLOG_PASSWORD"); pw != "" {
		return pw, nil
	}
	fmt.Fprint(os.Stderr, "password: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("read password: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func userCmd(args []string) error {
	if len(args) == 0 {
		return errors.New("user: missing subcommand (create | reset-password)")
	}
	sub, args := args[0], args[1:]
	fs := flag.NewFlagSet("user "+sub, flag.ContinueOnError)
	username := fs.String("username", "", "登录用户名（^[a-z0-9_]{3,32}$）")
	displayName := fs.String("display-name", "", "展示名称")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *username == "" {
		return errors.New("-username is required")
	}
	switch sub {
	case "create":
		if *displayName == "" {
			*displayName = *username
		}
		pw, err := readPassword()
		if err != nil {
			return err
		}
		return withStore(func(ctx context.Context, s *store.Store) error {
			if err := s.Migrate(ctx); err != nil {
				return err
			}
			u, err := auth.NewService(s).CreateUser(ctx, *username, *displayName, pw)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "created user %q (id %d)\n", u.Username, u.ID)
			return nil
		})
	case "reset-password":
		pw, err := readPassword()
		if err != nil {
			return err
		}
		return withStore(func(ctx context.Context, s *store.Store) error {
			if err := auth.NewService(s).ResetPassword(ctx, *username, pw); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "password reset for %q; all sessions revoked\n", *username)
			return nil
		})
	default:
		return fmt.Errorf("user: unknown subcommand %q", sub)
	}
}

func healthcheck(args []string) error {
	fs := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	url := fs.String("url", "http://127.0.0.1:8080/healthz", "健康检查地址")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(*url)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: %s", resp.Status)
	}
	return nil
}
