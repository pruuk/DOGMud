// Package lookdetail is the model side of generated closer looks
// (internal/lookdetail): when a player looks at something the room's
// description names but nothing in the room answers to, a model writes what
// a closer look shows, on the looker's own key. This module installs the
// generator.
//
// It never uses the server's key to write a detail: only the companion's
// key relay, for a player who left "Make the world livelier" ticked on the
// key page (this feature lends under apiframework.PurposeLookDetail, with
// its own breaker), against that player's own daily allowance. The server's
// key, when there is one, only moderates. The call, the turns and the
// moderation are internal/lively's, shared with every lively feature.
package lookdetail

import (
	"context"
	"fmt"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/lively"
	"github.com/GoMudEngine/GoMud/internal/lookdetail"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/plugins"
)

// feature is this module's own counters on a player's key.
var feature = lively.Feature{
	Name:              `lookdetail`,
	Purpose:           apiframework.PurposeLookDetail,
	Consumer:          apiframework.ConsumerLookDetail,
	Dim:               apiframework.DimLookDetailKeyholder,
	ModerationBreaker: `lookdetail-moderation`,
}

// LookDetailModule is the module's state. cfg is read by delivery
// goroutines behind mu.
type LookDetailModule struct {
	plug *plugins.Plugin

	mu  sync.Mutex
	cfg Config

	turns *lively.Turns
}

var module LookDetailModule

func init() {
	module = LookDetailModule{
		plug:  plugins.New(`lookdetail`, `0.1.0`),
		turns: lively.NewTurns(),
	}
	module.plug.Callbacks.SetOnLoad(module.onLoad)
	module.plug.Callbacks.SetOnSave(module.onSave)
}

func (m *LookDetailModule) get(k string) any { return m.plug.Config.Get(k) }

func (m *LookDetailModule) onLoad() {
	m.configure(buildConfig(m.get))
	events.RegisterListener(events.NewRound{}, m.onNewRound)
}

// onNewRound re-reads the server key's settings and the live settings on
// the game loop.
func (m *LookDetailModule) onNewRound(e events.Event) events.ListenerReturn {
	if !m.snapshot().Enabled {
		return events.Continue
	}
	apiframework.RefreshServer()
	if m.plug != nil {
		gap, perUser := minSecondsPerPlayer(m.get), dailyTokensPerUser(m.get)
		m.mu.Lock()
		m.cfg.MinSecondsPerPlayer, m.cfg.DailyTokensPerUser = gap, perUser
		m.mu.Unlock()
	}
	return events.Continue
}

func (m *LookDetailModule) onSave() error {
	if m.snapshot().Enabled {
		apiframework.SaveBudget()
	}
	return nil
}

// configure applies a config and installs (or removes) the generator.
func (m *LookDetailModule) configure(cfg Config) {
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
	if !cfg.Enabled {
		lookdetail.SetGenerator(nil)
		mudlog.Info(`lookdetail`, `closer looks`, `off`, `reason`, `Modules.lookdetail.Enabled is false`)
		return
	}
	lookdetail.SetGenerator(m)
	s := apiframework.RefreshServer()
	mudlog.Info(`lookdetail`, `closer looks`, `on players' own keys`, `moderated`, cfg.ModerateOutput && s.HasKey())
}

func (m *LookDetailModule) snapshot() Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg
}

// Reserve takes userId's turn when their key may write a detail now.
func (m *LookDetailModule) Reserve(userId int) bool {
	cfg := m.snapshot()
	return cfg.Enabled && m.turns.Reserve(feature, cfg.limits(), userId)
}

// Generate writes the detail on userId's key and gives their turn back.
// Off the mud lock.
func (m *LookDetailModule) Generate(ctx context.Context, userId int, req lookdetail.Request) (lookdetail.Result, error) {
	defer m.turns.Release(userId)
	cfg := m.snapshot()
	chat := apiframework.Chat{
		Messages:   buildMessages(req),
		SchemaName: lookdetail.ReplySchemaName,
		Schema:     lookdetail.ReplySchema(),
		MaxTokens:  cfg.MaxCompletionTokens,
	}
	var res lookdetail.Result
	decode := func(content string) error {
		parsed, err := lookdetail.ParseReply(content)
		if err != nil {
			return err // the schema ignored: the key's failure
		}
		if res, err = lookdetail.CleanResult(parsed); err != nil {
			return fmt.Errorf(`%w: %w`, lively.ErrUnusable, err)
		}
		return nil
	}
	_, tokens, err := lively.Ask(ctx, feature, cfg.limits(), m.turns.Relay(), userId, chat, decode)
	if cfg.LogRequests {
		mudlog.Info(`lookdetail`, `action`, `model call`, `room`, req.Title, `thing`, req.Thing,
			`prompt`, PromptVersion, `tokens`, tokens, `error`, errString(err))
	}
	if err != nil {
		return lookdetail.Result{}, err
	}
	keyholderOnly, err := lively.Moderate(feature, cfg.limits(), res.Text)
	if err != nil {
		return lookdetail.Result{}, err
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
