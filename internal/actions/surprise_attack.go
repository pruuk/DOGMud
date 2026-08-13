package actions

import (
	"fmt"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"math"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// SurpriseAttackOpts parameterizes a surprise-attack trigger attempt.
type SurpriseAttackOpts struct {
	// Target is the actor being attacked. Exactly one of Target.GetUserId()
	// or Target.GetMobInstanceId() should be non-zero when Target is non-nil.
	Target Actor
}

// SurpriseAttackResult is the structured outcome of a SurpriseAttack call.
// On Triggered=false, BlockReason is one of:
//
//	"not-hidden"  — attacker is not in the Hidden state
//	"no-target"   — Target is nil
//	"on-cooldown" — the special-move cooldown is active
//
// On Triggered=true, StrikeCount is the number of per-weapon swings that
// connected (misses do not count).
type SurpriseAttackResult struct {
	Triggered   bool
	StrikeCount int
	BlockReason string
}

// SurpriseAttack fires the pre-combat surprise-attack burst. It requires the
// attacker to be in the Hidden state and have a free special-move cooldown.
// On success it iterates every equipped weapon (falling back to unarmed fists)
// and rolls an attack against the target, applying a crit-style half-mitigation
// bypass and a dex+skullduggery surprise multiplier.
//
// The Awareness_Cascades hook — not this function — handles the
// Hidden → Revealing transition at end of round. Do not call CancelBuff or
// clear Hidden state here.
//
// Both the player wrapper (usercommands/attack.go) and the mob wrapper
// (mobcommands/attack.go) collapse to thin call-throughs. The hidden-state
// gate and cooldown try are handled internally so the callers need no
// pre-flight logic.
func SurpriseAttack(actor Actor, opts SurpriseAttackOpts) SurpriseAttackResult {

	// --- Gate 1: attacker must be hidden ---
	if !actor.GetCharacter().IsHidden() {
		return SurpriseAttackResult{BlockReason: "not-hidden"}
	}

	// --- Gate 2: target must be present ---
	if opts.Target == nil {
		return SurpriseAttackResult{BlockReason: "no-target"}
	}

	// --- Gate 3: special-move cooldown ---
	cfg := configs.GetBalanceConfig()
	if !actor.GetCharacter().TryCooldown("special-move",
		fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown)) {
		return SurpriseAttackResult{BlockReason: "on-cooldown"}
	}

	// --- Resolve target vitals ---
	targetChar := opts.Target.GetCharacter()
	targetName := opts.Target.GetName()

	// For mob targets, use the indexed mob name if we can resolve the room
	// and a viewing user id is available (player attacker).
	if opts.Target.GetMobInstanceId() > 0 && actor.GetRoom() != nil {
		dupIdx := actor.GetRoom().GetMobDuplicateIndex(opts.Target.GetMobInstanceId())
		indexed := targetChar.GetMobNameIndexed(actor.GetUserId(), dupIdx)
		targetName = indexed.String()
	}

	defenderMaxHP := targetChar.HealthMax.Value
	defenderMitigation := targetChar.GetPhysicalMitigation()

	// --- Attacker stats ---
	attackerChar := actor.GetCharacter()
	dex := attackerChar.Stats.Dexterity.ValueAdj
	skillRank := attackerChar.GetSkillLevel(skills.Skullduggery)

	// Surprise multiplier: (dex + skillRank) / 100, minimum 1.0
	surpriseMult := math.Max(1.0, float64(dex+skillRank)/100.0)

	// --- Collect attack weapons (mirrors the player implementation) ---
	type weaponEntry struct {
		itemId     int
		name       string
		dmgMult    float64
		hitPenalty float64 // fraction: 0.0 = no penalty, 0.10 = 10% miss added
	}

	weapons := []weaponEntry{}

	// Primary weapon
	if attackerChar.Equipment.Weapon.ItemId > 0 {
		spec := attackerChar.Equipment.Weapon.GetSpec()
		wm := spec.DamageMultiplier
		if wm <= 0 {
			wm = float64(cfg.UnarmedDamageMultiplier)
		}
		weapons = append(weapons, weaponEntry{
			itemId:     attackerChar.Equipment.Weapon.ItemId,
			name:       spec.NameSimple,
			dmgMult:    wm,
			hitPenalty: 0.0,
		})
	}

	// Offhand weapon
	if attackerChar.Equipment.Offhand.ItemId > 0 {
		offSpec := attackerChar.Equipment.Offhand.GetSpec()
		if offSpec.Type == items.Weapon {
			wm := offSpec.DamageMultiplier
			if wm <= 0 {
				wm = float64(cfg.UnarmedDamageMultiplier)
			}
			weapons = append(weapons, weaponEntry{
				itemId:     attackerChar.Equipment.Offhand.ItemId,
				name:       offSpec.NameSimple,
				dmgMult:    wm,
				hitPenalty: float64(cfg.SurpriseAttackOffhandPenalty),
			})
		}
	}

	// Extra arm 1
	if attackerChar.ExtraArms >= 1 && attackerChar.Equipment.ExtraArm1.ItemId > 0 {
		ea1Spec := attackerChar.Equipment.ExtraArm1.GetSpec()
		if ea1Spec.Type == items.Weapon {
			wm := ea1Spec.DamageMultiplier
			if wm <= 0 {
				wm = float64(cfg.UnarmedDamageMultiplier)
			}
			weapons = append(weapons, weaponEntry{
				itemId:     attackerChar.Equipment.ExtraArm1.ItemId,
				name:       ea1Spec.NameSimple,
				dmgMult:    wm,
				hitPenalty: float64(cfg.SurpriseAttackExtraArm1Penalty),
			})
		}
	}

	// Extra arm 2
	if attackerChar.ExtraArms >= 2 && attackerChar.Equipment.ExtraArm2.ItemId > 0 {
		ea2Spec := attackerChar.Equipment.ExtraArm2.GetSpec()
		if ea2Spec.Type == items.Weapon {
			wm := ea2Spec.DamageMultiplier
			if wm <= 0 {
				wm = float64(cfg.UnarmedDamageMultiplier)
			}
			weapons = append(weapons, weaponEntry{
				itemId:     attackerChar.Equipment.ExtraArm2.ItemId,
				name:       ea2Spec.NameSimple,
				dmgMult:    wm,
				hitPenalty: float64(cfg.SurpriseAttackExtraArm2Penalty),
			})
		}
	}

	// Extra arm 3
	if attackerChar.ExtraArms >= 3 && attackerChar.Equipment.ExtraArm3.ItemId > 0 {
		ea3Spec := attackerChar.Equipment.ExtraArm3.GetSpec()
		if ea3Spec.Type == items.Weapon {
			wm := ea3Spec.DamageMultiplier
			if wm <= 0 {
				wm = float64(cfg.UnarmedDamageMultiplier)
			}
			weapons = append(weapons, weaponEntry{
				itemId:     attackerChar.Equipment.ExtraArm3.ItemId,
				name:       ea3Spec.NameSimple,
				dmgMult:    wm,
				hitPenalty: float64(cfg.SurpriseAttackExtraArm3Penalty),
			})
		}
	}

	// Extra arm 4
	if attackerChar.ExtraArms >= 4 && attackerChar.Equipment.ExtraArm4.ItemId > 0 {
		ea4Spec := attackerChar.Equipment.ExtraArm4.GetSpec()
		if ea4Spec.Type == items.Weapon {
			wm := ea4Spec.DamageMultiplier
			if wm <= 0 {
				wm = float64(cfg.UnarmedDamageMultiplier)
			}
			weapons = append(weapons, weaponEntry{
				itemId:     attackerChar.Equipment.ExtraArm4.ItemId,
				name:       ea4Spec.NameSimple,
				dmgMult:    wm,
				hitPenalty: float64(cfg.SurpriseAttackExtraArm4Penalty),
			})
		}
	}

	// Fallback: unarmed
	if len(weapons) == 0 {
		weapons = append(weapons, weaponEntry{
			itemId:     0,
			name:       "fists",
			dmgMult:    float64(cfg.UnarmedDamageMultiplier),
			hitPenalty: 0.0,
		})
	}

	// --- Swing each weapon ---
	room := actor.GetRoom()
	attackerName := actor.GetName()
	strikeCount := 0
	anyHit := false

	for _, w := range weapons {
		// Per-weapon SELF-penalty: roll 0-99; if result < penaltyPct, this
		// weapon's swing is dropped.
		//
		// NOTE(U9): surprise attack has NO HIT RESOLUTION. The primary weapon is
		// appended above with hitPenalty 0.0, so penaltyPct is 0 and this roll
		// never fires for it -- every primary surprise swing is an automatic hit
		// regardless of the target. The roll only applies to offhand and
		// extra-arm swings, and there is no defender term anywhere: the target's
		// stats, skills and defences never enter. A surprise attack against a
		// novice and against the Elemental King resolve identically.
		//
		// U4 declined it. Giving it a defender is a behaviour change and U1-U5
		// are contracted as provable no-ops (standing rule 3); U3 made the same
		// call for the Position_GrappleTick z-normalisation. U9 owns it, as the
		// chunk that converts non-contests into contests (concentration,
		// knockdown, prone recovery). The user intends to REDESIGN this
		// skill/effect rather than only bolt a defender term onto it, so U9
		// should treat it as a design slice (brainstorm, spec, plan), not a
		// mechanical migration. Model the shift first -- every surprise attack
		// in the game moves.
		penaltyPct := int(w.hitPenalty * 100)
		roll := util.Rand(100)
		if roll < penaltyPct {
			// Missed this weapon swing due to hit penalty
			actor.SendText(messaging.CategorySystem,
				fmt.Sprintf(
					`<ansi fg="magenta-bold">*[SURPRISE ATTACK]*</ansi> `+
						`You swing your <ansi fg="item">%s</ansi> at `+
						`<ansi fg="mobname">%s</ansi> but miss!`,
					w.name, targetName,
				),
			)
			if room != nil {
				room.SendTextVisual(messaging.CategorySurpriseAttack,
					fmt.Sprintf(
						`<ansi fg="magenta-bold">*[SURPRISE ATTACK]*</ansi> `+
							`<ansi fg="username">%s</ansi> swings at %s from `+
							`the shadows but misses!`,
						attackerName, targetName,
					),
					actor.GetUserId(),
				)
			}
			continue
		}

		// Damage pipeline: CalcRawDamage → half-mitigation bypass →
		// surprise multiplier → dice.RollStat variance
		rawDmg := combat.CalcRawDamage(
			attackerChar.Stats.Strength.ValueAdj,
			skillRank,
			w.dmgMult,
			combat.ChannelPhysical,
		)

		// Surprise attack: apply only half of normal mitigation (crit-like bypass)
		halfMitigation := defenderMitigation * 0.5
		dmgMean := combat.ApplyMitigation(
			rawDmg,
			halfMitigation,
			combat.MitigationCap(combat.ChannelPhysical),
		)

		// Apply surprise multiplier
		dmgMean *= surpriseMult

		// Variance roll
		dmgResult := dice.RollStat(dmgMean)
		dmg := int(math.Round(math.Max(1.0, dmgResult.Value)))

		anyHit = true
		strikeCount++

		// Apply damage to target
		targetChar.ApplyHarm(characters.PoolHealth, dmg,
			state.ActorRef{UserId: attackerChar.GetUserId(), MobInstanceId: attackerChar.MobInstanceId})
		// NOTE(U5b-2): this health floor is inconsistent with the ~19 other
		// health-damage sites, which let health store overkill. U5b-1 kept it so
		// that PR stays provably behaviour-neutral; U5b-2 removes all seven such
		// floors together as one named, playtested change.
		if targetChar.Health < 0 {
			targetChar.Health = 0
		}

		// Per-weapon hit messages
		dmgDesc := combat.GetDamageDescription(dmg, defenderMaxHP)

		actor.SendText(messaging.CategorySystem,
			fmt.Sprintf(
				`<ansi fg="magenta-bold">*[SURPRISE ATTACK]*</ansi> `+
					`Your <ansi fg="item">%s</ansi> strikes `+
					`<ansi fg="mobname">%s</ansi> from the shadows! `+
					`(<ansi fg="damage">%s</ansi>)`,
				w.name, targetName, dmgDesc,
			),
		)

		if room != nil {
			room.SendTextVisual(messaging.CategorySurpriseAttack,
				fmt.Sprintf(
					`<ansi fg="magenta-bold">*[SURPRISE ATTACK]*</ansi> `+
						`<ansi fg="username">%s</ansi>'s <ansi fg="item">%s</ansi> `+
						`strikes %s from the shadows!`,
					attackerName, w.name, targetName,
				),
				actor.GetUserId(),
			)
		}

		// Notify the target if they are a player
		opts.Target.SendText(messaging.CategorySystem,
			fmt.Sprintf(
				`<ansi fg="magenta-bold">*[SURPRISE ATTACK]*</ansi> `+
					`<ansi fg="username">%s</ansi>'s <ansi fg="item">%s</ansi> `+
					`strikes you from the shadows! (<ansi fg="damage">%s</ansi>)`,
				attackerName, w.name, dmgDesc,
			),
		)
	}

	if !anyHit {
		// All weapons missed — still reveal the attacker's position
		actor.SendText(messaging.CategorySystem,
			fmt.Sprintf(
				`<ansi fg="magenta-bold">*[SURPRISE ATTACK]*</ansi> `+
					`You lunge at <ansi fg="mobname">%s</ansi> from `+
					`the shadows, but miss!`,
				targetName,
			),
		)
		if room != nil {
			room.SendTextVisual(messaging.CategorySurpriseAttack,
				fmt.Sprintf(
					`<ansi fg="magenta-bold">*[SURPRISE ATTACK]*</ansi> `+
						`<ansi fg="username">%s</ansi> lunges at %s from `+
						`the shadows, but misses!`,
					attackerName, targetName,
				),
				actor.GetUserId(),
			)
		}
	}

	// --- Skill progression ---
	actor.OnSkillUse(string(skills.Skullduggery))

	return SurpriseAttackResult{
		Triggered:   true,
		StrikeCount: strikeCount,
	}
}

// EngageAggroType fires the pre-combat surprise burst and reports the Aggro
// type the resulting engagement should carry.
//
// SurpriseAttack gates internally on both hidden state AND the special-move
// cooldown, so "was a surprise attack actually landed?" is exactly its
// Triggered flag — not IsHidden(). Callers must not pre-check IsHidden: a
// hidden-but-on-cooldown opener lands no surprise strikes and is an ordinary
// attack.
//
// This exists because the four engagement sites (usercommands/attack.go PvM and
// PvP, mobcommands/attack.go vs-player and vs-mob) each derived the type
// themselves, and they drifted: the player paths hardcoded DefaultAttack even
// when opening from stealth, while the mob paths keyed off IsHidden alone. That
// meant a player's stealth opener and a mob's produced different Aggro.Type
// values for the same situation, and downstream combat code keys off that
// field.
func EngageAggroType(actor Actor, target Actor) characters.AggroType {
	if res := SurpriseAttack(actor, SurpriseAttackOpts{Target: target}); res.Triggered {
		return characters.SurpriseAttack
	}
	return characters.DefaultAttack
}
