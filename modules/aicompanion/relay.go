package aicompanion

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// The relay carries a companion's model request to her owner's own browser,
// which adds the owner's key on the relay origin and posts it to the
// owner's provider, and carries the provider's raw reply back. Nothing of
// the key, the endpoint or any header passes through the server: a request
// is an id and a chat completions body, a reply an id, a status and a body.

// relayRequest is all that goes to the owner's browser: an id, the chat
// completions body, and how long the server will wait for the answer. No
// key, no URL, no header: the relay page adds its own. The relay stops its
// fetch at DeadlineMs, since an answer after it is paid for and never read.
type relayRequest struct {
	Id         string          `json:"id"`
	Body       json.RawMessage `json:"body"`
	DeadlineMs int64           `json:"deadlineMs,omitempty"`
}

// relayDeadlineMs is how long, from now, ctx lets a call wait: at least a
// millisecond, or 0 (not sent) when ctx sets no deadline.
func relayDeadlineMs(ctx context.Context) int64 {
	dl, ok := ctx.Deadline()
	if !ok {
		return 0
	}
	return max(time.Until(dl).Milliseconds(), 1)
}

// relayResponse is what comes back: the provider's status and raw body.
type relayResponse struct {
	Id     string `json:"id"`
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// relaySender delivers one GMCP message to a player's client and reports
// whether it could (companionai.SendRelay in the game).
type relaySender func(userId int, module string, payload []byte) bool

type pendingRelay struct {
	owner  int
	reply  chan relayResponse
	gone   chan struct{} // closed when the owner's relay goes away
	closed bool
}

// pendingRelays matches replies to the calls waiting for them. Replies
// arrive on connection goroutines and waits run on model goroutines, so it
// has its own lock and never touches game state.
type pendingRelays struct {
	mu      sync.Mutex
	byId    map[string]*pendingRelay
	dropped atomic.Int64 // replies refused: unknown id, wrong owner, or a second reply
}

func newPendingRelays() *pendingRelays { return &pendingRelays{byId: map[string]*pendingRelay{}} }

// maxRelayReply is the largest reply body accepted, the same bound the HTTP
// path reads. The websocket caps an inbound frame well below it, so this is
// a second line.
const maxRelayReply = 1 << 20

var (
	errRelayUnsent    = errors.New(`the owner has no browser to relay through`)
	errRelayGone      = errors.New(`the owner's relay went away before it answered`)
	errRelayKeyShaped = errors.New(`a relay reply looked like it carried a key; dropped`)
	errRelayTooLarge  = errors.New(`a relay reply was too large; dropped`)
	errRelayTimeout   = errors.New(`the owner's browser did not answer in time`)
)

// relayFinal reports relay failures a second try cannot mend: nobody to
// send to, the relay gone, a browser that stayed silent for the whole
// wait, or a reply refused for what it was. A silent browser is not asked
// again: the retry would hold the owner's companion for another full wait
// on a page that is most likely closed or asleep.
func relayFinal(err error) bool {
	return errors.Is(err, errRelayUnsent) || errors.Is(err, errRelayGone) ||
		errors.Is(err, errRelayKeyShaped) || errors.Is(err, errRelayTooLarge) ||
		errors.Is(err, errRelayTimeout)
}

func newRelayId() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// do sends one request body to the owner's browser and waits for the reply
// until ctx ends. It is reached only through sendRelay, behind the consent
// door. A nil table sends nothing.
func (p *pendingRelays) do(ctx context.Context, owner int, body []byte, send relaySender) (int, []byte, error) {
	if p == nil || send == nil {
		return 0, nil, errRelayUnsent
	}
	id := newRelayId()
	w := &pendingRelay{owner: owner, reply: make(chan relayResponse, 1), gone: make(chan struct{})}
	p.mu.Lock()
	p.byId[id] = w
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		if p.byId[id] == w {
			delete(p.byId, id)
		}
		p.mu.Unlock()
	}()

	payload, err := json.Marshal(relayRequest{Id: id, Body: body, DeadlineMs: relayDeadlineMs(ctx)})
	if err != nil {
		return 0, nil, err
	}
	if !send(owner, `Companion.Relay.Request`, payload) {
		return 0, nil, errRelayUnsent
	}
	select {
	case r := <-w.reply:
		if len(r.Body) > maxRelayReply {
			return 0, nil, errRelayTooLarge
		}
		if looksLikeAKey([]byte(r.Body)) {
			noteKeyShaped(owner)
			return 0, nil, errRelayKeyShaped
		}
		return r.Status, []byte(r.Body), nil
	case <-w.gone:
		return 0, nil, errRelayGone
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return 0, nil, fmt.Errorf(`%w: %w`, errRelayTimeout, ctx.Err())
		}
		return 0, nil, ctx.Err()
	}
}

