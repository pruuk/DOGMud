package hooks

import (
	"fmt"
	"math"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// calcSpellDuration computes a universal spell duration in rounds based on
// the spell's fold count, the caster's spellcasting skill, and willpower.
// Higher folds, skill, and willpower all extend duration.
// Formula: baseFolds × (10 + willpower/20 + spellcastingSkill/2)
func calcSpellDuration(baseFolds int, spellcastingSkill int, willpower int) int {
	if baseFolds < 1 {
		baseFolds = 4
	}
	duration := float64(baseFolds) * (10.0 + float64(willpower)/20.0 + float64(spellcastingSkill)/2.0)
	if duration < 10 {
		duration = 10
	}
	return int(math.Round(duration))
}

// resolveSpell is called when fold accumulation completes for a player caster.
// It dispatches to per-target resolution based on spell type and effect.
//
// Why this is NOT merged with resolveMobSpell:
//   - resolveSpell handles the "identify" spell type (no mob equivalent).
//   - HarmArea populates only mob targets for players; resolveMobSpell also
//     hits players in the room (mobs can cleave all occupants).
//   - HelpArea is player-only (mobs never cast area healing in this engine).
//   - Player targets go through resolveAgainstPlayer which has a help-spell
//     shortcut (TargetDefenseType == "") absent in the mob path.
//   - Post-resolution: player fires the onMagic script and consumes a
//     component; mob does neither.
//   - The per-target helpers (resolveAgainstMob vs resolveMobSpellAgainstMob,
//     resolveAgainstPlayer vs resolveMobSpellAgainstPlayer) have fundamentally
//     different signatures, messaging, and combat-record calls.
//
// Extracting the 6-line loop skeleton into a shared wrapper would require
// function-parameter callbacks or an interface, adding abstraction without
// meaningful savings. Keep them separate and well-documented instead.
// playerHarmTargetPermitted reports whether a player-cast spell of this type
// may land on mob right now.
//
// Spells fold over several rounds, so the target set chosen by InitiateCast is
// stale by the time the spell resolves: a mob can be charmed into a companion,
// or a builder can flag it protected, in between. Harmful spells therefore
// re-run the same authorization policy at resolution (review finding 3).
//
// Help spells are exempt — they legitimately target companions.
func playerHarmTargetPermitted(spellType spells.SpellType, mob *mobs.Mob) bool {
	switch spellType {
	case spells.HarmSingle, spells.HarmMulti, spells.HarmArea:
		return !mobs.CheckPlayerHarm(mob).Blocked()
	}
	return true
}

func resolveSpell(user *users.UserRecord, cs activity.CastingData, spellData *spells.SpellData, room *rooms.Room) {

	side := spellAttackSideFor(spellData, user.Character)
	magnitude := spellData.EffectMagnitude

	// --- Identify: resolve against caster's item, no targets ---
	if spellData.EffectType == "identify" {
		resolveIdentify(user, cs.SpellRest, room)
		return
	}

	// --- Populate area targets for HarmArea ---
	if spellData.Type == spells.HarmArea {
		allMobs := room.GetMobs(rooms.FindAll)
		filtered := make([]int, 0, len(allMobs))
		for _, mId := range allMobs {
			// Spare companions, non-combatants and attack-immune mobs.
			if !playerHarmTargetPermitted(spellData.Type, mobs.GetInstance(mId)) {
				continue
			}
			filtered = append(filtered, mId)
		}
		cs.TargetMobInstanceIds = filtered
	}

	// --- Populate area targets for HelpArea ---
	if spellData.Type == spells.HelpArea {
		cs.TargetUserIds = room.GetPlayers(rooms.FindAll)
		// Apply to ally mobs only (charmed/companion). REPLACES any residual
		// TargetMobInstanceIds from the cast's pre-resolution step —
		// otherwise the caster's pre-spell aggro target (an enemy mob) gets
		// healed alongside intended allies. Symmetric with HarmArea above.
		allMobs := room.GetMobs(rooms.FindAll)
		allies := make([]int, 0, len(allMobs))
		for _, mId := range allMobs {
			if m := mobs.GetInstance(mId); m != nil && m.Character.IsCharmed() {
				allies = append(allies, mId)
			}
		}
		cs.TargetMobInstanceIds = allies
	}

	// --- Resolve against mob targets ---
	// castFumbled tracks whether ANY per-target roll fumbled (ZScore <= -2.0).
	// A fumble gates the post-target effects (summon, charm, Go hooks) below
	// so a summon-spell caster who fumbles doesn't still get the companion.
	castFumbled := false
	targetsResolved := 0
	for _, mobInstId := range cs.TargetMobInstanceIds {
		mob := mobs.GetInstance(mobInstId)
		if mob == nil || mob.Character.Health < 1 {
			continue
		}
		if mob.Character.RoomId != room.RoomId {
			continue // target left the room before spell resolved
		}
		if !playerHarmTargetPermitted(spellData.Type, mob) {
			continue // gained protection while the spell was folding
		}
		if resolveAgainstMob(user, mob, room, spellData, side, magnitude) {
			castFumbled = true
		}
		targetsResolved++
	}

	// --- Resolve against player targets ---
	for _, targetUserId := range cs.TargetUserIds {
		targetUser := users.GetByUserId(targetUserId)
		if targetUser == nil {
			continue
		}
		if targetUser.Character.RoomId != room.RoomId {
			user.SendText(messaging.CategorySpellDisruption, fmt.Sprintf(`Your spell dissipates, unspent. <ansi fg="username">%s</ansi> is no longer here.`, targetUser.Character.Name))
			continue // target left the room before spell resolved
		}
		// Skip downed players for harm spells — they're already down.
		if targetUser.Character.Health < 1 &&
			(spellData.Type == spells.HarmSingle || spellData.Type == spells.HarmArea || spellData.Type == spells.HarmMulti) {
			continue
		}
		if spellData.TargetDefenseType == "" {
			// Help spell with no defense — always applies, as an uncontested
			// attack win (full multiplier, no crit, no defence to narrate).
			applyPlayerEffect(user, targetUser, room, spellData, magnitude, combat.ChannelDefenceResult{DamageMultiplier: 1})
		} else {
			if resolveAgainstPlayer(user, targetUser, room, spellData, side, magnitude) {
				castFumbled = true
			}
		}
		targetsResolved++
	}

	// --- Empty room / no valid targets feedback ---
	// Skip for summon/charm spells — they handle their own targeting via Go functions
	isSummonOrCharm := spellData != nil && (spellData.SummonMobId > 0 || spellData.EffectType == "charm")
	if targetsResolved == 0 && !isSummonOrCharm {
		user.SendText(messaging.CategorySpellDisruption, `Your spell erupts outward but finds no targets.`)
		sendVisualRoomText(room, messaging.CategorySpellDisruption, fmt.Sprintf(
			`<ansi fg="username">%s</ansi>'s spell crackles through the air harmlessly.`,
			user.Character.Name), user.UserId)
	}

	// --- Run spell script onMagic (if present) ---
	// Send YAML magic text (if defined).
	if spellData != nil && (spellData.MagicUserText != "" || spellData.MagicRoomText != "") {
		tCtx := textutil.TokenContext{
			SourceName:      user.Character.GetCharacterName(true),
			SourcePlainName: user.Character.GetCharacterName(false),
		}
		if len(cs.TargetUserIds) > 0 {
			if tUser := users.GetByUserId(cs.TargetUserIds[0]); tUser != nil {
				tCtx.TargetName = tUser.Character.GetCharacterName(true)
				tCtx.TargetPlainName = tUser.Character.GetCharacterName(false)
			}
		} else if len(cs.TargetMobInstanceIds) > 0 {
			if tMob := mobs.GetInstance(cs.TargetMobInstanceIds[0]); tMob != nil {
				tCtx.TargetName = tMob.Character.GetCharacterName(true)
				tCtx.TargetPlainName = tMob.Character.GetCharacterName(false)
			}
		}
		cfg := textutil.SendTextConfig{
			UserSendFunc: func(msg string) { user.SendText(spellSchoolCategory(spellData), msg) },
			RoomSendFunc: func(msg string, skip ...int) {
				if r := rooms.LoadRoom(user.Character.RoomId); r != nil {
					r.SendText(spellSchoolCategory(spellData), msg, skip...)
				}
			},
			ExcludeId: user.UserId,
		}
		textutil.SendPhaseText(spellData.MagicUserText, spellData.MagicRoomText, tCtx, "pink", cfg)
	}
	// Fumble gate for the post-target effects (summon / charm / Go hooks).
	// A fumbled cast consumed conviction + component but should NOT also land
	// the primary effect. A single flavor message; individual blocks skip
	// silently so we don't spam the player.
	if castFumbled && spellData != nil &&
		(spellData.SummonMobId > 0 || spellData.EffectType == "charm" ||
			cs.SpellId == "fold-anchor" || cs.SpellId == "fold-recall" || cs.SpellId == "purge-affliction") {
		user.SendText(messaging.CategorySpellDisruption, `<ansi fg="red">The weave unravels — the spell fails to take shape.</ansi>`)
	}

	// Resolve companion summon (if configured)
	if !castFumbled && spellData != nil && spellData.SummonMobId > 0 {
		resolveCompanionSummon(user, spellData, cs.SpellRest, room)
	}
	// Resolve charm spell
	if !castFumbled && spellData != nil && spellData.EffectType == "charm" {
		if len(cs.TargetMobInstanceIds) > 0 {
			if targetMob := mobs.GetInstance(cs.TargetMobInstanceIds[0]); targetMob != nil {
				resolveCharmSpell(user, targetMob, room)
			}
		}
	}

	// --- Go spell hooks — dispatch before JS scripts ---
	// Fumble aborts the hook body but falls through to the component-consume
	// block below so the catalyst is still used up.
	if !castFumbled {
		switch cs.SpellId {
		case "fold-anchor":
			resolveFoldAnchor(actions.NewUserActorInRoom(user, room))
			return
		case "fold-recall":
			resolveFoldRecall(actions.NewUserActorInRoom(user, room))
			return
		case "purge-affliction":
			if len(cs.TargetUserIds) > 0 {
				if targetUser := users.GetByUserId(cs.TargetUserIds[0]); targetUser != nil {
					resolvePurgeAffliction(user, targetUser)
				}
			} else {
				resolvePurgeAffliction(user, user) // self-cast
			}
			return
		}
	}

	// --- Consume component if required ---
	if spellData.ComponentTag != "" {
		consumeSpellComponent(user, spellData.ComponentTag)
	}
}

// runSpellChannelAttack is THE spell-contest seam (U6b Task 4): every spell
// resolver — player-cast and mob-cast — runs its ONE contest through it and
// threads the ChannelDefenceResult into the effect appliers, which consume it
// instead of rolling their own. It defaults to the canonical resolver;
// same-package dispatch tests replace it briefly with a literal outcome and
// restore it with t.Cleanup. Tests that need the seam's REAL side effects
// (cost admission, the progression bonus tier) leave this alone and swap the
// contest core via combat.SetChannelAttackContestRunnerForTest instead.
var runSpellChannelAttack = combat.ResolveChannelAttack

// spellAttackSideFor builds the caster's half of the one spell contest. The
// hit contest finally honours the spell's U9 primarystat: the score is the
// spell's own casting stat plus the school's governing skill, weighted by
// SkillWeight — the deleted hit-gate helper multiplied the weighted skill by
// a config skill factor (x3) on top, the x15-per-rank outlier U6b removes.
//
// StatName mirrors CasterStatValue's default: an empty primarystat reads as
// willpower there, so the progression events must name willpower too, not "".
func spellAttackSideFor(spellData *spells.SpellData, casterChar *characters.Character) combat.AttackSide {
	castSkill := skills.Spellcasting
	if spellData.HasSchool(spells.SchoolManifestation) {
		castSkill = skills.Manifestation
	}
	statName := spellData.PrimaryStat
	if statName == "" {
		statName = "willpower"
	}
	return combat.AttackSide{
		Stat:      spellData.CasterStatValue(casterChar.Stats),
		StatName:  statName,
		Skill:     castSkill,
		SkillRank: casterChar.GetSkillLevel(castSkill),
		Mult:      1.0,
	}
}

// scaleSpellDamageByDefence applies the threaded contest's damage multiplier:
// 1.0 on an attack win, 0.0 on a defensive crit, 0.0-0.5 on a rolled
// defensive win, exactly 0.5 on a floored save — the same semantics
// ExecuteSkillMove documents. A defended hit deals at least 1 damage unless
// the defence critted.
func scaleSpellDamageByDefence(dmg int, out combat.ChannelDefenceResult) int {
	mult := out.DamageMultiplier
	if mult >= 1.0 {
		return dmg
	}
	dmg = int(math.Round(float64(dmg) * mult))
	if dmg < 1 && mult > 0 {
		dmg = 1
	}
	return dmg
}

// resolveAgainstMob runs the ONE channel contest and applies the effect to a
// mob. Returns true if the cast fumbled (the seam's self-relative
// AttackerFumble). A fumble aborts any post-target spell effects (summon,
// charm, Go hooks) in the caller's main flow; component consumption still
// fires (the failed binding uses up the catalyst regardless).
func resolveAgainstMob(user *users.UserRecord, mob *mobs.Mob, room *rooms.Room, spellData *spells.SpellData, side combat.AttackSide, magnitude int) (fumbled bool) {

	out := runSpellChannelAttack(spellAttackChannel(spellData), side, user.Character, &mob.Character)

	round := util.GetRoundCount()

	// Backfire on fumble — resolved BEFORE success, per the seam's contract:
	// a fumbled cast aborts even a winning roll.
	if out.AttackerFumble {
		backfireDmg := magnitude / 4
		if backfireDmg < 1 {
			backfireDmg = 1
		}
		user.Character.ApplyHarm(characters.PoolHealth, backfireDmg, charActorRef(user.Character))
		user.SendText(messaging.CategorySpellDisruption, `<ansi fg="red">Your spell backfires violently, wounding you!</ansi>`)
		sendVisualRoomText(room, messaging.CategorySpellDisruption, fmt.Sprintf(
			`<ansi fg="red"><ansi fg="username">%s</ansi>'s spell backfires!</ansi>`, user.Character.Name), user.UserId)
		// Stage 30.1: Record backfire
		combat.RecordSpell(combat.User, combat.Mob, false, false, true, false, 0, out.AttackRollZScore, user.Character, &mob.Character, round)
		return true
	}

	// Boss-interrupt: a disruption spell cast at a mid-fold-cast mob cancels the
	// cast whether or not the target defends the damage — the interrupt is the
	// point, and a tanky boss shouldn't dodge it. (Backfires return above, so a
	// botched cast still can't interrupt.)
	if maybeInterruptSpellOnMob(mob, spellData.SpellId, state.ActorRef{UserId: user.UserId}) {
		user.SendText(messaging.CategorySpellDisruption, fmt.Sprintf(
			`<ansi fg="cyan-bold">Your %s scrambles %s's focus -- its spell collapses!</ansi>`,
			spellData.Name, mobDisplayName(mob, room, user.UserId)))
		sendVisualRoomText(room, messaging.CategorySpellDisruption, fmt.Sprintf(
			`<ansi fg="cyan">%s's spell collapses!</ansi>`,
			mobDisplayName(mob, room, user.UserId)), user.UserId)
	}

	dmgDealt := applyMobEffect(user, user.Character, mob, room, spellData, magnitude, out)
	// Stage 30.1: a defended cast records in the old fizzle column — the
	// defence stopped or blunted it — but keeps its partial damage.
	combat.RecordSpell(combat.User, combat.Mob, !out.Defended, out.AttackerCrit, false, out.Defended, dmgDealt, out.AttackRollZScore, user.Character, &mob.Character, round)

	// U6b Task 10: the MOB defender's crit defence counters the player caster.
	fireSpellCounterTier(room, out, spellAttackChannel(spellData),
		&mob.Character, user.Character, nil, user)

	return false
}

// maybeInterruptSpellOnMob cancels a mob's in-progress fold-cast if spellId
// is a configured boss-interrupt disruption spell (Balance.BossInterruptSpellIds)
// AND the mob is currently casting (Character.IsCasting()). Non-allowlisted
// spells never interrupt, even on a successful hit. Reuses the shared
// InterruptTargetCast primitive (conviction refund + TriggerCastCancel)
// rather than reimplementing cast cancellation here. Returns true if a cast
// was actually interrupted.
func maybeInterruptSpellOnMob(mob *mobs.Mob, spellId string, by state.ActorRef) bool {
	if mob == nil {
		return false
	}
	if !configs.GetBalanceConfig().IsBossInterruptSpell(spellId) {
		return false
	}
	if !mob.Character.IsCasting() {
		return false
	}
	return actions.InterruptTargetCast(&mob.Character, by)
}

// spellSchoolCategory picks the messaging Category from a spell's
// first declared school. Falls back to CategorySpellElemental if the
// spell has no school tag — the historical default for damage spells.
// A spell with multiple schools (rare) uses the first; the school
// list order in YAML is the author's preference.
func spellSchoolCategory(spellData *spells.SpellData) messaging.Category {
	if spellData == nil || len(spellData.Schools) == 0 {
		return messaging.CategorySpellElemental
	}
	switch spellData.Schools[0] {
	case spells.SchoolElemental:
		return messaging.CategorySpellElemental
	case spells.SchoolEnhancement:
		return messaging.CategorySpellEnhancement
	case spells.SchoolMental:
		return messaging.CategorySpellMental
	case spells.SchoolVital:
		return messaging.CategorySpellVital
	case spells.SchoolManifestation:
		return messaging.CategorySpellManifestation
	}
	return messaging.CategorySpellElemental
}

// sendSpellChannelDefenceMessages renders one canonical defence triad and
// applies the spell path's existing visual audience routing. Nil user records
// represent mob participants, which do not receive private player text.
func sendSpellChannelDefenceMessages(room *rooms.Room, category messaging.Category,
	out combat.ChannelDefenceResult, attackerName, defenderName, attackName string,
	attackerUser, defenderUser *users.UserRecord, indexOverride ...int) {
	if defenderUser != nil {
		if text := combat.ChannelDefenceShortageText(out, defenderUser.Character); text != "" {
			defenderUser.SendText(messaging.CategorySystem, text)
		}
	}
	triad := combat.RenderChannelDefenceMessages(out, combat.ChannelDefenceIdentities{
		Attacker: attackerName,
		Defender: defenderName,
	}, attackName, indexOverride...)
	if triad.ToRoom == "" {
		return
	}
	excluded := make([]int, 0, 2)
	if attackerUser != nil {
		if room != nil {
			room.SendTextVisualToUser(attackerUser, category, string(triad.ToAttacker))
		} else {
			attackerUser.SendText(category, string(triad.ToAttacker))
		}
		excluded = append(excluded, attackerUser.UserId)
	}
	if defenderUser != nil {
		if room != nil {
			room.SendTextVisualToUser(defenderUser, category, string(triad.ToDefender))
		} else {
			defenderUser.SendText(category, string(triad.ToDefender))
		}
		excluded = append(excluded, defenderUser.UserId)
	}
	if room != nil {
		sendVisualRoomText(room, category, string(triad.ToRoom), excluded...)
	}
}

// spellDefenceIdentity returns the display-ready identity for either kind of
// spell participant. Mob identities retain the room's duplicate index.
func spellDefenceIdentity(char *characters.Character, user *users.UserRecord, room *rooms.Room) string {
	if char == nil {
		return ""
	}
	if user != nil {
		return char.GetPlayerName(user.UserId).String()
	}
	if room != nil && char.MobInstanceId > 0 {
		if mob := mobs.GetInstance(char.MobInstanceId); mob != nil {
			return mobDisplayName(mob, room, 0)
		}
	}
	return char.GetMobName(0).String()
}

// setMobSpellAggro sets reciprocal aggro between the caster and the
// mob target immediately after a hostile spell lands.
//
// Note: applyMobEffect_buff does NOT call this helper — its aggro block
// is gated on spell Type being Harm*. Kept inline there.
func setMobSpellAggro(user *users.UserRecord, mob *mobs.Mob) {
	if !mob.Character.IsInCombat() {
		if user != nil {
			mob.Character.SetAggro(user.UserId, 0, characters.DefaultAttack)
		}
	}
	if user != nil && !user.Character.IsInCombat() {
		user.Character.SetAggro(0, mob.InstanceId, characters.DefaultAttack)
	}
}

// applyMobEffect_damage handles the "damage" EffectType case for applyMobEffect.
// Returns damage dealt to the mob.
func applyMobEffect_damage(
	user *users.UserRecord,
	casterChar *characters.Character,
	mob *mobs.Mob,
	room *rooms.Room,
	spellData *spells.SpellData,
	magnitude int,
	out combat.ChannelDefenceResult,
	critTag string,
	mName string,
) int {
	// U6b Task 4: one contest per cast. The resolver already ran it; this
	// applier CONSUMES the threaded result. A defended cast is a PARTIAL
	// outcome (the spell still lands, for less); the defensive crit zeroes it.
	dmg := calcSpellDamageForCharacter(spellData, casterChar, &mob.Character, magnitude, out.AttackerCrit)
	dmg = scaleSpellDamageByDefence(dmg, out)
	mob.Character.ApplyHarm(characters.PoolHealth, dmg, charActorRef(casterChar))
	cancelDamageBuffs(&mob.Character)
	// on_spell_hit item procs (e.g. Staff of the Hollow Choir CP-steal) fire
	// only on a landing harm hit that dealt damage. This applier is shared by
	// the player-caster→mob and mob-caster→mob paths, so wiring it here covers
	// both. For AoE/multi-target casts the dispatch runs once per damaged
	// target; the proc's own chance+cooldown pace it, so one cast steals from
	// at most a few targets before the cooldown gate closes — intended.
	if dmg > 0 {
		dispatchItemProcs("on_spell_hit", casterChar, &mob.Character, nil, dmg)
	}
	setMobSpellAggro(user, mob)
	sendSpellChannelDefenceMessages(room, spellSchoolCategory(spellData), out,
		spellDefenceIdentity(casterChar, user, room), mName, spellData.Name, user, nil)
	if user != nil {
		if !out.Defended {
			user.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
				`Your %s strikes %s! (<ansi fg="damage">%s</ansi>)%s`,
				spellData.Name, mName, combat.GetDamageDescription(dmg, mob.Character.HealthMax.Value), critTag))
			sendVisualRoomText(room, spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="username">%s</ansi>'s <ansi fg="cyan">%s</ansi> strikes %s!`,
				user.Character.Name, spellData.Name, mName), user.UserId)
		}
	}
	return dmg
}

