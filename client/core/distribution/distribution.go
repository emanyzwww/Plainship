// Package distribution 是分发层入口: 打包构建产物, 推送到服务端原子部署.
//
// 流程: 打包 BuildDir → gzip+tar → multipart POST (协议见 shared/proto) → 解析响应.
// 鉴权用 Space.LocalConfig.ServerToken (或 Options.Token), 走 Authorization: Bearer.
// 分发是单次事务: 任何一步失败都以 error 返回 (与"构建可容错, 发布必须明确失败"的定位一致).
package distribution

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/emanyzwww/papership-client/model/space"
	"github.com/emanyzwww/papership-shared/proto"
)

// Options 控制分发行为; 零值取 Space 配置默认值.
type Options struct {
	ServerURL string       // ServerURL 目标服务端地址; 空 → Space.Config.ServerURL.
	SiteID    string       // SiteID 站点标识; 空 → Space.Config.SiteID → Space.Name().
	Token     string       // Token 访问令牌; 空 → Space.LocalConfig.ServerToken.
	Client    *http.Client // Client 可注入的 HTTP 客户端 (测试用); nil → http.DefaultClient.
}

// Input 是分发阶段的输入.
type Input struct {
	Space *space.Space // Space 本次分发的 Space.
	Opts  Options      // Opts 分发选项.
}

// Result 是一次分发成功的产物.
type Result struct {
	Revision string // Revision 服务端返回的版本标识.
	SiteURL  string // SiteURL 服务端返回的站点地址.
	Files    int    // Files 打包文件数.
	Bytes    int64  // Bytes 压缩包字节数.
}

// Stage 是分发阶段: 实现 pipeline.Stage; 由 CLI 独立触发, 不并入 build.Run.
type Stage struct{}

// Run 执行一次带上下文的分发.
func (Stage) Run(ctx context.Context, in *Input) (*Result, error) {
	if in == nil {
		return nil, fmt.Errorf("distribution: nil input")
	}
	return Distribute(ctx, in.Space, in.Opts)
}

// Distribute 执行一次完整分发: 打包 → 推送 → 解析响应.
func Distribute(ctx context.Context, sp *space.Space, opts Options) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if sp == nil {
		return nil, fmt.Errorf("distribution: nil space")
	}

	serverURL := opts.ServerURL
	if serverURL == "" {
		serverURL = sp.Config.ServerURL
	}
	if serverURL == "" {
		return nil, fmt.Errorf("distribution: 未配置服务端地址 (Space.Config.ServerURL)")
	}
	siteID := opts.SiteID
	if siteID == "" {
		siteID = sp.Config.SiteID
	}
	if siteID == "" {
		siteID = sp.Name()
	}
	token := opts.Token
	if token == "" {
		token = sp.LocalConfig.ServerToken
	}
	client := opts.Client
	if client == nil {
		client = http.DefaultClient
	}

	// 1) 打包 BuildDir.
	bundle, files, err := packDir(sp.BuildDir())
	if err != nil {
		return nil, fmt.Errorf("distribution: 打包构建产物失败: %w", err)
	}
	if files == 0 {
		return nil, fmt.Errorf("distribution: 构建产物为空, 拒绝分发")
	}

	// 2) 组装 multipart 并推送.
	meta, err := json.Marshal(proto.DeployRequest{
		SiteID:      siteID,
		PayloadType: "tar.gz",
		Files:       files,
		Size:        int64(len(bundle)),
	})
	if err != nil {
		return nil, fmt.Errorf("distribution: 序列化元数据失败: %w", err)
	}
	body, contentType, err := buildMultipart(meta, bundle, siteID)
	if err != nil {
		return nil, fmt.Errorf("distribution: 组装请求失败: %w", err)
	}

	endpoint := strings.TrimSuffix(serverURL, "/") + proto.DeployPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("distribution: 构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("distribution: 推送失败: %w", err)
	}
	defer resp.Body.Close()

	var dr proto.DeployResponse
	decodeErr := json.NewDecoder(resp.Body).Decode(&dr)
	if resp.StatusCode != http.StatusOK {
		msg := "服务端拒绝部署"
		if decodeErr == nil && dr.Message != "" {
			msg = dr.Message
		}
		return nil, fmt.Errorf("distribution: %s (%d)", msg, resp.StatusCode)
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("distribution: 解析响应失败: %w", decodeErr)
	}
	if !dr.OK {
		msg := dr.Message
		if msg == "" {
			msg = "服务端返回未成功状态"
		}
		return nil, fmt.Errorf("distribution: %s", msg)
	}

	return &Result{
		Revision: dr.Revision,
		SiteURL:  dr.SiteURL,
		Files:    files,
		Bytes:    int64(len(bundle)),
	}, nil
}

// ==============================
// 打包.
// ==============================

// packDir 把目录打包为 gzip+tar 字节流, 返回 (包体, 文件数, 错误).
// 条目按相对路径排序, 保证包体确定性.
func packDir(dir string) ([]byte, int, error) {
	rels, err := listRelFiles(dir)
	if err != nil {
		return nil, 0, err
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, rel := range rels {
		abs := filepath.Join(dir, filepath.FromSlash(rel))
		info, err := os.Stat(abs)
		if err != nil {
			return nil, 0, err
		}
		f, err := os.Open(abs)
		if err != nil {
			return nil, 0, err
		}
		if err := tw.WriteHeader(&tar.Header{
			Name: rel,
			Mode: 0o644,
			Size: info.Size(),
		}); err != nil {
			f.Close()
			return nil, 0, err
		}
		if _, err := io.Copy(tw, f); err != nil {
			f.Close()
			return nil, 0, err
		}
		f.Close()
	}
	if err := tw.Close(); err != nil {
		return nil, 0, err
	}
	if err := gz.Close(); err != nil {
		return nil, 0, err
	}
	return buf.Bytes(), len(rels), nil
}

// listRelFiles 收集目录下全部文件的相对路径 (正斜杠), 排序.
func listRelFiles(dir string) ([]string, error) {
	var rels []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			return rerr
		}
		rels = append(rels, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(rels)
	return rels, nil
}

// buildMultipart 组装 multipart 请求体: meta 字段 + bundle 文件部件.
func buildMultipart(meta []byte, bundle []byte, siteID string) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("meta", string(meta)); err != nil {
		return nil, "", err
	}
	fw, err := w.CreateFormFile("bundle", siteID+".tar.gz")
	if err != nil {
		return nil, "", err
	}
	if _, err := fw.Write(bundle); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return &buf, w.FormDataContentType(), nil
}
