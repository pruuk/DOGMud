// Package lively is what every "make the world livelier" feature shares to
// write something on a player's own key: the turn-taking that spaces calls
// on one key, the call through the companion's key relay held against the
// player's own allowance and fed to the feature's own breaker, and the
// moderation that decides who may read the result.
//
// A lively feature (modules/npcidle, modules/roomlife) brings its own
// purpose (apiframework.LivelyPurpose), consumer, allowance dimension,
// prompt and reply schema. Nothing here ever touches the server's budget or
// its key, except to moderate.
package lively

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
)

// SchemaOverhead is a reply schema, sent with every call and not in the
// messages, in round numbers.
const SchemaOverhead = 150

var (
	// ErrNoRelay is no player key to write it with.
	ErrNoRelay = errors.New(`no player key to write it with`)
	// ErrNotModerated is a result the moderation check flagged.
	ErrNotModerated = errors.New(`moderation flagged it`)
	// ErrUnusable wraps a reply the model gave that the feature's rules
	// refuse: the key answered, so it is not the key's failure.
	ErrUnusable = errors.New(`unusable reply`)
)

// Feature names one lively feature's own counters.
type Feature struct {
	Name              string // for logs: npcidle, roomlife
	Purpose           string // apiframework.LivelyPurpose(Name)
	Consumer          string // the budget's consumer name
	Dim               string // the per-player allowance dimension
	ModerationBreaker string // the moderation check's own breaker
}

// Limits is one feature's live settings for a call.
type Limits struct {
	MinSecondsPerPlayer int
	DailyTokensPerUser  int
	TimeoutSeconds      int
	MaxCompletionTokens int
	ModerateOutput      bool
	ModerationModel     string
}

// Turns spaces one feature's calls on each player's key: one in flight at a
// time, and a gap after each starts. Safe from any goroutine.
type Turns struct {
	mu   sync.Mutex
	busy map[int]bool      // players with a call in flight on their key
	next map[int]time.Time // when each player's key may start another

	// Now and Relay are the clock and the player-key relay
	// (apiframework.PlayerRelay); tests stand others in.
	Now   func() time.Time
	Relay func() apiframework.Relay
}

// NewTurns is a Turns on the real clock and relay.
func NewTurns() *Turns {
	return &Turns{busy: map[int]bool{}, next: map[int]time.Time{}, Now: time.Now, Relay: apiframework.PlayerRelay}
}

// Reserve takes userId's turn for f when their key may be used now: their
// relay is up and allows f's purpose (the "Make the world livelier" box),
// their allowance for f is not spent, and their last call for f is done
// and far enough behind. Release gives it back.
func (t *Turns) Reserve(f Feature, l Limits, userId int) bool {
	if userId <= 0 {
		return false
	}
	r := t.Relay()
	if r == nil {
		return false
	}
	if _, ok := r.Model(userId, f.Purpose); !ok {
		return false
	}
	if l.DailyTokensPerUser > 0 && apiframework.Allowance(f.Dim, userId) >= l.DailyTokensPerUser {
		return false
	}
	now := t.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.busy[userId] || now.Before(t.next[userId]) {
		return false
	}
	t.busy[userId] = true
	t.next[userId] = now.Add(time.Duration(l.MinSecondsPerPlayer) * time.Second)
	return true
}

// Release gives userId's turn back; their gap still runs.
func (t *Turns) Release(userId int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.busy, userId)
}

// Busy reports whether userId has a call in flight (for tests and views).
func (t *Turns) Busy(userId int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.busy[userId]
}

