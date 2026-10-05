package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/merchantchests"
)

// MerchantChestRestock empties and restocks every merchant chest whose
// restock interval has run out (internal/merchantchests). Tick throttles
// itself, so this is cheap on the rounds it does nothing.
func MerchantChestRestock(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.NewRound)
	if !ok {
		return events.Continue
	}
	merchantchests.Tick(evt.RoundNumber)
	return events.Continue
}
