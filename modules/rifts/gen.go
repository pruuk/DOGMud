package rifts

// Generated rift rooms, the model side of internal/rifts gen.go: now and
// then, as a run's rooms are built, a new room for one of the profile's
// pools is written in the background on the own key of a player in the run
// and, once the engine's rules and moderation pass it, saved to the bank so
// the pool grows.
//
// It never uses the server's key to write a room: only the companion's key
// relay, for a player who left "Make the world livelier" ticked on the key
// page (this feature lends under apiframework.PurposeRifts, with its own
// breaker), against that player's own daily allowance. The server's key,
// when there is one, only moderates; a room the server cannot check is
// never saved, since everyone who walks the rift will read it. The call,
// the turns and the moderation are internal/lively's.

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/lively"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	rifts "github.com/GoMudEngine/GoMud/internal/rifts"
)

// genFeature is room writing's own counters on a player's key.
var genFeature = lively.Feature{
	Name:              `rifts`,
	Purpose:           apiframework.PurposeRifts,
	Consumer:          apiframework.ConsumerRifts,
	Dim:               apiframework.DimRiftsKeyholder,
	ModerationBreaker: `rifts-moderation`,
}

// roomWriter is the installed rifts.Generator. cfg is read by generation
// goroutines behind mu.
type roomWriter struct {
	mu    sync.Mutex
	cfg   GenConfig
	turns *lively.Turns

	moderatable func() bool // tests stand in for the server's moderation check
}

func newRoomWriter(cfg GenConfig) *roomWriter {
	return &roomWriter{cfg: cfg, turns: lively.NewTurns()}
}

func (w *roomWriter) snapshot() GenConfig {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.cfg
}

// setLive applies the settings read every round.
func (w *roomWriter) setLive(get lively.Getter) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cfg.Chance, w.cfg.MinSecondsPerPlayer, w.cfg.DailyTokensPerUser, w.cfg.MaxPerPool =
		genChance(get), genMinSeconds(get), genDailyTokens(get), genMaxPerPool(get)
}

// Chance is the percent of rooms built that ask for a new one.
func (w *roomWriter) Chance() int {
	c := w.snapshot()
	if !c.Enabled {
		return 0
	}
	return c.Chance
}

// MaxPerPool caps the generated rooms in one pool; 0 is no cap.
func (w *roomWriter) MaxPerPool() int { return w.snapshot().MaxPerPool }

// Reserve takes userId's turn when their key may write a room now.
func (w *roomWriter) Reserve(userId int) bool {
	cfg := w.snapshot()
	if !cfg.Enabled || !w.canModerate(cfg) {
		return false
	}
	return w.turns.Reserve(genFeature, cfg.limits(), userId)
}

// canModerate reports whether a room written now could be checked before
// it is banked. A room the server cannot check is never kept, so with
// ModerateOutput on and no way to check, no call is made at all: the
// player's tokens are not spent on a room that would be thrown away.
func (w *roomWriter) canModerate(cfg GenConfig) bool {
	if !cfg.ModerateOutput {
		return true
	}
	if w.moderatable != nil {
		return w.moderatable()
	}
	return apiframework.Server().HasKey() && !apiframework.Blocked(genFeature.ModerationBreaker, time.Now())
}

// Generate writes the room on userId's key and gives their turn back. Off
// the mud lock.
func (w *roomWriter) Generate(ctx context.Context, userId int, req rifts.GenRequest) (*rifts.Template, error) {
	defer w.turns.Release(userId)
	cfg := w.snapshot()
	chat := apiframework.Chat{
		Messages:   buildRoomMessages(req),
		SchemaName: rifts.RoomSchemaName,
		Schema:     rifts.RoomSchema(req),
		MaxTokens:  cfg.MaxCompletionTokens,
	}
	model := ``
	if r := w.turns.Relay(); r != nil {
		model, _ = r.Model(userId, genFeature.Purpose)
	}
	var room *rifts.Template
	decode := func(content string) error {
		reply, err := rifts.ParseRoomReply(content)
		if err != nil {
			return err // the schema ignored: the key's failure
		}
		if room, err = rifts.BuildGenerated(req, reply); err != nil {
			return fmt.Errorf(`%w: %w`, lively.ErrUnusable, err)
		}
		return nil
	}
	_, tokens, err := lively.Ask(ctx, genFeature, cfg.limits(), w.turns.Relay(), userId, chat, decode)
	if cfg.LogRequests {
		mudlog.Info(`rifts.generate`, `action`, `model call`, `profile`, req.ProfileId, `pool`, req.Pool,
			`prompt`, RoomPromptVersion, `tokens`, tokens, `error`, errString(err))
	}
	if err != nil {
		return nil, err
	}
	keyholderOnly, err := lively.Moderate(genFeature, cfg.limits(), roomTexts(room)...)
	if err != nil {
		return nil, err
	}
	if keyholderOnly {
		// Everyone who walks the rift reads a banked room: one the server
		// cannot check is not kept.
		return nil, fmt.Errorf(`not saved: the server cannot moderate it now`)
	}
	room.Model = model
	room.PromptVersion = RoomPromptVersion
	return room, nil
}

// roomTexts is everything a player can read in a room.
func roomTexts(t *rifts.Template) []string {
	out := []string{t.Title, t.Description}
	names := []string{}
	for name, look := range t.Nouns {
		out = append(out, look)
		names = append(names, name)
	}
	for _, d := range t.Doors {
		out = append(out, d.Description)
		names = append(names, d.Exit)
	}
	// The words players see in the exits list and type.
	out = append(out, strings.Join(names, `, `))
	out = append(out, t.IdleMessages...)
	if ps := t.Puzzle; ps != nil {
		out = append(out, ps.Solved, ps.Wrong)
		out = append(out, ps.Answers...)
	}
	if ts := t.Trap; ts != nil {
		out = append(out, ts.Triggered, ts.Avoided)
	}
	// Nothing empty goes to moderation (a lens table's puzzle has no wrong
	// answer text, for one).
	kept := out[:0]
	for _, s := range out {
		if strings.TrimSpace(s) != `` {
			kept = append(kept, s)
		}
	}
	return kept
}

func errString(err error) string {
	if err == nil {
		return ``
	}
	return err.Error()
}

// ---- config ----

// GenConfig is the resolved room-writing settings (the Generate* keys of
// Modules.rifts).
type GenConfig struct {
	Enabled bool

	// Chance is the percent (0 to 100) of rooms built, of a pool the
	// profile lets a model write for, that ask for a new room of it.
	Chance int

	// MinSecondsPerPlayer spaces the rooms one player's key writes.
	MinSecondsPerPlayer int

	TimeoutSeconds      int
	MaxCompletionTokens int

	// DailyTokensPerUser is what one player's key may spend on rooms in a
	// UTC day (apiframework's rifts.keyholder allowance). 0 is no cap.
	DailyTokensPerUser int

	// MaxPerPool caps the generated rooms one pool may hold; 0 is no cap.
	MaxPerPool int

	ModerateOutput  bool
	ModerationModel string
	LogRequests     bool
}

func genChance(get lively.Getter) int { return lively.Int(get(`GenerateChance`), 25, 0, 100) }
func genMinSeconds(get lively.Getter) int {
	return lively.Int(get(`GenerateMinSecondsPerPlayer`), 300, 0, 86400)
}
func genDailyTokens(get lively.Getter) int {
	return lively.Int(get(`GenerateDailyTokensPerUser`), 40000, 0, 1<<31-1)
}
func genMaxPerPool(get lively.Getter) int {
	return lively.Int(get(`GenerateMaxPerPool`), 200, 0, 100000)
}

// buildGenConfig resolves the settings with safe defaults and bounds.
// Enabled defaults to true: nothing is spent unless a player in a rift has
// their own key up in the companion's relay with "Make the world livelier"
// ticked, and then only on their key.
func buildGenConfig(get lively.Getter) GenConfig {
	if get == nil {
		get = func(string) any { return nil }
	}
	c := GenConfig{
		Enabled:             lively.Bool(get(`GenerateEnabled`), true),
		Chance:              genChance(get),
		MinSecondsPerPlayer: genMinSeconds(get),
		TimeoutSeconds:      lively.Int(get(`GenerateTimeoutSeconds`), 75, 10, 90),
		MaxCompletionTokens: lively.Int(get(`GenerateMaxCompletionTokens`), 4000, 800, 4000),
		DailyTokensPerUser:  genDailyTokens(get),
		MaxPerPool:          genMaxPerPool(get),
		ModerateOutput:      lively.Bool(get(`GenerateModerateOutput`), true),
		ModerationModel:     lively.String(get(`GenerateModerationModel`)),
		LogRequests:         lively.Bool(get(`GenerateLogRequests`), false),
	}
	if c.ModerationModel == `` {
		c.ModerationModel = `omni-moderation-latest`
	}
	return c
}

func (c GenConfig) limits() lively.Limits {
	return lively.Limits{
		MinSecondsPerPlayer: c.MinSecondsPerPlayer,
		DailyTokensPerUser:  c.DailyTokensPerUser,
		TimeoutSeconds:      c.TimeoutSeconds,
		MaxCompletionTokens: c.MaxCompletionTokens,
		ModerateOutput:      c.ModerateOutput,
		ModerationModel:     c.ModerationModel,
	}
}
