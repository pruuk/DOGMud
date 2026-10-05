package lookdetail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/lively"
	"github.com/GoMudEngine/GoMud/internal/lookdetail"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

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

type fakeRelay struct {
	content string
	body    string
	results []string
}

func (r *fakeRelay) Model(userId int, purpose string) (string, bool) {
	return `player-model`, userId == 7 && purpose == apiframework.PurposeLookDetail
}

func (r *fakeRelay) Send(ctx context.Context, userId int, body []byte, carries apiframework.Carries) (int, []byte, bool, error) {
	r.body = string(body)
	raw, _ := json.Marshal(map[string]any{
		`choices`: []any{map[string]any{`finish_reason`: `stop`, `message`: map[string]any{`content`: r.content}}},
		`usage`:   map[string]any{`total_tokens`: 300},
	})
	return 200, raw, true, nil
}

func (r *fakeRelay) Result(userId int, purpose string, err error) {
	r.results = append(r.results, fmt.Sprintf(`%s:%v`, purpose, err == nil))
}

func testModule(t *testing.T, r *fakeRelay) *LookDetailModule {
	t.Helper()
	restore := apiframework.SetServerForTest(apiframework.ServerSettings{
		Endpoint: apiframework.Endpoint{BaseURL: apiframework.DefaultBaseURL}, DailyTokenBudget: 1000, BreakerErrors: 3, BreakerSeconds: 60,
	})
	apiframework.ResetBudgetForTest(``)
	apiframework.ResetBreaker()
	t.Cleanup(func() { restore(); apiframework.ResetBudgetForTest(``); apiframework.ResetBreaker() })
	cfg := buildConfig(nil)
	cfg.ModerateOutput = false
	m := &LookDetailModule{cfg: cfg, turns: lively.NewTurns()}
	m.turns.Relay = func() apiframework.Relay { return r }
	return m
}

func look() lookdetail.Request {
	return lookdetail.Request{Thing: `fountain`, Excerpt: `A dry fountain stands in the square.`, Title: `Market Square`,
		Description: `A dry fountain stands in the square.`, NPCs: []string{`a flower seller`}, TimeOfDay: `day`}
}

func TestDefaults(t *testing.T) {
	c := buildConfig(nil)
	if !c.Enabled || c.MinSecondsPerPlayer != 5 || !c.ModerateOutput || c.DailyTokensPerUser != 20000 {
		t.Fatalf("on, lightly spaced, moderated, capped: %+v", c)
	}
}

func TestADetailIsWrittenOnTheLookersKey(t *testing.T) {
	r := &fakeRelay{content: `{"text":"Moss fills every crack of the basin, and a lopsided heart is scratched into the rim."}`}
	m := testModule(t, r)
	if !m.Reserve(7) || m.Reserve(8) {
		t.Fatal("only a player whose relay allows it")
	}
	res, err := m.Generate(context.Background(), 7, look())
	if err != nil || !strings.Contains(res.Text, `lopsided heart`) {
		t.Fatalf("written: %+v %v", res, err)
	}
	for _, want := range []string{`fountain`, `Market Square`, `a flower seller`, `look_detail`, `player-model`} {
		if !strings.Contains(r.body, want) {
			t.Fatalf("the request carries %s", want)
		}
	}
	if m.turns.Busy(7) || apiframework.Allowance(apiframework.DimLookDetailKeyholder, 7) != 300 || apiframework.Allowance(apiframework.DimRoomLifeKeyholder, 7) != 0 {
		t.Fatal("turn given back; its own allowance spent, no other feature's")
	}
	r.content = `{"text":"Short."}`
	if _, err := m.Generate(context.Background(), 7, look()); !errors.Is(err, lookdetail.ErrUnusable) {
		t.Fatalf("refused by the rules: %v", err)
	}
	if strings.Join(r.results, `,`) != apiframework.PurposeLookDetail+`:true,`+apiframework.PurposeLookDetail+`:true` {
		t.Fatalf("both answered: on its own breaker, as successes: %v", r.results)
	}
}

func TestTheRelayKnowsTheSchema(t *testing.T) {
	src, err := os.ReadFile(`../aicompanion/relayweb/relay.js`)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`var LIVELY_SCHEMAS = \[([^\]]*)\];`).FindStringSubmatch(string(src))
	if m == nil || !strings.Contains(m[1], `'`+lookdetail.ReplySchemaName+`'`) {
		t.Fatalf("relay.js LIVELY_SCHEMAS must list %q", lookdetail.ReplySchemaName)
	}
}

func TestThePromptKeepsItScenery(t *testing.T) {
	msgs := buildMessages(look())
	if len(msgs) != 2 || !strings.Contains(msgs[0].Content, `It is scenery, nothing more.`) || !strings.Contains(msgs[0].Content, `Do not mention any other traveller`) {
		t.Fatal("the system prompt forbids invented loot and other players")
	}
	if !strings.Contains(msgs[1].Content, `"thing": "fountain"`) {
		t.Fatal("the thing looked at")
	}
}
