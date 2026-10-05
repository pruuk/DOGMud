package npcidle

import (
	"github.com/GoMudEngine/GoMud/internal/lively"
)

// Config is the resolved Modules.npcidle configuration. Values arrive
// through the plugin config bag as `any` (lively's readers).
//
// There is no server key route: a moment is only ever written on the key of
// a player in the room, through the companion's key relay. The server's key
// is used only to moderate a moment before others read it (ModerateOutput).
type Config struct {
	Enabled bool

	// Chance is the percent (0 to 100) of an NPC's idle emotes and says,
	// with a player who allows it in the room, that are written fresh.
	Chance int

	// MinSecondsPerPlayer spaces the moments one player's key writes: after
	// one starts, none other starts on that key for this long.
	MinSecondsPerPlayer int

	TimeoutSeconds      int
	MaxCompletionTokens int

	// DailyTokensPerUser is what one player's key may spend on moments in a
	// UTC day (apiframework's npcidle.keyholder allowance). 0 is no cap.
	DailyTokensPerUser int

	ModerateOutput  bool
	ModerationModel string
	LogRequests     bool
}

type getter = lively.Getter

// The settings read live every round (onNewRound) as well as at load, so a
// `server set` of them reaches the next idle tick without a restart.
func chance(get getter) int              { return lively.Int(get(`Chance`), 5, 0, 100) }
func minSecondsPerPlayer(get getter) int { return lively.Int(get(`MinSecondsPerPlayer`), 30, 0, 3600) }
func dailyTokensPerUser(get getter) int {
	return lively.Int(get(`DailyTokensPerUser`), 20000, 0, 1<<31-1)
}

// buildConfig resolves the config with safe defaults and bounds.
//
// Enabled defaults to TRUE: nothing is spent unless a player has their own
// key up in the companion's key relay and left "Make the world livelier"
// ticked, and then only on their key.
func buildConfig(get getter) Config {
	if get == nil {
		get = func(string) any { return nil }
	}
	c := Config{
		Enabled:             lively.Bool(get(`Enabled`), true),
		Chance:              chance(get),
		MinSecondsPerPlayer: minSecondsPerPlayer(get),
		TimeoutSeconds:      lively.Int(get(`TimeoutSeconds`), 20, 3, 40),
		MaxCompletionTokens: lively.Int(get(`MaxCompletionTokens`), 600, 100, 2000),
		DailyTokensPerUser:  dailyTokensPerUser(get),
		ModerateOutput:      lively.Bool(get(`ModerateOutput`), true),
		ModerationModel:     lively.String(get(`ModerationModel`)),
		LogRequests:         lively.Bool(get(`LogRequests`), false),
	}
	if c.ModerationModel == `` {
		c.ModerationModel = `omni-moderation-latest`
	}
	return c
}

// limits is what lively needs for a call.
func (c Config) limits() lively.Limits {
	return lively.Limits{
		MinSecondsPerPlayer: c.MinSecondsPerPlayer,
		DailyTokensPerUser:  c.DailyTokensPerUser,
		TimeoutSeconds:      c.TimeoutSeconds,
		MaxCompletionTokens: c.MaxCompletionTokens,
		ModerateOutput:      c.ModerateOutput,
		ModerationModel:     c.ModerationModel,
	}
}
