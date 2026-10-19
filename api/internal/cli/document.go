package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.yaml.in/yaml/v3"
)

// 内容文件是带 YAML front matter 的 Markdown：front matter 描述元数据，其后是正文原文。
// pull 生成、push 读取并写回，使本地文件始终对应服务端的最新状态。
const fence = "---\n"

type postMeta struct {
	ID      int64    `yaml:"id,omitempty"`      // 服务端 ID；为空时 push 创建新文章
	Version int32    `yaml:"version,omitempty"` // 作为 If-Match，版本不符时 push 返回冲突
	Title   string   `yaml:"title"`
	Slug    string   `yaml:"slug"`
	Summary string   `yaml:"summary"`
	Tags    flowList `yaml:"tags"`             // 标签 slug
	Series  string   `yaml:"series,omitempty"` // 专栏 slug
	Cover   string   `yaml:"cover,omitempty"`  // 本地图片路径或 /media/{sha256}
}

type pageMeta struct {
	ID          int64  `yaml:"id,omitempty"`
	Version     int32  `yaml:"version,omitempty"`
	Title       string `yaml:"title"`
	Slug        string `yaml:"slug"`
	Description string `yaml:"description"`
}

// parseDocument 拆分 front matter 与正文；未知字段视为错误，避免拼写错误被静默忽略。
func parseDocument(data []byte, meta any) (string, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, fence) {
		return "", invalidError("文件必须以 --- 开头的 YAML front matter 开始")
	}
	rest := text[len(fence):]
	end := strings.Index(rest, "\n"+fence)
	var head string
	switch {
	case strings.HasPrefix(rest, fence):
		head, rest = "", rest[len(fence):]
	case end >= 0:
		head, rest = rest[:end+1], rest[end+1+len(fence):]
	case strings.HasSuffix(rest, "\n---"):
		head, rest = strings.TrimSuffix(rest, "---"), ""
	default:
		return "", invalidError("front matter 缺少结束的 ---")
	}
	dec := yaml.NewDecoder(strings.NewReader(head))
	dec.KnownFields(true)
	if err := dec.Decode(meta); err != nil && !errors.Is(err, io.EOF) {
		return "", invalidError("front matter 无法解析：%v", err)
	}
	// 结束行之后的一个空行是格式的一部分，不属于正文。
	return strings.TrimPrefix(rest, "\n"), nil
}

func formatDocument(meta any, body string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(fence)
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(meta); err != nil {
		return nil, fmt.Errorf("encode front matter: %w", err)
	}
	enc.Close()
	buf.WriteString(fence)
	buf.WriteString("\n")
	buf.WriteString(body)
	return buf.Bytes(), nil
}

// flowList 以 [a, b] 的行内形式输出，与手写 front matter 的习惯一致。
type flowList []string

func (l flowList) MarshalYAML() (any, error) {
	n := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, s := range l {
		n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: s})
	}
	return n, nil
}
