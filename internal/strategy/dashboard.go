package strategy

import (
	"embed"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
)

//go:embed dashboard_dist/*
var dashboardFS embed.FS

// DashboardHandler returns an http.Handler serving the embedded SPA.
// The caller must strip any path prefix before this handler is called.
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
			f, err = sub.Open("index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			path = "index.html"
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil || stat.IsDir() {
			http.NotFound(w, r)
			return
		}

		ct := mime.TypeByExtension(filepath.Ext(path))
		if ct != "" {
			w.Header().Set("Content-Type", ct)
		}

		if rs, ok := f.(io.ReadSeeker); ok {
			http.ServeContent(w, r, path, stat.ModTime(), rs)
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = io.Copy(w, f)
		}
	})
}
