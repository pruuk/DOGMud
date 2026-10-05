package hooks

import (
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// DecayFloors pays the daily floor decay (rooms/floor_decay.go) that loaded
// rooms owe, every FloorDecayCheckRounds rounds. Each room decays at most
// once per real-world day however often this runs; the cadence only decides
// how soon after midnight, or after the last player leaves a room, the pass
// is paid. It replaces the loot goblin as the world's item cleanup outside
// the cities, which their scavengers keep.
func DecayFloors(e events.Event) events.ListenerReturn {
	evt := e.(events.NewRound)

	every := uint64(configs.GetBalanceConfig().FloorDecayCheckRounds)
	if every == 0 || evt.RoundNumber%every != 0 {
		return events.Continue
	}

	if processed, removed := rooms.DecayLoadedFloors(time.Now()); processed > 0 {
		mudlog.Info(`FloorDecay`, `rooms`, processed, `itemsRemoved`, removed)
	}

	return events.Continue
}
