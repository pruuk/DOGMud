package aicompanion

import (
	"context"
	"errors"
	"fmt"
	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// withWebDomain pins the game's own host for one test. Test binaries never
// read config.yaml, so the ambient value is a Go default nobody chose.
func withWebDomain(t *testing.T, domain string) {
	t.Helper()
	prev := configs.GetFilePathsConfig().WebDomain
	if err := configs.AddOverlayOverrides(map[string]any{`FilePaths.WebDomain`: domain}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{`FilePaths.WebDomain`: string(prev)})
	})
}

// relayModule is a module offering player keys, with owner 5's relay up.
func relayModule(t *testing.T) *AICompanionModule {
	t.Helper()
	withWebDomain(t, `example.org`)
	freshServer(t, 5000, 3, 60)
	m := &AICompanionModule{cfg: Config{Enabled: true, PlayerKeys: true, RelayOrigin: `https://keys.example.org`,
		BreakerErrors: 3, BreakerSeconds: 60,
		DailyTokensPerCompanion: 1000, StrangerDailyTokens: 1000}}
	m.relays = newRelayTable()
	m.relays.ready(5, `player-model`, false)
	return m
}

func TestRelayOriginMustBeHTTPSAndForeign(t *testing.T) {
	for _, tc := range []struct {
		origin, web string
		ok          bool
	}{
		{`https://keys.example.org`, `example.org`, true},
		{`https://keys.example.org:8443`, `example.org`, true},
		{`https://keys.example.org/`, `example.org`, true},
		{`http://keys.example.org`, `example.org`, false},
		{`https://example.org`, `example.org`, false},
		{`https://EXAMPLE.org`, `example.org`, false},
		{`https://example.org:8443`, `example.org`, false},
		{`https://example.org`, `example.org:8080`, false},
		{`https://keys.example.org/path`, `example.org`, false},
		{`https://keys.example.org?x=1`, `example.org`, false},
		{`https://keys.example.org#x`, `example.org`, false},
		{`https://user@keys.example.org`, `example.org`, false},
		{`keys.example.org`, `example.org`, false},
		{``, `example.org`, false},
		// WebDomain as an operator may paste it, normalised as gameOrigin does.
		{`https://example.org`, `https://example.org`, false},
		{`https://example.org`, `example.org/`, false},
		{`https://example.org`, ` HTTPS://Example.ORG:8443/play `, false},
		{`https://keys.example.org`, `https://example.org/`, true},
	} {
		if got := validRelayOrigin(tc.origin, tc.web); got != tc.ok {
			t.Errorf("validRelayOrigin(%q, %q) = %v, want %v", tc.origin, tc.web, got, tc.ok)
		}
	}
}

func TestPlayerKeysAreOfferedOnlyOnAForeignOrigin(t *testing.T) {
	m := relayModule(t)
	if !m.playerKeysOffered() {
		t.Fatal("a valid relay origin on another host offers player keys")
	}
	withWebDomain(t, `keys.example.org`)
	if m.playerKeysOffered() {
		t.Fatal("a relay on the game's own host would let the game page read the key")
	}
	withWebDomain(t, `example.org`)
	m.cfg.Enabled = false
	if m.playerKeysOffered() {
		t.Fatal("the module switched off offers nothing")
	}
}

func TestPlayerKeysConfig(t *testing.T) {
	c := buildConfig(nil)
	if c.PlayerKeys || c.RelayOrigin != `` || c.RelayTimeoutSeconds != 30 {
		t.Fatalf("defaults: player keys off, no origin, 30s: %+v", []any{c.PlayerKeys, c.RelayOrigin, c.RelayTimeoutSeconds})
	}
	c = buildConfig(func(k string) any {
		return map[string]any{`PlayerKeys`: true, `RelayOrigin`: ` https://keys.example.org/ `, `RelayTimeoutSeconds`: 1}[k]
	})
	if !c.PlayerKeys || c.RelayOrigin != `https://keys.example.org` || c.RelayTimeoutSeconds != 5 {
		t.Fatalf("read, trimmed and clamped: %+v", []any{c.PlayerKeys, c.RelayOrigin, c.RelayTimeoutSeconds})
	}
}

func TestRouteOrderRelayThenServerThenNone(t *testing.T) {
	withWebDomain(t, `example.org`)
	m := &AICompanionModule{cfg: Config{Enabled: true, PlayerKeys: true, RelayOrigin: `https://keys.example.org`}}
	m.relays = newRelayTable()
	if r := m.route(5); r.kind != routeNone {
		t.Fatalf("no relay and no server key is tier 1, got %v", r.kind)
	}
	m.relays.ready(5, `gpt-4.1-mini`, false)
	if r := m.route(5); r.kind != routeRelay || r.model != `gpt-4.1-mini` {
		t.Fatalf("a live relay is used first, got %+v", r)
	}
	if r := m.route(6); r.kind != routeNone {
		t.Fatalf("one owner's relay pays for nobody else, got %v", r.kind)
	}
	pointAt(m, apiframework.DefaultBaseURL, `sk-server`)
	if r := m.route(5); r.kind != routeRelay {
		t.Fatalf("the owner's relay is used before the server's key, got %v", r.kind)
	}
	m.relays.gone(5)
	if r := m.route(5); r.kind != routeServer {
		t.Fatalf("with the relay gone the server key covers, got %v", r.kind)
	}
	m.cfg.PlayerKeys = false
	m.relays.ready(5, `x`, false)
	if r := m.route(5); r.kind != routeServer {
		t.Fatal("player keys switched off ignores a relay")
	}
	m.cfg.PlayerKeys = true
	m.cfg.RelayOrigin = `http://keys.example.org`
	if r := m.route(5); r.kind != routeServer {
		t.Fatal("an invalid relay origin offers no relay")
	}
}

