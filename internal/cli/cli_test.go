package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emanyzwww/plainship/internal/config"
	"github.com/emanyzwww/plainship/internal/git"
	"github.com/emanyzwww/plainship/internal/server"
	"github.com/emanyzwww/plainship/internal/version"
)

// runCLI 在指定工作目录执行 CLI, 返回输出与错误.
func runCLI(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWD)

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	// stdin 注入非终端 reader: 交互式确认 (如 publish) 在测试中自动跳过,
	// 避免依赖真实终端状态.
	root.SetIn(strings.NewReader(""))
	root.SetArgs(args)
	err = root.Execute()
	return buf.String(), err
}

// setGitIdentity 为测试仓库配置 Git 身份.
func setGitIdentity(t *testing.T, dir string) {
	t.Helper()
	if _, _, err := git.PassThrough(dir, "config", "user.email", "test@plainship.dev"); err != nil {
		t.Fatalf("设置 user.email 失败: %v", err)
	}
	if _, _, err := git.PassThrough(dir, "config", "user.name", "Plainship Test"); err != nil {
		t.Fatalf("设置 user.name 失败: %v", err)
	}
}

// writeDoc 写入一篇带合法 Front Matter 的文档.
func writeDoc(t *testing.T, dir, rel, title string) {
	t.Helper()
	path := filepath.Join(dir, "docs", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ntitle: " + title + "\ndate: 2026-08-13\n---\n\n正文内容.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCLI_New(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "mydoc")
	out, err := runCLI(t, dir, "new", root)
	if err != nil {
		t.Fatalf("new 失败: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Plainship Space") {
		t.Errorf("输出缺少 Space 提示: %s", out)
	}
	for _, d := range []string{"docs", "themes", ".git", ".plainship"} {
		if !dirExists(filepath.Join(root, d)) {
			t.Errorf("缺少目录 %s", d)
		}
	}
	if !fileExists(filepath.Join(root, "plainship.yaml")) {
		t.Error("缺少根目录配置文件 plainship.yaml")
	}
	if dirExists(filepath.Join(root, "build")) {
		t.Error("new 不应创建 build/")
	}
}

func TestCLI_New_RejectsExisting(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCLI(t, dir, "new", dir); err != nil {
		t.Fatalf("首次创建失败: %v", err)
	}
	if _, err := runCLI(t, dir, "new", dir); err == nil {
		t.Error("重复创建应报错")
	}
}

func TestCLI_Create(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCLI(t, dir, "new", dir); err != nil {
		t.Fatalf("new 失败: %v", err)
	}
	out, err := runCLI(t, dir, "create", "测试文档")
	if err != nil {
		t.Fatalf("create 失败: %v\n%s", err, out)
	}
	if !strings.Contains(out, "测试文档") {
		t.Errorf("输出缺少文档名: %s", out)
	}
	content, err := os.ReadFile(filepath.Join(dir, "docs", "测试文档.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "title: 测试文档") {
		t.Errorf("文档模板不正确: %s", content)
	}
	// 嵌套目录.
	out, err = runCLI(t, dir, "create", "guide/快速开始")
	if err != nil {
		t.Fatalf("嵌套 create 失败: %v\n%s", err, out)
	}
	if !fileExists(filepath.Join(dir, "docs", "guide", "快速开始.md")) {
		t.Error("嵌套目录文档未创建")
	}
	// 输出必须以换行结尾, 避免与后续命令行粘连.
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("create 输出应以换行结尾: %q", out)
	}
}

func TestCLI_Build(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCLI(t, dir, "new", dir); err != nil {
		t.Fatalf("new 失败: %v", err)
	}
	setGitIdentity(t, dir)
	writeDoc(t, dir, "测试文档.md", "测试文档")

	out, err := runCLI(t, dir, "build", "-m", "发布测试文档")
	if err != nil {
		t.Fatalf("build 失败: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Number: ps-0001") {
		t.Errorf("build 输出缺少编号: %s", out)
	}
	if !fileExists(filepath.Join(dir, "build", "index.html")) {
		t.Error("缺少 build/index.html")
	}
	if !fileExists(filepath.Join(dir, "build", "测试文档", "index.html")) {
		t.Error("缺少文档页面")
	}
	// 提交信息为机器格式.
	logOut, _, _ := git.PassThrough(dir, "log", "--format=%s")
	if !strings.Contains(logOut, "docs build=ps-0001 hash=") {
		t.Errorf("提交信息不是机器格式: %s", logOut)
	}
	// -m 消息应作为 body.
	bodyOut, _, _ := git.PassThrough(dir, "log", "-1", "--format=%B")
	if !strings.Contains(bodyOut, "发布测试文档") {
		t.Errorf("-m 消息未写入提交 body: %s", bodyOut)
	}
	// 编号 tag 存在.
	tags, _, _ := git.PassThrough(dir, "tag", "--list", "ps-*")
	if strings.TrimSpace(tags) != "ps-0001" {
		t.Errorf("tag 列表 = %q", tags)
	}
}

func TestCLI_Build_FailureNoCommit(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCLI(t, dir, "new", dir); err != nil {
		t.Fatalf("new 失败: %v", err)
	}
	setGitIdentity(t, dir)
	// 非法日期导致构建失败.
	path := filepath.Join(dir, "docs", "bad.md")
	if err := os.WriteFile(path, []byte("---\ntitle: bad\ndate: 不是日期\n---\n内容"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, dir, "build"); err == nil {
		t.Fatal("非法日期应导致 build 失败")
	}
	logOut, _, _ := git.PassThrough(dir, "log", "--oneline")
	if strings.TrimSpace(logOut) != "" {
		t.Errorf("构建失败后不应有提交: %s", logOut)
	}
	tags, _, _ := git.PassThrough(dir, "tag", "--list", "ps-*")
	if strings.TrimSpace(tags) != "" {
		t.Errorf("构建失败后不应有编号 tag: %s", tags)
	}
}

func TestCLI_Status(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCLI(t, dir, "new", dir); err != nil {
		t.Fatalf("new 失败: %v", err)
	}
	setGitIdentity(t, dir)
	writeDoc(t, dir, "测试文档.md", "测试文档")
	if _, err := runCLI(t, dir, "build"); err != nil {
		t.Fatalf("build 失败: %v", err)
	}
	out, err := runCLI(t, dir, "status")
	if err != nil {
		t.Fatalf("status 失败: %v", err)
	}
	for _, want := range []string{"Space:", "Git:", "Changes:", "config: clean", "theme: clean", "docs: clean", "number ps-0001", "Publish:"} {
		if !strings.Contains(out, want) {
			t.Errorf("status 输出缺少 %q: %s", want, out)
		}
	}
}

// TestCLI_Status_NewSpace 无提交 (unborn HEAD) 的新仓库也应显示分支名.
func TestCLI_Status_NewSpace(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCLI(t, dir, "new", dir); err != nil {
		t.Fatalf("new 失败: %v", err)
	}
	out, err := runCLI(t, dir, "status")
	if err != nil {
		t.Fatalf("status 失败: %v", err)
	}
	branchOK := false
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "  Branch: ") {
			branchOK = strings.TrimSpace(strings.TrimPrefix(line, "  Branch: ")) != ""
		}
	}
	if !branchOK {
		t.Errorf("新仓库 status 应显示分支名而非空: %s", out)
	}
}

func TestCLI_Build_WithoutSpace(t *testing.T) {
	if _, err := runCLI(t, t.TempDir(), "build"); err == nil {
		t.Error("非 Space 目录 build 应报错")
	}
}

func TestCLI_Publish_NoServerConfigFails(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCLI(t, dir, "new", dir); err != nil {
		t.Fatalf("new 失败: %v", err)
	}
	setGitIdentity(t, dir)
	writeDoc(t, dir, "测试文档.md", "测试文档")
	if _, err := runCLI(t, dir, "build"); err != nil {
		t.Fatalf("build 失败: %v", err)
	}
	out, err := runCLI(t, dir, "publish")
	if err == nil {
		t.Fatalf("未配置 server.url 时 publish 应报错: %s", out)
	}
	if !strings.Contains(err.Error(), "server.url is not configured") {
		t.Errorf("错误信息缺少 server.url 提示: %v", err)
	}
}

func TestCLI_Publish_RefusesUncommitted(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCLI(t, dir, "new", dir); err != nil {
		t.Fatalf("new 失败: %v", err)
	}
	setGitIdentity(t, dir)
	writeDoc(t, dir, "测试文档.md", "测试文档")
	c, _, _ := config.Load(dir, nil)
	c.SetSpaceRoot(dir)
	_ = c.SpaceSite.ServerURL.Set("http://127.0.0.1:1")
	if _, err := config.Save(c, config.SaveSpace); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, dir, "build"); err != nil {
		t.Fatalf("build 失败: %v", err)
	}
	// 修改文档但不提交.
	if err := os.WriteFile(filepath.Join(dir, "docs", "测试文档.md"), []byte("---\ntitle: 测试文档\ndate: 2026-08-13\n---\n修改后"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, dir, "publish"); err == nil {
		t.Fatal("有未提交变更时 publish 应拒绝")
	}
}

func TestCLI_Publish_RefusesNotBuilt(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCLI(t, dir, "new", dir); err != nil {
		t.Fatalf("new 失败: %v", err)
	}
	setGitIdentity(t, dir)
	writeDoc(t, dir, "测试文档.md", "测试文档")
	c, _, _ := config.Load(dir, nil)
	c.SetSpaceRoot(dir)
	_ = c.SpaceSite.ServerURL.Set("http://127.0.0.1:1")
	if _, err := config.Save(c, config.SaveSpace); err != nil {
		t.Fatal(err)
	}
	// 手动提交源码 (模拟内容已提交但从未 build 的状态).
	if _, _, err := git.PassThrough(dir, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := git.PassThrough(dir, "commit", "-m", "docs: manual"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, dir, "publish"); err == nil {
		t.Fatal("未构建时 publish 应拒绝")
	}
}

func TestCLI_Publish_Success(t *testing.T) {
	srv := server.New(t.TempDir(), "")
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	dir := t.TempDir()
	if _, err := runCLI(t, dir, "new", dir); err != nil {
		t.Fatalf("new 失败: %v", err)
	}
	setGitIdentity(t, dir)
	writeDoc(t, dir, "测试文档.md", "测试文档")
	c, _, _ := config.Load(dir, nil)
	c.SetSpaceRoot(dir)
	_ = c.SpaceSite.ServerURL.Set(ts.URL)
	if _, err := config.Save(c, config.SaveSpace); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, dir, "build"); err != nil {
		t.Fatalf("build 失败: %v", err)
	}
	out, err := runCLI(t, dir, "publish")
	if err != nil {
		t.Fatalf("publish 失败: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Published") {
		t.Errorf("输出缺少发布成功: %s", out)
	}
	// 服务器应已提供首页.
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("首页状态码 = %d", resp.StatusCode)
	}
}

func TestCLI_Version(t *testing.T) {
	out, err := runCLI(t, t.TempDir(), "version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, version.Version) {
		t.Errorf("version 输出: %s", out)
	}
}

func TestCLI_Connect(t *testing.T) {
	srv := server.New(t.TempDir(), "test-token-123")
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	dir := t.TempDir()
	if _, err := runCLI(t, dir, "new", dir); err != nil {
		t.Fatalf("new 失败: %v", err)
	}

	// 正确令牌: 连接成功并写入配置.
	out, err := runCLI(t, dir, "connect", ts.URL, "--token", "test-token-123")
	if err != nil {
		t.Fatalf("connect 失败: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Connected to server") {
		t.Errorf("connect 输出缺少成功提示: %s", out)
	}
	c, _, err := config.Load(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.SpaceSite.ServerURL.Get() != ts.URL || c.SpaceClient.ServerToken.Get() != "test-token-123" {
		t.Errorf("配置未正确写入: url=%q token=%q", c.SpaceSite.ServerURL.Get(), c.SpaceClient.ServerToken.Get())
	}

	// 错误令牌: 连接应失败且不写入配置.
	if _, err := runCLI(t, dir, "connect", ts.URL, "--token", "wrong"); err == nil {
		t.Fatal("错误令牌应连接失败")
	}
	c2, _, _ := config.Load(dir, nil)
	if c2.SpaceClient.ServerToken.Get() != "test-token-123" {
		t.Errorf("连接失败后配置不应被覆盖: %q", c2.SpaceClient.ServerToken.Get())
	}
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func TestCLI_Config_Project(t *testing.T) {
	// 隔离真实全局配置: 项目层测试不应受机器上 ~/.plainship/config.yaml 残留影响.
	// 同时重置包级 configGlobal (可能被其他测试的 -g 置位), 保证本测试自包含.
	configGlobal = false
	// 隔离用户主目录: Windows 用 USERPROFILE, Linux/macOS 用 HOME.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	dir := t.TempDir()
	if _, err := runCLI(t, dir, "new", dir); err != nil {
		t.Fatalf("new 失败: %v", err)
	}
	// 默认 lang = en.
	out, err := runCLI(t, dir, "config", "get", "lang")
	if err != nil {
		t.Fatalf("get lang 失败: %v", err)
	}
	if strings.TrimSpace(out) != "en" {
		t.Errorf("默认 lang = %q, 期望 en", out)
	}
	// set lang zh (项目层).
	out, err = runCLI(t, dir, "config", "set", "lang", "zh")
	if err != nil {
		t.Fatalf("set lang 失败: %v", err)
	}
	if !strings.Contains(out, "lang = zh") {
		t.Errorf("set 输出: %s", out)
	}
	// 配置文件已写入.
	if !fileExists(filepath.Join(dir, ".plainship", "config.yaml")) {
		t.Error("缺少 .plainship/config.yaml")
	}
	// get 显示项目值.
	out, _ = runCLI(t, dir, "config", "get", "lang")
	if strings.TrimSpace(out) != "zh" {
		t.Errorf("get lang = %q, 期望 zh", out)
	}
	// 未知 key 报错.
	if _, err := runCLI(t, dir, "config", "get", "nope"); err == nil {
		t.Error("未知 key 应报错")
	}
	// 非法语言报错 (严格校验, 不接受 fr 回退 en).
	if _, err := runCLI(t, dir, "config", "set", "lang", "fr"); err == nil {
		t.Error("非法语言应报错")
	}
	// 合法变体规范化.
	if _, err := runCLI(t, dir, "config", "set", "lang", "zh-CN"); err != nil {
		t.Errorf("zh-CN 应被接受: %v", err)
	}
	out, _ = runCLI(t, dir, "config", "get", "lang")
	if strings.TrimSpace(out) != "zh" {
		t.Errorf("zh-CN 应规范化为 zh, 实际 %q", out)
	}
	// unset 后回退默认.
	if _, err := runCLI(t, dir, "config", "unset", "lang"); err != nil {
		t.Fatalf("unset 失败: %v", err)
	}
	out, _ = runCLI(t, dir, "config", "get", "lang")
	if strings.TrimSpace(out) != "en" {
		t.Errorf("unset 后 lang = %q, 期望 en", out)
	}
}

func TestCLI_Config_Global(t *testing.T) {
	home := t.TempDir()
	// 隔离用户主目录: Windows 用 USERPROFILE, Linux/macOS 用 HOME.
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	dir := t.TempDir()
	if _, err := runCLI(t, dir, "new", dir); err != nil {
		t.Fatalf("new 失败: %v", err)
	}
	// 全局未设置时 get -g 报错.
	if _, err := runCLI(t, dir, "config", "get", "lang", "-g"); err == nil {
		t.Error("全局未设置时 get -g 应报错")
	}
	// set -g 写入全局.
	out, err := runCLI(t, dir, "config", "set", "lang", "zh", "-g")
	if err != nil {
		t.Fatalf("set -g 失败: %v", err)
	}
	if !strings.Contains(out, "lang = zh") {
		t.Errorf("set -g 输出: %s", out)
	}
	if !fileExists(filepath.Join(home, ".plainship", "config.yaml")) {
		t.Error("缺少全局配置文件")
	}
	out, _ = runCLI(t, dir, "config", "get", "lang", "-g")
	if strings.TrimSpace(out) != "zh" {
		t.Errorf("get -g lang = %q, 期望 zh", out)
	}
}
