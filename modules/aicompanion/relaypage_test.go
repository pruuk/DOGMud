package aicompanion

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/companionai"
)

func relayPageModule(t *testing.T) *AICompanionModule {
	t.Helper()
	withWebDomain(t, `example.org`)
	return &AICompanionModule{cfg: Config{Enabled: true, PlayerKeys: true, RelayOrigin: `https://keys.example.org`}}
}

func TestRelayPageServedOnlyOnTheRelayHost(t *testing.T) {
	m := relayPageModule(t)
	req := httptest.NewRequest(http.MethodGet, `https://keys.example.org/companion-relay.html`, nil)
	rec := httptest.NewRecorder()
	if !m.serveRelayPage(rec, req) || rec.Code != 200 {
		t.Fatalf("the relay host serves the page, got %d", rec.Code)
	}
	csp := rec.Header().Get(`Content-Security-Policy`)
	for _, want := range []string{`default-src 'none'`, `script-src 'self'`,
		`connect-src https: http://localhost:* http://127.0.0.1:*`,
		`base-uri 'none'`, `form-action 'none'`} {
		if !strings.Contains(csp, want) {
			t.Fatalf("CSP must contain %q: %s", want, csp)
		}
	}
	// Only the game origin may frame the relay: exactly one source.
	var ancestors []string
	for d := range strings.SplitSeq(csp, `;`) {
		if f := strings.Fields(d); len(f) > 0 && f[0] == `frame-ancestors` {
			ancestors = f[1:]
		}
	}
	if len(ancestors) != 1 || ancestors[0] != `https://example.org` {
		t.Fatalf("frame-ancestors must name exactly the game origin, got %v", ancestors)
	}
	if strings.Contains(csp, `unsafe-inline`) && !strings.Contains(csp, `style-src 'unsafe-inline'`) {
		t.Fatalf("only styles may be inline: %s", csp)
	}
	if strings.Count(csp, `unsafe-inline`) != 1 || strings.Contains(csp, `unsafe-eval`) {
		t.Fatalf("the relay page allows no inline or eval script: %s", csp)
	}
	if got := rec.Header().Get(`Cache-Control`); got != `no-store` {
		t.Fatalf("the relay page is never cached, got %q", got)
	}

	// The game host and any other host are left to the rest of the site.
	for _, host := range []string{`https://example.org/companion-relay.html`, `https://evil.example/companion-relay.html`} {
		if m.serveRelayPage(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, host, nil)) {
			t.Fatalf("%s never serves the relay", host)
		}
	}

	js := httptest.NewRequest(http.MethodGet, `https://keys.example.org/companion-relay.js`, nil)
	jrec := httptest.NewRecorder()
	if !m.serveRelayPage(jrec, js) || jrec.Code != 200 || !strings.Contains(jrec.Header().Get(`Content-Type`), `javascript`) {
		t.Fatal("the relay script is served with a script type")
	}
	if !strings.Contains(jrec.Body.String(), `relayOne`) {
		t.Fatal("the relay script is the embedded relay.js")
	}

	// Everything else on the relay host is claimed and refused, so no game
	// page ever runs on the origin that holds the key: not the pages, not
	// the websocket, not the admin or builder routes.
	for _, path := range []string{`/`, `/webclient-pure.html`, `/static/js/gmcp.js`,
		`/ws`, `/admin/`, `/build`, `/favicon.ico`} {
		rec := httptest.NewRecorder()
		if !m.serveRelayPage(rec, httptest.NewRequest(http.MethodGet, `https://keys.example.org`+path, nil)) || rec.Code != http.StatusNotFound {
			t.Fatalf("%s on the relay host must be claimed and 404, got %d", path, rec.Code)
		}
	}

	post := httptest.NewRecorder()
	if !m.serveRelayPage(post, httptest.NewRequest(http.MethodPost, `https://keys.example.org/companion-relay.html`, nil)) || post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("only GET and HEAD are answered, got %d", post.Code)
	}

	m.cfg.PlayerKeys = false
	if m.serveRelayPage(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, `https://keys.example.org/companion-relay.html`, nil)) {
		t.Fatal("with player keys off nothing is claimed")
	}
}

