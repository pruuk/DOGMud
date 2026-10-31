package web

import (
	"context"
	"net/http"
)

// adminOnlyPages are pages that sit in the public html folder but may only
// be served through an admin-gated route. serveTemplate is the catch-all "/"
// handler and serves any public .html file, and it resolves "/build.html",
// "/build/" and "/build" alike to build.html, so the gate keys on the file
// it resolves to, not on the request path: a route for each spelling would
// always miss one.
var adminOnlyPages = map[string]bool{
	`build.html`:      true,
	`build-help.html`: true,
}

type adminServedKey struct{}

// adminServed reports whether r came through serveBuildPage, which only a
// doBasicAuth-wrapped route reaches.
func adminServed(r *http.Request) bool {
	ok, _ := r.Context().Value(adminServedKey{}).(bool)
	return ok
}

// serveBuildPage renders the admin room-builder page (admin web-building 1b).
//
// Admin gating is applied by the doBasicAuth wrapper at registration time
// (web.go), so by the time this runs the request is an authenticated RoleAdmin
// user. It serves build.html through the standard template pipeline
// (serveTemplate maps the extension-less /build path to build.html), so the
// page can resolve {{ .CONFIG }} values — notably the static-asset base path —
// exactly like the other web pages. It marks the request admin-served, the
// only way serveTemplate will hand out an adminOnlyPages file.
//
// The page is otherwise self-contained: it opens its own WebSocket → GMCP
// session and drives the world through the RoleAdmin-gated Build.* packages.
func serveBuildPage(w http.ResponseWriter, r *http.Request) {
	serveTemplate(w, r.WithContext(context.WithValue(r.Context(), adminServedKey{}, true)))
}
