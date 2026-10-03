package aicompanion

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Three tiers pay for a companion's thinking. Tier 1 is nobody: she answers
// with her authored lines. Tier 2 is her owner, through their own key held
// in their browser on the relay origin (the relay). Tier 3 is the server's
// key, bounded by the daily budgets and the global breaker. Every call is
// routed once, when it is built (applyRoute), and the route it carries
// decides what it is reserved against, how it settles and which breaker
// its outcome reaches.

// routeKind is who pays for a call and how it travels.
type routeKind int

const (
	routeNone   routeKind = iota // tier 1: set lines, nothing leaves
	routeRelay                   // tier 2: the owner's own key, through their browser
	routeServer                  // tier 3: the server's key
)

type route struct {
	kind  routeKind
	model string // tier 2 only: the owner's chosen model, used for every tier
}

// validRelayOrigin accepts only "https://host[:port]" with no path, query,
// fragment or user, and not on the game's own host: the key must live on an
// origin the game page cannot read.
func validRelayOrigin(origin string, webDomain string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != `https` || u.Host == `` || u.Opaque != `` || u.User != nil ||
		(u.Path != `` && u.Path != `/`) || u.RawQuery != `` || u.Fragment != `` || u.ForceQuery {
		return false
	}
	return !strings.EqualFold(u.Hostname(), gameHostname(webDomain))
}

// gameHostname is the game's own host, read from FilePaths.WebDomain
// exactly as gameOrigin reads it (a pasted scheme, path or port is
// dropped, case is ignored), or "" when it is not a plain host. The relay
// page and the relay origin check share it, so what one calls the game's
// host the other does too.
func gameHostname(webDomain string) string {
	origin := gameOrigin(webDomain)
	if origin == `` {
		return ``
	}
	u, err := url.Parse(origin)
	if err != nil {
		return ``
	}
	return u.Hostname()
}

// playerKeysOffered reports whether tier 2 is on offer at all.
func (m *AICompanionModule) playerKeysOffered() bool {
	return m.cfg.Enabled && m.cfg.PlayerKeys &&
		validRelayOrigin(m.cfg.RelayOrigin, string(configs.GetFilePathsConfig().WebDomain))
}

// relayTable is which owners have a live, unlocked relay, and each one's
// own breaker. It is read from model goroutines and written from the
// connection goroutine, so it has its own lock.
type relayTable struct {
	mu     sync.Mutex
	owners map[int]*relayOwner
}

type relayOwner struct {
	model        string
	finds        bool // the owner allowed their key to name what they find (apiframework.PurposeFinds)
	lively       bool // the owner allowed "Make the world livelier": every apiframework.IsLively purpose
	failures     int
	breakerUntil time.Time
	noticeSent   bool // the owner was told this relay session that she fell back

	// One breaker per lent purpose (finds, and each lively feature's own):
	// another feature's calls on this key have their own, so a request the
	// player's provider will not serve for one never pauses the companion or
	// any other, nor the other way round. Created on first failure.
	lent map[string]*lentBreaker
}

// lentBreaker is one lent purpose's breaker on an owner's key.
type lentBreaker struct {
	failures int
	until    time.Time
}

func newRelayTable() *relayTable { return &relayTable{owners: map[int]*relayOwner{}} }

// ready records that the owner's relay is up with this model, lending it
// for finds when they allowed it and for nothing else. A relay that comes
// back keeps its breaker: reloading the page must not reset it. It starts a
// new relay session for the fallback notice, which may be given once more.
func (t *relayTable) ready(userId int, model string, finds bool) {
	t.readyFor(userId, model, finds, false)
}

// readyFor is ready with every permission the owner gave on the key page:
// finds, and lively (every lively feature at once).
func (t *relayTable) readyFor(userId int, model string, finds bool, lively bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if o := t.owners[userId]; o != nil {
		o.model = strings.TrimSpace(model)
		o.finds = finds
		o.lively = lively
		o.noticeSent = false
		return
	}
	t.owners[userId] = &relayOwner{model: strings.TrimSpace(model), finds: finds, lively: lively}
}

// allows reports whether the owner's key-page choices lend the key for
// purpose: finds for apiframework.PurposeFinds, lively for every
// apiframework.IsLively purpose, nothing for anything else.
func (o *relayOwner) allows(purpose string) bool {
	switch {
	case purpose == apiframework.PurposeFinds:
		return o.finds
	case apiframework.IsLively(purpose):
		return o.lively
	}
	return false
}

