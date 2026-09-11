package mobcommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Maul is a fanged beast attack that deals heavy damage and applies a strong
// bleed condition.
func Maul(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Must be in combat to use maul; silently skip if not in combat.
	if !mob.Character.IsInCombat() {
		return true, nil
	}

	res := actions.ExecuteMaul(&actions.MobActor{Mob: mob, Room: room})
	if res.Cost.Status == characters.CostRefused {
		return true, nil
	}

	// Anatomy/identity refusal: silently swallow so the btree can fall through.
	if res.NotFanged {
		return true, nil
	}
	// Any other early-exit condition (OnCooldown, NoTarget): silently return.
	if !res.Executed {
		return true, nil
	}

	// Format and send darkness-aware messages.
	target := res.Target
	result := res.MoveResult
	mobName := mob.Character.Name
	dmgDesc := combat.GetDamageDescription(result.Damage, result.TargetMaxHP)

	// Look up target player record for darkness-aware personal messaging.
	var targetUser *users.UserRecord
	if target.UserId > 0 {
		targetUser = users.GetByUserId(target.UserId)
	}
	canSee := targetUser == nil || canSeeInDark(targetUser, room)

	// Declared as the interface and left unset when the target is not a player.
	// Assigning a typed-nil *users.UserRecord would make it a non-nil interface
	// value. There is no Actor: a mob has no client.
	var acteeRecipient messaging.Recipient
	if targetUser != nil {
		acteeRecipient = targetUser
	}
	aud := messaging.Audience{
		ActorName: mobName,
		Actee:     acteeRecipient,
		ActeeId:   target.UserId,
		ActeeName: target.Name,
		Room:      room,
	}

	if result.Hit {
		hitActee := messaging.NoLine
		if targetUser != nil {
			if canSee {
				hitActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> savages you with vicious fangs, tearing bleeding wounds! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
			} else {
				hitActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`Something savages you with vicious fangs, tearing bleeding wounds! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
			}
		}
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.NoLine,
			Actee: hitActee,
			Observer: messaging.Say(messaging.CategoryHitNaturalSharp,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> savages <ansi fg="username">%s</ansi> with vicious fangs!`, mobName, target.Name)),
		}, aud)
	} else if result.Damage > 0 {
		partialActee := messaging.NoLine
		if targetUser != nil {
			if canSee {
				partialActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> snaps at you and you dodge most of it, but the fangs still tear you! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
			} else {
				partialActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`Something snaps at you and you dodge most of it, but the fangs still tear you! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
			}
		}
		defence, defended := moveDefenceLines(mob, room, target, result.Defence, "savage bite")
		partialObserver := messaging.Say(messaging.CategoryHitNaturalSharp,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> snaps its fangs at <ansi fg="username">%s</ansi>, who twists mostly free but still gets torn!`, mobName, target.Name))
		if defended {
			partialObserver = messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToRoom)
			sendMoveDefenceShortage(targetUser, defence)
		}
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    partialActee,
			Observer: partialObserver,
		}, aud)
	} else if defence, defended := moveDefenceLines(mob, room, target, result.Defence, "savage bite"); defended {
		// A defence stopped it outright: the defender's line and the room line
		// both come from the triad.
		sendMoveDefenceShortage(targetUser, defence)
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    acteeDefenceLine(targetUser, room, messaging.CategoryHitNaturalSharp, defence.ToDefender),
			Observer: messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToRoom),
		}, aud)
	} else {
		missActee := messaging.NoLine
		if targetUser != nil {
			if canSee {
				missActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> snaps its fangs at you, but misses!`, mobName))
			} else {
				missActee = messaging.Say(messaging.CategoryHitNaturalSharp, `Something snaps its fangs at you, but misses!`)
			}
		}
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.NoLine,
			Actee: missActee,
			Observer: messaging.Say(messaging.CategoryHitNaturalSharp,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> snaps its fangs at <ansi fg="username">%s</ansi>, but misses!`, mobName, target.Name)),
		}, aud)
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(&actions.MobActor{Mob: mob, Room: room}, res.Counter)

	return true, nil
}
