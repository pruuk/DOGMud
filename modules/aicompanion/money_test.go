package aicompanion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Task 14B: what a companion's calls cost, and who is charged for them.

// Player text reaches her mind whole, and every stored line goes out again
// in each prompt: each writer keeps at most maxStoredRunes of it, cut on a
// rune boundary.
func TestStoredTextIsCapped(t *testing.T) {
	long := strings.Repeat(`ж`, 2000) // two bytes a rune: a byte cut would split one
	m, c := senderModule(`https://api.example.invalid`, true)
	mind := c.mind
	mind.addLine(Line{Kind: `said`, Speaker: `Bram`, Text: long}, 50)
	mind.addMemory(Memory{Kind: `event`, Text: long, Importance: 5}, 50)
	mind.addFact(Fact{Text: long}, 50)
	mind.addPromise(`Bram`, long)
	mind.addHearsay(`Bram`, long, 1)
	mind.addOwnPhrase(long, 10)
	mind.addCore(CoreMemory{Text: long})
	m.noteConversation(c, 7, `Bram`, 2, Line{Speaker: `Bram`, Kind: `said`, Text: long})

	got := map[string]string{
		`line`:     mind.RecentLines[len(mind.RecentLines)-1].Text,
		`memory`:   mind.Memories[len(mind.Memories)-1].Text,
		`fact`:     mind.Facts[len(mind.Facts)-1].Text,
		`promise`:  mind.Promises[len(mind.Promises)-1].Text,
		`hearsay`:  mind.Hearsay[len(mind.Hearsay)-1].Text,
		`phrase`:   mind.OwnPhrases[len(mind.OwnPhrases)-1],
		`core`:     mind.CoreMemories[len(mind.CoreMemories)-1].Text,
		`converse`: c.convo.Lines[len(c.convo.Lines)-1].Text,
	}
	for what, text := range got {
		if n := len([]rune(text)); n != maxStoredRunes {
			t.Errorf("%s keeps %d runes, want %d", what, n, maxStoredRunes)
		}
		if strings.ContainsRune(text, '�') {
			t.Errorf("%s was cut inside a character", what)
		}
	}
	if capRunes(`  short  `) != `short` {
		t.Fatal("short text is only trimmed")
	}
}

// A reply asking the game dozens of questions at once is answered for the
// first few only.
func TestToolCallsPerReplyAreCapped(t *testing.T) {
	var calls []string
	for i := 0; i < 12; i++ {
		calls = append(calls, fmt.Sprintf(`{"id":"c%d","type":"function","function":{"name":"recall","arguments":"{}"}}`, i))
	}
	raw := `{"choices":[{"finish_reason":"tool_calls","message":{"tool_calls":[` + strings.Join(calls, `,`) + `]}}],"usage":{"total_tokens":10}}`
	res := decodeChatResponse(modelResult{}, 200, []byte(raw))
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(res.ToolCalls) != maxToolCallsPerReply || maxToolCallsPerReply != 4 {
		t.Fatalf("at most four questions a reply, got %d", len(res.ToolCalls))
	}
	if res.ToolCalls[0].Id != `c0` || res.ToolCalls[3].Id != `c3` {
		t.Fatalf("the first ones are kept: %+v", res.ToolCalls)
	}
}

// An error reply that came through a player's browser keeps none of its
// body: not in the error, so not in the log or the trace either.
func TestRelayErrorReplyCarriesNoBody(t *testing.T) {
	m, f := relayCallModule(t, true)
	c := relayCall(m)
	c.Retry = false
	done := make(chan modelResult, 1)
	go func() { done <- m.callModel(c) }()
	req := f.next(t)
	m.relayCalls.deliver(5, relayResponse{Id: req.Id, Status: 401, Body: `{"error":{"message":"Incorrect key for account acct_4242 of jane@example.com"}}`})
	res := <-done
	if res.Err == nil || res.Status != 401 {
		t.Fatalf("an error status fails the call: %+v", res)
	}
	if msg := res.Err.Error(); strings.Contains(msg, `jane`) || strings.Contains(msg, `acct_4242`) || strings.Contains(msg, `Incorrect`) {
		t.Fatalf("the relayed body leaked into the error: %q", msg)
	}
	if !strings.Contains(res.Err.Error(), `401`) {
		t.Fatalf("the status is kept: %q", res.Err.Error())
	}
	// The server's own provider is the operator's business: its error text
	// is still kept for them.
	direct := decodeChatResponse(modelResult{}, 500, []byte(`upstream busy`))
	if direct.Err == nil || !strings.Contains(direct.Err.Error(), `upstream busy`) {
		t.Fatalf("control: the server key's error keeps its text: %v", direct.Err)
	}
}

// A talk among passers-by is summed up at the cost of the one who said the
// most in it, not whoever spoke last.
func TestConversationPayerIsWhoSaidTheMost(t *testing.T) {
	m, c := senderModule(`https://api.example.invalid`, true)
	for i := 0; i < 5; i++ {
		m.noteConversation(c, 7, `Bram`, 2, Line{Speaker: `Bram`, Kind: `said`, Text: fmt.Sprintf(`line %d`, i)})
	}
	m.noteConversation(c, 7, `Ilse`, 3, Line{Speaker: `Ilse`, Kind: `said`, Text: `bye`})
	if p := c.convo.payer(); p != 2 {
		t.Fatalf("the one who said the most pays, got user %d", p)
	}
	m.noteConversation(c, 7, `Ilse`, 3, Line{Speaker: `Ilse`, Kind: `said`, Text: `a`})
	for i := 0; i < 4; i++ {
		m.noteConversation(c, 7, `Ilse`, 3, Line{Speaker: `Ilse`, Kind: `said`, Text: `b`})
	}
	if p := c.convo.payer(); p != 3 {
		t.Fatalf("and it moves when somebody else says more, got user %d", p)
	}
	m.noteConversation(c, 7, `Corvin`, 1, Line{Speaker: `Corvin`, Kind: `said`, Text: `hello`})
	if p := c.convo.payer(); p != 0 {
		t.Fatalf("a talk the owner joined is the owner's, got user %d", p)
	}
}