func TestRelayFailuresTripOnlyThatOwnersBreaker(t *testing.T) {
	m := relayModule(t)
	m.relays.ready(6, `m`, false)
	now := time.Now()
	for i := 0; i < 3; i++ {
		m.routeResult(route{kind: routeRelay, model: `m`}, 5, apiframework.Ticket{}, errors.New(`provider said no`), now)
	}
	if m.route(5).kind != routeNone {
		t.Fatal("owner 5's own breaker must be open")
	}
	if m.route(6).kind != routeRelay || m.breakerOpen(now) || m.fw().ConsumerFailures(apiframework.ConsumerCompanion) != 0 {
		t.Fatal("owner 6 and the global breaker are untouched")
	}
	if _, ok := m.relays.live(5, now.Add(61*time.Second)); !ok {
		t.Fatal("the owner's breaker closes after its cooldown")
	}

	// A refusal at the door never left the server, and a success resets.
	m.relays.ready(7, `m`, false)
	for i := 0; i < 3; i++ {
		m.routeResult(route{kind: routeRelay}, 7, apiframework.Ticket{}, errNoConsent, now)
	}
	m.routeResult(route{kind: routeRelay}, 7, apiframework.Ticket{}, errors.New(`x`), now)
	m.routeResult(route{kind: routeRelay}, 7, apiframework.Ticket{}, errors.New(`x`), now)
	m.routeResult(route{kind: routeRelay}, 7, apiframework.Ticket{}, nil, now)
	m.routeResult(route{kind: routeRelay}, 7, apiframework.Ticket{}, errors.New(`x`), now)
	if m.route(7).kind != routeRelay {
		t.Fatal("refusals do not count and a success clears the count")
	}

	// The server's key still trips the global breaker.
	for i := 0; i < 3; i++ {
		m.routeResult(route{kind: routeServer}, 6, apiframework.Ticket{}, errors.New(`x`), now)
	}
	if !m.breakerOpen(now) {
		t.Fatal("server-key failures open the global breaker")
	}
}

func TestModelReadyFollowsTheRoute(t *testing.T) {
	m := relayModule(t)
	m.fw().SetSpentForTest(apiframework.Server().DailyTokenBudget, serverHeld(m))
	setOwnerSpent(m, 5, m.cfg.DailyTokensPerCompanion)
	m.fw().SetBreakerForTest(0, time.Now().Add(time.Hour))
	if !m.modelReadyFor(5, 0) || !m.modelReady(5) {
		t.Fatal("the owner's own key is not held to the server's budgets or breaker")
	}
	if !m.modelReadyFor(5, 9) {
		t.Fatal("a passer-by talking to a relay companion is paid by the owner's key")
	}
	if m.modelReadyFor(6, 0) || m.modelReady() {
		t.Fatal("no relay and no server key is no call")
	}
	pointAt(m, apiframework.DefaultBaseURL, `k`)
	if m.modelReadyFor(6, 0) {
		t.Fatal("the server's key is still held to the breaker")
	}
	m.fw().SetBreakerForTest(0, time.Time{})
	if m.modelReadyFor(6, 0) {
		t.Fatal("and to the server's budget")
	}
	m.fw().SetSpentForTest(0, serverHeld(m))
	setOwnerSpent(m, 6, m.cfg.DailyTokensPerCompanion)
	if m.modelReadyFor(6, 0) || !m.modelReadyFor(6, 9) {
		t.Fatal("the owner's allowance binds the owner's calls, not a passer-by's")
	}
}

func TestApplyRouteUsesThePlayersModel(t *testing.T) {
	m := relayModule(t)
	pointAt(m, apiframework.DefaultBaseURL, `k`)
	call := modelCall{BaseURL: `https://api.openai.com/v1`, APIKey: `k`, Model: `server-model`, Effort: `low`, OwnerUserId: 5}
	m.applyRoute(&call)
	if call.Route.kind != routeRelay || call.Model != `player-model` || call.Effort != `` || call.APIKey != `` || call.BaseURL != `` {
		t.Fatalf("a relay call carries the player's model and nothing of the server's: %+v", call)
	}
	call = modelCall{BaseURL: `https://api.openai.com/v1`, APIKey: `k`, Model: `server-model`, Effort: `low`, OwnerUserId: 6}
	m.applyRoute(&call)
	if call.Route.kind != routeServer || call.Model != `server-model` || call.Effort != `low` || call.APIKey != `k` {
		t.Fatalf("a server call is unchanged: %+v", call)
	}
}

