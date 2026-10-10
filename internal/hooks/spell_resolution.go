package hooks

import (
	"fmt"
	"math"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// calcSpellDuration computes a universal spell duration in rounds based on
// the spell's fold count, the caster's spellcasting skill, and willpower.
// Higher folds, skill, and willpower all extend duration.
// Formula: baseFolds × (10 + willpower/20 + spellcastingSkill/2)
func calcSpellDuration(baseFolds int, spellcastingSkill int, willpower int) int {
	if baseFolds < 1 {
		baseFolds = 4
	}
	duration := float64(baseFolds) * spellDurationTerm(spellcastingSkill, willpower)
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
//   - HelpArea fills through spellHelpAreaTargets on both paths: a player
//     or a charmed mob helps the players and the party's companions, an
//     uncharmed mob helps its packmates.
//   - Both target paths take the non-harm shortcut (AttackType ==
//     combatvocab.AttackNone); the mob path gained it in M4b-2.
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
func playerHarmTargetPermitted(spellData *spells.SpellData, mob *mobs.Mob) bool {
	if spellData.IsHarm() {
		return !mobs.CheckPlayerHarm(mob).Blocked()
	}
	return true
}

func resolveSpell(user *users.UserRecord, cs activity.CastingData, spellData *spells.SpellData, room *rooms.Room) (anyLanded bool) {

	side := spellAttackSideFor(spellData, user.Character, combat.SightRoom(room))
	magnitude := spellData.EffectMagnitude

	// --- Identify: resolve against caster's item, no targets ---
	if spellData.EffectType == "identify" {
		resolveIdentify(user, cs.SpellRest, room)
		// Uncontested: there is no defence to beat, so it landed.
		return true
	}

	// --- Populate area targets for HarmArea ---
	if spellData.IsHarm() && spellData.Targeting == combatvocab.TargetArea {
		allMobs := room.GetMobs(rooms.FindAll)
		filtered := make([]int, 0, len(allMobs))
		for _, mId := range allMobs {
			// Spare companions, non-combatants and attack-immune mobs.
			if !playerHarmTargetPermitted(spellData, mobs.GetInstance(mId)) {
				continue
			}
			filtered = append(filtered, mId)
		}
		cs.TargetMobInstanceIds = filtered
	}

	// --- Populate area targets for HelpArea ---
	// REPLACES whatever the cast's initiation step filled in, so the
	// caster's pre-spell aggro target (an enemy mob) is not healed alongside
	// its allies. Symmetric with HarmArea above; the same filler serves mob
	// casters in resolveMobSpell.
	if !spellData.IsHarm() && spellData.Targeting == combatvocab.TargetArea {
		cs.TargetUserIds, cs.TargetMobInstanceIds = spellHelpAreaTargets(actions.NewUserActorInRoom(user, room), room)
	}

	// --- Resolve against mob targets ---
	// castFumbled tracks whether ANY per-target roll fumbled (ZScore <= -2.0).
	// A fumble gates the post-target effects (summon, charm, Go hooks) below
	// so a summon-spell caster who fumbles doesn't still get the companion.
	castFumbled := false
	// anyLanded (the NAMED RETURN) drives U10b-1 Task 13's ONE progression
	// award for the cast. ONE CAST IS ONE RESOLVED ACTION: a three-target spell
	// that beat one defence pays a single full-weight event, not three events
	// and not one per target hit. A cast every target defended pays the failure
	// fraction.
	targetsResolved := 0
	for _, mobInstId := range cs.TargetMobInstanceIds {
		mob := mobs.GetInstance(mobInstId)
		if mob == nil || mob.Character.Health < 1 {
			continue
		}
		if mob.Character.RoomId != room.RoomId {
			continue // target left the room before spell resolved
		}
		if !playerHarmTargetPermitted(spellData, mob) {
			continue // gained protection while the spell was folding
		}
		fumbled, landed := resolveAgainstMob(user, mob, room, spellData, side, magnitude)
		castFumbled = castFumbled || fumbled
		anyLanded = anyLanded || landed
		targetsResolved++
	}

	// --- Resolve against player targets ---
	for _, targetUserId := range cs.TargetUserIds {
		targetUser := users.GetByUserId(targetUserId)
		if targetUser == nil {
			continue
		}
		if targetUser.Character.RoomId != room.RoomId {
			// Named only as far as the caster can see in the caster's room
			// (#242): a caster who cannot make the room out reads
			// "Something is no longer here."
			user.SendText(messaging.CategorySpellDisruption, messaging.HideNames(
				fmt.Sprintf(`Your spell dissipates, unspent. <ansi fg="username">%s</ansi> is no longer here.`, targetUser.Character.Name),
				[]string{targetUser.Character.Name}, messaging.ParticipantSight(user.Character, room)))
			continue // target left the room before spell resolved
		}
		// Skip downed players for harm spells — they're already down.
		if targetUser.Character.Health < 1 && spellData.IsHarm() {
			continue
		}
		// A help spell (attack_type none) resolves uncontested inside
		// resolveAgainstPlayer, as it does in the other three resolvers.
		fumbled, landed := resolveAgainstPlayer(user, targetUser, room, spellData, side, magnitude)
		castFumbled = castFumbled || fumbled
		anyLanded = anyLanded || landed
		targetsResolved++
	}

	// --- Empty room / no valid targets feedback ---
	// Summons target nothing in the room, so the generic line does not apply.
	isSummon := spellData != nil && spellData.SummonMobId > 0
	isCharm := spellData != nil && spellData.EffectType == "charm"
	if targetsResolved == 0 && !isSummon {
		if isCharm {
			// Charm used to resolve AFTER this loop, reading
			// TargetMobInstanceIds[0] directly, so it did not care whether the
			// target survived the fold. Now that it resolves inside the loop it
			// does, and a 36-fold channel that finds nothing must still say so
			// -- silence after spending 120 conviction reads as a broken
			// command. The wording is charm's because "erupts outward" suits a
			// blast, not a held gaze.
			user.SendText(messaging.CategorySpellDisruption,
				`Your gaze finds nothing to hold, and the will you gathered scatters.`)
		} else {
			user.SendText(messaging.CategorySpellDisruption, `Your spell erupts outward but finds no targets.`)
			sendVisualRoomText(room, messaging.CategorySpellDisruption, fmt.Sprintf(
				`%s's spell crackles through the air harmlessly.`,
				playerSubjectName(user)), user.UserId)
			// The sound half (#242, owner ruling R4), for a reader who sees
			// nothing: what sendVisualElseAudible sends, kept as two calls so
			// the narration guard still sees this event's observer line.
			room.SendTextUnsighted(messaging.CategorySpellDisruption, messaging.SoundSpellSputtersOut, user.UserId)
		}
	}

	// --- Run spell script onMagic (if present) ---
	// Send YAML magic text (if defined).
	if spellData != nil && spellData.Narration(spells.PhaseMagic).Len() > 0 {
		tCtx := textutil.TokenContext{
			ActorName:      user.Character.GetCharacterName(true),
			ActorPlainName: user.Character.GetCharacterName(false),
		}
		if len(cs.TargetUserIds) > 0 {
			if tUser := users.GetByUserId(cs.TargetUserIds[0]); tUser != nil {
				tCtx.ActeeName = tUser.Character.GetCharacterName(true)
				tCtx.ActeePlainName = tUser.Character.GetCharacterName(false)
			}
		} else if len(cs.TargetMobInstanceIds) > 0 {
			if tMob := mobs.GetInstance(cs.TargetMobInstanceIds[0]); tMob != nil {
				tCtx.ActeeName = tMob.Character.GetCharacterName(true)
				tCtx.ActeePlainName = tMob.Character.GetCharacterName(false)
			}
		}
		roles := spellData.Narrate(spells.PhaseMagic, tCtx)
		if roles.Actor != "" {
			user.SendText(spellSchoolCategory(spellData), roles.Actor)
		}
		// Audio channel, as before this refactor: filed, not changed here.
		if roles.Observer != "" {
			if r := rooms.LoadRoom(user.Character.RoomId); r != nil {
				r.SendText(spellSchoolCategory(spellData), roles.Observer, user.UserId)
			}
		}
	}
	// Fumble gate for the post-target effects (summon / charm / Go hooks).
	// A fumbled cast consumed conviction + component but should NOT also land
	// the primary effect. A single flavor message; individual blocks skip
	// silently so we don't spam the player.
	if castFumbled && spellData != nil &&
		(spellData.SummonMobId > 0 || spellData.EffectType == "charm" ||
			cs.SpellId == "fold-anchor" || cs.SpellId == "fold-recall" || cs.SpellId == "purge-affliction") {
		user.SendText(messaging.CategorySpellDisruption, `<ansi fg="red">The weave unravels, and the spell fails to take shape.</ansi>`)
	}

	// Resolve companion summon (if configured)
	if !castFumbled && spellData != nil && spellData.SummonMobId > 0 {
		resolveCompanionSummon(user, spellData, cs.SpellRest, room)
	}
	// Charm used to resolve HERE, in a second private contest run after the
	// loop above had already contested every target and thrown the verdict
	// away. It now resolves inside the loop, in applySpellEffect's "charm" case,
	// off that one contest.
	//
	// Removing this block also fixes two live defects. The player no longer
	// sees a resist line and a success line for the same cast. And charm no
	// longer succeeds against a mob that died, left the room, or gained harm
	// protection mid-fold: this block read TargetMobInstanceIds[0] directly and
	// so ignored every filter the loop applies.

	// --- Go spell hooks — dispatch before JS scripts ---
	// Fumble aborts the hook body but falls through to the component-consume
	// block below so the catalyst is still used up.
	if !castFumbled {
		switch cs.SpellId {
		case "fold-anchor":
			resolveFoldAnchor(actions.NewUserActorInRoom(user, room))
			// Uncontested utility cast: no defence to beat.
			return true
		case "fold-recall":
			resolveFoldRecall(actions.NewUserActorInRoom(user, room))
			// Uncontested utility cast: no defence to beat.
			return true
		case "purge-affliction":
			// A mob target is read here for the same reason the other help
			// spells read it: a charmed companion is a legitimate target, and
			// this switch used to fall through to the self-cast arm whenever
			// only TargetMobInstanceIds was set, so naming a poisoned
			// companion purged the CASTER.
			//
			// Both named-target arms go through purgeTarget.stillPresent,
			// which mirrors the target loops' admission above: a target that
			// died or left mid-fold is neither purged nor narrated, and the
			// loop's own "finds no targets" line is all the caster reads. A
			// failed check must fall through to NOTHING, not to the self-cast
			// arm below.
			switch {
			case len(cs.TargetUserIds) > 0:
				if targetUser := users.GetByUserId(cs.TargetUserIds[0]); targetUser != nil {
					t := purgeTarget{char: targetUser.Character, user: targetUser, name: targetUser.Character.Name}
					if t.stillPresent(room, false) {
						resolvePurgeAffliction(user, room, t)
					}
				}
			case len(cs.TargetMobInstanceIds) > 0:
				if tMob := mobs.GetInstance(cs.TargetMobInstanceIds[0]); tMob != nil {
					t := purgeTarget{char: &tMob.Character, name: tMob.Character.Name, display: mobDisplayName(tMob, room, user.UserId)}
					if t.stillPresent(room, true) {
						resolvePurgeAffliction(user, room, t)
					}
				}
			default:
				resolvePurgeAffliction(user, room, purgeTarget{char: user.Character, user: user, name: user.Character.Name}) // self-cast
			}
			// Uncontested utility cast: no defence to beat.
			return true
		}
	}

	// --- Consume component if required ---
	if spellData.ComponentTag != "" {
		consumeSpellComponent(user, spellData.ComponentTag)
	}

	return anyLanded
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
//
// room is the cast's room (lighting plan 5b): the caster must see to aim, so
// Mult carries the sight row of the situational table. Nil is unity.
func spellAttackSideFor(spellData *spells.SpellData, casterChar *characters.Character, room messaging.RoomVisibility) combat.AttackSide {
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
		// Task 17: composed with the shared situational layer. Prone and
		// stamina are 1.0 on both spell channels by the declared table: you
		// cast fine from the ground, and the conviction-depletion penalty is
		// already applied in the damage term (calcSpellDamageForCharacter),
		// so it must not reach accuracy a second time here. The sight row
		// (lighting plan 5b) does apply. ForceCrit is per-target and set by
		// each resolveAgainst* call site.
		Mult: combat.SituationalAttackMult(casterChar, room, spellData.Attack()),
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
// landed reports that this target's contest was WON outright -- the caster's
// roll beat the defence. It is the spell channel's equivalent of melee's
// CleanHit, and U10b-1 Task 13 uses it to decide whether the cast's ONE
// progression award pays full weight or the failure fraction.
//
// A DEFENDED cast is not landed even though it still deals partial damage on
// the shared mitigation curve, matching SkillMoveResult.Hit's contract. A
// fumble is not landed either: it aborts before success.
func resolveAgainstMob(user *users.UserRecord, mob *mobs.Mob, room *rooms.Room, spellData *spells.SpellData, side combat.AttackSide, magnitude int) (fumbled bool, landed bool) {
	caster := actions.NewUserActorInRoom(user, room)
	target := actions.NewMobActorInRoom(mob, room)

	// A help spell (a heal on your companion, an area mend over allies) is
	// uncontested, as in every resolver (resolveHelpSpell). On master
	// 612b85d54 it ran a quell contest here, so a companion could "defend"
	// its own heal, a fumble backfired on the caster, and a defensive crit
	// earned the companion a counter-swing at its owner.
	if spellData.AttackType == combatvocab.AttackNone {
		return false, resolveHelpSpell(newSpellEffectCtx(user.Character, caster, target, room, spellData,
			magnitude, uncontestedSpellResult()))
	}

	// Task 17: the sleeping-victim forced crit reaches the spell channel.
	side.ForceCrit = combat.SleepingForceCrit(&mob.Character)

	// Charm alone carries an in-combat penalty (spec 4.1). A mind braced for
	// violence is harder to reach, and one braced against YOU is hardest.
	//
	// Normalise Mult FIRST. Zero is the zero value and AttackSide.score() reads
	// it as "unset, 1.0" (defence_multiplier.go:78-83), so multiplying into an
	// unset Mult yields 0, which reads back as 1.0 -- the penalty would vanish
	// silently while producing an entirely plausible number.
	if spellData.EffectType == "charm" {
		if side.Mult == 0 {
			side.Mult = 1.0
		}
		side.Mult *= charmInCombatMult(&mob.Character, user.UserId)
	}
	out := runSpellChannelAttack(combat.SightRoom(room), spellData.Attack(), side, user.Character, &mob.Character)
	c := newSpellEffectCtx(user.Character, caster, target, room, spellData, magnitude, out)

	// Backfire on fumble, resolved BEFORE success per the seam's contract: a
	// fumbled cast aborts even a winning roll.
	if out.AttackerFumble {
		applySpellBackfire(c)
		return true, false
	}

	// Boss-interrupt, for every pairing (interruptSpellTarget).
	interruptSpellTarget(c)

	recordSpellResolution(c, applySpellEffect(c))

	// U6b Task 10: the MOB defender's crit defence counters the player caster.
	fireSpellCounterTier(room, out, spellData.Attack(),
		&mob.Character, user.Character, nil, user)

	return false, !out.Defended
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
	// Through the seam: a participant who cannot see the other party reads
	// "something" in place of their name. SendTextVisualToUser used to drop
	// these lines entirely, so a defender in the dark was never told they had
	// defended at all.
	messaging.SendTrio(messaging.Trio{
		Actor:    messaging.Say(category, string(triad.ToAttacker)),
		Actee:    messaging.Say(category, string(triad.ToDefender)),
		Observer: messaging.Say(category, string(triad.ToRoom)),
	}, spellAudience(attackerUser, attackerName, defenderUser, defenderName, room))
}

// spellDefenceIdentity returns the display-ready identity for either kind of
// spell participant. Mob identities retain the room's duplicate index.
func spellDefenceIdentity(char *characters.Character, user *users.UserRecord, room *rooms.Room) string {
	if char == nil {
		return ""
	}
	if user != nil {
		// A narration name (#453): no adjective span and no " and <pet>",
		// which RenderChannelDefenceMessages' StripNameAdjectives does not
		// remove ("Aliceia and Fang dodges").
		f := char.GetPlayerName(user.UserId)
		f.Adjectives = nil
		f.QuestAlert = false
		f.PetName = ``
		return f.String()
	}
	if room != nil && char.MobInstanceId > 0 {
		if mob := mobs.GetInstance(char.MobInstanceId); mob != nil {
			return mobDisplayName(mob, room, 0)
		}
	}
	// A mob whose Character carries no instance id still follows the
	// room-wide rule mobDisplayName applies: a hidden mob is unseen (#382).
	if char.IsHidden() {
		return messaging.UnseenFigure(messaging.SightNone)
	}
	return char.GetMobName(0).String()
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
func resolveAgainstPlayer(user *users.UserRecord, target *users.UserRecord, room *rooms.Room, spellData *spells.SpellData, side combat.AttackSide, magnitude int) (fumbled bool, landed bool) {
	if spellData.AttackType == combatvocab.AttackNone {
		return false, resolveHelpSpell(newSpellEffectCtx(user.Character, actions.NewUserActorInRoom(user, room),
			actions.NewUserActorInRoom(target, room), room, spellData, magnitude, uncontestedSpellResult()))
	}

	// Task 17: the sleeping-victim forced crit reaches the spell channel.
	side.ForceCrit = combat.SleepingForceCrit(target.Character)
	out := runSpellChannelAttack(combat.SightRoom(room), spellData.Attack(), side, user.Character, target.Character)
	c := newSpellEffectCtx(user.Character, actions.NewUserActorInRoom(user, room),
		actions.NewUserActorInRoom(target, room), room, spellData, magnitude, out)

	// Backfire on fumble, resolved BEFORE success per the seam's contract.
	if out.AttackerFumble {
		applySpellBackfire(c)
		return true, false
	}

	interruptSpellTarget(c)

	recordSpellResolution(c, applySpellEffect(c))

	// U6b Task 10: the defending player's crit defence counters the caster.
	fireSpellCounterTier(room, out, spellData.Attack(),
		target.Character, user.Character, target, user)

	return false, !out.Defended
}

// spellNarratedByGoHook reports whether resolveSpell's Go hook switch owns a
// spell's narration. KEEP IT IN STEP WITH THAT SWITCH: a spell added there
// without being added here is told twice, once by applySpellDefaultEffect
// and once by its hook.
func spellNarratedByGoHook(spellId string) bool {
	switch spellId {
	case "fold-anchor", "fold-recall", "purge-affliction":
		return true
	}
	return false
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
//   - HelpArea fills through the same spellHelpAreaTargets as resolveSpell.
//   - Mob targets include a self-cast branch (MS) for help spells, through
//     resolveHelpSpell; a player's self-cast arrives as a player target.
//   - No onMagic script, no component consumption.
//   - Per-target helpers are entirely separate from the player equivalents.
//
// fizzled reports a fold that completed on a room its every target had left:
// nothing resolved, so the caller pays no progression award and rolls no
// spell discovery, as for the fold step's TargetGone fizzle.
func resolveMobSpell(mob *mobs.Mob, cs activity.CastingData, spellData *spells.SpellData, room *rooms.Room) (anyLanded, fizzled bool) {
	// Go spell hooks — dispatch position-mutating / non-target spells before
	// the type-based effect routing below. Mirrors the player path in
	// resolveSpell. Stage 3.0d.
	switch cs.SpellId {
	case "fold-anchor":
		resolveFoldAnchor(actions.NewMobActorInRoom(mob, room))
		return true, false // uncontested utility cast: no defence to beat
	case "fold-recall":
		actor := actions.NewMobActorInRoom(mob, room)
		if !validateFoldRecall(actor) {
			// The recall could not be validated. Nothing resolved, so the
			// cast did not land.
			return false, false
		}
		resolveFoldRecall(actor)
		return true, false
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
		return true, false // uncontested area drain
	}

	side := spellAttackSideFor(spellData, &mob.Character, combat.SightRoom(room))
	magnitude := spellData.EffectMagnitude

	if spellData.IsHarm() && spellData.Targeting == combatvocab.TargetArea {
		cs.TargetMobInstanceIds, cs.TargetUserIds = mobAreaHarmTargets(mob, room)
	}
	if !spellData.IsHarm() && spellData.Targeting == combatvocab.TargetArea {
		cs.TargetUserIds, cs.TargetMobInstanceIds = spellHelpAreaTargets(actions.NewMobActorInRoom(mob, room), room)
	}

	// found counts the targets still here to resolve against. With none, the
	// fold completed on a room its every target had left (#242): it fizzles
	// aloud, as the fold step's TargetGone does, instead of resolving nothing
	// and saying nothing.
	found := 0
	for _, mobInstId := range cs.TargetMobInstanceIds {
		if mobInstId == mob.InstanceId {
			// MS: the caster is its own target. Only a help spell puts a mob
			// in its own list (a HelpSingle with no target, or its own place
			// in an area help), and it takes the same uncontested step and
			// appliers as every other pairing. A mob never harms itself.
			found++
			if !spellData.IsHarm() {
				self := actions.NewMobActorInRoom(mob, room)
				anyLanded = resolveHelpSpell(newSpellEffectCtx(&mob.Character, self, self, room, spellData,
					magnitude, uncontestedSpellResult())) || anyLanded
			}
			continue
		}
		if target := mobs.GetInstance(mobInstId); target != nil && target.Character.Health > 0 && target.Character.RoomId == room.RoomId {
			found++
			anyLanded = resolveMobSpellAgainstMob(mob, target, room, spellData, side, magnitude) || anyLanded
		}
	}
	for _, userId := range cs.TargetUserIds {
		if target := users.GetByUserId(userId); target != nil && target.Character.RoomId == room.RoomId {
			found++
			anyLanded = resolveMobSpellAgainstPlayer(mob, target, room, spellData, side, magnitude) || anyLanded
		}
	}
	if found == 0 {
		fizzleMobFold(mob, room, cs)
		return false, true
	}

	return anyLanded, false
}

// resolveMobDrainArea is the resolution handler for a mob-cast spell whose
// EffectType is "drain_area" (the Core Guardian's "core recharge" ability
// design — see docs/superpowers/plans/completed/2026-07-06-crashsite-boss-mechanics.md
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
		sendVisualElseAudible(room, messaging.CategorySpellDisruption, fmt.Sprintf(
			`%s's <ansi fg="cyan">%s</ansi> crackles through the air, finding no one to drain.`,
			mobSubjectName(mob, room), spellData.Name),
			messaging.SoundSpellSputtersOut)
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
		c := newSpellEffectCtx(&mob.Character, actions.NewMobActorInRoom(mob, room),
			actions.NewUserActorInRoom(target, room), room, spellData, 0, pr.MoveResult.Defence)
		if !pr.MoveResult.Hit && pr.MoveResult.Damage == 0 {
			// Defended with zero damage (a defensive crit). This used to be a
			// silent miss; U6b Task 9 speaks the defence triad so the player
			// who fully stopped the pull learns what saved them. The area
			// drain reveals no one, so a player still hidden is unseen in
			// the room line, as a hidden mob is (roomName).
			sendSpellChannelDefenceMessages(room, spellSchoolCategory(spellData), pr.MoveResult.Defence,
				spellDefenceIdentity(&mob.Character, nil, room),
				roomName(target.Character.IsHidden(), spellDefenceIdentity(target.Character, target, room)),
				spellData.Name, nil, target)
			continue
		}
		if pr.MoveResult.Hit {
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.NoLine,
				Actee: messaging.Say(c.category(), fmt.Sprintf(
					`%s's <ansi fg="cyan">%s</ansi> saps your strength! (<ansi fg="damage">%s</ansi>)`,
					c.casterName(), spellData.Name,
					combat.GetDamageDescription(pr.MoveResult.Damage, target.Character.HealthMax.Value))),
				Observer: messaging.NoLine,
			}, c.audience())
			if !target.Character.IsInCombat() {
				targeting.Commit(target.Character, state.ActorRef{MobInstanceId: mob.InstanceId}, targeting.ReasonAttack)
			}
		} else {
			// Defended, but the drain still landed a partial pull. Since
			// Task 13 a defended maneuver can deal partial damage; say so
			// instead of letting the player's HP drop with no message at all.
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.NoLine,
				Actee: messaging.Say(c.category(), fmt.Sprintf(
					`%s's <ansi fg="cyan">%s</ansi> fails to take full hold of you, but still saps a little of your strength! (<ansi fg="damage">%s</ansi>)`,
					c.casterName(), spellData.Name,
					combat.GetDamageDescription(pr.MoveResult.Damage, target.Character.HealthMax.Value))),
				Observer: messaging.NoLine,
			}, c.audience())
		}
	}

	// The landing is the mob's own act: silent to a reader who does not
	// perceive a hidden mob (owner R3, #458). The no-one-to-drain line above
	// is a fizzle, a disruption, and keeps mobSubjectName (owner R4).
	sendVisualRoomText(room, spellSchoolCategory(spellData), fmt.Sprintf(
		`%s's <ansi fg="cyan">%s</ansi> tears the life from everyone in the room!`,
		mobSeenName(mob, room, 0), spellData.Name), mobActUnseenBy(room, mob)...)
}

