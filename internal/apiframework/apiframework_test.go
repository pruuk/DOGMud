package apiframework

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// No test here ever reads a real key or prints one: TestMain clears
// OPENAI_API_KEY, Server() is fixed per test, and key checks compare
// without printing.
func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)
	_ = os.Unsetenv(`OPENAI_API_KEY`)
	SetServerForTest(ServerSettings{Endpoint: Endpoint{BaseURL: DefaultBaseURL}, DailyTokenBudget: 1000, BreakerErrors: 2, BreakerSeconds: 60})
	ResetBudgetForTest(``)
	os.Exit(m.Run())
}

func TestChatBodyAndToolMessages(t *testing.T) {
	b, err := Chat{
		Model: `m`, SchemaName: `s`, Schema: map[string]any{`type`: `object`}, MaxTokens: 50, Effort: `low`,
		Messages: []Message{
			{Role: `user`, Content: `hi`},
			{Role: `assistant`, ToolCalls: []ToolCall{{Id: `c1`, Type: `function`, Function: ToolFunction{Name: `recall`, Arguments: `{}`}}}},
			{Role: `tool`, ToolCallId: `c1`, Content: `nothing`},
		},
	}.Body()
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"strict":true`, `"name":"s"`, `"max_completion_tokens":50`, `"reasoning_effort":"low"`,
		`"role":"assistant","content":null,"tool_calls"`, `"tool_call_id":"c1"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("body lacks %s: %s", want, s)
		}
	}
	if strings.Contains(s, `temperature`) {
		t.Fatal("temperature 0 is not sent")
	}
}

func TestDecodeChat(t *testing.T) {
	ok := DecodeChat(200, []byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"a\":1}"}}],"usage":{"total_tokens":42}}`))
	if ok.Err != nil || ok.Content != `{"a":1}` || ok.Tokens != 42 {
		t.Fatalf("decode: %+v", ok)
	}
	for name, raw := range map[string]string{
		`refused`:   `{"choices":[{"message":{"refusal":"no"}}]}`,
		`truncated`: `{"choices":[{"finish_reason":"length","message":{"content":"x"}}]}`,
		`empty`:     `{"choices":[{"finish_reason":"stop","message":{"content":" "}}]}`,
		`none`:      `{"choices":[]}`,
	} {
		if DecodeChat(200, []byte(raw)).Err == nil {
			t.Errorf("%s must be an error", name)
		}
	}
	calls := `{"choices":[{"message":{"tool_calls":[` + strings.Repeat(`{"id":"x","type":"function","function":{"name":"f","arguments":"{}"}},`, 6) + `{"id":"y","type":"function","function":{"name":"f","arguments":"{}"}}]}}]}`
	if r := DecodeChat(200, []byte(calls)); r.Err != nil || len(r.ToolCalls) != MaxToolCallsPerReply {
		t.Fatalf("tool calls capped: %+v", r)
	}
	if r := DecodeChat(500, []byte(`busy`)); r.Err == nil || !strings.Contains(r.Err.Error(), `500`) {
		t.Fatal("a bad status is an error")
	}
}

func TestCharged(t *testing.T) {
	if n, est := Charged(240, true, 200, 100, 800, false); n != 240 || est {
		t.Fatal("what was reported")
	}
	if n, est := Charged(0, true, 0, 100, 800, false); n != 900 || !est {
		t.Fatal("sent with no usage: the worst case")
	}
	if n, _ := Charged(0, false, 0, 100, 800, false); n != 0 {
		t.Fatal("never sent: nothing")
	}
	if n, _ := Charged(99999, true, 200, 100, 800, true); n != 900 {
		t.Fatal("a relayed count is held to the most one request could cost")
	}
	if n, _ := Charged(-5, false, 400, 100, 800, true); n != 0 {
		t.Fatal("never below nothing")
	}
}

// countingServer answers everything and counts what reaches it.
func countingServer(t *testing.T) (*httptest.Server, *atomic.Int64) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch {
		case strings.HasSuffix(r.URL.Path, `/moderations`):
			var in struct {
				Input []string `json:"input"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			out := `{"results":[`
			for i, s := range in.Input {
				if i > 0 {
					out += `,`
				}
				out += fmt.Sprintf(`{"flagged":%t}`, strings.Contains(s, `BAD`))
			}
			fmt.Fprint(w, out+`]}`)
		case strings.HasSuffix(r.URL.Path, `/models`):
			fmt.Fprint(w, `{"data":[{"id":"m"}]}`)
		default:
			_, _ = io.Copy(io.Discard, r.Body)
			fmt.Fprint(w, `{"choices":[{"finish_reason":"stop","message":{"content":"{}"}}],"usage":{"total_tokens":7}}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// The door: a request carrying player data needs an Admit hook, and a hook
// that refuses stops it before it leaves.
func TestPostDoor(t *testing.T) {
	srv, hits := countingServer(t)
	ep := Endpoint{BaseURL: srv.URL, APIKey: `k`}

	if ex := Post(context.Background(), ep, `/chat/completions`, []byte(`{}`), CarriesPlayerData, nil); !errors.Is(ex.Err, ErrNotAdmitted) || ex.Sent {
		t.Fatalf("player data with no door is refused: %+v", ex)
	}
	no := errors.New(`declined`)
	if ex := Post(context.Background(), ep, `/chat/completions`, []byte(`{}`), CarriesPlayerData, func(string) error { return no }); !errors.Is(ex.Err, no) || ex.Sent {
		t.Fatalf("a refusing door stops it: %+v", ex)
	}
	if hits.Load() != 0 {
		t.Fatal("nothing refused may reach the provider")
	}
	var path string
	ex := Post(context.Background(), ep, `/chat/completions`, []byte(`{}`), CarriesPlayerData, func(p string) error { path = p; return nil })
	if ex.Err != nil || ex.Status != 200 || !ex.Sent || path != `/chat/completions` {
		t.Fatalf("an admitted request: %+v %q", ex, path)
	}
	if ex := Post(context.Background(), ep, `/chat/completions`, []byte(`{}`), CarriesNoPlayerData, nil); ex.Err != nil || ex.Status != 200 {
		t.Fatalf("no player data needs no door: %+v", ex)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if ex := Post(cancelled, ep, `/chat/completions`, []byte(`{}`), CarriesNoPlayerData, nil); ex.Err == nil || ex.Sent {
		t.Fatal("a call given up on before it left sends nothing")
	}
	dead := Post(context.Background(), Endpoint{BaseURL: `http://127.0.0.1:1`}, `/x`, nil, CarriesNoPlayerData, nil)
	if dead.Err == nil || dead.Sent {
		t.Fatalf("a connection never made was never sent: %+v", dead)
	}
}

