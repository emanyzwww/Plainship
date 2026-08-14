package router

import (
	"testing"

	"github.com/emanyzwww/Plainship/internal/model"
)

func TestRouteFor(t *testing.T) {
	tests := []struct {
		name      string
		srcRel    string
		meta      model.Metadata
		wantRoute string
		wantOut   string
		wantErr   bool
	}{
		{"中文文件名", "docs/测试文档.md", nil, "测试文档/", "测试文档/index.html", false},
		{"英文文件名", "docs/hello.md", nil, "hello/", "hello/index.html", false},
		{"slug 覆盖", "docs/测试文档.md", model.Metadata{"slug": "hello-world"}, "hello-world/", "hello-world/index.html", false},
		{"嵌套目录", "docs/guide/快速开始.md", nil, "guide/快速开始/", "guide/快速开始/index.html", false},
		{"嵌套加 slug", "docs/guide/快速开始.md", model.Metadata{"slug": "quick-start"}, "guide/quick-start/", "guide/quick-start/index.html", false},
		{"目录索引页", "docs/guide/index.md", nil, "guide/", "guide/index.html", false},
		{"目录索引页显式 slug", "docs/guide/index.md", model.Metadata{"slug": "intro"}, "guide/intro/", "guide/intro/index.html", false},
		{"根目录 index.md 保留", "docs/index.md", nil, "index/", "index/index.html", false},
		{"非法 slug 穿越", "docs/a.md", model.Metadata{"slug": "../evil"}, "", "", true},
		{"非法 slug 绝对路径", "docs/a.md", model.Metadata{"slug": "/etc/passwd"}, "", "", true},
		{"非法 slug 反斜杠", "docs/a.md", model.Metadata{"slug": "a\\b"}, "", "", true},
		{"非法 slug 空段", "docs/a.md", model.Metadata{"slug": "a//b"}, "", "", true},
	}
	for _, tt := range tests {
		route, out, err := RouteFor(tt.srcRel, tt.meta)
		if tt.wantErr {
			if err == nil {
				t.Errorf("%s: 应返回错误, 实际 route=%q out=%q", tt.name, route, out)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: RouteFor 返回错误: %v", tt.name, err)
			continue
		}
		if route != tt.wantRoute || out != tt.wantOut {
			t.Errorf("%s: route=%q out=%q, 期望 route=%q out=%q", tt.name, route, out, tt.wantRoute, tt.wantOut)
		}
	}
}

func TestResolveLink(t *testing.T) {
	r := New()
	r.Register("docs/guide/foo.md", "guide/foo/")
	r.Register("docs/bar.md", "bar/")
	r.Register("docs/guide/index.md", "guide/")

	tests := []struct {
		name   string
		srcRel string
		dest   string
		want   string
	}{
		// 解析结果均为根相对 URL, 保证任意页面深度下地址正确.
		{"相对链接", "docs/guide/foo.md", "./next.md", "/guide/next/"},
		{"无扩展名不变", "docs/guide/foo.md", "https://example.com/a.md", "https://example.com/a.md"},
		{"协议相对不变", "docs/guide/foo.md", "//cdn.example.com/a.md", "//cdn.example.com/a.md"},
		{"mailto 不变", "docs/guide/foo.md", "mailto:a@b.com", "mailto:a@b.com"},
		{"已登记路由", "docs/bar.md", "../guide/foo.md", "/guide/foo/"},
		{"目录索引页链接", "docs/bar.md", "../guide/index.md", "/guide/"},
		{"根路径", "docs/guide/foo.md", "/bar.md", "/bar/"},
		{"带锚点", "docs/guide/foo.md", "../bar.md#section", "/bar/#section"},
		{"带查询", "docs/guide/foo.md", "../bar.md?x=1", "/bar/?x=1"},
		{"非 md 不变", "docs/guide/foo.md", "image.png", "image.png"},
		{"绝对非 md 不变", "docs/guide/foo.md", "/assets/app.css", "/assets/app.css"},
		{"锚点不变", "docs/guide/foo.md", "#section", "#section"},
		{"空目标不变", "docs/guide/foo.md", "", ""},
	}
	for _, tt := range tests {
		got := r.ResolveLink(tt.srcRel, tt.dest)
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestResolveLink_Assets(t *testing.T) {
	r := New()
	r.Register("docs/guide/foo.md", "guide/foo/")
	r.RegisterAsset("docs/guide/img/logo.png", "docs/guide/img/logo.png")
	r.RegisterAsset("docs/img.png", "docs/img.png")

	tests := []struct {
		name   string
		srcRel string
		dest   string
		want   string
	}{
		{"资源相对当前文档目录", "docs/guide/foo.md", "img/logo.png", "/docs/guide/img/logo.png"},
		{"资源带锚点", "docs/guide/foo.md", "img/logo.png#x", "/docs/guide/img/logo.png#x"},
		{"上级目录资源", "docs/guide/foo.md", "../img.png", "/docs/img.png"},
		{"未登记资源保持不变", "docs/guide/foo.md", "img/missing.png", "img/missing.png"},
	}
	for _, tt := range tests {
		got := r.ResolveLink(tt.srcRel, tt.dest)
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestResolveLink_Base(t *testing.T) {
	// 站点部署在子路径时, 所有解析结果带上基础路径前缀.
	r := NewWithBase("/repo")
	r.Register("docs/bar.md", "bar/")
	r.RegisterAsset("docs/img.png", "docs/img.png")

	tests := []struct {
		name   string
		srcRel string
		dest   string
		want   string
	}{
		{"路由带基础路径", "docs/bar.md", "./bar.md", "/repo/bar/"},
		{"资源带基础路径", "docs/bar.md", "img.png", "/repo/docs/img.png"},
		{"外部链接不受影响", "docs/bar.md", "https://example.com/x.md", "https://example.com/x.md"},
	}
	for _, tt := range tests {
		got := r.ResolveLink(tt.srcRel, tt.dest)
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestJoinURL(t *testing.T) {
	tests := []struct {
		base string
		p    string
		want string
	}{
		{"", "", "/"},
		{"", "foo/", "/foo/"},
		{"", "assets/app.css", "/assets/app.css"},
		{"/repo", "foo/", "/repo/foo/"},
		{"/repo/", "foo/", "/repo/foo/"},
		{"/repo", "", "/repo/"},
		{"/repo", "/foo/", "/repo/foo/"},
	}
	for _, tt := range tests {
		got := JoinURL(tt.base, tt.p)
		if got != tt.want {
			t.Errorf("JoinURL(%q, %q) = %q, want %q", tt.base, tt.p, got, tt.want)
		}
	}
}

func TestEncodePath(t *testing.T) {
	got := EncodePath("测试文档/")
	want := "%E6%B5%8B%E8%AF%95%E6%96%87%E6%A1%A3/"
	if got != want {
		t.Errorf("EncodePath = %q, want %q", got, want)
	}
}
