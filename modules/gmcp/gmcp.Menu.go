package gmcp

import (
	"sync"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Char.Menu tells the client whether a game menu or prompt is open, so the
// web client queues ticks and triggers until it closes (#375). Telnet
// clients with GMCP can read the same signal in their own scripts.
func init() {
	events.RegisterListener(events.MenuChanged{}, menuChangedHandler)
	events.RegisterListener(events.PlayerSpawn{}, menuPlayerSpawnHandler)
	events.RegisterListener(events.NewRound{}, menuNewRoundHandler)
}

// menuSpawnPending holds users who just spawned, for a one-round re-push.
var (
	menuSpawnPending   = map[int]struct{}{}
	menuSpawnPendingMu sync.Mutex
)

// A reconnect reuses the user record, so an open menu or prompt survives it
// but the fresh client starts unpaused. Tell it once at spawn and once next
// round (a web client's GMCP hello can race the login burst). Only "open" is
// sent: a fresh client already assumes closed.
func menuPlayerSpawnHandler(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.PlayerSpawn)
	if !ok || evt.UserId == 0 {
		return events.Continue
	}
	sendMenuIfOpen(evt.UserId)
	menuSpawnPendingMu.Lock()
	menuSpawnPending[evt.UserId] = struct{}{}
	menuSpawnPendingMu.Unlock()
	return events.Continue
}

func menuNewRoundHandler(e events.Event) events.ListenerReturn {
	if _, ok := e.(events.NewRound); !ok {
		return events.Continue
	}
	menuSpawnPendingMu.Lock()
	if len(menuSpawnPending) == 0 {
		menuSpawnPendingMu.Unlock()
		return events.Continue
	}
	pending := menuSpawnPending
	menuSpawnPending = map[int]struct{}{}
	menuSpawnPendingMu.Unlock()
	for userId := range pending {
		sendMenuIfOpen(userId)
	}
	return events.Continue
}

func sendMenuIfOpen(userId int) {
	user := users.GetByUserId(userId)
	if user == nil || !user.MenuOrPromptOpen() || !isGMCPEnabled(user.ConnectionId()) {
		return
	}
	events.AddToQueue(GMCPOut{
		UserId:  userId,
		Module:  `Char.Menu`,
		Payload: map[string]bool{"open": true},
	})
}

func menuChangedHandler(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.MenuChanged)
	if !ok || evt.UserId == 0 {
		return events.Continue
	}
	user := users.GetByUserId(evt.UserId)
	if user == nil || !isGMCPEnabled(user.ConnectionId()) {
		return events.Continue
	}
	// Priority -1 (lower pops first): internal/users queues MenuChanged and then
	// the menu text, so a default-priority frame would land after the text and a
	// client trigger matching the text could fire before it knows a menu is open.
	events.AddToQueue(GMCPOut{
		UserId:  evt.UserId,
		Module:  `Char.Menu`,
		Payload: map[string]bool{"open": evt.Open},
	}, -1)
	return events.Continue
}
