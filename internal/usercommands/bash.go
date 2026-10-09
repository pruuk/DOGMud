package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// bashCategories: every role rides CategoryBash, the player's own lines
// included, so they wrap and colour like the room's (#449).
//
// Kept on ONE line (rather than the more usual one-field-per-line struct
// literal): send_trio_only_guard_test.go scans line by line for a guarded
// category (Kick/Trip/Bash) paired with a recognised producer shape on that
// SAME line, and moveCategories{...} is one of those shapes.
var bashCategories = moveCategories{Actor: messaging.CategoryBash, Actee: messaging.CategoryBash, Observer: messaging.CategoryBash}

var stageSpecialMoveTarget = actions.StageMeleeTarget

func Bash(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	actor, handled := stageSpecialMoveTarget(user, room, rest, actions.MeleeTargetOpts{
		Verb: "bash",
	})
	if handled {
		return true, nil
	}

	// Delegate core bash logic to the shared action.
	bashResult := actions.ExecuteBash(actor)
	if bashResult.Cost.Status == characters.CostRefused {
		user.SendText(messaging.CategorySystem, actions.CostRefusalText(bashResult.Cost))
		return true, nil
	}

	if bashResult.Crafting {
		// Safety net — should have been caught by the pre-reject above.
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You can't bash while focused on your work. Finish or be interrupted first.</ansi>`)
		return true, nil
	}

	switch {
	case bashResult.NoShield:
		user.SendText(messaging.CategorySystem, "You need a shield equipped to perform a shield bash!")
		return true, nil
	case bashResult.OnCooldown:
		user.SendText(messaging.CategorySystem, "You need a moment to recover before attempting another special move.")
		return true, nil
	case bashResult.NoTarget:
		user.SendText(messaging.CategorySystem, "Your target is gone!")
		return true, nil
	}

	// Format and send messages.
	target := bashResult.Target
	result := bashResult.MoveResult
	dmgDesc := combat.GetDamageDescription(result.Damage, result.TargetMaxHP)

	// Look up the target player record for personal messaging (nil if mob target).
	var targetUser *users.UserRecord
	if target.UserId > 0 {
		targetUser = users.GetByUserId(target.UserId)
	}

	// Declared as the interface and left unset for a mob target. Assigning a
	// typed-nil *users.UserRecord would make it a non-nil interface value.
	var acteeRecipient messaging.Recipient
	if targetUser != nil {
		acteeRecipient = targetUser
	}
	aud := messaging.Audience{
		Actor:     user,
		ActorId:   user.UserId,
		ActorName: user.Character.Name,
		Actee:     acteeRecipient,
		ActeeId:   target.UserId,
		ActeeName: target.Name,
		Room:      room,
	}

	ids := moveIdentities{
		Actor:      fmt.Sprintf(`<ansi fg="username">%s</ansi>`, user.Character.Name),
		ActorPlain: user.Character.Name,
		Actee:      fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, target.Name),
		ActeePlain: target.Name,
	}
	damageTokens := map[string]string{movenarration.TokenDamage: dmgDesc}

	if result.Hit {
		if result.KnockedDown {
			sendMoveEvent("bash", "player_knockdown", ids, aud, bashCategories, damageTokens)
		} else {
			sendMoveEvent("bash", "player_hit", ids, aud, bashCategories, damageTokens)
		}
	} else if result.Damage > 0 {
		// Defended-partial: the personal lines carry the damage, and the room
		// line names the defence that blunted the bash (U6b Task 9), falling
		// back to plain stagger text when there was no defence to name.
		roles, _ := renderMoveEvent("bash", "player_partial", ids, damageTokens)
		defence, defended := moveDefenceLines(user, room, target, result.Defence, "shield bash")
		observer := lineOrNone(messaging.CategoryBash, roles.Observer)
		if defended {
			observer = messaging.Say(messaging.CategoryBash, defence.ToRoom)
			sendMoveDefenceShortage(targetUser, defence)
		}
		messaging.SendTrio(messaging.Trio{
			Actor:    lineOrNone(bashCategories.Actor, roles.Actor),
			Actee:    lineOrNone(bashCategories.Actee, roles.Actee),
			Observer: observer,
		}, aud)
	} else if defence, defended := moveDefenceLines(user, room, target, result.Defence, "shield bash"); defended {
		// A defence stopped it outright: all three lines come from the triad.
		sendMoveDefenceShortage(targetUser, defence)
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.Say(messaging.CategoryBash, defence.ToAttacker),
			Actee:    acteeDefenceLine(targetUser, room, messaging.CategoryBash, defence.ToDefender, user.Character.Name),
			Observer: messaging.Say(messaging.CategoryBash, defence.ToRoom),
		}, aud)
	} else {
		// No defence to narrate (e.g. a fumbled swing): plain miss text.
		sendMoveEvent("bash", "player_miss", ids, aud, bashCategories, nil)
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(actor, bashResult.Counter)

	return true, nil
}
