package web

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestAssetVersionIsSet guards the cache-busting tag the templates put on
// static links as ?v={{ .ASSETV }}.
func TestAssetVersionIsSet(t *testing.T) {
	if assetVersion == "" {
		t.Fatal("assetVersion is empty: every ?v= link would be blank and browsers would keep stale CSS and JS")
	}
}

// TestTemplateDataCarriesAssetVersion fails if the ASSETV key leaves the
// template data. text/template would then render "?v=<no value>", which
// still looks tagged to the link test below, so check the source directly.
func TestTemplateDataCarriesAssetVersion(t *testing.T) {
	src, err := os.ReadFile("web.go")
	if err != nil {
		t.Fatalf("read web.go: %v", err)
	}
	if !strings.Contains(string(src), `"ASSETV": assetVersion,`) {
		t.Fatal(`web.go's templateData no longer sets "ASSETV": assetVersion, so ?v= links render as "<no value>"`)
	}
}

// TestStaticLinksCarryAssetVersion fails when any public page links a
// stylesheet, script, image, icon or manifest under /static/ without
// ?v={{ .ASSETV }}. Without the tag, browsers keep an old copy after a
// deploy and the page draws with stale styles or a stale logo.
// (manifest.json's own icon list is plain JSON and cannot carry the tag.)
func TestStaticLinksCarryAssetVersion(t *testing.T) {
	root := diagramsRepoRoot(t)
	pages, err := filepath.Glob(filepath.Join(root, "_datafiles", "html", "public", "*.html"))
	if err != nil || len(pages) == 0 {
		t.Fatalf("no public pages found (err %v)", err)
	}
	untagged := regexp.MustCompile(`(?:href|src)="[^"]*/static/[^"?]+\.(?:css|js|png|ico|json|svg|webp|jpg)"`)
	for _, page := range pages {
		data, err := os.ReadFile(page)
		if err != nil {
			t.Fatalf("read %s: %v", page, err)
		}
		if m := untagged.FindAll(data, -1); len(m) > 0 {
			t.Errorf("%s links static files without ?v={{ .ASSETV }}: %q", filepath.Base(page), m)
		}
	}
}