// companion-unstick abandons a call that may have been paid for and frees
// her to start another at once: it waits a minute between uses.
func TestUnstickHasACooldown(t *testing.T) {
	m, c := senderModule(`https://api.example.invalid`, true)
	m.cfg.Enabled = true
	m.ctrls = map[int]*controller{1: c}
	owner := users.NewTestUser(1, `corvin`, `Corvin`, 0)
	if _, err := m.cmdUnstick(``, owner, nil, 0); err != nil {
		t.Fatal(err)
	}
	first := c.seq
	if first == 0 {
		t.Fatal("fixture: the first reset goes through")
	}
	c.pending = []stimulus{{Kind: `heard`}}
	if _, err := m.cmdUnstick(``, owner, nil, 0); err != nil {
		t.Fatal(err)
	}
	if c.seq != first || len(c.pending) != 1 {
		t.Fatalf("a second reset at once is refused: seq %d -> %d, pending %d", first, c.seq, len(c.pending))
	}
	rs := int(configs.GetTimingConfig().RoundSeconds)
	if n, want := owner.Character.GetCooldown(unstickCooldownTag), (60+rs-1)/rs; n != want {
		t.Fatalf("the wait is a real minute, %d rounds, got %d", want, n)
	}
}

// A passer-by's pause lasts StrangerAskSeconds of real time, counted in the
// rounds a character cooldown ticks in: thirty seconds, not thirty rounds.
func TestStrangerAskCooldownCountsRealSeconds(t *testing.T) {
	m, c, bram := strangerModule()
	m.hearSaid(c, bram, `Bram`, `Mara, which way to the river?`, 7, true, time.Now().Unix())
	rs := int(configs.GetTimingConfig().RoundSeconds)
	want := (m.cfg.StrangerAskSeconds + rs - 1) / rs
	if got := bram.Character.GetCooldown(fmt.Sprintf(`aicompanion-ask-%d`, c.instanceId)); got != want {
		t.Fatalf("%d seconds is %d rounds of %d seconds, got %d", m.cfg.StrangerAskSeconds, want, rs, got)
	}
}

