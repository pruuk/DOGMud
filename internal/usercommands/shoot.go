package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/opinions"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Shoot fires a loaded ranged weapon at a target. The shot resolves
// immediately (one shot per command) — same-room `shoot <target>` or
// cross-room `shoot <target> <direction>`. Firing unloads the weapon;
// callers must `reload` to fire again.
//
// This is the loaded-weapon model (ranged-weapons T6). It REPLACES the
// legacy remote-aggro behavior where a single `shoot` set up a continuous
// round-loop auto-attack into the adjacent room.
func Shoot(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	// --- Pre-fire target resolution & guards ---
	// ExecuteFire applies damage the instant it is called, so every guard that
	// must PREVENT a shot (friendly-fire, PvP legality, self-target) has to run
	// here, before firing. We re-resolve the would-be target (mirroring
	// ExecuteFire's parse) without applying damage. The duplicated resolution is
	// the deliberate price of safe pre-fire gating.
	tUserId, _, tRoom, _ := resolveShootTarget(room, rest)

	// Issue 3 (self-target): room.FindByName can resolve the shooter.
	if tUserId > 0 && tUserId == user.UserId {
		user.SendText(messaging.CategorySystem, `You can't shoot yourself.`)
		return true, nil
	}

	// Player-target guards: party friendly-fire + PvP legality.
	if tUserId > 0 {
		if p := users.GetByUserId(tUserId); p != nil {
			if partyInfo := parties.Get(user.UserId); partyInfo != nil && partyInfo.IsMember(tUserId) {
				user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="username">%s</ansi> is in your party!`, p.Character.Name))
				return true, nil
			}
			// Issue 1 (PvP bypass): gate BEFORE firing, exactly as melee's
			// attack.go does via room.CanPvp. CanPvp's limited-mode branch keys
			// off the RECEIVER room's IsPvp(), so for a cross-room shot we also
			// require the TARGET's room to permit PvP — you must not be able to
			// snipe into a sanctuary from a lawless room outside it.
			if pvpErr := room.CanPvp(user, p); pvpErr != nil {
				user.SendText(messaging.CategorySystem, pvpErr.Error())
				return true, nil
			}
			if tRoom != nil && tRoom != room {
				if pvpErr := tRoom.CanPvp(user, p); pvpErr != nil {
					user.SendText(messaging.CategorySystem, pvpErr.Error())
					return true, nil
				}
			}
		}
	}

	result := actions.ExecuteFire(&actions.UserActor{User: user, Room: room}, rest)

	if result.Cost.Status == characters.CostRefused {
		user.SendText(messaging.CategorySystem, actions.CostRefusalText(result.Cost))
		return true, nil
	}

	// --- Early exits (no shot fired) ---
	if result.Crafting {
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You can't shoot while focused on your work. Finish or be interrupted first.</ansi>`)
		return true, nil
	}
	if result.NoWeapon {
		user.SendText(messaging.CategorySystem, `You don't have a ranged weapon equipped.`)
		return true, nil
	}
	if result.NotLoaded {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`Your <ansi fg="itemname">%s</ansi> isn't loaded. Try <ansi fg="command">reload</ansi>.`, result.WeaponName))
		return true, nil
	}
	if result.BadSyntax {
		user.SendText(messaging.CategorySystem, `Syntax: <ansi fg="command">shoot &lt;target&gt; [direction]</ansi>`)
		return true, nil
	}
	if result.ExitLocked {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`The <ansi fg="exit">%s</ansi> exit is locked.`, result.ExitName))
		return true, nil
	}
	if result.NoTarget {
		user.SendText(messaging.CategorySystem, `Could not find your target.`)
		return true, nil
	}
	if result.Blinded {
		user.SendText(messaging.CategorySystem, `You can't see well enough to aim.`)
		return true, nil
	}
	if result.TooDarkToAim {
		user.SendText(messaging.CategorySystem,
			`It is too dark to aim. You need light, or eyes that do not need it.`)
		return true, nil
	}
	if result.IsCharmed {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> is your friend!`, result.TargetName))
		return true, nil
	}
	if result.IsNonCombatant {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`You can't attack <ansi fg="mobname">%s</ansi>.`, result.TargetName))
		return true, nil
	}

	if !result.Executed {
		return true, nil
	}

	// --- Messaging ---
	sendShootMessages(user, room, result)

	hit := result.MoveResult.Hit
	// Dealt is true on a clean hit AND on a defended shot that still drew
	// blood. A partial draws evidence just like a hit does (the victim's own
	// message names the shooter), so it reveals and is attributed the same
	// way. Only a true zero-damage miss leaves no trace.
	dealt := hit || result.MoveResult.Damage > 0

	// Issue 5 (cross-room hidden reveal): same-room shots reveal the shooter
	// through the normal aggro→combat path (CancelCombatBuffs in the combat
	// round handler, set up by the pre-fire aggro above). A cross-room shooter
	// never enters that loop, so a hidden sniper would stay hidden forever.
	// Mirror melee's reveal-on-engage: a cross-room shot that deals ANY
	// damage (a clean hit or a defended partial) drops stealth. Only a
	// zero-damage cross-room miss stays hidden — the sniper gets exactly one
	// free clean miss, not one free hit. (Same-room shooter aggro is set inside
	// ExecuteFire, AFTER cost admission, so a refused shot never engages.)
	if result.CrossRoom && dealt {
		user.Character.CancelCombatBuffs()
	}

	// --- Retaliation + crime (mob targets only) ---
	if result.IsTargetMob {
		// The mob (and its faction) only react if the shot was noticed: any
		// damage dealt (hit or partial) always reveals the shooter; a
		// zero-damage miss from stealth does not. A visible shooter is
		// always noticed.
		noticed := dealt || !result.IsSneaking
		if m := mobs.GetInstance(result.TargetMobInstanceId); m != nil && noticed {

			if dealt {
				m.Character.TrackPlayerDamage(user.UserId, result.MoveResult.Damage)
				// mob_hurt behavior tree — same trigger the unified combat
				// handler fires (fireDefenderBehaviorTrigger).
				behaviortree.TryMobBehavior(m.InstanceId, behaviortree.EventContext{
					EventType: "mob_hurt",
					RoomId:    m.Character.RoomId,
					UserId:    user.UserId,
				})
			}

			if result.CrossRoom {
				// Aggro alone won't make a mob chase cross-room — the unified
				// combat handler drops a mob attacker's aggro the moment its
				// target isn't in the room. Cross-room pursuit instead runs
				// through the revenge-mob GOAL: the PlayerAttackedMob event
				// (below) seeds it, and CombatMemory is what lets that goal's
				// context score see the shooter as "recently seen" so the
				// goal planner emits `pathto` toward the shooter's room.
				m.CombatMemory = mobs.SetCombatMemory(user.UserId, 0, user.Character.RoomId, util.GetRoundCount())
			} else if m.Character.Aggro == nil {
				m.Character.SetAggro(user.UserId, 0, characters.DefaultAttack)
			}

			// Seed revenge + disposition + faction crime — mirrors the melee
			// fresh-aggression block in attack.go. PlayerAttackedMob drives
			// the revenge-mob seeder (the cross-room pursuit machinery);
			// recordAssaultCrime lets IdentifiedPerp attribute the shot.
			events.AddToQueue(events.PlayerAttackedMob{
				UserId:        user.UserId,
				MobInstanceId: m.InstanceId,
			})
			opinions.Bump(int(m.MobId), user.UserId, int(configs.GetBalanceConfig().OpinionAttackBump))
			actions.RecordAssaultCrime(user, m, rooms.LoadRoom(result.TargetRoomId))
		}
	}

	// --- Progression: perception always, ranged-combat on hit (mirror melee) ---
	user.Character.OnStatUse("perception", user.UserId)
	if hit {
		user.Character.OnSkillUse(string(skills.RangedCombat), user.UserId)
	}

	// Notify the quest engine of the `shoot` command so quests can gate a step
	// on the act of shooting (Spoke G's "shoot the practice butt" / "shoot the
	// foe across the canyon" beats). Mirrors forage/drink/throw/cast. RoomId is
	// the SHOOTER's room — for a cross-room shot the quest gates on the room the
	// player shoots FROM, not the target's room.
	bridge := questengine.NewGameBridge(user, room.RoomId)
	questengine.GetEngine().Notify("command", questengine.EventDetails{
		UserId:  user.UserId,
		RoomId:  room.RoomId,
		Command: "shoot",
	}, bridge, bridge)

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(&actions.UserActor{User: user, Room: room}, result.Counter)

	return true, nil
}

