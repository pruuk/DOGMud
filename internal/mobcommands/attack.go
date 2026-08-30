package mobcommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Attack(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	attackPlayerId := 0
	attackMobInstanceId := 0

	if rest == `` {
		// If no argument supplied, attack whoever is attacking the player currently.
		for _, mId := range room.GetMobs(rooms.FindFightingMob) {
			m := mobs.GetInstance(mId)
			if m.Character.IsInCombat() && m.Character.CurrentCombatTarget().MobInstanceId == mob.InstanceId {
				attackMobInstanceId = m.InstanceId
				break
			}
		}

		if attackMobInstanceId == 0 {
			for _, uId := range room.GetPlayers(rooms.FindFightingMob) {
				u := users.GetByUserId(uId)
				if u.Character.IsInCombat() && u.Character.CurrentCombatTarget().MobInstanceId == mob.InstanceId {
					attackPlayerId = u.UserId
					break
				}
			}
		}
	} else {
		// Wildcard and named-target resolution delegated to shared helper.
		t := actions.FindAttackTarget(rest, room, 0, mob.InstanceId)
		attackPlayerId = t.UserId
		attackMobInstanceId = t.MobInstanceId
	}

	isSneaking := mob.Character.IsHidden()

	/*
		combatAddlWaitRounds := mob.Character.Equipment.Weapon.GetSpec().WaitRounds + mob.Character.Equipment.Weapon.GetSpec().WaitRounds
		attkType := characters.DefaultAttack
		if mob.Character.Equipment.Weapon.GetSpec().Subtype == items.Shooting {
			attkType = characters.Shooting
		}
	*/

	if attackPlayerId > 0 {

		u := users.GetByUserId(attackPlayerId)

		if u != nil {

			// Track that they've attacked this player
			mob.PlayerAttacked(attackPlayerId)

			// Hidden mobs open from stealth: the first strike of the combat
			// round resolves as a surprise. Don't clear the Hidden buff here
			// — leave it for the combat loop's CancelIfCombat pass so the
			// opener resolves with the mob still hidden.
			// EngageAggroType gates on hidden state AND the special-move
			// cooldown internally, so no IsHidden pre-check here: a
			// hidden-but-on-cooldown opener is an ordinary attack.
			aggroType := characters.DefaultAttack
			if targetUser := users.GetByUserId(attackPlayerId); targetUser != nil {
				// Refusal signal discarded: it is feedback for the ATTACKER,
				// and the attacker here is a mob. The victim must not learn
				// that the thing stalking them failed to line up an ambush.
				aggroType, _ = actions.EngageAggroType(
					actions.NewMobActorInRoom(mob, room),
					actions.NewUserActorInRoom(targetUser, room),
				)
			}
			// Only announce if not already fighting this target
			alreadyFighting := mob.Character.IsInCombat() && mob.Character.CurrentCombatTarget().UserId == attackPlayerId
			// U12c-0b: report the engagement, not the attempt. A vetoed commit
			// must not announce "prepares to fight you!" to a victim nobody is
			// actually fighting.
			engaged := targeting.Commit(&mob.Character,
				state.ActorRef{UserId: attackPlayerId},
				targeting.ReasonForAggroType(aggroType))

			if engaged && !isSneaking && !alreadyFighting {

				if canSeeInDark(u, room) {
					u.SendText(messaging.CategoryHitMelee, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> prepares to fight you!`, mob.Character.Name))
				} else {
					u.SendText(messaging.CategoryHitMelee, `Something prepares to fight you!`)
				}

				room.SendTextVisual(messaging.CategoryHitMelee,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> prepares to fight <ansi fg="username">%s</ansi>`, mob.Character.Name, u.Character.Name),
					u.UserId)

			}
		}

		return true, nil

	} else if attackMobInstanceId > 0 {

		m := mobs.GetInstance(attackMobInstanceId)

		if m != nil {

			// See above: EngageAggroType decides, not IsHidden alone.
			// Validate refreshes buff-derived state before the hidden check
			// inside EngageAggroType.
			if mob.Character.IsHidden() {
				mob.Character.Validate(true)
			}
			// Refusal signal discarded: mob against mob has no player to tell.
			mobAggroType, _ := actions.EngageAggroType(
				actions.NewMobActorInRoom(mob, room),
				actions.NewMobActorInRoom(m, room),
			)
			// Must be read BEFORE Commit, which is what changes the target.
			//
			// ⚠️ This guard mirrors the player-target branch above and was
			// MISSING here, which is the companion-assist double the player
			// actually sees. A companion assists by attacking the enemy MOB, so
			// it comes down this branch, and two systems command it: the
			// reactive CombatPhase_CompanionAssist.go and the polling
			// handleCompanionOwnerAssist. TryClaimAssistCommand dedupes them
			// within a round, but the reactive path fires a round earlier, so
			// the second command lands next round with a fresh claim. The
			// command itself is harmless -- it re-commits the same target --
			// but the unguarded announce printed "prepares to fight" twice.
			// lookfortrouble.go describes the same shape for grace-protected
			// players: commands that bounce while the message fires each time.
			alreadyFighting := mob.Character.IsInCombat() &&
				mob.Character.CurrentCombatTarget().MobInstanceId == attackMobInstanceId

			// U12c-0b: report the engagement, not the attempt. A vetoed commit
			// must not announce a fight nobody is having.
			engaged := targeting.Commit(&mob.Character,
				state.ActorRef{MobInstanceId: attackMobInstanceId},
				targeting.ReasonForAggroType(mobAggroType))

			if engaged && !isSneaking && !alreadyFighting {
				room.SendTextVisual(messaging.CategoryHitMelee,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> prepares to fight <ansi fg="mobname">%s</ansi>`, mob.Character.Name, m.Character.Name))
			}

		}

		return true, nil
	}

	if !isSneaking {
		room.SendTextVisual(messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> looks confused and upset.`, mob.Character.Name))
	}

	return true, nil
}