// applyMobEffect_dot handles the "dot" EffectType case for applyMobEffect.
// Returns 0 (no immediate damage; condition is applied for periodic ticks).
//
// The affliction is a binary status: like ExecuteSkillMove's StatusApplied,
// it lands only on an attack win. A defended cast narrates the channel
// defence triad and applies nothing (there is no partial dot).
func applyMobEffect_dot(
	user *users.UserRecord,
	casterChar *characters.Character,
	mob *mobs.Mob,
	room *rooms.Room,
	spellData *spells.SpellData,
	magnitude int,
	out combat.ChannelDefenceResult,
	critTag string,
	mName string,
) int {
	if out.Defended {
		setMobSpellAggro(user, mob)
		sendSpellChannelDefenceMessages(room, spellSchoolCategory(spellData), out,
			spellDefenceIdentity(casterChar, user, room), mName, spellData.Name, user, nil)
		return 0
	}
	casterSkill := 0
	casterWil := 100
	if user != nil {
		casterSkill = user.Character.GetSkillLevel(skills.Spellcasting)
		casterWil = spellData.CasterStatValue(user.Character.Stats)
	}
	dotDuration := calcSpellDuration(spellData.BaseFolds, casterSkill, casterWil) / 3
	if dotDuration < 3 {
		dotDuration = 3
	}
	mob.Character.AddCondition(characters.ConditionPoisoned, dotDuration, float64(magnitude), "spell")
	setMobSpellAggro(user, mob)
	if user != nil {
		user.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
			`Your %s afflicts %s!%s`,
			spellData.Name, mName, critTag))
		sendVisualRoomText(room, spellSchoolCategory(spellData), fmt.Sprintf(
			`<ansi fg="username">%s</ansi>'s <ansi fg="cyan">%s</ansi> afflicts %s!`,
			user.Character.Name, spellData.Name, mName), user.UserId)
	}
	return 0
}

