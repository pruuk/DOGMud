package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/companionai"
)

// withPublicHtml points serveTemplate at a scratch web root holding the
// given pages, for the duration of a test.
func withPublicHtml(t *testing.T, pages map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range pages {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prev := httpRoot
	httpRoot = dir
	t.Cleanup(func() { httpRoot = prev })
}

// withFakeRelay installs a relay page that claims every request for
// keys.example.org, the way the aicompanion module does for its relay host:
// the page itself is served, any other path there is a 404.
func withFakeRelay(t *testing.T) {
	t.Helper()
	companionai.SetRelayPage(func(w http.ResponseWriter, r *http.Request) bool {
		if r.Host != `keys.example.org` {
			return false
		}
		if r.URL.Path != `/companion-relay.html` {
			http.NotFound(w, r)
			return true
		}
		_, _ = io.WriteString(w, `relay page`)
		return true
	}, func() string { return `https://keys.example.org` })
	t.Cleanup(func() { companionai.SetRelayPage(nil) })
}

// siteMux is the game's routes as Listen registers them, on a mux of the
// test's own: the page handler at "/", and the routes that are NOT pages.
func siteMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(`/`, serveTemplate)
	for _, route := range []string{`/ws`, `/admin/`, `/build`, `/favicon.ico`} {
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `mux `+r.URL.Path)
		})
	}
	return relayFirst(mux)
}

func serve(t *testing.T, host, target string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Host = host
	rec := httptest.NewRecorder()
	siteMux().ServeHTTP(rec, req)
	return rec.Result()
}

