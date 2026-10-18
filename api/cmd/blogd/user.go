package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"blog/api/internal/auth"
	"blog/api/internal/store"
)

func userCreate(ctx context.Context, args []string) error {
	fs := newFlags("user create")
	username := fs.String("username", "", "登录用户名（^[a-z0-9_]{3,32}$）")
	displayName := fs.String("display-name", "", "展示名称，默认同用户名")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *username == "" {
		return errors.New("-username is required")
	}
	if *displayName == "" {
		*displayName = *username
	}
	pw, err := readPassword()
	if err != nil {
		return err
	}
	return withStore(ctx, func(s *store.Store) error {
		u, err := auth.NewService(s).CreateUser(ctx, *username, *displayName, pw)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "created user %q (id %d)\n", u.Username, u.ID)
		return nil
	})
}

func userResetPassword(ctx context.Context, args []string) error {
	fs := newFlags("user reset-password")
	username := fs.String("username", "", "登录用户名")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *username == "" {
		return errors.New("-username is required")
	}
	pw, err := readPassword()
	if err != nil {
		return err
	}
	return withStore(ctx, func(s *store.Store) error {
		if err := auth.NewService(s).ResetPassword(ctx, *username, pw); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "password reset for %q; all sessions and API tokens revoked\n", *username)
		return nil
	})
}

// readPassword 优先读取 BLOG_PASSWORD；否则从标准输入读一行，是终端时不回显。
func readPassword() (string, error) {
	if pw := os.Getenv("BLOG_PASSWORD"); pw != "" {
		return pw, nil
	}
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, "password: ")
		pw, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}
		return string(pw), nil
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("read password: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}