func TestRelayCallsReserveNothingOfTheServers(t *testing.T) {
	m := relayModule(t)
	relay := route{kind: routeRelay, model: `player-model`}
	m.fw().SetSpentForTest(apiframework.Server().DailyTokenBudget, serverHeld(m))
	setOwnerSpent(m, 5, m.cfg.DailyTokensPerCompanion)

	h, ok := m.reserveRoute(relay, 5, 0, 900)
	if !ok {
		t.Fatal("the owner's own key is not refused for the server's spent budgets")
	}
	if serverSpent(m) != apiframework.Server().DailyTokenBudget || serverHeld(m) != 0 || ownerSpent(m, 5) != m.cfg.DailyTokensPerCompanion {
		t.Fatalf("and holds nothing against them: today=%d outstanding=%d owner=%d", serverSpent(m), serverHeld(m), ownerSpent(m, 5))
	}
	// R22: the hold reserveRoute gave back, not one rebuilt here, is what
	// settles.
	m.settleRoute(h, 700)
	if serverSpent(m) != apiframework.Server().DailyTokenBudget || serverHeld(m) != 0 || ownerSpent(m, 5) != m.cfg.DailyTokensPerCompanion {
		t.Fatalf("nor settles anything against them: today=%d outstanding=%d owner=%d", serverSpent(m), serverHeld(m), ownerSpent(m, 5))
	}
	if tryRoute(m, route{kind: routeNone}, 5, 0, 1) {
		t.Fatal("tier 1 reserves nothing because it calls nothing")
	}
}

func TestStrangerRelayCallsStopAtTheStrangerCap(t *testing.T) {
	m := relayModule(t)
	relay := route{kind: routeRelay, model: `player-model`}

	if !tryRoute(m, relay, 5, 2, 900) {
		t.Fatal("a passer-by's question that fits their allowance is admitted")
	}
	if tryRoute(m, relay, 5, 2, 900) {
		t.Fatal("a second that would overshoot it is refused while the first is held")
	}
	if strangerSpent(m, 2) != 900 || serverSpent(m) != 0 || serverHeld(m) != 0 || ownerSpent(m, 5) != 0 {
		t.Fatalf("held against the passer-by alone: stranger=%d today=%d outstanding=%d owner=%d",
			strangerSpent(m, 2), serverSpent(m), serverHeld(m), ownerSpent(m, 5))
	}
	settleToday(m, relay, 5, 2, 900, 100)
	if strangerSpent(m, 2) != 100 {
		t.Fatalf("settled to what was used: %d", strangerSpent(m, 2))
	}
	if !tryRoute(m, relay, 5, 3, 900) {
		t.Fatal("another passer-by has their own allowance")
	}
	settleToday(m, relay, 5, 3, 900, 0)
	if strangerSpent(m, 3) != 0 {
		t.Fatalf("a failed call refunds all of it, once: %d", strangerSpent(m, 3))
	}
}

func TestStrangerRelayReservationsCannotSlipPastTheCapTogether(t *testing.T) {
	m := relayModule(t)
	relay := route{kind: routeRelay, model: `player-model`}
	admitted := 0
	done := make(chan bool)
	for i := 0; i < 20; i++ {
		go func() {
			util.LockMud()
			ok := tryRoute(m, relay, 5, 2, 400)
			util.UnlockMud()
			done <- ok
		}()
	}
	for i := 0; i < 20; i++ {
		if <-done {
			admitted++
		}
	}
	if admitted != 2 || strangerSpent(m, 2) != 800 {
		t.Fatalf("a 1000-token allowance admits two 400-token holds, got %d (held %d)", admitted, strangerSpent(m, 2))
	}
}

