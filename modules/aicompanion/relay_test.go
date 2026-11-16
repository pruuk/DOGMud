package aicompanion

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/events"
)

// fakeRelay stands in for the owner's browser. Every request it is handed
// arrives on a channel, so a test waits for it instead of polling a slice
// another goroutine is appending to.
type fakeRelay struct {
	sent   chan relayRequest
	module chan string
	refuse bool
}

func newFakeRelay() *fakeRelay {
	return &fakeRelay{sent: make(chan relayRequest, 8), module: make(chan string, 8)}
}

func (f *fakeRelay) send(userId int, module string, payload []byte) bool {
	if f.refuse {
		return false
	}
	var r relayRequest
	_ = json.Unmarshal(payload, &r)
	f.module <- module
	f.sent <- r
	return true
}

// next waits for the request the relay was handed.
func (f *fakeRelay) next(t *testing.T) relayRequest {
	t.Helper()
	select {
	case r := <-f.sent:
		return r
	case <-time.After(5 * time.Second):
		t.Fatal("nothing was sent to the owner's browser")
	}
	return relayRequest{}
}

// answer replies to the next request from owner with status and body.
func (f *fakeRelay) answer(t *testing.T, p *pendingRelays, owner int, status int, body string) {
	t.Helper()
	r := f.next(t)
	if !p.deliver(owner, relayResponse{Id: r.Id, Status: status, Body: body}) {
		t.Error("the owner's reply to a pending id must be accepted")
	}
}

func (p *pendingRelays) pendingCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.byId)
}

func TestRelayRequestCarriesOnlyTheBody(t *testing.T) {
	p := newPendingRelays()
	f := newFakeRelay()
	body := []byte(`{"model":"m","messages":[]}`)
	go f.answer(t, p, 5, 200, `{"choices":[]}`)
	status, raw, err := p.do(context.Background(), 5, body, f.send)
	if err != nil || status != 200 || string(raw) != `{"choices":[]}` {
		t.Fatalf("round trip: %d %q %v", status, raw, err)
	}
	if mod := <-f.module; mod != `Companion.Relay.Request` {
		t.Fatalf("sent as %q", mod)
	}
	// Re-send to capture the wire form of one request.
	go f.answer(t, p, 5, 200, `{}`)
	if _, _, err := p.do(context.Background(), 5, body, func(u int, mod string, payload []byte) bool {
		for _, bad := range []string{`Authorization`, `Bearer`, `http://`, `https://`, `sk-`} {
			if strings.Contains(string(payload), bad) {
				t.Errorf("a relay request must carry only an id and the body, found %q in %s", bad, payload)
			}
		}
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(payload, &fields)
		if len(fields) != 2 || fields[`id`] == nil || string(fields[`body`]) != string(body) {
			t.Errorf("the request is exactly {id, body}: %s", payload)
		}
		return f.send(u, mod, payload)
	}); err != nil {
		t.Fatal(err)
	}
}

// The browser is told how long the server will wait, so it stops a fetch
// whose answer nobody will read (the relay caps it at its own 90 seconds).
func TestRelayRequestCarriesTheDeadline(t *testing.T) {
	p := newPendingRelays()
	f := newFakeRelay()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	go f.answer(t, p, 5, 200, `{}`)
	var fields map[string]json.RawMessage
	if _, _, err := p.do(ctx, 5, []byte(`{}`), func(u int, mod string, payload []byte) bool {
		_ = json.Unmarshal(payload, &fields)
		return f.send(u, mod, payload)
	}); err != nil {
		t.Fatal(err)
	}
	var ms int64
	if err := json.Unmarshal(fields[`deadlineMs`], &ms); err != nil {
		t.Fatalf("a request with a deadline carries deadlineMs: %v %v", fields, err)
	}
	if ms <= 25000 || ms > 30000 {
		t.Fatalf("deadlineMs is the time left on the call, about 30000: %d", ms)
	}
	if len(fields) != 3 {
		t.Fatalf("the request is exactly {id, body, deadlineMs}: %v", fields)
	}

	expired, cancel2 := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel2()
	if got := relayDeadlineMs(expired); got != 1 {
		t.Fatalf("a deadline already past is sent as 1 ms, never 0 or less: %d", got)
	}
	if got := relayDeadlineMs(context.Background()); got != 0 {
		t.Fatalf("no deadline sends none: %d", got)
	}
}

