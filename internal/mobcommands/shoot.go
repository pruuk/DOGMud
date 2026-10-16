package mobcommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Shoot fires a mob's loaded ranged weapon at a target (loaded-weapon model,
// ranged-weapons T6). One shot per command; the weapon unloads on fire. No
// crime/justice recording (that's a player concern); retaliation just aggros
// the target back onto the shooter.
func Fire(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	result := actions.ExecuteFire(&actions.MobActor{Mob: mob, Room: room}, rest)

	// Silent early exits — including CostRefused — because mobs don't narrate
	// their own failed attempts. ExecuteFire keeps every mechanic unchanged.
	if !result.Executed {
		return true, nil
	}

	hit := result.MoveResult.Hit
	partial := !hit && result.MoveResult.Damage > 0
	// Dealt is true on a clean hit AND on a defended shot that still drew
	// blood. A partial draws evidence just like a hit does (the victim's own
	// message names the shooter), so it reveals the same way. Only a true
	// zero-damage miss leaves no trace.
	dealt := hit || result.MoveResult.Damage > 0

	// Cross-room reveal (mirror melee's reveal-on-engage and the player shoot
	// path): a hidden mob whose cross-room shot deals ANY damage (a clean hit
	// or a defended partial) drops stealth. Same-room shots reveal through the
	// combat round handler's CancelCombatConditions once the target is aggroed; a
	// cross-room shooter never enters that loop, so without this it would
	// stay hidden forever. Only a zero-damage cross-room miss stays hidden —
	// the sniper gets exactly one free clean miss, not one free hit.
	if result.CrossRoom && dealt {
		mob.Character.CancelCombatConditions()
	}

	mobName := fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, mob.Character.Name)
	weapon := fmt.Sprintf(`<ansi fg="itemname">%s</ansi>`, result.WeaponName)

	targetColored := fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, result.TargetName)
	if !result.IsTargetMob {
		targetColored = fmt.Sprintf(`<ansi fg="username">%s</ansi>`, result.TargetName)
	}

	ids := moveIdentities{
		Actor:      mobName,
		ActorPlain: mob.Character.Name,
		Actee:      targetColored,
		ActeePlain: result.TargetName,
	}

	// U6b Task 9: a defended same-room shot speaks the channel defence triad
	// (dodge or block). Cross-room shots keep their origin-anonymous arrival
	// lines: the triad names the shooter, which the target's room cannot see.
	var triadDef, triadRoom string
	if !result.CrossRoom && result.MoveResult.Defence.Defended {
		triad := combat.RenderChannelDefenceMessages(result.MoveResult.Defence, combat.ChannelDefenceIdentities{
			Attacker: mobName,
			Defender: targetColored,
		}, "aimed shot")
		triadDef, triadRoom = string(triad.ToDefender), string(triad.ToRoom)
	}

	// Direct line to a player target, built here so the shot reaches all three
	// audiences in one send. The shortage text stays a plain SendText — it is
	// a separate at-most-once mechanical note, not part of the shot's
	// narration.
	var u *users.UserRecord
	if !result.IsTargetMob && result.TargetUserId > 0 {
		u = users.GetByUserId(result.TargetUserId)
	}
	targetLine := messaging.NoLine
	if u != nil {
		if result.MoveResult.Defence.Defended {
			if text := combat.ChannelDefenceShortageText(result.MoveResult.Defence, u.Character); text != "" {
				u.SendText(messaging.CategoryHitRanged, text)
			}
		}
		// Stealth is not darkness. The pipeline hides the shooter's name by the
		// reader's sight; a sneaking shooter is hidden from everyone regardless,
		// so the name never enters the text in the first place.
		actor := mobName
		if result.IsSneaking {
			actor = messaging.StealthActor
		}
		targetIds := ids
		targetIds.Actor = actor
		// Mobs and players share one pool per event (M6 slice 2, ledger row
		// 56); the player variants carry {damage}, so it is filled here too.
		damageTokens := map[string]string{movenarration.TokenDamage: combat.GetDamageDescription(result.MoveResult.Damage, result.MoveResult.TargetMaxHP)}
		switch {
		case hit:
			roles, _ := renderMoveEvent("shoot", "hit", targetIds, damageTokens)
			targetLine = lineOrNone(messaging.CategoryHitRanged, roles.Actee)
		case partial:
			roles, _ := renderMoveEvent("shoot", "partial", targetIds, damageTokens)
			targetLine = lineOrNone(messaging.CategoryHitRanged, roles.Actee)
		case triadDef != "":
			// Not migrated: triadDef is the channel defence triad's own text
			// (combat.RenderChannelDefenceMessages), sourced outside this
			// store. Stealth still applies here, unconditionally, the same
			// way it does above; darkness is handled by SendTrio itself.
			personal := triadDef
			if result.IsSneaking {
				personal = messaging.Anonymize(personal)
			}
			targetLine = messaging.Say(messaging.CategoryHitRanged, personal)
		default:
			roles, _ := renderMoveEvent("shoot", "miss", targetIds, nil)
			targetLine = lineOrNone(messaging.CategoryHitRanged, roles.Actee)
		}
	}

	// Declared as the interface and left unset for a mob target. Assigning a
	// typed-nil *users.UserRecord would make it a non-nil interface value.
	// There is no Actor: a mob has no client.
	var acteeRecipient messaging.Recipient
	if u != nil {
		acteeRecipient = u
	}
	aud := messaging.Audience{
		ActorName: mob.Character.Name,
		Actee:     acteeRecipient,
		ActeeId:   result.TargetUserId,
		ActeeName: result.TargetName,
		Room:      room,
	}

	if !result.CrossRoom {
		sameRoomLine := messaging.NoLine
		if !result.IsSneaking {
			if triadRoom != "" {
				// Not migrated: triadRoom is the channel defence triad's own
				// text, sourced outside this store.
				sameRoomLine = messaging.Say(messaging.CategoryHitRanged, triadRoom)
			} else {
				roles, _ := renderMoveEvent("shoot", "fire_announce", ids, map[string]string{movenarration.TokenWeapon: weapon})
				sameRoomLine = lineOrNone(messaging.CategoryHitRanged, roles.Observer)
			}
		}
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    targetLine,
			Observer: sameRoomLine,
		}, aud)
	} else {
		// Cross-room: TWO rooms with two different exclusion lists, so two
		// sends. The shooter's room sees the shot leave (suppressed when
		// sneaking); the target's room sees it arrive.
		departLine := messaging.NoLine
		if !result.IsSneaking {
			roles, _ := renderMoveEvent("shoot", "fire_depart", ids, map[string]string{
				movenarration.TokenWeapon:   weapon,
				movenarration.TokenExitName: result.ExitName,
			})
			departLine = lineOrNone(messaging.CategoryHitRanged, roles.Observer)
		}
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    targetLine,
			Observer: departLine,
		}, aud)

		// Target's room sees the shot arrive. The player target is excluded
		// -- they already got their own line from the send above -- which is
		// why ActeeId is set here while Actee is not.
		//
		// Six enumerated events, not a composed sentence: the key is picked
		// from two axes, whether an exit back to the shooter is known
		// (tr.FindExitTo) and the outcome, because each of the six is a
		// complete sentence on the remote_observer role rather than an origin
		// fragment plus a verb fragment.
		if tr := rooms.LoadRoom(result.TargetRoomId); tr != nil {
			fromDir := tr.FindExitTo(room.RoomId)
			known := fromDir != ""

			// The six arms are spelled out rather than built by concatenating
			// an origin and an outcome. A composed key would be shorter here
			// and INVISIBLE to the root key-agreement guard, which finds
			// referenced events by matching literal sendMoveEvent/
			// renderMoveEvent call sites: a grep can only find the name you
			// guessed. Keeping each key literal is what lets the build fail on
			// an event the YAML stops authoring.
			var eventKey movenarration.EventKey
			switch {
			case known && hit:
				eventKey = "arrival_known_hit"
			case known && partial:
				eventKey = "arrival_known_partial"
			case known:
				eventKey = "arrival_known_miss"
			case hit:
				eventKey = "arrival_unknown_hit"
			case partial:
				eventKey = "arrival_unknown_partial"
			default:
				eventKey = "arrival_unknown_miss"
			}

			arrivalTokens := map[string]string{}
			if known {
				arrivalTokens[movenarration.TokenExitName] = fromDir
			}
			roles, _ := renderMoveEvent("shoot", eventKey, ids, arrivalTokens)
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.NoLine,
				Actee:    messaging.NoLine,
				Observer: lineOrNone(messaging.CategoryHitRanged, roles.ActeeObserver),
			}, messaging.Audience{ActorName: mob.Character.Name, ActeeId: result.TargetUserId, ActeeName: result.TargetName, Room: tr})
		}
	}

	// Retaliation: the target aggros back onto the shooter (same-room only —
	// cross-room aggro on a defender is dropped by the combat handler). Any
	// damage dealt (hit or partial) provokes retaliation the same way a clean
	// hit does; only a zero-damage miss from stealth does not.
	if !result.CrossRoom && (dealt || !result.IsSneaking) {
		if result.IsTargetMob {
			if target := mobs.GetInstance(result.TargetMobInstanceId); target != nil && !target.Character.IsInCombat() {
				targeting.Commit(&target.Character, state.ActorRef{MobInstanceId: mob.InstanceId}, targeting.ReasonAttack)
			}
		} else if result.TargetUserId > 0 {
			if u := users.GetByUserId(result.TargetUserId); u != nil && !u.Character.IsInCombat() {
				targeting.Commit(u.Character, state.ActorRef{MobInstanceId: mob.InstanceId}, targeting.ReasonAttack)
			}
		}
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(&actions.MobActor{Mob: mob, Room: room}, result.Counter)

	return true, nil
}
