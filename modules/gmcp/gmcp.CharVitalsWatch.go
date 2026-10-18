package gmcp

import (
	"sync"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Char.Vitals on change.
//
// Char.Vitals used to go out only on events.CharacterVitalsChanged, and most
// pool changes never queue one: combat damage, a spell's stamina or
// conviction cost, a reservation starting or ending, and stamina and
// conviction regen (the round regen hook only compares health). The web
// client's Vitals window then froze until something unrelated (a move, an
// equipment change) pushed a fresh payload, while Char.Conditions, which
// rides its own per-round events, kept up. Playtest 2026-10-17.
//
// Rather than chase every writer of every pool, the module checks once a
// round: it builds the payload each GMCP client would be sent and, when it
// differs from the last one actually sent to that user, queues
// CharacterVitalsChanged. That one event already fans out to Char.Vitals,
// Party.Vitals (so party members' bars follow too) and Commands.State, and
// it is what every explicit emitter queues, so there is still one path.
// A round with no change sends nothing.

var (
	lastSentVitals   = map[int]GMCPCharModule_Payload_Vitals{}
	lastSentVitalsMu sync.Mutex
)

// buildVitalsPayload is the Char.Vitals snapshot for one character. Shared by
// GetCharNode and the round watcher so the two can never disagree about what
// "changed" means.
func buildVitalsPayload(c *characters.Character) GMCPCharModule_Payload_Vitals {
	return GMCPCharModule_Payload_Vitals{
		Hp:            c.DisplayHealth(),
		HpMax:         c.HealthMax.Value,
		Stamina:       c.Stamina,
		StaminaMax:    c.StaminaMax.Value,
		Conviction:    c.Conviction,
		ConvictionMax: c.ConvictionMax.Value,

		HpReserved:         c.GetPoolReservation("health", c.HealthMax.Value),
		StaminaReserved:    c.GetPoolReservation("stamina", c.StaminaMax.Value),
		ConvictionReserved: c.GetPoolReservation("conviction", c.ConvictionMax.Value),

		Toxicity: c.ToxicityBandName(),
	}
}

// recordSentVitals notes the Char.Vitals payload just sent to a user.
func recordSentVitals(userId int, v GMCPCharModule_Payload_Vitals) {
	lastSentVitalsMu.Lock()
	lastSentVitals[userId] = v
	lastSentVitalsMu.Unlock()
}

// forgetSentVitals drops a user's record, so a later login starts clean.
func forgetSentVitals(userId int) {
	lastSentVitalsMu.Lock()
	delete(lastSentVitals, userId)
	lastSentVitalsMu.Unlock()
}

// vitalsDue reports whether a user's current vitals differ from the last
// Char.Vitals sent to them. A user never sent one is due.
func vitalsDue(u *users.UserRecord) bool {
	now := buildVitalsPayload(u.Character)
	lastSentVitalsMu.Lock()
	last, ok := lastSentVitals[u.UserId]
	lastSentVitalsMu.Unlock()
	return !ok || last != now
}

// vitalsWatchHandler runs each round and queues CharacterVitalsChanged for
// every GMCP client whose vitals moved since the last push. Telnet users
// without GMCP cost one map lookup and nothing else.
func (g *GMCPCharModule) vitalsWatchHandler(e events.Event) events.ListenerReturn {

	if _, typeOk := e.(events.NewRound); !typeOk {
		return events.Continue
	}

	for _, u := range users.GetAllActiveUsers() {
		if u.UserId == 0 || u.Character == nil || !isGMCPEnabled(u.ConnectionId()) {
			continue
		}
		if vitalsDue(u) {
			events.AddToQueue(events.CharacterVitalsChanged{UserId: u.UserId})
		}
	}

	return events.Continue
}

func (g *GMCPCharModule) vitalsWatchDespawnHandler(e events.Event) events.ListenerReturn {
	if evt, ok := e.(events.PlayerDespawn); ok && evt.UserId > 0 {
		forgetSentVitals(evt.UserId)
	}
	return events.Continue
}
