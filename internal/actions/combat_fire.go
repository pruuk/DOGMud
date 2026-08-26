package actions

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/progression"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// FireResult holds the outcome of a shoot/fire attempt.
type FireResult struct {
	WeaponName string
	TargetName string

	TargetUserId        int
	TargetMobInstanceId int
	TargetRoomId        int
	IsTargetMob         bool

	CrossRoom  bool
	ExitName   string
	IsSneaking bool

	// Revealed reports that this shot was a same-room surprise shot and gave
	// the shooter's position away (U10d). Distinct from IsSneaking, which is
	// only "the shooter was hidden when the shot was declared": a cross-room
	// shot is sneaking and is NOT revealed.
	Revealed bool

	// SurpriseOnCooldown reports that the shooter WAS hidden in the same room
	// but the shared special-move timer was already claimed, so the shot
	// resolved as an ordinary one. The wrapper must speak this: otherwise the
	// ambush silently does nothing and reads as a bug. It is the common case
	// rather than a corner — a loaded bow implies a recent reload, and reload
	// burns the same timer.
	SurpriseOnCooldown bool

	// AimedWhileEngaged reports that this shot was taken while something in the
	// SHOOTER's room had the shooter as its aggro target, so it did NOT carry
	// RangedUnengagedDamageMultiplier. The wrapper must be able to say so once
	// per engagement: damage that silently drops to a fraction reads as a bug.
	// Set on every such shot, same-room or cross-room, and set independently of
	// the knob's value -- it reports the SITUATION, not the arithmetic.
	AimedWhileEngaged bool

	MoveResult combat.SkillMoveResult
	Executed   bool
	Cost       characters.CostCommitResult

	// Counter is the counter tier outcome (U6b Tasks 10-11): non-zero when a
	// same-room shot was crit-defended and answered. The command wrapper
	// speaks its narration AFTER the shot's own outcome via
	// DispatchCounterMessages.
	Counter combat.CounterResult

	NoWeapon       bool
	NotLoaded      bool
	BadSyntax      bool
	ExitLocked     bool
	NoTarget       bool
	IsCharmed      bool
	IsNonCombatant bool
	Crafting       bool

	// TooDarkToAim and Blinded are distinct from NoTarget, and from each other,
	// on purpose. In both the target is present and named correctly and the
	// shooter simply cannot see to line up an aimed shot -- but "it is too dark"
	// is a lie to a blinded player standing in daylight, and "you are blind" is a
	// lie to a sighted one standing in an unlit cave. Folding either into
	// NoTarget renders as "Could not find your target.", which reads as a typo.
	TooDarkToAim bool
	Blinded      bool
}

