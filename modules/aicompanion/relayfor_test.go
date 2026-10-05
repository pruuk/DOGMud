package aicompanion

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
)

// The companion lends its key relay to other features (bauble naming)
// through apiframework, only for a player who allowed it on the key page.

func TestRelayIsLentOnlyForAllowedFinds(t *testing.T) {
	m := relayModule(t) // owner 5's relay is up, finds not allowed
	r := relayFor{m: m}
	if _, ok := r.Model(5, apiframework.PurposeFinds); ok {
		t.Fatal("a player who did not tick it lends nothing")
	}
	m.relays.ready(5, `player-model`, true)
	if model, ok := r.Model(5, apiframework.PurposeFinds); !ok || model != `player-model` {
		t.Fatalf("allowed: their model (%q %v)", model, ok)
	}
	if _, ok := r.Model(5, `anything else`); ok {
		t.Fatal("only the purpose they allowed")
	}
	if _, ok := r.Model(6, apiframework.PurposeFinds); ok {
		t.Fatal("one player's key names nobody else's finds")
	}
	m.relays.gone(5)
	if _, ok := r.Model(5, apiframework.PurposeFinds); ok {
		t.Fatal("a relay that went away is not used")
	}
	m.relays.ready(5, `player-model`, true)
	m.cfg.PlayerKeys = false
	if _, ok := r.Model(5, apiframework.PurposeFinds); ok {
		t.Fatal("player keys switched off: nothing is lent")
	}
}

// A bauble request carries no player data, so it passes the consent door
// even for a player who never agreed to the companion's model; the same
// request marked as carrying player data does not.
func TestLentRelayDoorAndBreaker(t *testing.T) {
	m := relayModule(t)
	m.relays.ready(5, `player-model`, true)
	m.relayCalls = newPendingRelays()
	f := newFakeRelay()
	m.relaySend = f.send
	m.cfg.RelayTimeoutSeconds = 5
	m.cfg.RequireConsent = true
	m.bonds = bondState{Users: map[int]*bondRecord{}}
	m.syncConsent() // consent is asked for, and nobody has agreed to anything
	r := relayFor{m: m}

	go f.answer(t, m.relayCalls, 5, 200, `{"choices":[]}`)
	status, raw, sent, err := r.Send(context.Background(), 5, []byte(`{"model":"x"}`), apiframework.CarriesNoPlayerData)
	if err != nil || status != 200 || !sent || string(raw) != `{"choices":[]}` {
		t.Fatalf("no player data: through (%d %v %v)", status, sent, err)
	}
	if _, _, sent, err := r.Send(context.Background(), 5, []byte(`{}`), apiframework.CarriesPlayerData); !errors.Is(err, errNoConsent) || sent {
		t.Fatalf("player data without consent is refused at the door (%v %v)", sent, err)
	}

	for i := 0; i < m.cfg.BreakerErrors; i++ {
		r.Result(5, apiframework.PurposeFinds, errors.New(`provider said no`))
	}
	if _, ok := r.Model(5, apiframework.PurposeFinds); ok {
		t.Fatal("failures feed the player's own breaker")
	}
}

// What another feature reports on a player's key is counted by the
// companion's own rule (routeResult): a relay that went away, a reply refused
// for looking like a key, a refused door or a call given up on never opens
// the player's breaker, which is also their companion's.
func TestLentRelayCountsOnlyWhatTheCompanionCounts(t *testing.T) {
	m := relayModule(t)
	m.relays.ready(5, `player-model`, true)
	r := relayFor{m: m}
	for i := 0; i < m.cfg.BreakerErrors+2; i++ {
		for _, err := range []error{errRelayGone, errRelayKeyShaped, errNoConsent, context.Canceled} {
			r.Result(5, apiframework.PurposeFinds, err)
		}
	}
	if _, ok := r.Model(5, apiframework.PurposeFinds); !ok {
		t.Fatal("none of these is the provider failing")
	}
	if _, ok := m.relays.live(5, time.Now()); !ok {
		t.Fatal("the companion's relay is untouched")
	}
}