// U10d shooter-facing copy. Every one of these speaks a flag ExecuteFire
// already sets; none of them changes a mechanic.
//
// All are authored to render inside 80 columns with a p90 target name (20
// characters, measured across the authored mob set) and the longest damage band
// substituted, since the messaging pipeline's wrap stage is off (shouldWrap
// returns false for every Category) and the client is handed the line exactly as
// written. shoot_narration_test.go pins every composition.
//
// None of them carries the melee `*[SURPRISE ATTACK]*` banner. That marker
// exists for a line narrated by the generic weapon message pool, which says
// nothing about an ambush; these lines name the shot from cover themselves, and
// a 20-column marker plus a target name plus a damage band does not fit in 80.
const (
	// surpriseShotRevealedText speaks FireResult.Revealed. Losing stealth with
	// no line of text reads as a bug, and the present tense is doing real work:
	// this shot still narrated anonymously to the room and to the target (the
	// IsSneaking branches below), and the shooter is exposed from here on.
	surpriseShotRevealedText = `The shot gives your place away. You are no longer hidden.`

	// surpriseShotDeniedText speaks FireResult.SurpriseOnCooldown. NOT a corner
	// case: reload burns the same shared special-move timer the opener needs,
	// so the natural reload-sneak-shoot order denies the ambush whenever the
	// reload was recent. Without this line the player sees an ordinary shot and
	// concludes the feature does not work.
	//
	// The second sentence is the engine's established refusal wording ("You need
	// a moment to recover before attempting another special move." -- throw.go,
	// mutation_helpers.go), trimmed to fit, so the same cooldown is refused in
	// the same words wherever a player meets it. "special move" is a taught term
	// (there is a `help special` topic).
	surpriseShotDeniedText = `No ambush ready. You need a moment to recover before another special move.`

	// aimedWhileEngagedText speaks FireResult.AimedWhileEngaged. Spoken at most
	// once per engagement (see Character.RangedEngagedCueSpoken) -- damage that
	// silently drops to a fraction reads as a bug, but repeating the reason
	// every round is worse noise than the silence it fixes.
	aimedWhileEngagedText = `Someone is attacking you. You cannot steady your aim, so the shot is weak.`
)

