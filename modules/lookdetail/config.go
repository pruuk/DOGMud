package lookdetail

import (
	"github.com/GoMudEngine/GoMud/internal/lively"
)

// Config is the resolved Modules.lookdetail configuration.
//
// There is no server key route: a closer look is only ever written on the
// looker's own key, through the companion's key relay. The server's key is
// used only to moderate a detail before it is cached for others.
type Config struct {
	Enabled bool

	// MinSecondsPerPlayer spaces the details one player's key writes. A look
	// answered from the cache costs nothing and is not spaced.
	MinSecondsPerPlayer int

	TimeoutSeconds      int
	MaxCompletionTokens int

	// DailyTokensPerUser is what one player's key may spend on details in a
	// UTC day (apiframework's lookdetail.keyholder allowance). 0 is no cap.
	DailyTokensPerUser int

	ModerateOutput  bool
	ModerationModel string
	LogRequests     bool
}

type getter = lively.Getter

// The settings read live every round (onNewRound) as well as at load.
func minSecondsPerPlayer(get getter) int { return lively.Int(get(`MinSecondsPerPlayer`), 5, 0, 3600) }
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
		MinSecondsPerPlayer: minSecondsPerPlayer(get),
		TimeoutSeconds:      lively.Int(get(`TimeoutSeconds`), 20, 3, 40),
		MaxCompletionTokens: lively.Int(get(`MaxCompletionTokens`), 800, 100, 2000),
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