// deliver hands a reply to the call waiting for it. It accepts a reply only
// from the owner the request went to, only for an id still pending, and
// only once.
func (p *pendingRelays) deliver(from int, r relayResponse) bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	w := p.byId[r.Id]
	if w == nil || w.owner != from {
		p.mu.Unlock()
		p.dropped.Add(1)
		return false
	}
	delete(p.byId, r.Id)
	p.mu.Unlock()
	w.reply <- r // buffered, and only this one send ever happens
	return true
}

// abandon fails every call waiting on this owner's relay at once: the
// browser that would have answered is gone.
func (p *pendingRelays) abandon(owner int) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, w := range p.byId {
		if w.owner != owner {
			continue
		}
		delete(p.byId, id)
		if !w.closed {
			w.closed = true
			close(w.gone)
		}
	}
}

// keyShaped matches what an API key or an auth header looks like. A reply
// that contains one is a bug somewhere (the relay echoing its headers, or a
// provider echoing the request), and is dropped rather than parsed or logged.
// "Bearer" and "authorization" alone are ordinary words a companion may say
// (a standard-bearer, an authorization from the captain), so they count only
// as an auth header would carry them: "authorization: bearer", or "bearer"
// followed by an sk- key or a token-length run of key characters.
var keyShaped = regexp.MustCompile(`(?i)(\bsk-[a-z0-9_-]{8,}|authorization\s*:\s*bearer|\bbearer\s+(sk-|[a-z0-9._-]{20,}))`)

func looksLikeAKey(b []byte) bool { return keyShaped.Match(b) }

var keyShapedLog = struct {
	sync.Mutex
	last int64
}{}

// noteKeyShaped logs a dropped key-shaped reply at most once a minute, and
// never the text that matched.
func noteKeyShaped(owner int) {
	keyShapedLog.Lock()
	now := time.Now().Unix()
	if now-keyShapedLog.last < 60 {
		keyShapedLog.Unlock()
		return
	}
	keyShapedLog.last = now
	keyShapedLog.Unlock()
	mudlog.Warn(`aicompanion`, `action`, `relayReply`, `owner`, owner,
		`error`, `a relay reply looked like it carried a key and was dropped unread`)
}

// maxRelayModel bounds the model name a relay announces. Real names are a
// few dozen characters ("openai/gpt-4o-mini", "llama3.1:8b").
const maxRelayModel = 100

// relayModelOK accepts a model name a relay announces: short, printable, and
// nothing that looks like a key, since the name is echoed in every request
// and shown in status.
func relayModelOK(model string) bool {
	if model == `` || len(model) > maxRelayModel || looksLikeAKey([]byte(model)) {
		return false
	}
	return !strings.ContainsFunc(model, func(r rune) bool {
		return !unicode.IsPrint(r) || unicode.IsSpace(r)
	})
}

// onRelayInbound receives Companion.Relay.* from a player's client. It runs
// on the connection goroutine, not under the mud lock, and touches only the
// relay tables, which have their own locks.
func (m *AICompanionModule) onRelayInbound(userId int, command string, payload []byte) {
	if !m.playerKeysOffered() {
		return
	}
	switch command {
	case `Companion.Relay.Ready`:
		var r struct {
			Model string `json:"model"`
			// Finds is the owner's "Also name things I find while
			// searching" on the key page: their key may name their
			// finds (baubles). Absent means no.
			Finds bool `json:"finds"`
			// Lively is the owner's "Make the world livelier" on the key
			// page: their key may be lent to every feature that makes the
			// world livelier around them (apiframework.IsLively purposes,
			// such as townsfolk idle moments). The page starts it ticked
			// and always sends it; absent (an older page) means no.
			Lively bool `json:"lively"`
		}
		if json.Unmarshal(payload, &r) != nil {
			return
		}
		model := strings.TrimSpace(r.Model)
		if relayModelOK(model) {
			m.relays.readyFor(userId, model, r.Finds, r.Lively)
		}
	case `Companion.Relay.Gone`:
		m.relayGone(userId)
	case `Companion.Relay.Response`:
		var r relayResponse
		if json.Unmarshal(payload, &r) == nil {
			m.relayCalls.deliver(userId, r)
		}
	}
}

// relayGone takes the owner's relay down and fails whatever was waiting on
// it, on a Gone from the client and on logout.
func (m *AICompanionModule) relayGone(userId int) {
	if m.relays != nil {
		m.relays.gone(userId)
	}
	m.relayCalls.abandon(userId)
}