// A passer-by's talk with a relay companion is summed up on the owner's
// key and charged only to the passer-by's allowance: the server's spent
// budgets do not stop it, it never goes to the server's endpoint, and its
// failure reaches the owner's breaker, never the global one.
func TestRelaySummaryChargesOnlyThePasserBy(t *testing.T) {
	srv, hits := countingServer(t)
	m, c := strangerTalk(t, srv.URL)
	withWebDomain(t, `example.org`)
	m.cfg.PlayerKeys, m.cfg.RelayOrigin = true, `https://keys.example.org`
	m.relays = newRelayTable()
	m.relays.ready(1, `player-model`, false)
	// On the owner's own key passers-by prompt nothing until the owner
	// lets them.
	m.bonds.Users[1].StrangersOn = true
	m.fw().SetSpentForTest(apiframework.Server().DailyTokenBudget, serverHeld(m))
	setOwnerSpent(m, 1, m.cfg.DailyTokensPerCompanion)

	util.LockMud()
	m.closeConversation(c, `test`)
	held := strangerSpent(m, 2)
	util.UnlockMud()
	if held == 0 {
		t.Fatal("the summary was started on the owner's key and held against the passer-by")
	}
	if len(c.mind.Memories) != 0 {
		t.Fatalf("no plain note while the call is out: %+v", c.mind.Memories)
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		util.LockMud()
		failures := m.relays.owners[1].failures
		left := strangerSpent(m, 2)
		util.UnlockMud()
		if failures == 1 && left == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("never settled: relay failures=%d stranger=%d", failures, left)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if hits.Load() != 0 {
		t.Fatalf("a relay call never goes to the server's endpoint: %d requests", hits.Load())
	}
	if serverSpent(m) != apiframework.Server().DailyTokenBudget || ownerSpent(m, 1) != m.cfg.DailyTokensPerCompanion || serverHeld(m) != 0 {
		t.Fatalf("the server's ledgers are untouched: today=%d owner=%d outstanding=%d", serverSpent(m), ownerSpent(m, 1), serverHeld(m))
	}
	if m.fw().ConsumerFailures(apiframework.ConsumerCompanion) != 0 {
		t.Fatal("a relay failure never counts toward the global breaker")
	}
}

// The owner's own reflection and core memory run on the owner's key with
// the server's budgets spent, hold nothing of the server's, and fail to the
// owner's breaker. A core memory whose call fails keeps the bare fact, as
// it would with no model at all.
func TestRelayReflectionAndCoreMemorySpendNothingOfTheServers(t *testing.T) {
	srv, hits := countingServer(t)
	m, c := senderModule(srv.URL, true)
	withWebDomain(t, `example.org`)
	m.cfg.PlayerKeys, m.cfg.RelayOrigin = true, `https://keys.example.org`
	m.cfg.BreakerErrors = 10
	m.relays = newRelayTable()
	m.relays.ready(1, `player-model`, false)
	m.minds = map[string]*Mind{mindIdentifier(c.mind.OwnerUserId, c.mind.MobId): c.mind}
	m.ctrls = map[int]*controller{c.ownerUserId: c}
	m.fw().SetSpentForTest(apiframework.Server().DailyTokenBudget, serverHeld(m))
	setOwnerSpent(m, 1, m.cfg.DailyTokensPerCompanion)

	now := time.Now().Unix()
	util.LockMud()
	for i := 0; i < 6; i++ {
		c.mind.addLine(Line{Speaker: `Corvin`, Kind: `said`, Text: `hello`, Unix: now}, 50)
	}
	m.startReflection(c.mind, c.profile, `Corvin`, 0)
	m.recordCore(c, `Corvin`, romanceCourting, true)
	cores := len(c.mind.CoreMemories)
	util.UnlockMud()
	if cores != 0 {
		t.Fatal("the core memory call was started, not skipped for the server's spent budget")
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		util.LockMud()
		failures := m.relays.owners[1].failures
		util.UnlockMud()
		if failures == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("both calls should have failed to the owner's breaker, got %d", failures)
		}
		time.Sleep(10 * time.Millisecond)
	}
	util.LockMud()
	defer util.UnlockMud()
	if hits.Load() != 0 || m.fw().ConsumerFailures(apiframework.ConsumerCompanion) != 0 {
		t.Fatalf("nothing reached the server's endpoint (%d) or the global breaker (%d)", hits.Load(), m.fw().ConsumerFailures(apiframework.ConsumerCompanion))
	}
	if serverSpent(m) != apiframework.Server().DailyTokenBudget || ownerSpent(m, 1) != m.cfg.DailyTokensPerCompanion || serverHeld(m) != 0 {
		t.Fatalf("the server's ledgers are untouched: today=%d owner=%d outstanding=%d", serverSpent(m), ownerSpent(m, 1), serverHeld(m))
	}
	if len(c.mind.CoreMemories) != 1 || c.mind.CoreMemories[0].Text != `Something changed between Corvin and me here.` {
		t.Fatalf("a failed call keeps the bare fact: %+v", c.mind.CoreMemories)
	}
}

// A muted owner's companion says nothing of her own: say and emote alike
// are free text, and her words are her owner's to answer for. Not muted,
// the lines pass unchanged.
func TestMutedOwnerSilencesHer(t *testing.T) {
	lines := []SpeechLine{{Kind: `say`, Text: `Hello there.`}, {Kind: `emote`, Text: `waves.`}}
	owner := users.NewTestUser(1, `corvin`, `Corvin`, 0)
	if got := spokenLines(owner, lines); len(got) != 2 || got[0] != lines[0] || got[1] != lines[1] {
		t.Fatalf("an owner who is not muted leaves her lines alone: %+v", got)
	}
	owner.Muted = true
	if got := spokenLines(owner, lines); len(got) != 0 {
		t.Fatalf("a muted owner silences say and emote alike: %+v", got)
	}
	if got := spokenLines(nil, lines); len(got) != 0 {
		t.Fatal("with no owner to answer for her, she says nothing")
	}
}

