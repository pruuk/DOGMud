package web

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestAssetVersionIsSet guards the cache-busting tag the templates put on
// static links as ?v={{ .ASSETV }}.
func TestAssetVersionIsSet(t *testing.T) {
	if assetVersion == "" {
		t.Fatal("assetVersion is empty: every ?v= link would be blank and browsers would keep stale CSS and JS")
	}
}

// TestStaticLinksCarryAssetVersion fails when a page links a stylesheet or
// script under /static/ without ?v={{ .ASSETV }}. Without the tag, browsers
// keep an old copy after a deploy and the page draws with stale styles.
func TestStaticLinksCarryAssetVersion(t *testing.T) {
	root := diagramsRepoRoot(t)
	untagged := regexp.MustCompile(`/static/(?:css|js)/[^"?]+\.(?:css|js)"`)
	for _, rel := range []string{
		"_datafiles/html/public/_header.html",
		"_datafiles/html/public/webclient-pure.html",
		"_datafiles/html/public/build.html",
	} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if m := untagged.FindAll(data, -1); len(m) > 0 {
			t.Errorf("%s links static files without ?v={{ .ASSETV }}: %q", rel, m)
		}
	}
}