// On a server (isolateBooks unset) every module spends from the one shared
// set of books, the one baubles spend from too.
func TestProductionSpendsFromTheSharedBooks(t *testing.T) {
	isolateBooks = false
	t.Cleanup(func() { isolateBooks = true })
	m := &AICompanionModule{}
	if m.fw() != apiframework.Shared() || m.books.Load() != nil {
		t.Fatal("a server's companion uses the shared books")
	}
}

// Analysis item 3: a player's key that serves their companion but not
// bauble naming (the provider refuses the bauble request, again and again)
// pauses naming on that key, and never their companion.
func TestFindsFailuresNeverPauseTheCompanion(t *testing.T) {
	m := relayModule(t)
	m.relays.ready(5, `player-model`, true)
	r := relayFor{m: m}
	for i := 0; i < m.cfg.BreakerErrors+2; i++ {
		r.Result(5, apiframework.PurposeFinds, &apiframework.StatusError{Status: 400})
	}
	if _, ok := r.Model(5, apiframework.PurposeFinds); ok {
		t.Fatal("naming on this key is paused")
	}
	if model, ok := m.relays.live(5, time.Now()); !ok || model != `player-model` {
		t.Fatal("the companion's relay is untouched")
	}
	if m.route(5).kind != routeRelay {
		t.Fatal("she still runs on the owner's key")
	}
	// Nor the other way round: her own success on the key does not lift
	// naming's pause.
	m.relays.success(5)
	if _, ok := r.Model(5, apiframework.PurposeFinds); ok {
		t.Fatal("still paused")
	}
}

// Analysis item 8: the server budget is given back the very hold it gave
// out. The ledger's day is the only day, so the hold carries it.
func TestSettlementReturnsTheLedgersOwnHold(t *testing.T) {
	m := &AICompanionModule{cfg: Config{}}
	tomorrow := time.Now().UTC().Add(24 * time.Hour)
	m.fw().SetClockForTest(func() time.Time { return tomorrow })
	h, ok := m.reserveRoute(route{kind: routeServer}, 1, 0, 900)
	if !ok || h.fw.Tokens != 900 || h.fw.Day != tomorrow.Format(`2006-01-02`) || !h.fw.SpendServer {
		t.Fatalf("held on the ledger's day: %+v", h)
	}
	m.settleRoute(h, 100)
	share := 0
	for _, c := range m.fw().Today().ByConsumer {
		if c.Consumer == apiframework.ConsumerCompanion {
			share = c.Tokens
		}
	}
	if share != 100 || m.fw().Today().Tokens != 100 || m.fw().Today().Outstanding != 0 {
		t.Fatalf("settled to what it used on the ledger's day: share=%d total=%d held=%d",
			share, m.fw().Today().Tokens, m.fw().Today().Outstanding)
	}
}

// A call held back while another probes a half-open breaker made no call:
// it is no error in the tier stats, and no call counted.
func TestAHeldBackCallIsNoError(t *testing.T) {
	m := &AICompanionModule{}
	m.recordCall(tierMain, modelResult{Err: errServerResting})
	if st := m.stats[tierMain]; st != nil && (st.Calls != 0 || st.Errors != 0) {
		t.Fatalf("no call, no error: %+v", st)
	}
	m.fw().SetConsumerBreakerForTest(apiframework.ConsumerCompanion, 0, time.Now().Add(-time.Second))
	probe, ok := m.fw().Allow(apiframework.ConsumerCompanion, time.Now())
	if !ok || !probe.Probing() {
		t.Fatal("fixture: the probe")
	}
	res := m.callModel(modelCall{Route: route{kind: routeServer}})
	if !errors.Is(res.Err, errServerResting) || res.Sent {
		t.Fatalf("a second caller is held back and sends nothing: %v", res.Err)
	}
	m.breakerResult(res.Ticket, res.Err, time.Now())
	if m.fw().ConsumerFailures(apiframework.ConsumerCompanion) != 0 {
		t.Fatal("and it is no failure")
	}
}

