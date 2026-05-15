package strategy

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dashboard_dist/*
var dashboardFS embed.FS

// DashboardHandler returns an http.Handler serving the embedded SPA.
// The caller is responsible for stripping any path prefix before calling.
// Unknown paths fall back to index.html for client-side routing.
func DashboardHandler() http.Handler {
	sub, err := fs.Sub(dashboardFS, "dashboard_dist")
	if err != nil {
		return http.NotFoundHandler()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" || path == "" {
			path = "index.html"
		} else if path[0] == '/' {
			path = path[1:]
		}

		f, err := sub.Open(path)
		if err != nil {
			path = "index.html"
		} else {
			f.Close()
		}

		r.URL.Path = "/" + path
		http.FileServer(http.FS(sub)).ServeHTTP(w, r)
	})
}
