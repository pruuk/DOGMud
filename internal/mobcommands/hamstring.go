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

// hamstringAttack is what a defence line calls the move ("sweeps aside the
// fumbled low lunge"). It is the move as the reader sees it in the store's
// own lines ("lunges low and tears into your legs"), never the command name:
// "hamstring slash" read as an internal label beside a rat's bite (#631).
const hamstringAttack = "low lunge"

// Hamstring is a wolf physical attack that applies a bleed condition.
func Hamstring(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	if !mob.Character.IsInCombat() {
		return true, nil
	}

	res := actions.ExecuteHamstring(&actions.MobActor{Mob: mob, Room: room})
	if res.Cost.Status == characters.CostRefused {
		return true, nil
	}

	if res.OnCooldown || res.NoTarget || !res.Executed {
		return true, nil
	}

	target := res.Target
	result := res.MoveResult

	mobName := mob.Character.Name

	// Look up target player record: needed for the actee recipient and for the
	// defence-triad call sites below. SendTrio hides names by sight itself, so
	// there is no darkness branch here.
	var targetUser *users.UserRecord
	if target.UserId > 0 {
		targetUser = users.GetByUserId(target.UserId)
	}

	dmgDesc := combat.GetDamageDescription(result.Damage, result.TargetMaxHP)

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

	damageTokens := map[string]string{movenarration.TokenDamage: dmgDesc}

	if result.Hit {
		sendMoveEvent("hamstring", "hit", ids, aud, messaging.CategoryHitNaturalSharp, damageTokens)
	} else if result.Damage > 0 {
		// Defended-partial: every role reads the partial tier (sendPartialMove).
		defence, _ := moveDefenceLines(mob, room, target, result.Defence, hamstringAttack)
		sendPartialMove("hamstring", "partial", ids, aud, messaging.CategoryHitNaturalSharp, damageTokens, defence.Shortage)
	} else if defence, defended := moveDefenceLines(mob, room, target, result.Defence, hamstringAttack); defended {
		// A defence stopped it outright: the defender's line and the room line
		// both come from the triad.
		sendMoveDefenceShortage(messaging.Say(messaging.CategoryHitNaturalSharp, defence.Shortage), aud)
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    acteeDefenceLine(targetUser, room, messaging.CategoryHitNaturalSharp, defence.ToDefender, mobName),
			Observer: messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToRoom),
		}, aud)
	} else {
		sendMoveEvent("hamstring", "miss", ids, aud, messaging.CategoryHitNaturalSharp, nil)
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(&actions.MobActor{Mob: mob, Room: room}, res.Counter)

	return true, nil
}
