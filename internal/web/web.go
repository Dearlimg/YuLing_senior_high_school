// Package web serves the browser application embedded in the server binary.
package web

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:static
var assets embed.FS

// DemoImage provides only bundled sample photography for explicit demo seeding.
func DemoImage(name string) ([]byte, error) {
	return assets.ReadFile("static/assets/" + name)
}

// Register keeps application routes refreshable without swallowing missing API routes.
func Register(r *gin.Engine) {
	root, err := fs.Sub(assets, "static")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(root))
	hash := sha256.New()
	if err := fs.WalkDir(root, "assets", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && (strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css")) {
			data, err := fs.ReadFile(root, path)
			if err != nil {
				return err
			}
			_, _ = hash.Write([]byte(path))
			_, _ = hash.Write(data)
		}
		return nil
	}); err != nil {
		panic(err)
	}
	revision := fmt.Sprintf("%x", hash.Sum(nil))[:12]
	versionPrefix := "/assets/" + revision + "/"
	r.GET("/assets/*path", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=3600")
		if strings.HasPrefix(c.Request.URL.Path, versionPrefix) {
			c.Request.URL.Path = "/assets/" + strings.TrimPrefix(c.Request.URL.Path, versionPrefix)
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
			files.ServeHTTP(c.Writer, c.Request)
			return
		}
		if strings.HasSuffix(c.Request.URL.Path, ".js") || strings.HasSuffix(c.Request.URL.Path, ".css") {
			// Unversioned modules must revalidate after a binary deployment.
			c.Header("Cache-Control", "no-cache")
		}
		files.ServeHTTP(c.Writer, c.Request)
	})
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		isPage := path == "/" || path == "/directory" || path == "/register" || path == "/class" || path == "/recover" ||
			path == "/universities" || path == "/moments" || path == "/forum" || path == "/privacy" ||
			path == "/login" || path == "/me" || path == "/admin" ||
			path == "/events" || path == "/annual-report" || path == "/updates"
		if strings.HasPrefix(path, "/alumni/") && !strings.Contains(path, ".") {
			isPage = true
		}
		if c.Request.Method != http.MethodGet || !isPage {
			c.JSON(http.StatusNotFound, gin.H{"code": 40401, "msg": "页面或接口不存在"})
			return
		}
		page, readErr := fs.ReadFile(root, "index.html")
		if readErr != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Header("Cache-Control", "no-cache")
		page = []byte(strings.ReplaceAll(string(page), "__REV__", revision))
		c.Data(http.StatusOK, "text/html; charset=utf-8", page)
	})
}