// liveFor is live for a purpose other than the companion herself: the
// relay must be up, the companion's breaker and the purpose's own closed
// (a key that is failing her is failing everything), and the owner must
// have allowed that purpose on the key page (relayOwner.allows).
func (t *relayTable) liveFor(userId int, purpose string, now time.Time) (string, bool) {
	model, ok := t.live(userId, now)
	if !ok {
		return ``, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	o := t.owners[userId]
	if o == nil || !o.allows(purpose) {
		return ``, false
	}
	if b := o.lent[purpose]; b != nil && now.Before(b.until) {
		return ``, false
	}
	return model, true
}

// findsResult is one outcome of a finds call on the owner's key, for the
// finds breaker only; the companion's is never touched by it.
func (t *relayTable) findsResult(userId int, failed bool, now time.Time, cfg Config) {
	t.purposeResult(userId, apiframework.PurposeFinds, failed, now, cfg)
}

// purposeResult is one outcome of a lent call on the owner's key, for that
// purpose's breaker only; the companion's and every other purpose's are
// never touched by it. A purpose no key-page choice covers counts against
// nothing.
func (t *relayTable) purposeResult(userId int, purpose string, failed bool, now time.Time, cfg Config) {
	if purpose != apiframework.PurposeFinds && !apiframework.IsLively(purpose) {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	o := t.owners[userId]
	if o == nil {
		return
	}
	b := o.lent[purpose]
	if b == nil {
		if !failed {
			return
		}
		if o.lent == nil {
			o.lent = map[string]*lentBreaker{}
		}
		b = &lentBreaker{}
		o.lent[purpose] = b
	}
	if !failed {
		b.failures = 0
		return
	}
	b.failures++
	if cfg.BreakerErrors > 0 && b.failures >= cfg.BreakerErrors {
		b.until = now.Add(time.Duration(cfg.BreakerSeconds) * time.Second)
		b.failures = 0
	}
}

// gone records that the owner's relay is down (locked, closed, logged out).
// Its breaker is kept, for the same reason.
func (t *relayTable) gone(userId int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if o := t.owners[userId]; o != nil {
		o.model = ``
	}
}

// live returns the owner's model when their relay is up and their own
// breaker is closed.
func (t *relayTable) live(userId int, now time.Time) (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	o := t.owners[userId]
	if o == nil || o.model == `` || now.Before(o.breakerUntil) {
		return ``, false
	}
	return o.model, true
}

// failure counts one failed call against the owner's own breaker, and opens
// it for BreakerSeconds after BreakerErrors in a row.
func (t *relayTable) failure(userId int, now time.Time, cfg Config) {
	t.mu.Lock()
	defer t.mu.Unlock()
	o := t.owners[userId]
	if o == nil {
		return
	}
	o.failures++
	if cfg.BreakerErrors > 0 && o.failures >= cfg.BreakerErrors {
		o.breakerUntil = now.Add(time.Duration(cfg.BreakerSeconds) * time.Second)
		o.failures = 0
	}
}

// noticeDue reports, once per relay session, that the owner should be told
// their companion fell back on set lines.
func (t *relayTable) noticeDue(userId int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	o := t.owners[userId]
	if o == nil || o.noticeSent {
		return false
	}
	o.noticeSent = true
	return true
}

func (t *relayTable) success(userId int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if o := t.owners[userId]; o != nil {
		o.failures = 0
	}
}

// route decides who pays for a call on this owner's behalf: their own
// relay first, then the server's key, else nothing and set lines. A
// passer-by talking to her is still routed by her owner: the owner's key
// pays, and StrangerDailyTokens bounds what the passer-by may spend of it.
//
// With RequirePlayerKey (the default) there is no server key route at all:
// a companion speaks only on the key of the player she is speaking for,
// and to anyone else she says nothing (fallback).
func (m *AICompanionModule) route(ownerId int) route {
	if ownerId > 0 && m.relays != nil && m.playerKeysOffered() {
		if model, ok := m.relays.live(ownerId, time.Now()); ok {
			return route{kind: routeRelay, model: model}
		}
	}
	if m.cfg.Enabled && m.apiKey() != `` && !m.cfg.RequirePlayerKey {
		return route{kind: routeServer}
	}
	return route{kind: routeNone}
}

// applyRoute fills in who pays for a call and, for a player's own key, the
// model they chose. A relay call carries nothing of the server's: no key,
// no endpoint, and no reasoning effort, which a player's provider may not
// accept. Every modelCall is built through here, once, and keeps its route
// to the end: the reservation, the settlement and the breaker all read it.
func (m *AICompanionModule) applyRoute(c *modelCall) {
	c.Route = m.route(c.OwnerUserId)
	if c.Route.kind == routeRelay {
		c.Model, c.Effort, c.APIKey, c.BaseURL = c.Route.model, ``, ``, ``
	}
}

// allowanceCharges names the per-user allowances a call counts against
// (apiframework.Charge), each with its limit from the config now. A call a
// passer-by prompted is theirs: their own StrangerDailyTokens, and what
// passers-by together may spend of this owner's companion
// (StrangerTokensPerOwner, when there is an owner); never the owner's
// allowance. Every other call is the owner's (DailyTokensPerCompanion). 0
// is no cap for any of them.
func (m *AICompanionModule) allowanceCharges(ownerId int, askerId int) []apiframework.Charge {
	if askerId > 0 {
		cs := []apiframework.Charge{{Dim: apiframework.DimCompanionStranger, UserId: askerId, Limit: m.cfg.StrangerDailyTokens}}
		if ownerId > 0 {
			cs = append(cs, apiframework.Charge{Dim: apiframework.DimCompanionStrangersFor, UserId: ownerId, Limit: m.cfg.StrangerTokensPerOwner})
		}
		return cs
	}
	return []apiframework.Charge{{Dim: apiframework.DimCompanionOwner, UserId: ownerId, Limit: m.cfg.DailyTokensPerCompanion}}
}

// reserveRoute holds a call's worst case against whoever pays for it, in
// one check-and-hold step on the ledger (apiframework Reserve: all or
// nothing, under its own lock). The server's key is held against the
// server's budget and the payer's allowances (allowanceCharges). A player's
// own key spends nothing of the server's, so it is held against nothing,
// except that a passer-by's question is still held against their
// StrangerDailyTokens and the owner's StrangerTokensPerOwner: the owner's
// key is not theirs to spend without end.
//
// The hold it returns is what settleRoute takes back.
func (m *AICompanionModule) reserveRoute(r route, ownerId int, askerId int, tokens int) (hold, bool) {
	h := hold{r: r}
	spendServer := false
	switch r.kind {
	case routeServer:
		spendServer = true
	case routeRelay:
		if askerId <= 0 {
			return h, true // the owner's own key, for the owner: nothing to hold
		}
	default:
		return h, false
	}
	fh, err := m.fw().Reserve(apiframework.ConsumerCompanion, tokens, spendServer, m.allowanceCharges(ownerId, askerId)...)
	if err != nil {
		h.refusal = err
		return h, false
	}
	h.fw = fh
	return h, true
}

// hold is one call's reservation, as reserveRoute made it: the route, the
// ledger's own hold (empty when nothing was held), and, when reserveRoute
// said no, the ledger's refusal (apiframework.RefusedBy names its counter;
// nil when there was no route at all).
type hold struct {
	r       route
	fw      apiframework.Hold
	refusal error
}

// settleRoute settles a reservation made by reserveRoute, exactly once. The
// ledger applies every rule: a count relayed through the owner's browser
// is held between nothing and the reservation, no counter goes below
// nothing, and a hold from an earlier day gives nothing back to today's.
func (m *AICompanionModule) settleRoute(h hold, used int) {
	if h.fw.Consumer == `` {
		return
	}
	m.fw().Settle(h.fw, used, false)
}

// routeResult feeds a call's outcome to the breaker of whoever paid: the
// owner's own for their key, so one player's broken provider cannot stop
// everyone's companions, and the global one for the server's.
func (m *AICompanionModule) routeResult(r route, ownerId int, t apiframework.Ticket, err error, now time.Time) {
	if r.kind != routeRelay {
		m.breakerResult(t, err, now)
		return
	}
	if m.relays == nil || errors.Is(err, errNoConsent) || errors.Is(err, errRelayGone) ||
		errors.Is(err, errRelayKeyShaped) || errors.Is(err, context.Canceled) {
		// The door refused a request that never left, the owner's page
		// went away, the guard refused a reply for looking like a key, or
		// the module gave up on the answer: none of them is the provider
		// failing.
		return
	}
	if err == nil {
		m.relays.success(ownerId)
		return
	}
	m.relays.failure(ownerId, now, m.cfg)
	m.noticeFallback(ownerId)
}

// noticeFallback tells the owner, once per relay session, that their
// companion fell back on set lines because their key's provider did not
// answer: in plain words, never the error, which may carry the provider's
// own text. It is reached only for a failure routeResult counts against the
// owner, so a relay that went away or a call the module gave up on says
// nothing. Runs under the mud lock, as every routeResult does.
func (m *AICompanionModule) noticeFallback(ownerId int) {
	if !m.relays.noticeDue(ownerId) {
		return
	}
	name := `Your companion`
	if c := m.ctrls[ownerId]; c != nil && c.profile != nil {
		name = c.profile.Name
	}
	m.tellOwner(ownerId, fmt.Sprintf(`(%s falls back on a few set words: your key's provider did not answer.)`, name))
}

// tellOwner sends a player a system line, wrapped at 80 columns: the
// system category is never wrapped for them.
func (m *AICompanionModule) tellOwner(userId int, text string) {
	text = messaging.WrapAnsi(text, 80)
	if m.tell != nil {
		m.tell(userId, text)
		return
	}
	if u := users.GetByUserId(userId); u != nil {
		u.SendText(messaging.CategorySystem, text)
	}
}