// ExecuteFire resolves a ranged shot immediately. rest is either "<target>"
// (same room) or "<target words...> <direction>" (adjacent room). The weapon
// must be loaded; firing unloads it (even on a miss). An ORDINARY shot does not
// consume the special-move cooldown — reloading does — but the U10d same-room
// surprise shot does, and is refused (and downgraded to an ordinary shot) when
// that timer is already claimed. Every shot consumes the attacker's combat
// round via RecordAndWait.
//
// Callers are responsible for: messaging, OnSkillUse/OnStatUse progression,
// retaliation aggro on the target, crime recording, and combat-initiation
// aggro for same-room shots.
//
// DESIGN DECISION (2026-08-14): aimed thrown weapons -- darts, javelins,
// throwing knives -- belong HERE, under ranged-combat, not under the `throw`
// command. `throw` is the grenade verb: it is untargeted and resolves as a room
// AoE against every hostile present, which is the right shape for an explosive
// and the wrong shape for a javelin. This path already has what an aimed throw
// needs: single-target resolution, cross-room shots, Perception-based aiming
// (deliberate-move semantics rather than auto-attack swings), the reload
// machinery, and correct crime and revenge seeding.
//
// The open problem, if that is ever built: a thrown weapon is its own
// ammunition, while this path requires a wielded ranged weapon via
// findRangedWeaponSlot. Either such a weapon equips and consumes itself on use,
// or a `thrown` weapon subtype gets taught to this resolver. That is a feature,
// not a refactor. See the matching note on Throw in internal/usercommands.
func ExecuteFire(actor Actor, rest string) FireResult {
	char := actor.GetCharacter()

	// Don't interrupt any active activity (cast/craft/salvage) to fire.
	if char.IsActing() {
		return FireResult{Crafting: true}
	}

	weapon := findRangedWeaponSlot(actor)
	if weapon == nil {
		return FireResult{NoWeapon: true}
	}
	if !weapon.Loaded {
		return FireResult{WeaponName: weapon.DisplayName(), NotLoaded: true}
	}
	weaponSnapshot := *weapon

	args := strings.Fields(rest)
	if len(args) < 1 {
		return FireResult{WeaponName: weapon.DisplayName(), BadSyntax: true}
	}

	// Try the last word as a direction (cross-room shot); fall back to a
	// same-room target if it isn't a usable exit.
	room := actor.GetRoom()
	if room == nil {
		return FireResult{WeaponName: weapon.DisplayName(), NoTarget: true}
	}

	crossRoom := false
	exitName := ""
	targetRoom := room
	targetWords := args

	if len(args) >= 2 {
		if name, roomId := room.FindExitByName(args[len(args)-1]); name != "" {
			exitInfo, _ := room.GetExitInfo(name)
			if exitInfo.Lock.IsLocked() {
				return FireResult{WeaponName: weapon.DisplayName(), ExitName: name, ExitLocked: true}
			}
			if adj := rooms.LoadRoom(roomId); adj != nil {
				crossRoom = true
				exitName = name
				targetRoom = adj
				targetWords = args[:len(args)-1]
			}
		}
	}

	targetUserId, targetMobInstanceId := targetRoom.FindByName(strings.Join(targetWords, " "))
	if targetUserId == 0 && targetMobInstanceId == 0 && crossRoom {
		// The trailing word may have been part of the target name after all;
		// retry as a same-room shot using the full argument string.
		crossRoom, exitName, targetRoom = false, "", room
		targetUserId, targetMobInstanceId = room.FindByName(strings.Join(args, " "))
	}
	if targetUserId == 0 && targetMobInstanceId == 0 {
		return FireResult{WeaponName: weapon.DisplayName(), NoTarget: true}
	}

	result := FireResult{
		WeaponName:          weapon.DisplayName(),
		TargetUserId:        targetUserId,
		TargetMobInstanceId: targetMobInstanceId,
		TargetRoomId:        targetRoom.RoomId,
		CrossRoom:           crossRoom,
		ExitName:            exitName,
		IsSneaking:          char.IsHidden(),
	}

	// Resolve the defender character. Charmed / non-combatant checks happen
	// BEFORE unloading — a refused shot is not wasted.
	var defChar *characters.Character
	if targetMobInstanceId > 0 {
		m := mobs.GetInstance(targetMobInstanceId)
		if m == nil {
			return FireResult{WeaponName: weapon.DisplayName(), NoTarget: true}
		}

		// Deliberately NOT mobs.CheckPlayerHarm: this path spares only the
		// SHOOTER'S OWN companion (charmerKey), where the shared policy spares
		// anyone's, and it reports the two rejection reasons through separate
		// result flags. Narrowing or merging it would change behavior.
		//
		// Charmed-mob friendly-fire prevention. Player actors charm by userId;
		// mob actors charm by instanceId.
		charmerKey := actor.GetUserId()
		if charmerKey == 0 {
			charmerKey = actor.GetMobInstanceId()
		}
		if m.Character.IsCharmed(charmerKey) {
			result.IsCharmed = true
			result.TargetName = m.Character.Name
			return result
		}
		if m.IsNonCombatant() || m.PlayerAttackImmune {
			result.IsNonCombatant = true
			result.TargetName = m.Character.Name
			return result
		}

		result.IsTargetMob = true
		result.TargetName = m.Character.Name
		defChar = &m.Character
	} else {
		u := users.GetByUserId(targetUserId)
		if u == nil {
			return FireResult{WeaponName: weapon.DisplayName(), NoTarget: true}
		}
		result.TargetName = u.Character.Name
		defChar = u.Character
	}

	// FindByName deliberately includes every occupant. Apply the same
	// actor-aware sight query used by combat rendering, plus the target's
	// canonical hidden state, before admission. An unseen named occupant is not
	// a valid line of fire and must not cost stamina or mutate combat state.
	//
	// Three DISTINCT reasons, reported separately, because they need three
	// different sentences. CanSeeClearly folds blindness and an unlit room into
	// one bool, which is right for rendering and wrong for explaining.
	//
	// A hidden target is genuinely not findable, so NoTarget is honest. The
	// other two are not: the target is right there and correctly named, and an
	// aimed shot is a Perception action that needs to see what it is aiming at.
	// Telling that player "Could not find your target." sends them hunting for a
	// typo that does not exist.
	//
	// Melee deliberately has no equivalent gate. Swinging blind at something in
	// the same room is a reasonable thing to let a player do; lining up a bow
	// shot is not.
	if defChar.IsHidden() {
		result.NoTarget = true
		return result
	}
	if char.Perception != nil && char.Perception.State() == perception.Blinded {
		result.Blinded = true
		return result
	}
	if !messaging.CanSeeClearly(char, targetRoom) {
		result.TooDarkToAim = true
		return result
	}

	cfg := configs.GetBalanceConfig()
	result.Cost = admitFullCost(actor, costs.ActionShoot, characters.PoolStamina,
		float64(cfg.ShootBaseStaminaCost))
	if result.Cost.Status == characters.CostRefused {
		return result
	}
	if !weapon.Equals(weaponSnapshot) || !weapon.Loaded {
		return result
	}

	// U10d. NOTE the ordering: a same-room shot calls SetAggro below, which
	// cascades Hidden -> Revealing. result.IsSneaking was captured before that.
	//
	// Cross-room is excluded deliberately: it never SetAggro's, is reach-gated
	// out of counterattacks (the one uncounterable attack), and narrates
	// anonymously. A stacked crit on top of all three would be a boss killed
	// from the next room at no risk and with no way to learn who did it.
	surpriseShot := !crossRoom && result.IsSneaking
	if surpriseShot && !char.TryCooldown("special-move",
		fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown)) {
		// DENIED, and the player must be told (Task 14 speaks the line).
		surpriseShot = false
		result.SurpriseOnCooldown = true
	}

	bonusCrit := 1.0
	if surpriseShot {
		// The RANGED knob, not the melee one. It ships lower (0.5 against 1.0)
		// because a shot answers one fewer defence and the opener inherits
		// RangedUnengagedDamageMultiplier -- it is unengaged by definition (a
		// hidden shooter is not being hit). That knob is now wired, below, and
		// the two COMPOUND: the 0.5 here was always sized for this world.
		// Passing the melee knob here would put the ambush near 18,000 instead
		// of ~9,080.
		bonusCrit = combat.OpeningStrikeMultiplier(char,
			float64(cfg.SurpriseRangedStrikeMultiplier))
	}

	// A same-room opening shot enters combat only after paid admission. This
	// gives RecordAndWait an engagement to charge without mutating aggro for a
	// refused shot. Cross-room shots remain one-shot and aggro-free.
	if !crossRoom && char.Aggro == nil {
		char.SetAggro(targetUserId, targetMobInstanceId, characters.DefaultAttack)
	}

	if surpriseShot {
		// U10d: firing from stealth gives away your position.
		//
		// Belt and braces. The ordinary same-room shot is ALREADY revealed
		// indirectly, via SetAggro -> TransitionToEngaging -> the Awareness
		// cascade, and this call is a no-op there.
		//
		// It is load-bearing on the three paths where SetAggro returns before,
		// or loses, that phase transition (internal/characters/
		// combat_state_compat.go): the grace-period guard on a protected player
		// target (:85), the taunt-hold guard (:94), and a VETOED
		// TransitionToEngaging (:133-148, whose error is deliberately
		// discarded). In all three no cascade fires, and without this call a
		// hidden shooter would take the ambush bonus and stay hidden. The
		// grace-guard case -- hidden shooter, grace-protected player target,
		// same room -- is plainly reachable.
		//
		// NOT justified by "a re-hidden archer who is already engaged": Sneak
		// (sneak.go:60-62) is the only entry into awareness.Hidden and it
		// refuses outright while char.Aggro != nil, so that shooter cannot
		// exist today.
		_ = char.Awareness.TransitionToRevealing(state.TransitionReason{
			Trigger: awareness.TriggerRangedSurpriseShot,
		})
		result.Revealed = true
	}

	// The shot: unload first (fires even on a miss), then resolve.
	weapon.Loaded = false

	// U10d. The bow's flat damage_multiplier came down onto the melee band, and
	// the compensation it was carrying moved here, where it is situational: a
	// shot pays for its one-attack-per-round economy only while nothing in the
	// room is hitting the shooter. You cannot aim while someone is on you.
	unengagedMult := 1.0
	if shooterIsUnengaged(char, room) {
		unengagedMult = float64(cfg.RangedUnengagedDamageMultiplier)
	} else {
		// The wrapper speaks this (Task 14). Damage that silently drops to a
		// fraction with no line of text reads as a bug.
		result.AimedWhileEngaged = true
	}

	shotMult := weapon.GetSpec().DamageMultiplier * float64(cfg.RangedShotScale) * unengagedMult
	rangedRank := char.GetSkillLevel(skills.RangedCombat)

	// U6b Tasks 7+8: fire routes through the channel seam. ChannelRanged
	// decides the defence set — dodge for everyone, block only for shielded
	// defenders (a real contest entry, replacing the deleted flat
	// shield-bonus knob and the folded defence scalar) — and a
	// shot can crit against CritBarFor's pair bar. The attack stat is
	// GetEffectivePerception() (Assumption 2: aimed shots are
	// deliberate-move actions, not auto-attack swings).
	result.MoveResult = combat.ExecuteSkillMove(combat.SkillMoveParams{
		Attacker: char,
		Defender: defChar,
		Channel:  combat.ChannelRanged,
		Attack: combat.AttackSide{
			Stat: char.GetEffectivePerception(), StatName: "perception",
			Skill: skills.RangedCombat, SkillRank: rangedRank,
			Mult:      combat.SituationalAttackMult(char, combat.ChannelRanged),
			ForceCrit: combat.SleepingForceCrit(defChar),
			CritOnWin: surpriseShot,
		},
		BonusCritMultiplier:  bonusCrit,
		DamagePercent:        shotMult,
		KnockdownFactor:      0,
		DamageStat:           char.GetEffectivePerception(),
		MitigationMultiplier: 1.0,
	})
	result.Executed = true

	// U6b Task 10: a crit-defended shot earns the defender a counter-swing,
	// REACH-GATED — only when the shooter shares the room. The cross-room
	// shot is the ONE uncounterable attack (owner decision: a property of
	// the weapon, not a wiring hole). The wrapper speaks the counter AFTER
	// the shot's own outcome via DispatchCounterMessages (Task 11).
	result.Counter = counterSkillMoveExit(actor, defChar, result.MoveResult, combat.ChannelRanged, !crossRoom)

	// Analytics + round consumption (same pattern as kick). Every shot burns
	// the combat round; only the U10d surprise opener claims the shared
	// special-move cooldown, and it does so at the decision above.
	sourceType := combat.User
	if !actor.IsPlayer() {
		sourceType = combat.Mob
	}
	targetType := combat.User
	if result.IsTargetMob {
		targetType = combat.Mob
	}
	// result.MoveResult.Damage is always the amount actually applied (hit,
	// partial-on-defended, or 0 on a defensive crit), so it is truthful
	// whether or not the shot landed.
	dmg := result.MoveResult.Damage
	RecordAndWait(char, "shoot", sourceType, defChar, targetType, result.MoveResult.Hit, dmg, util.GetRoundCount())

	awardFireProgression(actor, surpriseShot, result.MoveResult.Hit)

	return result
}