// Every feature that makes the world livelier is lent the key by the one
// "Make the world livelier" box, apart from finds: each box lends its own
// purposes and nothing more.
func TestRelayIsLentForLivelyFeaturesOnlyWhenAllowed(t *testing.T) {
	m := relayModule(t)
	r := relayFor{m: m}
	other := apiframework.LivelyPurpose(`another-feature`)
	m.relays.readyFor(5, `player-model`, false, false)
	if _, ok := r.Model(5, apiframework.PurposeNPCIdle); ok {
		t.Fatal("a player who unticked it lends nothing to a lively feature")
	}
	m.relays.readyFor(5, `player-model`, false, true)
	for _, p := range []string{apiframework.PurposeNPCIdle, other} {
		if model, ok := r.Model(5, p); !ok || model != `player-model` {
			t.Fatalf("allowed: every lively feature gets their model (%s %q %v)", p, model, ok)
		}
	}
	if _, ok := r.Model(5, apiframework.PurposeFinds); ok {
		t.Fatal("allowing a livelier world does not lend the key for finds")
	}
	if _, ok := r.Model(5, apiframework.PurposeLively); ok {
		t.Fatal("the bare permission is no feature's purpose")
	}
	m.relays.ready(5, `player-model`, true)
	if _, ok := r.Model(5, apiframework.PurposeNPCIdle); ok {
		t.Fatal("a Ready without the lively choice lends nothing to lively features")
	}
	if _, ok := r.Model(5, apiframework.PurposeFinds); !ok {
		t.Fatal("and still lends it for finds")
	}
}

// Lively features share a permission but not a breaker: a player's provider
// that will not serve one pauses that one on that key only, never another
// lively feature, never finds, never the companion; and the reverse.
func TestEachLivelyFeatureHasItsOwnBreaker(t *testing.T) {
	m := relayModule(t)
	m.relays.readyFor(5, `player-model`, true, true)
	r := relayFor{m: m}
	other := apiframework.LivelyPurpose(`another-feature`)
	for i := 0; i < m.cfg.BreakerErrors+2; i++ {
		r.Result(5, apiframework.PurposeNPCIdle, &apiframework.StatusError{Status: 400})
	}
	if _, ok := r.Model(5, apiframework.PurposeNPCIdle); ok {
		t.Fatal("idle moments on this key are paused")
	}
	if _, ok := r.Model(5, other); !ok {
		t.Fatal("another lively feature is untouched")
	}
	if _, ok := r.Model(5, apiframework.PurposeFinds); !ok {
		t.Fatal("finds are untouched")
	}
	if _, ok := m.relays.live(5, time.Now()); !ok {
		t.Fatal("the companion is untouched")
	}

	m.relays.readyFor(6, `player-model`, true, true)
	for i := 0; i < m.cfg.BreakerErrors+2; i++ {
		r.Result(6, apiframework.PurposeFinds, &apiframework.StatusError{Status: 400})
	}
	if _, ok := r.Model(6, apiframework.PurposeNPCIdle); !ok {
		t.Fatal("finds failing does not pause lively features")
	}
	for i := 0; i < m.cfg.BreakerErrors+2; i++ {
		r.Result(6, `no such purpose`, errors.New(`x`))
		r.Result(6, apiframework.PurposeLively, errors.New(`x`))
	}
	if _, ok := r.Model(6, apiframework.PurposeNPCIdle); !ok {
		t.Fatal("a purpose no box covers counts against nothing")
	}
	r.Result(6, apiframework.PurposeNPCIdle, errors.New(`x`))
	r.Result(6, apiframework.PurposeNPCIdle, nil)
	for i := 0; i < m.cfg.BreakerErrors-1; i++ {
		r.Result(6, apiframework.PurposeNPCIdle, errors.New(`x`))
	}
	if _, ok := r.Model(6, apiframework.PurposeNPCIdle); !ok {
		t.Fatal("a success resets the run of failures")
	}
}
