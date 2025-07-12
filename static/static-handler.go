package static

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Options struct {
	Dir         string // filesystem directory to serve
	Prefix      string // url prefix, default "/"
	IndexFile   string // fallback, default "index.html"
	StripPrefix bool   // remove Prefix before fs lookup
	HotReload   bool   // enable hot reload of static files
}

// StaticHandler is a struct-based implementation of http.Handler for serving static files
type StaticHandler struct {
	options Options
	fs      http.FileSystem
}

// ServeHTTP implements the http.Handler interface
func (h *StaticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// If hot reload is enabled, set headers to prevent caching
	if h.options.HotReload {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	}

	upath := r.URL.Path
	if h.options.StripPrefix && strings.HasPrefix(upath, h.options.Prefix) {
		upath = strings.TrimPrefix(upath, h.options.Prefix)
		if upath == "" {
			upath = "/"
		}
	}
	f, err := h.fs.Open(filepath.Clean(upath))
	if err == nil {
		defer f.Close()
		stat, _ := f.Stat()
		if stat.IsDir() {
			index := filepath.Join(upath, h.options.IndexFile)
			if _, err = h.fs.Open(index); err == nil {
				http.ServeFile(w, r, filepath.Join(h.options.Dir, index))
				return
			}
		}
		http.ServeFile(w, r, filepath.Join(h.options.Dir, upath))
		return
	}
	// fallback to index for SPA routes
	if _, err = os.Stat(filepath.Join(h.options.Dir, h.options.IndexFile)); err == nil {
		http.ServeFile(w, r, filepath.Join(h.options.Dir, h.options.IndexFile))
		return
	}
	http.NotFound(w, r)
}

// NewStaticHandler creates a new StaticHandler with the given options
// If HotReload is set to true, the handler will add cache control headers to prevent
// browsers from caching static files, ensuring that the latest version is always served.
// This is useful during development to see changes immediately without having to clear
// the browser cache or restart the server.
func NewStaticHandler(o Options) http.Handler {
	if o.Dir == "" {
		o.Dir = "."
	}
	if o.Prefix == "" {
		o.Prefix = "/"
	}
	if o.IndexFile == "" {
		o.IndexFile = "index.html"
	}

	return &StaticHandler{
		options: o,
		fs:      http.Dir(o.Dir),
	}
}