// shooterIsUnengaged reports whether nothing in the room currently has the
// shooter as its aggro target.
//
// A room scan rather than Character.Attackers(): that list is never populated
// in production (combatphase.RegisterMachine has no production callers), so it
// always reads empty and would hand out the bonus unconditionally.
//
// It scans the SHOOTER's room, never the target's. So a cross-room sniper who
// is himself in melee IS engaged and loses the bonus -- that is the point of
// the rule, not an edge case. Pass actor.GetRoom(), NOT targetRoom.
//
// The shooter's OWN charmed companion is skipped. A companion aggroed on its
// owner is not "someone is hitting me", and it is reachable: steal
// (steal.go, which deliberately opts out of mobs.CheckPlayerHarm) and plant
// both answer a failed roll with `attack @<owner>`, and the behaviour-tree
// mob_idle attack fallback (behaviortree/actions_combat.go) picks a random
// player in the room with no owner exclusion. All three leave the mob charmed
// and still listed as a companion. The BETRAYAL cases are deliberately NOT
// excluded and must not be: charm lapse (NewRound_MobRoundTick.go:472-491) and
// dismiss (dismiss.go:85-134) both RemoveCharm BEFORE they SetAggro, so an
// ex-companion turning on you reads here as the genuine attacker it is.
//
// The skip is keyed on uid ALONE, with no mob-instance fallback, and that is
// deliberate. CharmInfo.UserId (charminfo.go) is always a PLAYER id -- every
// production Charm() caller passes user.UserId, and the lone exception
// (behaviortree/actions_mob.go) passes literal 0. So a `charmerKey` that falls
// back to MobInstanceId would be comparing two different id spaces, and they
// collide for real: instanceCounter hands out ids from 1 upward and prod user
// ids are also small. A hostile archer whose InstanceId happened to equal some
// player's UserId, attacked by THAT player's companion, would silently keep the
// full multiplier. A mob shooter has no companions to spare, so the fallback
// should not exist. Note the friendly-fire gate at :198-204 still carries the
// old conflated idiom -- pre-existing, and it merely refuses an action visibly
// rather than multiplying damage in silence.
func shooterIsUnengaged(char *characters.Character, room *rooms.Room) bool {
	if room == nil {
		return true
	}
	uid, mid := char.GetUserId(), char.MobInstanceId

	for _, instId := range room.GetMobs(rooms.FindFighting) {
		m := mobs.GetInstance(instId)
		// Both guards, for parity with combat_retarget.go. The IsInCombat()
		// half is redundant TODAY -- IsInCombat() falls back to `Aggro != nil`
		// (character.go), which the preceding check has already established --
		// so do not reason about it as though it screened anything extra.
		if m == nil || m.Character.Aggro == nil || !m.Character.IsInCombat() {
			continue
		}
		if uid > 0 && m.Character.IsCharmed(uid) {
			continue
		}
		// No `instId == mid` self-skip to mirror the players loop's
		// `pId == uid`: mob self-aggro is unreachable, not an omission. Every
		// SetAggro site was walked; the only candidate (actions_party.go) needs
		// a party leader already aggroed on its own member, which nothing
		// produces.
		if (uid > 0 && m.Character.Aggro.UserId == uid) ||
			(mid > 0 && m.Character.Aggro.MobInstanceId == mid) {
			return false
		}
	}
	for _, pId := range room.GetPlayers(rooms.FindFighting) {
		u := users.GetByUserId(pId)
		if u == nil || u.Character.Aggro == nil || pId == uid || !u.Character.IsInCombat() {
			continue
		}
		if (uid > 0 && u.Character.Aggro.UserId == uid) ||
			(mid > 0 && u.Character.Aggro.MobInstanceId == mid) {
			return false
		}
	}
	return true
}