// landed carries the same meaning as on the player path: the contest was WON
// outright. See resolveAgainstMob.
func resolveMobSpellAgainstMob(caster *mobs.Mob, target *mobs.Mob, room *rooms.Room,
	spellData *spells.SpellData, side combat.AttackSide, magnitude int) (landed bool) {
	// A help spell (a heal, or a condition buff cast on an ally mob) is a
	// cooperative cast, not an attack: uncontested, as in every resolver
	// (resolveHelpSpell). Crash-site boss-mechanics Chunk B: the Repair Frame
	// add heals Warden-Prime and the Core Guardian this way.
	casterActor := actions.NewMobActorInRoom(caster, room)
	targetActor := actions.NewMobActorInRoom(target, room)
	if spellData.AttackType == combatvocab.AttackNone {
		return resolveHelpSpell(newSpellEffectCtx(&caster.Character, casterActor, targetActor, room, spellData,
			magnitude, uncontestedSpellResult()))
	}
	// Task 17: the sleeping-victim forced crit reaches the spell channel.
	side.ForceCrit = combat.SleepingForceCrit(&target.Character)
	out := runSpellChannelAttack(combat.SightRoom(room), spellData.Attack(), side, &caster.Character, &target.Character)
	c := newSpellEffectCtx(&caster.Character, casterActor, targetActor, room, spellData, magnitude, out)
	if out.AttackerFumble {
		applySpellBackfire(c)
		return false
	}
	interruptSpellTarget(c)
	recordSpellResolution(c, applySpellEffect(c))

	// U6b Task 10: the defending mob's crit defence counters the mob caster.
	fireSpellCounterTier(room, out, spellData.Attack(),
		&target.Character, &caster.Character, nil, nil)

	return !out.Defended
}

