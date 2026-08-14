package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/emanyzwww/Plainship/internal/i18n"
)

// handleStatus 返回站点的发布状态.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(w, r) {
		return
	}

	siteID := r.PathValue("siteID")
	if !siteIDPattern.MatchString(siteID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": i18n.T(i18n.ServerStatusSiteIDInvalid)})
		return
	}
	active, err := s.activeBuildID(siteID)
	status := map[string]any{
		"ok":        true,
		"siteId":    siteID,
		"active":    active,
		"published": active != "",
	}
	if err != nil {
		status["active"] = ""
		status["published"] = false
	}
	writeJSON(w, http.StatusOK, status)
}

// handleReleaseInfo 返回一次构建的元数据.
// 与 status 接口保持一致, 需要 Bearer Token 鉴权.
func (s *Server) handleReleaseInfo(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(w, r) {
		return
	}
	siteID := r.PathValue("siteID")
	buildID := r.PathValue("buildID")
	if !siteIDPattern.MatchString(siteID) || !buildIDPattern.MatchString(buildID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": i18n.T(i18n.ServerStatusParamsInvalid)})
		return
	}
	data, err := os.ReadFile(filepath.Join(s.releaseDir(siteID, buildID), "release.json"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "message": i18n.T(i18n.ServerStatusNotFound)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "release": json.RawMessage(data)})
}