// awardFireProgression fires the ONE ordinary progression award a resolved shot
// earns: ranged-combat and, on a landed ambush, skullduggery, contesting in a
// single Best-of rather than taking an award each.
//
// U10b-1 Task 11. Before it these were two awards in two different files --
// ranged-combat plus an unconditional perception roll in
// usercommands/shoot.go, and skullduggery here -- so one resolved shot could
// pay three progression events. Perception is no longer rolled separately: it
// is ranged-combat's primary stat, so the award rolls it once via
// OnSkillUseScaled. On a landed shot that is a CUT, since perception used to be
// rolled twice (once explicitly, once through the skill).
//
// won is the contest WIN (SkillMoveResult.Hit), not "dealt damage". A defended
// shot still applies partial damage on the shared mitigation curve and that is
// a loss, which now pays ProgressionFailureFraction rather than nothing.
//
// ⚠️ BOTH ROLLS ARE SYNTHESISED, unlike the melee attacker contest, which
// selects on the attack roll that actually happened. combat.SkillMoveResult
// exposes margins but not its AttackRoll, and plumbing one out of the shared
// ExecuteSkillMove seam -- used by bash, kick, trip, hamstring and the rest --
// is wider than this task. Synthesising BOTH keeps the comparison internally
// fair, which is what the selection needs, and is strictly better than mixing
// one real roll with one synthetic one. Revisit if that seam ever exposes the
// roll.
//
// ⚠️ RANGED-COMBAT IS GATED ON IsPlayer TO PRESERVE AN EXISTING GAP, not
// because mobs should not train it. mobcommands/shoot.go has never awarded any
// progression, so a mob archer trains nothing today; closing that is the
// "mob archer ranged-combat progression" faucet the U10b-1 design explicitly
// defers to U10b-2. Removing this gate is that task, and it should be removed
// deliberately with the rate change measured, not as a side effect here.
func awardFireProgression(actor Actor, surpriseShot, hit bool) {
	char := actor.GetCharacter()
	if char == nil {
		return
	}

	cands := make([]progression.Candidate, 0, 2)
	if actor.IsPlayer() {
		cands = append(cands, char.CandidateFor(string(skills.RangedCombat)))
	}
	// Gated on the shot LANDING, matching the melee ambush: skullduggery here
	// means "the approach worked", not "the approach was attempted".
	if surpriseShot && hit {
		cands = append(cands, char.CandidateFor(string(skills.Skullduggery)))
	}
	if len(cands) == 0 {
		return
	}
	actor.AwardResolved(hit, cands...)
}
