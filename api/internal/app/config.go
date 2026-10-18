package app

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL    string
	RenderURL      string // 渲染服务地址，如 http://render:8090
	ListenAddr     string
	SiteURL        string // 站点源（scheme://host，无路径），用于 CSRF 的 Origin 校验与生成绝对链接
	MediaDir       string
	TrustProxy     bool // 信任反向代理传入的 X-Real-IP，仅在代理之后部署时开启
	MigrateOnStart bool
	ImageWorkers   int
}

// LoadConfig 读取 HTTP 服务的完整配置。
func LoadConfig() (Config, error) {
	c := Config{
		ListenAddr: env("BLOG_LISTEN_ADDR", ":8080"),
		SiteURL:    strings.TrimRight(os.Getenv("BLOG_SITE_URL"), "/"),
		MediaDir:   env("BLOG_MEDIA_DIR", "./data/media"),
	}
	var errs []error
	var err error
	if c.DatabaseURL, err = DatabaseURL(); err != nil {
		errs = append(errs, err)
	}
	if c.RenderURL, err = RenderURL(); err != nil {
		errs = append(errs, err)
	}
	if c.TrustProxy, err = envBool("BLOG_TRUST_PROXY", false); err != nil {
		errs = append(errs, err)
	}
	if c.MigrateOnStart, err = envBool("BLOG_MIGRATE_ON_START", true); err != nil {
		errs = append(errs, err)
	}
	if c.ImageWorkers, err = envInt("BLOG_IMAGE_WORKERS", 2); err != nil {
		errs = append(errs, err)
	}
	if u, err := url.Parse(c.SiteURL); c.SiteURL == "" || err != nil || u.Scheme == "" || u.Host == "" || u.Path != "" {
		errs = append(errs, errors.New("BLOG_SITE_URL must be an origin such as https://blog.example.com"))
	}
	return c, errors.Join(errs...)
}

// DatabaseURL 读取数据库连接串，运维命令只需要这一项。
func DatabaseURL() (string, error) {
	dsn := os.Getenv("BLOG_DATABASE_URL")
	if dsn == "" {
		return "", errors.New("BLOG_DATABASE_URL is required")
	}
	return dsn, nil
}

// RenderURL 读取渲染服务地址。
func RenderURL() (string, error) {
	raw := strings.TrimRight(os.Getenv("BLOG_RENDER_URL"), "/")
	if u, err := url.Parse(raw); raw == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("BLOG_RENDER_URL must be a URL such as http://render:8090")
	}
	return raw, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", key)
	}
	return b, nil
}

func envInt(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return n, nil
}