// resolveMobSpellAgainstPlayer runs the ONE channel contest for a mob-cast
// spell at a player and applies the effect through applySpellEffect. Crit-received toughening
// for the defender fires inside the seam's bonus tier — the U9-era direct
// block this function used to carry became a duplicate and was deleted with
// the collapse (U6b Task 4).
// landed carries the same meaning as on the player path: the contest was WON
// outright. See resolveAgainstMob.
func resolveMobSpellAgainstPlayer(caster *mobs.Mob, target *users.UserRecord, room *rooms.Room,
	spellData *spells.SpellData, side combat.AttackSide, magnitude int) (landed bool) {
	// A mob's help spell on a player is uncontested, as on every other
	// pairing (audit row 3): it used to be contested, then fall to the
	// default arm and apply nothing.
	if spellData.AttackType == combatvocab.AttackNone {
		return resolveHelpSpell(newSpellEffectCtx(&caster.Character, actions.NewMobActorInRoom(caster, room),
			actions.NewUserActorInRoom(target, room), room, spellData, magnitude, uncontestedSpellResult()))
	}
	// Task 17: the sleeping-victim forced crit reaches the spell channel.
	side.ForceCrit = combat.SleepingForceCrit(target.Character)
	out := runSpellChannelAttack(combat.SightRoom(room), spellData.Attack(), side, &caster.Character, target.Character)
	c := newSpellEffectCtx(&caster.Character, actions.NewMobActorInRoom(caster, room),
		actions.NewUserActorInRoom(target, room), room, spellData, magnitude, out)
	if out.AttackerFumble {
		applySpellBackfire(c)
		return false
	}
	interruptSpellTarget(c)
	recordSpellResolution(c, applySpellEffect(c))

	// U6b Task 10: the PLAYER defender's crit defence counters the mob caster.
	fireSpellCounterTier(room, out, spellData.Attack(),
		target.Character, &caster.Character, target, nil)

	return !out.Defended
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

// charmInCombatMult is charm's attack-side penalty for reaching into a mind that
// is already fighting.
//
// Restored 2026-08-24. Spec 4.1's mechanics table lists both multipliers as
// UNCHANGED across the U10c rewrite; slice B deleted them along with
// resolveCharmSpell and put nothing in their place. Nothing else covered for it:
// combat.SituationalAttackMult returns a flat 1.0 for every channel except melee
// and ranged, and the defy defence carries no combat term -- so charm quietly got
// easier mid-fight while charm.yaml, charm.template and a gameplay tip all went on
// telling players it had got harder.
//
// Literals rather than balance knobs by owner ruling 2026-08-24.
func charmInCombatMult(target *characters.Character, casterUserId int) float64 {
	if target == nil || !target.IsInCombat() {
		return 1.0
	}
	if target.CurrentCombatTarget().UserId == casterUserId {
		return 0.75 // fighting the caster -- steepest
	}
	return 0.85 // fighting someone else -- moderate
}
