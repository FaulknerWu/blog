package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"blog/api/internal/httpx"
	"blog/api/internal/store"
)

// Scope 限定 API 令牌可执行的操作。浏览器会话拥有全部权限；账户、会话与令牌管理只接受浏览器会话。
type Scope string

const (
	ScopeContentRead    Scope = "content:read"    // 读取后台内容（文章、页面、标签、专栏、媒体、站点设置）
	ScopeContentWrite   Scope = "content:write"   // 创建与修改文章、页面草稿，管理标签与专栏
	ScopeContentPublish Scope = "content:publish" // 发布、撤回、删除文章与页面
	ScopeMediaWrite     Scope = "media:write"     // 上传、修改、删除图片
	ScopeSettingsWrite  Scope = "settings:write"  // 修改站点设置
)

var Scopes = []Scope{ScopeContentRead, ScopeContentWrite, ScopeContentPublish, ScopeMediaWrite, ScopeSettingsWrite}

// TokenPrefix 让令牌在日志与代码中一眼可辨，也便于密钥扫描工具识别。
const TokenPrefix = "blog_pat_"

const maxTokenName = 100

type Via int

const (
	ViaSession Via = iota + 1
	ViaToken
)

// Can 判断主体是否拥有 scope；浏览器会话不受 scope 限制。
func (p *Principal) Can(scope Scope) bool {
	return p.Via == ViaSession || slices.Contains(p.Scopes, string(scope))
}

type TokenInput struct {
	Name      string     `json:"name"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func validateToken(in TokenInput) error {
	var v httpx.Validator
	v.Length("#/name", strings.TrimSpace(in.Name), 1, maxTokenName)
	if len(in.Scopes) == 0 {
		v.Add("#/scopes", "required", "至少选择一项权限")
	}
	for i, s := range in.Scopes {
		if !slices.Contains(Scopes, Scope(s)) {
			v.Add(fmt.Sprintf("#/scopes/%d", i), "invalid_value", "未知的权限")
		}
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(time.Now()) {
		v.Add("#/expires_at", "out_of_range", "过期时间必须晚于当前时间")
	}
	return v.Err()
}

// CreateToken 为用户签发 API 令牌，明文只在此时返回一次，数据库只保存哈希。
func (s *Service) CreateToken(ctx context.Context, userID int64, in TokenInput) (string, store.ApiToken, error) {
	if err := validateToken(in); err != nil {
		return "", store.ApiToken{}, err
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", store.ApiToken{}, err
	}
	secret := TokenPrefix + base64.RawURLEncoding.EncodeToString(raw[:])
	scopes := slices.Compact(slices.Sorted(slices.Values(in.Scopes)))
	t, err := s.store.CreateAPIToken(ctx, store.CreateAPITokenParams{
		UserID:    userID,
		Name:      strings.TrimSpace(in.Name),
		TokenHash: hashToken(secret),
		Scopes:    scopes,
		ExpiresAt: in.ExpiresAt,
	})
	if store.ConstraintViolation(err) == "api_tokens_user_name_key" {
		return "", store.ApiToken{}, httpx.Conflict("同名令牌已存在",
			httpx.FieldError{Pointer: "#/name", Code: "duplicate", Detail: "同名令牌已存在"})
	}
	return secret, t, err
}

// CreateTokenFor 供服务端运维命令按用户名签发令牌。
func (s *Service) CreateTokenFor(ctx context.Context, username string, in TokenInput) (string, store.ApiToken, error) {
	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return "", store.ApiToken{}, fmt.Errorf("find user %q: %w", username, err)
	}
	return s.CreateToken(ctx, user.ID, in)
}

func (s *Service) authenticateToken(ctx context.Context, secret string) (*Principal, error) {
	if !strings.HasPrefix(secret, TokenPrefix) || utf8.RuneCountInString(secret) > 128 {
		return nil, httpx.Unauthenticated("令牌格式不正确")
	}
	row, err := s.store.GetActiveAPITokenByHash(ctx, hashToken(secret))
	if store.IsNotFound(err) {
		return nil, httpx.Unauthenticated("令牌无效或已过期")
	}
	if err != nil {
		return nil, err
	}
	if row.LastUsedAt == nil || time.Since(*row.LastUsedAt) >= touchInterval {
		if err := s.store.TouchAPIToken(ctx, row.ID); err != nil {
			return nil, err
		}
	}
	return &Principal{
		UserID:      row.UserID,
		Username:    row.Username,
		DisplayName: row.DisplayName,
		Via:         ViaToken,
		TokenID:     row.ID,
		Scopes:      row.Scopes,
	}, nil
}

func (s *Service) ListTokens(ctx context.Context, userID int64) ([]store.ApiToken, error) {
	return s.store.ListAPITokens(ctx, userID)
}

func (s *Service) ListTokensFor(ctx context.Context, username string) ([]store.ApiToken, error) {
	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("find user %q: %w", username, err)
	}
	return s.store.ListAPITokens(ctx, user.ID)
}

func (s *Service) RevokeToken(ctx context.Context, userID, tokenID int64) error {
	n, err := s.store.DeleteAPIToken(ctx, store.DeleteAPITokenParams{ID: tokenID, UserID: userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return httpx.NotFound("令牌不存在")
	}
	return nil
}

func (s *Service) RevokeTokenFor(ctx context.Context, username, name string) error {
	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("find user %q: %w", username, err)
	}
	n, err := s.store.DeleteAPITokenByName(ctx, store.DeleteAPITokenByNameParams{Name: name, UserID: user.ID})
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("token %q not found", name)
	}
	return nil
}