// applyMobEffect_knockdown handles the "knockdown" EffectType case for applyMobEffect.
// Returns damage dealt to the mob.
//
// U6b Task 4: one contest per cast. The defence scales the DAMAGE (a defended
// cast still lands a partial hit), while the knockdown is a binary status —
// there is no partially knocked down — that lands only on an attack win,
// exactly ExecuteSkillMove's Hit/StatusApplied split. Before the collapse the
// hit gate and the defence were two separate contests, so a defended target
// could "still go down"; with one contest a defence win IS the miss, and the
// knockdown is stopped with it.
func applyMobEffect_knockdown(
	user *users.UserRecord,
	casterChar *characters.Character,
	mob *mobs.Mob,
	room *rooms.Room,
	spellData *spells.SpellData,
	magnitude int,
	out combat.ChannelDefenceResult,
	critTag string,
	mName string,
) int {
	dmg := calcSpellDamageForCharacter(spellData, casterChar, &mob.Character, magnitude, out.AttackerCrit)
	dmg = scaleSpellDamageByDefence(dmg, out)
	return applyMobKnockdownOutcome(user, casterChar, mob, room, spellData, dmg, out, critTag, mName)
}

// applyMobKnockdownOutcome applies the already-scaled damage and, on an attack
// win only, the binary knockdown. See applyMobEffect_knockdown for the split.
func applyMobKnockdownOutcome(
	user *users.UserRecord,
	casterChar *characters.Character,
	mob *mobs.Mob,
	room *rooms.Room,
	spellData *spells.SpellData,
	dmg int,
	out combat.ChannelDefenceResult,
	critTag string,
	mName string,
) int {
	mob.Character.ApplyHarm(characters.PoolHealth, dmg, charActorRef(casterChar))
	cancelDamageBuffs(&mob.Character)
	if dmg > 0 {
		dispatchItemProcs("on_spell_hit", casterChar, &mob.Character, nil, dmg)
	}
	// Chunk 4b W5 cutover: spell knockdowns default to Supine (the
	// "slams to the ground" wording fits backward force). Skip the
	// legacy parallel-write if the FSM transition fails so the two
	// views stay consistent.
	knocked := false
	if !out.Defended {
		knocked = true
		if err := mob.Character.Position.TransitionToSupine(
			position.SupineData{MinRecoveryRounds: 1},
			state.TransitionReason{Trigger: position.TriggerKnockdownSpell},
		); err != nil {
			mudlog.Warn("applyMobEffect_knockdown: TransitionToSupine failed", "mob", mob.InstanceId, "err", err)
			// Target was already grappled/prone — the spell hit and dealt damage,
			// but no knockdown occurred; don't narrate one (grapple move-collision).
			knocked = false
		}
	}
	setMobSpellAggro(user, mob)
	sendSpellChannelDefenceMessages(room, spellSchoolCategory(spellData), out,
		spellDefenceIdentity(casterChar, user, room), mName, spellData.Name, user, nil)
	// A defended cast earns no strike line: the defence triad above already
	// narrated the outcome, and a defended cast never knocks down.
	if user != nil && !out.Defended {
		if !knocked {
			user.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
				`Your %s strikes %s, but %s is already down. (<ansi fg="damage">%s</ansi>)%s`,
				spellData.Name, mName, mName, combat.GetDamageDescription(dmg, mob.Character.HealthMax.Value), critTag))
		} else {
			user.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
				`Your %s slams %s to the ground! (<ansi fg="damage">%s</ansi>)%s`,
				spellData.Name, mName, combat.GetDamageDescription(dmg, mob.Character.HealthMax.Value), critTag))
		}
		if knocked {
			sendVisualRoomText(room, spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="username">%s</ansi>'s <ansi fg="cyan">%s</ansi> knocks %s to the ground!`,
				user.Character.Name, spellData.Name, mName), user.UserId)
		}
	}
	return dmg
}

