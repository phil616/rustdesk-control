package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var files embed.FS

func Handler() http.Handler {
	root, err := fs.Sub(files, "dist")
	if err != nil {
		panic(err)
	}
	server := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if _, e := fs.Stat(root, p); e != nil {
			if strings.HasPrefix(p, "assets/") || p == "source.tar.gz" || p == "LICENSE" || p == "THIRD-PARTY-NOTICES.txt" {
				http.NotFound(w, r)
				return
			}
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			server.ServeHTTP(w, r2)
			return
		}
		server.ServeHTTP(w, r)
	})
}
