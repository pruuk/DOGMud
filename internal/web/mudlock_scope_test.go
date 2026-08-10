package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Chunk 3.5 / finding 34 — bound the admin world-lock scope.
//
// The global world lock is the whole game: while it is held nobody acts and no
// round advances. RunWithMUDLocked used to wrap the ENTIRE request, so the lock
// covered authentication (including bcrypt) and the response write to a
// possibly-slow client.
// ---------------------------------------------------------------------------

// The response must be produced under the lock but delivered after it, so a
// stalled client cannot hold the game hostage. The observable proxy for that is
// that the handler's writes are buffered rather than reaching the
// ResponseWriter as the handler runs.
func TestRunWithMUDLocked_ResponseIsBufferedNotStreamed(t *testing.T) {
	var seenDuringHandler int

	rec := httptest.NewRecorder()
	h := RunWithMUDLocked(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(`{"ok":true}`))
		// Nothing may have reached the real writer yet: the lock is still held.
		seenDuringHandler = rec.Body.Len()
	})

	h(rec, httptest.NewRequest(http.MethodGet, "/admin/api/thing", nil))

	assert.Zero(t, seenDuringHandler,
		"the handler's bytes must not reach the client while the world lock is held")

	res := rec.Result()
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusTeapot, res.StatusCode, "status must survive buffering")
	assert.Equal(t, "application/json", res.Header.Get("Content-Type"), "headers must survive buffering")
	assert.Equal(t, `{"ok":true}`, string(body), "body must survive buffering")
}

// A handler that writes nothing must still produce a valid response.
func TestRunWithMUDLocked_EmptyHandlerStillWritesAStatus(t *testing.T) {
	rec := httptest.NewRecorder()

	RunWithMUDLocked(func(w http.ResponseWriter, r *http.Request) {})(
		rec, httptest.NewRequest(http.MethodGet, "/admin/", nil))

	assert.Equal(t, http.StatusOK, rec.Result().StatusCode,
		"a silent handler defaults to 200, matching net/http")
}

// An explicit error status set without a body must survive.
func TestRunWithMUDLocked_ErrorStatusSurvives(t *testing.T) {
	rec := httptest.NewRecorder()

	RunWithMUDLocked(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	})(rec, httptest.NewRequest(http.MethodGet, "/admin/", nil))

	res := rec.Result()
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusForbidden, res.StatusCode)
	assert.Contains(t, string(body), "nope")
}

// Only the FIRST WriteHeader wins, matching net/http semantics, so a handler
// that writes a status and then writes a body does not get overwritten.
func TestRunWithMUDLocked_FirstWriteHeaderWins(t *testing.T) {
	rec := httptest.NewRecorder()

	RunWithMUDLocked(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.WriteHeader(http.StatusInternalServerError) // ignored, as in net/http
		_, _ = w.Write([]byte("body"))
	})(rec, httptest.NewRequest(http.MethodGet, "/admin/", nil))

	assert.Equal(t, http.StatusCreated, rec.Result().StatusCode)
}
