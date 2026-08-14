package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeRelPath(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"普通路径", "a/b/c.html", filepath.Join("a", "b", "c.html"), false},
		{"正斜杠路径", "a/b/c", filepath.Join("a", "b", "c"), false},
		{"反斜杠归一化", "a\\b\\c.html", filepath.Join("a", "b", "c.html"), false},
		{"前导斜杠", "/a/b", filepath.Join("a", "b"), false},
		{"前导点斜杠", "./a", filepath.Join("a"), false},
		{"点", ".", "", true},
		{"空", "", "", true},
		{"父级穿越", "../x", "", true},
		{"深层父级", "a/../../x", "", true},
		{"反斜杠父级", "..\\x", "", true},
		{"反斜杠深层", "a\\..\\..\\x", "", true},
		{"前导斜杠绝对路径归一化", "/etc/passwd", filepath.Join("etc", "passwd"), false},
		{"Windows 盘符", "C:/x", "", true},
		{"合法点开头文件名", "..foo.md", "..foo.md", false},
		{"空段", "a//b", filepath.Join("a", "b"), false},
	}
	for _, tt := range tests {
		got, err := SafeRelPath(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("%s: 应返回错误, 实际 %q", tt.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: 错误 %v", tt.name, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

// TestSafeRelPath_WindowsTraversal 复现 dev 服务器曾经的漏洞: path.Clean
// 不识别反斜杠, 但 SafeRelPath 先归一化反斜杠, ".." 段必然被拒绝.
func TestSafeRelPath_WindowsTraversal(t *testing.T) {
	for _, p := range []string{
		"..\\..\\Windows\\win.ini",
		"..\\..\\..\\Users\\x\\secret.txt",
		"a\\..\\..\\etc\\passwd",
		"b1\\..\\..\\sites\\other",
	} {
		if _, err := SafeRelPath(p); err == nil {
			t.Errorf("路径 %q 应被拒绝", p)
		}
	}
}

func TestSafeJoin(t *testing.T) {
	base := t.TempDir()
	if _, err := SafeJoin(base, "a/b.txt"); err != nil {
		t.Errorf("SafeJoin 正常路径失败: %v", err)
	}
	if _, err := SafeJoin(base, "../evil"); err == nil {
		t.Error("SafeJoin 越界路径应报错")
	}
	if _, err := SafeJoin(base, "..\\evil"); err == nil {
		t.Error("SafeJoin 反斜杠越界路径应报错")
	}
}

func TestSafeJoin_SymlinkEscape(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(base, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("当前环境不支持符号链接")
	}
	// SafeJoin 只做词法校验: 符号链接在 join 结果上可能指向外部,
	// 调用方应结合 Lstat 检查; 这里验证词法结果仍在 base 内.
	joined, err := SafeJoin(base, "link/secret.txt")
	if err != nil {
		t.Fatalf("SafeJoin 失败: %v", err)
	}
	if !strings.HasPrefix(joined, base+string(filepath.Separator)) {
		t.Errorf("join 结果越界: %s", joined)
	}
}