func bodyOf(t *testing.T, res *http.Response) string {
	t.Helper()
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The relay host is answered by the module and nothing else; every other
// host falls through to the ordinary pages.
func TestServeTemplate_RelayHostIsClaimedByTheModuleOnly(t *testing.T) {
	withPublicHtml(t, map[string]string{`companion-relay.html`: `site page`})
	withFakeRelay(t)

	if got := bodyOf(t, serve(t, `keys.example.org`, `/companion-relay.html`)); got != `relay page` {
		t.Fatalf("the relay host must be answered by the module, got %q", got)
	}
	if got := bodyOf(t, serve(t, `mud.example.org`, `/companion-relay.html`)); got != `site page` {
		t.Fatalf("another host must fall through to the site, got %q", got)
	}

	// The claim sits outside the mux: the websocket, the admin pages, the
	// builder and the favicon route are never reached on the relay host, and
	// are untouched on the game host.
	for _, route := range []string{`/ws`, `/admin/`, `/build`, `/favicon.ico`} {
		res := serve(t, `keys.example.org`, route)
		if body := bodyOf(t, res); res.StatusCode != http.StatusNotFound || strings.HasPrefix(body, `mux`) {
			t.Fatalf("%s on the relay host must be claimed and refused before the mux, got %d %q", route, res.StatusCode, body)
		}
		if got := bodyOf(t, serve(t, `mud.example.org`, route)); got != `mux `+route {
			t.Fatalf("%s on the game host must reach its route, got %q", route, got)
		}
	}

	// With no relay installed nothing is claimed anywhere.
	companionai.SetRelayPage(nil)
	if got := bodyOf(t, serve(t, `keys.example.org`, `/ws`)); got != `mux /ws` {
		t.Fatalf("with no relay the wrapper passes everything on, got %q", got)
	}
}

// The game page may frame the relay origin and nothing else; the outer
// /webclient page frames the game page from our own origin, so the policy
// must never reach it.
func TestServeTemplate_GamePageCSPFramesOnlyTheRelay(t *testing.T) {
	withPublicHtml(t, map[string]string{
		`webclient-pure.html`: `game page`,
		`webclient.html`:      `<iframe src="/webclient-pure.html"></iframe>`,
	})

	// No relay offered: no policy, the page behaves as before.
	if csp := serve(t, `mud.example.org`, `/webclient-pure.html`).Header.Get(`Content-Security-Policy`); csp != `` {
		t.Fatalf("with no relay the game page gets no policy, got %q", csp)
	}

	withFakeRelay(t)
	for _, target := range []string{`/webclient-pure.html`, `/webclient-pure`} {
		csp := serve(t, `mud.example.org`, target).Header.Get(`Content-Security-Policy`)
		for _, want := range []string{`frame-src https://keys.example.org`, `object-src 'none'`, `base-uri 'self'`} {
			if !strings.Contains(csp, want) {
				t.Fatalf("%s: policy %q lacks %q", target, csp, want)
			}
		}
		if strings.Contains(csp, `script-src`) || strings.Contains(csp, `default-src`) {
			t.Fatalf("%s: policy %q would block the page's inline and CDN scripts", target, csp)
		}
	}
	for _, target := range []string{`/webclient`, `/webclient.html`} {
		if csp := serve(t, `mud.example.org`, target).Header.Get(`Content-Security-Policy`); csp != `` {
			t.Fatalf("%s frames the game page from our own origin and must get no frame-src policy, got %q", target, csp)
		}
	}
}

// The game page learns the relay origin from a script literal. The web
// server's templates are text/template, which escapes nothing, so the origin
// is JSON-encoded in Go: a hostile value must stay one string literal and
// never close the script element.
func TestRelayOriginJSON_StaysOneScriptLiteral(t *testing.T) {
	hostile := `https://x"</script><script>alert(1)//`
	companionai.SetRelayPage(func(w http.ResponseWriter, r *http.Request) bool { return false },
		func() string { return hostile })
	t.Cleanup(func() { companionai.SetRelayPage(nil) })

	got := relayOriginJSON()
	if strings.ContainsAny(got, `<>`) || strings.Contains(got, `</script`) {
		t.Fatalf("relayOriginJSON let markup through: %s", got)
	}
	var back string
	if err := json.Unmarshal([]byte(got), &back); err != nil || back != hostile {
		t.Fatalf("relayOriginJSON is not one JSON string holding the origin: %s (%v)", got, err)
	}

	withPublicHtml(t, map[string]string{
		`webclient-pure.html`: `<script>window.COMPANION_RELAY_ORIGIN = {{ .COMPANION_RELAY_ORIGIN_JSON }};</script>`,
	})
	page := bodyOf(t, serve(t, `mud.example.org`, `/webclient-pure.html`))
	if strings.Count(page, `</script>`) != 1 || strings.Contains(page, `<script>alert`) {
		t.Fatalf("a hostile relay origin escaped its script literal: %s", page)
	}
}

// The item icon folder is config printed inside a script, so it is encoded the
// same way: a hostile CDN location stays one string literal.
func TestItemIconBaseJSON_StaysOneScriptLiteral(t *testing.T) {
	hostile := `https://cdn"</script><script>alert(1)//`
	got := itemIconBaseJSON(hostile)
	if strings.ContainsAny(got, `<>`) {
		t.Fatalf("itemIconBaseJSON let markup through: %s", got)
	}
	var back string
	if err := json.Unmarshal([]byte(got), &back); err != nil || back != hostile+`/static/images/items` {
		t.Fatalf("itemIconBaseJSON is not one JSON string holding the folder: %s (%v)", got, err)
	}
	if got := itemIconBaseJSON(``); got != `"/static/images/items"` {
		t.Fatalf("with no CDN the folder literal is %s, want \"/static/images/items\"", got)
	}
}

// No inline script on the game page may print a config value raw: inside a
// script the html encoder is the wrong one, so such a value goes through a
// JSON-encoded template value instead (itemIconBaseJSON, relayOriginJSON).
func TestGamePageInlineScriptsPrintNoRawConfig(t *testing.T) {
	page, err := os.ReadFile(filepath.Join(diagramsRepoRoot(t), "_datafiles", "html", "public", "webclient-pure.html"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(page)
	inline := 0
	for {
		start := strings.Index(src, `<script>`)
		if start < 0 {
			break
		}
		end := strings.Index(src[start:], `</script>`)
		if end < 0 {
			t.Fatal("unclosed inline script")
		}
		body := src[start : start+end]
		inline++
		if strings.Contains(body, `.CONFIG.`) {
			t.Errorf("an inline script prints a config value raw: %.120s", body)
		}
		src = src[start+end:]
	}
	if inline == 0 || !strings.Contains(string(page), `window.ITEM_ICON_BASE = {{ .ITEM_ICON_BASE_JSON }};`) {
		t.Fatalf("found %d inline scripts and no ITEM_ICON_BASE_JSON line, so this test proves nothing", inline)
	}
}

// With no relay offered the page gets an empty string, not a missing value.
func TestRelayOriginJSON_EmptyWhenNoRelay(t *testing.T) {
	companionai.SetRelayPage(nil)
	if got := relayOriginJSON(); got != `""` {
		t.Fatalf("with no relay the origin literal is %s, want \"\"", got)
	}
}
