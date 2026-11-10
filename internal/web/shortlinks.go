package web

import "net/http"

// shortLinks are short addresses on the game's own site for long ones the
// game prints, such as the MOTD's links (#844). A short link fits a narrow
// screen, never expires, and tracks nobody. The MOTD keeps a long word whole
// so a client can still fold it as a link (owner, 2026-11-10); these keep the
// words short instead.
var shortLinks = map[string]string{
	"/novel":       "https://github.com/pruuk/DOGMud/blob/master/novel/what_the_moons_keep.md",
	"/patch-notes": "https://github.com/pruuk/DOGMud/blob/master/docs/PATCH_NOTES.md",
}

// shortLinkHandler sends a permanent redirect to target.
func shortLinkHandler(target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	}
}

// registerShortLinks adds every short link to the default mux.
func registerShortLinks() {
	for path, target := range shortLinks {
		http.HandleFunc("GET "+path, shortLinkHandler(target))
	}
}
