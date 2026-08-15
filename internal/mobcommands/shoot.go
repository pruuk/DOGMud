package mobcommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Shoot fires a mob's loaded ranged weapon at a target (loaded-weapon model,
// ranged-weapons T6). One shot per command; the weapon unloads on fire. No
// crime/justice recording (that's a player concern); retaliation just aggros
// the target back onto the shooter.
func Shoot(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	result := actions.ExecuteFire(&actions.MobActor{Mob: mob, Room: room}, rest)

	// Silent early exits — mobs don't narrate their own failed attempts.
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
	// combat round handler's CancelCombatBuffs once the target is aggroed; a
	// cross-room shooter never enters that loop, so without this it would
	// stay hidden forever. Only a zero-damage cross-room miss stays hidden —
	// the sniper gets exactly one free clean miss, not one free hit.
	if result.CrossRoom && dealt {
		mob.Character.CancelCombatBuffs()
	}

	mobName := fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, mob.Character.Name)
	weapon := fmt.Sprintf(`<ansi fg="itemname">%s</ansi>`, result.WeaponName)

	targetColored := fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, result.TargetName)
	if !result.IsTargetMob {
		targetColored = fmt.Sprintf(`<ansi fg="username">%s</ansi>`, result.TargetName)
	}

	// Direct line to a player target.
	if !result.IsTargetMob && result.TargetUserId > 0 {
		if u := users.GetByUserId(result.TargetUserId); u != nil {
			shooter := mobName
			if result.IsSneaking || !canSeeInDark(u, room) {
				shooter = `Someone`
			}
			switch {
			case hit:
				u.SendText(messaging.CategoryHitRanged, fmt.Sprintf(`%s's shot strikes you!`, shooter))
			case partial:
				u.SendText(messaging.CategoryHitRanged, fmt.Sprintf(`%s's shot goes wide, but the edge of it still clips you!`, shooter))
			default:
				u.SendText(messaging.CategoryHitRanged, fmt.Sprintf(`%s's shot narrowly misses you!`, shooter))
			}
		}
	}

	if !result.IsSneaking {
		if !result.CrossRoom {
			room.SendTextVisual(messaging.CategoryHitRanged,
				fmt.Sprintf(`%s fires their %s at %s!`, mobName, weapon, targetColored),
				result.TargetUserId)
		} else {
			// Shooter's room sees the shot leave.
			room.SendTextVisual(messaging.CategoryHitRanged,
				fmt.Sprintf(`%s fires their %s %sward.`, mobName, weapon, result.ExitName))
			// Target's room sees it arrive.
			if tr := rooms.LoadRoom(result.TargetRoomId); tr != nil {
				fromDir := tr.FindExitTo(room.RoomId)
				origin := `from somewhere nearby`
				if fromDir != "" {
					origin = fmt.Sprintf(`from beyond the <ansi fg="exit">%s</ansi>`, fromDir)
				}
				var verb string
				switch {
				case hit:
					verb = `and strikes`
				case partial:
					verb = `and clips`
				default:
					verb = `and narrowly misses`
				}
				tr.SendTextVisual(messaging.CategoryHitRanged,
					fmt.Sprintf(`A shot streaks in %s %s %s!`, origin, verb, targetColored),
					result.TargetUserId)
			}
		}
	}

	// Retaliation: the target aggros back onto the shooter (same-room only —
	// cross-room aggro on a defender is dropped by the combat handler). Any
	// damage dealt (hit or partial) provokes retaliation the same way a clean
	// hit does; only a zero-damage miss from stealth does not.
	if !result.CrossRoom && (dealt || !result.IsSneaking) {
		if result.IsTargetMob {
			if target := mobs.GetInstance(result.TargetMobInstanceId); target != nil && target.Character.Aggro == nil {
				target.Character.SetAggro(0, mob.InstanceId, characters.DefaultAttack)
			}
		} else if result.TargetUserId > 0 {
			if u := users.GetByUserId(result.TargetUserId); u != nil && u.Character.Aggro == nil {
				u.Character.SetAggro(0, mob.InstanceId, characters.DefaultAttack)
			}
		}
	}

	return true, nil
}
