package hooks

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// idleRollInput gathers the multipliers for one mob's idle round (#317): the
// two global scales, the zone of the room the mob stands in (an instance
// clone keeps its template zone), and the mob's own wandermultiplier.
func idleRollInput(mob *mobs.Mob, room *rooms.Room) mobs.IdleRollInput {
	b := configs.GetBalanceConfig()
	in := mobs.IdleRollInput{
		ChatterScale:  float64(b.MobIdleChatterScale),
		MovementScale: float64(b.MobIdleMovementScale),
		ZoneAmbient:   1,
		ZoneWander:    1,
		MobWander:     mob.WanderScale(),
	}
	if room != nil {
		in.ZoneAmbient = rooms.ZoneAmbientMultiplier(room.Zone)
		in.ZoneWander = rooms.ZoneWanderMultiplier(room.Zone)
	}
	return in
}

// issueIdleChatter queues an idle chatter command. An emote or say is an
// ambient line: it goes out whole (a ';' inside the text is kept) through
// the `ambient` mob command, so the listener budget applies. Any other
// chatter verb (consume, cast...) is queued as before.
func issueIdleChatter(mob *mobs.Mob, cmd string) {
	switch mobs.IdleVerb(cmd) {
	case `emote`, `say`:
		mob.CommandNoSplit(`ambient ` + string(ambient.SourceMobChatter) + ` ` + strings.TrimSpace(cmd))
	default:
		mob.Command(cmd)
	}
}
