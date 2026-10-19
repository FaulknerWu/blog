// Package cli 实现 blog 命令行客户端：以 API 令牌远程操作博客，输出稳定的 JSON 与退出码，面向 agent 使用。
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Env 是 CLI 与外部环境的全部接触面，测试时整体替换。
type Env struct {
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	Getenv    func(string) string
	ConfigDir string       // 空表示 $XDG_CONFIG_HOME/blog
	HTTP      *http.Client // 空表示默认客户端
	Version   string
}

type app struct {
	env     Env
	profile string
	api     *client
}

// runFunc 执行已解析参数的命令，args 为位置参数。
type runFunc func(ctx context.Context, a *app, args []string) error

type command struct {
	name    string // 以空格分隔的命令路径，如 "post push"
	args    string // 位置参数说明，如 "<file>"
	summary string
	offline bool // 不需要 API 凭据
	// setup 在参数集上注册 flag，并返回读取这些 flag 的执行函数。
	setup func(fs *flag.FlagSet) runFunc
}

var commands []command

func register(cs ...command) { commands = append(commands, cs...) }

// Run 执行一条命令并返回进程退出码。结果以 JSON 写到 stdout，错误以 problem JSON 写到 stderr。
func Run(ctx context.Context, args []string, env Env) int {
	if env.HTTP == nil {
		env.HTTP = &http.Client{Timeout: 2 * time.Minute}
	}
	a := &app{env: env}
	global := flag.NewFlagSet("blog", flag.ContinueOnError)
	global.SetOutput(io.Discard)
	global.StringVar(&a.profile, "profile", "", "使用的配置 profile")
	if err := global.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(env.Stdout)
			return exitOK
		}
		return report(env, usageError("%v", err))
	}
	args = global.Args()
	if len(args) == 0 || args[0] == "help" {
		printUsage(env.Stdout)
		if len(args) == 0 {
			return exitUsage
		}
		return exitOK
	}

	cmd, rest := findCommand(args)
	if cmd == nil {
		return report(env, usageError("未知命令 %q，运行 blog help 查看全部命令", strings.Join(args, " ")))
	}
	fs := flag.NewFlagSet("blog "+cmd.name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	run := cmd.setup(fs)
	pos, err := parseInterspersed(fs, rest)
	if errors.Is(err, flag.ErrHelp) {
		printCommandUsage(env.Stdout, cmd, fs)
		return exitOK
	}
	if err != nil {
		return report(env, usageError("%v（blog %s -h 查看用法）", err, cmd.name))
	}
	if !cmd.offline {
		p, err := a.credentials()
		a.api = &client{base: p.URL, token: p.Token, http: env.HTTP, err: err}
	}
	if err := run(ctx, a, pos); err != nil {
		return report(env, err)
	}
	return exitOK
}

// findCommand 按最长前缀匹配命令路径。
func findCommand(args []string) (*command, []string) {
	for n := min(len(args), 2); n > 0; n-- {
		name := strings.Join(args[:n], " ")
		for i := range commands {
			if commands[i].name == name {
				return &commands[i], args[n:]
			}
		}
	}
	return nil, nil
}

// parseInterspersed 允许 flag 出现在位置参数之后（blog post push a.md --dry-run）。
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `usage: blog [--profile NAME] <command> [args] [flags]

以 API 令牌操作博客。结果以 JSON 写到 stdout；出错时 stderr 输出 problem JSON，退出码见 blog schema。
凭据：blog auth login，或环境变量 BLOG_URL、BLOG_TOKEN（优先于配置文件）。

commands:
`)
	for _, c := range commands {
		fmt.Fprintf(w, "  %-34s %s\n", strings.TrimSpace(c.name+" "+c.args), c.summary)
	}
	fmt.Fprintln(w, "\n各命令的参数见 blog <command> -h；机器可读的完整描述见 blog schema。")
}

func printCommandUsage(w io.Writer, c *command, fs *flag.FlagSet) {
	fmt.Fprintf(w, "usage: blog %s\n\n%s\n", strings.TrimSpace(c.name+" "+c.args+" [flags]"), c.summary)
	var flags bytes.Buffer
	fs.SetOutput(&flags)
	fs.PrintDefaults()
	if flags.Len() > 0 {
		fmt.Fprintf(w, "\nflags:\n%s", flags.String())
	}
}

// out 把结果以缩进 JSON 写到 stdout。
func (a *app) out(v any) error {
	var data []byte
	switch b := v.(type) {
	case json.RawMessage:
		var buf bytes.Buffer
		if err := json.Indent(&buf, b, "", "  "); err != nil {
			return err
		}
		data = buf.Bytes()
	case []byte:
		return a.out(json.RawMessage(b))
	default:
		var err error
		if data, err = json.MarshalIndent(v, "", "  "); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(a.env.Stdout, string(data))
	return err
}

func compactJSON(b []byte) []byte {
	var buf bytes.Buffer
	if err := json.Compact(&buf, b); err != nil {
		return b
	}
	return buf.Bytes()
}

func needArgs(args []string, n int, usage string) error {
	if len(args) != n {
		return usageError("需要参数 %s", usage)
	}
	return nil
}

func requireYes(yes bool, what string) error {
	if !yes {
		return usageError("%s不可撤销，确认请加 --yes", what)
	}
	return nil
}

func init() {
	register(authCommands()...)
	register(contentCommands(postKind)...)
	register(contentCommands(pageKind)...)
	register(tagCommands()...)
	register(seriesCommands()...)
	register(mediaCommands()...)
	register(settingsCommands()...)
	register(command{
		name: "schema", summary: "输出全部命令、参数与退出码的 JSON 描述", offline: true,
		setup: func(fs *flag.FlagSet) runFunc {
			return func(ctx context.Context, a *app, args []string) error { return a.out(schema(a.env.Version)) }
		},
	})
}

type flagSchema struct {
	Name    string `json:"name"`
	Usage   string `json:"usage"`
	Default string `json:"default,omitempty"`
}

type commandSchema struct {
	Name    string       `json:"name"`
	Args    string       `json:"args,omitempty"`
	Summary string       `json:"summary"`
	Flags   []flagSchema `json:"flags"`
}

func schema(version string) any {
	cmds := make([]commandSchema, 0, len(commands))
	for _, c := range commands {
		fs := flag.NewFlagSet(c.name, flag.ContinueOnError)
		c.setup(fs)
		flags := []flagSchema{}
		fs.VisitAll(func(f *flag.Flag) {
			flags = append(flags, flagSchema{Name: "--" + f.Name, Usage: f.Usage, Default: f.DefValue})
		})
		cmds = append(cmds, commandSchema{Name: c.name, Args: c.args, Summary: c.summary, Flags: flags})
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
	return map[string]any{
		"name":         "blog",
		"version":      version,
		"output":       "stdout 为 JSON；错误时 stderr 为一行 RFC 9457 problem JSON",
		"global_flags": []flagSchema{{Name: "--profile", Usage: "使用的配置 profile"}},
		"env": map[string]string{
			"BLOG_URL":     "站点地址，如 https://blog.example.com（优先于配置文件）",
			"BLOG_TOKEN":   "API 令牌 blog_pat_…（优先于配置文件）",
			"BLOG_PROFILE": "默认 profile",
		},
		"exit_codes": exitCodes,
		"commands":   cmds,
	}
}
