package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emanyzwww/Plainship/internal/manifest"
	"github.com/emanyzwww/Plainship/internal/space"
	"github.com/emanyzwww/Plainship/internal/state"
)

// setupSyncedSpace 创建 Space 并执行一次构建, 然后记录一次"已同步"状态.
func setupSyncedSpace(t *testing.T) (string, *manifest.Manifest) {
	t.Helper()
	root := t.TempDir()
	s, err := space.Create(root)
	if err != nil {
		t.Fatal(err)
	}
	// 写一篇文档并构建.
	path := filepath.Join(s.DocsDir(), "测试文档.md")
	if err := os.WriteFile(path, []byte("---\ntitle: 测试\n---\n内容"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 直接调用构建器.
	m := buildForTest(t, s)
	return root, m
}

// TestDiff_InitialUpload tests the first sync uploads everything.
func TestDiff_InitialUpload(t *testing.T) {
	root, m := setupSyncedSpace(t)
	dist := filepath.Join(root, "build")
	diff, err := Diff(root, dist, m, false)
	if err != nil {
		t.Fatalf("Diff 失败: %v", err)
	}
	if diff.UploadCount == 0 {
		t.Error("首次同步应上传全部文件")
	}
	if len(diff.Deletes) != 0 {
		t.Errorf("首次同步不应有删除: %v", diff.Deletes)
	}
	// 验证上传内容可读.
	for rel, data := range diff.Upload {
		if len(data) == 0 {
			t.Errorf("文件 %s 内容为空", rel)
		}
	}
}

// TestDiff_NoChangesAfterSync tests no uploads after sync state recorded.
func TestDiff_NoChangesAfterSync(t *testing.T) {
	root, m := setupSyncedSpace(t)
	dist := filepath.Join(root, "build")
	// 模拟一次成功同步: 记录 dist 全部文件的哈希.
	if err := recordSyncState(t, root, dist, m.BuildID); err != nil {
		t.Fatal(err)
	}

	diff, err := Diff(root, dist, m, false)
	if err != nil {
		t.Fatalf("Diff 失败: %v", err)
	}
	if diff.UploadCount != 0 {
		t.Errorf("同步后应无上传: %d 个文件", diff.UploadCount)
	}
	if len(diff.Deletes) != 0 {
		t.Errorf("同步后不应有删除: %v", diff.Deletes)
	}
}

// TestDiff_FullSync 验证全量模式: 即使有同步记录也上传全部文件, 且不产生删除.
func TestDiff_FullSync(t *testing.T) {
	root, m := setupSyncedSpace(t)
	dist := filepath.Join(root, "build")
	// 先记录一次同步状态 (模拟服务器曾有历史版本).
	if err := recordSyncState(t, root, dist, m.BuildID); err != nil {
		t.Fatal(err)
	}

	diff, err := Diff(root, dist, m, true)
	if err != nil {
		t.Fatalf("Diff 失败: %v", err)
	}
	if diff.UploadCount == 0 {
		t.Error("全量模式应上传全部文件")
	}
	if len(diff.Deletes) != 0 {
		t.Errorf("全量模式不应有删除: %v", diff.Deletes)
	}
}

// recordSyncState 将 dist 中全部文件标记为已同步.
func recordSyncState(t *testing.T, root, dist, buildID string) error {
	t.Helper()
	ss := stateFor(root)
	ss.LastBuildID = buildID
	ss.ServerURL = "http://localhost:9090"
	ss.SiteID = "my-docs"
	ss.Files = map[string]string{}
	err := filepath.Walk(dist, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dist, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		ss.Files[filepath.ToSlash(rel)] = hashBytes(data)
		return nil
	})
	if err != nil {
		return err
	}
	return state.SaveSyncState(root, ss)
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// TestDiff_DetectsDeletion tests deleted files are detected after removal.
func TestDiff_DetectsDeletion(t *testing.T) {
	root, m := setupSyncedSpace(t)
	dist := filepath.Join(root, "build")
	if err := recordSyncState(t, root, dist, m.BuildID); err != nil {
		t.Fatal(err)
	}

	// 记录一个同步状态中的额外文件, 模拟服务器上多出的文件.
	ss := stateFor(root)
	ss.Files["extra/index.html"] = "deadbeef"
	if err := state.SaveSyncState(root, ss); err != nil {
		t.Fatal(err)
	}

	diff, err := Diff(root, dist, m, false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range diff.Deletes {
		if d == "extra/index.html" {
			found = true
		}
	}
	if !found {
		t.Errorf("未检测到应删除的文件: %v", diff.Deletes)
	}
}

// TestNormalizeServerURL 验证服务器地址规范化.
func TestNormalizeServerURL(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"  ", ""},
		{"127.0.0.1", "http://127.0.0.1"},
		{"127.0.0.1:9090", "http://127.0.0.1:9090"},
		{"localhost:9090", "http://localhost:9090"},
		{"http://localhost:9090", "http://localhost:9090"},
		{"https://example.com/", "https://example.com"},
		{"example.com/path/", "http://example.com/path"},
	}
	for _, tt := range tests {
		got := NormalizeServerURL(tt.in)
		if got != tt.want {
			t.Errorf("NormalizeServerURL(%q) = %q, 期望 %q", tt.in, got, tt.want)
		}
	}
}

// TestRequestMarshal checks the protocol request shape.
func TestRequestMarshal(t *testing.T) {
	req := Request{
		ProtocolVersion: ProtocolVersion,
		SiteID:          "my-docs",
		BuildID:         "build-001",
		Files: []FilePayload{{
			Path:    "index.html",
			Content: "aGk=",
			Hash:    "abc",
		}},
		Deletes: []string{"old/index.html"},
	}
	req.FullSync = true
	data, err := jsonMarshal(req)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{"protocolVersion", "siteId", "buildId", "files", "deletes", "fullSync"} {
		if !strings.Contains(s, want) {
			t.Errorf("请求缺少字段 %s: %s", want, s)
		}
	}
}