func TestModerateAndListModels(t *testing.T) {
	srv, _ := countingServer(t)
	ep := Endpoint{BaseURL: srv.URL, APIKey: `k`}
	flags, err := Moderate(ep, `m`, time.Second, []string{`fine`, `BAD`}, CarriesNoPlayerData, nil)
	if err != nil || len(flags) != 2 || flags[0] || !flags[1] {
		t.Fatalf("moderation: %v %v", flags, err)
	}
	if _, err := Moderate(ep, `m`, time.Second, []string{`x`}, CarriesPlayerData, nil); !errors.Is(err, ErrNotAdmitted) {
		t.Fatal("moderating a player's words goes through the door")
	}
	if ids := ListModels(ep); !ids[`m`] {
		t.Fatalf("models: %v", ids)
	}
}

func TestBudgetReserveSettleAndShares(t *testing.T) {
	ResetBudgetForTest(``)
	h1, err := budget.reserve(ConsumerCompanion, 600, 1000, 0, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := budget.reserve(ConsumerBaubles, 500, 1000, 0, true, nil); !errors.Is(err, ErrOverBudget) {
		t.Fatal("one budget: the companion's hold leaves no room")
	}
	h2, err := budget.reserve(ConsumerBaubles, 300, 1000, 0, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	budget.settle(h1, 100, false)
	budget.settle(h2, 250, true)
	u := Today()
	if u.Tokens != 350 || u.Outstanding != 0 || u.Calls != 2 || u.Failures != 1 {
		t.Fatalf("settled: %+v", u)
	}
	shares := map[string]int{}
	for _, c := range u.ByConsumer {
		shares[c.Consumer] = c.Tokens
	}
	if shares[ConsumerCompanion] != 100 || shares[ConsumerBaubles] != 250 {
		t.Fatalf("per consumer: %v", shares)
	}
	if _, err := budget.reserve(ConsumerBaubles, 1000, 0, 0, true, nil); err != nil {
		t.Fatal("limit 0 is no cap")
	}
}

// A call in flight across midnight is charged to the day it finished in,
// at what it really used; its hold neither leaks into nor vanishes from the
// new day.
func TestBudgetRollsOverWithCallsInFlight(t *testing.T) {
	ResetBudgetForTest(``)
	day1 := time.Date(2026, 9, 26, 23, 59, 0, 0, time.UTC)
	SetClockForTest(func() time.Time { return day1 })
	t.Cleanup(func() { SetClockForTest(time.Now) })
	h, err := budget.reserve(ConsumerBaubles, 1000, 5000, 0, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	SetClockForTest(func() time.Time { return day1.Add(2 * time.Minute) })
	if u := Today(); u.Day != `2026-09-27` || u.Tokens != 1000 || u.Outstanding != 1000 {
		t.Fatalf("the new day starts at what is still held: %+v", u)
	}
	budget.settle(h, 200, false)
	if u := Today(); u.Tokens != 200 || u.Outstanding != 0 {
		t.Fatalf("after settling: %+v", u)
	}
}

func TestBudgetSavesAndLoads(t *testing.T) {
	dir := t.TempDir()
	ResetBudgetForTest(dir)
	t.Cleanup(func() { ResetBudgetForTest(``) })
	h, _ := budget.reserve(ConsumerCompanion, 500, 0, 0, true, nil)
	budget.settle(h, 123, false)
	SaveBudget()

	ResetBudgetForTest(dir) // a restart
	if u := Today(); u.Tokens != 123 {
		t.Fatalf("a restart keeps the day's spending: %+v", u)
	}

	// A corrupt file is moved aside and the day starts from nothing.
	if err := os.WriteFile(filepath.Join(dir, `budget.yaml`), []byte("day: [unclosed"), 0644); err != nil {
		t.Fatal(err)
	}
	ResetBudgetForTest(dir)
	if u := Today(); u.Tokens != 0 {
		t.Fatalf("corrupt: a fresh day: %+v", u)
	}
	entries, _ := os.ReadDir(dir)
	quarantined := false
	for _, e := range entries {
		if strings.Contains(e.Name(), `.corrupt-`) {
			quarantined = true
		}
	}
	if !quarantined {
		t.Fatal("the corrupt file is kept aside, never deleted")
	}
}

func TestSeedTokensOnlyOnceAndOnlyToday(t *testing.T) {
	ResetBudgetForTest(``)
	today := Today().Day
	SeedTokens(ConsumerCompanion, `1999-01-01`, 400)
	if Today().Tokens != 0 {
		t.Fatal("another day's spending is not today's")
	}
	SeedTokens(ConsumerCompanion, today, 400)
	SeedTokens(ConsumerCompanion, today, 400)
	if Today().Tokens != 400 {
		t.Fatalf("seeded once: %d", Today().Tokens)
	}
}

// status is a provider reply with that HTTP status, as DecodeChat gives it.
func status(code int) error { return DecodeChat(code, []byte(`nope`)).Err }

func TestProviderFailureIsOnlyTheProviderOrKey(t *testing.T) {
	for _, err := range []error{status(500), status(503), status(429), status(401), status(403), status(408),
		context.DeadlineExceeded, &net.OpError{Op: `dial`, Err: errors.New(`refused`)}} {
		if !ProviderFailure(err) {
			t.Errorf("%v says the provider or key is unwell", err)
		}
	}
	for _, err := range []error{nil, status(400), status(404), status(422), context.Canceled,
		errors.New(`model refused`), errors.New(`model returned empty content`), fmt.Errorf(`decode response: %w`, errors.New(`bad json`))} {
		if ProviderFailure(err) {
			t.Errorf("%v is one request, not the provider", err)
		}
	}
	// A 403 that refuses one model (the project cannot use it) is that
	// feature's model, not the key: a bauble model the project may not use
	// must not pause the companion. A bare 403 is the key.
	for i, detail := range []string{`{"error":{"message":"Project proj_x does not have access to model gpt-5-nano"}}`,
		`{"error":{"code":"model_not_found"}}`} {
		if ProviderFailure(DecodeChat(403, []byte(detail)).Err) {
			t.Errorf("403 reply %d refuses one model, not the credentials", i)
		}
	}
	if !ProviderFailure(DecodeChat(403, []byte(`{"error":{"message":"Country, region, or territory not supported"}}`)).Err) {
		t.Error("any other 403 is the key or the provider")
	}
	var se *StatusError
	if !errors.As(status(404), &se) || se.Status != 404 || !strings.HasPrefix(se.Error(), `model API status 404`) {
		t.Fatal("DecodeChat's status error keeps the status and the old text")
	}
}

func TestBreakerOpensForItsCooldownOnly(t *testing.T) {
	k := NewBooksForTest()
	now := time.Unix(1000, 0)
	k.Record(ConsumerCompanion, Ticket{}, status(500), now)
	if k.Blocked(ConsumerCompanion, now) || k.BreakerFailures() != 1 {
		t.Fatal("one failure does not open it (BreakerErrors 2)")
	}
	k.Record(ConsumerCompanion, Ticket{}, status(500), now)
	if !k.Blocked(ConsumerCompanion, now) || !k.BreakerOpen(now) {
		t.Fatal("two open it")
	}
	if _, ok := k.Allow(ConsumerCompanion, now.Add(59*time.Second)); ok {
		t.Fatal("nobody gets through while it is open")
	}
	k.Record(ConsumerCompanion, Ticket{}, nil, now)
	if k.BreakerFailures() != 0 {
		t.Fatal("a success resets the run")
	}
	k.ResetBreaker()
	if k.Blocked(ConsumerCompanion, now) {
		t.Fatal("reset closes it")
	}
}

// The analysis's case: the provider does not offer the bauble model (a 400
// or 404 every time). That pauses baubles only; the companion's calls,
// answered fine, go on.
func TestABaubleOnlyFailureNeverBlocksTheCompanion(t *testing.T) {
	k := NewBooksForTest()
	now := time.Unix(1000, 0)
	for i := 0; i < 10; i++ {
		tk, ok := k.Allow(ConsumerBaubles, now)
		if !ok {
			break
		}
		k.Record(ConsumerBaubles, tk, status(404), now)
	}
	if !k.Blocked(ConsumerBaubles, now) {
		t.Fatal("baubles pause themselves")
	}
	if k.BreakerOpen(now) || k.Blocked(ConsumerCompanion, now) || k.BreakerFailures() != 0 {
		t.Fatal("the provider answered: the companion is untouched")
	}
	if _, ok := k.Allow(ConsumerCompanion, now); !ok {
		t.Fatal("the companion is let through")
	}
	// A provider outage, from whichever feature sees it, is everyone's.
	k.Record(ConsumerBaubles, Ticket{}, status(503), now)
	k.Record(ConsumerBaubles, Ticket{}, status(503), now)
	if !k.Blocked(ConsumerCompanion, now) {
		t.Fatal("a provider outage pauses every feature")
	}
}

// Half-open: after the cooldown exactly one caller probes; the rest wait for
// its answer. Success closes it, failure opens it again at once.
func TestHalfOpenLetsOneProbeThrough(t *testing.T) {
	k := NewBooksForTest()
	now := time.Unix(1000, 0)
	k.Record(ConsumerCompanion, Ticket{}, status(500), now)
	k.Record(ConsumerCompanion, Ticket{}, status(500), now)
	later := now.Add(61 * time.Second)

	probe, ok := k.Allow(ConsumerBaubles, later)
	if !ok || !probe.Probing() {
		t.Fatal("the first caller after the cooldown is the probe")
	}
	for _, c := range []string{ConsumerBaubles, ConsumerCompanion} {
		if _, ok := k.Allow(c, later); ok {
			t.Fatalf("%s waits for the probe", c)
		}
		if !k.Blocked(c, later) {
			t.Fatalf("%s is told to wait", c)
		}
	}
	k.Record(ConsumerBaubles, probe, status(502), later)
	if _, ok := k.Allow(ConsumerCompanion, later.Add(59*time.Second)); ok {
		t.Fatal("a failed probe opens it again for the whole cooldown")
	}

	again := later.Add(61 * time.Second)
	probe, ok = k.Allow(ConsumerCompanion, again)
	if !ok || !probe.Probing() {
		t.Fatal("a new probe after the second cooldown")
	}
	k.Record(ConsumerCompanion, probe, nil, again)
	for i := 0; i < 3; i++ {
		if tk, ok := k.Allow(ConsumerCompanion, again); !ok || tk.Probing() {
			t.Fatal("a successful probe closes it: everyone through, no probes")
		}
	}
}

// A probe given back unjudged, or never reported at all, does not leave the
// breaker stuck half-open.
func TestAProbeReleasedOrLostFreesTheWay(t *testing.T) {
	k := NewBooksForTest()
	now := time.Unix(1000, 0)
	k.SetBreakerForTest(0, now)
	probe, ok := k.Allow(ConsumerCompanion, now)
	if !ok || !probe.Probing() {
		t.Fatal("probe")
	}
	k.Release(ConsumerCompanion, probe)
	if probe2, ok := k.Allow(ConsumerCompanion, now); !ok || !probe2.Probing() {
		t.Fatal("a released probe lets the next caller probe")
	}
	// A slow probe (a companion decision with a retry and tool rounds can
	// take over two minutes) is not joined by a second.
	if _, ok := k.Allow(ConsumerCompanion, now.Add(3*time.Minute)); ok {
		t.Fatal("still waiting on the unreported probe")
	}
	if tk, ok := k.Allow(ConsumerCompanion, now.Add(5*time.Minute+time.Second)); !ok || !tk.Probing() {
		t.Fatal("an unreported probe expires and another may probe")
	}
	// Releasing a ticket that is not the probe changes nothing.
	k.Release(ConsumerCompanion, Ticket{})
	if _, ok := k.Allow(ConsumerCompanion, now.Add(5*time.Minute+2*time.Second)); ok {
		t.Fatal("the live probe is still out")
	}
	// Releasing a ticket already recorded changes nothing either.
	k2 := NewBooksForTest()
	k2.SetBreakerForTest(0, now)
	p1, _ := k2.Allow(ConsumerCompanion, now)
	k2.Record(ConsumerCompanion, p1, status(500), now) // reopens
	later := now.Add(61 * time.Second)
	p2, ok := k2.Allow(ConsumerCompanion, later)
	if !ok || !p2.Probing() {
		t.Fatal("a new probe after the cooldown")
	}
	k2.Release(ConsumerCompanion, p1) // the old probe's late release
	if _, ok := k2.Allow(ConsumerCompanion, later); ok {
		t.Fatal("an old ticket's release does not free the new probe")
	}
}

// Keys: an environment variable is read only when one is named (no silent
// default), and no check here ever prints a key.
func TestResolveKeyHasNoSilentDefault(t *testing.T) {
	t.Setenv(`OPENAI_API_KEY`, `sk-must-never-be-read`)
	if ResolveKey(``, ``) != `` {
		t.Fatal("an unnamed variable is never read, not even OPENAI_API_KEY")
	}
	if ResolveKey(``, ` configured `) != `configured` {
		t.Fatal("the configured key, trimmed")
	}
	t.Setenv(`APIFRAMEWORK_TEST_KEY`, ` from-env `)
	if ResolveKey(`APIFRAMEWORK_TEST_KEY`, `configured`) != `from-env` {
		t.Fatal("a named variable wins over the configured key")
	}
	t.Setenv(`APIFRAMEWORK_TEST_KEY`, ``)
	if ResolveKey(`APIFRAMEWORK_TEST_KEY`, `configured`) != `configured` {
		t.Fatal("an empty variable falls back to the configured key")
	}
}

func TestEndpointAllowed(t *testing.T) {
	for _, good := range []string{`https://api.openai.com/v1`, `https://x.openai.azure.com/v1`, `https://API.OpenAI.com/v1`} {
		if !EndpointAllowed(good, false) {
			t.Errorf("%q is OpenAI or Azure OpenAI over https", good)
		}
	}
	// Exactly api.openai.com and *.openai.azure.com (spec S2): any other
	// openai.com or azure.com host is somebody else's server.
	for _, bad := range []string{
		`http://api.openai.com/v1`, `https://evil.example.com/v1`, `notaurl`,
		`https://files.openai.com/v1`, `https://evil.azure.com/v1`, `https://openai.azure.com.evil.example/v1`,
		`https://xopenai.azure.com/v1`, `https://api.openai.com.evil.example/v1`,
		// Azure AI Services hosts need AllowCustomEndpoint (documented).
		`https://x.cognitiveservices.azure.com/v1`, `https://x.services.ai.azure.com/v1`,
	} {
		if EndpointAllowed(bad, false) {
			t.Errorf("%q must be refused without AllowCustomEndpoint", bad)
		}
	}
	if !EndpointAllowed(`https://llm.internal.example/v1`, true) || EndpointAllowed(`http://llm.internal.example/v1`, true) {
		t.Fatal("custom endpoints: https only, and only when allowed")
	}
}

// A server configured before the framework kept its key, endpoint, budget
// and breaker under Modules.aicompanion, and its config.yaml (never updated
// by a patch) has no APIFramework section. Server must read it exactly as
// the companion did (its buildConfig and apiKey before the move), or the
// companion silently loses its key or changes its limits.
func TestServerReadsAnOldConfigAsTheCompanionDid(t *testing.T) {
	none := configs.APIFramework{}
	has := func(s ServerSettings, want string) bool {
		for _, l := range s.Legacy {
			if l == want {
				return true
			}
		}
		return false
	}

	// Nothing set anywhere: the companion always read OPENAI_API_KEY.
	t.Setenv(DefaultKeyEnv, `env-key`)
	s := resolveServer(none, legacyConfig{})
	if s.Endpoint.APIKey != `env-key` || len(s.Legacy) != 0 {
		t.Fatalf("the default variable is read with nothing configured (legacy=%v)", s.Legacy)
	}
	if s.Endpoint.BaseURL != DefaultBaseURL || s.DailyTokenBudget != DefaultDailyTokenBudget ||
		s.BreakerErrors != DefaultBreakerErrors || s.BreakerSeconds != DefaultBreakerSeconds {
		t.Fatalf("the companion's old defaults: base=%q budget=%d breaker=%d/%d",
			s.Endpoint.BaseURL, s.DailyTokenBudget, s.BreakerErrors, s.BreakerSeconds)
	}

	// The variable wins over a key in the file, as it did.
	old := legacyConfig{`APIKey`: `file-key`}
	if s := resolveServer(none, old); s.Endpoint.APIKey != `env-key` {
		t.Fatal("the environment variable wins over the old APIKey")
	}
	t.Setenv(DefaultKeyEnv, ``)
	if s := resolveServer(none, old); s.Endpoint.APIKey != `file-key` || !has(s, `APIKey`) {
		t.Fatalf("with no variable, the old APIKey is used and reported (legacy=%v)", s.Legacy)
	}

	// An old APIKeyEnv naming another variable.
	t.Setenv(`APIFRAMEWORK_TEST_OLD_ENV`, `other-env-key`)
	if s := resolveServer(none, legacyConfig{`APIKeyEnv`: `APIFRAMEWORK_TEST_OLD_ENV`}); s.Endpoint.APIKey != `other-env-key` || !has(s, `APIKeyEnv`) {
		t.Fatal("the old APIKeyEnv names the variable")
	}

	// An old custom endpoint: allowed there, it is used; not allowed, it is
	// refused and the official one used (the companion's
	// TestEndpointMustBeOpenAIOverHTTPS, moved here).
	custom := legacyConfig{`BaseURL`: `https://llm.example.org/v1/`, `AllowCustomEndpoint`: true}
	if s := resolveServer(none, custom); s.Endpoint.BaseURL != `https://llm.example.org/v1` || s.RejectedBaseURL != `` || !has(s, `BaseURL`) {
		t.Fatalf("the old custom endpoint, trailing slash trimmed: %q rejected=%q", s.Endpoint.BaseURL, s.RejectedBaseURL)
	}
	for _, bad := range []string{`https://llm.example.org/v1`, `http://api.openai.com/v1`} {
		s := resolveServer(none, legacyConfig{`BaseURL`: bad})
		if s.Endpoint.BaseURL != DefaultBaseURL || s.RejectedBaseURL != bad {
			t.Errorf("%q is refused without AllowCustomEndpoint and the official endpoint used: %q", bad, s.Endpoint.BaseURL)
		}
	}
	if s := resolveServer(configs.APIFramework{BaseURL: `https://api.openai.com/v1`}, custom); s.Endpoint.BaseURL != DefaultBaseURL {
		t.Fatal("APIFramework.BaseURL wins")
	}
	// Analysis item 5: an explicit APIFramework false overrides an old true
	// (the old custom endpoint is then refused), and an explicit true works
	// on its own.
	if s := resolveServer(configs.APIFramework{AllowCustomEndpoint: `false`}, custom); s.Endpoint.BaseURL != DefaultBaseURL ||
		s.RejectedBaseURL != `https://llm.example.org/v1` || has(s, `AllowCustomEndpoint`) {
		t.Fatalf("explicit false wins over the old true: %q rejected=%q", s.Endpoint.BaseURL, s.RejectedBaseURL)
	}
	if s := resolveServer(configs.APIFramework{AllowCustomEndpoint: `true`, BaseURL: `https://llm.example.org/v1`}, legacyConfig{}); s.Endpoint.BaseURL != `https://llm.example.org/v1` {
		t.Fatal("explicit true allows it")
	}
	if s := resolveServer(none, custom); !has(s, `AllowCustomEndpoint`) {
		t.Fatal("inheriting the old true is reported as legacy")
	}
	for raw, want := range map[string]string{`true`: `true`, ` TRUE `: `true`, `false`: `false`, `yes please`: `false`, ``: ``} {
		c := configs.APIFramework{AllowCustomEndpoint: configs.ConfigString(raw)}
		c.Validate()
		if string(c.AllowCustomEndpoint) != want {
			t.Errorf("%q validates to %q, want %q (anything unreadable is false)", raw, c.AllowCustomEndpoint, want)
		}
	}

	// The budget: the old value, and 0 there was no cap.
	if s := resolveServer(none, legacyConfig{`DailyTokenBudget`: 500}); s.DailyTokenBudget != 500 || !has(s, `DailyTokenBudget`) {
		t.Fatalf("the old budget: %d", s.DailyTokenBudget)
	}
	if s := resolveServer(none, legacyConfig{`DailyTokenBudget`: 0}); s.DailyTokenBudget != 0 {
		t.Fatalf("an old budget of 0 was no cap and still is: %d", s.DailyTokenBudget)
	}
	if s := resolveServer(configs.APIFramework{DailyTokenBudget: 800}, legacyConfig{`DailyTokenBudget`: 0}); s.DailyTokenBudget != 800 {
		t.Fatal("APIFramework.DailyTokenBudget wins")
	}
	if s := resolveServer(configs.APIFramework{DailyTokenBudget: -1}, legacyConfig{}); s.DailyTokenBudget != 0 {
		t.Fatal("a negative APIFramework budget is no cap")
	}

	// The breaker: the old values, with the old floors (0 there was 1 error
	// and 5 seconds).
	if s := resolveServer(none, legacyConfig{`BreakerErrors`: 0, `BreakerSeconds`: 0}); s.BreakerErrors != 1 || s.BreakerSeconds != 5 {
		t.Fatalf("an old breaker of 0 keeps its old floors: %d/%d", s.BreakerErrors, s.BreakerSeconds)
	}
	if s := resolveServer(none, legacyConfig{`BreakerErrors`: uint64(4)}); s.BreakerErrors != 4 {
		t.Fatalf("any integer type is read: %d", s.BreakerErrors)
	}
	if s := resolveServer(none, legacyConfig{`BreakerErrors`: 3, `BreakerSeconds`: 300}); s.BreakerErrors != 3 || s.BreakerSeconds != 300 {
		t.Fatalf("the old breaker: %d/%d", s.BreakerErrors, s.BreakerSeconds)
	}
	if s := resolveServer(configs.APIFramework{BreakerErrors: 7, BreakerSeconds: 2}, legacyConfig{`BreakerErrors`: 3}); s.BreakerErrors != 7 || s.BreakerSeconds != 5 {
		t.Fatalf("APIFramework wins, at least 5 seconds: %d/%d", s.BreakerErrors, s.BreakerSeconds)
	}

	// APIFramework set: nothing is read from the old place.
	t.Setenv(`APIFRAMEWORK_TEST_NEW_ENV`, ``)
	full := configs.APIFramework{APIKeyEnv: `APIFRAMEWORK_TEST_NEW_ENV`, APIKey: `framework-key`, BaseURL: DefaultBaseURL, DailyTokenBudget: 900, BreakerErrors: 4, BreakerSeconds: 40}
	if s := resolveServer(full, legacyConfig{`APIKey`: `file-key`, `DailyTokenBudget`: 5}); s.Endpoint.APIKey != `framework-key` || len(s.Legacy) != 0 || s.DailyTokenBudget != 900 {
		t.Fatalf("APIFramework wins everywhere (legacy=%v budget=%d)", s.Legacy, s.DailyTokenBudget)
	}
}

// End to end through the real config: the old key under
// Modules.aicompanion is found by Server() itself.
func TestServerFallsBackToTheCompanionsOldKey(t *testing.T) {
	prev := settingsOverride.Swap(nil) // read the real config
	t.Cleanup(func() { settingsOverride.Store(prev) })
	set := func(kv map[string]any) {
		t.Helper()
		if err := configs.AddOverlayOverrides(kv); err != nil {
			t.Fatal(err)
		}
	}
	set(map[string]any{`APIFramework.APIKey`: ``, `APIFramework.APIKeyEnv`: ``, `Modules.aicompanion.APIKey`: `legacy-key`})
	t.Cleanup(func() {
		set(map[string]any{`APIFramework.APIKey`: ``, `Modules.aicompanion.APIKey`: ``})
	})
	s := RefreshServer()
	if !s.HasKey() || s.Endpoint.APIKey != `legacy-key` || len(s.Legacy) != 1 {
		t.Fatalf("the old key is used and reported (has=%v legacy=%v)", s.HasKey(), s.Legacy)
	}
	set(map[string]any{`APIFramework.APIKey`: `framework-key`})
	// Server is the snapshot, safe off the game loop: it does not read the
	// config until the next refresh (on the game loop, where it is written).
	if s := Server(); s.Endpoint.APIKey != `legacy-key` {
		t.Fatal("Server returns the snapshot until RefreshServer")
	}
	if s := RefreshServer(); len(s.Legacy) != 0 || s.Endpoint.APIKey != `framework-key` {
		t.Fatalf("APIFramework wins (legacy=%v)", s.Legacy)
	}
	if s := Server(); s.Endpoint.APIKey != `framework-key` {
		t.Fatal("and the refresh is what Server returns")
	}
	snapshot.Store(nil)
}

func sprint(format string, v any) string { return fmt.Sprintf(format, v) }

// A caller's own books are its own: spending and failures there never reach
// the shared set, nor the other way round.
func TestBooksAreIsolated(t *testing.T) {
	ResetBudgetForTest(``)
	ResetBreaker()
	own := NewBooksForTest()
	h, err := own.Reserve(ConsumerCompanion, 300, true)
	if err != nil {
		t.Fatal(err)
	}
	own.Record(ConsumerCompanion, Ticket{}, status(503), time.Now())
	if Today().Tokens != 0 || BreakerFailures() != 0 {
		t.Fatalf("the shared books are untouched: tokens=%d failures=%d", Today().Tokens, BreakerFailures())
	}
	if own.Today().Tokens != 300 || own.BreakerFailures() != 1 {
		t.Fatalf("its own books: tokens=%d failures=%d", own.Today().Tokens, own.BreakerFailures())
	}
	if _, err := Reserve(ConsumerBaubles, 50, true); err != nil {
		t.Fatal(err)
	}
	own.Settle(h, 100, false)
	if own.Today().Tokens != 100 || Today().Tokens != 50 {
		t.Fatalf("each settles alone: own=%d shared=%d", own.Today().Tokens, Today().Tokens)
	}
	if Shared().Today().Tokens != 50 {
		t.Fatal("Shared is what the package functions use")
	}
	ResetBudgetForTest(``)
}

// A provider's error text can quote the key it was sent ("Incorrect API key
// provided: sk-proj-...") and DecodeChat keeps that text in the error, which
// callers log. Anything shaped like an OpenAI key is scrubbed first, so no
// part of one survives, even one the 300-byte cut would have split (spec S2).
func TestDecodeChatScrubsKeysFromErrorText(t *testing.T) {
	cases := map[string]string{
		`quoted`:   `{"error":{"message":"Incorrect API key provided: sk-proj-abc123SECRETwxyz. You can find your API key at https://platform.openai.com"}}`,
		`masked`:   `{"error":{"message":"Incorrect API key provided: sk-proj-****wxyz."}}`,
		`plain`:    `bad key sk-abcdefSECRET0123456789`,
		`boundary`: strings.Repeat(`x`, 290) + ` sk-SECRETSECRETSECRETSECRETSECRET`,
	}
	for name, body := range cases {
		err := DecodeChat(401, []byte(body)).Err
		if err == nil {
			t.Fatalf("%s: a 401 is an error", name)
		}
		got := err.Error()
		if strings.Contains(got, `SECRET`) || strings.Contains(got, `abc123`) || strings.Contains(got, `wxyz`) {
			t.Errorf("%v: key material survived in %d bytes of error text", name, len(got))
		}
		if !strings.Contains(got, `401`) {
			t.Errorf("%s: the status is still reported", name)
		}
	}
	// Ordinary words that merely contain "sk-" are left alone.
	if got := DecodeChat(500, []byte(`task-queue risk-free`)).Err.Error(); !strings.Contains(got, `task-queue risk-free`) {
		t.Errorf("words ending in sk- are not keys, got %d bytes", len(got))
	}
}

// A check that belongs to one feature but is not a model call on the
// server's key (baubles' moderation of text a player's own key wrote)
// counts against that feature's own breaker alone, never the provider's
// the companion shares (owner ruling 2026-09-29).
func TestRecordConsumerFeedsOnlyItsOwnBreaker(t *testing.T) {
	k := NewBooksForTest()
	now := time.Unix(1000, 0)
	k.RecordConsumer(ConsumerBaubles, status(500), now)
	k.RecordConsumer(ConsumerBaubles, status(500), now)
	if !k.Blocked(ConsumerBaubles, now) {
		t.Fatal("two failures open baubles' own breaker (BreakerErrors 2)")
	}
	if k.BreakerOpen(now) || k.BreakerFailures() != 0 || k.Blocked(ConsumerCompanion, now) {
		t.Fatal("a provider-shaped failure (500) still never reaches the provider breaker or the companion")
	}
	k.ResetBreaker()
	k.RecordConsumer(ConsumerBaubles, status(500), now)
	k.RecordConsumer(ConsumerBaubles, nil, now)
	if k.ConsumerFailures(ConsumerBaubles) != 0 {
		t.Fatal("a success resets the run")
	}
}

// Lively features share one permission (PurposeLively) but each lends under
// its own purpose.
func TestLivelyPurposes(t *testing.T) {
	if LivelyPurpose(`npcidle`) != PurposeNPCIdle || !IsLively(PurposeNPCIdle) ||
		LivelyPurpose(`roomlife`) != PurposeRoomLife || !IsLively(PurposeRoomLife) ||
		LivelyPurpose(`lookdetail`) != PurposeLookDetail || !IsLively(PurposeLookDetail) ||
		LivelyPurpose(`rifts`) != PurposeRifts || !IsLively(PurposeRifts) || !IsLively(LivelyPurpose(`crier`)) {
		t.Fatal("a lively feature's purpose is lively")
	}
	for _, p := range []string{PurposeLively, PurposeLively + `:`, PurposeFinds, `livelyx:npcidle`, ``} {
		if IsLively(p) {
			t.Fatalf("%q is not a lively feature's purpose", p)
		}
	}
}
