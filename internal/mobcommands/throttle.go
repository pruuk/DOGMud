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

// Throttle is a fanged beast attack that clamps the throat, dealing damage,
// applying a stamina-drain DoT, and potentially interrupting an active spellcast.
func Throttle(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Must be in combat to use throttle; silently skip if not in combat.
	if !mob.Character.IsInCombat() {
		return true, nil
	}

	res := actions.ExecuteThrottle(&actions.MobActor{Mob: mob, Room: room})
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

	damageTokens := map[string]string{movenarration.TokenDamage: dmgDesc}

	if result.Hit {
		sendMoveEvent("throttle", "hit", ids, aud, messaging.CategoryHitNaturalSharp, damageTokens)

		// Cast-interrupt detail line riding on the hit above. Mobs and players
		// share one pool (M6 slice 2, ledger row 56), so the room is told of
		// the interrupt as it is for a player's throttle, and an observer who
		// sees nothing hears the chant break off, as from a player's (#242).
		//
		// Not gated on targetUser: the event carries observer and sound lines
		// now, so a mob throttling a casting MOB is narrated to the room as a
		// player doing it is. SendTrio skips the actee line when aud.Actee is
		// unset.
		if res.InterruptedCast {
			sendMoveEvent("throttle", "cast_interrupt", ids, aud, messaging.CategorySpellDisruption, nil, messaging.SoundChantBreaksOff)
		}
	} else if result.Damage > 0 {
		// Defended-partial: every role reads the partial tier (sendPartialMove).
		defence, _ := moveDefenceLines(mob, room, target, result.Defence, "throttle lunge")
		sendPartialMove("throttle", "partial", ids, aud, messaging.CategoryHitNaturalSharp, damageTokens, defence.Shortage)
	} else if defence, defended := moveDefenceLines(mob, room, target, result.Defence, "throttle lunge"); defended {
		// A defence stopped it outright: the defender's line and the room line
		// both come from the triad.
		sendMoveDefenceShortage(messaging.Say(messaging.CategoryHitNaturalSharp, defence.Shortage), aud)
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    acteeDefenceLine(targetUser, room, messaging.CategoryHitNaturalSharp, defence.ToDefender, mobName),
			Observer: messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToRoom),
		}, aud)
	} else {
		sendMoveEvent("throttle", "miss", ids, aud, messaging.CategoryHitNaturalSharp, nil)
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(&actions.MobActor{Mob: mob, Room: room}, res.Counter)

	return true, nil
}
