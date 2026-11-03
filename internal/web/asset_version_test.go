package web

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestStaticAssetVersionFollowsTheContent(t *testing.T) {
	root := t.TempDir()
	css := filepath.Join(root, `static`, `css`)
	if err := os.MkdirAll(css, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(css, `a.css`)
	if err := os.WriteFile(file, []byte(`one`), 0o644); err != nil {
		t.Fatal(err)
	}

	first := computeStaticAssetVersion(root)
	if first == `` {
		t.Fatal(`no version for a folder with a file in it`)
	}
	if again := computeStaticAssetVersion(root); again != first {
		t.Errorf(`same files gave a new version: %q then %q`, first, again)
	}

	if err := os.WriteFile(file, []byte(`two`), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed := computeStaticAssetVersion(root); changed == first {
		t.Errorf(`changed file kept the version %q`, first)
	}
}

// A returning player must never get new HTML with an old stylesheet or script,
// so every local stylesheet and script a public page links carries the version.
func TestPublicPagesVersionTheirStaticAssets(t *testing.T) {
	public := filepath.Join(`..`, `..`, `_datafiles`, `html`, `public`)
	tag := regexp.MustCompile(`(?:href|src)="\{\{ \.CONFIG\.FilePaths\.WebCDNLocation \}\}/static/(?:css|js)/[^"]+"`)

	pages, err := filepath.Glob(filepath.Join(public, `*.html`))
	if err != nil {
		t.Fatal(err)
	}

	seen := 0
	for _, page := range pages {
		body, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		for _, link := range tag.FindAllString(string(body), -1) {
			seen++
			if !strings.HasSuffix(link, `?v={{ .ASSET_VERSION }}"`) {
				t.Errorf(`%s: %s has no version`, filepath.Base(page), link)
			}
		}
	}
	if seen < 20 {
		t.Errorf(`found only %d asset links; the pattern no longer matches the pages`, seen)
	}
}
