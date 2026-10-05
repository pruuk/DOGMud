package roomlife

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
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/roomlife"
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
	return `player-model`, userId == 7 && purpose == apiframework.PurposeRoomLife
}

func (r *fakeRelay) Send(ctx context.Context, userId int, body []byte, carries apiframework.Carries) (int, []byte, bool, error) {
	r.body = string(body)
	raw, _ := json.Marshal(map[string]any{
		`choices`: []any{map[string]any{`finish_reason`: `stop`, `message`: map[string]any{`content`: r.content}}},
		`usage`:   map[string]any{`total_tokens`: 250},
	})
	return 200, raw, true, nil
}

func (r *fakeRelay) Result(userId int, purpose string, err error) {
	r.results = append(r.results, fmt.Sprintf(`%s:%v`, purpose, err == nil))
}

func testModule(t *testing.T, r *fakeRelay) *RoomLifeModule {
	t.Helper()
	restore := apiframework.SetServerForTest(apiframework.ServerSettings{
		Endpoint: apiframework.Endpoint{BaseURL: apiframework.DefaultBaseURL}, DailyTokenBudget: 1000, BreakerErrors: 3, BreakerSeconds: 60,
	})
	apiframework.ResetBudgetForTest(``)
	apiframework.ResetBreaker()
	t.Cleanup(func() { restore(); apiframework.ResetBudgetForTest(``); apiframework.ResetBreaker() })
	cfg := buildConfig(nil)
	cfg.ModerateOutput = false
	m := &RoomLifeModule{cfg: cfg, turns: lively.NewTurns()}
	m.turns.Relay = func() apiframework.Relay { return r }
	return m
}

func place(canSee bool) roomlife.Request {
	return roomlife.Request{Title: `Baker's Row`, Description: `A narrow street of shuttered shopfronts.`,
		Examples: []string{`A magpie chatters from the eaves.`}, NPCs: []string{`a weary carter`}, Travellers: 2, CanSee: canSee, TimeOfDay: `night`}
}

func TestDefaults(t *testing.T) {
	c := buildConfig(nil)
	if !c.Enabled || c.Chance != 10 || c.MinSecondsPerPlayer != 60 || !c.ModerateOutput || c.DailyTokensPerUser != 20000 {
		t.Fatalf("on, 10%%, spaced, moderated, capped: %+v", c)
	}
}

func TestAnEventIsWrittenOnThePlayersKey(t *testing.T) {
	r := &fakeRelay{content: `{"kind":"heard","text":"You hear a shutter bang twice somewhere up the row."}`}
	m := testModule(t, r)
	if !m.Reserve(7) || m.Reserve(8) {
		t.Fatal("only a player whose relay allows it")
	}
	res, err := m.Generate(context.Background(), 7, place(false))
	if err != nil || res.Kind != roomlife.KindHeard || !strings.HasPrefix(res.Text, `You hear a shutter`) {
		t.Fatalf("a sound: %+v %v", res, err)
	}
	for _, want := range []string{`Baker's Row`, `a weary carter`, `magpie`, `\"can_see\": false`, `room_event`, `player-model`} {
		if !strings.Contains(r.body, want) {
			t.Fatalf("the request carries %s", want)
		}
	}
	if m.turns.Busy(7) || apiframework.Allowance(apiframework.DimRoomLifeKeyholder, 7) != 250 || apiframework.Allowance(apiframework.DimNPCIdleKeyholder, 7) != 0 {
		t.Fatal("turn given back; its own allowance spent, not idle moments'")
	}
}

// The light rule reaches the model's answer: a seen event written for a
// keyholder who cannot see is refused, and that is not the key's failure.
func TestASeenEventInTheDarkIsRefusedNotCountedAgainstTheKey(t *testing.T) {
	r := &fakeRelay{content: `{"kind":"seen","text":"A rat darts along the foot of the wall."}`}
	m := testModule(t, r)
	if _, err := m.Generate(context.Background(), 7, place(false)); !errors.Is(err, roomlife.ErrUnusable) {
		t.Fatalf("refused: %v", err)
	}
	if strings.Join(r.results, `,`) != apiframework.PurposeRoomLife+`:true` {
		t.Fatalf("on its own breaker, as an answer: %v", r.results)
	}
	if res, err := m.Generate(context.Background(), 7, place(true)); err != nil || res.Kind != roomlife.KindSeen {
		t.Fatalf("lit: seen is fine: %+v %v", res, err)
	}
}

func TestTheRelayKnowsTheSchema(t *testing.T) {
	src, err := os.ReadFile(`../aicompanion/relayweb/relay.js`)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`var LIVELY_SCHEMAS = \[([^\]]*)\];`).FindStringSubmatch(string(src))
	if m == nil || !strings.Contains(m[1], `'`+roomlife.ReplySchemaName+`'`) {
		t.Fatalf("relay.js LIVELY_SCHEMAS must list %q", roomlife.ReplySchemaName)
	}
}

func TestThePromptKeepsPlayersOut(t *testing.T) {
	msgs := buildMessages(place(true))
	if len(msgs) != 2 || !strings.Contains(msgs[0].Content, `never to them`) || !strings.Contains(msgs[0].Content, `can_see is false`) {
		t.Fatal("the system prompt keeps players out of it and names the light rule")
	}
	if !strings.Contains(msgs[1].Content, `"travellers": 2`) {
		t.Fatal("travellers are counted")
	}
}

// A place outside Gaius (a rift) sends its setting and no time of day, and
// the prompt says the setting overrides the world.
func TestThePromptCarriesAnotherWorldsSetting(t *testing.T) {
	req := place(true)
	req.Setting, req.TimeOfDay = `A maze of black crystal in another dimension. No animals.`, ``
	msgs := buildMessages(req)
	if !strings.Contains(msgs[0].Content, `overrides the world above`) {
		t.Fatal("the system prompt lets a setting override Gaius")
	}
	if !strings.Contains(msgs[1].Content, `"setting": "A maze of black crystal`) || strings.Contains(msgs[1].Content, `time_of_day`) {
		t.Fatalf("setting sent, no time of day: %s", msgs[1].Content)
	}
	if strings.Contains(buildMessages(place(true))[1].Content, `"setting"`) {
		t.Fatal("an ordinary place sends no setting")
	}
}
