package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/tips"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// BroadcastTips sends the next gameplay tip (internal/tips) to every active
// player who is not deafened and has not turned tips off with `set tips`,
// every Balance.TipIntervalRounds rounds (75 is about 5 minutes at 4-second
// rounds; 0 turns tips off, #317).

const tipsConfigKey = `tips`

func BroadcastTips(e events.Event) events.ListenerReturn {
	evt := e.(events.NewRound)

	interval := uint64(configs.GetBalanceConfig().TipIntervalRounds)
	if interval == 0 || evt.RoundNumber%interval != 0 {
		return events.Continue
	}

	tip := tips.Next()
	if tip == `` {
		return events.Continue
	}

	fullText := `<ansi fg="cyan-bold">[Tip]</ansi> ` + tip

	for _, u := range users.GetAllActiveUsers() {
		if u.Deafened {
			continue
		}
		// Tips default to ON (nil = on).
		if on, ok := u.GetConfigOption(tipsConfigKey).(bool); ok && !on {
			continue
		}
		u.SendText(messaging.CategoryTip, fullText)

		// Also fire a Communication event for the web client's comms window,
		// aimed at this player alone so the opt-out above holds there too
		// (#496): an untargeted broadcast reaches every online user.
		events.AddToQueue(events.Communication{
			TargetUserId: u.UserId,
			CommType:     "broadcast",
			Name:         "Tip",
			Message:      tip,
		})
	}

	return events.Continue
}