func applyMobEffect_buff(
	user *users.UserRecord,
	casterChar *characters.Character,
	mob *mobs.Mob,
	room *rooms.Room,
	spellData *spells.SpellData,
	out combat.ChannelDefenceResult,
	critTag string,
	mName string,
) int {
	// U6b Task 4: a buff is a binary status — a defended cast narrates the
	// channel defence triad and applies nothing. Hostile intent still aggros
	// (the harm-type gate below is shared with the landed path).
	if out.Defended {
		sendSpellChannelDefenceMessages(room, spellSchoolCategory(spellData), out,
			spellDefenceIdentity(casterChar, user, room), mName, spellData.Name, user, nil)
		if spellData.Type == spells.HarmSingle || spellData.Type == spells.HarmArea || spellData.Type == spells.HarmMulti {
			setMobSpellAggro(user, mob)
		}
		return 0
	}
	for _, buffId := range spellData.BuffIds {
		mob.AddBuff(buffId, "spell")
		// Compute tick snapshot for config-driven buffs
		if user != nil {
			if buffSpec := buffs.GetBuffSpec(buffId); buffSpec != nil && buffSpec.TickPool != "" {
				skillLevel := user.Character.GetSkillLevel(skills.Spellcasting)
				scalingMult := combat.SkillMultiplier(skillLevel)
				// Apply weapon spell damage multiplier if equipped, scaled
				// by gear-effectiveness for incorporeal casters.
				if user.Character.Equipment.Weapon.ItemId > 0 {
					if weaponSpec := items.GetItemSpec(user.Character.Equipment.Weapon.ItemId); weaponSpec != nil && weaponSpec.SpellDamageMultiplier > 0 {
						scalingMult *= weaponSpec.SpellDamageMultiplier * mutations.GearEffectivenessMultiplier(user.Character.Mutations)
					}
				}
				var maxPool int
				switch buffSpec.TickPool {
				case "health":
					maxPool = mob.Character.HealthMax.Value
				case "stamina":
					maxPool = mob.Character.StaminaMax.Value
				case "conviction":
					maxPool = mob.Character.ConvictionMax.Value
				}
				tickAmt := buffs.ComputeTickAmount(maxPool, buffSpec.TickPercent, buffSpec.TickVariance, buffSpec.TickMin, scalingMult)
				mob.Character.Buffs.SetTickAmount(buffId, tickAmt)
			}
		}
	}
	// Conditional aggro for harmful buff spells — kept inline because it is
	// gated on Harm* spell types; not consolidated in Task 7's setMobSpellAggro.
	if spellData.Type == spells.HarmSingle || spellData.Type == spells.HarmArea || spellData.Type == spells.HarmMulti {
		if !mob.Character.IsInCombat() {
			if user != nil {
				mob.Character.SetAggro(user.UserId, 0, characters.DefaultAttack)
			}
		}
		if user != nil && !user.Character.IsInCombat() {
			user.Character.SetAggro(0, mob.InstanceId, characters.DefaultAttack)
		}
	}
	if user != nil {
		user.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
			`Your %s takes effect on %s!%s`,
			spellData.Name, mName, critTag))
		sendVisualRoomText(room, spellSchoolCategory(spellData), fmt.Sprintf(
			`<ansi fg="username">%s</ansi>'s <ansi fg="cyan">%s</ansi> affects %s!`,
			user.Character.Name, spellData.Name, mName), user.UserId)
	}
	return 0
}

// applyMobEffect_heal handles the "heal" EffectType case for applyMobEffect —
// a caster (mob or player) casting a HelpSingle heal at ANOTHER mob (e.g. an
// ally construct healing a boss, or a player healing a charmed companion).
// Prior to Chunk B of the crash-site boss-mechanics work this case did not
// exist: applyMobEffect's switch only handled damage/dot/knockdown/buff, so
// a mob-to-mob (or player-to-companion) "heal" cast silently fell through to
// applyMobEffect_default and did nothing. Mirrors applyMobSelfEffect's
// "heal" case (percentage-of-max regen via ConditionRegen) but targets
// `mob` instead of the caster. Returns 0 (no damage dealt) to match the
// applyMobEffect_* int-return convention.
func applyMobEffect_heal(
	casterChar *characters.Character,
	mob *mobs.Mob,
	room *rooms.Room,
	spellData *spells.SpellData,
	magnitude int,
	mName string,
) int {
	skillLevel := 0
	willpower := 0
	casterName := "Something"
	if casterChar != nil {
		skillLevel = casterChar.GetSkillLevel(skills.Spellcasting)
		willpower = spellData.CasterStatValue(casterChar.Stats)
		casterName = casterChar.Name
	}
	regenMult := float64(magnitude)
	if regenMult < 1.0 {
		regenMult = 1.0
	}
	durationRounds := calcSpellDuration(spellData.BaseFolds, skillLevel, willpower) / 2
	if durationRounds < 6 {
		durationRounds = 6
	}
	mob.Character.AddCondition(characters.ConditionRegen, durationRounds, regenMult, "heal spell")
	sendVisualRoomText(room, messaging.CategorySpellVital, fmt.Sprintf(
		`<ansi fg="cyan">%s</ansi>'s %s washes over %s, knitting wounds shut.`,
		casterName, spellData.Name, mName))
	return 0
}

func applyMobEffect_default(
	user *users.UserRecord,
	casterChar *characters.Character,
	room *rooms.Room,
	spellData *spells.SpellData,
	out combat.ChannelDefenceResult,
	mName string,
) int {
	if out.Defended {
		sendSpellChannelDefenceMessages(room, spellSchoolCategory(spellData), out,
			spellDefenceIdentity(casterChar, user, room), mName, spellData.Name, user, nil)
		return 0
	}
	if user != nil {
		user.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
			`Your %s takes effect on %s.`,
			spellData.Name, mName))
	}
	return 0
}

// applyMobEffect applies the spell effect to a mob and returns damage dealt (0 for non-damage effects).
// user may be nil when the caster is a mob (guards all user.* references).
// casterChar is the caster's Character pointer (may be nil for mob-on-mob when unavailable).
//
// U6b Task 4: `out` is the resolver's ONE channel contest, threaded through.
// The appliers consume it — damage scaling, defence narration, the
// defensive-crit negation — instead of rolling a contest of their own.
func applyMobEffect(user *users.UserRecord, casterChar *characters.Character, mob *mobs.Mob, room *rooms.Room, spellData *spells.SpellData, magnitude int, out combat.ChannelDefenceResult) int {
	critTag := ""
	if out.AttackerCrit {
		critTag = ` <ansi fg="yellow">[CRIT!]</ansi>`
	}
	viewerId := 0
	if user != nil {
		viewerId = user.UserId
	}
	mName := mobDisplayName(mob, room, viewerId)

	switch spellData.EffectType {
	case "damage":
		return applyMobEffect_damage(user, casterChar, mob, room, spellData, magnitude, out, critTag, mName)
	case "dot":
		return applyMobEffect_dot(user, casterChar, mob, room, spellData, magnitude, out, critTag, mName)
	case "knockdown":
		return applyMobEffect_knockdown(user, casterChar, mob, room, spellData, magnitude, out, critTag, mName)
	case "buff":
		return applyMobEffect_buff(user, casterChar, mob, room, spellData, out, critTag, mName)
	case "heal":
		return applyMobEffect_heal(casterChar, mob, room, spellData, magnitude, mName)
	default:
		return applyMobEffect_default(user, casterChar, room, spellData, out, mName)
	}
}

