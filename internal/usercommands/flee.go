package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const fleeShortageText = "You break away on instinct rather than technique, too spent to use your training."

// fleeRefusalText is what the player is told for each refusal. Every line is
// unchanged from before slice 4a moved the rules into actions.BeginFlee.
var fleeRefusalText = map[actions.FleeRefusal]string{
	// A no-go root (a Jailed holding cell, 5.1c) pins the player; flee must
	// honour it or it becomes a jail-escape hole (smoke BUG-02).
	actions.FleeRefuseRooted: `You're locked in, with nowhere to flee to.`,
	// Blood Frenzy, hamstrung, winded, tackled: you can fight, not retreat.
	actions.FleeRefuseNoFlee: `You can't break off to flee right now. You can only fight.`,
	// A second flee while the first resolves used to print nothing at all.
	actions.FleeRefuseAlready: `You're already trying to break away. Give it a moment.`,
	// Also rejects a stale queued flee after a lethal round respawned you.
	actions.FleeRefuseNotInCombat: `You're not in combat; there's nothing to flee from.`,
	actions.FleeRefuseGrappled:    `<ansi fg="red">You can't flee while grappled!</ansi>`,
	// Knockdown is common and is exactly when a player wants to run, so say
	// that standing up is what unblocks it.
	actions.FleeRefuseProne:    `<ansi fg="red">You can't flee from the ground. Stand up first!</ansi>`,
	actions.FleeRefuseNotReady: `You can't break away just yet.`,
}

func Flee(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	begin := actions.BeginFlee(actions.NewUserActorInRoom(user, room), "")
	if !begin.Accepted {
		user.SendText(messaging.CategorySystem, fleeRefusalText[begin.Refusal])
		return true, nil
	}
	if begin.Short {
		user.SendText(messaging.CategorySystem, fleeShortageText)
	}
	user.SendText(messaging.CategorySystem, `You attempt to flee...`)
	return true, nil
}
