package servercli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emanyzwww/plainship/internal/version"
)

// runServerCLI 执行 plainship-server CLI, 返回输出与错误.
func runServerCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetIn(strings.NewReader(""))
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestToken_AutoGenerateAndPersist(t *testing.T) {
	data := filepath.Join(t.TempDir(), "data")
	// 首次调用生成并持久化.
	tok1, created, err := LoadOrCreateToken(data)
	if err != nil {
		t.Fatalf("LoadOrCreateToken 失败: %v", err)
	}
	if !created {
		t.Error("首次调用应生成新令牌")
	}
	if len(tok1) < 8 || tok1[:3] != "ps_" {
		t.Errorf("令牌格式不正确: %q", tok1)
	}
	if !fileExists(filepath.Join(data, "server.token")) {
		t.Error("令牌未持久化到 server.token")
	}
	// 再次调用应读取同一令牌, 不重新生成.
	tok2, created2, err := LoadOrCreateToken(data)
	if err != nil {
		t.Fatalf("再次 LoadOrCreateToken 失败: %v", err)
	}
	if created2 || tok2 != tok1 {
		t.Errorf("令牌未复用: created=%v tok2=%q tok1=%q", created2, tok2, tok1)
	}
	// token 命令应打印该令牌.
	out, err := runServerCLI(t, "token", "--data", data)
	if err != nil {
		t.Fatalf("token 命令失败: %v\n%s", err, out)
	}
	if !strings.Contains(out, tok1) {
		t.Errorf("token 命令输出缺少令牌: %s", out)
	}
}

func TestToken_NotFound(t *testing.T) {
	_, err := runServerCLI(t, "token", "--data", filepath.Join(t.TempDir(), "nope"))
	if err == nil {
		t.Fatal("令牌不存在时应报错")
	}
}

func TestVersion(t *testing.T) {
	out, err := runServerCLI(t, "version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, version.Version) {
		t.Errorf("version 输出: %s", out)
	}
	if !strings.Contains(out, "Plainship Server") {
		t.Errorf("version 输出应标明服务端: %s", out)
	}
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
