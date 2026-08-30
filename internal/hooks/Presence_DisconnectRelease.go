package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/presence"
	"github.com/GoMudEngine/GoMud/internal/targeting"
)

func init() {
	characters.OnCharacterCreated(wireDisconnectReleasesEngagement)
}

// wireDisconnectReleasesEngagement drops a character's engagement when it
// leaves the world, so it is removed from its target's inbound-attacker list.
//
// ⚠️ Required since combatphase resolves an ActorRef on demand rather than
// holding a registry of Machine pointers. A UserId is STABLE across sessions,
// so a leftover {UserId: N} entry does not go stale-and-nil at logout the way a
// registry pointer did: it resolves again, to the NEW machine belonging to the
// same player when they next log in. recoveryContest would then contest a
// prone mob's stand against a player who is not attacking it.
//
// `quit` is refused in combat (usercommands/quit.go), but link-death and the
// AFK Disconnected timeout (NewRound_PresenceTick.go) are not, so this is
// reachable in ordinary play.
//
// Lives here rather than in users.LogOutUserByConnectionId because that would
// need internal/users to import internal/targeting, which cycles
// (targeting -> mobs/rooms -> users). It is the same shape as
// wireInboundAggroCleanup, which handles the death case.
func wireDisconnectReleasesEngagement(c *characters.Character) {
	if c.Presence == nil {
		return
	}
	c.Presence.RegisterObserver("release_engagement_on_disconnected",
		func(from, to presence.State, r state.TransitionReason) {
			if to != presence.Disconnected {
				return
			}
			targeting.Release(c, targeting.ReasonDisengage)
		})
}