// sendShootMessages emits the shooter line, room broadcasts, and the target's
// direct line for an executed shot. No raw numbers — damage is described via
// combat.GetDamageDescription.
func sendShootMessages(user *users.UserRecord, room *rooms.Room, result actions.FireResult) {

	hit := result.MoveResult.Hit
	partial := !hit && result.MoveResult.Damage > 0
	tier := combat.GetDamageDescription(result.MoveResult.Damage, result.MoveResult.TargetMaxHP)

	// The refusal comes FIRST: it explains why the shot the player set up as an
	// ambush is about to narrate as an ordinary one.
	if result.SurpriseOnCooldown {
		user.SendText(messaging.CategorySurpriseAttack, surpriseShotDeniedText)
	}

	// Color the target name by type.
	targetColored := fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, result.TargetName)
	if !result.IsTargetMob {
		targetColored = fmt.Sprintf(`<ansi fg="username">%s</ansi>`, result.TargetName)
	}

	// U6b Task 9: a defended same-room shot speaks the channel defence triad
	// (dodge or block), naming what actually stopped or blunted it.
	// Cross-room shots keep their origin-anonymous arrival lines: the triad
	// names the shooter, which the target's room cannot see.
	var triadAtk, triadDef, triadRoom string
	if !result.CrossRoom && result.MoveResult.Defence.Defended {
		triad := combat.RenderChannelDefenceMessages(result.MoveResult.Defence, combat.ChannelDefenceIdentities{
			Attacker: user.Character.GetPlayerName(user.UserId).String(),
			Defender: targetColored,
		}, "aimed shot")
		triadAtk, triadDef, triadRoom = string(triad.ToAttacker), string(triad.ToDefender), string(triad.ToRoom)
	}

	// Shooter's own feedback. A fully stopped shot (a defensive crit) speaks
	// the triad; a defended partial keeps the damage-carrying line.
	//
	// U10d: a shot that carried the opener gets its own wording and its own
	// Category. CategorySurpriseAttack is in neither verbosity suppression
	// allowlist, so the one shot that decides an ambush survives medium and
	// light verbosity while the ordinary ranged band does not.
	//
	// ORDER: the triad case sits ABOVE partial here, unlike the ordinary arms
	// below. SkillMoveResult.Hit is `!Defended`, so `partial` (!hit && Damage >
	// 0) is BY CONSTRUCTION a defended shot, and Revealed implies same-room, so
	// triadAtk is always populated when it is true. Ordering partial first --
	// which is what the ordinary arms do, deliberately, to keep their
	// damage-carrying line -- would therefore shadow the triad on every
	// defended-partial ambush and never tell the shooter what stopped it. The
	// melee side names the defence on exactly this outcome; folding the damage
	// band onto the triad line mirrors that composite.
	switch {
	case result.Revealed:
		user.SendText(messaging.CategorySurpriseAttack,
			surpriseShotShooterLine(hit, triadAtk, targetColored, tier, result.MoveResult.Damage > 0))
	case hit:
		user.SendText(messaging.CategoryHitRanged, fmt.Sprintf(`Your shot takes %s (<ansi fg="damage">%s</ansi>)!`, targetColored, tier))
	case partial:
		user.SendText(messaging.CategoryHitRanged, fmt.Sprintf(`Your shot goes wide of %s, but the edge of it still clips them! (<ansi fg="damage">%s</ansi>)`, targetColored, tier))
	case triadAtk != "":
		user.SendText(messaging.CategoryDodge, triadAtk)
	default:
		user.SendText(messaging.CategoryDodge, fmt.Sprintf(`Your shot goes wide of %s!`, targetColored))
	}

	// The reveal reads AFTER the shot it was caused by, and after the room and
	// target lines below still narrate this shot anonymously — that ordering is
	// the point: the shot itself was unseen, and the shooter is exposed from
	// here on.
	if result.Revealed {
		user.SendText(messaging.CategorySurpriseAttack, surpriseShotRevealedText)
	}

	// The engaged-aim cue, at most once per engagement. A shot taken with
	// nothing on the shooter clears the latch, so the next time they are pinned
	// down it is news again -- which is also what heals the one case EndAggro
	// cannot reach, a cross-room shooter who never held Aggro of their own.
	//
	// CategorySystem, not CategorySurpriseAttack, and the difference is not an
	// oversight: this fires on ORDINARY shots and has nothing to do with an
	// ambush. It is a mechanical explanation, like the cost refusal and the
	// defence shortage text this same function already routes to CategorySystem.
	speak, nowSpoken := shouldSpeakEngagedCue(result.AimedWhileEngaged, user.Character.RangedEngagedCueSpoken)
	user.Character.RangedEngagedCueSpoken = nowSpoken
	if speak {
		user.SendText(messaging.CategorySystem, aimedWhileEngagedText)
	}

	// Direct line to a player target.
	if !result.IsTargetMob && result.TargetUserId > 0 {
		if p := users.GetByUserId(result.TargetUserId); p != nil {
			if result.MoveResult.Defence.Defended {
				if text := combat.ChannelDefenceShortageText(result.MoveResult.Defence, p.Character); text != "" {
					p.SendText(messaging.CategorySystem, text)
				}
			}
			shooter := fmt.Sprintf(`<ansi fg="username">%s</ansi>`, user.Character.Name)
			if result.IsSneaking {
				shooter = `Someone`
			}
			switch {
			case hit:
				p.SendText(messaging.CategoryHitRanged, fmt.Sprintf(`%s's shot strikes you (<ansi fg="damage">%s</ansi>)!`, shooter, tier))
			case partial:
				p.SendText(messaging.CategoryHitRanged, fmt.Sprintf(`%s's shot goes wide, but the edge of it still clips you! (<ansi fg="damage">%s</ansi>)`, shooter, tier))
			case triadDef != "":
				personal := triadDef
				if result.IsSneaking {
					personal = messaging.Anonymize(personal)
				}
				p.SendText(messaging.CategoryHitRanged, personal)
			default:
				p.SendText(messaging.CategoryHitRanged, fmt.Sprintf(`%s's shot narrowly misses you!`, shooter))
			}
		}
	}

	weapon := fmt.Sprintf(`<ansi fg="itemname">%s</ansi>`, result.WeaponName)
	shooterName := fmt.Sprintf(`<ansi fg="username">%s</ansi>`, user.Character.Name)

	if !result.CrossRoom {
		// Same-room broadcast (exclude shooter + player target). Suppressed
		// when sneaking. A defended shot's room line comes from the triad.
		if !result.IsSneaking {
			if triadRoom != "" {
				room.SendTextVisual(messaging.CategoryHitRanged, triadRoom,
					user.UserId, result.TargetUserId)
			} else {
				room.SendTextVisual(messaging.CategoryHitRanged,
					fmt.Sprintf(`%s fires their %s at %s!`, shooterName, weapon, targetColored),
					user.UserId, result.TargetUserId)
			}
		}
		return
	}

	// Cross-room. Shooter's room sees the shot leave (suppressed when sneaking).
	if !result.IsSneaking {
		room.SendTextVisual(messaging.CategoryHitRanged,
			fmt.Sprintf(`%s fires their %s %sward.`, shooterName, weapon, result.ExitName),
			user.UserId)
	}

	// Target's room sees the shot arrive (exclude the player target — they
	// already got the direct line above).
	if tr := rooms.LoadRoom(result.TargetRoomId); tr != nil {
		fromDir := tr.FindExitTo(room.RoomId)
		origin := `from somewhere nearby`
		if fromDir != "" {
			origin = fmt.Sprintf(`from beyond the <ansi fg="exit">%s</ansi>`, fromDir)
		}
		switch {
		case hit:
			tr.SendTextVisual(messaging.CategoryHitRanged,
				fmt.Sprintf(`A shot streaks in %s and strikes %s!`, origin, targetColored),
				result.TargetUserId)
		case partial:
			tr.SendTextVisual(messaging.CategoryHitRanged,
				fmt.Sprintf(`A shot streaks in %s and clips %s!`, origin, targetColored),
				result.TargetUserId)
		default:
			tr.SendTextVisual(messaging.CategoryHitRanged,
				fmt.Sprintf(`A shot streaks in %s and narrowly misses %s!`, origin, targetColored),
				result.TargetUserId)
		}
	}
}

