package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/opinions"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Attack(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	attackPlayerId := 0
	attackMobInstanceId := 0

	if rest == `` {
		partyInfo := parties.Get(user.UserId)

		// If no argument supplied, attack whoever is attacking the player currently.
		for _, mId := range room.GetMobs(rooms.FindFightingPlayer) {
			m := mobs.GetInstance(mId)
			if m == nil || !m.Character.IsInCombat() {
				continue
			}

			if m.Character.EngagedTarget().UserId == user.UserId {
				attackMobInstanceId = m.InstanceId
				break
			}

			if partyInfo != nil {
				if partyInfo.IsMember(m.Character.EngagedTarget().UserId) {
					attackMobInstanceId = m.InstanceId
					break
				}
			}
		}

		if attackMobInstanceId == 0 {
			for _, uId := range room.GetPlayers(rooms.FindFightingPlayer) {
				u := users.GetByUserId(uId)
				if !u.Character.IsInCombat() {
					continue
				}

				if u.Character.EngagedTarget().UserId == user.UserId {
					attackPlayerId = u.UserId
					break
				}

				if partyInfo != nil {
					if partyInfo.IsMember(u.Character.EngagedTarget().UserId) {
						attackPlayerId = u.UserId
						break
					}
				}
			}
		}

		// Finally, if still no targets, check if any party members are aggroed and just glom onto that
		if attackMobInstanceId == 0 && attackPlayerId == 0 {
			if partyInfo != nil {
				for uId := range partyInfo.GetMembers() {
					if partyUser := users.GetByUserId(uId); partyUser != nil {
						if !partyUser.Character.IsInCombat() {
							continue
						}

						if partyUser.Character.EngagedTarget().MobInstanceId > 0 {
							attackMobInstanceId = partyUser.Character.EngagedTarget().MobInstanceId
							break
						}

						if partyUser.Character.EngagedTarget().UserId > 0 {
							attackPlayerId = partyUser.Character.EngagedTarget().UserId
							break
						}

					}
				}
			}
		}

	} else {
		// Wildcard and named-target resolution delegated to shared helper.
		t := actions.FindAttackTarget(rest, room, user.UserId, 0)
		attackPlayerId = t.UserId
		attackMobInstanceId = t.MobInstanceId
	}

	if attackMobInstanceId == 0 && attackPlayerId == 0 {
		user.SendText(messaging.CategorySystem, "You attack the darkness!")
		return true, nil
	}

	isSneaking := user.Character.IsHidden()

	/*
		combatAddlWaitRounds := user.Character.Equipment.Weapon.GetSpec().WaitRounds + user.Character.Equipment.Weapon.GetSpec().WaitRounds
		attkType := characters.DefaultAttack
		if user.Character.Equipment.Weapon.GetSpec().Subtype == items.Shooting {
			attkType = characters.Shooting
		}
	*/

	// --- TARGET SWITCHING LOGIC (Stage 7.4) ---
	// If already in combat and trying to attack a different target, use target switching
	if user.Character.IsInCombat() {
		currentTargetUserId := user.Character.CurrentCombatTarget().UserId
		currentTargetMobId := user.Character.CurrentCombatTarget().MobInstanceId

		isDifferentTarget := false
		if attackMobInstanceId > 0 && attackMobInstanceId != currentTargetMobId {
			isDifferentTarget = true
		}
		if attackPlayerId > 0 && attackPlayerId != currentTargetUserId {
			isDifferentTarget = true
		}

		// If switching targets, use the Target command logic instead
		if isDifferentTarget && (user.Character.Aggro.Type == characters.DefaultAttack || user.Character.Aggro.Type == characters.Shooting) {
			// Build target name for the Target command
			targetName := rest
			if targetName == "" || targetName[0] == '*' {
				// For empty or random targets, find the actual name
				if attackMobInstanceId > 0 {
					if m := mobs.GetInstance(attackMobInstanceId); m != nil {
						targetName = m.Character.Name
					}
				} else if attackPlayerId > 0 {
					if p := users.GetByUserId(attackPlayerId); p != nil {
						targetName = p.Character.Name
					}
				}
			}
			// Delegate to Target command for proper switching logic
			return Target(targetName, user, room, flags)
		}
	}
	// --- END TARGET SWITCHING LOGIC ---

	if attackMobInstanceId > 0 {

		m := mobs.GetInstance(attackMobInstanceId)

		// A resolved id whose instance is gone (a stale id flowing through
		// #id targeting, or the instance destroyed between resolution and
		// here) used to fall through and return with NO output at all. A
		// failed resolution must always message the player.
		if m == nil {
			user.SendText(messaging.CategorySystem, `You don't see them here.`)
			return true, nil
		}

		if m != nil {
			dupIdx := room.GetMobDuplicateIndex(m.InstanceId)
			mName := m.Character.GetMobNameIndexed(user.UserId, dupIdx).String()

			switch mobs.CheckPlayerHarm(m) {
			case mobs.HarmBlockedCompanion:
				user.SendText(messaging.CategorySystem, fmt.Sprintf(`%s is someone's companion!`, mName))
				return true, nil
			case mobs.HarmBlockedNonCombatant, mobs.HarmBlockedAttackImmune:
				user.SendText(messaging.CategorySystem, fmt.Sprintf(`You can't attack <ansi fg="mobname">%s</ansi>.`, m.Character.Name))
				mobs.FireAttackRejected(m, user.UserId)
				return true, nil
			}

			if party := parties.Get(user.UserId); party != nil {
				for _, id := range party.UserIds {
					if id == user.UserId {
						continue
					}
					if partyUser := users.GetByUserId(id); partyUser != nil {
						if partyUser.Character.RoomId == user.Character.RoomId &&
							partyUser.Character.GetSetting("autoattack") != "off" &&
							!partyUser.Character.IsInCombat() {
							// U10d: no pre-combat burst is fired here. The
							// party member's own `attack` command runs the
							// normal path, which reaches EngageAggroType and
							// types their engagement from stealth correctly.
							partyUser.Command(fmt.Sprintf(`attack #%d`, attackMobInstanceId))
						}
					}
				}
			}

			// Type the engagement from stealth so the opening strike of the
			// combat round resolves as a surprise. EngageAggroType gates on
			// hidden state AND the special-move cooldown internally, so call
			// it unconditionally and do not pre-check IsHidden here.
			aggroType := characters.DefaultAttack
			ambushDenied := false
			if targetMob := mobs.GetInstance(attackMobInstanceId); targetMob != nil {
				aggroType, ambushDenied = actions.EngageAggroType(
					actions.NewUserActorInRoom(user, room),
					actions.NewMobActorInRoom(targetMob, room),
				)
			}

			// Detect "fresh aggression" before SetAggro overwrites prior state:
			// either no prior aggro, or aggro on a different target.
			// NOTE: Keep Aggro read here for Task 12 — CombatPhase is not
			// populated by writers until Task 15; EngagedTarget() would
			// return zero and cause double-bumps until the sunset in Task 18.
			isFreshAggro := user.Character.Aggro == nil ||
				user.Character.Aggro.MobInstanceId != attackMobInstanceId

			// U12c-0b: a commit can be REFUSED by a combat-phase veto (dead
			// target, despawning, non-combatant). CheckPlayerHarm above screens
			// companion/non-combatant/attack-immune but NOT target life or
			// presence, so a target mid-despawn reaches here. Do not announce a
			// fight that did not start.
			engaged := targeting.Commit(user.Character,
				state.ActorRef{MobInstanceId: attackMobInstanceId},
				targeting.ReasonForAggroType(aggroType))

			// Chunk 4.5: notify seeders that a player engaged a mob.
			// Fires on every attack commitment, not just fresh aggro,
			// so rules 6 + 9 see repeated aggressive actions too.
			// Only fires for mob targets (player-vs-player handled above).
			events.AddToQueue(events.PlayerAttackedMob{
				UserId:        user.UserId,
				MobInstanceId: attackMobInstanceId,
			})

			if isFreshAggro {
				if mob := mobs.GetInstance(attackMobInstanceId); mob != nil {
					// Per-NPC opinion (chunk 1.1).
					opinions.Bump(int(mob.MobId), user.UserId,
						int(configs.GetBalanceConfig().OpinionAttackBump))
					// Per-faction crime + rep (chunk 1.3).
					actions.RecordAssaultCrime(user, mob, room)
				}
			}

			if engaged {
				user.SendText(messaging.CategoryHitMelee,
					fmt.Sprintf(`You prepare to enter into mortal combat with %s.`, mName),
				)
			}

			sendMeleeAmbushDenial(user, ambushDenied)

			if !isSneaking {
				room.SendTextVisual(messaging.CategoryHitMelee,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> prepares to fight %s.`, user.Character.Name, mName),
					user.UserId,
				)
			}

			for _, instId := range room.GetMobs(rooms.FindCharmed) {
				if m := mobs.GetInstance(instId); m != nil {
					if !m.Character.IsInCombat() && m.Character.IsCharmed(user.UserId) {
						// Only auto-assist if the companion has AutoAssist enabled
						comp := user.Character.GetCompanionByInstanceId(instId)
						if comp != nil && comp.AutoAssist {
							m.Command(fmt.Sprintf(`attack #%d`, attackMobInstanceId))
						}
					}
				}
			}

		}

	} else if attackPlayerId > 0 {

		p := users.GetByUserId(attackPlayerId)

		// Same silent fall-through as the mob branch above. This one is
		// reachable in production: `attack @<id>` (the party auto-attack
		// form) resolves through r.players without a liveness check, so a
		// stale player id arrived here as a non-nil id with no user record
		// behind it and the command returned with no output.
		if p == nil {
			user.SendText(messaging.CategorySystem, `You don't see them here.`)
			return true, nil
		}

		if p != nil {

			if pvpErr := room.CanPvp(user, p); pvpErr != nil {
				user.SendText(messaging.CategorySystem, pvpErr.Error())
				return true, nil
			}

			if partyInfo := parties.Get(user.UserId); partyInfo != nil {
				if partyInfo.IsMember(attackPlayerId) {
					user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="username">%s</ansi> is in your party!`, p.Character.Name))
					return true, nil
				}
			}

			if party := parties.Get(user.UserId); party != nil {
				if party.IsLeader(user.UserId) {
					for _, id := range party.GetAutoAttackUserIds() {
						if id == user.UserId {
							continue
						}
						if partyUser := users.GetByUserId(id); partyUser != nil {
							if partyUser.Character.RoomId == user.Character.RoomId {
								partyUser.Command(fmt.Sprintf(`attack @%d`, attackPlayerId)) // # denotes a specific mob instanceId
							}
						}
					}
				}
			}

			// Type the engagement from stealth (PvP). EngageAggroType gates on
			// hidden state AND the special-move cooldown internally, so call
			// it unconditionally and do not pre-check IsHidden here.
			pvpAggroType := characters.DefaultAttack
			pvpAmbushDenied := false
			if targetUser := users.GetByUserId(attackPlayerId); targetUser != nil {
				pvpAggroType, pvpAmbushDenied = actions.EngageAggroType(
					actions.NewUserActorInRoom(user, room),
					actions.NewUserActorInRoom(targetUser, room),
				)
			}

			targeting.Commit(user.Character,
				state.ActorRef{UserId: attackPlayerId},
				targeting.ReasonForAggroType(pvpAggroType))

			user.SendText(messaging.CategoryHitMelee,
				fmt.Sprintf(`You prepare to enter into mortal combat with <ansi fg="username">%s</ansi>.`, p.Character.Name),
			)

			sendMeleeAmbushDenial(user, pvpAmbushDenied)

			if !isSneaking {

				p.SendText(messaging.CategoryHitMelee,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> prepares to fight you!`, user.Character.Name),
				)

				room.SendTextVisual(messaging.CategoryHitMelee,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> prepares to fight <ansi fg="mobname">%s</ansi>.`, user.Character.Name, p.Character.Name),
					user.UserId, attackPlayerId)
			}

			for _, instId := range room.GetMobs(rooms.FindCharmed) {
				if m := mobs.GetInstance(instId); m != nil {
					if !m.Character.IsInCombat() && m.Character.IsCharmed(user.UserId) {
						comp := user.Character.GetCompanionByInstanceId(instId)
						if comp != nil && comp.AutoAssist {
							m.Command(fmt.Sprintf(`attack @%d`, attackPlayerId))
						}
					}
				}
			}

		}

	}

	return true, nil
}

// U10d, melee half — the VOICE of a refused ambush.
//
// The ranged half has spoken its own refusal since Task 14 (shoot.go); the
// melee half was silent, and silence here is worse than a missing line.
// SetAggro cascades Hidden -> Revealing whatever the aggro type is
// (internal/hooks/Awareness_Cascades.go), so a melee ambusher whose shared
// special-move timer was already claimed spent their cover, took an ordinary
// swing, and was told neither thing. To the player the ambush simply did not
// work.
const (
	// surpriseMeleeDeniedText is the ranged refusal VERBATIM. One shared
	// cooldown, one wording, wherever a player meets it. The reasoning behind
	// the words themselves lives on surpriseShotDeniedText in shoot.go.
	surpriseMeleeDeniedText = surpriseShotDeniedText

	// surpriseMeleeRevealedText names the consequence the refusal does not,
	// and it is the more expensive of the two: the ambush is off, the cover is
	// spent anyway. Shaped after surpriseShotRevealedText ("The shot gives
	// your place away...") so the two halves read as one feature.
	surpriseMeleeRevealedText = `Closing in gives your place away. You are no longer hidden.`
)

// sendMeleeAmbushDenial speaks a refused melee opener to the attacker.
//
// Call it AFTER SetAggro. The reveal line is gated on what actually became of
// the attacker's cover, and the cascade that spends it runs inside SetAggro.
//
// The reveal is CHECKED rather than assumed because SetAggro has paths that
// return before the Combat Phase transition ever happens — the grace-period
// guard on a protected player target and the taunt-hold guard, both in
// internal/characters/combat_state_compat.go — plus paths where the transition
// is vetoed and the error discarded. On those the attacker keeps their cover,
// and asserting otherwise would be a lie about the one thing this line exists
// to report.
//
// Both lines ride CategorySurpriseAttack, the same category shoot.go uses for
// its refusal: it sits in neither verbosity suppression allowlist, so the news
// that an ambush did not happen survives a quiet verbosity setting.
func sendMeleeAmbushDenial(user *users.UserRecord, denied bool) {
	if !denied {
		return
	}
	user.SendText(messaging.CategorySurpriseAttack, surpriseMeleeDeniedText)
	if !user.Character.IsHidden() {
		user.SendText(messaging.CategorySurpriseAttack, surpriseMeleeRevealedText)
	}
}
