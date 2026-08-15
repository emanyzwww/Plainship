package server

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"github.com/emanyzwww/plainship/internal/i18n"
)

// checkAuth 校验 Bearer Token.
func (s *Server) checkAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.Token == "" {
		return true
	}
	auth := r.Header.Get("Authorization")
	token := strings.TrimPrefix(auth, "Bearer ")
	if subtle.ConstantTimeCompare([]byte(token), []byte(s.Token)) != 1 {
		s.log(slog.LevelWarn, "auth failed", "path", r.URL.Path)
		http.Error(w, i18n.T(i18n.ServerAuthUnauthorized), http.StatusUnauthorized)
		return false
	}
	return true
}
