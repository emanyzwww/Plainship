// Command server 是 PaperShip 服务端入口: 接收部署包并静态站点.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/emanyzwww/papership-server/internal/api"
	"github.com/emanyzwww/papership-shared/proto"
)

func main() {
	port := envOr("PORT", "8080")
	sitesDir := envOr(api.SitesDirEnv, "./sites")
	if err := os.MkdirAll(sitesDir, 0o755); err != nil {
		log.Fatalf("创建站点目录失败: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle(proto.DeployPath, api.DeployHandler(sitesDir, os.Getenv(api.TokenEnv)))
	mux.Handle("/", http.FileServer(http.Dir(sitesDir)))

	log.Printf("PaperShip server: :%s (sites: %s)", port, sitesDir)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

// envOr 读取环境变量, 为空时返回默认值.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