// Ask sends chat through userId's own key for f and returns the reply's
// content once decode accepts it. chat.Model is set to the player's own
// model. It is held against f's allowance for the player only, never the
// server's budget; the outcome feeds f's own breaker on their key: a
// provider error, or content decode rejects with an error not wrapping
// ErrUnusable (the schema ignored), counts; content decode rejects wrapping
// ErrUnusable (the rules refuse what the model wrote) does not. A refused
// reservation, a relay that went away and a call given up on count for
// nothing. Off the mud lock: it touches no game state.
func Ask(ctx context.Context, f Feature, l Limits, r apiframework.Relay, userId int, chat apiframework.Chat, decode func(content string) error) (content string, tokens int, err error) {
	if r == nil {
		return ``, 0, ErrNoRelay
	}
	model, ok := r.Model(userId, f.Purpose)
	if !ok {
		return ``, 0, ErrNoRelay
	}
	chat.Model, chat.Effort = model, ``
	ctx, cancel := context.WithTimeout(ctx, time.Duration(l.TimeoutSeconds)*time.Second)
	defer cancel()
	report := func(err error) {
		if !errors.Is(ctx.Err(), context.Canceled) {
			r.Result(userId, f.Purpose, err)
		}
	}
	body, err := chat.Body()
	if err != nil {
		return ``, 0, err
	}
	prompt := apiframework.EstimateTokens(chat.Messages) + SchemaOverhead
	charges := []apiframework.Charge{{Dim: f.Dim, UserId: userId, Limit: l.DailyTokensPerUser}}
	hold, err := apiframework.Reserve(f.Consumer, prompt+chat.MaxTokens, false, charges...)
	if err != nil {
		// A spent allowance makes no call and is nobody's failure.
		return ``, 0, err
	}
	status, raw, sent, err := r.Send(ctx, userId, body, apiframework.CarriesNoPlayerData)
	if err == nil && status != http.StatusOK {
		// The provider's text about the player's own account is neither
		// kept nor logged; the status says enough.
		err = &apiframework.StatusError{Status: status}
	}
	if err != nil {
		n, _ := apiframework.Charged(0, sent, status, prompt, chat.MaxTokens, true)
		apiframework.Settle(hold, n, false)
		report(err)
		return ``, n, err
	}
	reply := apiframework.DecodeChat(status, raw)
	// The count came through the player's browser, which they can write:
	// held to what one request could cost.
	tokens, _ = apiframework.Charged(reply.Tokens, true, status, prompt, chat.MaxTokens, true)
	apiframework.Settle(hold, tokens, false)
	if reply.Err != nil {
		report(reply.Err)
		return ``, tokens, reply.Err
	}
	if err := decode(reply.Content); err != nil {
		if errors.Is(err, ErrUnusable) {
			report(nil)
		} else {
			report(err)
		}
		return ``, tokens, err
	}
	report(nil)
	return reply.Content, tokens, nil
}

// Moderate decides who may read text a player's key wrote, the baubles
// policy: with ModerateOutput on and the server able to check (a server
// key, the provider and moderation breakers closed), it is checked, and a
// flag or a failed check keeps it out entirely. When the server cannot
// check it, it may be shown to the player whose key wrote it alone
// (keyholderOnly). With ModerateOutput off it is shown to everyone
// unchecked. The check is free and reserves nothing.
func Moderate(f Feature, l Limits, texts ...string) (keyholderOnly bool, err error) {
	if !l.ModerateOutput {
		return false, nil
	}
	now := time.Now()
	s := apiframework.Server()
	if !s.HasKey() || apiframework.Blocked(f.ModerationBreaker, now) {
		return true, nil
	}
	flags, err := apiframework.Moderate(s.Endpoint, l.ModerationModel, time.Duration(l.TimeoutSeconds)*time.Second,
		texts, apiframework.CarriesNoPlayerData, nil)
	apiframework.RecordConsumer(f.ModerationBreaker, err, now)
	if err != nil {
		return false, fmt.Errorf(`moderation: %w`, err)
	}
	for _, flagged := range flags {
		if flagged {
			return false, ErrNotModerated
		}
	}
	return false, nil
}

// Config readers for a lively module's settings, which arrive through the
// plugin config bag as `any` (the same rules as baubles).

// Getter reads one setting.
type Getter func(key string) any

// String reads a string setting.
func String(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

// Bool reads a boolean setting, def when unset.
func Bool(v any, def bool) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		if b, err := strconv.ParseBool(strings.TrimSpace(t)); err == nil {
			return b
		}
	}
	return def
}

// Int reads a whole-number setting, def when unset or unreadable, held to
// lo..hi.
func Int(v any, def int, lo int, hi int) int {
	n := def
	switch t := v.(type) {
	case int:
		n = t
	case int64:
		n = int(t)
	case uint64:
		n = int(t)
	case float64:
		n = int(t)
	case string:
		if x, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			n = x
		}
	}
	return min(max(n, lo), hi)
}
