// Package roomlife is the model side of generated ambient events
// (internal/roomlife): now and then, when a room is due to show one of its
// set ambient lines, a model writes a fresh event instead, seen or heard,
// on the own key of a player in the room. This module installs the
// generator.
//
// It never uses the server's key to write an event: only the companion's
// key relay, for a player who left "Make the world livelier" ticked on the
// key page (this feature lends under apiframework.PurposeRoomLife, with its
// own breaker), against that player's own daily allowance. The server's
// key, when there is one, only moderates. The call, the turns and the
// moderation are internal/lively's, shared with every lively feature.
package roomlife

import (
	"context"
	"fmt"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/lively"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/plugins"
	"github.com/GoMudEngine/GoMud/internal/roomlife"
)

// feature is this module's own counters on a player's key.
var feature = lively.Feature{
	Name:              `roomlife`,
	Purpose:           apiframework.PurposeRoomLife,
	Consumer:          apiframework.ConsumerRoomLife,
	Dim:               apiframework.DimRoomLifeKeyholder,
	ModerationBreaker: `roomlife-moderation`,
}

// RoomLifeModule is the module's state. cfg is read by delivery goroutines
// behind mu.
type RoomLifeModule struct {
	plug *plugins.Plugin

	mu  sync.Mutex
	cfg Config

	turns *lively.Turns
}

var module RoomLifeModule

func init() {
	module = RoomLifeModule{
		plug:  plugins.New(`roomlife`, `0.1.0`),
		turns: lively.NewTurns(),
	}
	module.plug.Callbacks.SetOnLoad(module.onLoad)
	module.plug.Callbacks.SetOnSave(module.onSave)
}

func (m *RoomLifeModule) get(k string) any { return m.plug.Config.Get(k) }

func (m *RoomLifeModule) onLoad() {
	m.configure(buildConfig(m.get))
	events.RegisterListener(events.NewRound{}, m.onNewRound)
}

// onNewRound re-reads the server key's settings and the live settings on
// the game loop, so a `server set` of Chance, MinSecondsPerPlayer or
// DailyTokensPerUser reaches the next round.
func (m *RoomLifeModule) onNewRound(e events.Event) events.ListenerReturn {
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

func (m *RoomLifeModule) onSave() error {
	if m.snapshot().Enabled {
		apiframework.SaveBudget()
	}
	return nil
}

// configure applies a config and installs (or removes) the generator.
func (m *RoomLifeModule) configure(cfg Config) {
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
	if !cfg.Enabled {
		roomlife.SetGenerator(nil)
		mudlog.Info(`roomlife`, `ambient events`, `off`, `reason`, `Modules.roomlife.Enabled is false`)
		return
	}
	roomlife.SetGenerator(m)
	s := apiframework.RefreshServer()
	mudlog.Info(`roomlife`, `ambient events`, `on players' own keys`, `chance`, cfg.Chance,
		`moderated`, cfg.ModerateOutput && s.HasKey())
}

func (m *RoomLifeModule) snapshot() Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg
}

// Chance is the percent of due ambient lines tried as events.
func (m *RoomLifeModule) Chance() int {
	c := m.snapshot()
	if !c.Enabled {
		return 0
	}
	return c.Chance
}

// Reserve takes userId's turn when their key may write an event now.
func (m *RoomLifeModule) Reserve(userId int) bool {
	cfg := m.snapshot()
	return cfg.Enabled && m.turns.Reserve(feature, cfg.limits(), userId)
}

// Generate writes the event on userId's key and gives their turn back. Off
// the mud lock.
func (m *RoomLifeModule) Generate(ctx context.Context, userId int, req roomlife.Request) (roomlife.Result, error) {
	defer m.turns.Release(userId)
	cfg := m.snapshot()
	chat := apiframework.Chat{
		Messages:   buildMessages(req),
		SchemaName: roomlife.ReplySchemaName,
		Schema:     roomlife.ReplySchema(),
		MaxTokens:  cfg.MaxCompletionTokens,
	}
	var res roomlife.Result
	decode := func(content string) error {
		parsed, err := roomlife.ParseReply(content)
		if err != nil {
			return err // the schema ignored: the key's failure
		}
		if res, err = roomlife.CleanResult(parsed, req.CanSee); err != nil {
			return fmt.Errorf(`%w: %w`, lively.ErrUnusable, err)
		}
		return nil
	}
	_, tokens, err := lively.Ask(ctx, feature, cfg.limits(), m.turns.Relay(), userId, chat, decode)
	if cfg.LogRequests {
		mudlog.Info(`roomlife`, `action`, `model call`, `room`, req.Title, `canSee`, req.CanSee,
			`prompt`, PromptVersion, `tokens`, tokens, `error`, errString(err))
	}
	if err != nil {
		return roomlife.Result{}, err
	}
	keyholderOnly, err := lively.Moderate(feature, cfg.limits(), res.Text)
	if err != nil {
		return roomlife.Result{}, err
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
