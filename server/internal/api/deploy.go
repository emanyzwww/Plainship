// Package api 提供服务端 HTTP 端点: 接收部署包, 原子切换站点目录, 提供静态站点服务.
package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/emanyzwww/papership-shared/proto"
)

// SitesDirEnv 是站点存放目录的环境变量名 (默认 "./sites").
const SitesDirEnv = "PAPERSHIP_SITES"

// TokenEnv 是部署令牌的环境变量名; 未设置时不校验鉴权.
const TokenEnv = "PAPERSHIP_TOKEN"

// maxBundleBytes 是部署包体上限.
const maxBundleBytes = 512 << 20 // 512MiB.

// DeployHandler 构造 /api/deploy 端点:
//   - multipart 收到 meta (DeployRequest JSON) + bundle (gzip+tar);
//   - 解包到临时目录后原子切换为站点目录 (旧目录先移走, 失败回滚);
//   - 站点目录名即 SiteID, 直接挂到静态文件服务下.
func DeployHandler(sitesDir, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if token != "" && r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxBundleBytes)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			respondError(w, http.StatusBadRequest, "解析请求失败: "+err.Error())
			return
		}
		var req proto.DeployRequest
		if err := json.Unmarshal([]byte(r.FormValue("meta")), &req); err != nil {
			respondError(w, http.StatusBadRequest, "meta 解析失败: "+err.Error())
			return
		}
		if !safeSiteID(req.SiteID) {
			respondError(w, http.StatusBadRequest, "非法的站点标识")
			return
		}
		f, _, err := r.FormFile("bundle")
		if err != nil {
			respondError(w, http.StatusBadRequest, "缺少 bundle 部件: "+err.Error())
			return
		}
		bundle, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			respondError(w, http.StatusBadRequest, "读取包体失败: "+err.Error())
			return
		}

		// 解包到临时目录.
		staging, err := os.MkdirTemp(sitesDir, req.SiteID+".tmp-")
		if err != nil {
			respondError(w, http.StatusInternalServerError, "创建临时目录失败: "+err.Error())
			return
		}
		if _, err := extractTarGz(bundle, staging); err != nil {
			os.RemoveAll(staging)
			respondError(w, http.StatusBadRequest, "包体解压失败: "+err.Error())
			return
		}

		// 原子切换: live → backup, staging → live; 失败回滚.
		live := filepath.Join(sitesDir, req.SiteID)
		backup := filepath.Join(sitesDir, req.SiteID+".old-"+randHex(4))
		if _, err := os.Stat(live); err == nil {
			if err := os.Rename(live, backup); err != nil {
				os.RemoveAll(staging)
				respondError(w, http.StatusInternalServerError, "移动旧站点失败: "+err.Error())
				return
			}
			defer os.RemoveAll(backup)
		}
		if err := os.Rename(staging, live); err != nil {
			if _, rbErr := os.Stat(backup); rbErr == nil {
				_ = os.Rename(backup, live) // 尽力回滚.
			}
			os.RemoveAll(staging)
			respondError(w, http.StatusInternalServerError, "切换站点目录失败: "+err.Error())
			return
		}

		revision := fmt.Sprintf("%d-%s", time.Now().Unix(), randHex(4))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(proto.DeployResponse{
			OK:       true,
			Revision: revision,
			SiteURL:  "/" + req.SiteID + "/",
			Message:  fmt.Sprintf("已部署 %d 个文件", req.Files),
		})
	}
}

// respondError 输出统一错误 JSON.
func respondError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(proto.DeployResponse{OK: false, Message: msg})
}

// safeSiteID 校验站点标识: 非空, 非 . / .., 不含路径分隔符与盘符符号.
func safeSiteID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	return !strings.ContainsAny(id, `/\:`)
}

// extractTarGz 把 gzip+tar 包解压到 dst; 拒绝越界路径 (.. / 绝对路径 / 盘符).
func extractTarGz(bundle []byte, dst string) (int, error) {
	gz, err := gzip.NewReader(bytesReader(bundle))
	if err != nil {
		return 0, err
	}
	defer gz.Close()
	tw := tar.NewReader(gz)
	files := 0
	for {
		hdr, err := tw.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return files, err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg, tar.TypeRegA:
		default:
			continue // 符号链接等一律忽略, 防越界.
		}
		if !safeTarName(hdr.Name) {
			return files, fmt.Errorf("非法包内路径 %q", hdr.Name)
		}
		abs := filepath.Join(dst, filepath.FromSlash(hdr.Name))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return files, err
		}
		out, err := os.OpenFile(abs, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return files, err
		}
		if _, err := io.CopyN(out, tw, hdr.Size); err != nil {
			out.Close()
			return files, err
		}
		out.Close()
		files++
	}
	return files, nil
}

// safeTarName 校验包内路径: 清洗后不得越出目标目录 (.. 前缀 / 绝对路径 / 盘符).
func safeTarName(name string) bool {
	if name == "" || strings.Contains(name, ":") {
		return false
	}
	clean := path.Clean(filepath.ToSlash(name))
	if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return false
	}
	return true
}

// randHex 生成随机十六进制串 (供 revision 与备份目录名).
func randHex(n int) string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)[:n]
}

// bytesReader 是 bytes.NewReader 的别名, 避免与 io 包撞名.
func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }
