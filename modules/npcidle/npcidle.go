// Package npcidle is the model side of generated idle moments
// (internal/npcidle): now and then an NPC's idle emote or say is written
// fresh by a model, on the own key of a player in the room with it, instead
// of one of its set lines. This module installs the generator.
//
// It never uses the server's key to write a moment. A moment is written
// only through the companion's key relay (apiframework.PlayerRelay), for a
// player who left "Make the world livelier" ticked on the key page (the
// lively permission; this feature lends under apiframework.PurposeNPCIdle,
// with its own breaker), against that player's own daily allowance. The server's key, when there is one, only
// moderates a moment before anyone else reads it.
package npcidle

import (
	"context"
	"fmt"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/lively"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/npcidle"
	"github.com/GoMudEngine/GoMud/internal/plugins"
)

// feature is this module's own counters on a player's key: its purpose
// (its own breaker), consumer, allowance and moderation breaker.
var feature = lively.Feature{
	Name:              `npcidle`,
	Purpose:           apiframework.PurposeNPCIdle,
	Consumer:          apiframework.ConsumerNPCIdle,
	Dim:               apiframework.DimNPCIdleKeyholder,
	ModerationBreaker: `npcidle-moderation`,
}

// NpcIdleModule is the module's state. cfg is read by delivery goroutines
// behind mu.
type NpcIdleModule struct {
	plug *plugins.Plugin

	mu  sync.Mutex
	cfg Config

	// turns spaces moments on each player's key (lively.Turns).
	turns *lively.Turns
}

var module NpcIdleModule

func init() {
	module = NpcIdleModule{
		plug:  plugins.New(`npcidle`, `0.1.0`),
		turns: lively.NewTurns(),
	}
	module.plug.Callbacks.SetOnLoad(module.onLoad)
	module.plug.Callbacks.SetOnSave(module.onSave)
}

func (m *NpcIdleModule) get(k string) any { return m.plug.Config.Get(k) }

func (m *NpcIdleModule) onLoad() {
	m.configure(buildConfig(m.get))
	events.RegisterListener(events.NewRound{}, m.onNewRound)
}

// onNewRound re-reads, on the game loop, the server key's settings
// (moderation reads only the snapshot, apiframework.RefreshServer) and the
// settings a moment reads live, so a `server set` of Chance,
// MinSecondsPerPlayer or DailyTokensPerUser reaches the next idle tick.
func (m *NpcIdleModule) onNewRound(e events.Event) events.ListenerReturn {
	if !m.snapshot().Enabled {
		return events.Continue
	}
	apiframework.RefreshServer()
	if m.plug != nil {
		c, gap, perUser := chance(m.get), minSecondsPerPlayer(m.get), dailyTokensPerUser(m.get)
		m.mu.Lock()
		m.cfg.Chance, m.cfg.MinSecondsPerPlayer, m.cfg.DailyTokensPerUser = c, gap, perUser
		m.mu.Unlock()
	}
	return events.Continue
}

func (m *NpcIdleModule) onSave() error {
	if m.snapshot().Enabled {
		apiframework.SaveBudget()
	}
	return nil
}

// configure applies a config and installs (or removes) the generator.
func (m *NpcIdleModule) configure(cfg Config) {
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
	if !cfg.Enabled {
		npcidle.SetGenerator(nil)
		mudlog.Info(`npcidle`, `idle moments`, `off`, `reason`, `Modules.npcidle.Enabled is false`)
		return
	}
	npcidle.SetGenerator(m)
	s := apiframework.RefreshServer() // at load, on the game loop
	mudlog.Info(`npcidle`, `idle moments`, `on players' own keys`, `chance`, cfg.Chance,
		`moderated`, cfg.ModerateOutput && s.HasKey())
}

func (m *NpcIdleModule) snapshot() Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg
}

// Chance is the percent of idle emotes and says tried as moments.
func (m *NpcIdleModule) Chance() int {
	c := m.snapshot()
	if !c.Enabled {
		return 0
	}
	return c.Chance
}

// Reserve takes userId's turn when their key may write a moment now
// (lively.Turns.Reserve). Under the mud lock.
func (m *NpcIdleModule) Reserve(userId int) bool {
	cfg := m.snapshot()
	return cfg.Enabled && m.turns.Reserve(feature, cfg.limits(), userId)
}

// Generate writes the moment on userId's key (lively.Ask) and gives their
// turn back. Off the mud lock: it touches no game state.
func (m *NpcIdleModule) Generate(ctx context.Context, userId int, req npcidle.Request) (npcidle.Result, error) {
	defer m.turns.Release(userId)
	cfg := m.snapshot()
	chat := apiframework.Chat{
		Messages:   buildMessages(req),
		SchemaName: npcidle.ReplySchemaName,
		Schema:     npcidle.ReplySchema(),
		MaxTokens:  cfg.MaxCompletionTokens,
	}
	var res npcidle.Result
	decode := func(content string) error {
		parsed, err := npcidle.ParseReply(content)
		if err != nil {
			return err // the schema ignored: the key's failure
		}
		// The rules refusing what it wrote (run again by the engine with
		// the NPC's name) is not the key's failure.
		if res, err = npcidle.CleanResult(parsed, ``); err != nil {
			return fmt.Errorf(`%w: %w`, lively.ErrUnusable, err)
		}
		return nil
	}
	_, tokens, err := lively.Ask(ctx, feature, cfg.limits(), m.turns.Relay(), userId, chat, decode)
	if cfg.LogRequests {
		mudlog.Info(`npcidle`, `action`, `model call`, `npc`, req.NPC.Name, `room`, req.Place.Title,
			`prompt`, PromptVersion, `tokens`, tokens, `error`, errString(err))
	}
	if err != nil {
		return npcidle.Result{}, err
	}
	keyholderOnly, err := lively.Moderate(feature, cfg.limits(), res.Text)
	if err != nil {
		return npcidle.Result{}, err
	}
	res.KeyholderOnly = keyholderOnly
	return res, nil
}

func errString(err error) string {
	if err == nil {
		return ``
	}
	return err.Error()
}
