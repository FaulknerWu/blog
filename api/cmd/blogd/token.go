package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"blog/api/internal/auth"
	"blog/api/internal/store"
)

func tokenCreate(ctx context.Context, args []string) error {
	fs := newFlags("token create")
	username := fs.String("username", "", "令牌所属用户")
	name := fs.String("name", "", "令牌名称，同一用户内唯一（如 claude-code）")
	scopes := fs.String("scope", "", "逗号分隔的权限："+scopeList())
	ttl := fs.Duration("expires-in", 0, "有效期（如 2160h），0 表示不过期")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *username == "" || *name == "" || *scopes == "" {
		return errors.New("-username, -name and -scope are required")
	}
	in := auth.TokenInput{Name: *name, Scopes: strings.Split(*scopes, ",")}
	if *ttl > 0 {
		exp := time.Now().Add(*ttl)
		in.ExpiresAt = &exp
	}
	return withStore(ctx, func(s *store.Store) error {
		secret, t, err := auth.NewService(s).CreateTokenFor(ctx, *username, in)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "created token %q (id %d) with scopes %s\n", t.Name, t.ID, strings.Join(t.Scopes, ","))
		fmt.Println(secret)
		return nil
	})
}

func tokenList(ctx context.Context, args []string) error {
	fs := newFlags("token list")
	username := fs.String("username", "", "令牌所属用户")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *username == "" {
		return errors.New("-username is required")
	}
	return withStore(ctx, func(s *store.Store) error {
		tokens, err := auth.NewService(s).ListTokensFor(ctx, *username)
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tSCOPES\tLAST USED\tEXPIRES")
		for _, t := range tokens {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", t.ID, t.Name, strings.Join(t.Scopes, ","),
				formatTime(t.LastUsedAt, "never"), formatTime(t.ExpiresAt, "never"))
		}
		return w.Flush()
	})
}

func tokenRevoke(ctx context.Context, args []string) error {
	fs := newFlags("token revoke")
	username := fs.String("username", "", "令牌所属用户")
	name := fs.String("name", "", "令牌名称")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *username == "" || *name == "" {
		return errors.New("-username and -name are required")
	}
	return withStore(ctx, func(s *store.Store) error {
		if err := auth.NewService(s).RevokeTokenFor(ctx, *username, *name); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "revoked token %q\n", *name)
		return nil
	})
}

func scopeList() string {
	names := make([]string, 0, len(auth.Scopes))
	for _, s := range auth.Scopes {
		names = append(names, string(s))
	}
	return strings.Join(names, ", ")
}

func formatTime(t *time.Time, zero string) string {
	if t == nil {
		return zero
	}
	return t.Local().Format(time.DateTime)
}
