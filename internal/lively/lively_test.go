package lively

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// The test binary never sees a real key.
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

var (
	featA = Feature{Name: `a`, Purpose: apiframework.LivelyPurpose(`a`), Consumer: `a`, Dim: `a.keyholder`, ModerationBreaker: `a-moderation`}
	featB = Feature{Name: `b`, Purpose: apiframework.LivelyPurpose(`b`), Consumer: `b`, Dim: `b.keyholder`, ModerationBreaker: `b-moderation`}
	lim   = Limits{MinSecondsPerPlayer: 30, DailyTokensPerUser: 1000, TimeoutSeconds: 5, MaxCompletionTokens: 200, ModerateOutput: true, ModerationModel: `m`}
)

// fakeRelay answers as a player's provider would, for every lively purpose.
type fakeRelay struct {
	content string
	status  int
	results []string
	carries apiframework.Carries
	model   string
}

func (r *fakeRelay) Model(userId int, purpose string) (string, bool) {
	return `player-model`, userId == 7 && apiframework.IsLively(purpose)
}

func (r *fakeRelay) Send(ctx context.Context, userId int, body []byte, carries apiframework.Carries) (int, []byte, bool, error) {
	r.carries = carries
	var b struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(body, &b)
	r.model = b.Model
	if r.status != 0 {
		return r.status, nil, true, nil
	}
	raw, _ := json.Marshal(map[string]any{
		`choices`: []any{map[string]any{`finish_reason`: `stop`, `message`: map[string]any{`content`: r.content}}},
		`usage`:   map[string]any{`total_tokens`: 100},
	})
	return 200, raw, true, nil
}

func (r *fakeRelay) Result(userId int, purpose string, err error) {
	r.results = append(r.results, fmt.Sprintf(`%s:%v`, purpose, err == nil))
}

func fresh(t *testing.T, baseURL string, key string) {
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

func TestTurnsAreSpacedPerFeatureUser(t *testing.T) {
	fresh(t, apiframework.DefaultBaseURL, ``)
	r := &fakeRelay{}
	turns := NewTurns()
	clock := time.Now()
	turns.Now = func() time.Time { return clock }
	turns.Relay = func() apiframework.Relay { return r }
	if turns.Reserve(featA, lim, 8) {
		t.Fatal("no relay for that player")
	}
	if !turns.Reserve(featA, lim, 7) || turns.Reserve(featA, lim, 7) {
		t.Fatal("one at a time")
	}
	turns.Release(7)
	if turns.Reserve(featA, lim, 7) {
		t.Fatal("spaced")
	}
	clock = clock.Add(31 * time.Second)
	if !turns.Reserve(featA, lim, 7) {
		t.Fatal("again after the gap")
	}
	turns.Release(7)
	clock = clock.Add(time.Hour)
	apiframework.Shared().SetAllowanceForTest(featA.Dim, 7, 1000)
	if turns.Reserve(featA, lim, 7) {
		t.Fatal("allowance spent")
	}
}

func TestAskReportsOnlyTheKeysFailures(t *testing.T) {
	fresh(t, apiframework.DefaultBaseURL, ``)
	r := &fakeRelay{content: `{"ok":true}`}
	chat := apiframework.Chat{Messages: []apiframework.Message{{Role: `user`, Content: `hi`}}, SchemaName: `x`, Schema: map[string]any{}, MaxTokens: 200}
	ok := func(string) error { return nil }

	content, tokens, err := Ask(context.Background(), featA, lim, r, 7, chat, ok)
	if err != nil || content != `{"ok":true}` || tokens != 100 || r.model != `player-model` || r.carries != apiframework.CarriesNoPlayerData {
		t.Fatalf("on the player's own model, no player data: %q %d %v %q", content, tokens, err, r.model)
	}
	if apiframework.Today().Tokens != 0 || apiframework.Allowance(featA.Dim, 7) != 100 || apiframework.Allowance(featB.Dim, 7) != 0 {
		t.Fatal("the feature's own allowance only, never the server's budget")
	}

	_, _, err = Ask(context.Background(), featA, lim, r, 7, chat, func(string) error { return fmt.Errorf(`%w: rules`, ErrUnusable) })
	if !errors.Is(err, ErrUnusable) {
		t.Fatal("a refused answer is no result")
	}
	_, _, err = Ask(context.Background(), featA, lim, r, 7, chat, func(string) error { return errors.New(`not the schema`) })
	if err == nil {
		t.Fatal("an answer outside the schema is no result")
	}
	r.status = 500
	if _, _, err = Ask(context.Background(), featB, lim, r, 7, chat, ok); err == nil {
		t.Fatal("a provider error is no result")
	}
	want := featA.Purpose + `:true,` + featA.Purpose + `:true,` + featA.Purpose + `:false,` + featB.Purpose + `:false`
	if got := strings.Join(r.results, `,`); got != want {
		t.Fatalf("answered (even if refused) is a success; the schema ignored and the provider failing are failures, each on its own purpose:\n got %v\nwant %v", got, want)
	}
	if _, _, err := Ask(context.Background(), featA, lim, nil, 7, chat, ok); !errors.Is(err, ErrNoRelay) {
		t.Fatal("no relay")
	}
	if _, _, err := Ask(context.Background(), featA, lim, r, 8, chat, ok); !errors.Is(err, ErrNoRelay) {
		t.Fatal("a player whose relay does not allow it")
	}
}

func TestModerate(t *testing.T) {
	fresh(t, apiframework.DefaultBaseURL, ``)
	if only, err := Moderate(featA, lim, `text`); err != nil || !only {
		t.Fatal("no server key: the keyholder's alone")
	}
	off := lim
	off.ModerateOutput = false
	if only, err := Moderate(featA, off, `text`); err != nil || only {
		t.Fatal("moderation off: everyone, unchecked")
	}
	var flagged atomic.Bool
	var checks atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checks.Add(1)
		fmt.Fprintf(w, `{"results":[{"flagged":%t}]}`, flagged.Load())
	}))
	t.Cleanup(srv.Close)
	fresh(t, srv.URL, `sk-test`)
	if only, err := Moderate(featA, lim, `text`); err != nil || only || checks.Load() != 1 {
		t.Fatal("checked and clean: everyone")
	}
	flagged.Store(true)
	if _, err := Moderate(featA, lim, `text`); !errors.Is(err, ErrNotModerated) {
		t.Fatal("flagged: kept out")
	}
}

func TestConfigReaders(t *testing.T) {
	if Int(nil, 5, 0, 100) != 5 || Int(`250`, 5, 0, 100) != 100 || Int(-3, 5, 0, 100) != 0 || Int(7.0, 5, 0, 100) != 7 {
		t.Fatal("Int: default, bounds, types")
	}
	if !Bool(nil, true) || Bool(`false`, true) || !Bool(`junk`, true) || Bool(false, true) {
		t.Fatal("Bool: default when unset or unreadable")
	}
	if String(`  x `) != `x` || String(3) != `` {
		t.Fatal("String")
	}
}
