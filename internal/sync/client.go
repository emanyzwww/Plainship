// Package sync 实现 Plainship 同步协议与客户端.
// 协议版本化: 客户端将已构建的静态资源上传到 Plainship Server.
package sync

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emanyzwww/plainship/internal/fsutil"
	"github.com/emanyzwww/plainship/internal/hash"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/manifest"
	"github.com/emanyzwww/plainship/internal/state"
)

// ProtocolVersion 是当前同步协议版本.
const ProtocolVersion = 1

// FilePayload 是一个待同步的文件.
type FilePayload struct {
	Path    string `json:"path"`    // 相对站点根的路径, 例如 测试文档/index.html
	Content string `json:"content"` // Base64 编码的文件内容
	Hash    string `json:"hash"`    // 文件内容哈希
}

// Request 是同步请求体.
type Request struct {
	ProtocolVersion int           `json:"protocolVersion"`
	SiteID          string        `json:"siteId"`
	BuildID         string        `json:"buildId"`
	Files           []FilePayload `json:"files"`
	Deletes         []string      `json:"deletes"` // 相对路径列表
	// FullSync 为 true 时服务器清空目标 release 目录后整体重建 (不继承旧版本),
	// 用于服务器无历史版本或与本地构建不一致的场景, 防止陈旧文件残留.
	FullSync bool `json:"fullSync"`
}

// Response 是服务器返回结果.
type Response struct {
	OK           bool   `json:"ok"`
	Message      string `json:"message"`
	BuildID      string `json:"buildId"`
	Active       bool   `json:"active"`
	StoredFiles  int    `json:"storedFiles"`
	DeletedFiles int    `json:"deletedFiles"`
}

// Client 是同步客户端.
type Client struct {
	ServerURL string
	SiteID    string
	Token     string
	// Timeout 是请求超时时间, 默认 60 秒.
	Timeout time.Duration
	// HTTPClient 可注入自定义客户端, 用于测试.
	HTTPClient *http.Client
}

// New 创建同步客户端.
// serverURL 缺少协议前缀时自动补全 http://, 例如 "127.0.0.1:9090".
func New(serverURL, siteID, token string) *Client {
	serverURL = NormalizeServerURL(serverURL)
	return &Client{
		ServerURL: serverURL,
		SiteID:    siteID,
		Token:     token,
		Timeout:   60 * time.Second,
	}
}

// NormalizeServerURL 规范化服务器地址.
// 规则:
//   - 空字符串原样返回.
//   - 缺少 http:// 或 https:// 时自动补全 http://.
//   - 去除结尾的 /.
func NormalizeServerURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		raw = "http://" + raw
	}
	return strings.TrimRight(raw, "/")
}

// DiffResult 是本地构建与上次同步之间的差异.
type DiffResult struct {
	// Upload 是需要上传的文件 (相对 dist 路径 -> 内容).
	Upload map[string][]byte
	// Deletes 是需要在服务器删除的路径.
	Deletes []string
	// UploadCount 与 DeleteCount 是计数.
	UploadCount, DeleteCount int
}

// Diff 对比 dist 与上次同步状态, 计算需要上传与删除的文件.
// fullSync 为 true 时上传全部文件且不产生删除 (用于服务器无历史版本的情况).
func Diff(spaceRoot, outputDir string, m *manifest.Manifest, fullSync bool) (*DiffResult, error) {
	prev := state.LoadSyncState(spaceRoot)
	result := &DiffResult{Upload: map[string][]byte{}}

	// 读取本次构建产物.
	fileHashes := map[string]string{}
	allFiles, err := fsutil.ListFiles(outputDir)
	if err != nil {
		return nil, err
	}
	for _, rel := range allFiles {
		rel = filepath.ToSlash(rel)
		abs := filepath.Join(outputDir, filepath.FromSlash(rel))
		h, err := hash.File(abs)
		if err != nil {
			return nil, err
		}
		fileHashes[rel] = h
		// 未同步过或内容变化 -> 需要上传; 全量模式 -> 全部上传.
		prevHash, ok := prev.Files[rel]
		if fullSync || !ok || prevHash != h {
			data, err := os.ReadFile(abs)
			if err != nil {
				return nil, err
			}
			result.Upload[rel] = data
		}
	}

	// 全量模式: 服务器将整体重建, 不需要删除操作.
	if !fullSync {
		// 上次同步过但本次不存在的文件 -> 需要删除.
		for rel := range prev.Files {
			if _, ok := fileHashes[rel]; !ok {
				result.Deletes = append(result.Deletes, rel)
			}
		}
	}
	// 处理清单中的删除项 (源文件删除导致产物删除).
	for _, d := range m.Deleted {
		if _, exists := fileHashes[d.Output]; !exists {
			found := false
			for _, del := range result.Deletes {
				if del == d.Output {
					found = true
					break
				}
			}
			if !found {
				result.Deletes = append(result.Deletes, d.Output)
			}
		}
	}
	result.UploadCount = len(result.Upload)
	result.DeleteCount = len(result.Deletes)
	return result, nil
}