// surpriseShotShooterLine composes the shooter's own line for a shot that
// carried the U10d opener. Pure, so the branch ORDER can be pinned by a truth
// table instead of by standing up a hidden archer and a defended target.
//
// THE ORDER IS THE POINT. combat.SkillMoveResult.Hit is `!Defended`, so
// "defended but some damage got through" is `!hit && dealtDamage` -- and the
// ordinary shot arms in sendShootMessages test exactly that combination BEFORE
// they test the defence triad, deliberately, to keep their damage-carrying
// line. Copying that order here shadowed the triad on every defended-partial
// ambush: the shooter was told the shot clipped the target and never told what
// stopped it, on the one shot where that matters most. The triad goes FIRST,
// and the damage band is folded onto it, mirroring what the melee composite
// does for the identical outcome.
//
// triad is combat.RenderChannelDefenceMessages' attacker line; it names the
// dodge or block that answered the shot. It is non-empty whenever the shot was
// defended and the shooter shares the room, which a revealed shot always does.
// The empty-triad arm is therefore the clean miss.
//
// targetColored and triad arrive already ANSI-tagged; tags render as zero
// columns, which is what lets these lines stay inside 80.
func surpriseShotShooterLine(hit bool, triad, targetColored, tier string, dealtDamage bool) string {
	switch {
	case hit:
		return fmt.Sprintf(`Your shot from cover takes %s unaware! (<ansi fg="damage">%s</ansi>)`, targetColored, tier)
	case triad != "":
		if !dealtDamage {
			// A defensive crit deals exactly 0. Appending "(negligible damage)"
			// to a shot that was stopped dead would be the same
			// self-contradiction U6 Task 16b removed from the melee side.
			return triad
		}
		return triad + fmt.Sprintf(` (<ansi fg="damage">%s</ansi>)`, tier)
	default:
		return fmt.Sprintf(`Your shot from cover goes wide of %s!`, targetColored)
	}
}