// speak itself honours the mute, on every tier: nothing is said, so
// nothing is remembered as said.
func TestSpeakHonoursTheMute(t *testing.T) {
	owner, _, _, her := harmWorld(t, `off`)
	m, c := senderModule(`https://api.example.invalid`, true)
	c.instanceId = her.InstanceId
	owner.Muted = true
	m.speak(c, her, []SpeechLine{{Kind: `say`, Text: `Hello there.`}}, route{kind: routeServer})
	if len(c.mind.RecentLines) != 0 {
		t.Fatalf("a muted owner's companion said something: %+v", c.mind.RecentLines)
	}
	owner.Muted = false
	m.speak(c, her, []SpeechLine{{Kind: `say`, Text: `Hello there.`}}, route{kind: routeServer})
	if len(c.mind.RecentLines) != 1 {
		t.Fatalf("control: not muted, she speaks: %+v", c.mind.RecentLines)
	}
}

// On the owner's own key nothing moderates her words, so each line is
// logged against the owner. On the server's key the lines passed
// moderation and are not logged.
func TestRelaySpeechIsLoggedAgainstTheOwner(t *testing.T) {
	attrs := speechLogLine(route{kind: routeRelay}, 7, `Mara`, `say`, `Hello there.`)
	if len(attrs) == 0 {
		t.Fatal("relay speech is logged")
	}
	got := map[string]any{}
	for i := 0; i+1 < len(attrs); i += 2 {
		got[attrs[i].(string)] = attrs[i+1]
	}
	if got[`owner`] != 7 || got[`companion`] != `Mara` || got[`kind`] != `say` || got[`text`] != `Hello there.` || got[`action`] != `speech` {
		t.Fatalf("the log names the owner, her, the kind and the text: %+v", got)
	}
	if attrs := speechLogLine(route{kind: routeServer}, 7, `Mara`, `say`, `x`); attrs != nil {
		t.Fatalf("server-key speech is not logged: %+v", attrs)
	}
	if attrs := speechLogLine(route{kind: routeNone}, 7, `Mara`, `emote`, `x`); attrs != nil {
		t.Fatalf("set lines are not logged: %+v", attrs)
	}
}

// deferredModule is a consenting owner 1 with a relay module around them.
func deferredModule(t *testing.T) (*AICompanionModule, *controller, *fakeRelay) {
	t.Helper()
	m, c := senderModule(`https://api.example.invalid`, true)
	withWebDomain(t, `example.org`)
	m.cfg.PlayerKeys, m.cfg.RelayOrigin = true, `https://keys.example.org`
	m.cfg.RelayTimeoutSeconds = 5
	m.cfg.MinSessionLinesForReflection = 1
	m.relays = newRelayTable()
	m.relayCalls = newPendingRelays()
	f := newFakeRelay()
	m.relaySend = f.send
	m.minds = map[string]*Mind{mindIdentifier(c.mind.OwnerUserId, c.mind.MobId): c.mind}
	now := time.Now().Unix()
	for i := 0; i < 4; i++ {
		c.mind.addLine(Line{Speaker: `Corvin`, Kind: `said`, Text: `hello`, Unix: now}, 50)
	}
	return m, c, f
}

// A relay owner's reflection cannot reach a browser that is closing, so it
// waits for their next login with the relay up. One waits per owner, the
// newest; it starts once, only for an owner who is online, and only once
// their relay is live.
func TestReflectionWaitsForTheRelay(t *testing.T) {
	m, c, f := deferredModule(t)
	m.relays.ready(1, `player-model`, false)
	c.relaySeen = true
	util.LockMud()
	m.detachReflection(c, `Corvin`)
	m.relayGone(1)
	util.UnlockMud()
	select {
	case r := <-f.sent:
		t.Fatalf("the reflection went to a relay that was closing: %+v", r)
	default:
	}
	first := m.deferredReflect[1]
	if first == nil {
		t.Fatal("a relay owner's reflection is kept for later")
	}

	// A newer session's reflection replaces the older one.
	util.LockMud()
	m.detachReflection(c, `Corvin`)
	util.UnlockMud()
	if m.deferredReflect[1] == nil || m.deferredReflect[1] == first {
		t.Fatal("the newer reflection replaces the older")
	}

	if d := m.dueReflection(1, true); d != nil {
		t.Fatal("not while the relay is down")
	}
	m.relays.ready(1, `player-model`, false)
	if d := m.dueReflection(1, false); d != nil {
		t.Fatal("not while the owner is logged out, even with a relay up")
	}
	d := m.dueReflection(1, true)
	if d == nil {
		t.Fatal("online with the relay up, it is due")
	}
	if again := m.dueReflection(1, true); again != nil {
		t.Fatal("exactly once")
	}
	util.LockMud()
	m.launchReflection(d)
	util.UnlockMud()
	r := f.next(t)
	if !strings.Contains(string(r.Body), `player-model`) {
		t.Fatalf("it runs on the owner's model through the relay: %s", r.Body)
	}
}

