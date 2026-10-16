package usercommands

import (
	"errors"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/life"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func Suicide(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	if user.Character.Zone == `Shadow Realm` {
		user.SendText(messaging.CategoryRefusal, `You're already dead!`)
		return true, errors.New(`already dead`)
	}

	// Dedupe double-fire: the combat loop can queue `suicide` multiple
	// times before the first one executes (damage from multiple
	// sources in the same round, or rapid re-checks). Skip the second
	// and subsequent fires in the same round to avoid stacking stat
	// decay + skill rust.
	currentRound := util.GetRoundCount()
	if user.Character.LastSuicideRound == currentRound {
		return true, nil
	}
	user.Character.LastSuicideRound = currentRound

	// Revive-on-death condition: heal + clear condition, no death.
	// This path must NEVER reach the Life machine — it keeps the
	// character Alive.
	if user.Character.HasConditionFlag(conditions.ReviveOnDeath) {
		// U5c: this resolves the character's life state without going through
		// Die, so clear the queued-death token here too. Otherwise a
		// CharacterDied still in flight from the blow that brought them here
		// would flush afterwards, find them alive with the revive condition already
		// consumed, and kill them anyway — defeating the condition outright.
		user.Character.DeathQueued = false

		user.Character.Health = user.Character.HealthMax.Value
		user.SendText(messaging.CategoryConditionApply, `You are revived in a shower of magical sparks!`)
		room.SendTextVisual(messaging.CategoryConditionApply,
			`<ansi fg="username">`+user.Character.Name+`</ansi> is suddenly revived in a shower of sparks!`,
			user.UserId,
		)
		user.Character.CancelConditionsWithFlag(conditions.ReviveOnDeath)
		return true, nil
	}

	// Route through the Life cascade. All cleanup, announcements,
	// corpse creation, teleport, decay, and respawn happen via
	// observers wired to the three transitions inside Die().
	user.Character.Die(
		state.ActorRef{UserId: user.UserId},
		life.TriggerSuicide,
	)

	return true, nil
}