// usageServer answers every chat completion with the given body, after
// waiting on hold when it is not nil.
func usageServer(t *testing.T, body string, hold chan struct{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hold != nil {
			select {
			case <-hold:
			case <-r.Context().Done():
				return
			}
		}
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func usageCall(m *AICompanionModule, baseURL string) modelCall {
	c := modelCall{BaseURL: baseURL, APIKey: `k`, Model: `m`, Timeout: 2 * time.Second, MaxTokens: 100,
		Messages: []chatMessage{{Role: `user`, Content: strings.Repeat(`word `, 400)}}, SchemaName: `s`,
		Schema: decisionSchema(), OwnerUserId: 1}
	m.applyRoute(&c)
	return c
}

// A request that left and came back with no usage may still have been
// billed, completion and all: it is counted at its prompt estimate plus
// its MaxTokens, not as nothing and not as the prompt alone. One that
// never left counts nothing.
func TestSentCallWithNoUsageCountsItsPrompt(t *testing.T) {
	m, _ := senderModule(`https://api.example.invalid`, true)
	probe := usageCall(m, `x`)
	prompt := estimateTokens(probe.Messages) + requestOverhead(probe) + probe.MaxTokens

	// An answer with no usage at all.
	srv := usageServer(t, `{"choices":[{"finish_reason":"stop","message":{"content":"{}"}}]}`, nil)
	res := m.callModelOnce(usageCall(m, srv.URL))
	if res.Err != nil || res.Tokens != prompt || !res.Estimated {
		t.Fatalf("an answer with no usage counts its prompt %d: %+v", prompt, res)
	}
	// Real usage is taken as given.
	srv = usageServer(t, `{"choices":[{"finish_reason":"stop","message":{"content":"{}"}}],"usage":{"total_tokens":7}}`, nil)
	if res = m.callModelOnce(usageCall(m, srv.URL)); res.Tokens != 7 || res.Estimated {
		t.Fatalf("reported usage stands: %+v", res)
	}
	// A timeout: sent, never answered.
	hold := make(chan struct{})
	defer close(hold)
	srv = usageServer(t, `{}`, hold)
	c := usageCall(m, srv.URL)
	c.Timeout = 200 * time.Millisecond
	if res = m.callModelOnce(c); res.Err == nil || res.Tokens != prompt {
		t.Fatalf("a timed-out request counts its prompt: %+v", res)
	}
	// Given up on after it was sent (companion-unstick, logout).
	c = usageCall(m, srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	c.Ctx = ctx
	go func() { time.Sleep(200 * time.Millisecond); cancel() }()
	if res = m.callModelOnce(c); !res.Canceled || res.Tokens != prompt {
		t.Fatalf("a call cancelled after it was sent counts its prompt: %+v", res)
	}
	// Given up on before it left.
	if res = m.callModelOnce(c); !res.Canceled || res.Tokens != 0 || res.Sent {
		t.Fatalf("a call cancelled before it left counts nothing: %+v", res)
	}
	// Refused at the door.
	m.consent.agreed = map[int]bool{}
	if res = m.callModelOnce(usageCall(m, srv.URL)); !errors.Is(res.Err, errNoConsent) || res.Tokens != 0 {
		t.Fatalf("a request the door refused counts nothing: %+v", res)
	}
	m.consent.agreed = map[int]bool{1: true}
	// Nobody to connect to.
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()
	if res = m.callModelOnce(usageCall(m, deadURL)); res.Err == nil || res.Tokens != 0 {
		t.Fatalf("a connection never made counts nothing: %+v", res)
	}
}

// A cancelled call is neither a failure nor a success to the breaker: it
// reaches it not at all, so it cannot reset the count of real failures.
func TestCancelledCallNeverReachesTheBreaker(t *testing.T) {
	m, _ := senderModule(`https://api.example.invalid`, true)
	freshServer(t, 2000000, 3, 60)
	m.fw().SetConsumerBreakerForTest(apiframework.ConsumerCompanion, 2, time.Time{})
	m.ctrls = map[int]*controller{}
	m.applyResult(1, 1, 0, 0, 100, nil, nil, tierMain, `m`, route{kind: routeServer},
		modelResult{Err: context.Canceled, Canceled: true})
	if m.fw().ConsumerFailures(apiframework.ConsumerCompanion) != 2 {
		t.Fatalf("a cancel leaves the failure count alone, got %d", m.fw().ConsumerFailures(apiframework.ConsumerCompanion))
	}
	m.applyResult(1, 1, 0, 0, 100, nil, nil, tierMain, `m`, route{kind: routeServer},
		modelResult{Err: errors.New(`status 500`)})
	if !m.breakerOpen(time.Now()) {
		t.Fatal("so the next real failure still opens the breaker")
	}
}

// Usage that came back through a player's browser is the player's to
// write: it is held between nothing and the most the request could cost,
// and a passer-by is never charged past their reservation.
func TestRelayUsageIsNeverTrusted(t *testing.T) {
	m, f := relayCallModule(t, true)
	for _, tc := range []struct {
		usage string
		want  func(prompt, max int) int
	}{
		{`"usage":{"total_tokens":999999999}`, func(p, x int) int { return p + x }},
		{`"usage":{"total_tokens":-5000}`, func(p, x int) int { return p + x }}, // no usage: the whole request
		{`"usage":{"total_tokens":12}`, func(p, x int) int { return 12 }},
	} {
		c := relayCall(m)
		c.Retry = false
		prompt := estimateTokens(c.Messages) + requestOverhead(c)
		done := make(chan modelResult, 1)
		go func() { done <- m.callModel(c) }()
		req := f.next(t)
		m.relayCalls.deliver(5, relayResponse{Id: req.Id, Status: 200,
			Body: `{"choices":[{"finish_reason":"stop","message":{"content":"{}"}}],` + tc.usage + `}`})
		res := <-done
		if want := tc.want(prompt, c.MaxTokens); res.Tokens != want {
			t.Fatalf("%s: counted %d, want %d", tc.usage, res.Tokens, want)
		}
	}

	relay := route{kind: routeRelay, model: `player-model`}
	if !tryRoute(m, relay, 5, 2, 400) {
		t.Fatal("fixture: the passer-by's question fits")
	}
	settleToday(m, relay, 5, 2, 400, 5000)
	if strangerSpent(m, 2) != 400 {
		t.Fatalf("a passer-by pays at most what was held, got %d", strangerSpent(m, 2))
	}
	if !tryRoute(m, relay, 5, 3, 400) {
		t.Fatal("fixture: a second passer-by's question fits")
	}
	settleToday(m, relay, 5, 3, 400, -900)
	if strangerSpent(m, 3) != 0 {
		t.Fatalf("and never less than nothing, got %d", strangerSpent(m, 3))
	}
}

// panicTransport stands for anything that blows up while a call is out.
type panicTransport struct{}

func (panicTransport) RoundTrip(*http.Request) (*http.Response, error) { panic(`transport blew up`) }

// A reflection, a conversation summary or a core memory whose goroutine
// panics before its reply is applied still gives its reservation back, and
// its breaker leave: a panicked probe of a half-open breaker does not keep
// every companion on set lines until it expires.
func TestPanickedBackgroundCallsSettle(t *testing.T) {
	prev := apiframework.HTTPClient
	apiframework.HTTPClient = &http.Client{Transport: panicTransport{}}
	t.Cleanup(func() { apiframework.HTTPClient = prev })

	for name, start := range map[string]func(m *AICompanionModule, c *controller){
		`reflection`: func(m *AICompanionModule, c *controller) {
			m.startReflection(c.mind, c.profile, `Corvin`, 0)
		},
		`summary`: func(m *AICompanionModule, c *controller) {
			now := time.Now().Unix()
			c.convo = &conversation{RoomId: 7, Partner: `Corvin`, StartUnix: now, LastUnix: now, Exchanges: 5, OwnerSpoke: true,
				Lines: []Line{{Speaker: `Corvin`, Kind: `said`, Text: `a`}, {Speaker: `Corvin`, Kind: `said`, Text: `b`}}}
			m.closeConversation(c, `test`)
		},
		`core`: func(m *AICompanionModule, c *controller) {
			m.recordCore(c, `Corvin`, romanceCourting, true)
		},
	} {
		m, c := senderModule(`https://api.example.invalid`, true)
		m.minds = map[string]*Mind{mindIdentifier(c.mind.OwnerUserId, c.mind.MobId): c.mind}
		// Half-open: the call about to be made is the breaker's probe.
		m.fw().SetConsumerBreakerForTest(apiframework.ConsumerCompanion, 0, time.Now().Add(-time.Second))
		now := time.Now().Unix()
		util.LockMud()
		for i := 0; i < 6; i++ {
			c.mind.addLine(Line{Speaker: `Corvin`, Kind: `said`, Text: fmt.Sprintf(`line %d`, i), Unix: now}, 50)
		}
		start(m, c)
		held := serverHeld(m)
		util.UnlockMud()
		if held == 0 {
			t.Fatalf("%s: fixture: the call held a reservation", name)
		}
		deadline := time.Now().Add(3 * time.Second)
		for {
			util.LockMud()
			left, owner := serverHeld(m), ownerSpent(m, 1)
			util.UnlockMud()
			if left == 0 && owner == 0 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: a panicked call never gave back its reservation: outstanding=%d owner=%d", name, left, owner)
			}
			time.Sleep(10 * time.Millisecond)
		}
		for {
			if tk, ok := m.fw().Allow(apiframework.ConsumerCompanion, time.Now()); ok {
				if !tk.Probing() {
					t.Fatalf("%s: fixture: the breaker was half-open", name)
				}
				m.fw().Release(apiframework.ConsumerCompanion, tk)
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: a panicked probe was never handed back", name)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// On the owner's own key passers-by prompt nothing until the owner says
// so; on the server's key they may, until the owner says not. The owner's
// word, either way, holds on both keys, and a record saved before the
// choice had a default still means what it meant.
func TestStrangersDefaultOffOnlyOnTheOwnersKey(t *testing.T) {
	m := relayModule(t)
	m.bonds = bondState{Users: map[int]*bondRecord{5: {Profile: `mara`}}}
	relay, server := route{kind: routeRelay}, route{kind: routeServer}
	if !m.strangersOffOn(5, relay) || m.strangersOffOn(5, server) {
		t.Fatal("unset: off on the owner's key, on for the server's")
	}
	if !m.strangersOff(5) || m.strangerMayPrompt(5, 2) {
		t.Fatal("with the owner's relay live, a passer-by prompts nothing by default")
	}
	if !m.strangerMayPrompt(5, 0) {
		t.Fatal("the owner's own calls are untouched")
	}
	m.bonds.Users[5].StrangersOn = true
	if m.strangersOffOn(5, relay) || m.strangersOffOn(5, server) || !m.strangerMayPrompt(5, 2) {
		t.Fatal("strangers on: on for both keys")
	}
	m.bonds.Users[5].StrangersOn, m.bonds.Users[5].StrangersOff = false, true
	if !m.strangersOffOn(5, relay) || !m.strangersOffOn(5, server) {
		t.Fatal("strangers off: off for both keys")
	}
	if m.strangersOffOn(6, server) || !m.strangersOffOn(6, relay) {
		t.Fatal("no record at all is the default too")
	}

	var old bondState
	if err := yaml.Unmarshal([]byte("users:\n  5:\n    profile: mara\n    strangers_off: true\n"), &old); err != nil {
		t.Fatal(err)
	}
	m.bonds = old
	if !m.strangersOffOn(5, server) {
		t.Fatal("an old record that said off is still off, on the server's key too")
	}
	old.Users[5].StrangersOff, old.Users[5].StrangersOn = false, true
	b, err := yaml.Marshal(&old)
	if err != nil {
		t.Fatal(err)
	}
	var back bondState
	if err := yaml.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if !back.Users[5].StrangersOn || back.Users[5].StrangersOff {
		t.Fatalf("strangers on survives a save: %s", b)
	}
}

// companion-ai strangers on and off record the owner's word, and the bare
// status says, on the owner's own key, that passers-by are held off until
// the owner lets them.
func TestCompanionAIStrangersDefaultWording(t *testing.T) {
	owner, _, _, _ := harmWorld(t, `off`)
	m, _ := senderModule(`https://api.example.invalid`, true)
	withWebDomain(t, `example.org`)
	m.cfg.PlayerKeys, m.cfg.RelayOrigin = true, `https://keys.example.org`
	m.relays = newRelayTable()
	m.relays.ready(1, `player-model`, false)
	events.DrainQueuedMessagesForTest(1)
	say := func(arg string) string {
		t.Helper()
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
		return strings.Join(strings.Fields(sent[0]), ` `)
	}
	if got := say(`strangers`); !strings.Contains(got, `your own key`) || !strings.Contains(got, `strangers on`) {
		t.Fatalf("by default on the owner's key, the status says they are held off and how to allow them: %q", got)
	}
	if got := say(`strangers on`); !m.bonds.Users[1].StrangersOn || m.bonds.Users[1].StrangersOff || !strings.Contains(got, `paid for from your key`) {
		t.Fatalf("strangers on is recorded and says who pays: %q %+v", got, m.bonds.Users[1])
	}
	if !m.strangerMayPrompt(1, 2) {
		t.Fatal("and passers-by may now prompt calls on the owner's key")
	}
	if got := say(`strangers off`); !m.bonds.Users[1].StrangersOff || m.bonds.Users[1].StrangersOn || !strings.Contains(got, `set lines`) {
		t.Fatalf("strangers off is recorded: %q %+v", got, m.bonds.Users[1])
	}
}

// Passers-by together may spend only so much of one owner's companion in a
// day, on either key, however many of them there are.
func TestStrangerTokensPerOwnerCapsThemTogether(t *testing.T) {
	if got := buildConfig(nil).StrangerTokensPerOwner; got != 100000 {
		t.Fatalf("default StrangerTokensPerOwner is 100000, got %d", got)
	}
	for _, rt := range []route{{kind: routeServer}, {kind: routeRelay, model: `player-model`}} {
		m := relayModule(t)
		freshServer(t, 100000, 3, 60)
		m.cfg.StrangerDailyTokens, m.cfg.StrangerTokensPerOwner = 1000, 1500
		if !tryRoute(m, rt, 5, 2, 900) || !tryRoute(m, rt, 5, 3, 500) {
			t.Fatalf("%v: two passers-by within both caps are admitted", rt.kind)
		}
		if tryRoute(m, rt, 5, 4, 200) {
			t.Fatalf("%v: a third, within their own allowance, would overshoot the owner's cap", rt.kind)
		}
		if !tryRoute(m, rt, 6, 4, 200) {
			t.Fatalf("%v: another owner's companion has its own cap", rt.kind)
		}
		settleToday(m, rt, 5, 2, 900, 100)
		if strangersForSpent(m, 5) != 600 {
			t.Fatalf("%v: a settlement gives back what was not used, got %d", rt.kind, strangersForSpent(m, 5))
		}
		if !tryRoute(m, rt, 5, 4, 200) {
			t.Fatalf("%v: and the room it frees is usable", rt.kind)
		}
	}

	m := relayModule(t)
	m.cfg.StrangerTokensPerOwner = 1500
	c := &controller{ownerUserId: 5, instanceId: 42}
	m.bonds = bondState{Users: map[int]*bondRecord{5: {StrangersOn: true}}}
	setStrangersForSpent(m, 5, 1500)
	bram := &users.UserRecord{UserId: 2, Character: &characters.Character{Name: `Bram`}}
	if m.strangerMayAsk(bram, c) {
		t.Fatal("a spent owner's cap queues nothing more from passers-by")
	}
}

// R35, R36, R37: the first boot after the move hands the old save's
// allowances to the ledger once; the save still writes them, from the
// ledger, as the backup a quarantined budget.yaml re-seeds from.
func TestFirstBootSeedsTheOldAllowancesOnce(t *testing.T) {
	m := &AICompanionModule{cfg: Config{}}
	day := m.fw().Day()
	old := budgetState{Day: day, Tokens: 900, Calls: 4,
		Owners: map[int]int{5: 1200}, Strangers: map[int]int{2: 300}, StrangersFor: map[int]int{5: 300},
		Notices: map[int]int{5: 2}}
	for boot := 0; boot < 2; boot++ { // a second boot reading the same old file seeds nothing more
		m.restoreBudget(old)
		if ownerSpent(m, 5) != 1200 || strangerSpent(m, 2) != 300 || strangersForSpent(m, 5) != 300 || serverSpent(m) != 900 {
			t.Fatalf("boot %d: owner=%d stranger=%d perOwner=%d server=%d",
				boot, ownerSpent(m, 5), strangerSpent(m, 2), strangersForSpent(m, 5), serverSpent(m))
		}
	}
	if m.callsToday != 4 || m.noticesToday[5] != 2 || m.countersDay != day {
		t.Fatalf("the module's own counts: calls=%d notices=%d day=%s", m.callsToday, m.noticesToday[5], m.countersDay)
	}
	saved := m.budgetStateToSave()
	if saved.Owners[5] != 1200 || saved.Strangers[2] != 300 || saved.StrangersFor[5] != 300 {
		t.Fatalf("the backup is written from the ledger's counts: %+v", saved)
	}
	setOwnerSpent(m, 5, 1500)
	if m.budgetStateToSave().Owners[5] != 1500 {
		t.Fatal("the backup follows the ledger, not the file it was seeded from")
	}
	if saved.Day != day || saved.Calls != 4 || saved.Notices[5] != 2 || saved.Tokens != 900 {
		t.Fatalf("the rest is still written: %+v", saved)
	}

	stale := &AICompanionModule{cfg: Config{}}
	stale.restoreBudget(budgetState{Day: `1999-01-01`, Owners: map[int]int{5: 1}})
	if ownerSpent(stale, 5) != 0 {
		t.Fatal("a stale day seeds nothing")
	}

	// The old file's keys still read (the save before the move wrote them).
	var back budgetState
	if err := yaml.Unmarshal([]byte("day: \"2026-09-25\"\nowners:\n  5: 7\nstrangers:\n  2: 8\nstrangers_for:\n  5: 1234\n"), &back); err != nil {
		t.Fatal(err)
	}
	if back.Owners[5] != 7 || back.Strangers[2] != 8 || back.StrangersFor[5] != 1234 {
		t.Fatalf("old keys read: %+v", back)
	}
}

// A fight a passer-by started is planned on her owner's account, as any
// other fight: defending her owner is the owner's concern. The plan and
// the fight's end name no payer, so strangers being off on the owner's own
// key never leaves her planless.
func TestFightAPasserByStartedIsPlannedOnTheOwnersAccount(t *testing.T) {
	_, bram, _, her := harmWorld(t, `on`)
	m, c := senderModule(`https://api.example.invalid`, true)
	m.cfg.Enabled = true
	c.instanceId = her.InstanceId
	c.lastAttackBy = map[int]int64{2: time.Now().Unix() - 5}
	m.ctrls = map[int]*controller{1: c}
	m.bonds.Users[1].StrangersOff = true
	c.inFlight = true // queue only

	// Bram's blow is already waiting to be answered, as the listener
	// queues it.
	c.pending = []stimulus{{Kind: `attacked`, Speaker: `Bram`, AskerUserId: 2}}
	bram.Character.SetAggro(0, her.InstanceId, characters.DefaultAttack)
	her.Character.SetAggro(2, 0, characters.DefaultAttack)
	m.combatTick(c, users.GetByUserId(1), 10)
	if c.fight == nil || countKind(c.pending, `fight`) != 1 {
		t.Fatalf("fixture: Bram's attack starts a fight and a plan is asked for: %+v", c.pending)
	}
	batch, _ := nextBatch(c.pending, 1)
	if countKind(batch, `fight`) != 1 {
		t.Fatalf("the plan jumps the queue: it is decided first: %+v", batch)
	}
	if asker := strangerBehind(batch, 1); asker != 0 || !m.strangerMayPrompt(1, asker) {
		t.Fatalf("the plan is her owner's to pay for, strangers off or not: asker %d", asker)
	}
	if promptedBy(stimulus{Kind: `fight_over`, FromOwner: true}, 1) != 1 {
		t.Fatal("and so is the end of the fight")
	}
}

// goldWorld is her, her owner Corvin and a passer-by Bram in one room, with
// a module driving her for Corvin.
func goldWorld(t *testing.T) (*AICompanionModule, *controller, *users.UserRecord, *users.UserRecord, *rooms.Room, *mobs.Mob) {
	t.Helper()
	owner, bram, room, her := harmWorld(t, `off`)
	m, c := senderModule(`https://api.example.invalid`, true)
	m.cfg.Enabled = true
	c.instanceId = her.InstanceId
	c.lastAttackBy = map[int]int64{}
	m.ctrls = map[int]*controller{1: c}
	her.Character.Gold = 10
	m.noticeGold(c, her, owner, 1) // the purse is read once
	return m, c, owner, bram, room, her
}

// goldMemories is each coin gift she noted, in order (the working lines,
// which keep a repeat that the memory store folds into one).
func goldMemories(c *controller) []string {
	var out []string
	for _, l := range c.mind.RecentLines {
		if strings.Contains(l.Text, `put some coin in your hand`) {
			out = append(out, l.Text)
		}
	}
	return out
}

// Gold a passer-by gives her is theirs: remembered as from them, queued as
// their prompt, and never warming her to her owner. The purse growth it
// caused is not credited a second time.
func TestGoldIsCreditedToWhoGaveIt(t *testing.T) {
	m, c, owner, _, _, her := goldWorld(t)
	her.Character.Gold += 50
	m.onGoldGiven(events.GoldGiven{UserId: 2, MobInstanceId: her.InstanceId, Amount: 50})
	for r := uint64(2); r < 5; r++ {
		m.noticeGold(c, her, owner, r)
	}
	if got := goldMemories(c); len(got) != 1 || !strings.Contains(got[0], `Bram`) {
		t.Fatalf("one gift, from Bram: %q", got)
	}
	if n := c.mind.ruleChangesSince(`gift`, 0); n != 0 {
		t.Fatalf("a passer-by's coin does not warm her to her owner: %d changes", n)
	}
	if len(c.pending) != 1 || c.pending[0].FromOwner || c.pending[0].AskerUserId != 2 {
		t.Fatalf("queued as Bram's, to be paid by Bram: %+v", c.pending)
	}

	// The purse can be read before the event arrives: it waits a round.
	c.pending = nil
	her.Character.Gold += 20
	m.noticeGold(c, her, owner, 10)
	m.onGoldGiven(events.GoldGiven{UserId: 2, MobInstanceId: her.InstanceId, Amount: 20})
	m.noticeGold(c, her, owner, 11)
	m.noticeGold(c, her, owner, 12)
	if got := goldMemories(c); len(got) != 2 || !strings.Contains(got[1], `Bram`) {
		t.Fatalf("read first, still Bram's and once: %q", got)
	}
	if n := c.mind.ruleChangesSince(`gift`, 0); n != 0 {
		t.Fatalf("and her owner is never thanked for it: %d changes", n)
	}
}

// Her owner's own gold, named by the event, is the owner's gift.
func TestOwnersGoldIsTheOwnersGift(t *testing.T) {
	m, c, owner, _, room, her := goldWorld(t)
	room.RemovePlayer(2) // alone with her owner, so a second credit would show
	her.Character.Gold += 50
	m.onGoldGiven(events.GoldGiven{UserId: 1, MobInstanceId: her.InstanceId, Amount: 50})
	for r := uint64(2); r < 5; r++ {
		m.noticeGold(c, her, owner, r)
	}
	if got := goldMemories(c); len(got) != 1 || !strings.Contains(got[0], `Corvin`) {
		t.Fatalf("one gift, from Corvin: %q", got)
	}
	if n := c.mind.ruleChangesSince(`gift`, 0); n != 1 {
		t.Fatalf("her owner's coin warms her, once: %d changes", n)
	}
	if len(c.pending) != 1 || !c.pending[0].FromOwner {
		t.Fatalf("queued as the owner's: %+v", c.pending)
	}
}

// Coin no event names is credited to her owner only when nobody else was
// there who could have given it.
func TestUnnamedGoldIsNeverTheOwnersWhenOthersAreThere(t *testing.T) {
	m, c, owner, _, room, her := goldWorld(t)
	her.Character.Gold += 50
	m.noticeGold(c, her, owner, 2)
	m.noticeGold(c, her, owner, 3)
	if got := goldMemories(c); len(got) != 0 || c.mind.ruleChangesSince(`gift`, 0) != 0 {
		t.Fatalf("with Bram in the room nobody is thanked: %q", got)
	}

	room.RemovePlayer(2)
	her.Character.Gold += 50
	m.noticeGold(c, her, owner, 4)
	if got := goldMemories(c); len(got) != 0 {
		t.Fatalf("not in the round it was seen, while an event may still name a giver: %q", got)
	}
	m.noticeGold(c, her, owner, 5)
	if got := goldMemories(c); len(got) != 1 || !strings.Contains(got[0], `Corvin`) {
		t.Fatalf("alone with her owner, it is her owner's: %q", got)
	}
}

// The follow-up to a look is paid by whoever paid for the look: a
// passer-by's stays theirs (and opens no owner-only verb), her owner's
// stays the owner's, and so never joins a passer-by's next question.
func TestLookedFollowUpCarriesThePayer(t *testing.T) {
	saw := actionOutcome{Perceived: `a rusty key`}
	bram := lookedFollowUp([]stimulus{{Kind: `heard`, AskerUserId: 2}}, 1, saw)
	if bram.AskerUserId != 2 || strangerBehind([]stimulus{bram}, 1) != 2 || ownerPrompted([]stimulus{bram}) {
		t.Fatalf("a passer-by's look is followed up on their account: %+v", bram)
	}
	corvin := lookedFollowUp([]stimulus{{Kind: `heard`, FromOwner: true}}, 1, saw)
	if promptedBy(corvin, 1) != 1 || strangerBehind([]stimulus{corvin}, 1) != 0 || !ownerPrompted([]stimulus{corvin}) {
		t.Fatalf("her owner's look is followed up on the owner's: %+v", corvin)
	}
	batch, _ := nextBatch([]stimulus{{Kind: `heard`, AskerUserId: 3}, corvin}, 1)
	if len(batch) != 1 || strangerBehind(batch, 1) != 3 || batch[0].Kind != `heard` {
		t.Fatalf("and never rides a passer-by's question: %+v", batch)
	}
	own := lookedFollowUp([]stimulus{{Kind: `quiet`, FromOwner: true}}, 1, saw)
	if own.PaidBy != 0 || own.AskerUserId != 0 {
		t.Fatalf("her own look carries no payer: %+v", own)
	}
}

// "You notice" moments are calls on whoever pays: at most
// NoticeCallsPerDay a day per owner, and on the owner's own key with
// strangers off none while another player is in the room, since any
// passer-by walking in and out would otherwise spend the owner's key.
func TestNoticedIsCappedAndSparesTheOwnersKey(t *testing.T) {
	_, _, room, her := harmWorld(t, `off`)
	m := relayModule(t)
	m.relays.ready(1, `player-model`, false)
	m.bonds = bondState{Users: map[int]*bondRecord{}}
	m.cfg.NoticeCallsPerDay = 2
	m.cfg.NoticeCooldownSeconds = 0
	profiles, _ := loadProfiles()
	c := &controller{ownerUserId: 1, instanceId: her.InstanceId, profile: profiles[`mara`], mind: newMind(1, profiles[`mara`])}
	see := []thing{{Name: `a rusty key`}}
	now := time.Now()

	m.notice(c, see, now)
	if queued(c, `noticed`) {
		t.Fatal("on her owner's own key, strangers off, Bram in the room: nothing noticed aloud")
	}
	room.RemovePlayer(2)
	m.notice(c, see, now)
	if countKind(c.pending, `noticed`) != 1 {
		t.Fatalf("with only her owner here she notices: %+v", c.pending)
	}
	m.notice(c, see, now)
	m.notice(c, see, now)
	if n := countKind(c.pending, `noticed`); n != 2 {
		t.Fatalf("at most NoticeCallsPerDay a day: %d", n)
	}

	// On the server's key, strangers on by default: Bram does not stop it.
	c.pending = nil
	room.AddPlayer(2)
	m.relays.gone(1)
	pointAt(m, apiframework.DefaultBaseURL, `k`)
	m.noticesToday = nil
	m.notice(c, see, now)
	if countKind(c.pending, `noticed`) != 1 {
		t.Fatalf("on the server's key a passer-by in the room does not stop her noticing: %+v", c.pending)
	}
}

// heldServer takes each chat completion, hands its body to the test and
// waits until the test lets it answer with no usage.
func heldServer(t *testing.T) (*httptest.Server, chan []byte, chan struct{}) {
	t.Helper()
	bodies, release := make(chan []byte, 4), make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies <- b
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		fmt.Fprint(w, `{"choices":[{"finish_reason":"stop","message":{"content":"{}"}}]}`)
	}))
	t.Cleanup(srv.Close)
	return srv, bodies, release
}

// holdFromBody is the worst case of a request as it left: its messages,
// the schema that goes with every call, and a full answer.
func holdFromBody(t *testing.T, body []byte) int {
	t.Helper()
	var req struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
		MaxCompletionTokens int `json:"max_completion_tokens"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	msgs := make([]chatMessage, 0, len(req.Messages))
	for _, mm := range req.Messages {
		msgs = append(msgs, chatMessage{Content: mm.Content})
	}
	return worstCaseTokens(estimateTokens(msgs)+requestOverhead(modelCall{Schema: map[string]any{}}), req.MaxCompletionTokens, 0, false)
}

// The background calls (reflection, conversation summary, core memory)
// hold what they send, the schema included, as a decision does: a hold
// that leaves the schema out lets a budget be overspent by it.
func TestBackgroundHoldsIncludeTheSchema(t *testing.T) {
	srv, bodies, release := heldServer(t)
	m, c := senderModule(srv.URL, true)
	m.minds = map[string]*Mind{mindIdentifier(c.mind.OwnerUserId, c.mind.MobId): c.mind}
	now := time.Now().Unix()
	for i := 0; i < 6; i++ {
		c.mind.addLine(Line{Speaker: `Corvin`, Kind: `said`, Text: fmt.Sprintf(`line %d`, i), Unix: now}, 50)
	}
	start := map[string]func(){
		`reflection`: func() { m.startReflection(c.mind, c.profile, `Corvin`, 0) },
		`summary`: func() {
			c.convo = &conversation{RoomId: 7, Partner: `Corvin`, StartUnix: now, LastUnix: now, Exchanges: 5, OwnerSpoke: true,
				Lines: []Line{{Speaker: `Corvin`, Kind: `said`, Text: `a`}, {Speaker: `Corvin`, Kind: `said`, Text: `b`}}}
			m.closeConversation(c, `test`)
		},
		`core memory`: func() { m.recordCore(c, `Corvin`, romanceCourting, true) },
	}
	for name, begin := range start {
		util.LockMud()
		begin()
		util.UnlockMud()
		var body []byte
		select {
		case body = <-bodies:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: no request arrived", name)
		}
		util.LockMud()
		held := serverHeld(m)
		util.UnlockMud()
		if want := holdFromBody(t, body); held != want {
			t.Errorf("%s: held %d, want %d (the schema included)", name, held, want)
		}
		release <- struct{}{}
		waitSettled(t, m)
	}
}

// Each round of questions can bring back as many answers as a reply may
// ask (maxToolCallsPerReply), each as long as an answer may be
// (maxToolAnswerRunes, counted at a token a rune, as estimateTokens would
// count its bytes at worst). The next round sends all of it again, so the
// worst case holds all of it.
func TestWorstCaseHoldsFullToolAnswers(t *testing.T) {
	plain := worstCaseTokens(1000, 500, 0, false)
	oneRound := worstCaseTokens(1000, 500, 1, false)
	answers := maxToolCallsPerReply * (maxToolAnswerRunes + 8)
	if want := plain + 1000 + 500 + answers + 500; oneRound != want {
		t.Fatalf("one round of questions: %d, want %d", oneRound, want)
	}
	long := make([]chatMessage, 0, maxToolCallsPerReply)
	for i := 0; i < maxToolCallsPerReply; i++ {
		long = append(long, chatMessage{Role: `tool`, Content: strings.Repeat(`𝄞`, maxToolAnswerRunes)})
	}
	if got := estimateTokens(long); got > answers {
		t.Fatalf("the allowance covers %d answers at the cap: %d estimated, %d held", maxToolCallsPerReply, got, answers)
	}
}

// A round of questions that was answered and billed stays spent when the
// game's answering of the next round panics: the settlement is told what
// the completed rounds cost, not nothing.
func TestPanicInToolAnswersKeepsCompletedSpend(t *testing.T) {
	_, _, _, her := harmWorld(t, `off`)
	srv := usageServer(t, `{"choices":[{"finish_reason":"tool_calls","message":{"tool_calls":[`+
		`{"id":"q1","type":"function","function":{"name":"recall","arguments":"{\"query\":\"the old mill\"}"}}]}}],`+
		`"usage":{"total_tokens":77}}`, nil)
	m := &AICompanionModule{}
	m.syncConsent()
	// Her mind is missing, so answering "recall" panics.
	m.ctrls = map[int]*controller{1: {ownerUserId: 1, instanceId: her.InstanceId, seq: 1}}
	call := modelCall{BaseURL: srv.URL, APIKey: `k`, Model: `m`, Timeout: 2 * time.Second,
		Messages: []chatMessage{{Role: `user`, Content: `hi`}}, SchemaName: `s`, Schema: decisionSchema(), OwnerUserId: 1}
	spent := 0
	func() {
		defer func() { _ = recover() }()
		m.callWithTools(call, 1, 1, 0, nil, 2, &spent)
		t.Fatal("fixture: answering must panic")
	}()
	if spent != 77 {
		t.Fatalf("the completed round's spend is kept: %d", spent)
	}
}

// A call held before midnight and settled after it gives nothing back to
// its payer: their count started the new day at nothing, and a refund
// would come off what they really spent today.
func TestAHoldAcrossMidnightRefundsNothing(t *testing.T) {
	m := relayModule(t)
	m.cfg.DailyTokensPerCompanion, m.cfg.StrangerDailyTokens, m.cfg.StrangerTokensPerOwner = 100000, 100000, 100000
	server, relay := route{kind: routeServer}, route{kind: routeRelay, model: `player-model`}
	today := time.Now().UTC()
	m.fw().SetClockForTest(func() time.Time { return today })

	ownerHold, ok1 := m.reserveRoute(server, 5, 0, 900)
	strangerHold, ok2 := m.reserveRoute(relay, 5, 2, 400)
	if !ok1 || !ok2 {
		t.Fatal("fixture: both holds fit")
	}
	// They were held yesterday, and the day turns.
	m.fw().SetClockForTest(func() time.Time { return today.Add(24 * time.Hour) })
	// Today's own spending.
	setOwnerSpent(m, 5, 500)
	setStrangerSpent(m, 2, 300)
	setStrangersForSpent(m, 5, 300)

	m.settleRoute(ownerHold, 100)
	m.settleRoute(strangerHold, 0)
	if ownerSpent(m, 5) != 500 {
		t.Fatalf("the owner's count today is untouched by yesterday's hold: %d", ownerSpent(m, 5))
	}
	if strangerSpent(m, 2) != 300 || strangersForSpent(m, 5) != 300 {
		t.Fatalf("so is the passer-by's: %d, %d", strangerSpent(m, 2), strangersForSpent(m, 5))
	}
	if serverHeld(m) != 0 {
		t.Fatalf("and nothing is left held: %d", serverHeld(m))
	}

	// Control: the same holds settled on their own day do give back.
	h, _ := m.reserveRoute(server, 5, 0, 900)
	m.settleRoute(h, 100)
	if ownerSpent(m, 5) != 600 {
		t.Fatalf("a same-day hold settles at what was used: %d", ownerSpent(m, 5))
	}
}

// An owner whose own key was live this session pays for their own
// companion's summaries: with their relay down (at logout their browser
// is closing) a finished talk waits for it, as the reflection does, and is
// never moved to the server's key.
func TestRelayOwnersSummaryWaitsForTheirRelay(t *testing.T) {
	srv, hits := countingServer(t)
	m, f := relayCallModule(t, true)
	pointAt(m, srv.URL, `k`)
	m.cfg.Model, m.cfg.FastModel = `m`, `m`
	m.cfg.ConversationSummaries = true
	m.cfg.DailyTokensPerCompanion = 100000
	freshServer(t, 1000000, 3, 60)
	m.cfg.FastTimeoutSeconds, m.cfg.FastMaxCompletionTokens = 5, 200
	profiles, _ := loadProfiles()
	p := profiles[`mara`]
	c := &controller{profile: p, mind: newMind(5, p), ownerUserId: 5, relaySeen: true}
	m.minds = map[string]*Mind{mindIdentifier(5, c.mind.MobId): c.mind}
	m.ctrls = map[int]*controller{5: c}
	now := time.Now().Unix()
	talk := func() {
		c.convo = &conversation{RoomId: 7, Partner: `Corvin`, StartUnix: now, LastUnix: now, Exchanges: 5, OwnerSpoke: true,
			Lines: []Line{{Speaker: `Corvin`, Kind: `said`, Text: `a`}, {Speaker: `Corvin`, Kind: `said`, Text: `b`}}}
	}

	m.relays.gone(5)
	if m.route(5).kind != routeServer {
		t.Fatal("fixture: with the relay down the server's key would answer")
	}
	util.LockMud()
	talk()
	m.closeConversation(c, `they logged out`)
	util.UnlockMud()
	if hits.Load() != 0 {
		t.Fatalf("nothing goes to the server's key: %d requests", hits.Load())
	}
	if len(m.deferredSummaries[5]) != 1 {
		t.Fatalf("the talk waits for her owner's relay: %d kept", len(m.deferredSummaries[5]))
	}
	if len(c.mind.Memories) != 0 {
		t.Fatalf("and the talk is not reduced to its best note: %+v", c.mind.Memories)
	}

	// Back, relay up: the talk is summed up through it.
	m.relays.ready(5, `player-model`, false)
	util.LockMud()
	m.startDueSummaries(5)
	util.UnlockMud()
	req := f.next(t)
	m.relayCalls.deliver(5, relayResponse{Id: req.Id, Status: 200,
		Body: `{"choices":[{"finish_reason":"stop","message":{"content":"{\"summary\":\"We talked.\",\"importance\":4,\"emotion\":\"neutral\",\"facts\":[]}"}}]}`})
	waitSettled(t, m)
	if hits.Load() != 0 {
		t.Fatalf("still nothing on the server's key: %d", hits.Load())
	}

	// Control: an owner who never used their own key is summed up on the
	// server's key at once.
	c.relaySeen = false
	m.relays.gone(5)
	util.LockMud()
	talk()
	m.closeConversation(c, `they logged out`)
	util.UnlockMud()
	waitSettled(t, m)
	if hits.Load() != 1 {
		t.Fatalf("control: the server's key sums up everyone else's talk: %d", hits.Load())
	}
}