// The round tick is what starts a waiting reflection, for an owner who is
// online and whose relay is live, and it still passes the consent door.
func TestTheRoundStartsAWaitingReflection(t *testing.T) {
	m, c, f := deferredModule(t)
	m.relays.ready(1, `player-model`, false)
	c.relaySeen = true
	util.LockMud()
	m.detachReflection(c, `Corvin`)
	m.relays.gone(1)
	// Consent withdrawn while they were away: nothing is sent.
	m.bonds.Users[1].Consented, m.bonds.Users[1].Refused = false, true
	m.saveBonds()
	m.relays.ready(1, `player-model`, false)
	m.startDueReflection(1)
	util.UnlockMud()
	select {
	case r := <-f.sent:
		t.Fatalf("a reflection was sent for an owner who withdrew consent: %+v", r)
	case <-time.After(100 * time.Millisecond):
	}

	m, c, f = deferredModule(t)
	m.relays.ready(1, `player-model`, false)
	c.relaySeen = true
	util.LockMud()
	m.detachReflection(c, `Corvin`)
	m.relays.gone(1)
	m.relays.ready(1, `player-model`, false)
	m.startDueReflection(1)
	util.UnlockMud()
	f.next(t)
}

// An owner who never had a relay this session reflects at logout, as
// before, on the server's key.
func TestReflectionWithoutARelayRunsAtOnce(t *testing.T) {
	srv, hits := countingServer(t)
	m, c, f := deferredModule(t)
	pointAt(m, srv.URL, m.apiKey())
	util.LockMud()
	m.detachReflection(c, `Corvin`)
	util.UnlockMud()
	if m.deferredReflect[1] != nil {
		t.Fatal("no relay this session: nothing waits")
	}
	deadline := time.Now().Add(5 * time.Second)
	for hits.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the reflection ran on the server's key at logout")
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case r := <-f.sent:
		t.Fatalf("nothing goes to a relay: %+v", r)
	default:
	}
}

// With strangers off, a passer-by prompts no call on anyone's key: the
// predicate, and the conversation summary a talk with passers-by alone
// would start.
func TestStrangersOffStopsStrangerPrompts(t *testing.T) {
	srv, hits := countingServer(t)
	m, c := senderModule(srv.URL, true)
	if !m.strangerMayPrompt(1, 2) || !m.strangerMayPrompt(1, 0) {
		t.Fatal("strangers on by default")
	}
	m.bonds.Users[1].StrangersOff = true
	if m.strangerMayPrompt(1, 2) {
		t.Fatal("strangers off: a passer-by prompts no call")
	}
	if !m.strangerMayPrompt(1, 0) {
		t.Fatal("the owner's own prompts are untouched")
	}

	now := time.Now().Unix()
	convo := &conversation{RoomId: 7, Partner: `Bram`, StartUnix: now, LastUnix: now, Exchanges: 5,
		Lines: []Line{{Speaker: `Bram`, Kind: `said`, Text: `a`}}}
	util.LockMud()
	started := m.summariseConversation(c, convo, 2)
	util.UnlockMud()
	if started || hits.Load() != 0 {
		t.Fatalf("a passers-by talk is not summed up on the owner's key: started=%v hits=%d", started, hits.Load())
	}
	m.bonds.Users[1].StrangersOff = false
	util.LockMud()
	started = m.summariseConversation(c, convo, 2)
	util.UnlockMud()
	if !started {
		t.Fatal("control: strangers on, the summary starts")
	}
}

// Old bond records have no strangers field and load as strangers on; the
// field survives a save and a load, through the same YAML the plugin
// store writes.
func TestStrangersOffPersistsOnTheBond(t *testing.T) {
	var old bondState
	if err := yaml.Unmarshal([]byte("users:\n  1:\n    profile: mara\n    met: true\n    consented: true\n"), &old); err != nil {
		t.Fatal(err)
	}
	if old.Users[1] == nil || old.Users[1].StrangersOff {
		t.Fatalf("an old record loads as strangers on: %+v", old.Users[1])
	}
	old.Users[1].StrangersOff = true
	b, err := yaml.Marshal(&old)
	if err != nil {
		t.Fatal(err)
	}
	var back bondState
	if err := yaml.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if !back.Users[1].StrangersOff {
		t.Fatalf("strangers off survives a save: %s", b)
	}
}

// The owner hears once per relay session that her words fell back, in
// plain words and never the error; a second failure says nothing, and a
// relay that comes back may say it again.
func TestRelayFallbackNoticeOncePerSession(t *testing.T) {
	m := relayModule(t)
	var told []string
	m.tell = func(userId int, text string) {
		if userId == 5 {
			told = append(told, text)
		}
	}
	now := time.Now()
	m.routeResult(route{kind: routeRelay}, 5, apiframework.Ticket{}, errors.New(`model API status 500: boom`), now)
	m.routeResult(route{kind: routeRelay}, 5, apiframework.Ticket{}, errors.New(`model API status 500: boom`), now)
	if len(told) != 1 {
		t.Fatalf("told once, got %d: %q", len(told), told)
	}
	if plain := strings.Join(strings.Fields(told[0]), ` `); strings.Contains(plain, `500`) || strings.Contains(plain, `boom`) || !strings.Contains(plain, `your key's provider did not answer`) {
		t.Fatalf("plain words, never the error: %q", told[0])
	}
	for _, line := range strings.Split(told[0], "\n") {
		if len([]rune(line)) > 80 {
			t.Fatalf("the notice wraps at 80 columns: %q", told[0])
		}
	}
	m.relays.ready(5, `player-model`, false)
	m.routeResult(route{kind: routeServer}, 5, apiframework.Ticket{}, errors.New(`x`), now)
	m.routeResult(route{kind: routeRelay}, 5, apiframework.Ticket{}, errRelayGone, now)
	m.routeResult(route{kind: routeRelay}, 5, apiframework.Ticket{}, context.Canceled, now)
	if len(told) != 1 {
		t.Fatalf("only a relay call the provider failed is worth telling: %q", told)
	}
	m.routeResult(route{kind: routeRelay}, 5, apiframework.Ticket{}, errRelayTimeout, now)
	if len(told) != 2 {
		t.Fatalf("a relay that came back may be told again: %q", told)
	}
}

