package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Wake wakes a sleeper in the room by name (owner 2026-11-07): `wake marn`.
// Bare `wake` has nothing left to do: a sleeping player is woken by the
// dispatcher before any command but sit and rest runs (keepsSleep).
//
// The name aims by sight (#454), as for consider: a typed name at clear
// sight, `figure` or `2.figure` at shapes, nothing with no sight. Every line
// names each party by its reader's sight (messaging.SendTrio), and waking
// someone is noisy, so it reveals a hidden waker as shout does.
func Wake(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	name := strings.TrimSpace(rest)
	if name == `` || room == nil {
		return true, nil
	}
	name, aimRefusal := actions.AimBySight(user.Character, user.UserId, room, name, `wake`)
	if aimRefusal != `` {
		user.SendText(messaging.CategoryRefusal, aimRefusal)
		return true, nil
	}
	sight := messaging.ParticipantSight(user.Character, room)
	playerId, mobId := room.FindByNameSeenBy(user.Character, name)
	if mobId > 0 {
		if m := mobs.GetInstance(mobId); m != nil {
			if !m.Character.HasConditionFlag(conditions.Sleeping) {
				wakeNotAsleep(user, `mobname`, m.Character.Name, sight)
				return true, nil
			}
			wakeIsNoisy(user.Character)
			m.Character.CancelConditionsWithFlag(conditions.Sleeping)
			mobs.OnSleeperWoken(&m.Character)
			if keeperInOwnShop(m) {
				relightLantern(m, room) // a keeper's lantern went dark while they slept
			}
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.Say(messaging.CategoryMobEmote, fmt.Sprintf(`You wake <ansi fg="mobname">%s</ansi>.`, m.Character.Name)),
				Actee:    messaging.NoLine, // a mob has no client
				Observer: messaging.Say(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> wakes <ansi fg="mobname">%s</ansi>.`, user.Character.Name, m.Character.Name)),
			}, messaging.Audience{
				Actor: user, ActorId: user.UserId, ActorName: user.Character.Name,
				ActeeName: m.Character.Name, Room: room,
			})
			return true, nil
		}
	}
	if playerId > 0 && playerId != user.UserId {
		if other := users.GetByUserId(playerId); other != nil {
			if !other.Character.HasConditionFlag(conditions.Sleeping) {
				wakeNotAsleep(user, `username`, other.Character.Name, sight)
				return true, nil
			}
			wakeIsNoisy(user.Character)
			// Woken before the lines go out, so the sleeper's own line is
			// judged at their waking sight.
			other.Character.CancelConditionsWithFlag(conditions.Sleeping)
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.Say(messaging.CategoryMobEmote, fmt.Sprintf(`You wake <ansi fg="username">%s</ansi>.`, other.Character.Name)),
				Actee:    messaging.Say(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> wakes you.`, user.Character.Name)),
				Observer: messaging.Say(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> wakes <ansi fg="username">%s</ansi>.`, user.Character.Name, other.Character.Name)),
			}, messaging.Audience{
				Actor: user, ActorId: user.UserId, ActorName: user.Character.Name,
				Actee: other, ActeeId: other.UserId, ActeeName: other.Character.Name,
				Room: room,
			})
			return true, nil
		}
	}
	refusal.NotHereCreature(user)
	return true, nil
}

// wakeNotAsleep refuses a wake aimed at someone awake, naming them only as
// the player makes them out (a figure at shapes).
func wakeNotAsleep(user *users.UserRecord, tag, name string, sight messaging.SightDecision) {
	line := fmt.Sprintf(`<ansi fg="%s">%s</ansi> is not asleep.`, tag, name)
	user.SendText(messaging.CategoryRefusal, messaging.HideNames(line, []string{name}, sight))
}

// wakeIsNoisy reveals a hidden waker: waking someone is a noisy action, as
// shouting is (actions.Shout).
func wakeIsNoisy(c *characters.Character) {
	if c.IsHidden() {
		_ = c.Awareness.TransitionToRevealing(state.TransitionReason{
			Trigger:  awareness.TriggerNoisyAction,
			Metadata: map[string]any{"command": "wake"},
		})
	}
}
