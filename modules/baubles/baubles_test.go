package baubles

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	eng "github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"gopkg.in/yaml.v2"
)

// The test binary never sees a real key: a developer's OPENAI_API_KEY is
// cleared, the framework's server settings are fixed per test, and no
// failure message prints a key.
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

const goodContent = `{"name":"Painted Wooden Horse","name_simple":"horse","description":"A child's toy horse, its red paint flaking from the mane.","material":"pine","weight_lbs":0.6,"value":12}`

// fakeOpenAI serves /chat/completions with content and /moderations with
// flagged, and records the last chat request body and whether the right
// key came with it (never the key itself).
type fakeOpenAI struct {
	srv            *httptest.Server
	content        string
	status         int
	flagged        bool
	modStatus      int          // when set, the moderation endpoint answers this status
	flagWord       string       // when set, any moderation input containing it is flagged
	lastModeration atomic.Value // the last moderation inputs, newline-joined
	moderations    int32        // moderation calls made, status answered or not
	hang           bool         // chat calls answer only after a second
	chats          int32
	lastBody       atomic.Value
	rightAuth      atomic.Bool
}

func newFakeOpenAI(t *testing.T) *fakeOpenAI {
	f := &fakeOpenAI{content: goodContent, status: 200}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch {
		case strings.HasSuffix(r.URL.Path, `/chat/completions`):
			atomic.AddInt32(&f.chats, 1)
			f.lastBody.Store(string(body))
			f.rightAuth.Store(r.Header.Get(`Authorization`) == `Bearer sk-test`)
			if f.hang {
				time.Sleep(time.Second)
			}
			if f.status != 200 {
				w.WriteHeader(f.status)
				return
			}
			resp := map[string]any{
				`choices`: []any{map[string]any{`finish_reason`: `stop`, `message`: map[string]any{`content`: f.content}}},
				`usage`:   map[string]any{`total_tokens`: 240},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case strings.HasSuffix(r.URL.Path, `/moderations`):
			atomic.AddInt32(&f.moderations, 1)
			if f.modStatus != 0 {
				w.WriteHeader(f.modStatus)
				return
			}
			// One result per input, as the real endpoint answers (and as
			// apiframework.Moderate requires).
			var in struct {
				Input []string `json:"input"`
			}
			_ = json.Unmarshal(body, &in)
			f.lastModeration.Store(strings.Join(in.Input, "\n"))
			results := make([]string, len(in.Input))
			for i, text := range in.Input {
				flag := (i == 0 && f.flagged) || (f.flagWord != `` && strings.Contains(text, f.flagWord))
				results[i] = fmt.Sprintf(`{"flagged":%t}`, flag)
			}
			fmt.Fprintf(w, `{"results":[%s]}`, strings.Join(results, `,`))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// server gives one test the server's key at baseURL (empty key: none), a
// budget and a breaker, all fresh.
func server(t *testing.T, baseURL string, key string, budget int, breakerErrors int) {
	t.Helper()
	restore := apiframework.SetServerForTest(apiframework.ServerSettings{
		Endpoint:         apiframework.Endpoint{BaseURL: baseURL, APIKey: key},
		DailyTokenBudget: budget, BreakerErrors: breakerErrors, BreakerSeconds: 60,
	})
	apiframework.ResetBudgetForTest(``)
	apiframework.ResetBreaker()
	t.Cleanup(func() {
		restore()
		apiframework.ResetBudgetForTest(``)
		apiframework.ResetBreaker()
		apiframework.SetRelay(nil)
	})
}

// testModule is an enabled module whose server key points at the fake.
func testModule(t *testing.T, f *fakeOpenAI, change func(c *Config)) *BaublesModule {
	t.Helper()
	server(t, f.srv.URL, `sk-test`, 2000000, 3)
	cfg := buildConfig(func(k string) any {
		if k == `Enabled` {
			return true
		}
		return nil
	})
	cfg.ModerateOutput = false
	cfg.TimeoutSeconds = 3
	if change != nil {
		change(&cfg)
	}
	m := &BaublesModule{}
	m.configure(cfg)
	t.Cleanup(func() { eng.SetGenerator(nil, nil) })
	return m
}

func request() eng.GenRequest {
	return eng.GenRequest{
		Tier:            eng.TierAverage,
		Source:          eng.SourceSearch,
		Place:           eng.NewPlace(12, `ashwick`, `Windward Marches`, `city`),
		RoomTitle:       `The Toymaker's Back Room`,
		RoomDescription: `Shelves of <ansi fg="red">half-painted</ansi> toys.`,
		RoomNouns:       []string{`shelves`, `toys`},
		TimeOfDay:       `day`,
		RecentNames:     []string{`Tin Soldier`},
		FinderUserId:    7,
	}
}

func baublesTokens() int {
	for _, c := range apiframework.Today().ByConsumer {
		if c.Consumer == apiframework.ConsumerBaubles {
			return c.Tokens
		}
	}
	return 0
}

func TestGenerateOnTheServersKey(t *testing.T) {
	f := newFakeOpenAI(t)
	m := testModule(t, f, nil)

	res, err := m.generate(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	if res.Reply.Name != `Painted Wooden Horse` || res.Generator != eng.GeneratorOpenAI || res.Tokens != 240 || res.PromptVersion != PromptVersion || res.PlayerKey {
		t.Fatalf("result: %+v", res)
	}
	if !f.rightAuth.Load() {
		t.Fatal("the server's key goes in the Authorization header")
	}
	body := f.lastBody.Load().(string)
	if strings.Contains(body, `sk-test`) {
		t.Fatal("the key must never be in the request body")
	}
	for _, want := range []string{`Toymaker`, `half-painted`, `10 to 15`, `Tin Soldier`, `"strict":true`, `weight_lbs`, `"reasoning_effort":"minimal"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("request lacks %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, `<ansi`) {
		t.Fatal("markup is stripped before it reaches the model")
	}
	if baublesTokens() != 240 || apiframework.Today().Tokens != 240 || apiframework.Today().Outstanding != 0 {
		t.Fatalf("settled against the one budget: %+v", apiframework.Today())
	}
	if info, ok := eng.CurrentGenerator(); !ok || info.Name != `openai` || info.Model != `gpt-5-nano` {
		t.Fatalf("installed: %+v %v", info, ok)
	}
}

func TestGenerateFailuresAreErrors(t *testing.T) {
	cases := map[string]func(f *fakeOpenAI){
		`server error`: func(f *fakeOpenAI) { f.status = 500 },
		`auth error`:   func(f *fakeOpenAI) { f.status = 401 },
		`bad json`:     func(f *fakeOpenAI) { f.content = `{"name":"x"` },
		`extra field`:  func(f *fakeOpenAI) { f.content = strings.Replace(goodContent, `}`, `,"x":1}`, 1) },
	}
	for name, change := range cases {
		f := newFakeOpenAI(t)
		change(f)
		m := testModule(t, f, nil)
		if _, err := m.generate(context.Background(), request()); err == nil {
			t.Fatalf("%s: expected an error", name)
		}
	}
}

func TestModerationFailsClosed(t *testing.T) {
	f := newFakeOpenAI(t)
	f.flagged = true
	m := testModule(t, f, func(c *Config) { c.ModerateOutput = true })
	if _, err := m.generate(context.Background(), request()); err == nil {
		t.Fatal("flagged text is refused")
	}

	f.flagged = false
	res, err := m.generate(context.Background(), request())
	if err != nil || !res.Moderated {
		t.Fatalf("clean text passes and is marked moderated: %+v %v", res, err)
	}

	// A check that cannot be made (the moderation endpoint is not there)
	// keeps the text out, exactly as a flag does.
	noMod := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, `/chat/completions`) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				`choices`: []any{map[string]any{`finish_reason`: `stop`, `message`: map[string]any{`content`: goodContent}}},
				`usage`:   map[string]any{`total_tokens`: 240},
			})
			return
		}
		w.WriteHeader(404)
	}))
	t.Cleanup(noMod.Close)
	server(t, noMod.URL, `sk-test`, 2000000, 3)
	if _, err := m.generate(context.Background(), request()); err == nil {
		t.Fatal("a moderation check that could not be made is a refusal")
	}
}

// The budget is the ONE every feature shares: what the companion spends
// leaves less for baubles.
func TestTheBudgetIsShared(t *testing.T) {
	f := newFakeOpenAI(t)
	m := testModule(t, f, nil)
	server(t, f.srv.URL, `sk-test`, 3000, 3)

	if _, err := apiframework.Reserve(apiframework.ConsumerCompanion, 2500, true); err != nil {
		t.Fatal(err)
	}
	if _, err := m.generate(context.Background(), request()); !errors.Is(err, apiframework.ErrOverBudget) {
		t.Fatalf("the companion's spending leaves no room: %v", err)
	}
	if atomic.LoadInt32(&f.chats) != 0 {
		t.Fatal("a refused reservation makes no call")
	}
}

// The server key's breaker is shared too: failures open it for everyone.
func TestTheBreakerIsShared(t *testing.T) {
	f := newFakeOpenAI(t)
	f.status = 500
	m := testModule(t, f, nil)
	server(t, f.srv.URL, `sk-test`, 2000000, 2)
	_, _ = m.generate(context.Background(), request())
	_, _ = m.generate(context.Background(), request())
	before := atomic.LoadInt32(&f.chats)
	if _, err := m.generate(context.Background(), request()); !errors.Is(err, errBreakerOpen) {
		t.Fatalf("breaker: %v", err)
	}
	if atomic.LoadInt32(&f.chats) != before {
		t.Fatal("an open breaker makes no call")
	}
	if !apiframework.BreakerOpen(time.Now()) {
		t.Fatal("it is the framework's breaker, which the companion reads too")
	}
	if !strings.Contains(m.info().Detail, `breaker (every feature's) is open`) {
		t.Fatal("status reports the breaker")
	}
}

// One find is one outcome for the breakers, however many attempts it took:
// a retried failure is one failure, not two (analysis item 2).
func TestOneFindIsOneBreakerOutcome(t *testing.T) {
	f := newFakeOpenAI(t)
	f.status = 500
	m := testModule(t, f, func(c *Config) { c.RetryTransient = true })
	server(t, f.srv.URL, `sk-test`, 2000000, 2)
	if _, err := m.generate(context.Background(), request()); err == nil {
		t.Fatal("it failed")
	}
	if n := atomic.LoadInt32(&f.chats); n != 2 {
		t.Fatalf("retried once: %d calls", n)
	}
	if apiframework.BreakerFailures() != 1 || apiframework.BreakerOpen(time.Now()) {
		t.Fatalf("one failure, not two, and nothing opened: %d", apiframework.BreakerFailures())
	}
	if apiframework.Blocked(apiframework.ConsumerCompanion, time.Now()) {
		t.Fatal("the companion is let through")
	}
}

// A bauble model or request the provider refuses (a 400 or 404, every time)
// pauses baubles, and never the AI companion (analysis item 1).
func TestARefusedBaubleModelNeverPausesTheCompanion(t *testing.T) {
	f := newFakeOpenAI(t)
	f.status = 404
	m := testModule(t, f, nil) // breaker at 3
	for i := 0; i < 6; i++ {
		_, _ = m.generate(context.Background(), request())
	}
	if n := atomic.LoadInt32(&f.chats); n != 3 {
		t.Fatalf("baubles stop calling once their own breaker opens: %d calls", n)
	}
	if !apiframework.Blocked(apiframework.ConsumerBaubles, time.Now()) {
		t.Fatal("baubles are paused")
	}
	if apiframework.BreakerOpen(time.Now()) || apiframework.Blocked(apiframework.ConsumerCompanion, time.Now()) {
		t.Fatal("the provider answered every time: the companion is untouched")
	}
	if tk, ok := apiframework.Allow(apiframework.ConsumerCompanion, time.Now()); !ok {
		t.Fatal("a companion call is let through")
	} else {
		apiframework.Release(apiframework.ConsumerCompanion, tk)
	}
	if !strings.Contains(m.info().Detail, `naming breaker is open`) {
		t.Fatal("status says it is baubles' own naming breaker")
	}
}

// Moderation policy (spec S3; owner ruling 2026-09-29): a flag always keeps
// a find out, and a check that is made and fails keeps out a find on EITHER
// key. Every failed check is held against the moderation breaker alone,
// never the naming breaker and never the provider's the companion shares.
func TestModerationOutageRefusesAPlayerKeyFind(t *testing.T) {
	f := newFakeOpenAI(t)
	f.modStatus = 500
	m := testModule(t, f, func(c *Config) { c.ModerateOutput = true })
	if _, err := m.generate(context.Background(), request()); err == nil {
		t.Fatal("the server's key: no check, no name")
	}
	relay := &fakeRelay{allowed: map[int]bool{7: true}, model: `player-model`, provider: f}
	apiframework.SetRelay(relay)
	if _, err := m.generate(context.Background(), request()); err == nil || relay.sends != 1 {
		t.Fatalf("the finder's key named it, the check failed: refused (sends=%d)", relay.sends)
	}
	// Both failed checks, the server-key one and the player-key one, are on
	// the moderation breaker; the naming breaker and the provider's hold none.
	if n := apiframework.Shared().ConsumerFailures(moderationBreaker); n != 2 ||
		apiframework.Shared().ConsumerFailures(apiframework.ConsumerBaubles) != 0 || apiframework.BreakerFailures() != 0 {
		t.Fatalf("the moderation breaker holds both failed checks (%d), the naming breaker and the provider's none", n)
	}
	f.modStatus, f.flagged = 0, true
	if _, err := m.generate(context.Background(), request()); err == nil {
		t.Fatal("a flag keeps a player-key find out")
	}
	f.flagged = false
	if res, err := m.generate(context.Background(), request()); err != nil || !res.PlayerKey || !res.Moderated || res.FinderOnly {
		t.Fatalf("a clean check: named on the finder's key, moderated, everyone's: %+v %v", res, err)
	}
}

// The moderation breaker is read exactly once per find (H2 Task 10 review):
// moderationPossible's own read and moderate's later, separate read used to
// be two live reads of the same breaker a few statements apart. A breaker
// that tripped open in that gap (another find's failed check, on another
// goroutine) made the SECOND read see it open when the FIRST, which gated
// whether a player-key find is even attempted, had already decided it was
// closed: the find was then refused outright (errBreakerOpen) rather than
// honouring the decision moderationPossible had already made for it. moderate
// now takes one read (blocked) and never re-reads apiframework.Blocked, so a
// later change to the breaker cannot reach a decision already taken.
func TestModerationBreakerIsReadOnceNotReRead(t *testing.T) {
	f := newFakeOpenAI(t)
	m := testModule(t, f, func(c *Config) { c.ModerateOutput = true })
	relay := &fakeRelay{allowed: map[int]bool{7: true}, model: `player-model`, provider: f}
	apiframework.SetRelay(relay)

	t.Cleanup(func() { moderateReadForTest = func() {} })
	// Lands right where the old code's second, redundant read ran: the
	// breaker was closed for moderate's one true read (moderationPossible
	// said the find could be attempted) and only opens here.
	moderateReadForTest = func() {
		apiframework.Shared().SetConsumerBreakerForTest(moderationBreaker, 5, time.Now().Add(time.Minute))
	}

	res, err := m.generate(context.Background(), request())
	if errors.Is(err, errBreakerOpen) {
		t.Fatalf("the breaker opening after the one read must not refuse a find that read already allowed: %+v %v", res, err)
	}
	if err != nil || !res.PlayerKey || !res.Moderated || res.FinderOnly {
		t.Fatalf("the read at decision time was closed, so the find is named and moderated as usual: %+v %v", res, err)
	}
}

// A find that runs out of time is the provider not answering: it counts
// (analysis: a timeout released unjudged would never open a breaker). A
// find given up on (a copyover's flush) is nobody's failure.
func TestATimeoutCountsACancelDoesNot(t *testing.T) {
	f := newFakeOpenAI(t)
	f.hang = true
	m := testModule(t, f, nil) // breaker at 3
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		_, _ = m.generate(ctx, request())
		cancel()
	}
	if !apiframework.BreakerOpen(time.Now()) {
		t.Fatalf("three timeouts open the provider breaker: %d", apiframework.BreakerFailures())
	}

	server(t, f.srv.URL, `sk-test`, 2000000, 3)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	_, _ = m.generate(ctx, request())
	if apiframework.BreakerFailures() != 0 || apiframework.Shared().ConsumerFailures(apiframework.ConsumerBaubles) != 0 {
		t.Fatal("a cancelled find counts against nothing")
	}
}

// A model that keeps answering with something that is not a usable bauble
// is paused by baubles' own breaker, and never by the provider's: it
// answered (analysis: unusable replies were never counted at all).
func TestUnusableRepliesPauseBaublesOnly(t *testing.T) {
	f := newFakeOpenAI(t)
	f.content = `{"name":"x"}` // parses as JSON, is no bauble
	m := testModule(t, f, nil)
	for i := 0; i < 5; i++ {
		_, _ = m.generate(context.Background(), request())
	}
	if n := atomic.LoadInt32(&f.chats); n != 3 {
		t.Fatalf("asked until baubles' own breaker opened: %d calls", n)
	}
	if apiframework.BreakerOpen(time.Now()) || apiframework.Blocked(apiframework.ConsumerCompanion, time.Now()) {
		t.Fatal("the provider answered; the companion is untouched")
	}
}

func TestBusySlotsAreRefusedNotQueued(t *testing.T) {
	f := newFakeOpenAI(t)
	m := testModule(t, f, func(c *Config) { c.MaxConcurrent = 1 })
	m.slots <- struct{}{} // the one slot is taken
	start := time.Now()
	if _, err := m.generate(context.Background(), request()); err == nil || time.Since(start) > time.Second {
		t.Fatal("a find beyond the slots is refused at once")
	}
}

func TestConfigureInstallsOnlyWhenEnabled(t *testing.T) {
	server(t, apiframework.DefaultBaseURL, ``, 2000000, 3)
	m := &BaublesModule{}
	m.configure(buildConfig(nil))
	if _, ok := eng.CurrentGenerator(); ok {
		t.Fatal("off by default: generic trinkets")
	}
	on := buildConfig(func(k string) any {
		if k == `Enabled` {
			return true
		}
		return nil
	})
	m.configure(on)
	if _, ok := eng.CurrentGenerator(); !ok {
		t.Fatal("on: installed, even with no server key (a finder may bring their own)")
	}
	eng.SetGenerator(nil, nil)
}

func TestBuildConfigDefaultsAndBounds(t *testing.T) {
	c := buildConfig(nil)
	if c.Enabled || !c.UsePlayerKeys || c.Model != `gpt-5-nano` || c.ReasoningEffort != `minimal` || c.TimeoutSeconds != 15 ||
		!c.ModerateOutput || c.MaxConcurrent != 4 || c.MaxCompletionTokens != 800 {
		t.Fatalf("defaults: %+v", c)
	}
	if c.DailyTokensPerUser != 20000 {
		t.Fatalf("default DailyTokensPerUser: %d", c.DailyTokensPerUser)
	}
	if buildConfig(func(k string) any {
		if k == `DailyTokensPerUser` {
			return -5
		}
		return nil
	}).DailyTokensPerUser != 0 {
		t.Fatal("a negative allowance is no cap")
	}
	c = buildConfig(func(k string) any {
		switch k {
		case `TimeoutSeconds`:
			return 999
		case `ReasoningEffort`:
			return `whatever`
		case `UsePlayerKeys`:
			return `false`
		}
		return nil
	})
	if c.TimeoutSeconds != 30 || c.ReasoningEffort != `` || c.UsePlayerKeys {
		t.Fatalf("bounds: %+v", c)
	}
}

// fakeRelay is a finder's own key: it answers through its own fake
// provider, and records what it was sent.
type fakeRelay struct {
	allowed  map[int]bool
	model    string
	fail     bool
	reply    string // when set, the provider "answers" this, status 200
	carries  apiframework.Carries
	body     string
	sends    int
	results  []error
	provider *fakeOpenAI
}

func (r *fakeRelay) Model(userId int, purpose string) (string, bool) {
	if purpose != apiframework.PurposeFinds || !r.allowed[userId] {
		return ``, false
	}
	return r.model, true
}

func (r *fakeRelay) Send(ctx context.Context, userId int, body []byte, carries apiframework.Carries) (int, []byte, bool, error) {
	r.sends++
	r.carries = carries
	r.body = string(body)
	if r.fail {
		return 0, nil, false, errors.New(`relay went away`)
	}
	if r.reply != `` {
		return http.StatusOK, []byte(r.reply), true, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, r.provider.srv.URL+`/chat/completions`, bytes.NewReader(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw, true, nil
}

func (r *fakeRelay) Result(userId int, purpose string, err error) {
	if purpose != apiframework.PurposeFinds {
		panic(`baubles reported on a purpose other than finds: ` + purpose)
	}
	r.results = append(r.results, err)
}

func TestFindersOwnKeyNamesTheirFind(t *testing.T) {
	serverSide := newFakeOpenAI(t)
	m := testModule(t, serverSide, func(c *Config) { c.ModerateOutput = true })
	player := newFakeOpenAI(t)
	relay := &fakeRelay{allowed: map[int]bool{7: true}, model: `player-model`, provider: player}
	apiframework.SetRelay(relay)

	res, err := m.generate(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	if !res.PlayerKey || !res.Moderated || res.Model != `player-model` || relay.sends != 1 || atomic.LoadInt32(&serverSide.chats) != 0 {
		t.Fatalf("named on the finder's key alone: %+v sends=%d server=%d", res, relay.sends, serverSide.chats)
	}
	if relay.carries != apiframework.CarriesNoPlayerData {
		t.Fatal("a bauble request carries no player data")
	}
	if !strings.Contains(relay.body, `"model":"player-model"`) || strings.Contains(relay.body, `reasoning_effort`) {
		t.Fatalf("the player's model, and no server-only settings: %s", relay.body)
	}
	if apiframework.Today().Tokens != 0 {
		t.Fatal("a player's own key spends nothing of the server's budget")
	}
	if len(relay.results) != 1 || relay.results[0] != nil {
		t.Fatalf("the player's own breaker is fed: %v", relay.results)
	}
}

func TestFindersKeyOnlyWhenAllowed(t *testing.T) {
	serverSide := newFakeOpenAI(t)
	m := testModule(t, serverSide, func(c *Config) { c.ModerateOutput = true })
	relay := &fakeRelay{allowed: map[int]bool{8: true}, model: `player-model`, provider: newFakeOpenAI(t)}
	apiframework.SetRelay(relay)

	res, err := m.generate(context.Background(), request()) // the finder is 7
	if err != nil || res.PlayerKey || relay.sends != 0 || atomic.LoadInt32(&serverSide.chats) != 1 {
		t.Fatalf("not allowed: the server's key: %+v %v sends=%d", res, err, relay.sends)
	}

	m2 := testModule(t, serverSide, func(c *Config) { c.UsePlayerKeys, c.ModerateOutput = false, true })
	relay.allowed[7] = true
	apiframework.SetRelay(relay)
	if res, _ := m2.generate(context.Background(), request()); res.PlayerKey || relay.sends != 0 {
		t.Fatal("UsePlayerKeys false: never a player's key")
	}
}

func TestFindersKeyFailingFallsBackToTheServer(t *testing.T) {
	serverSide := newFakeOpenAI(t)
	m := testModule(t, serverSide, func(c *Config) { c.ModerateOutput = true })
	relay := &fakeRelay{allowed: map[int]bool{7: true}, model: `player-model`, fail: true, provider: newFakeOpenAI(t)}
	apiframework.SetRelay(relay)

	res, err := m.generate(context.Background(), request())
	if err != nil || res.PlayerKey || atomic.LoadInt32(&serverSide.chats) != 1 {
		t.Fatalf("the server's key after the finder's failed: %+v %v", res, err)
	}
	if len(relay.results) != 1 || relay.results[0] == nil {
		t.Fatal("the failure reaches the player's own breaker")
	}
}

// A reply that answers but cannot be used is reported against the finder's
// key for finds (the relay keeps that breaker apart from their companion's:
// aicompanion's TestFindsFailuresNeverPauseTheCompanion), and the find
// falls back to the server's key.
func TestFindersUnusableReplyIsReportedForFindsOnly(t *testing.T) {
	serverSide := newFakeOpenAI(t)
	m := testModule(t, serverSide, func(c *Config) { c.ModerateOutput = true })
	relay := &fakeRelay{allowed: map[int]bool{7: true}, model: `player-model`, reply: `{"choices":[]}`, provider: newFakeOpenAI(t)}
	apiframework.SetRelay(relay)

	res, err := m.generate(context.Background(), request())
	if err != nil || res.PlayerKey || atomic.LoadInt32(&serverSide.chats) != 1 {
		t.Fatalf("the server's key after an unusable reply: %+v %v", res, err)
	}
	if len(relay.results) != 1 || relay.results[0] == nil {
		t.Fatalf("the unusable reply is reported: %v", relay.results)
	}
}

func TestNoKeyAtAll(t *testing.T) {
	f := newFakeOpenAI(t)
	m := testModule(t, f, func(c *Config) { c.ModerateOutput = true })
	server(t, f.srv.URL, ``, 2000000, 3) // no server key
	if _, err := m.generate(context.Background(), request()); !errors.Is(err, errNoRoute) {
		t.Fatalf("no key anywhere: a generic trinket: %v", err)
	}
	// With no server key nothing can moderate a finder's own key's text, so
	// it is named there and kept to its finder (owner ruling 2026-09-29):
	// FinderOnly, not Moderated, and no moderation call is made.
	relay := &fakeRelay{allowed: map[int]bool{7: true}, model: `player-model`, provider: f}
	apiframework.SetRelay(relay)
	res, err := m.generate(context.Background(), request())
	if err != nil || !res.PlayerKey || res.Moderated || !res.FinderOnly || relay.sends != 1 {
		t.Fatalf("finder's key, kept to the finder: %+v %v sends=%d", res, err, relay.sends)
	}
	if got := f.lastModeration.Load(); got != nil {
		t.Fatalf("nothing was sent to moderation: %v", got)
	}
}

func TestPromptKeepsPlayerTextOut(t *testing.T) {
	req := request()
	msgs := buildMessages(req)
	if len(msgs) != 2 || msgs[0].Role != `system` || msgs[1].Role != `user` {
		t.Fatalf("messages: %+v", msgs)
	}
	if !strings.Contains(msgs[0].Content, `Gaius`) || !strings.Contains(msgs[0].Content, eng.WeightGuidance) {
		t.Fatal("system prompt carries the world and the weight scale")
	}
	if !strings.Contains(msgs[0].Content, `NATURAL find`) || !strings.Contains(msgs[0].Content, `only nature`) {
		t.Fatal("system prompt allows natural finds in wild places (prompt version 3)")
	}
	if !strings.Contains(msgs[1].Content, `found while searching`) || !strings.Contains(msgs[1].Content, `the place itself`) {
		t.Fatalf("user message: %s", msgs[1].Content)
	}
	for _, m := range msgs {
		if strings.Contains(m.Content, `finder`) || strings.Contains(m.Content, `"7"`) {
			t.Fatal("who found it is never sent to the model")
		}
	}
	req.Container = `old sea chest`
	req.Source = eng.SourceBurglary
	if u := buildMessages(req)[1].Content; !strings.Contains(u, `old sea chest`) || !strings.Contains(u, `inside someone's home`) {
		t.Fatalf("container and source: %s", u)
	}
}

// A targeted search (`search hearth`) sends what was searched and its
// authored description, markup stripped and capped; a room search sends no
// description field at all.
func TestPromptCarriesTheSearchedFeature(t *testing.T) {
	req := request()
	if u := buildMessages(req)[1].Content; strings.Contains(u, `searched_description`) {
		t.Fatalf("no feature, no description: %s", u)
	}
	req.Container = `hearth`
	req.ContainerDescription = `A wide stone <ansi fg="red">hearth</ansi>, cold and full of old ash.`
	u := buildMessages(req)[1].Content
	if !strings.Contains(u, `"searched":"hearth"`) || !strings.Contains(u, `full of old ash`) || strings.Contains(u, `ansi`) {
		t.Fatalf("feature and description: %s", u)
	}

	long := strings.Repeat(`é`, maxSearchedDescription) // two bytes each
	got := truncate(long, maxSearchedDescription)
	if len(got) > maxSearchedDescription || !utf8.ValidString(got) {
		t.Fatalf("truncate: %d bytes, valid %v", len(got), utf8.ValidString(got))
	}
}

func TestPromptPreviewIsInstalledEvenWhenOff(t *testing.T) {
	m := &BaublesModule{}
	m.configure(buildConfig(nil))
	t.Cleanup(func() { eng.SetPromptPreview(nil) })
	msgs, ok := eng.PreviewPrompt(request())
	if !ok || len(msgs) != 2 || !strings.HasPrefix(msgs[0], `system: `) || !strings.Contains(msgs[1], `Toymaker`) {
		t.Fatalf("preview: %v %v", ok, msgs)
	}
}

// A pickpocketed find is asked for as something lifted from that person
// (taken_from: the NPC's authored name) and pocket-sized (size_rule, with
// the weight limit the catalog enforces); a search find carries neither.
// The thief is never named.
func TestPickpocketPromptIsPocketSized(t *testing.T) {
	req := request()
	req.Source, req.Victim = eng.SourcePickpocket, `a harried <b>clerk</b>`
	user := buildMessages(req)[1].Content
	for _, want := range []string{`"taken_from":"a harried clerk"`, `"size_rule":"pocket-sized`, `weight_lbs at most 1.0`, `lifted from someone's pocket`} {
		if !strings.Contains(user, want) {
			t.Errorf("pickpocket prompt lacks %s:\n%s", want, user)
		}
	}
	if !strings.Contains(buildMessages(req)[0].Content, `POCKET-SIZED`) {
		t.Error("the system prompt states the pocket rule")
	}
	if strings.Contains(user, `"7"`) || strings.Contains(user, `finder`) {
		t.Error("the finder is never in the prompt")
	}
	plain := buildMessages(request())[1].Content
	if strings.Contains(plain, `taken_from`) || strings.Contains(plain, `size_rule`) {
		t.Errorf("a search find carries neither: %s", plain)
	}
}

// A pickpocket's find follows the one key order: the thief's own key when
// they allowed finds, else the server's.
func TestPickpocketUsesTheThiefsKeyFirst(t *testing.T) {
	serverSide := newFakeOpenAI(t)
	m := testModule(t, serverSide, func(c *Config) { c.ModerateOutput = true })
	relay := &fakeRelay{allowed: map[int]bool{7: true}, model: `player-model`, provider: newFakeOpenAI(t)}
	apiframework.SetRelay(relay)
	req := request()
	req.Source, req.Victim = eng.SourcePickpocket, `a harried clerk`
	res, err := m.generate(context.Background(), req)
	if err != nil || !res.PlayerKey || relay.sends != 1 || atomic.LoadInt32(&serverSide.chats) != 0 {
		t.Fatalf("the thief's key first: %+v %v sends=%d server=%d", res, err, relay.sends, serverSide.chats)
	}
	if relay.carries != apiframework.CarriesNoPlayerData || strings.Contains(relay.body, `"7"`) {
		t.Fatalf("the request carries nothing of the thief's: %q", relay.body)
	}

	// A thief who did not allow finds is named on the server's key.
	relay.allowed = map[int]bool{}
	res, err = m.generate(context.Background(), req)
	if err != nil || res.PlayerKey || relay.sends != 1 || atomic.LoadInt32(&serverSide.chats) != 1 {
		t.Fatalf("no consent, the server's key: %+v %v sends=%d", res, err, relay.sends)
	}
}

// Where the server cannot moderate (no server key, ModerateOutput off, the
// provider breaker or the moderation breaker open), a finder's own key still names
// the find, and its text is kept to that finder (owner ruling 2026-09-29):
// FinderOnly, not Moderated, and no moderation call is made.
func TestPlayerKeyTextThatCannotBeModeratedIsFinderOnly(t *testing.T) {
	cases := map[string]func(t *testing.T, f *fakeOpenAI) *BaublesModule{
		`no server key`: func(t *testing.T, f *fakeOpenAI) *BaublesModule {
			m := testModule(t, f, func(c *Config) { c.ModerateOutput = true })
			server(t, f.srv.URL, ``, 2000000, 3)
			return m
		},
		`moderation off`: func(t *testing.T, f *fakeOpenAI) *BaublesModule {
			return testModule(t, f, nil)
		},
		`provider breaker open`: func(t *testing.T, f *fakeOpenAI) *BaublesModule {
			m := testModule(t, f, func(c *Config) { c.ModerateOutput = true })
			apiframework.SetBreakerForTest(5, time.Now().Add(time.Minute))
			return m
		},
		`moderation breaker open`: func(t *testing.T, f *fakeOpenAI) *BaublesModule {
			m := testModule(t, f, func(c *Config) { c.ModerateOutput = true })
			apiframework.Shared().SetConsumerBreakerForTest(moderationBreaker, 0, time.Now().Add(time.Minute))
			return m
		},
	}
	for name, build := range cases {
		f := newFakeOpenAI(t)
		m := build(t, f)
		relay := &fakeRelay{allowed: map[int]bool{7: true}, model: `player-model`, provider: newFakeOpenAI(t)}
		apiframework.SetRelay(relay)
		res, err := m.generate(context.Background(), request())
		if err != nil || !res.PlayerKey || res.Moderated || !res.FinderOnly || relay.sends != 1 {
			t.Errorf("%v: named on the finder's key, kept to the finder: %+v %v sends=%d", name, res, err, relay.sends)
		}
		if got := f.lastModeration.Load(); got != nil {
			t.Errorf("%v: no moderation call is made: %v", name, got)
		}
	}
}

// The moderation breaker is its own (review finding d): a server-key find
// whose naming succeeds does not reset a run of failed checks, so checks
// failing on both routes open it, and then a finder's own key's text is
// kept to the finder with no check tried, while the naming breaker, which
// saw only successes, stays closed. Sequence: server find, player find,
// server find, each with the check failing (BreakerErrors 3).
func TestFailedChecksOpenTheModerationBreakerAlone(t *testing.T) {
	f := newFakeOpenAI(t)
	f.modStatus = 500
	m := testModule(t, f, func(c *Config) { c.ModerateOutput = true })
	relay := &fakeRelay{allowed: map[int]bool{}, model: `player-model`, provider: newFakeOpenAI(t)}
	apiframework.SetRelay(relay)

	for i, playerKey := range []bool{false, true, false} {
		relay.allowed[7] = playerKey
		if _, err := m.generate(context.Background(), request()); err == nil {
			t.Fatalf("find %d: the check failed, so it is refused", i)
		}
	}
	if !apiframework.Blocked(moderationBreaker, time.Now()) {
		t.Fatal("three failed checks in a row, whatever the route, open the moderation breaker")
	}
	if apiframework.Blocked(apiframework.ConsumerBaubles, time.Now()) || apiframework.BreakerOpen(time.Now()) {
		t.Fatal("every naming call succeeded: the naming and provider breakers stay closed")
	}

	f.modStatus = 0
	checks := atomic.LoadInt32(&f.moderations)
	relay.allowed[7] = true
	res, err := m.generate(context.Background(), request())
	if err != nil || !res.FinderOnly || res.Moderated || atomic.LoadInt32(&f.moderations) != checks {
		t.Fatalf("with the breaker open, the finder's text is kept to them, unchecked: %+v %v", res, err)
	}
	relay.allowed[7] = false
	if _, err := m.generate(context.Background(), request()); !errors.Is(err, errBreakerOpen) {
		t.Fatalf("and a server-key find is refused, unchecked: %v", err)
	}
}

// Player-key text outside the allowlist (ruling 15) is not the finder's key
// failing: their finds breaker hears nothing, and the find goes on to the
// server's key, which names it.
func TestPlayerKeyTextOutsideTheAllowlistFallsBackToTheServer(t *testing.T) {
	serverSide := newFakeOpenAI(t)
	m := testModule(t, serverSide, func(c *Config) { c.ModerateOutput = true })
	odd := strings.Replace(goodContent, `Painted Wooden Horse`, "Painted Wooden H\xc3\xb6rse", 1) // o with diaeresis, U+00F6
	relay := &fakeRelay{allowed: map[int]bool{7: true}, model: `player-model`, reply: chatBody(odd, 240), provider: newFakeOpenAI(t)}
	apiframework.SetRelay(relay)
	res, err := m.generate(context.Background(), request())
	if err != nil || res.PlayerKey || res.Reply.Name != `Painted Wooden Horse` || relay.sends != 1 || atomic.LoadInt32(&serverSide.chats) != 1 {
		t.Fatalf("refused on the finder's key, named on the server's: %+v %v sends=%d server=%d", res, err, relay.sends, serverSide.chats)
	}
	if len(relay.results) != 0 {
		t.Fatalf("an allowlist refusal is not the finder's key failing: %v", relay.results)
	}
}

// Curly quotes and dashes from a finder's own key are folded to ASCII
// before the allowlist looks (ruling 15), so the find keeps its route.
func TestPlayerKeyTypographyIsFoldedNotRefused(t *testing.T) {
	serverSide := newFakeOpenAI(t)
	m := testModule(t, serverSide, func(c *Config) { c.ModerateOutput = true })
	curly := strings.Replace(goodContent, `A child's toy horse, its red`, "A child\xe2\x80\x99s toy horse \xe2\x80\x94 its red", 1) // U+2019, U+2014
	relay := &fakeRelay{allowed: map[int]bool{7: true}, model: `player-model`, reply: chatBody(curly, 240), provider: newFakeOpenAI(t)}
	apiframework.SetRelay(relay)
	res, err := m.generate(context.Background(), request())
	if err != nil || !res.PlayerKey || atomic.LoadInt32(&serverSide.chats) != 0 {
		t.Fatalf("named on the finder's key: %+v %v server=%d", res, err, serverSide.chats)
	}
	if want := `A child's toy horse - its red paint flaking from the mane.`; res.Reply.Description != want {
		t.Fatalf("the cleaned, folded text is what is kept:\n got %v\nwant %v", res.Reply.Description, want)
	}
}

// Every text field is moderated, on every route (spec S3, ruling 15): the
// name, the keyword players type, the description, and the material that
// appraise shows.
func TestEveryTextFieldIsModerated(t *testing.T) {
	f := newFakeOpenAI(t)
	f.flagWord = `pine`
	m := testModule(t, f, func(c *Config) { c.ModerateOutput = true })
	if _, err := m.generate(context.Background(), request()); err == nil {
		t.Fatal("a flagged material refuses the find")
	}
	got, _ := f.lastModeration.Load().(string)
	want := "Painted Wooden Horse\nhorse\nA child's toy horse, its red paint flaking from the mane.\npine"
	if got != want {
		t.Fatalf("name, keyword, description and material, in that order:\n got %v\nwant %v", got, want)
	}
}

// The system prompt states the characters the player-key allowlist accepts
// (ruling 15), so a model on either key is asked for text that passes.
func TestSystemPromptStatesTheAllowedCharacters(t *testing.T) {
	if PromptVersion < 5 {
		t.Fatalf("the prompt changed: PromptVersion must be at least 5, is %d", PromptVersion)
	}
	for _, want := range []string{`plain ASCII`, `' " - , . ! ?`, `no colons`} {
		if !strings.Contains(systemPrompt, want) {
			t.Errorf("the system prompt does not say %v", want)
		}
	}
}

// A token count relayed through a player's browser is theirs to write: it
// is held to what one request could cost before it reaches the record or
// the statistics (spec S3).
func TestARelayedTokenCountIsClamped(t *testing.T) {
	serverSide := newFakeOpenAI(t)
	m := testModule(t, serverSide, func(c *Config) { c.ModerateOutput = true })
	relay := &fakeRelay{allowed: map[int]bool{7: true}, model: `player-model`, reply: chatBody(goodContent, 999999), provider: newFakeOpenAI(t)}
	apiframework.SetRelay(relay)
	res, err := m.generate(context.Background(), request())
	if err != nil || !res.PlayerKey {
		t.Fatalf("named on the finder's key: %+v %v", res, err)
	}
	most := apiframework.EstimateTokens(buildMessages(request())) + schemaOverhead + m.snapshot().MaxCompletionTokens
	if res.Tokens != most {
		t.Fatalf("clamped to the most one request costs (%d), got %d", most, res.Tokens)
	}
}

// A relay call takes the finder's own slot (one in flight per finder), not
// one of the server's shared slots (spec S3).
func TestAFindersOwnKeyTakesTheirOwnSlot(t *testing.T) {
	serverSide := newFakeOpenAI(t)
	m := testModule(t, serverSide, func(c *Config) { c.MaxConcurrent, c.ModerateOutput = 1, true })
	relay := &fakeRelay{allowed: map[int]bool{7: true}, model: `player-model`, provider: newFakeOpenAI(t)}
	apiframework.SetRelay(relay)

	m.slots <- struct{}{} // every server slot is busy
	res, err := m.generate(context.Background(), request())
	<-m.slots
	if err != nil || !res.PlayerKey || relay.sends != 1 {
		t.Fatalf("the finder's key names it with the server's slots full: %+v %v sends=%d", res, err, relay.sends)
	}

	release, ok := m.takeFinderSlot(7)
	if !ok {
		t.Fatal("the finder's slot is free again")
	}
	res, err = m.generate(context.Background(), request())
	release()
	if err != nil || res.PlayerKey || relay.sends != 1 || atomic.LoadInt32(&serverSide.chats) != 1 {
		t.Fatalf("with their own call in flight, the server's key names it: %+v %v sends=%d", res, err, relay.sends)
	}
	if _, ok := m.takeFinderSlot(7); !ok {
		t.Fatal("released")
	}
}

// chatBody is a provider's chat completions answer with this content.
func chatBody(content string, tokens int) string {
	b, _ := json.Marshal(map[string]any{
		`choices`: []any{map[string]any{`finish_reason`: `stop`, `message`: map[string]any{`content`: content}}},
		`usage`:   map[string]any{`total_tokens`: tokens},
	})
	return string(b)
}

func finderSpent(id int) int { return apiframework.Allowance(apiframework.DimBaublesFinder, id) }

// A find is charged to its finder, on the server's key and on their own.
// Moderation is on, as in slice H's player-key tests, so the finder's find
// is moderated and everyone's (it is charged the same either way).
func TestAFindIsChargedToItsFinder(t *testing.T) {
	f := newFakeOpenAI(t)
	m := testModule(t, f, func(c *Config) { c.ModerateOutput = true })
	if _, err := m.generate(context.Background(), request()); err != nil {
		t.Fatal(err)
	}
	if finderSpent(7) != 240 {
		t.Fatalf("the server's key: the finder is charged what it cost: %d", finderSpent(7))
	}
	relay := &fakeRelay{allowed: map[int]bool{7: true}, model: `player-model`, provider: newFakeOpenAI(t)}
	apiframework.SetRelay(relay)
	before := apiframework.Today().Tokens
	if res, err := m.generate(context.Background(), request()); err != nil || !res.PlayerKey {
		t.Fatalf("fixture: named on the finder's key: %+v %v", res, err)
	}
	if finderSpent(7) != 480 || apiframework.Today().Tokens != before {
		t.Fatalf("their own key: charged to them, nothing of the server's: finder=%d server %d->%d",
			finderSpent(7), before, apiframework.Today().Tokens)
	}
}

// Over their allowance, a finder's find is a generic trinket: no call on
// either key, and the relay's breaker is not fed.
func TestAFinderOverTheirAllowanceGetsNoName(t *testing.T) {
	f := newFakeOpenAI(t)
	// The relay route opens with or without moderation (slice H keeps text
	// it cannot moderate to its finder): only the allowance stops it. Its
	// refusal falls through to the server's key, which the same allowance
	// refuses too.
	m := testModule(t, f, nil)
	relay := &fakeRelay{allowed: map[int]bool{7: true}, model: `player-model`, provider: newFakeOpenAI(t)}
	apiframework.SetRelay(relay)
	apiframework.Shared().SetAllowanceForTest(apiframework.DimBaublesFinder, 7, m.snapshot().DailyTokensPerUser)
	for i := 0; i < 4; i++ { // more refusals in a row than testModule's BreakerErrors (3)
		_, err := m.generate(context.Background(), request())
		if !errors.Is(err, apiframework.ErrOverAllowance) || apiframework.RefusedBy(err) != apiframework.DimBaublesFinder {
			t.Fatalf("over the allowance, and it says whose: %v", err)
		}
	}
	if relay.sends != 0 || atomic.LoadInt32(&f.chats) != 0 || len(relay.results) != 0 {
		t.Fatalf("no call anywhere, the relay's breaker unfed: sends=%d chats=%d results=%v", relay.sends, f.chats, relay.results)
	}
	if apiframework.Blocked(apiframework.ConsumerBaubles, time.Now()) {
		t.Fatal("a refusal feeds no breaker of the server's")
	}
	m.mu.Lock()
	server, player, failures := m.stats.server, m.stats.player, m.stats.failures
	m.mu.Unlock()
	if server != 0 || player != 0 || failures != 0 {
		t.Fatalf("a refusal is no naming and no failure in bauble status: server=%d player=%d failed=%d", server, player, failures)
	}
}

// The finder's allowance refusing their own key is the find's answer when
// the server's route then cannot even try (no server key, or its breaker
// open): the same allowance would refuse it there too. It is no failure in
// `bauble status`, and the refusal, not the later route error, is returned
// (and so logged).
func TestAnAllowanceRefusalIsNotMaskedByALaterRouteError(t *testing.T) {
	cases := map[string]struct {
		setup func(t *testing.T, f *fakeOpenAI)
		later error
	}{
		`no server key`: {func(t *testing.T, f *fakeOpenAI) { server(t, f.srv.URL, ``, 2000000, 3) }, errNoRoute},
		`breaker open`: {func(t *testing.T, f *fakeOpenAI) {
			apiframework.SetBreakerForTest(5, time.Now().Add(time.Minute))
		}, errBreakerOpen},
	}
	for name, tc := range cases {
		f := newFakeOpenAI(t)
		m := testModule(t, f, nil)
		tc.setup(t, f)
		relay := &fakeRelay{allowed: map[int]bool{7: true}, model: `player-model`, provider: newFakeOpenAI(t)}
		apiframework.SetRelay(relay)
		apiframework.Shared().SetAllowanceForTest(apiframework.DimBaublesFinder, 7, m.snapshot().DailyTokensPerUser)

		_, err := m.generate(context.Background(), request())
		if errors.Is(err, tc.later) || apiframework.RefusedBy(err) != apiframework.DimBaublesFinder {
			t.Errorf("%v: the allowance refusal is the find's answer, not %v: %v", name, tc.later, err)
		}
		m.mu.Lock()
		failures := m.stats.failures
		m.mu.Unlock()
		if failures != 0 {
			t.Errorf("%v: a refusal is no failure in bauble status: failed=%d", name, failures)
		}
		if relay.sends != 0 || len(relay.results) != 0 {
			t.Errorf("%v: no call on the finder's key, its breaker unfed: sends=%d results=%v", name, relay.sends, relay.results)
		}
	}
}

// DailyTokensPerUser is read live: a `server set` applies to the next find
// once a round has passed (onNewRound, where the share knobs refresh too),
// with no reload of the module.
func TestTheFinderAllowanceIsReadLive(t *testing.T) {
	f := newFakeOpenAI(t)
	m := testModule(t, f, nil)
	m.plug = module.plug // reads Modules.baubles, as the loaded module does
	configs.SetConfigWithLookupsForTest(t, configs.Config{Modules: configs.Modules{
		`baubles`: map[string]any{`Enabled`: true, `DailyTokensPerUser`: 20000},
	}})
	apiframework.Shared().SetAllowanceForTest(apiframework.DimBaublesFinder, 7, 1000)
	if _, err := m.generate(context.Background(), request()); err != nil {
		t.Fatalf("fixture: under a 20000-token allowance, named: %v", err)
	}

	if err := configs.SetVal(`Modules.baubles.DailyTokensPerUser`, `1000`); err != nil {
		t.Fatalf("server set: %v", err)
	}
	m.onNewRound(events.NewRound{})
	if _, err := m.generate(context.Background(), request()); apiframework.RefusedBy(err) != apiframework.DimBaublesFinder {
		t.Fatalf("lowered to 1000 with 1240 spent: the next find is refused: %v", err)
	}

	if err := configs.SetVal(`Modules.baubles.DailyTokensPerUser`, `0`); err != nil {
		t.Fatalf("server set: %v", err)
	}
	m.onNewRound(events.NewRound{})
	if _, err := m.generate(context.Background(), request()); err != nil {
		t.Fatalf("0 is no cap: the next find is named: %v", err)
	}
}

// Baubles hold at most their share of the day's budget; the companion
// still has the rest.
func TestBaublesOverTheirShareFallBack(t *testing.T) {
	f := newFakeOpenAI(t)
	m := testModule(t, f, nil)
	restore := apiframework.SetServerForTest(apiframework.ServerSettings{
		Endpoint:         apiframework.Endpoint{BaseURL: f.srv.URL, APIKey: `sk-test`},
		DailyTokenBudget: 10000, BaublesSharePercent: 25, BreakerErrors: 3, BreakerSeconds: 60,
	})
	t.Cleanup(restore)
	if _, err := apiframework.Reserve(apiframework.ConsumerBaubles, 2400, true); err != nil {
		t.Fatal(err)
	}
	if _, err := m.generate(context.Background(), request()); !errors.Is(err, apiframework.ErrOverShare) ||
		apiframework.RefusedBy(err) != apiframework.RefusedShare {
		t.Fatalf("over a 2500-token share, and it says so: %v", err)
	}
	if atomic.LoadInt32(&f.chats) != 0 {
		t.Fatal("a refused reservation makes no call")
	}
	if _, err := apiframework.Reserve(apiframework.ConsumerCompanion, 5000, true); err != nil {
		t.Fatal("the companion still has the rest of the day")
	}
}

// An admin's regeneration has no finder: it charges no allowance and
// counts under the baubles share.
func TestAdminRegenChargesNoFinder(t *testing.T) {
	f := newFakeOpenAI(t)
	m := testModule(t, f, nil)
	req := request()
	req.FinderUserId = 0
	if _, err := m.generate(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if finderSpent(0) != 0 || baublesTokens() != 240 {
		t.Fatalf("no finder charged, the share counts it: finder0=%d share=%d", finderSpent(0), baublesTokens())
	}
}

// The committed config.yaml's Modules.baubles block, read by repo path and
// put through buildConfig, gives each finder the shipped 20000-token day.
func TestTheShippedFinderAllowance(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(`..`, `..`, `_datafiles`, `config.yaml`))
	if err != nil {
		t.Fatalf("read shipped config: %v", err)
	}
	var shipped struct {
		Modules struct {
			Baubles map[string]any `yaml:"baubles"`
		} `yaml:"Modules"`
	}
	if err := yaml.Unmarshal(data, &shipped); err != nil {
		t.Fatalf("decode shipped config: %v", err)
	}
	if v, ok := shipped.Modules.Baubles[`DailyTokensPerUser`]; !ok || v != 20000 {
		t.Fatalf("Modules.baubles.DailyTokensPerUser in config.yaml: %v (%v)", v, ok)
	}
	c := buildConfig(func(k string) any { return shipped.Modules.Baubles[k] })
	if c.DailyTokensPerUser != 20000 {
		t.Fatalf("shipped DailyTokensPerUser through buildConfig: %d", c.DailyTokensPerUser)
	}
}

// The status counts roll on the ledger's day, the one clock every daily
// count shares, not on a clock of their own.
func TestStatusCountsRollOnTheLedgersClock(t *testing.T) {
	apiframework.ResetBudgetForTest(``)
	t.Cleanup(func() { apiframework.ResetBudgetForTest(``) })
	day1 := time.Date(2020, 3, 4, 12, 0, 0, 0, time.UTC)
	apiframework.SetClockForTest(func() time.Time { return day1 })
	m := &BaublesModule{}
	m.count(false, false)
	m.count(true, true)
	apiframework.SetClockForTest(func() time.Time { return day1.Add(24 * time.Hour) })
	m.count(false, false)
	m.mu.Lock()
	day, server, player, failures := m.stats.day, m.stats.server, m.stats.player, m.stats.failures
	m.mu.Unlock()
	if day != `2020-03-05` || server != 1 || player != 0 || failures != 0 {
		t.Fatalf("a new ledger day starts the counts afresh: day=%s server=%d player=%d failed=%d", day, server, player, failures)
	}
	// bauble status on a later day, before any call of that day, shows
	// that day's counts: none.
	apiframework.SetClockForTest(func() time.Time { return day1.Add(48 * time.Hour) })
	if d := m.info().Detail; !strings.Contains(d, `Today: 0 named on the server's key, 0 on finders' own keys, 0 failed.`) {
		t.Fatalf("the status shows the ledger's day: %s", d)
	}
}
