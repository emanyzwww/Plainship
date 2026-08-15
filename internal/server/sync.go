package server

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/emanyzwww/plainship/internal/fsutil"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/protocol"
)

// handleSync 处理 POST /api/v1/sites/{siteID}/sync.
// 流程: 校验 -> 存储 release -> 原子激活.
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	if !siteIDPattern.MatchString(siteID) {
		writeJSON(w, http.StatusBadRequest, protocol.Response{OK: false, Message: i18n.T(i18n.ServerSyncSiteIDInvalid)})
		return
	}
	if !s.checkAuth(w, r) {
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBodySize))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, protocol.Response{OK: false, Message: i18n.T(i18n.ServerSyncBodyTooLarge)})
		return
	}
	var req protocol.Request
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, protocol.Response{OK: false, Message: i18n.T(i18n.ServerSyncBadJSON, err.Error())})
		return
	}
	// 校验协议版本与站点归属.
	if req.ProtocolVersion != protocol.ProtocolVersion {
		writeJSON(w, http.StatusBadRequest, protocol.Response{OK: false, Message: i18n.T(i18n.ServerSyncVersionMismatch, protocol.ProtocolVersion, req.ProtocolVersion)})
		return
	}
	if req.SiteID != siteID {
		writeJSON(w, http.StatusBadRequest, protocol.Response{OK: false, Message: i18n.T(i18n.ServerSyncSiteIDMismatch)})
		return
	}
	if !buildIDPattern.MatchString(req.BuildID) {
		writeJSON(w, http.StatusBadRequest, protocol.Response{OK: false, Message: i18n.T(i18n.ServerSyncBuildIDInvalid)})
		return
	}

	// 写入 release 目录.
	// 全量同步或新 buildID 时清空, 防止失败残留污染本次发布;
	// 同 buildID 重推, 客户端重试, 不清空, 客户端只携带差异文件.
	dir := s.releaseDir(siteID, req.BuildID)
	active, activeErr := s.activeBuildID(siteID)
	if req.FullSync || activeErr != nil || active != req.BuildID {
		if err := os.RemoveAll(dir); err != nil {
			writeJSON(w, http.StatusInternalServerError, protocol.Response{OK: false, Message: i18n.T(i18n.ServerSyncMkdirFail)})
			return
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, protocol.Response{OK: false, Message: i18n.T(i18n.ServerSyncMkdirFail)})
		return
	}
	// 增量同步: 继承当前激活 release 的全部文件作为基底, 保证任何 release 都是完整快照.
	// 全量同步不继承: 服务器整体重建, 防止陈旧文件残留.
	if !req.FullSync && activeErr == nil && active != req.BuildID {
		prevDir := s.releaseDir(siteID, active)
		if fsutil.IsDir(prevDir) {
			if err := fsutil.CopyDir(prevDir, dir); err != nil {
				writeJSON(w, http.StatusInternalServerError, protocol.Response{OK: false, Message: i18n.T(i18n.ServerSyncInheritFail, err.Error())})
				return
			}
		}
	}
	stored := 0
	totalDecoded := 0
	for _, f := range req.Files {
		rel, err := fsutil.SafeRelPath(f.Path)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, protocol.Response{OK: false, Message: i18n.T(i18n.ServerSyncBadPath, f.Path)})
			return
		}
		data, err := base64.StdEncoding.DecodeString(f.Content)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, protocol.Response{OK: false, Message: i18n.T(i18n.ServerSyncBadEncoding, f.Path)})
			return
		}
		if len(data) > MaxFileSize {
			writeJSON(w, http.StatusBadRequest, protocol.Response{OK: false, Message: i18n.T(i18n.ServerSyncFileTooLarge, f.Path)})
			return
		}
		// 限制单请求解码后的总字节数, 防止恶意请求拖垮内存.
		totalDecoded += len(data)
		if totalDecoded > MaxDecodedSize {
			writeJSON(w, http.StatusBadRequest, protocol.Response{OK: false, Message: i18n.T(i18n.ServerSyncBodyTooLarge)})
			return
		}
		if err := fsutil.WriteFile(filepath.Join(dir, rel), data); err != nil {
			writeJSON(w, http.StatusInternalServerError, protocol.Response{OK: false, Message: i18n.T(i18n.ServerSyncWriteFail, f.Path)})
			return
		}
		stored++
	}
	// 应用删除: 先全部校验, 任一非法则整体拒绝, 再实际移除文件.
	for _, d := range req.Deletes {
		if _, err := fsutil.SafeRelPath(d); err != nil {
			writeJSON(w, http.StatusBadRequest, protocol.Response{OK: false, Message: i18n.T(i18n.ServerSyncBadDelete, d)})
			return
		}
	}
	for _, d := range req.Deletes {
		rel, err := fsutil.SafeRelPath(d)
		if err != nil {
			continue // 已在上方校验, 不会走到这里.
		}
		_ = os.Remove(filepath.Join(dir, rel))
	}
	// 写入 release 元数据.
	meta := map[string]any{
		"buildId":         req.BuildID,
		"siteId":          req.SiteID,
		"protocolVersion": req.ProtocolVersion,
		"syncedAt":        time.Now().Format(time.RFC3339),
		"files":           len(req.Files),
		"deletes":         req.Deletes,
	}
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "release.json"), metaJSON, 0o644)

	// 原子激活: 先写临时文件再 rename.
	if err := activateRelease(s, siteID, req.BuildID); err != nil {
		s.log(slog.LevelError, "activate failed", "site", siteID, "build", req.BuildID, "err", err)
		writeJSON(w, http.StatusInternalServerError, protocol.Response{OK: false, Message: i18n.T(i18n.ServerSyncActivateFail, err.Error())})
		return
	}
	s.log(slog.LevelInfo, "sync ok",
		"site", siteID, "build", req.BuildID,
		"files", stored, "deletes", len(req.Deletes), "full", req.FullSync)
	writeJSON(w, http.StatusOK, protocol.Response{
		OK: true, BuildID: req.BuildID, Active: true,
		StoredFiles: stored, DeletedFiles: len(req.Deletes),
		Message: i18n.T(i18n.ServerSyncPublishOK),
	})
}

// activateRelease 原子更新 current.json.
func activateRelease(s *Server, siteID, buildID string) error {
	dir := s.releaseDir(siteID, buildID)
	if !fsutil.Exists(filepath.Join(dir, "index.html")) {
		return i18n.Errorf(i18n.ServerSyncNoIndex)
	}
	ptr := currentPtr{BuildID: buildID, ActivatedAt: time.Now().Format(time.RFC3339Nano)}
	data, err := json.MarshalIndent(ptr, "", "  ")
	if err != nil {
		return err
	}
	target := s.currentFilePath(siteID)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}