func TestRelayRepliesMatchIdAndOwnerOnce(t *testing.T) {
	p := newPendingRelays()
	f := newFakeRelay()
	done := make(chan error, 1)
	go func() {
		_, _, err := p.do(context.Background(), 5, []byte(`{}`), f.send)
		done <- err
	}()
	id := f.next(t).Id
	if len(id) != 32 {
		t.Fatalf("an id is 128 random bits in hex: %q", id)
	}
	if p.deliver(6, relayResponse{Id: id, Status: 200, Body: `{}`}) {
		t.Fatal("another player's reply must not be accepted")
	}
	if p.deliver(5, relayResponse{Id: `nope`, Status: 200, Body: `{}`}) {
		t.Fatal("an unknown id must not be accepted")
	}
	if !p.deliver(5, relayResponse{Id: id, Status: 200, Body: `{}`}) {
		t.Fatal("the owner's reply with the right id is accepted")
	}
	if p.deliver(5, relayResponse{Id: id, Status: 200, Body: `{}`}) {
		t.Fatal("a second reply to one id is dropped")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRelayTimesOut(t *testing.T) {
	p := newPendingRelays()
	f := newFakeRelay()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, _, err := p.do(ctx, 5, []byte(`{}`), f.send)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("no answer in time is a failed call: %v", err)
	}
	if p.pendingCount() != 0 {
		t.Fatal("a timed-out request leaves nothing pending")
	}
}

func TestRelayCancelStopsTheWait(t *testing.T) {
	p := newPendingRelays()
	f := newFakeRelay()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		f.next(t)
		cancel()
	}()
	_, _, err := p.do(ctx, 5, []byte(`{}`), f.send)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled call stops waiting: %v", err)
	}
	if p.pendingCount() != 0 {
		t.Fatal("a cancelled request leaves nothing pending")
	}
}

func TestRelayUnsentFailsAtOnce(t *testing.T) {
	p := newPendingRelays()
	f := newFakeRelay()
	f.refuse = true
	start := time.Now()
	_, _, err := p.do(context.Background(), 5, []byte(`{}`), f.send)
	if !errors.Is(err, errRelayUnsent) {
		t.Fatalf("a relay the sender cannot reach is errRelayUnsent: %v", err)
	}
	if time.Since(start) > time.Second || p.pendingCount() != 0 {
		t.Fatal("an unsent request neither waits nor stays pending")
	}
	if transient(modelResult{Err: err}) {
		t.Fatal("no browser to relay through is not worth a retry")
	}
}

