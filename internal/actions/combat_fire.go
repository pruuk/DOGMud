package actions

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
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
// must be loaded; firing unloads it (even on a miss). Firing does NOT consume
// the special-move cooldown — reloading does. It DOES consume the attacker's
// combat round via RecordAndWait.
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

	// A same-room opening shot enters combat only after paid admission. This
	// gives RecordAndWait an engagement to charge without mutating aggro for a
	// refused shot. Cross-room shots remain one-shot and aggro-free.
	if !crossRoom && char.Aggro == nil {
		char.SetAggro(targetUserId, targetMobInstanceId, characters.DefaultAttack)
	}

	// The shot: unload first (fires even on a miss), then resolve.
	weapon.Loaded = false

	shotMult := weapon.GetSpec().DamageMultiplier * float64(cfg.RangedShotScale)
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
		},
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

	// Analytics + round consumption (same pattern as kick). Fire never burns
	// the special-move cooldown — only the combat round.
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

	return result
}