// Status 查询服务器上站点的发布状态.
// 返回是否已发布 (服务器存在可继承的激活版本).
func (c *Client) Status() (bool, error) {
	published, _, err := c.StatusDetail()
	return published, err
}

// StatusDetail 查询服务器上站点的发布状态与当前激活的 buildID.
// active 用于客户端判断是否需要全量重建 (本地 LastBuildID 与服务器不一致时).
func (c *Client) StatusDetail() (published bool, active string, err error) {
	if c.ServerURL == "" {
		return false, "", i18n.Errorf(i18n.SyncNoServerURL)
	}
	url := fmt.Sprintf("%s/api/v1/sites/%s/status", c.ServerURL, c.SiteID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, "", err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: c.Timeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, "", i18n.Errorf(i18n.SyncConnFail, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", err
	}
	if resp.StatusCode != http.StatusOK {
		return false, "", i18n.Errorf(i18n.SyncServerErr, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var statusResp struct {
		OK        bool   `json:"ok"`
		Published bool   `json:"published"`
		Active    string `json:"active"`
	}
	if err := json.Unmarshal(body, &statusResp); err != nil {
		return false, "", i18n.Errorf(i18n.SyncParseFail, err)
	}
	return statusResp.Published, statusResp.Active, nil
}

// Sync 执行一次完整同步.
// fullSync 为 true 时全量上传所有文件 (服务器无历史版本或与本地不一致时使用).
// 返回服务器响应. 成功后更新本地同步状态.
func (c *Client) Sync(spaceRoot string, outputDir string, m *manifest.Manifest, fullSync bool) (*Response, error) {
	if c.ServerURL == "" {
		return nil, i18n.Errorf(i18n.SyncNoServerURLSync)
	}
	diff, err := Diff(spaceRoot, outputDir, m, fullSync)
	if err != nil {
		return nil, err
	}
	return c.SyncWithDiff(spaceRoot, outputDir, m, fullSync, diff)
}

// SyncWithDiff 使用调用方已计算好的 diff 执行同步.
// 避免 core.Publish 与 Sync 各自重复计算 diff (重复读盘 + TOCTOU).
func (c *Client) SyncWithDiff(spaceRoot string, outputDir string, m *manifest.Manifest, fullSync bool, diff *DiffResult) (*Response, error) {
	if c.ServerURL == "" {
		return nil, i18n.Errorf(i18n.SyncNoServerURLSync)
	}
	req := &Request{
		ProtocolVersion: ProtocolVersion,
		SiteID:          c.SiteID,
		BuildID:         m.BuildID,
		Files:           make([]FilePayload, 0, diff.UploadCount),
		Deletes:         diff.Deletes,
		FullSync:        fullSync,
	}
	for rel, data := range diff.Upload {
		req.Files = append(req.Files, FilePayload{
			Path:    rel,
			Content: base64.StdEncoding.EncodeToString(data),
			Hash:    hash.Bytes(data),
		})
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/api/v1/sites/%s/sync", c.ServerURL, c.SiteID)
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.Token)
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: c.Timeout}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, i18n.Errorf(i18n.SyncConnFail, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, i18n.Errorf(i18n.SyncServerErr, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var result Response
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, i18n.Errorf(i18n.SyncParseFail, err)
	}
	if !result.OK {
		return nil, i18n.Errorf(i18n.SyncSyncFail, result.Message)
	}
	// 更新本地同步状态.
	ss := &state.SyncState{
		LastBuildID: m.BuildID,
		Files:       map[string]string{},
		ServerURL:   c.ServerURL,
		SiteID:      c.SiteID,
	}
	files, err := fsutil.ListFiles(outputDir)
	if err != nil {
		return nil, err
	}
	for _, rel := range files {
		rel = filepath.ToSlash(rel)
		h, err := hash.File(filepath.Join(outputDir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		ss.Files[rel] = h
	}
	if err := state.SaveSyncState(spaceRoot, ss); err != nil {
		return nil, err
	}
	return &result, nil
}
