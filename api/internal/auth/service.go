package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"time"
	"unicode/utf8"

	"blog/api/internal/httpx"
	"blog/api/internal/store"
)

const (
	SessionTTL    = 30 * 24 * time.Hour // 滑动有效期：每次活跃都顺延
	SessionMaxAge = 90 * 24 * time.Hour // 绝对上限：自创建起无论是否活跃都不超过
	touchInterval = time.Minute         // 最多每分钟写一次 last_seen_at，减少写库

	minPasswordLen = 12
	maxPasswordLen = 1024
)

// Principal 是通过鉴权的请求主体，来自浏览器会话（Via=ViaSession）或 API 令牌（Via=ViaToken）。
type Principal struct {
	UserID      int64
	Username    string
	DisplayName string
	Via         Via

	SessionID int64 // 仅会话
	ExpiresAt time.Time
	Refreshed bool

	TokenID int64 // 仅令牌
	Scopes  []string
}

type Service struct {
	store *store.Store
}

func NewService(s *store.Store) *Service { return &Service{store: s} }

// hashToken 返回 token 的 SHA-256。数据库只存哈希，即使数据泄露也无法直接冒用会话。
func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func sessionExpiry(createdAt, now time.Time) time.Time {
	if limit := createdAt.Add(SessionMaxAge); now.Add(SessionTTL).After(limit) {
		return limit
	}
	return now.Add(SessionTTL)
}

func (s *Service) Login(ctx context.Context, username, password, userAgent string, ip netip.Addr) (string, store.User, store.Session, error) {
	invalid := httpx.Unauthenticated("用户名或密码错误")
	user, err := s.store.GetUserByUsername(ctx, username)
	if store.IsNotFound(err) {
		_, _ = VerifyPassword(password, dummyHash())
		return "", store.User{}, store.Session{}, invalid
	}
	if err != nil {
		return "", store.User{}, store.Session{}, err
	}
	ok, err := VerifyPassword(password, user.PasswordHash)
	if err != nil {
		return "", store.User{}, store.Session{}, fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return "", store.User{}, store.Session{}, invalid
	}

	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", store.User{}, store.Session{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	var ipPtr *netip.Addr
	if ip.IsValid() {
		ipPtr = &ip
	}
	if utf8.RuneCountInString(userAgent) > 512 {
		userAgent = string([]rune(userAgent)[:512])
	}
	sess, err := s.store.CreateSession(ctx, store.CreateSessionParams{
		TokenHash: hashToken(token),
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(SessionTTL),
		UserAgent: userAgent,
		Ip:        ipPtr,
	})
	if err != nil {
		return "", store.User{}, store.Session{}, err
	}
	return token, user, sess, nil
}

func (s *Service) authenticateSession(ctx context.Context, token string) (*Principal, error) {
	if token == "" {
		return nil, httpx.Unauthenticated("请先登录")
	}
	row, err := s.store.GetActiveSessionByTokenHash(ctx, hashToken(token))
	if store.IsNotFound(err) {
		return nil, httpx.Unauthenticated("会话已失效，请重新登录")
	}
	if err != nil {
		return nil, err
	}
	p := &Principal{
		UserID:      row.UserID,
		Username:    row.Username,
		DisplayName: row.DisplayName,
		Via:         ViaSession,
		SessionID:   row.ID,
		ExpiresAt:   row.ExpiresAt,
	}
	now := time.Now()
	if now.Sub(row.LastSeenAt) >= touchInterval {
		exp := sessionExpiry(row.CreatedAt, now)
		if err := s.store.TouchSession(ctx, store.TouchSessionParams{ID: row.ID, ExpiresAt: exp}); err != nil {
			return nil, err
		}
		p.ExpiresAt, p.Refreshed = exp, true
	}
	return p, nil
}

func (s *Service) Logout(ctx context.Context, p *Principal) error {
	_, err := s.store.DeleteUserSession(ctx, store.DeleteUserSessionParams{ID: p.SessionID, UserID: p.UserID})
	return err
}