// resolveAgainstPlayer runs the ONE channel contest and applies the effect to
// a player. Returns true if the cast fumbled (the seam's self-relative
// AttackerFumble). See resolveAgainstMob for the fumble semantics carrying
// over to summon/charm/Go-hook gating.
//
// Crit-received toughening for the defender now fires INSIDE the seam's bonus
// tier (combat.ResolveChannelAttack -> awardChannelDefenceBonus), which is why
// there is no direct ApplyProgression call here any more — the U9-era block
// this function used to carry became a duplicate the moment the seam saw the
// crit, and the once-per-round dedupe would have masked the double-fire
// rather than prevented it.
func resolveAgainstPlayer(user *users.UserRecord, target *users.UserRecord, room *rooms.Room, spellData *spells.SpellData, side combat.AttackSide, magnitude int) (fumbled bool) {

	out := runSpellChannelAttack(spellAttackChannel(spellData), side, user.Character, target.Character)

	// Backfire on fumble — resolved BEFORE success, per the seam's contract.
	if out.AttackerFumble {
		backfireDmg := magnitude / 4
		if backfireDmg < 1 {
			backfireDmg = 1
		}
		user.Character.ApplyHarm(characters.PoolHealth, backfireDmg, charActorRef(user.Character))
		user.SendText(messaging.CategorySpellDisruption, `<ansi fg="red">Your spell backfires violently, wounding you!</ansi>`)
		sendVisualRoomText(room, messaging.CategorySpellDisruption, fmt.Sprintf(
			`<ansi fg="red"><ansi fg="username">%s</ansi>'s spell backfires!</ansi>`, user.Character.Name), user.UserId)
		return true
	}

	applyPlayerEffect(user, target, room, spellData, magnitude, out)

	// Set reciprocal aggro for harm spells
	if spellData.Type == spells.HarmSingle || spellData.Type == spells.HarmArea || spellData.Type == spells.HarmMulti {
		if !user.Character.IsInCombat() {
			user.Character.SetAggro(target.UserId, 0, characters.DefaultAttack)
		}
		if !target.Character.IsInCombat() {
			target.Character.SetAggro(user.UserId, 0, characters.DefaultAttack)
		}
	}

	// U6b Task 10: the defending player's crit defence counters the caster.
	fireSpellCounterTier(room, out, spellAttackChannel(spellData),
		target.Character, user.Character, target, user)

	return false
}

// applyPlayerEffect applies the spell effect to a player target.
//
// U6b Task 4: `out` is the resolver's ONE channel contest, threaded through
// (help spells with no defense pass an uncontested attack win). Non-damage
// effects are binary statuses: a defended cast narrates the channel defence
// triad and applies nothing, mirroring ExecuteSkillMove's StatusApplied split.
func applyPlayerEffect(user *users.UserRecord, target *users.UserRecord, room *rooms.Room, spellData *spells.SpellData, magnitude int, out combat.ChannelDefenceResult) {

	critTag := ""
	if out.AttackerCrit {
		critTag = ` <ansi fg="yellow">[CRIT!]</ansi>`
	}

	if out.Defended && spellData.EffectType != "damage" {
		sendSpellChannelDefenceMessages(room, spellSchoolCategory(spellData), out,
			spellDefenceIdentity(user.Character, user, room),
			spellDefenceIdentity(target.Character, target, room), spellData.Name, user, target)
		return
	}

	switch spellData.EffectType {
	case "damage":
		// One contest, on the channel's own defence set — already run by the
		// resolver. A defended cast deals partial damage; the defensive crit
		// negates it entirely.
		dmg := calcSpellDamageForCharacter(spellData, user.Character, target.Character, magnitude, out.AttackerCrit)
		dmg = scaleSpellDamageByDefence(dmg, out)
		sendSpellChannelDefenceMessages(room, spellSchoolCategory(spellData), out,
			spellDefenceIdentity(user.Character, user, room),
			spellDefenceIdentity(target.Character, target, room), spellData.Name, user, target)
		if out.DefensiveCrit {
			return
		}
		target.Character.ApplyHarm(characters.PoolHealth, dmg, charActorRef(user.Character))
		cancelDamageBuffs(target.Character)
		if dmg > 0 {
			dispatchItemProcs("on_spell_hit", user.Character, target.Character, nil, dmg)
		}
		dmgDesc := combat.GetDamageDescription(dmg, target.Character.HealthMax.Value)
		if !out.Defended {
			user.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
				`Your %s strikes `+
					`<ansi fg="username">%s</ansi>! `+
					`(<ansi fg="damage">%s</ansi>)%s`,
				spellData.Name, target.Character.Name, dmgDesc, critTag))
			sendVisualRoomText(room, spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="username">%s</ansi>'s <ansi fg="cyan">%s</ansi> strikes `+
					`<ansi fg="username">%s</ansi>!`,
				user.Character.Name, spellData.Name, target.Character.Name),
				user.UserId, target.UserId)
			target.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="red"><ansi fg="username">%s</ansi>'s `+
					`%s strikes you! `+
					`(<ansi fg="damage">%s</ansi>)</ansi>`,
				user.Character.Name, spellData.Name,
				combat.GetDamageDescription(dmg, target.Character.HealthMax.Value)))
		}

	case "purge":
		target.Character.CancelBuffsWithFlag(buffs.Poison)
		target.Character.RemoveCondition(characters.ConditionPoisoned)
		user.SendText(messaging.CategorySpellVital, fmt.Sprintf(
			`<ansi fg="green">Your %s cleanses <ansi fg="username">%s</ansi> of afflictions.%s</ansi>`,
			spellData.Name, target.Character.Name, critTag))
		if target.UserId != user.UserId {
			target.SendText(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="green"><ansi fg="username">%s</ansi>'s %s purges the toxins from your body.</ansi>`,
				user.Character.Name, spellData.Name))
		} else {
			target.SendText(messaging.CategorySpellVital, `<ansi fg="green">You purge the afflictions from your body.</ansi>`)
		}
		sendVisualRoomText(room, messaging.CategorySpellVital, fmt.Sprintf(
			`<ansi fg="username">%s</ansi>'s <ansi fg="cyan">%s</ansi> cleanses <ansi fg="username">%s</ansi>.`,
			user.Character.Name, spellData.Name, target.Character.Name), user.UserId, target.UserId)

	case "heal":
		skillLevel := user.Character.GetSkillLevel(skills.Spellcasting)
		// Magnitude from YAML is the regen multiplier (e.g. 3 = 3x base regen)
		regenMult := float64(magnitude)
		if regenMult < 1.0 {
			regenMult = 1.0
		}
		if out.AttackerCrit {
			// Crit: boost the multiplier portion above 1x by 2x
			regenMult = 1.0 + (regenMult-1.0)*2.0
		}
		durationRounds := calcSpellDuration(spellData.BaseFolds, skillLevel, spellData.CasterStatValue(user.Character.Stats)) / 2
		if durationRounds < 6 {
			durationRounds = 6
		}
		target.Character.AddCondition(characters.ConditionRegen, durationRounds, regenMult, "heal spell")
		user.SendText(messaging.CategorySpellVital, fmt.Sprintf(
			`<ansi fg="green">You weave restorative magic around <ansi fg="username">%s</ansi>.%s</ansi>`,
			target.Character.Name, critTag))
		if target.UserId != user.UserId {
			target.SendText(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="green"><ansi fg="username">%s</ansi>'s %s envelops you in healing energy. Your wounds begin to mend.</ansi>`,
				user.Character.Name, spellData.Name))
		} else {
			target.SendText(messaging.CategorySpellVital, `<ansi fg="green">A warm glow of healing magic envelops you. Your wounds begin to mend.</ansi>`)
		}
		sendVisualRoomText(room, messaging.CategorySpellVital, fmt.Sprintf(
			`<ansi fg="username">%s</ansi>'s <ansi fg="cyan">%s</ansi> envelops <ansi fg="username">%s</ansi> in healing light.`,
			user.Character.Name, spellData.Name, target.Character.Name), user.UserId, target.UserId)

	case "buff":
		for _, buffId := range spellData.BuffIds {
			target.AddBuff(buffId, "spell")
			// Compute tick snapshot for config-driven buffs
			if buffSpec := buffs.GetBuffSpec(buffId); buffSpec != nil && buffSpec.TickPool != "" {
				skillLevel := user.Character.GetSkillLevel(skills.Spellcasting)
				scalingMult := combat.SkillMultiplier(skillLevel)
				// Apply weapon spell damage multiplier if equipped, scaled
				// by gear-effectiveness for incorporeal casters.
				if user.Character.Equipment.Weapon.ItemId > 0 {
					if weaponSpec := items.GetItemSpec(user.Character.Equipment.Weapon.ItemId); weaponSpec != nil && weaponSpec.SpellDamageMultiplier > 0 {
						scalingMult *= weaponSpec.SpellDamageMultiplier * mutations.GearEffectivenessMultiplier(user.Character.Mutations)
					}
				}
				var maxPool int
				switch buffSpec.TickPool {
				case "health":
					maxPool = target.Character.HealthMax.Value
				case "stamina":
					maxPool = target.Character.StaminaMax.Value
				case "conviction":
					maxPool = target.Character.ConvictionMax.Value
				}
				tickAmt := buffs.ComputeTickAmount(maxPool, buffSpec.TickPercent, buffSpec.TickVariance, buffSpec.TickMin, scalingMult)
				target.Character.Buffs.SetTickAmount(buffId, tickAmt)
			}
		}
		user.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
			`Your %s takes effect on <ansi fg="username">%s</ansi>!%s`,
			spellData.Name, target.Character.Name, critTag))
		if target.UserId != user.UserId {
			target.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="username">%s</ansi>'s %s takes effect on you!`,
				user.Character.Name, spellData.Name))
		}

	case "shield":
		skillLevel := user.Character.GetSkillLevel(skills.Spellcasting)
		weightedSkill := int(math.Round(float64(skillLevel) * float64(configs.GetBalanceConfig().SkillWeight)))
		shieldBonus := (spellData.CasterStatValue(user.Character.Stats) + weightedSkill) / 3
		if shieldBonus < 1 {
			shieldBonus = 1
		}
		// Scale shield strength by spell magnitude (100 = 1.0x baseline)
		if magnitude > 0 {
			shieldBonus = int(math.Round(float64(shieldBonus) * float64(magnitude) / 100.0))
			if shieldBonus < 1 {
				shieldBonus = 1
			}
		}
		duration := calcSpellDuration(spellData.BaseFolds, skillLevel, spellData.CasterStatValue(user.Character.Stats))
		if out.AttackerCrit {
			shieldBonus = int(float64(shieldBonus) * 1.5)
		}
		target.Character.AddCondition(characters.ConditionShield, duration, float64(shieldBonus), "spell")
		target.SendText(spellSchoolCategory(spellData), `A shimmering magical barrier forms around you, bolstering your defenses.`)
		if target.UserId != user.UserId {
			user.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
				`A shimmering magical barrier forms around <ansi fg="username">%s</ansi>, bolstering their defenses.`,
				target.Character.Name))
		}
		sendVisualRoomText(room, spellSchoolCategory(spellData), fmt.Sprintf(
			`A shimmering barrier surrounds <ansi fg="username">%s</ansi>.`, target.Character.Name), target.UserId)

	default:
		user.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
			`Your %s takes effect on <ansi fg="username">%s</ansi>.`,
			spellData.Name, target.Character.Name))
	}
}

