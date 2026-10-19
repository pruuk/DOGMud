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

// Smother is the smoke elemental's special move (spec 2026-10-16 #541, Appendix A): smoke pours into the target's eyes and lungs, blinding and choking it, and can choke off a spell.
func Smother(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Must be in combat to smother; silently skip if not in combat.
	if !mob.Character.IsInCombat() {
		return true, nil
	}

	res := actions.ExecuteSmother(&actions.MobActor{Mob: mob, Room: room})
	if res.Cost.Status == characters.CostRefused {
		return true, nil
	}

	// Anatomy/identity refusal: silently swallow so the btree can fall through.
	if res.NotSmoke {
		return true, nil
	}
	// Any other early-exit condition (OnCooldown, NoTarget): silently return.
	if !res.Executed {
		return true, nil
	}

	target := res.Target
	result := res.MoveResult
	mobName := mob.Character.Name
	dmgDesc := combat.GetDamageDescription(result.Damage, result.TargetMaxHP)

	// The target player record is the actee recipient and feeds the defence
	// triad below. SendTrio hides names by sight itself.
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

	// Declared as the interface and left unset when the target is not a player:
	// a typed-nil *users.UserRecord would be a non-nil interface value. There
	// is no Actor: a mob has no client.
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
		// The hit blinds the target, so the actee's lines are judged at the
		// sight they had before the smoke (res.ActeeSightBefore; owner rule
		// 2026-10-05), not at the blindness the hit has just caused.
		if roles, ok := renderMoveEvent("smother", "hit", ids, damageTokens); ok {
			sendMoveRolesJudgedBefore(roles, aud, messaging.CategoryHitNaturalSharp, res.ActeeSightBefore)
		}

		// Cast-interrupt detail line riding on the hit above; an observer who
		// sees nothing hears the chant break off, as for throttle.
		if res.InterruptedCast {
			if roles, ok := renderMoveEvent("smother", "cast_interrupt", ids, nil); ok {
				sendMoveRolesJudgedBefore(roles, aud, messaging.CategorySpellDisruption, res.ActeeSightBefore, messaging.SoundChantBreaksOff)
			}
		}
	} else if result.Damage > 0 {
		// Defended-partial: the actee line carries the damage from the store;
		// the room line names the defence that blunted the smoke when one fired.
		roles, _ := renderMoveEvent("smother", "partial", ids, damageTokens)
		defence, defended := moveDefenceLines(mob, room, target, result.Defence, "rolling smoke")
		partialObserver := lineOrNone(messaging.CategoryHitNaturalSharp, roles.Observer)
		if defended {
			partialObserver = messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToRoom)
			sendMoveDefenceShortage(messaging.Say(messaging.CategoryHitNaturalSharp, defence.Shortage), aud)
		}
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    lineOrNone(messaging.CategoryHitNaturalSharp, roles.Actee),
			Observer: partialObserver,
		}, aud)
	} else if defence, defended := moveDefenceLines(mob, room, target, result.Defence, "rolling smoke"); defended {
		// A defence stopped it outright: the defender's line and the room line
		// both come from the triad.
		sendMoveDefenceShortage(messaging.Say(messaging.CategoryHitNaturalSharp, defence.Shortage), aud)
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    acteeDefenceLine(targetUser, room, messaging.CategoryHitNaturalSharp, defence.ToDefender, mobName),
			Observer: messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToRoom),
		}, aud)
	} else {
		sendMoveEvent("smother", "miss", ids, aud, messaging.CategoryHitNaturalSharp, nil)
	}

	// The counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(&actions.MobActor{Mob: mob, Room: room}, res.Counter)

	return true, nil
}
