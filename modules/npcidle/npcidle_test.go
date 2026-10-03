package npcidle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/lively"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/npcidle"
)

// The test binary never sees a real key: a developer's OPENAI_API_KEY is
// cleared and the framework's server settings are fixed per test.
func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)
	_ = os.Unsetenv(`OPENAI_API_KEY`)
	apiframework.SetServerForTest(apiframework.ServerSettings{
		Endpoint:         apiframework.Endpoint{BaseURL: apiframework.DefaultBaseURL},
		DailyTokenBudget: 2000000, BreakerErrors: 3, BreakerSeconds: 300,
	})
	apiframework.ResetBudgetForTest(``)
	os.Exit(m.Run())
}

const goodMoment = `{"kind":"emote","text":"flaps a rag at a sparrow that has claimed the top shelf"}`

// fakeRelay stands in for a player's key relay: it answers every request
// with content, as the player's provider would.
type fakeRelay struct {
	allowed map[int]bool
	content string
	status  int
	fail    error
	sends   int
	body    string
	carries apiframework.Carries
	results []string // purpose:outcome
}

func (r *fakeRelay) Model(userId int, purpose string) (string, bool) {
	if purpose != apiframework.PurposeNPCIdle || !r.allowed[userId] {
		return ``, false
	}
	return `player-model`, true
}

func (r *fakeRelay) Send(ctx context.Context, userId int, body []byte, carries apiframework.Carries) (int, []byte, bool, error) {
	r.sends++
	r.body, r.carries = string(body), carries
	if r.fail != nil {
		return 0, nil, false, r.fail
	}
	if r.status != 0 && r.status != 200 {
		return r.status, nil, true, nil
	}
	b, _ := json.Marshal(map[string]any{
		`choices`: []any{map[string]any{`finish_reason`: `stop`, `message`: map[string]any{`content`: r.content}}},
		`usage`:   map[string]any{`total_tokens`: 300},
	})
	return 200, b, true, nil
}

func (r *fakeRelay) Result(userId int, purpose string, err error) {
	r.results = append(r.results, fmt.Sprintf(`%s:%v`, purpose, err == nil))
}

// fakeModeration is the server's moderation endpoint.
type fakeModeration struct {
	srv     *httptest.Server
	flagged bool
	status  int
	checks  int32
}

