package mobcommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Kick(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Must be in combat to use kick; silently skip if not in combat.
	if !mob.Character.IsInCombat() {
		return true, nil
	}

	// Delegate core kick logic to the shared action (includes stomp/knee variant
	// detection so mobs now use the appropriate variant automatically).
	res := actions.ExecuteKick(&actions.MobActor{Mob: mob, Room: room})
	if res.Cost.Status == characters.CostRefused {
		return true, nil
	}

	// Any early-exit condition: silently return.
	if !res.Executed {
		return true, nil
	}

	// Format and send darkness-aware messages.
	target := res.Target
	result := res.MoveResult
	mobName := mob.Character.Name
	dmgDesc := combat.GetDamageDescription(result.Damage, result.TargetMaxHP)

	// Look up target player record: needed for the actee recipient and for the
	// defence-triad call sites below. SendTrio hides names by sight itself, so
	// there is no darkness branch here.
	var targetUser *users.UserRecord
	if target.UserId > 0 {
		targetUser = users.GetByUserId(target.UserId)
	}

	ids := moveIdentities{
		Actor:      fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, mobName),
		ActorPlain: mobName,
		Actee:      fmt.Sprintf(`<ansi fg="username">%s</ansi>`, target.Name),
		ActeePlain: target.Name,
	}

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

	// Attack name for the defence triad renderer (U6b Task 9).
	attackName := "kick"
	switch res.Variant {
	case actions.KickStomp:
		attackName = "stomp"
	case actions.KickKnee:
		attackName = "knee strike"
	}

	damageTokens := map[string]string{movenarration.TokenDamage: dmgDesc}

	if result.Hit {
		switch res.Variant {
		case actions.KickStomp:
			sendMoveEvent("kick", "stomp_hit", ids, aud, messaging.CategoryKick, damageTokens)

		case actions.KickKnee:
			sendMoveEvent("kick", "knee_hit", ids, aud, messaging.CategoryKick, damageTokens)

		default: // KickStandard
			if result.KnockedDown {
				sendMoveEvent("kick", "standard_knockdown", ids, aud, messaging.CategoryKick, damageTokens)
			} else {
				sendMoveEvent("kick", "standard_hit", ids, aud, messaging.CategoryKick, damageTokens)
			}
		}
	} else if result.Damage > 0 {
		// Defended-partial: every role reads the partial tier (sendPartialMove).
		var partialEvent movenarration.EventKey
		switch res.Variant {
		case actions.KickStomp:
			partialEvent = "stomp_partial"
		case actions.KickKnee:
			partialEvent = "knee_partial"
		default: // KickStandard
			partialEvent = "standard_partial"
		}
		defence, _ := moveDefenceLines(mob, room, target, result.Defence, attackName)
		sendPartialMove("kick", partialEvent, ids, aud, messaging.CategoryKick, damageTokens, defence.Shortage)
	} else if defence, defended := moveDefenceLines(mob, room, target, result.Defence, attackName); defended {
		// A defence stopped it outright: the defender's line and the room line
		// both come from the triad.
		sendMoveDefenceShortage(messaging.Say(messaging.CategoryKick, defence.Shortage), aud)
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    acteeDefenceLine(targetUser, room, messaging.CategoryKick, defence.ToDefender, mobName),
			Observer: messaging.Say(messaging.CategoryKick, defence.ToRoom),
		}, aud)
	} else {
		switch res.Variant {
		case actions.KickStomp:
			sendMoveEvent("kick", "stomp_miss", ids, aud, messaging.CategoryKick, nil)

		case actions.KickKnee:
			sendMoveEvent("kick", "knee_miss", ids, aud, messaging.CategoryKick, nil)

		default: // KickStandard
			sendMoveEvent("kick", "standard_miss", ids, aud, messaging.CategoryKick, nil)
		}
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(&actions.MobActor{Mob: mob, Room: room}, res.Counter)

	return true, nil
}
