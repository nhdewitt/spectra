package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/nhdewitt/spectra/web"
)

// spaHandler served the embedded React frontend.
// API routes are handled before this, so this only catches non-API paths.
// For any path that doesn't match a static file, it serves index.html
// to support client-side routing.
func spaHandler() http.Handler {
	dist, err := fs.Sub(web.Assets, "dist")
	if err != nil {
		panic("embedded dist not found: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		if f, err := dist.Open(path); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
