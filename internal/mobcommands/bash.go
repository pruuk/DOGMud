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
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Bash(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Must be in combat to use bash; silently skip if not in combat.
	if !mob.Character.IsInCombat() {
		return true, nil
	}

	// Delegate core bash logic to the shared action.
	bashResult := actions.ExecuteBash(&actions.MobActor{Mob: mob, Room: room})
	if bashResult.Cost.Status == characters.CostRefused {
		return true, nil
	}

	// Any early-exit condition: silently return.
	if !bashResult.Executed {
		return true, nil
	}

	// Format and send darkness-aware messages.
	target := bashResult.Target
	result := bashResult.MoveResult
	mobName := mob.Character.Name
	dmgDesc := combat.GetDamageDescription(result.Damage, result.TargetMaxHP)

	// Look up target player record: needed for the actee recipient and for the
	// defence-triad call sites below. SendTrio hides names by sight itself, so
	// there is no darkness branch here.
	var targetUser *users.UserRecord
	if target.UserId > 0 {
		targetUser = users.GetByUserId(target.UserId)
	}

	// Natural bashers (elementals, golems) slam instead of shield-bashing.
	bashLabel := "shield bash"
	bashVerb := "bashes"
	bashWith := "with their shield"
	if sp := species.GetSpecies(mob.Character.SpeciesId); sp != nil && sp.NaturalBash {
		bashLabel = "crushing slam"
		bashVerb = "slams into"
		bashWith = "with tremendous force"
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

	tokens := map[string]string{
		movenarration.TokenDamage: dmgDesc,
		movenarration.TokenLabel:  bashLabel,
		movenarration.TokenVerb:   bashVerb,
		movenarration.TokenWith:   bashWith,
	}

	if result.Hit {
		if result.KnockedDown {
			sendMoveEvent("bash", "knockdown", ids, aud, messaging.CategoryBash, tokens)
		} else {
			sendMoveEvent("bash", "hit", ids, aud, messaging.CategoryBash, tokens)
		}
	} else if result.Damage > 0 {
		// Defended-partial: the actee line still carries the damage from the
		// store; the room line names the defence that blunted the bash, so it
		// is swapped for the defence triad's ToRoom text when a defence
		// actually fired.
		roles, _ := renderMoveEvent("bash", "partial", ids, tokens)
		defence, defended := moveDefenceLines(mob, room, target, result.Defence, bashLabel)
		partialObserver := lineOrNone(messaging.CategoryBash, roles.Observer)
		if defended {
			partialObserver = messaging.Say(messaging.CategoryBash, defence.ToRoom)
			sendMoveDefenceShortage(messaging.Say(messaging.CategoryBash, defence.Shortage), aud)
		}
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    lineOrNone(messaging.CategoryBash, roles.Actee),
			Observer: partialObserver,
		}, aud)
	} else if defence, defended := moveDefenceLines(mob, room, target, result.Defence, bashLabel); defended {
		// A defence stopped it outright: the defender's line and the room line
		// both come from the triad.
		sendMoveDefenceShortage(messaging.Say(messaging.CategoryBash, defence.Shortage), aud)
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    acteeDefenceLine(targetUser, room, messaging.CategoryBash, defence.ToDefender, mobName),
			Observer: messaging.Say(messaging.CategoryBash, defence.ToRoom),
		}, aud)
	} else {
		sendMoveEvent("bash", "miss", ids, aud, messaging.CategoryBash, tokens)
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(&actions.MobActor{Mob: mob, Room: room}, bashResult.Counter)

	return true, nil
}
