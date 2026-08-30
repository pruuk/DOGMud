package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A browser's FIRST request to a Basic-auth resource carries no Authorization
// header by design: it expects a 401 with a WWW-Authenticate challenge and only
// then retries with credentials. Counting that first half of the handshake as a
// failed authentication attempt means ordinary page loads manufacture
// "failures", and a dashboard that pulls several resources trips the five-strike
// lockout on its own.
//
// This is what broke the combat, economy and progression dashboards: the JSON
// endpoints answered 429 with a plain-text body, which the page reported as
// `SyntaxError: Unexpected token 'T', "Too many f"... is not valid JSON`.
func TestNoCredentialIsNotAFailedAttempt(t *testing.T) {
	authMu.Lock()
	authFailures = map[string]*authFailureRecord{}
	authMu.Unlock()

	h := doBasicAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Ten unauthenticated requests: the normal handshake, ten times over.
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
		req.RemoteAddr = "203.0.113.7:5000"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code,
			"request %d: a credential-less request must be challenged with 401, "+
				"never throttled with 429 -- the client has not failed anything yet", i)
		assert.NotEmpty(t, rec.Header().Get("WWW-Authenticate"),
			"request %d: a 401 must carry the challenge, or the browser never "+
				"learns to send credentials and can never recover", i)
	}
}

// A lockout must still be escapable. If the 429 omits the challenge header the
// browser is never prompted, so it keeps sending credential-less requests and
// re-locks the moment the window expires: a self-sustaining outage.
func TestThrottledResponseStillCarriesTheChallenge(t *testing.T) {
	const ip = "203.0.113.9"

	authMu.Lock()
	authFailures = map[string]*authFailureRecord{}
	authMu.Unlock()

	h := doBasicAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Present a WRONG credential enough times to lock the source out. These are
	// genuine failures and must count.
	for i := 0; i < authMaxFailures+1; i++ {
		req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
		req.RemoteAddr = ip + ":5000"
		req.SetBasicAuth("nosuchuser", "wrongpassword")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	req.RemoteAddr = ip + ":5000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusTooManyRequests, rec.Code,
		"a wrong credential repeated must still lock the source out")
	assert.NotEmpty(t, rec.Header().Get("WWW-Authenticate"),
		"even a 429 must carry the challenge so the client can recover")
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}