// A browser that never answers is not asked twice: the timeout is final.
func TestRelayTimeoutIsNotRetried(t *testing.T) {
	m, f := relayCallModule(t, true)
	m.cfg.RelayTimeoutSeconds = 1
	c := relayCall(m)
	if !c.Retry {
		t.Fatal("fixture: the call allows a retry")
	}
	res := m.callModel(c)
	if !errors.Is(res.Err, errRelayTimeout) || transient(res) {
		t.Fatalf("a relay timeout is final: %v", res.Err)
	}
	f.next(t)
	select {
	case r := <-f.sent:
		t.Fatalf("a silent browser was asked again: %+v", r)
	default:
	}
}

// dispatch answers a passer-by with set lines, and starts no call, while
// her owner has strangers off; the owner's own words still start one.
func TestStrangersOffDispatchesSetLines(t *testing.T) {
	_, _, _, her := harmWorld(t, `off`)
	srv, _ := countingServer(t)
	m, c := senderModule(srv.URL, true)
	c.instanceId = her.InstanceId
	m.ctrls = map[int]*controller{1: c}
	m.minds = map[string]*Mind{mindIdentifier(c.mind.OwnerUserId, c.mind.MobId): c.mind}
	m.bonds.Users[1].StrangersOff = true

	util.LockMud()
	c.push(stimulus{Kind: `heard`, Speaker: `Bram`, Text: `Mara, hello`, AskerUserId: 2})
	m.dispatch(c)
	inFlight, said := c.inFlight, len(c.mind.RecentLines)
	util.UnlockMud()
	if inFlight || said == 0 {
		t.Fatalf("a passer-by gets set lines and no call: inFlight=%v lines=%d", inFlight, said)
	}

	util.LockMud()
	c.push(stimulus{Kind: `heard`, Speaker: `Corvin`, Text: `Mara, hello`, FromOwner: true, AskerUserId: 1})
	m.dispatch(c)
	inFlight = c.inFlight
	c.cancelInFlight()
	util.UnlockMud()
	// The cancelled call still settles under the lock; the next test
	// rebuilds the world, so it must be finished first.
	m.decisions.Wait()
	if !inFlight {
		t.Fatal("control: the owner's own words start a call")
	}
}

// companion-ai strangers off and on set the bond record, and every line
// the command sends fits 80 columns.
func TestCompanionAIStrangersCommand(t *testing.T) {
	owner, _, _, _ := harmWorld(t, `off`)
	m, _ := senderModule(`https://api.example.invalid`, true)
	m.cfg.Enabled = true
	withWebDomain(t, `example.org`)
	m.cfg.PlayerKeys, m.cfg.RelayOrigin = true, `https://keys.example.org`
	m.relays = newRelayTable()
	m.relays.ready(1, `player-model`, false)
	events.DrainQueuedMessagesForTest(1)
	for _, arg := range []string{`strangers off`, `strangers`, ``, `strangers  on`, `nonsense`, `on`, `off`, ``} {
		if _, err := m.cmdAI(arg, owner, nil, 0); err != nil {
			t.Fatal(err)
		}
		sent := events.DrainQueuedMessagesForTest(1)
		if len(sent) != 1 {
			t.Fatalf("%q: one reply, got %q", arg, sent)
		}
		for _, line := range strings.Split(strings.TrimRight(sent[0], "\n"), "\n") {
			if n := len([]rune(ansiTag.ReplaceAllString(line, ``))); n > 80 {
				t.Fatalf("%q: a line of %d columns: %q", arg, n, line)
			}
		}
		if arg == `` && m.bonds.Users[1].Consented && !strings.Contains(strings.Join(strings.Fields(sent[0]), ` `), `your own key`) {
			t.Fatalf("the status says which tier answers: %q", sent[0])
		}
		if arg == `strangers off` && !m.bonds.Users[1].StrangersOff {
			t.Fatal("strangers off is set on the bond")
		}
		if arg == `strangers  on` && m.bonds.Users[1].StrangersOff {
			t.Fatal("strangers on clears it")
		}
	}
}

// ansiTag matches the markup a rendered line carries, which takes no column.
var ansiTag = regexp.MustCompile(`<[^>]*>`)