// The key window is a top-level page: nothing, not even the game page, may
// frame it, so a page drawn over it can never be mistaken for it. Neither
// page has a form or a password field (no password manager is offered the
// key), and the frame has no input at all: the key is typed only in the
// window.
func TestRelaySetupPageIsServedAndNeverFramed(t *testing.T) {
	m := relayPageModule(t)
	rec := httptest.NewRecorder()
	if !m.serveRelayPage(rec, httptest.NewRequest(http.MethodGet, `https://keys.example.org/companion-relay-setup.html`, nil)) || rec.Code != 200 {
		t.Fatalf("the relay host serves the key window, got %d", rec.Code)
	}
	csp := rec.Header().Get(`Content-Security-Policy`)
	if !strings.Contains(csp, `frame-ancestors 'none'`) || strings.Contains(csp, `example.org`) {
		t.Fatalf("the key window may be framed by nobody, got %s", csp)
	}
	for _, want := range []string{`default-src 'none'`, `script-src 'self'`, `form-action 'none'`} {
		if !strings.Contains(csp, want) {
			t.Fatalf("the key window keeps the relay policy %q: %s", want, csp)
		}
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<body data-page="setup">`) || !strings.Contains(body, `<script src="/companion-relay.js"></script>`) {
		t.Fatalf("the key window is the embedded setup page:\n%s", body)
	}
	if strings.Contains(body, `{{`) {
		t.Fatal("the key window has no placeholder to render")
	}

	frame := httptest.NewRecorder()
	m.serveRelayPage(frame, httptest.NewRequest(http.MethodGet, `https://keys.example.org/companion-relay.html`, nil))
	if fcsp := frame.Header().Get(`Content-Security-Policy`); !strings.Contains(fcsp, `frame-ancestors https://example.org`) {
		t.Fatalf("the frame is still framable by the game origin: %s", fcsp)
	}
	for name, page := range map[string]string{`frame`: frame.Body.String(), `key window`: body} {
		lower := strings.ToLower(page)
		if strings.Contains(lower, `<form`) || strings.Contains(lower, `type="password"`) || strings.Contains(lower, `type="submit"`) {
			t.Fatalf("the %s has no form, password field or submit button:\n%s", name, page)
		}
	}
	if strings.Contains(strings.ToLower(frame.Body.String()), `<input`) {
		t.Fatalf("the frame shows no input; the key is typed in the window:\n%s", frame.Body.String())
	}
	for _, id := range []string{`key`, `pass`, `unlockpass`} {
		at := strings.Index(body, `<input id="`+id+`"`)
		if at < 0 {
			t.Fatalf("the key window has the %s input", id)
		}
		tag := body[at : at+strings.Index(body[at:], `>`)]
		if !strings.Contains(tag, `type="text"`) || !strings.Contains(tag, `autocomplete="off"`) || !strings.Contains(tag, `class="secret"`) {
			t.Fatalf("the %s input is plain text, no autocomplete, masked by style: %s", id, tag)
		}
	}
}

func TestRelayPageHostMatchIgnoresPortAndCase(t *testing.T) {
	m := relayPageModule(t)
	req := httptest.NewRequest(http.MethodGet, `/companion-relay.html`, nil)
	req.Host = `KEYS.Example.org:443`
	rec := httptest.NewRecorder()
	if !m.serveRelayPage(rec, req) || rec.Code != 200 {
		t.Fatalf("host with port and other case is still the relay host, got %d", rec.Code)
	}
}

func TestRelayPageNamesTheGameOriginInItsMeta(t *testing.T) {
	m := relayPageModule(t)
	rec := httptest.NewRecorder()
	m.serveRelayPage(rec, httptest.NewRequest(http.MethodGet, `https://keys.example.org/companion-relay.html`, nil))
	body := rec.Body.String()
	if !strings.Contains(body, `<meta name="game-origin" content="https://example.org">`) {
		t.Fatalf("the page names the game origin from config:\n%s", body)
	}
	if strings.Contains(body, `{{GAME_ORIGIN}}`) {
		t.Fatal("the placeholder is replaced")
	}
	// No inline script and no inline handler: the CSP forbids both, so
	// either would be a dead page, and neither may ever be needed.
	if strings.Contains(body, `<script>`) || strings.Contains(strings.ToLower(body), ` on`+`submit=`) ||
		strings.Contains(strings.ToLower(body), ` on`+`click=`) {
		t.Fatal("the relay page has no inline script or handler")
	}
	if !strings.Contains(body, `<body data-page="frame">`) {
		t.Fatal("the relay page is marked as the frame for relay.js")
	}
	if !strings.Contains(body, `<script src="/companion-relay.js"></script>`) {
		t.Fatal("the page loads its script from its own origin")
	}
}

func TestRelayPageEscapesTheGameOrigin(t *testing.T) {
	got := string(renderRelayHTML(`https://x"><script>alert(1)</script>`))
	if strings.Contains(got, `"><script>alert`) {
		t.Fatalf("the game origin is HTML-escaped into the meta tag:\n%s", got)
	}
	if !strings.Contains(got, `content="https://x&#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;"`) {
		t.Fatalf("escaped value not found:\n%s", got)
	}
}

func TestGameOriginNormalisesWebDomain(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`example.org`, `https://example.org`},
		{` Example.ORG `, `https://example.org`},
		{`example.org:8443`, `https://example.org:8443`},
		{`https://example.org/`, `https://example.org`},
		{`http://example.org`, `https://example.org`},
		{`example.org/path`, `https://example.org`},
		{`user@example.org`, ``},
		{``, ``},
		{`exa mple.org`, ``},
		{`x"><script>`, ``},
	} {
		if got := gameOrigin(tc.in); got != tc.want {
			t.Errorf("gameOrigin(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRelayPageRefusedWithoutAGameOrigin(t *testing.T) {
	m := relayPageModule(t)
	withWebDomain(t, ``)
	rec := httptest.NewRecorder()
	if !m.serveRelayPage(rec, httptest.NewRequest(http.MethodGet, `https://keys.example.org/companion-relay.html`, nil)) || rec.Code != http.StatusNotFound {
		t.Fatalf("with no game origin to trust, the relay host serves nothing, got %d", rec.Code)
	}
}

func TestInstallRelayPageFollowsTheOffer(t *testing.T) {
	m := relayPageModule(t)
	t.Cleanup(func() { companionai.SetRelayPage(nil) })
	m.installRelayPage()
	if got := companionai.RelayOrigin(); got != `https://keys.example.org` {
		t.Fatalf("installed with its origin, got %q", got)
	}
	rec := httptest.NewRecorder()
	if !companionai.ServeRelayPage(rec, httptest.NewRequest(http.MethodGet, `https://keys.example.org/companion-relay.html`, nil)) {
		t.Fatal("the engine seam reaches the module's page")
	}
	m.cfg.PlayerKeys = false
	m.installRelayPage()
	if companionai.RelayOrigin() != `` || companionai.ServeRelayPage(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, `https://keys.example.org/companion-relay.html`, nil)) {
		t.Fatal("with player keys not offered the seam is removed")
	}
}
