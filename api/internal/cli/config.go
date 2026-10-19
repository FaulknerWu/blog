package cli

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// config 保存在 $XDG_CONFIG_HOME/blog/config.json（权限 0600），每个 profile 是一个站点与令牌。
type config struct {
	Current  string             `json:"current"`
	Profiles map[string]profile `json:"profiles"`
}

type profile struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

const defaultProfile = "default"

func (a *app) configPath() (string, error) {
	if a.env.ConfigDir != "" {
		return filepath.Join(a.env.ConfigDir, "config.json"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "blog", "config.json"), nil
}

func (a *app) loadConfig() (config, error) {
	cfg := config{Profiles: map[string]profile{}}
	path, err := a.configPath()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, invalidError("配置文件 %s 无法解析：%v", path, err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]profile{}
	}
	return cfg, nil
}

func (a *app) saveConfig(cfg config) error {
	path, err := a.configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'), 0o600)
}

// profileName 依次取 --profile、BLOG_PROFILE、配置中的当前 profile，最后是 default。
func (a *app) profileName(cfg config) string {
	switch {
	case a.profile != "":
		return a.profile
	case a.env.Getenv("BLOG_PROFILE") != "":
		return a.env.Getenv("BLOG_PROFILE")
	case cfg.Current != "":
		return cfg.Current
	}
	return defaultProfile
}

// credentials 解析站点地址与令牌：环境变量 BLOG_URL、BLOG_TOKEN 优先于配置文件，便于 agent 在沙箱中注入。
func (a *app) credentials() (profile, error) {
	cfg, err := a.loadConfig()
	if err != nil {
		return profile{}, err
	}
	p := cfg.Profiles[a.profileName(cfg)]
	if v := a.env.Getenv("BLOG_URL"); v != "" {
		p.URL = v
	}
	if v := a.env.Getenv("BLOG_TOKEN"); v != "" {
		p.Token = v
	}
	p.URL = strings.TrimRight(p.URL, "/")
	if p.URL == "" || p.Token == "" {
		return p, authError("缺少站点地址或令牌：运行 blog auth login，或设置 BLOG_URL 与 BLOG_TOKEN")
	}
	return p, nil
}

// writeFileAtomic 先写同目录临时文件再改名，避免中途失败留下半个文件。
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(perm); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