// spellAttackChannel maps a spell's target_defense_type onto the U6 attack
// channel whose defence set answers it.
//
// Since U6b Task 4 this is the ONLY read of target_defense_type in spell
// resolution: the field picks which defence set answers the one contest
// (a "physical" spell is dodged/blocked, everything else is quelled), and the
// defender's score comes from GetDefenseScoreFor via the seam — the deleted
// deleted defence-value helper's raw-stat read is gone with the two-contest gate.
//
// Everything that is not explicitly "physical" -- including "mental", "none" and
// the empty default -- answers as mental. That is the conservative direction:
// quell is a single-defence set, so an unclassified spell faces one defence
// rather than two.
func spellAttackChannel(spellData *spells.SpellData) combat.AttackChannel {
	if spellData != nil && spellData.TargetDefenseType == "physical" {
		return combat.ChannelSpellPhysical
	}
	return combat.ChannelSpellMental
}

// calcSpellDamage and calcMobSpellDamage have been unified into
// calcSpellDamageForCharacter() in combat_shared_helpers.go (Stage 38.1).

// consumeSpellComponent removes the first matching component item from caster's inventory.
func consumeSpellComponent(user *users.UserRecord, tag string) {
	for i, itm := range user.Character.Items {
		if itm.GetSpec().ComponentTag == tag {
			user.Character.Items = append(user.Character.Items[:i], user.Character.Items[i+1:]...)
			user.SendText(messaging.CategorySystem, fmt.Sprintf(
				`<ansi fg="yellow">You consume a %s as a spell component.</ansi>`, tag))
			return
		}
	}
}

// resolveMobSpell is called when a mob's fold accumulation completes.
// resolveMobSpell is called when fold accumulation completes for a mob caster.
// It dispatches to per-target resolution based on spell type and effect.
//
// Why this is NOT merged with resolveSpell (see that function for details):
//   - HarmArea here populates both mob AND player targets; player casters only
//     hit mobs (players in the room are excluded from player-cast area spells).
//   - Mob targets include a self-cast branch (applyMobSelfEffect) for help
//     spells; player casters never self-target via this dispatcher.
//   - No onMagic script, no component consumption.
//   - Per-target helpers are entirely separate from the player equivalents.
func resolveMobSpell(mob *mobs.Mob, cs activity.CastingData, spellData *spells.SpellData, room *rooms.Room) {
	// Go spell hooks — dispatch position-mutating / non-target spells before
	// the type-based effect routing below. Mirrors the player path in
	// resolveSpell. Stage 3.0d.
	switch cs.SpellId {
	case "fold-anchor":
		resolveFoldAnchor(actions.NewMobActorInRoom(mob, room))
		return
	case "fold-recall":
		actor := actions.NewMobActorInRoom(mob, room)
		if !validateFoldRecall(actor) {
			return
		}
		resolveFoldRecall(actor)
		return
	}

	// drain_area is a boss-ability effect type: it drains every living
	// player in the room and heals the caster by the aggregate lifesteal
	// (actions.ExecuteDrainArea). It bypasses the HarmArea target
	// population + per-target opposed-roll dispatch below entirely — the
	// area drain resolves its own per-player hit/miss via ExecuteSkillMove
	// inside ExecuteDrainArea, so running it through the generic
	// spellAttack-vs-defense roll here would double-roll each player.
	// Reachable ONLY at fold-cast completion (handleMobFoldCasting calls
	// resolveMobSpell here), so a spell authored with EffectType
	// "drain_area" and BaseFolds >= 2 telegraphs and is interruptible for
	// free — this function never runs until the cast finishes.
	if spellData.EffectType == "drain_area" {
		resolveMobDrainArea(mob, room, spellData)
		return
	}

	side := spellAttackSideFor(spellData, &mob.Character)
	magnitude := spellData.EffectMagnitude

	if spellData.Type == spells.HarmArea {
		allMobs := room.GetMobs(rooms.FindAll)
		filtered := make([]int, 0, len(allMobs))
		charmedByUserId := mob.Character.GetCharmedUserId()
		for _, mId := range allMobs {
			if mId == mob.InstanceId {
				continue // don't target self
			}
			// If this mob is charmed by a player, don't hit that player's other companions
			// Also never hit non-combatant mobs (shopkeepers etc.)
			if m := mobs.GetInstance(mId); m != nil {
				if m.IsNonCombatant() {
					continue
				}
				if charmedByUserId > 0 && m.Character.IsCharmed(charmedByUserId) {
					continue
				}
			}
			filtered = append(filtered, mId)
		}
		cs.TargetMobInstanceIds = filtered
		cs.TargetUserIds = room.GetPlayers(rooms.FindAll)
		// If charmed, don't hit the owner
		if charmedByUserId > 0 {
			ownerFiltered := make([]int, 0, len(cs.TargetUserIds))
			for _, pId := range cs.TargetUserIds {
				if pId != charmedByUserId {
					ownerFiltered = append(ownerFiltered, pId)
				}
			}
			cs.TargetUserIds = ownerFiltered
		}
	}

	for _, mobInstId := range cs.TargetMobInstanceIds {
		if mobInstId == mob.InstanceId {
			// Self-cast (HelpSingle with self target)
			applyMobSelfEffect(mob, room, spellData, magnitude)
			continue
		}
		if target := mobs.GetInstance(mobInstId); target != nil && target.Character.Health > 0 && target.Character.RoomId == room.RoomId {
			resolveMobSpellAgainstMob(mob, target, room, spellData, side, magnitude)
		}
	}
	for _, userId := range cs.TargetUserIds {
		if target := users.GetByUserId(userId); target != nil && target.Character.RoomId == room.RoomId {
			resolveMobSpellAgainstPlayer(mob, target, room, spellData, side, magnitude)
		}
	}
}

