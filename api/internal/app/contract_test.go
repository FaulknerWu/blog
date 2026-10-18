package app_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"blog/api/internal/app"
)

// TestRoutesMatchOpenAPI 确保 openapi.yaml 与实际注册的路由一一对应：新增或删除接口时两边必须同步修改。
func TestRoutesMatchOpenAPI(t *testing.T) {
	raw, err := os.ReadFile("../../openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}
	methods := []string{"get", "put", "post", "delete", "patch", "head", "options"}
	var documented []string
	for path, item := range spec.Paths {
		for key := range item {
			if slices.Contains(methods, key) {
				documented = append(documented, strings.ToUpper(key)+" "+path)
			}
		}
	}

	// 只读取路由表，不处理请求，因此不需要数据库与渲染服务。
	a, err := app.New(app.Config{SiteURL: siteURL, RenderURL: "http://render.invalid", MediaDir: t.TempDir(), ImageWorkers: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	registered := a.Routes()

	slices.Sort(documented)
	slices.Sort(registered)
	for _, r := range registered {
		if !slices.Contains(documented, r) {
			t.Errorf("route %q is not documented in openapi.yaml", r)
		}
	}
	for _, d := range documented {
		if !slices.Contains(registered, d) {
			t.Errorf("openapi.yaml documents %q but no such route is registered", d)
		}
	}
}
