package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/oceanplexian/lwts/server/internal/config"
)

// renderIndexHTML reads STATIC_DIR/index.html and, when header plugins are
// configured, injects a FOUC guard + the plugin <script defer> tags into
// <head>.
//
// The guard (`<style id="lwts-plugin-fouc">.header-title{visibility:hidden}
// </style>`) hides the default LWTS brand so it never flashes before the
// plugin mounts. The plugin scripts are injected as defer in <head> so they
// load in parallel with the bundle and execute before DOMContentLoaded,
// letting plugins.js mount them on boot with no flash. If no header plugins
// are configured the raw index.html is returned unchanged.
func renderIndexHTML(cfg *config.Config) (string, error) {
	raw, err := os.ReadFile(filepath.Join(cfg.StaticDir, "index.html"))
	if err != nil {
		return "", err
	}
	if len(cfg.HeaderPlugins) == 0 {
		return string(raw), nil
	}

	var scripts strings.Builder
	for _, p := range cfg.HeaderPlugins {
		scripts.WriteString(`<script defer src="`)
		scripts.WriteString(p)
		scripts.WriteString(`"></script>`)
	}
	injection := `<style id="lwts-plugin-fouc">.header-title{visibility:hidden}</style>` + scripts.String()

	html := strings.Replace(string(raw), "<head>", "<head>"+injection, 1)
	if html == string(raw) {
		// Fallback: append before </head> if <head> wasn't matched verbatim.
		html = strings.Replace(string(raw), "</head>", injection+"</head>", 1)
	}
	return html, nil
}

// serveIndex writes the (plugin-injected) index.html with no-cache headers.
// If the pre-rendered HTML is empty (e.g. read failed at boot) it falls back
// to serving the raw file.
func serveIndex(w http.ResponseWriter, r *http.Request, indexHTML, staticDir string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	if indexHTML != "" {
		_, _ = io.WriteString(w, indexHTML)
		return
	}
	http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
}