// resolveMobDrainArea is the resolution handler for a mob-cast spell whose
// EffectType is "drain_area" (the Core Guardian's "core recharge" ability
// design — see docs/superpowers/plans/2026-07-06-crashsite-boss-mechanics.md
// Chunk D). It drains every living player in the room and heals the caster
// by the aggregate lifesteal via actions.ExecuteDrainArea (which mirrors the
// single-target vampire ExecuteDrain math exactly).
//
// Author's note for the spell YAML that will invoke this (Task D2): give it
// effect_type: drain_area, a type that reads as a room-wide harm ability
// (e.g. harm-area) for AI-targeting purposes even though this handler
// ignores the generic HarmArea per-target dispatch, and base_folds >= 2 so
// it telegraphs via the existing fold-cast windup and is interruptible via
// the disruptor system — this function only runs once fold accumulation
// completes (handleMobFoldCasting -> resolveMobSpell -> here), so telegraph
// and interrupt are inherited for free; no changes needed here for either.
func resolveMobDrainArea(mob *mobs.Mob, room *rooms.Room, spellData *spells.SpellData) {
	result := actions.ExecuteDrainArea(actions.NewMobActorInRoom(mob, room))

	if !result.Executed {
		sendVisualRoomText(room, messaging.CategorySpellDisruption, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> crackles through the air, finding no one to drain.`,
			mob.Character.Name, spellData.Name))
		return
	}

	// Core Charge (crash-site-boss-mechanics Chunk D, the Core Guardian's
	// drain-fed discharge gate): incremented HERE, at drain *resolution*,
	// not at cast-initiation. Build-time decision (Task D3): an interrupted
	// drain never reaches this point at all -- resolveMobDrainArea is only
	// entered once a fold-cast completes (see the doc comment above), so a
	// disruptor-cancelled drain automatically denies the charge without any
	// extra guard, satisfying spec §10.4 ("an interrupted drain ... denies
	// the charge/heal entirely").
	//
	// BehaviorState (mob.BTreeState) is a per-mob-instance store that lives
	// on the mob itself (internal/mobs/mobs.go); behaviortree.EnsureBTreeState
	// lazily initializes and returns it. It is the EXACT SAME object the
	// btree's own `increment_state`/`state_greater_than` actions/conditions
	// read and write during tree evaluation (see internal/behaviortree/
	// actions_state.go, conditions_state.go) -- there is no separate storage
	// to keep in sync. Writing it here from Go is therefore equivalent to a
	// btree `increment_state` call, just triggered from the spell-resolution
	// side (which is the only place that knows "the drain actually landed")
	// rather than from the tree (which cannot observe fold-cast completion
	// directly). This lets the Core Guardian's btree
	// (9562-the_core_guardian.yaml) gate its core-discharge purely on
	// `state_greater_than core_charge N`, with zero Go-side awareness of
	// discharge itself.
	chargeState := behaviortree.EnsureBTreeState(mob)
	chargeState.Set("core_charge", chargeState.GetInt("core_charge")+1)

	for _, pr := range result.PlayerResults {
		target := users.GetByUserId(pr.UserId)
		if target == nil {
			continue
		}
		if !pr.MoveResult.Hit && pr.MoveResult.Damage == 0 {
			// Defended with zero damage (a defensive crit). This used to be a
			// silent miss; U6b Task 9 speaks the defence triad so the player
			// who fully stopped the pull learns what saved them.
			sendSpellChannelDefenceMessages(room, spellSchoolCategory(spellData), pr.MoveResult.Defence,
				spellDefenceIdentity(&mob.Character, nil, room),
				spellDefenceIdentity(target.Character, target, room),
				spellData.Name, nil, target)
			continue
		}
		if pr.MoveResult.Hit {
			target.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> saps your strength! (<ansi fg="damage">%s</ansi>)`,
				mob.Character.Name, spellData.Name,
				combat.GetDamageDescription(pr.MoveResult.Damage, target.Character.HealthMax.Value)))
			if !target.Character.IsInCombat() {
				target.Character.SetAggro(0, mob.InstanceId, characters.DefaultAttack)
			}
		} else {
			// Defended, but the drain still landed a partial pull. Since
			// Task 13 a defended maneuver can deal partial damage; say so
			// instead of letting the player's HP drop with no message at all.
			target.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> fails to take full hold of you, but still saps a little of your strength! (<ansi fg="damage">%s</ansi>)`,
				mob.Character.Name, spellData.Name,
				combat.GetDamageDescription(pr.MoveResult.Damage, target.Character.HealthMax.Value)))
		}
	}

	sendVisualRoomText(room, spellSchoolCategory(spellData), fmt.Sprintf(
		`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> tears the life from everyone in the room!`,
		mob.Character.Name, spellData.Name))

	// U6b Task 11: each earned counter renders AFTER the drain's own outcome.
	for _, pr := range result.PlayerResults {
		actions.DispatchCounterMessages(actions.NewMobActorInRoom(mob, room), pr.Counter)
	}
}

// applyMobSelfEffect handles self-targeted help spells (heal, minor-shield).
func applyMobSelfEffect(mob *mobs.Mob, room *rooms.Room, spellData *spells.SpellData, magnitude int) {
	switch spellData.EffectType {
	case "heal":
		skillLevel := mob.Character.GetSkillLevel(skills.Spellcasting)
		regenMult := float64(magnitude)
		if regenMult < 1.0 {
			regenMult = 1.0
		}
		durationRounds := calcSpellDuration(spellData.BaseFolds, skillLevel, spellData.CasterStatValue(mob.Character.Stats)) / 2
		if durationRounds < 6 {
			durationRounds = 6
		}
		mob.Character.AddCondition(characters.ConditionRegen, durationRounds, regenMult, "heal spell")
		sendVisualRoomText(room, messaging.CategorySpellVital, fmt.Sprintf(
			`%s channels restorative magic.`, mobDisplayName(mob, room, 0)))
	case "buff":
		for _, buffId := range spellData.BuffIds {
			mob.AddBuff(buffId, "spell")
			// Compute tick snapshot for config-driven buffs (matches
			// applyMobEffect_buff for consistency across all caster paths).
			if buffSpec := buffs.GetBuffSpec(buffId); buffSpec != nil && buffSpec.TickPool != "" {
				skillLevel := mob.Character.GetSkillLevel(skills.Spellcasting)
				scalingMult := combat.SkillMultiplier(skillLevel)
				var maxPool int
				switch buffSpec.TickPool {
				case "health":
					maxPool = mob.Character.HealthMax.Value
				case "stamina":
					maxPool = mob.Character.StaminaMax.Value
				case "conviction":
					maxPool = mob.Character.ConvictionMax.Value
				}
				tickAmt := buffs.ComputeTickAmount(maxPool, buffSpec.TickPercent, buffSpec.TickVariance, buffSpec.TickMin, scalingMult)
				mob.Character.Buffs.SetTickAmount(buffId, tickAmt)
			}
		}
	case "shield":
		skillLevel := mob.Character.GetSkillLevel(skills.Spellcasting)
		weightedSkill := int(math.Round(float64(skillLevel) * float64(configs.GetBalanceConfig().SkillWeight)))
		shieldBonus := (spellData.CasterStatValue(mob.Character.Stats) + weightedSkill) / 3
		if shieldBonus < 1 {
			shieldBonus = 1
		}
		// Scale shield strength by spell magnitude (100 = 1.0x baseline)
		if magnitude > 0 {
			shieldBonus = int(math.Round(float64(shieldBonus) * float64(magnitude) / 100.0))
			if shieldBonus < 1 {
				shieldBonus = 1
			}
		}
		duration := calcSpellDuration(spellData.BaseFolds, skillLevel, spellData.CasterStatValue(mob.Character.Stats))
		mob.Character.AddCondition(characters.ConditionShield, duration, float64(shieldBonus), "spell")
		sendVisualRoomText(room, spellSchoolCategory(spellData), fmt.Sprintf(
			`A shimmering barrier forms around %s.`, mobDisplayName(mob, room, 0)))
	}
}

func resolveMobSpellAgainstMob(caster *mobs.Mob, target *mobs.Mob, room *rooms.Room,
	spellData *spells.SpellData, side combat.AttackSide, magnitude int) {
	// Help-type effects (e.g. a construct add healing an ally boss) are a
	// cooperative cast, not an attack — the target should not roll defense
	// against a friendly heal, and a "fumble" backfire makes no sense for
	// it either. Bypass the contest/backfire gate entirely and apply
	// directly, as an uncontested attack win. (Crash-site boss-mechanics
	// Chunk B: the Repair Frame add heals Warden-Prime / the Core Guardian
	// this way.)
	if spellData.EffectType == "heal" {
		applyMobEffect(nil, &caster.Character, target, room, spellData, magnitude, combat.ChannelDefenceResult{DamageMultiplier: 1})
		return
	}
	out := runSpellChannelAttack(spellAttackChannel(spellData), side, &caster.Character, &target.Character)
	if out.AttackerFumble {
		dmg := magnitude / 4
		if dmg < 1 {
			dmg = 1
		}
		caster.Character.ApplyHarm(characters.PoolHealth, dmg, charActorRef(&caster.Character))
		sendVisualRoomText(room, messaging.CategorySpellDisruption, fmt.Sprintf(`<ansi fg="mobname">%s</ansi>'s spell backfires!`, caster.Character.Name))
		return
	}
	applyMobEffect(nil, &caster.Character, target, room, spellData, magnitude, out)

	// U6b Task 10: the defending mob's crit defence counters the mob caster.
	fireSpellCounterTier(room, out, spellAttackChannel(spellData),
		&target.Character, &caster.Character, nil, nil)
}

