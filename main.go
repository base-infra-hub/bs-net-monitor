package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"bs-net-monitor/internal/api"
	"bs-net-monitor/internal/app"
	"bs-net-monitor/internal/conf"
	"bs-net-monitor/internal/detector"
	"bs-net-monitor/pkg/logger"
)

// mimeType 根据文件扩展名返回标准 Content-Type。
func mimeType(ext string) string {
	switch ext {
	case ".html":
		return "text/html; charset=utf-8"
	case ".js", ".mjs":
		return "application/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json", ".map":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".txt":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

// openDistFile 打开 dist 内文件；命中目录或打开失败时返回错误。
func openDistFile(fsys http.FileSystem, name string) (http.File, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	if err != nil || stat.IsDir() {
		f.Close()
		return nil, fmt.Errorf("invalid file: %s", name)
	}
	return f, nil
}

// serveFileOrIndex 从 embed FS 中读取文件；文件不存在且路径无扩展名时回退 index.html（SPA History 模式）。
func serveFileOrIndex(c *gin.Context, fsys http.FileSystem, filePath string) {
	file := strings.Trim(filePath, "/")
	if file == "" {
		file = "index.html"
	}

	f, err := openDistFile(fsys, file)
	if err != nil {
		// 文件不存在且属于前端路由（路径无扩展后缀名），Fallback 回退至 index.html 以支持 History 模式
		if path.Ext(file) == "" {
			file = "index.html"
			f, err = openDistFile(fsys, file)
		}
		if err != nil {
			c.String(http.StatusNotFound, "Not Found")
			return
		}
	}
	defer f.Close()

	c.Status(http.StatusOK)
	c.Header("Content-Type", mimeType(path.Ext(file)))
	io.Copy(c.Writer, f)
}

const webPrefix = "/web"

//go:embed all:dist
var webDist embed.FS

func main() {
	cfg, err := conf.LoadConfig()
	if err != nil {
		log.Fatalf("[配置错误] 加载配置失败: %v", err)
	}

	if err := logger.Init(cfg.Log); err != nil {
		log.Fatalf("[日志错误] 初始化日志失败: %v", err)
	}

	if err := app.InitApp(cfg); err != nil {
		log.Fatalf("[初始化错误] %v", err)
	}

	// 注册优雅停机钩子：收到 SIGINT/SIGTERM 时停止检测引擎
	detector.GetManager().RegisterShutdownHook()

	cfg.PrintConfig()

	r := api.NewRouter()

	// 根路径直接重定向到 /web/
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/web/")
	})

	// 在 /web 路径下提供内嵌的 Web 管理后台，并支持 Vue Router history 模式 fallback
	webFS, err := fs.Sub(webDist, "dist")
	if err != nil {
		log.Fatalf("[静态资源] 初始化失败: %v", err)
	}
	staticFS := http.FS(webFS)
	r.NoRoute(func(c *gin.Context) {
		reqPath := c.Request.URL.Path
		if !strings.HasPrefix(reqPath, webPrefix) {
			c.String(http.StatusNotFound, "Not Found")
			return
		}
		// 规范化无斜杠的情况：/web ➔ /web/
		if reqPath == webPrefix {
			c.Redirect(http.StatusFound, "/web/")
			return
		}
		serveFileOrIndex(c, staticFS, strings.TrimPrefix(reqPath, webPrefix))
	})

	addr := fmt.Sprintf(":%d", cfg.Server.HTTPPort)

	log.Printf("[HTTP] 服务启动于端口 %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("[HTTP] 启动失败: %v", err)
	}
}
