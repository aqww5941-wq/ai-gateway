package static

import (
	"embed"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
)

// dist is generated directly by `go run ./cmd/build -target frontend`.
// It is the only frontend build output tracked and embedded by the repository.
//
//go:embed dist/*
var dist embed.FS

// SPAHandler returns an http.Handler that serves the embedded React app.
// Requests that don't match a real file fall back to index.html so that
// client-side tab navigation (e.g. /admin/dashboard/routes) works.
func SPAHandler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic("static: embedded dist directory not found — run `go run ./cmd/build` first: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Path relative to /admin/dashboard/ — strip prefix.
		p := strings.TrimPrefix(r.URL.Path, "/admin/dashboard")
		if p == "" {
			p = "/"
		}

		r2 := &http.Request{}
		*r2 = *r
		r2.URL = &url.URL{}
		*r2.URL = *r.URL
		r2.URL.Path = p

		f, err := sub.Open(strings.TrimPrefix(p, "/"))
		if err != nil {
			r2.URL.Path = "/"
		} else {
			f.Close()
		}
		fileServer.ServeHTTP(w, r2)
	})
}