// resolveMobSpellAgainstPlayer runs the ONE channel contest for a mob-cast
// spell at a player and consumes the result inline (this path predates the
// applier split and keeps its inline effect arms). Crit-received toughening
// for the defender fires inside the seam's bonus tier — the U9-era direct
// block this function used to carry became a duplicate and was deleted with
// the collapse (U6b Task 4).
func resolveMobSpellAgainstPlayer(caster *mobs.Mob, target *users.UserRecord, room *rooms.Room,
	spellData *spells.SpellData, side combat.AttackSide, magnitude int) {
	out := runSpellChannelAttack(spellAttackChannel(spellData), side, &caster.Character, target.Character)
	round := util.GetRoundCount()
	if out.AttackerFumble {
		dmg := magnitude / 4
		if dmg < 1 {
			dmg = 1
		}
		caster.Character.ApplyHarm(characters.PoolHealth, dmg, charActorRef(&caster.Character))
		sendVisualRoomText(room, messaging.CategorySpellDisruption, fmt.Sprintf(`<ansi fg="mobname">%s</ansi>'s spell backfires!`, caster.Character.Name))
		// Stage 30.1: Record backfire
		combat.RecordSpell(combat.Mob, combat.User, false, false, true, false, 0, out.AttackRollZScore, &caster.Character, target.Character, round)
		return
	}
	isCrit := out.AttackerCrit
	mobSpellDmg := 0
	critTag := ""
	if isCrit {
		critTag = ` <ansi fg="yellow">[CRIT!]</ansi>`
	}
	switch spellData.EffectType {
	case "damage":
		// One contest (above), on the channel's own defence set. A defended
		// cast deals partial damage; the defensive crit negates it entirely.
		dmg := calcSpellDamageForCharacter(spellData, &caster.Character, target.Character, magnitude, isCrit)
		dmg = scaleSpellDamageByDefence(dmg, out)
		sendSpellChannelDefenceMessages(room, spellSchoolCategory(spellData), out,
			spellDefenceIdentity(&caster.Character, nil, room),
			spellDefenceIdentity(target.Character, target, room), spellData.Name, nil, target)
		if out.DefensiveCrit {
			break
		}
		mobSpellDmg = dmg
		target.Character.ApplyHarm(characters.PoolHealth, dmg, charActorRef(&caster.Character))
		cancelDamageBuffs(target.Character)
		if dmg > 0 {
			dispatchItemProcs("on_spell_hit", &caster.Character, target.Character, nil, dmg)
		}
		if !out.Defended {
			target.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> `+
					`strikes you! (<ansi fg="damage">%s</ansi>)%s`,
				caster.Character.Name, spellData.Name,
				combat.GetDamageDescription(dmg, target.Character.HealthMax.Value), critTag))
			sendVisualRoomText(room, spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> strikes `+
					`<ansi fg="username">%s</ansi>!`,
				caster.Character.Name, spellData.Name, target.Character.Name), target.UserId)
		}
		if !target.Character.IsInCombat() {
			target.Character.SetAggro(0, caster.InstanceId, characters.DefaultAttack)
		}
	case "dot":
		// The affliction is a binary status: it lands only on an attack win.
		// A defended cast narrates the channel defence triad and applies
		// nothing (there is no partial dot).
		if out.Defended {
			sendSpellChannelDefenceMessages(room, spellSchoolCategory(spellData), out,
				spellDefenceIdentity(&caster.Character, nil, room),
				spellDefenceIdentity(target.Character, target, room), spellData.Name, nil, target)
			if !target.Character.IsInCombat() {
				target.Character.SetAggro(0, caster.InstanceId, characters.DefaultAttack)
			}
			break
		}
		castSkill := skills.Spellcasting
		if spellData.HasSchool(spells.SchoolManifestation) {
			castSkill = skills.Manifestation
		}
		dotDuration := calcSpellDuration(spellData.BaseFolds, caster.Character.GetSkillLevel(castSkill), spellData.CasterStatValue(caster.Character.Stats)) / 3
		if dotDuration < 3 {
			dotDuration = 3
		}
		target.Character.AddCondition(characters.ConditionPoisoned, dotDuration, float64(magnitude), "spell")
		target.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> afflicts you!%s`,
			caster.Character.Name, spellData.Name, critTag))
		sendVisualRoomText(room, spellSchoolCategory(spellData), fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> afflicts <ansi fg="username">%s</ansi>!`,
			caster.Character.Name, spellData.Name, target.Character.Name), target.UserId)
		if !target.Character.IsInCombat() {
			target.Character.SetAggro(0, caster.InstanceId, characters.DefaultAttack)
		}
	case "knockdown":
		// Defence scales the DAMAGE (partial hit); the knockdown is a binary
		// status that lands only on an attack win — ExecuteSkillMove's
		// Hit/StatusApplied split, matching the player-cast branch above.
		dmg := calcSpellDamageForCharacter(spellData, &caster.Character, target.Character, magnitude, isCrit)
		dmg = scaleSpellDamageByDefence(dmg, out)
		mobSpellDmg = dmg
		target.Character.ApplyHarm(characters.PoolHealth, dmg, charActorRef(&caster.Character))
		if dmg > 0 {
			dispatchItemProcs("on_spell_hit", &caster.Character, target.Character, nil, dmg)
		}
		sendSpellChannelDefenceMessages(room, spellSchoolCategory(spellData), out,
			spellDefenceIdentity(&caster.Character, nil, room),
			spellDefenceIdentity(target.Character, target, room), spellData.Name, nil, target)
		// Chunk 4b W5 cutover: mob-cast knockdown on player. Same
		// Supine choice as the player-cast branch above.
		knocked := false
		if !out.Defended {
			knocked = true
			if err := target.Character.Position.TransitionToSupine(
				position.SupineData{MinRecoveryRounds: 1},
				state.TransitionReason{Trigger: position.TriggerKnockdownSpell},
			); err != nil {
				mudlog.Warn("mob spell knockdown: TransitionToSupine failed",
					"target_user", target.UserId, "err", err)
				// Already grappled/prone — spell hit + damaged, but no knockdown.
				knocked = false
			}
		}
		if knocked {
			target.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> slams you `+
					`to the ground! (<ansi fg="damage">%s</ansi>)%s`,
				caster.Character.Name, spellData.Name,
				combat.GetDamageDescription(dmg, target.Character.HealthMax.Value), critTag))
			sendVisualRoomText(room, spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> knocks `+
					`<ansi fg="username">%s</ansi> to the ground!`,
				caster.Character.Name, spellData.Name, target.Character.Name), target.UserId)
		} else if !out.Defended {
			target.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> strikes you, but you're `+
					`already down. (<ansi fg="damage">%s</ansi>)%s`,
				caster.Character.Name, spellData.Name,
				combat.GetDamageDescription(dmg, target.Character.HealthMax.Value), critTag))
		}
		if !target.Character.IsInCombat() {
			target.Character.SetAggro(0, caster.InstanceId, characters.DefaultAttack)
		}
	case "buff":
		// Binary status: a defended cast narrates the triad and applies nothing.
		if out.Defended {
			sendSpellChannelDefenceMessages(room, spellSchoolCategory(spellData), out,
				spellDefenceIdentity(&caster.Character, nil, room),
				spellDefenceIdentity(target.Character, target, room), spellData.Name, nil, target)
			if spellData.Type == spells.HarmSingle || spellData.Type == spells.HarmArea || spellData.Type == spells.HarmMulti {
				if !target.Character.IsInCombat() {
					target.Character.SetAggro(0, caster.InstanceId, characters.DefaultAttack)
				}
			}
			break
		}
		for _, buffId := range spellData.BuffIds {
			target.AddBuff(buffId, "spell")
		}
		// Set aggro for harmful buff spells
		if spellData.Type == spells.HarmSingle || spellData.Type == spells.HarmArea || spellData.Type == spells.HarmMulti {
			if !target.Character.IsInCombat() {
				target.Character.SetAggro(0, caster.InstanceId, characters.DefaultAttack)
			}
		}
		target.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> takes effect on you!%s`,
			caster.Character.Name, spellData.Name, critTag))
		sendVisualRoomText(room, spellSchoolCategory(spellData), fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> affects <ansi fg="username">%s</ansi>!`,
			caster.Character.Name, spellData.Name, target.Character.Name), target.UserId)
	default:
		if out.Defended {
			sendSpellChannelDefenceMessages(room, spellSchoolCategory(spellData), out,
				spellDefenceIdentity(&caster.Character, nil, room),
				spellDefenceIdentity(target.Character, target, room), spellData.Name, nil, target)
			break
		}
		target.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> takes effect on you.`,
			caster.Character.Name, spellData.Name))
	}
	// Stage 30.1: a defended cast records in the old fizzle column — the
	// defence stopped or blunted it — but keeps its partial damage.
	combat.RecordSpell(combat.Mob, combat.User, !out.Defended, isCrit, false, out.Defended, mobSpellDmg, out.AttackRollZScore, &caster.Character, target.Character, round)

	// U6b Task 10: the PLAYER defender's crit defence counters the mob caster.
	fireSpellCounterTier(room, out, spellAttackChannel(spellData),
		target.Character, &caster.Character, target, nil)
}

// resolveIdentify finds the named item on the caster and renders
// the identify template with descriptive item properties.
func resolveIdentify(user *users.UserRecord, itemName string, room *rooms.Room) {

	if itemName == "" {
		user.SendText(messaging.CategorySystem, "Identify what? (Usage: cast identify <item>)")
		return
	}

	// Search backpack and equipped items as a single pool
	matchItem, _, found := user.Character.FindItem(itemName)

	if !found {
		user.SendText(messaging.CategorySystem, "You can't seem to identify that.")
		return
	}

	iSpec := matchItem.GetSpec()

	type identifyDetails struct {
		Item     *items.Item
		ItemSpec *items.ItemSpec
	}

	details := identifyDetails{
		Item:     &matchItem,
		ItemSpec: &iSpec,
	}

	user.SendText(messaging.CategorySpellMental,
		fmt.Sprintf(`You concentrate on the <ansi fg="item">%s</ansi>...`,
			matchItem.DisplayName()),
	)
	sendVisualRoomText(room, messaging.CategorySpellMental,
		fmt.Sprintf(
			`<ansi fg="username">%s</ansi> concentrates on their <ansi fg="item">%s</ansi>...`,
			user.Character.Name, matchItem.DisplayName()),
		user.UserId,
	)

	identifyTxt, _ := templates.Process("descriptions/identify", details, user.UserId)
	user.SendText(messaging.CategorySpellMental, identifyTxt)
}
