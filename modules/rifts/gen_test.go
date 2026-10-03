package rifts

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
	rifts "github.com/GoMudEngine/GoMud/internal/rifts"
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
	return `player-model`, userId == 7 && purpose == apiframework.PurposeRifts
}

func (r *fakeRelay) Send(ctx context.Context, userId int, body []byte, carries apiframework.Carries) (int, []byte, bool, error) {
	r.body = string(body)
	raw, _ := json.Marshal(map[string]any{
		`choices`: []any{map[string]any{`finish_reason`: `stop`, `message`: map[string]any{`content`: r.content}}},
		`usage`:   map[string]any{`total_tokens`: 1800},
	})
	return 200, raw, true, nil
}

func (r *fakeRelay) Result(userId int, purpose string, err error) {
	r.results = append(r.results, fmt.Sprintf(`%s:%v`, purpose, err == nil))
}

func testWriter(t *testing.T, r *fakeRelay) *roomWriter {
	t.Helper()
	restore := apiframework.SetServerForTest(apiframework.ServerSettings{
		Endpoint: apiframework.Endpoint{BaseURL: apiframework.DefaultBaseURL}, DailyTokenBudget: 1000, BreakerErrors: 3, BreakerSeconds: 60,
	})
	apiframework.ResetBudgetForTest(``)
	apiframework.ResetBreaker()
	t.Cleanup(func() { restore(); apiframework.ResetBudgetForTest(``); apiframework.ResetBreaker() })
	cfg := buildGenConfig(nil)
	cfg.ModerateOutput = false
	w := newRoomWriter(cfg)
	w.turns.Relay = func() apiframework.Relay { return r }
	return w
}

// passageRequest is a request for a new passage of the shipped Obelisk.
func passageRequest(t *testing.T) rifts.GenRequest {
	t.Helper()
	loaded, err := rifts.ReadProfiles(`../../_datafiles/world/dogmud/rifts`)
	if err != nil {
		t.Fatal(err)
	}
	p := loaded[`obelisk`]
	if p == nil {
		t.Fatal(`the obelisk ships`)
	}
	return p.GenRequest(rifts.PoolPassage)
}

// newPassage is a reply a model might give: one of the examples, renamed
// and reworded at the start so it is not a copy.
func newPassage(t *testing.T, req rifts.GenRequest, title string) string {
	t.Helper()
	r, err := rifts.ParseRoomReply(req.Examples[0])
	if err != nil {
		t.Fatal(err)
	}
	r.Title = title
	r.Description = `Freshly written, ` + r.Description
	b, _ := json.Marshal(r)
	return string(b)
}

func TestGenDefaults(t *testing.T) {
	c := buildGenConfig(nil)
	if !c.Enabled || c.Chance != 25 || c.MinSecondsPerPlayer != 300 || !c.ModerateOutput || c.DailyTokensPerUser != 40000 || c.MaxPerPool != 200 {
		t.Fatalf("on, 25%%, spaced, moderated, capped: %+v", c)
	}
	off := buildGenConfig(func(k string) any {
		return map[string]any{`GenerateEnabled`: false, `GenerateChance`: 500}[k]
	})
	if off.Enabled || off.Chance != 100 {
		t.Fatalf("off, chance held to 100: %+v", off)
	}
}

func TestARoomIsWrittenOnThePlayersKey(t *testing.T) {
	req := passageRequest(t)
	r := &fakeRelay{content: newPassage(t, req, `The Corridor of Second Thoughts`)}
	w := testWriter(t, r)
	if !w.Reserve(7) || w.Reserve(8) {
		t.Fatal("only a player whose relay allows it")
	}
	room, err := w.Generate(context.Background(), 7, req)
	if err != nil {
		t.Fatal(err)
	}
	if room.Title != `The Corridor of Second Thoughts` || room.Pool != rifts.PoolPassage || room.Source != `generated` ||
		room.Model != `player-model` || room.PromptVersion != RoomPromptVersion || !strings.HasPrefix(room.Id, `gen-a-`) {
		t.Fatalf("a passage, with its provenance: %+v", room)
	}
	for _, want := range []string{`Watchers`, `rift_room`, `player-model`, `used_titles`, `reserved_nouns`, `prism`, `passage`, `seeds`,
		strings.ReplaceAll(req.UsedTitles[0], `'`, `'`)} {
		if !strings.Contains(r.body, want) {
			t.Fatalf("the request carries %s", want)
		}
	}
	if w.turns.Busy(7) || apiframework.Allowance(apiframework.DimRiftsKeyholder, 7) != 1800 || apiframework.Allowance(apiframework.DimRoomLifeKeyholder, 7) != 0 {
		t.Fatal("turn given back; its own allowance spent, not ambient events'")
	}
}

// A room the bank's rules refuse is the rules', not the key's failure; a
// reply that ignores the schema is the key's.
func TestRefusedRoomsAndTheBreaker(t *testing.T) {
	req := passageRequest(t)
	r := &fakeRelay{content: newPassage(t, req, req.UsedTitles[0])}
	w := testWriter(t, r)
	if _, err := w.Generate(context.Background(), 7, req); !errors.Is(err, lively.ErrUnusable) || !errors.Is(err, rifts.ErrUnusableRoom) {
		t.Fatalf("a used title is refused: %v", err)
	}
	r.content = `{"room": "a corridor"}`
	if _, err := w.Generate(context.Background(), 7, req); err == nil || errors.Is(err, lively.ErrUnusable) {
		t.Fatalf("the schema ignored: %v", err)
	}
	if got := strings.Join(r.results, `,`); got != apiframework.PurposeRifts+`:true,`+apiframework.PurposeRifts+`:false` {
		t.Fatalf("on its own breaker: an answer, then a failure: %v", got)
	}
}

// A banked room is read by everyone: one the server cannot moderate (no
// server key here) is not kept.
func TestAnUncheckableRoomIsNotKept(t *testing.T) {
	req := passageRequest(t)
	r := &fakeRelay{content: newPassage(t, req, `The Corridor of Second Thoughts`)}
	w := testWriter(t, r)
	w.cfg.ModerateOutput = true
	if w.Reserve(7) {
		t.Fatal("no call is made for a room that could not be kept")
	}
	if room, err := w.Generate(context.Background(), 7, req); err == nil || room != nil {
		t.Fatalf("and one written anyway is not kept: %+v %v", room, err)
	}
	w.moderatable = func() bool { return true }
	if !w.Reserve(7) {
		t.Fatal("with the server able to check, the key may write")
	}
	w.turns.Release(7)
}

// The names players see in the exits list and type are moderated too.
func TestRoomTextsIncludeNames(t *testing.T) {
	room := &rifts.Template{Title: `T`, Description: `D`, Nouns: map[string]string{`slab`: `a slab`},
		Doors: []rifts.DoorSpec{{Exit: `arch`, Description: `an arch`}}}
	got := strings.Join(roomTexts(room), `|`)
	if !strings.Contains(got, `slab`) || !strings.Contains(got, `arch, `) && !strings.Contains(got, `, arch`) {
		t.Fatalf("names moderated: %s", got)
	}
}

func TestTheRelayKnowsTheRoomSchema(t *testing.T) {
	src, err := os.ReadFile(`../aicompanion/relayweb/relay.js`)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`var LIVELY_SCHEMAS = \[([^\]]*)\];`).FindStringSubmatch(string(src))
	if m == nil || !strings.Contains(m[1], `'`+rifts.RoomSchemaName+`'`) {
		t.Fatalf("relay.js LIVELY_SCHEMAS must list %q", rifts.RoomSchemaName)
	}
}
