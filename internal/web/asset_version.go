package web

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

var (
	assetVersionOnce sync.Once
	assetVersion     string
)

// staticAssetVersion is a short hash of the public stylesheets and scripts,
// worked out once per boot. Pages put it on their links (`gomud.css?v=...`), so
// a deploy that changes any of them sends every returning browser for the new
// copies, while a deploy that changes none keeps their caches. The page HTML is
// a template with no validators, so browsers already fetch that fresh. Without
// this they can pair new HTML with an old stylesheet until their cache expires.
func staticAssetVersion(httpRoot string) string {
	assetVersionOnce.Do(func() {
		assetVersion = computeStaticAssetVersion(httpRoot)
	})
	return assetVersion
}

// computeStaticAssetVersion hashes the path and content of every file under
// static/css and static/js. WalkDir visits in lexical order, so the same files
// always give the same hash. An unreadable file is skipped: a version that
// misses one file still beats a page that cannot be served.
func computeStaticAssetVersion(httpRoot string) string {
	sum := sha256.New()
	for _, sub := range []string{`css`, `js`} {
		root := filepath.Join(httpRoot, `static`, sub)
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			f, err := os.Open(p)
			if err != nil {
				return nil
			}
			defer func() { _ = f.Close() }()
			if rel, err := filepath.Rel(httpRoot, p); err == nil {
				_, _ = io.WriteString(sum, filepath.ToSlash(rel))
			}
			_, _ = io.Copy(sum, f)
			return nil
		})
	}
	return hex.EncodeToString(sum.Sum(nil))[:12]
}
