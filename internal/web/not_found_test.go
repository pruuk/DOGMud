package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// notFoundRoot is a tiny public folder: a 404 page, an ordinary image and a
// refused "_" image.
func notFoundRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, `404.html`), []byte(`<p>lost page</p>`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, `ok.png`), []byte(`png-bytes`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, `_hidden.png`), []byte(`secret-bytes`), 0o644))
	prev := httpRoot
	httpRoot = dir
	t.Cleanup(func() { httpRoot = prev })
	return dir
}

// countingWriter records every WriteHeader call, so a second status (Go's
// "superfluous response.WriteHeader") shows up as a count of two.
type countingWriter struct {
	*httptest.ResponseRecorder
	headers int
}

func (c *countingWriter) WriteHeader(code int) {
	c.headers++
	c.ResponseRecorder.WriteHeader(code)
}

// A refused "_" file that is not a page gets one plain 404, never its
// bytes and never a second status.
func TestRefusedNonPageFileIsAPlain404(t *testing.T) {
	notFoundRoot(t)
	w := &countingWriter{ResponseRecorder: httptest.NewRecorder()}
	serveTemplate(w, httptest.NewRequest(http.MethodGet, `/_hidden.png`, nil))
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Equal(t, 1, w.headers, "one status only")
	require.NotContains(t, w.Body.String(), `secret-bytes`)
}

// An ordinary image is still served.
func TestOrdinaryImageIsServed(t *testing.T) {
	notFoundRoot(t)
	rec := httptest.NewRecorder()
	serveTemplate(rec, httptest.NewRequest(http.MethodGet, `/ok.png`, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, `png-bytes`, rec.Body.String())
}

// The mutation fallback picture must not start with "_", or the rule above
// refuses it and every mutation without art shows a broken image.
func TestMutationFallbackPictureIsServable(t *testing.T) {
	dir := publicHtmlDir(t)
	_, err := os.Stat(filepath.Join(dir, `static`, `images`, `mutations`, `generic.png`))
	require.NoError(t, err)
	page, err := os.ReadFile(filepath.Join(dir, `webclient-pure.html`))
	require.NoError(t, err)
	require.NotContains(t, string(page), `/mutations/_`)
	require.Contains(t, string(page), `/static/images/mutations/generic.png`)
}