// shouldSpeakEngagedCue is the once-per-engagement latch for the engaged-aim
// cue, split out as a pure function so the truth table can be pinned without
// standing up a user, a room and a loaded bow.
//
// speak is true only on the FIRST shot of a run of engaged shots. nowSpoken is
// what the caller must store afterwards, and it is simply aimedWhileEngaged:
// a shot that went off with nothing on the shooter clears the latch, so being
// pinned down again later speaks the cue again. The other end of the
// engagement is Character.EndAggro, which clears the same field when combat
// finishes.
func shouldSpeakEngagedCue(aimedWhileEngaged, alreadySpoken bool) (speak bool, nowSpoken bool) {
	return aimedWhileEngaged && !alreadySpoken, aimedWhileEngaged
}

// resolveShootTarget mirrors ExecuteFire's target parsing to determine what a
// shoot command WOULD hit, without firing or applying damage. It powers the
// pre-fire guards (self-target, party friendly-fire, PvP legality) and the
// opening-shot aggro preset. Returns the resolved player id, mob instance id,
// the room the target stands in, and whether the shot is cross-room. Any id may
// be 0 when unresolved; targetRoom is the shooter's room for same-room / absent
// targets.
func resolveShootTarget(room *rooms.Room, rest string) (userId, mobInstanceId int, targetRoom *rooms.Room, crossRoom bool) {
	if room == nil {
		return 0, 0, nil, false
	}
	args := strings.Fields(rest)
	if len(args) < 1 {
		return 0, 0, room, false
	}
	targetRoom = room
	targetWords := args
	if len(args) >= 2 {
		if name, roomId := room.FindExitByName(args[len(args)-1]); name != "" {
			if adj := rooms.LoadRoom(roomId); adj != nil {
				crossRoom = true
				targetRoom = adj
				targetWords = args[:len(args)-1]
			}
		}
	}
	userId, mobInstanceId = targetRoom.FindByName(strings.Join(targetWords, " "))
	if userId == 0 && mobInstanceId == 0 && crossRoom {
		// The trailing word may have been part of the target name after all;
		// retry as a same-room shot using the full argument string.
		crossRoom = false
		targetRoom = room
		userId, mobInstanceId = room.FindByName(strings.Join(args, " "))
	}
	return userId, mobInstanceId, targetRoom, crossRoom
}