func newFakeModeration(t *testing.T) *fakeModeration {
	f := &fakeModeration{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, `/moderations`) {
			w.WriteHeader(404)
			return
		}
		atomic.AddInt32(&f.checks, 1)
		if f.status != 0 {
			w.WriteHeader(f.status)
			return
		}
		fmt.Fprintf(w, `{"results":[{"flagged":%t}]}`, f.flagged)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// server gives one test the server's key at baseURL (empty key: none) and
// fresh books.
func server(t *testing.T, baseURL string, key string) {
	t.Helper()
	restore := apiframework.SetServerForTest(apiframework.ServerSettings{
		Endpoint:         apiframework.Endpoint{BaseURL: baseURL, APIKey: key},
		DailyTokenBudget: 1000, BreakerErrors: 3, BreakerSeconds: 60,
	})
	apiframework.ResetBudgetForTest(``)
	apiframework.ResetBreaker()
	t.Cleanup(func() {
		restore()
		apiframework.ResetBudgetForTest(``)
		apiframework.ResetBreaker()
	})
}

// testModule is an enabled module on relay r, with no server key.
func testModule(t *testing.T, r *fakeRelay, change func(c *Config)) *NpcIdleModule {
	t.Helper()
	server(t, apiframework.DefaultBaseURL, ``)
	cfg := buildConfig(nil)
	cfg.TimeoutSeconds = 5
	if change != nil {
		change(&cfg)
	}
	m := &NpcIdleModule{cfg: cfg, turns: lively.NewTurns()}
	m.turns.Relay = func() apiframework.Relay {
		if r == nil {
			return nil
		}
		return r
	}
	return m
}

func request() npcidle.Request {
	return npcidle.Request{
		NPC:        npcidle.NPC{Name: `Old Brask`, Description: `A stooped shopkeeper.`, Merchant: true, Wares: []string{`Rye Loaf`}},
		Place:      npcidle.Place{Title: `The Flour Shop`, Description: `Sacks lean against a crooked shelf.`},
		Travellers: 1,
		TimeOfDay:  `day`,
	}
}

func TestDefaults(t *testing.T) {
	c := buildConfig(nil)
	if !c.Enabled || c.Chance != 5 || c.MinSecondsPerPlayer != 30 || !c.ModerateOutput || c.DailyTokensPerUser != 20000 {
		t.Fatalf("on by default, 5%%, spaced, moderated, capped: %+v", c)
	}
	c = buildConfig(func(k string) any {
		return map[string]any{`Chance`: 250, `Enabled`: `false`, `MinSecondsPerPlayer`: -3}[k]
	})
	if c.Enabled || c.Chance != 100 || c.MinSecondsPerPlayer != 0 {
		t.Fatalf("bounded and read as written: %+v", c)
	}
}

func TestReserveTakesOnlyAKeyThatMayBeUsed(t *testing.T) {
	r := &fakeRelay{allowed: map[int]bool{7: true}}
	m := testModule(t, r, nil)
	clock := time.Now()
	m.turns.Now = func() time.Time { return clock }

	if m.Reserve(8) {
		t.Fatal("a player who did not allow it, or has no relay up, lends nothing")
	}
	if !m.Reserve(7) {
		t.Fatal("allowed: their turn is taken")
	}
	if m.Reserve(7) {
		t.Fatal("one moment at a time on a key")
	}
	m.turns.Release(7)
	if m.Reserve(7) {
		t.Fatal("spaced by MinSecondsPerPlayer")
	}
	clock = clock.Add(31 * time.Second)
	if !m.Reserve(7) {
		t.Fatal("after the spacing, again")
	}
	m.turns.Release(7)

	clock = clock.Add(time.Hour)
	m.cfg.Enabled = false
	if m.Reserve(7) || m.Chance() != 0 {
		t.Fatal("switched off: nothing")
	}
	m.cfg.Enabled = true

	m.turns.Relay = func() apiframework.Relay { return nil }
	if m.Reserve(7) {
		t.Fatal("no relay at all (the companion module off): nothing")
	}
}

func TestASpentAllowanceTakesNothing(t *testing.T) {
	r := &fakeRelay{allowed: map[int]bool{7: true}, content: goodMoment}
	m := testModule(t, r, func(c *Config) { c.DailyTokensPerUser = 500 })
	apiframework.Shared().SetAllowanceForTest(apiframework.DimNPCIdleKeyholder, 7, 500)
	if m.Reserve(7) {
		t.Fatal("their allowance for the day is spent")
	}
}

func TestAMomentIsWrittenOnThePlayersKeyAlone(t *testing.T) {
	r := &fakeRelay{allowed: map[int]bool{7: true}, content: goodMoment}
	m := testModule(t, r, func(c *Config) { c.ModerateOutput = false })
	if !m.Reserve(7) {
		t.Fatal("fixture")
	}
	res, err := m.Generate(context.Background(), 7, request())
	if err != nil || res.Kind != npcidle.KindEmote || !strings.Contains(res.Text, `sparrow`) || res.KeyholderOnly {
		t.Fatalf("a moment for everyone: %+v %v", res, err)
	}
	if r.carries != apiframework.CarriesNoPlayerData {
		t.Fatal("authored text only: no player data")
	}
	var body struct {
		Model          string `json:"model"`
		ResponseFormat struct {
			JSONSchema struct {
				Name string `json:"name"`
			} `json:"json_schema"`
		} `json:"response_format"`
	}
	_ = json.Unmarshal([]byte(r.body), &body)
	if body.Model != `player-model` || body.ResponseFormat.JSONSchema.Name != npcidle.ReplySchemaName {
		t.Fatalf("the player's model, under the schema the relay allows: %+v", body)
	}
	if !strings.Contains(r.body, `Old Brask`) || !strings.Contains(r.body, `Rye Loaf`) || !strings.Contains(r.body, `crooked shelf`) {
		t.Fatal("the NPC, its wares and the room are in the prompt")
	}
	if strings.Join(r.results, `,`) != apiframework.PurposeNPCIdle+`:true` {
		t.Fatalf("its outcome feeds this feature's own breaker on the key only: %v", r.results)
	}
	today := apiframework.Today()
	if today.Tokens != 0 {
		t.Fatalf("nothing of the server's budget is spent: %d", today.Tokens)
	}
	if apiframework.Allowance(apiframework.DimNPCIdleKeyholder, 7) != 300 {
		t.Fatalf("the player's own allowance is: %d", apiframework.Allowance(apiframework.DimNPCIdleKeyholder, 7))
	}
	if m.turns.Busy(7) {
		t.Fatal("their turn was given back")
	}
}

func TestFailuresAreTheKeysOnlyWhenTheyAre(t *testing.T) {
	r := &fakeRelay{allowed: map[int]bool{7: true}, status: 500}
	m := testModule(t, r, nil)
	if _, err := m.Generate(context.Background(), 7, request()); err == nil {
		t.Fatal("a provider error is no moment")
	}
	r.status, r.content = 200, `not json`
	if _, err := m.Generate(context.Background(), 7, request()); err == nil {
		t.Fatal("an answer outside the schema is no moment")
	}
	r.content = `{"kind":"emote","text":"waves at you warmly"}`
	if _, err := m.Generate(context.Background(), 7, request()); !errors.Is(err, npcidle.ErrUnusable) {
		t.Fatalf("a moment the rules refuse is no moment: %v", err)
	}
	want := strings.Join([]string{apiframework.PurposeNPCIdle + `:false`, apiframework.PurposeNPCIdle + `:false`, apiframework.PurposeNPCIdle + `:true`}, `,`)
	if got := strings.Join(r.results, `,`); got != want {
		t.Fatalf("the provider failing and the schema ignored count; an answer the rules refuse does not: %v", got)
	}
	if m.turns.Busy(7) {
		t.Fatal("every ending gives the turn back")
	}
}

func TestModeration(t *testing.T) {
	r := &fakeRelay{allowed: map[int]bool{7: true}, content: goodMoment}

	// No server key: nobody can check it, so it is the keyholder's alone.
	m := testModule(t, r, nil)
	res, err := m.Generate(context.Background(), 7, request())
	if err != nil || !res.KeyholderOnly {
		t.Fatalf("unmoderated text reaches only the player whose key wrote it: %+v %v", res, err)
	}

	mod := newFakeModeration(t)
	server(t, mod.srv.URL, `sk-test`)
	res, err = m.Generate(context.Background(), 7, request())
	if err != nil || res.KeyholderOnly || atomic.LoadInt32(&mod.checks) != 1 {
		t.Fatalf("checked and clean: everyone reads it (%+v %v)", res, err)
	}

	mod.flagged = true
	if _, err := m.Generate(context.Background(), 7, request()); !errors.Is(err, lively.ErrNotModerated) {
		t.Fatalf("flagged: kept out entirely: %v", err)
	}
	mod.flagged, mod.status = false, 500
	if _, err := m.Generate(context.Background(), 7, request()); err == nil {
		t.Fatal("a check that fails keeps it out")
	}

	m.cfg.ModerateOutput = false
	checks := atomic.LoadInt32(&mod.checks)
	res, err = m.Generate(context.Background(), 7, request())
	if err != nil || res.KeyholderOnly || atomic.LoadInt32(&mod.checks) != checks {
		t.Fatalf("moderation off: shown to everyone, unchecked (%+v %v)", res, err)
	}
}

// The browser relay must accept this module's schema name, and only under
// the lively choice: it is listed in relay.js LIVELY_SCHEMAS.
func TestTheRelayKnowsTheSchema(t *testing.T) {
	src, err := os.ReadFile(`../aicompanion/relayweb/relay.js`)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`var LIVELY_SCHEMAS = \[([^\]]*)\];`).FindStringSubmatch(string(src))
	if m == nil {
		t.Fatal("relay.js declares LIVELY_SCHEMAS")
	}
	if !strings.Contains(m[1], `'`+npcidle.ReplySchemaName+`'`) {
		t.Fatalf("relay.js LIVELY_SCHEMAS must list %q: [%s]", npcidle.ReplySchemaName, m[1])
	}
}

func TestThePromptNamesNoPlayer(t *testing.T) {
	msgs := buildMessages(request())
	if len(msgs) != 2 || msgs[0].Role != `system` || !strings.Contains(msgs[0].Content, `Never use the word you`) {
		t.Fatal("the system prompt keeps players out of it")
	}
	if !strings.Contains(msgs[1].Content, `"travellers": 1`) {
		t.Fatal("travellers are counted")
	}
}
