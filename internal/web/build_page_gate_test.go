package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedPublicRoot points serveTemplate at a throwaway public folder holding
// the two admin builder pages, a public page and the 404 page.
func seedPublicRoot(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"build.html":      "BUILD-PAGE",
		"build-help.html": "BUILD-HELP-PAGE",
		"index.html":      "INDEX-PAGE",
		"404.html":        "NOT-FOUND-PAGE",
		"_header.html":    `{{define "header"}}{{end}}`,
		"_footer.html":    `{{define "footer"}}{{end}}`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	orig := httpRoot
	httpRoot = dir
	t.Cleanup(func() { httpRoot = orig })
}

func get(t *testing.T, h http.HandlerFunc, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

// The builder pages live in the public folder, so serveTemplate (the
// catch-all "/" handler) must refuse them on every spelling of the path;
// only the doBasicAuth-wrapped serveBuildPage may serve them.
func TestServeTemplate_RefusesAdminBuilderPages(t *testing.T) {
	seedPublicRoot(t)
	for _, target := range []string{"/build.html", "/build/", "/build", "/build-help.html", "/build-help/", "/build-help", "/./build.html", "//build.html"} {
		rec := get(t, serveTemplate, target)
		body := rec.Body.String()
		if rec.Code != http.StatusNotFound || strings.Contains(body, "BUILD") {
			t.Errorf("%s: status %d body %q, want 404 without the builder page", target, rec.Code, body)
		}
	}
	if rec := get(t, serveTemplate, "/index.html"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "INDEX-PAGE") {
		t.Errorf("a public page must still serve: status %d body %q", rec.Code, rec.Body.String())
	}
}

func TestServeBuildPage_ServesTheBuilderPages(t *testing.T) {
	seedPublicRoot(t)
	for target, want := range map[string]string{"/build": "BUILD-PAGE", "/build-help": "BUILD-HELP-PAGE"} {
		rec := get(t, serveBuildPage, target)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s through serveBuildPage: status %d body %q, want %s", target, rec.Code, rec.Body.String(), want)
		}
	}
}
