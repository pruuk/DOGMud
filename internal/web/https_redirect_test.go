package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// ---------------------------------------------------------------------------
// Review finding 20b — the http->https redirect derived its destination from
// the Host header, which is request input. Anyone could hand out a link to this
// server and have visitors bounced somewhere else, under this site's domain
// name, as a CACHEABLE 301.
// ---------------------------------------------------------------------------

// withWebDomain sets the configured canonical hostname for the duration of a
// test.
func withWebDomain(t *testing.T, domain string) {
	t.Helper()
	prev := configs.GetFilePathsConfig().WebDomain
	if err := configs.AddOverlayOverrides(map[string]any{
		"FilePaths.WebDomain": domain,
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{
			"FilePaths.WebDomain": string(prev),
		})
	})
}

// redirectLocation drives the handler and returns the Location header.
func redirectLocation(t *testing.T, hostHeader, target string) (int, string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Host = hostHeader

	rec := httptest.NewRecorder()
	newHttpsRedirectHandler(443)(rec, req)

	res := rec.Result()
	defer res.Body.Close()
	return res.StatusCode, res.Header.Get("Location")
}

// THE test. An attacker-supplied Host must never appear in the destination.
func TestHttpsRedirect_AttackerHostIsNotUsedAsTheDestination(t *testing.T) {
	withWebDomain(t, "mud.example.com")

	for _, evil := range []string{
		"evil.example",
		"evil.example:8080",
		"mud.example.com.evil.example", // suffix trick
		"evilmud.example.com",          // prefix trick
		"EVIL.EXAMPLE",
	} {
		t.Run(evil, func(t *testing.T) {
			status, loc := redirectLocation(t, evil, "/webclient")

			if status != http.StatusMovedPermanently {
				t.Fatalf("status = %d, want 301", status)
			}

			u, err := url.Parse(loc)
			if err != nil {
				t.Fatalf("Location %q did not parse: %v", loc, err)
			}
			if u.Hostname() != "mud.example.com" {
				t.Errorf("redirected to %q; the Host header steered the destination", loc)
			}
			if strings.Contains(strings.ToLower(loc), "evil") {
				t.Errorf("attacker host leaked into the Location header: %q", loc)
			}
		})
	}
}

// A legitimate multi-hostname deployment must keep working: a Host the operator
// has declared is used as-is, so this is not a behaviour regression for anyone
// with a correct configuration.
func TestHttpsRedirect_ConfiguredHostIsHonoured(t *testing.T) {
	withWebDomain(t, "mud.example.com")

	_, loc := redirectLocation(t, "mud.example.com", "/webclient")
	if want := "https://mud.example.com:443/webclient"; loc != want {
		t.Errorf("Location = %q, want %q", loc, want)
	}
}

// Loopback is always allowed, so a developer running locally is not redirected
// off their own machine.
func TestHttpsRedirect_LoopbackIsAllowed(t *testing.T) {
	withWebDomain(t, "mud.example.com")

	for _, host := range []string{"localhost", "localhost:8090", "127.0.0.1"} {
		_, loc := redirectLocation(t, host, "/")
		u, err := url.Parse(loc)
		if err != nil {
			t.Fatalf("Location %q did not parse: %v", loc, err)
		}
		if got := u.Hostname(); got != hostOnly(host) {
			t.Errorf("host %q redirected to %q, want the same host back", host, got)
		}
	}
}

// The path and query must survive, or the redirect breaks deep links.
func TestHttpsRedirect_PreservesPathAndQuery(t *testing.T) {
	withWebDomain(t, "mud.example.com")

	_, loc := redirectLocation(t, "mud.example.com", "/admin/rooms?zone=frostfang&id=42")
	if want := "https://mud.example.com:443/admin/rooms?zone=frostfang&id=42"; loc != want {
		t.Errorf("Location = %q, want %q", loc, want)
	}
}

// An absolute-form request line (legal in HTTP, used by proxies) puts a whole
// URL in RequestURI. Pasting that after the host would produce a nonsense
// target, so the parsed path is used instead.
func TestHttpsRedirect_AbsoluteFormRequestDoesNotSmuggleAHost(t *testing.T) {
	withWebDomain(t, "mud.example.com")

	req := httptest.NewRequest(http.MethodGet, "http://evil.example/pwn", nil)
	req.Host = "mud.example.com"

	rec := httptest.NewRecorder()
	newHttpsRedirectHandler(443)(rec, req)

	loc := rec.Result().Header.Get("Location")
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("Location %q did not parse: %v", loc, err)
	}
	if u.Hostname() != "mud.example.com" {
		t.Errorf("absolute-form request steered the destination: %q", loc)
	}
}

// With nothing configured and an unrecognised Host, there is no safe
// destination to invent -- any host we chose would be one the requester
// supplied. Refusing is the correct answer.
func TestHttpsRedirect_RefusesWhenNothingIsTrusted(t *testing.T) {
	withWebDomain(t, "")

	prevMSSP := configs.GetServerConfig().MSSP.Hostname
	if err := configs.AddOverlayOverrides(map[string]any{"Server.MSSP.Hostname": ""}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{"Server.MSSP.Hostname": string(prevMSSP)})
	})

	status, loc := redirectLocation(t, "evil.example", "/")

	if status == http.StatusMovedPermanently {
		t.Errorf("redirected to %q with no trusted host configured", loc)
	}
	if status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", status)
	}
}

func TestRedirectTargetHost_FallsBackToTheConfiguredDomain(t *testing.T) {
	withWebDomain(t, "mud.example.com")

	host, ok := redirectTargetHost("attacker.example")
	if !ok {
		t.Fatal("expected a usable host")
	}
	if host != "mud.example.com" {
		t.Errorf("host = %q, want the configured domain", host)
	}
}