// A pending call is failed at once when its owner's relay goes away,
// rather than waiting out the deadline for a reply that cannot come.
func TestRelayGoneFailsPendingCalls(t *testing.T) {
	p := newPendingRelays()
	f := newFakeRelay()
	done := make(chan error, 1)
	go func() {
		_, _, err := p.do(context.Background(), 5, []byte(`{}`), f.send)
		done <- err
	}()
	id := f.next(t).Id
	p.abandon(6)
	select {
	case err := <-done:
		t.Fatalf("another owner's relay going must not touch this call: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	p.abandon(5)
	select {
	case err := <-done:
		if !errors.Is(err, errRelayGone) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the call kept waiting after its owner's relay went")
	}
	if p.deliver(5, relayResponse{Id: id, Status: 200, Body: `{}`}) {
		t.Fatal("a reply to an abandoned request is dropped")
	}
}

func TestKeyShapedRepliesAreRefused(t *testing.T) {
	for _, body := range []string{
		`{"x":"sk-proj-abc123def456ghi789"}`,
		`{"x":"sk-or-v1-0123456789abcdef"}`,
		`{"echo":"Authorization: Bearer abc"}`,
		`{"echo":"bearer abcdefghijklmnopqrst"}`,
	} {
		if !looksLikeAKey([]byte(body)) {
			t.Errorf("must refuse %s", body)
		}
	}
	if looksLikeAKey([]byte(`{"choices":[{"message":{"content":"{\"speech\":[{\"kind\":\"say\",\"text\":\"I ask for nothing.\"}]}"}}]}`)) {
		t.Error("an ordinary reply must pass")
	}
	// Words a companion may well say: a bearer of something, an
	// authorization she was given. Only a key or an auth header is refused.
	for _, body := range []string{
		`{"text":"The standard-bearer carrying the banner fell at the gate."}`,
		`{"text":"Bearer of the realm's seal, stand aside."}`,
		`{"text":"Authorization: the captain gave it himself."}`,
	} {
		if looksLikeAKey([]byte(body)) {
			t.Errorf("ordinary words must pass: %s", body)
		}
	}
	for _, body := range []string{
		`{"echo":"Bearer sk-abc"}`,
		`{"echo":"bearer abcdefghijklmnopqrstuvwx"}`,
		`{"echo":"Authorization:Bearer x"}`,
	} {
		if !looksLikeAKey([]byte(body)) {
			t.Errorf("must refuse %s", body)
		}
	}
}

// A key-shaped reply is the guard refusing what came back, not the
// provider failing: it neither trips the owner's breaker nor tells them
// their provider did not answer.
func TestKeyShapedReplyIsNotABreakerFailure(t *testing.T) {
	m := relayModule(t)
	m.relays.ready(5, `m`, false)
	var told []string
	m.tell = func(_ int, text string) { told = append(told, text) }
	now := time.Now()
	for range 5 {
		m.routeResult(route{kind: routeRelay, model: `m`}, 5, apiframework.Ticket{}, errRelayKeyShaped, now)
	}
	if m.route(5).kind != routeRelay {
		t.Fatal("key-shaped replies must not open the owner's breaker")
	}
	if len(told) != 0 {
		t.Fatalf("a key-shaped reply is not the provider failing: %q", told)
	}
}

// The guard is applied to what actually comes back, not only in isolation.
func TestRelayKeyShapedReplyIsDropped(t *testing.T) {
	p := newPendingRelays()
	f := newFakeRelay()
	go f.answer(t, p, 5, 200, `{"choices":[{"message":{"content":"sk-proj-abc123def456ghi789"}}]}`)
	status, raw, err := p.do(context.Background(), 5, []byte(`{}`), f.send)
	if !errors.Is(err, errRelayKeyShaped) || raw != nil || status != 0 {
		t.Fatalf("a key-shaped reply is a failed call with nothing kept: %d %q %v", status, raw, err)
	}
	if transient(modelResult{Err: err}) {
		t.Fatal("a key-shaped reply is not worth a retry")
	}
}

func TestRelayOversizedReplyIsRefused(t *testing.T) {
	p := newPendingRelays()
	f := newFakeRelay()
	go f.answer(t, p, 5, 200, `"`+strings.Repeat(`a`, 1<<20)+`"`)
	_, raw, err := p.do(context.Background(), 5, []byte(`{}`), f.send)
	if !errors.Is(err, errRelayTooLarge) || raw != nil {
		t.Fatalf("a reply over 1 MiB is refused: %v", err)
	}
}

// relayCallModule is a module routing owner 5 through a fake browser.
func relayCallModule(t *testing.T, consented bool) (*AICompanionModule, *fakeRelay) {
	t.Helper()
	m := relayModule(t)
	m.cfg.RelayTimeoutSeconds = 5
	m.relayCalls = newPendingRelays()
	f := newFakeRelay()
	m.relaySend = f.send
	m.consent.agreed = map[int]bool{}
	if consented {
		m.consent.agreed[5] = true
	}
	return m, f
}

func relayCall(m *AICompanionModule) modelCall {
	c := modelCall{BaseURL: `https://api.example.invalid/v1`, APIKey: `sk-server-key-never-sent`,
		Model: `server-model`, Timeout: time.Second, MaxTokens: 100, SchemaName: `decision`,
		Schema: map[string]any{`type`: `object`}, OwnerUserId: 5, Retry: true}
	m.applyRoute(&c)
	return c
}

// The consent door still stands in front of the relay: an owner who has
// not agreed has nothing sent to their browser, and the refusal is neither
// retried nor counted against their breaker.
func TestRelayDoorRefusesAnOwnerWhoHasNotAgreed(t *testing.T) {
	m, f := relayCallModule(t, false)
	c := relayCall(m)
	if c.Route.kind != routeRelay {
		t.Fatal("fixture: the call must route through the relay")
	}
	res := m.callModel(c)
	if !errors.Is(res.Err, errNoConsent) {
		t.Fatalf("an unconsented owner is refused at the door: %v", res.Err)
	}
	select {
	case r := <-f.sent:
		t.Fatalf("nothing may reach the browser, sent %+v", r)
	default:
	}
	m.routeResult(c.Route, 5, apiframework.Ticket{}, res.Err, time.Now())
	if m.relays.owners[5].failures != 0 {
		t.Fatal("a refusal at the door is not the provider's failure")
	}
}

// A relay reply is decoded by the same code as an HTTP one, and the call
// carries the owner's model and nothing of the server's.
func TestRelayCallDecodesLikeHTTP(t *testing.T) {
	m, f := relayCallModule(t, true)
	c := relayCall(m)
	reply := `{"choices":[{"finish_reason":"stop","message":{"content":"{\"speech\":[]}"}}],"usage":{"total_tokens":42}}`
	go func() {
		r := f.next(t)
		var req map[string]any
		if err := json.Unmarshal(r.Body, &req); err != nil {
			t.Errorf("the body is the chat completions JSON: %v", err)
		}
		if req[`model`] != `player-model` {
			t.Errorf("the owner's model is sent: %v", req[`model`])
		}
		if _, ok := req[`reasoning_effort`]; ok {
			t.Error("no reasoning effort on the owner's key")
		}
		m.relayCalls.deliver(5, relayResponse{Id: r.Id, Status: 200, Body: reply})
	}()
	res := m.callModelOnce(c)
	if res.Err != nil || res.Content != `{"speech":[]}` || res.Tokens != 42 || res.Status != 200 {
		t.Fatalf("decoded: %+v", res)
	}

	go func() {
		r := f.next(t)
		m.relayCalls.deliver(5, relayResponse{Id: r.Id, Status: 401, Body: `{"error":"no"}`})
	}()
	res = m.callModelOnce(c)
	if res.Err == nil || res.Status != 401 || !strings.Contains(res.Err.Error(), `status 401`) {
		t.Fatalf("a provider error status is a failed call: %+v", res)
	}
}

// Ready carries only a model name. One that looks like a key, is absurdly
// long or carries control characters does not bring a relay up.
func TestRelayReadyRefusesAnOddModel(t *testing.T) {
	m := relayModule(t)
	m.relays = newRelayTable()
	m.relayCalls = newPendingRelays()
	for _, model := range []string{
		`sk-proj-abc123def456ghi789`,
		strings.Repeat(`m`, 101),
		"gpt\n4",
		`   `,
	} {
		payload, _ := json.Marshal(map[string]string{`model`: model})
		m.onRelayInbound(9, `Companion.Relay.Ready`, payload)
		if _, ok := m.relays.live(9, time.Now()); ok {
			t.Errorf("model %q must not bring a relay up", model)
		}
	}
	m.onRelayInbound(9, `Companion.Relay.Ready`, []byte(`{"model":"openai/gpt-4o-mini"}`))
	if model, ok := m.relays.live(9, time.Now()); !ok || model != `openai/gpt-4o-mini` {
		t.Fatalf("an ordinary model brings it up: %q %v", model, ok)
	}
	m.onRelayInbound(9, `Companion.Relay.Gone`, nil)
	if _, ok := m.relays.live(9, time.Now()); ok {
		t.Fatal("Gone takes it down")
	}
}

// With player keys not on offer the module ignores every relay message.
func TestRelayInboundIgnoredWhenNotOffered(t *testing.T) {
	m := relayModule(t)
	m.relays = newRelayTable()
	m.relayCalls = newPendingRelays()
	m.cfg.PlayerKeys = false
	m.onRelayInbound(9, `Companion.Relay.Ready`, []byte(`{"model":"m"}`))
	if _, ok := m.relays.live(9, time.Now()); ok {
		t.Fatal("no relay comes up while player keys are off")
	}
}

// A reply arriving over GMCP reaches the waiting call.
func TestRelayResponseInboundReachesTheCall(t *testing.T) {
	m, f := relayCallModule(t, true)
	done := make(chan error, 1)
	go func() {
		_, _, err := m.relayCalls.do(context.Background(), 5, []byte(`{}`), f.send)
		done <- err
	}()
	id := f.next(t).Id
	payload, _ := json.Marshal(relayResponse{Id: id, Status: 200, Body: `{}`})
	m.onRelayInbound(6, `Companion.Relay.Response`, payload)
	m.onRelayInbound(5, `Companion.Relay.Response`, payload)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the owner's reply never reached the call")
	}
}

// Logging out takes the owner's relay down and fails what was waiting on it.
func TestDespawnTakesTheRelayDown(t *testing.T) {
	m, f := relayCallModule(t, true)
	m.ctrls = map[int]*controller{}
	done := make(chan error, 1)
	go func() {
		_, _, err := m.relayCalls.do(context.Background(), 5, []byte(`{}`), f.send)
		done <- err
	}()
	f.next(t)
	m.onPlayerDespawn(events.PlayerDespawn{UserId: 5})
	if _, ok := m.relays.live(5, time.Now()); ok {
		t.Fatal("the relay is down after logout")
	}
	select {
	case err := <-done:
		if !errors.Is(err, errRelayGone) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the pending call kept waiting after logout")
	}
}