func (s *Service) ListSessions(ctx context.Context, userID int64) ([]store.Session, error) {
	return s.store.ListActiveSessions(ctx, userID)
}

func (s *Service) RevokeSession(ctx context.Context, userID, sessionID int64) error {
	n, err := s.store.DeleteUserSession(ctx, store.DeleteUserSessionParams{ID: sessionID, UserID: userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return httpx.NotFound("会话不存在")
	}
	return nil
}

func (s *Service) ChangePassword(ctx context.Context, p *Principal, current, next string) error {
	var v httpx.Validator
	if current == "" {
		v.Add("#/current_password", "required", "不能为空")
	}
	validateNewPassword(&v, "#/new_password", next)
	if err := v.Err(); err != nil {
		return err
	}
	user, err := s.store.GetUserByID(ctx, p.UserID)
	if err != nil {
		return err
	}
	ok, err := VerifyPassword(current, user.PasswordHash)
	if err != nil {
		return fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return httpx.Validation(httpx.FieldError{Pointer: "#/current_password", Code: "incorrect", Detail: "当前密码不正确"})
	}
	hash, err := HashPassword(next)
	if err != nil {
		return err
	}
	return s.store.InTx(ctx, func(q *store.Queries) error {
		if _, err := q.UpdateUserPassword(ctx, store.UpdateUserPasswordParams{ID: p.UserID, PasswordHash: hash}); err != nil {
			return err
		}
		// 修改密码后吊销当前会话以外的所有会话。
		return q.DeleteOtherSessions(ctx, store.DeleteOtherSessionsParams{UserID: p.UserID, KeepID: p.SessionID})
	})
}

func validateNewPassword(v *httpx.Validator, pointer, pw string) {
	switch n := utf8.RuneCountInString(pw); {
	case n < minPasswordLen:
		v.Add(pointer, "too_short", fmt.Sprintf("长度不能少于 %d 个字符", minPasswordLen))
	case n > maxPasswordLen:
		v.Add(pointer, "too_long", fmt.Sprintf("长度不能超过 %d 个字符", maxPasswordLen))
	}
}

func (s *Service) CleanupExpired(ctx context.Context) (int64, error) {
	return s.store.DeleteExpiredSessions(ctx)
}

var ErrUserExists = errors.New("user already exists")

var usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,32}$`)

func (s *Service) CreateUser(ctx context.Context, username, displayName, password string) (store.User, error) {
	if !usernamePattern.MatchString(username) {
		return store.User{}, errors.New("username must match ^[a-z0-9_]{3,32}$")
	}
	var v httpx.Validator
	validateNewPassword(&v, "password", password)
	if err := v.Err(); err != nil {
		return store.User{}, fmt.Errorf("password must be %d–%d characters", minPasswordLen, maxPasswordLen)
	}
	hash, err := HashPassword(password)
	if err != nil {
		return store.User{}, err
	}
	u, err := s.store.CreateUser(ctx, store.CreateUserParams{Username: username, DisplayName: displayName, PasswordHash: hash})
	if store.ConstraintViolation(err) == "users_username_key" {
		return store.User{}, ErrUserExists
	}
	return u, err
}

// ResetPassword 是服务端运维操作，通常意味着账户可能已泄露，因此同时吊销全部会话与 API 令牌。
func (s *Service) ResetPassword(ctx context.Context, username, password string) error {
	var v httpx.Validator
	validateNewPassword(&v, "password", password)
	if err := v.Err(); err != nil {
		return fmt.Errorf("password must be %d–%d characters", minPasswordLen, maxPasswordLen)
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("find user %q: %w", username, err)
	}
	return s.store.InTx(ctx, func(q *store.Queries) error {
		if _, err := q.UpdateUserPassword(ctx, store.UpdateUserPasswordParams{ID: user.ID, PasswordHash: hash}); err != nil {
			return err
		}
		if err := q.DeleteUserSessions(ctx, user.ID); err != nil {
			return err
		}
		return q.DeleteUserAPITokens(ctx, user.ID)
	})
}