// The round tick itself starts a waiting reflection once its owner is
// online with the relay up, and marks the session as one on their own key.
func TestSyncStartsTheWaitingReflection(t *testing.T) {
	owner, _, _, _ := harmWorld(t, `off`)
	m, c, f := deferredModule(t)
	m.byMob = map[int]*Profile{c.profile.MobId: c.profile}
	m.roster = rosterState{Profiles: map[string]*rosterEntry{c.profile.Id: {Owner: 1}}}
	owner.Character.Companions = []characters.CompanionInfo{{MobId: c.profile.MobId, SourceType: characters.CompanionBonded}}
	m.relays.ready(1, `player-model`, false)
	c.relaySeen = true
	util.LockMud()
	m.detachReflection(c, `Corvin`)
	m.relays.gone(1)
	fresh := &controller{ownerUserId: 1, profile: c.profile, mind: c.mind, lastAttackBy: map[int]int64{}}
	m.ctrls = map[int]*controller{1: fresh}
	m.sync(1)
	util.UnlockMud()
	if m.deferredReflect[1] == nil || fresh.relaySeen {
		t.Fatal("with the relay down the round starts nothing")
	}
	m.relays.ready(1, `player-model`, false)
	util.LockMud()
	m.sync(2)
	util.UnlockMud()
	if m.deferredReflect[1] != nil || !fresh.relaySeen {
		t.Fatal("online with the relay up, the round takes the waiting reflection")
	}
	f.next(t)
}

// Her words to one person are spoken aloud too, and a muted owner
// silences them as well.
func TestMutedOwnerSilencesSayto(t *testing.T) {
	owner, _, _, her := harmWorld(t, `off`)
	m, c, _ := strangerModule()
	stims := []stimulus{{Kind: `heard`, FromOwner: true}}
	owner.Muted = true
	out := m.performAction(c, her, owner, harmScene(0, 2), ActionProposal{Verb: `sayto`, Ref: `t2`, Query: `hello`}, stims, 0, 0)
	if out.Issued || out.Refused == `` {
		t.Fatalf("a muted owner's companion speaks to nobody: %+v", out)
	}
	owner.Muted = false
	out = m.performAction(c, her, owner, harmScene(0, 2), ActionProposal{Verb: `sayto`, Ref: `t2`, Query: `hello`}, stims, 0, 0)
	if !out.Issued {
		t.Fatalf("control: not muted, she speaks: %+v", out)
	}
}

// A relay that went away (the page closed, the owner logged out) and a
// call the module gave up on are not the provider failing, and do not
// count toward the owner's breaker; a provider error does.
func TestRelayGoneAndCancelledSpareTheBreaker(t *testing.T) {
	m := relayModule(t)
	m.tell = func(int, string) {}
	now := time.Now()
	m.routeResult(route{kind: routeRelay}, 5, apiframework.Ticket{}, errRelayGone, now)
	m.routeResult(route{kind: routeRelay}, 5, apiframework.Ticket{}, fmt.Errorf(`wrapped: %w`, context.Canceled), now)
	if n := m.relays.owners[5].failures; n != 0 {
		t.Fatalf("gone and cancelled count nothing, got %d", n)
	}
	m.routeResult(route{kind: routeRelay}, 5, apiframework.Ticket{}, errRelayTimeout, now)
	if n := m.relays.owners[5].failures; n != 1 {
		t.Fatalf("control: a silent provider counts, got %d", n)
	}
}

// tryRoute is reserveRoute for a test that settles the same day.
func tryRoute(m *AICompanionModule, r route, ownerId int, askerId int, tokens int) bool {
	_, ok := m.reserveRoute(r, ownerId, askerId, tokens)
	return ok
}

// settleToday settles a reservation tryRoute made today: the hold the
// ledger would have given, rebuilt from the same route and payer.
func settleToday(m *AICompanionModule, r route, ownerId int, askerId int, reserved int, used int) {
	h := hold{r: r}
	if r.kind == routeServer || (r.kind == routeRelay && askerId > 0) {
		h.fw = apiframework.Hold{Consumer: apiframework.ConsumerCompanion, Tokens: reserved, Day: m.fw().Day(),
			SpendServer: r.kind == routeServer, Charges: m.allowanceCharges(ownerId, askerId)}
	}
	m.settleRoute(h, used)
}

// Every setting buildConfig reads is documented in settings.md, so an
// operator can find it: a mistyped key is a silent default, and an
// undocumented one is a setting nobody knows to set.
func TestEverySettingIsDocumented(t *testing.T) {
	var keys []string
	buildConfig(func(k string) any { keys = append(keys, k); return nil })
	_, here, _, _ := runtime.Caller(0)
	doc, err := os.ReadFile(filepath.Join(filepath.Dir(here), `..`, `..`, `docs`, `aicompanion`, `settings.md`))
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) < 50 {
		t.Fatalf("fixture: buildConfig reads its settings through the getter, got %d", len(keys))
	}
	for _, k := range keys {
		if !regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(k) + `:`).Match(doc) {
			t.Errorf("%s is not in docs/aicompanion/settings.md", k)
		}
	}
}
