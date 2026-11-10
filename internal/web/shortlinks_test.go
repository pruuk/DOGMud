package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestShortLinksRedirectPermanently(t *testing.T) {
	if len(shortLinks) == 0 {
		t.Fatal("no short links; this test proves nothing")
	}
	for path, target := range shortLinks {
		rec := httptest.NewRecorder()
		shortLinkHandler(target).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusMovedPermanently {
			t.Errorf("%s answered %d, want 301", path, rec.Code)
		}
		if got := rec.Header().Get("Location"); got != target {
			t.Errorf("%s redirects to %q, want %q", path, got, target)
		}
	}
}

// motdLine reads the text inside the shipped MOTD's {{ t "..." }} wrapper.
var motdLine = regexp.MustCompile(`(?m)^  Motd: '\{\{ t "(.*)" \}\}'$`)

// The shipped MOTD names the short links, and every word in it fits the
// narrowest screen (30 columns), so no line of it runs past the edge.
func TestMotdUsesTheShortLinks(t *testing.T) {
	data, err := os.ReadFile("../../_datafiles/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m := motdLine.FindSubmatch([]byte(strings.ReplaceAll(string(data), "\r\n", "\n")))
	if m == nil {
		t.Fatal("no Server.Motd line in config.yaml")
	}
	motd := strings.ReplaceAll(string(m[1]), `\n`, " ")
	for path := range shortLinks {
		if !strings.Contains(motd, "dogmud.org"+path) {
			t.Errorf("the MOTD does not use the short link dogmud.org%s", path)
		}
	}
	for _, word := range strings.Fields(motd) {
		if len([]rune(word)) > 30 {
			t.Errorf("the MOTD holds a word wider than a 30-column screen: %q", word)
		}
	}
}
