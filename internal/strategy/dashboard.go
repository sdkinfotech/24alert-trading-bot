package strategy

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dashboard_dist/*
var dashboardFS embed.FS

// DashboardHandler returns an http.Handler serving the embedded SPA under /dashboard/.
// Falls back to index.html for client-side routing.
func DashboardHandler() http.Handler {
	sub, err := fs.Sub(dashboardFS, "dashboard_dist")
	if err != nil {
		return http.NotFoundHandler()
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/dashboard")
		if path == "" || path == "/" {
			path = "/index.html"
		}
		// Try serving the file; if not found, serve index.html for SPA routing
		if _, err := fs.Stat(sub, strings.TrimPrefix(path, "/")); err != nil {
			r.URL.Path = "/index.html"
		} else {
			r.URL.Path = path
		}
		fileServer.ServeHTTP(w, r)
	})
}
