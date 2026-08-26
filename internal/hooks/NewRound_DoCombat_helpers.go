package hooks

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// processDefenderProgression fires skill and stat progression for a defender
// based on which defense types were used across all swings in the round.
//
// It awards ONCE PER DEFENCE TYPE per round, not once per swing: a defender who
// dodges four times gets one dodge award. That de-duplication is the only thing
// this function does that combat.AwardDefenceProgression does not, and it is why
// the per-type rows themselves live over there rather than in a switch here.
//
// U6 Task 12 moved those rows. Before it, this switch covered dodge / parry /
// block with no default arm while avoidance.go awarded the two non-physical
// rows itself. Deleting avoidance.go without a single shared mapping would have
// silently deleted defender progression on both non-physical channels.
//
// Quell and defy cannot reach this function today -- SwingEvent.DefenseUsed is
// populated only by the melee path, which never emits either. They are covered
// by AwardDefenceProgression regardless, so wiring either into melee later is a
// row in DefenceSetFor and nothing else.
func processDefenderProgression(c *characters.Character, userId int, result combat.AttackResult) {
	for _, d := range defenceTypesUsed(result) {
		combat.AwardDefenceProgression(c, userId, string(d), true)
	}
}

// defenceTypesUsed returns the set of defences that registered this round, in
// the same fixed order processDefenderProgression uses. Extracted so the seam
// and the ordinary award read one definition of "which defences happened".
func defenceTypesUsed(result combat.AttackResult) []combat.DefenseType {
	used := make(map[combat.DefenseType]bool, 3)
	for _, se := range result.SwingEvents {
		if se.DefenseUsed != combat.DefenseNone {
			used[se.DefenseUsed] = true
		}
	}
	out := make([]combat.DefenseType, 0, 3)
	for _, d := range []combat.DefenseType{combat.DefenseDodge, combat.DefenseParry, combat.DefenseBlock} {
		if used[d] {
			out = append(out, d)
		}
	}
	return out
}

// defenceSkillFor names the skill and stat the defender's OBSERVED event trains
// when a crit or fumble happens. It uses the first defence that registered this
// round; with no defence registered it returns empty, which suppresses the
// event rather than guessing.
//
// It delegates to combat.DefenceSkillAndStat rather than switching again here.
// A second copy of the five-defence mapping is exactly the drift this arc
// exists to remove, and it would go stale the first time a defence changed what
// it trains.
func defenceSkillFor(used []combat.DefenseType) string {
	if len(used) == 0 {
		return ""
	}
	skill, _ := combat.DefenceSkillAndStat(string(used[0]))
	return skill
}

// defenceStatFor is defenceSkillFor's stat counterpart, from the same mapping.
func defenceStatFor(used []combat.DefenseType) string {
	if len(used) == 0 {
		return ""
	}
	_, stat := combat.DefenceSkillAndStat(string(used[0]))
	return stat
}

// attackerBonusSkillAndStat names the skill the attacker's crit or FUMBLE
// bonus trains.
//
// It deliberately does NOT gate on CleanHit. A fumbled swing has CleanHit
// false, so deriving the bonus skill from a CleanHit-gated field would leave it
// empty and applyBonusProgression would skip the roll -- silently deleting
// attacker fumble progression, which pre-U9 fired via OnCriticalFailure with
// the real skill tag and which spec 7.1 lists as an INCREASE.
//
// Falls back through: the first weapon's tag, then the character's current
// combat skill (correct for the unarmed case, which has no WeaponHits at all).
func attackerBonusSkillAndStat(res combat.AttackResult, atkChar *characters.Character) (skill, stat string) {
	if len(res.WeaponHits) > 0 {
		skill = res.WeaponHits[0].SkillTag
	}
	if skill == "" && atkChar != nil {
		skill = string(atkChar.GetCombatSkillTag())
	}
	if skill == "" {
		return "", ""
	}
	return skill, skills.GetSkillPrimaryStat(skill)
}

// mobDisplayName returns the formatted display name for a mob in combat text,
// including duplicate index coloring when multiple mobs share the same name.
func mobDisplayName(mob *mobs.Mob, room *rooms.Room, viewingUserId int) string {
	dupIdx := room.GetMobDuplicateIndex(mob.InstanceId)
	return mob.Character.GetMobNameIndexed(viewingUserId, dupIdx).String()
}

// sendVisualRoomText sends a visual message that requires sight.
// Delegates to Room.SendTextVisual which handles darkness filtering.
//
// The cat parameter classifies the message so the central pipeline
// can apply category-appropriate color + normalization. Callers used
// to omit this; T11 added it as a required arg.
func sendVisualRoomText(room *rooms.Room, cat messaging.Category, visualMsg string, excludeUserIds ...int) {
	if room == nil {
		return
	}
	room.SendTextVisual(cat, visualMsg, excludeUserIds...)
}

// isExcludedUser checks if a userId is in the exclusion list.
func isExcludedUser(uid int, excludeIds []int) bool {
	for _, id := range excludeIds {
		if uid == id {
			return true
		}
	}
	return false
}

// sendDarkRoomCombatFallback sends a one-time "sounds of fighting" message
// to non-nightvision players in dark rooms.
func sendDarkRoomCombatFallback(room *rooms.Room, excludeUserIds ...int) {
	if room == nil || room.GetVisibility() >= 1 {
		return
	}
	for _, uid := range room.GetPlayers() {
		if isExcludedUser(uid, excludeUserIds) {
			continue
		}
		u := users.GetByUserId(uid)
		if u != nil && !u.Character.HasFlagFromAnySource(buffs.NightVision) {
			u.SendText(messaging.CategoryDefault, `<ansi fg="yellow">You hear the sounds of fighting nearby.</ansi>`)
		}
	}
}

// replaceDarknessMessages replaces detailed combat messages with generic
// darkness text for combatants who cannot see.
func replaceDarknessMessages(result *combat.AttackResult, sourceCanSee bool, targetCanSee bool) {
	if sourceCanSee && targetCanSee {
		return
	}

	// Build replacement messages based on swing events. Dark-room
	// substitutes carry the same per-line outcome category as the
	// equivalent sighted line — zero-damage defense crit / dodge →
	// CategoryDodge, hits → CategoryHitMelee, miss/fumble →
	// CategoryHitMelee. A DEFLECTED swing (defence won, partial damage
	// through — U6 Task 14) is a hit-band line for both viewers, so the
	// damage-to-viewer verbosity floor still applies to the defender.
	if !sourceCanSee {
		newMsgs := make([]combat.TaggedMessage, 0, len(result.SwingEvents))
		for _, se := range result.SwingEvents {
			if se.DoubleFumble {
				continue
			}
			switch {
			case se.Fumble:
				newMsgs = append(newMsgs, combat.TaggedMessage{Category: messaging.CategoryHitMelee, Text: `<ansi fg="fumble-text">!!!</ansi> <ansi fg="yellow">You stumble badly in the darkness!</ansi> <ansi fg="fumble-text">!!!</ansi>`})
			case se.Crit:
				newMsgs = append(newMsgs, combat.TaggedMessage{Category: messaging.CategoryHitMelee, Text: `<ansi fg="crit-text">***</ansi> <ansi fg="attack-good">You land a devastating blow in the dark!</ansi> <ansi fg="crit-text">***</ansi>`})
			case se.DefenseUsed != "" && !se.DefenseCrit && se.Damage > 0:
				// Deflected: the "turned aside" line below implies zero
				// damage, which is no longer true for a partial deflection.
				newMsgs = append(newMsgs, combat.TaggedMessage{Category: messaging.CategoryHitMelee, Text: `<ansi fg="attack-bad">Something turns your blow aside, but you feel it land!</ansi>`})
			case se.DefenseCrit || se.DefenseUsed != "":
				newMsgs = append(newMsgs, combat.TaggedMessage{Category: messaging.CategoryDodge, Text: `<ansi fg="attack-bad">Your attack is turned aside by something!</ansi>`})
			case se.Hit:
				newMsgs = append(newMsgs, combat.TaggedMessage{Category: messaging.CategoryHitMelee, Text: `<ansi fg="attack-good">You strike blindly and connect!</ansi>`})
			default:
				newMsgs = append(newMsgs, combat.TaggedMessage{Category: messaging.CategoryHitMelee, Text: `<ansi fg="yellow">You swing wildly in the darkness!</ansi>`})
			}
		}
		if len(newMsgs) > 0 {
			result.MessagesToSource = newMsgs
		}
	}

	if !targetCanSee {
		newMsgs := make([]combat.TaggedMessage, 0, len(result.SwingEvents))
		for _, se := range result.SwingEvents {
			if se.DoubleFumble {
				continue
			}
			switch {
			case se.Fumble:
				newMsgs = append(newMsgs, combat.TaggedMessage{Category: messaging.CategoryHitMelee, Text: `<ansi fg="yellow">You hear your attacker stumble!</ansi>`})
			case se.Crit:
				newMsgs = append(newMsgs, combat.TaggedMessage{Category: messaging.CategoryHitMelee, Text: `<ansi fg="crit-text">***</ansi> <ansi fg="red">Something hits you hard in the dark!</ansi> <ansi fg="crit-text">***</ansi>`})
			case se.DefenseUsed != "" && !se.DefenseCrit && se.Damage > 0:
				// Deflected: real damage reached the viewer, so this is a
				// hit-band line (the verbosity floor must not suppress it),
				// not a CategoryDodge line like the clean fend-off below.
				newMsgs = append(newMsgs, combat.TaggedMessage{Category: messaging.CategoryHitMelee, Text: `<ansi fg="defense-good">You fend off something in the dark, but it still catches you!</ansi>`})
			case se.DefenseCrit || se.DefenseUsed != "":
				newMsgs = append(newMsgs, combat.TaggedMessage{Category: messaging.CategoryDodge, Text: `<ansi fg="defense-good">You fend off something in the dark!</ansi>`})
			case se.Hit:
				newMsgs = append(newMsgs, combat.TaggedMessage{Category: messaging.CategoryHitMelee, Text: `<ansi fg="red">Something strikes you in the dark!</ansi>`})
			default:
				newMsgs = append(newMsgs, combat.TaggedMessage{Category: messaging.CategoryHitMelee, Text: `<ansi fg="yellow">You hear something whoosh past!</ansi>`})
			}
		}
		if len(newMsgs) > 0 {
			result.MessagesToTarget = newMsgs
		}
	}
}

// handlePlayerShieldDecay processes Minor Shield round expiry for a player.
func handlePlayerShieldDecay(user *users.UserRecord) {
	if user.Character.HasCondition(characters.ConditionShield) {
		if user.Character.GetConditionDuration(characters.ConditionShield) <= 1 {
			user.Character.RemoveCondition(characters.ConditionShield)
			user.SendText(messaging.CategoryBuffExpire, `<ansi fg="cyan">Your Minor Shield dissipates.</ansi>`)
		} else {
			user.Character.DecrementCondition(characters.ConditionShield)
		}
	}
}

// castingTargetChar returns the first target character from a CastingData, or nil.
func castingTargetChar(cs activity.CastingData) *characters.Character {
	for _, mobInstId := range cs.TargetMobInstanceIds {
		if m := mobs.GetInstance(mobInstId); m != nil {
			return &m.Character
		}
	}
	for _, uid := range cs.TargetUserIds {
		if u := users.GetByUserId(uid); u != nil {
			return u.Character
		}
	}
	return nil
}

// recordConcentrationFailure records a fizzle event for a broken spell.
func recordConcentrationFailure(src, tgt combat.SourceTarget, srcChar *characters.Character, tgtChar *characters.Character) {
	combat.RecordSpell(src, tgt, false, false, false, true, 0, 0, srcChar, tgtChar, util.GetRoundCount())
}

// handlePlayerFoldCasting processes fold spell casting for a player.
// Returns true if the player is casting and should skip combat.
func handlePlayerFoldCasting(user *users.UserRecord, userId int) bool {
	if user.Character.Activity == nil || !user.Character.Activity.IsCasting() {
		return false
	}

	// Capture state before processFoldRound clears it on terminal conditions.
	csBeforeProcess, _ := user.Character.Activity.CastingData()

	// Bleeding out = automatic concentration break (player-only check).
	if user.Character.IsDisabled() {
		recordConcentrationFailure(combat.User, combat.Mob, user.Character, castingTargetChar(csBeforeProcess))
		clearCastingActivity(user.Character, activity.TriggerConcentrationBreak)
		events.AddToQueue(events.CastInterrupted{UserId: user.UserId, SpellId: csBeforeProcess.SpellId})
		return true
	}

	result := processFoldRound(user.Character)

	// Emit CastInterrupted for outside-force position breaks (prone/grapple from
	// combat) so the web-client action queue can re-arm the cast. CastComplete
	// and StillCasting are not interruptions; TargetGone / InsufficientConviction
	// are not re-armable external forces.
	if result.ProneBroke || result.GrappleBroke {
		events.AddToQueue(events.CastInterrupted{UserId: user.UserId, SpellId: csBeforeProcess.SpellId})
	}

	switch {
	case result.ProneBroke:
		recordConcentrationFailure(combat.User, combat.Mob, user.Character, castingTargetChar(csBeforeProcess))
		user.SendText(messaging.CategorySpellDisruption, `<ansi fg="red">You lose your concentration as you hit the ground!</ansi>`)
		room := rooms.LoadRoom(user.Character.RoomId)
		if room != nil {
			sendVisualRoomText(room, messaging.CategorySpellDisruption, fmt.Sprintf(
				`<ansi fg="username">%s</ansi>'s concentration breaks.`, user.Character.Name), user.UserId)
		}

	case result.GrappleBroke:
		// Chunk 4e T4: grapple breaks concentration same as Prone (spec §4.2).
		recordConcentrationFailure(combat.User, combat.Mob, user.Character, castingTargetChar(csBeforeProcess))
		user.SendText(messaging.CategorySpellDisruption, `<ansi fg="red">Your concentration shatters — you cannot hold the fold while grappled!</ansi>`)
		room := rooms.LoadRoom(user.Character.RoomId)
		if room != nil {
			sendVisualRoomText(room, messaging.CategorySpellDisruption, fmt.Sprintf(
				`<ansi fg="username">%s</ansi>'s concentration breaks.`, user.Character.Name), user.UserId)
		}

	case result.TargetGone:
		recordConcentrationFailure(combat.User, combat.Mob, user.Character, castingTargetChar(csBeforeProcess))
		user.SendText(messaging.CategorySpellDisruption, `<ansi fg="red">Your spell fizzles — the target is gone.</ansi>`)

	case result.SpellDataMissing:
		user.SendText(messaging.CategorySpellDisruption, `<ansi fg="red">The spell dissipates — its data cannot be found.</ansi>`)

	case result.InsufficientConviction:
		recordConcentrationFailure(combat.User, combat.Mob, user.Character, castingTargetChar(csBeforeProcess))
		user.SendText(messaging.CategorySpellDisruption, `<ansi fg="red">Your conviction wavers — the fold collapses.</ansi>`)

	case result.CastComplete:
		cs := result.CastingData
		spellData := result.SpellData
		// Send YAML wait text (if defined).
		if spellData != nil && (spellData.WaitUserText != "" || spellData.WaitRoomText != "") {
			tCtx := textutil.TokenContext{
				SourceName:      user.Character.GetCharacterName(true),
				SourcePlainName: user.Character.GetCharacterName(false),
			}
			cfg := textutil.SendTextConfig{
				UserSendFunc: func(msg string) { user.SendText(messaging.CategorySpellFold, msg) },
				RoomSendFunc: func(msg string, skip ...int) {
					if r := rooms.LoadRoom(user.Character.RoomId); r != nil {
						r.SendText(messaging.CategorySpellFold, msg, skip...)
					}
				},
				ExcludeId: user.UserId,
			}
			textutil.SendPhaseText(spellData.WaitUserText, spellData.WaitRoomText, tCtx, "pink", cfg)
		}

		resolveRoom := rooms.LoadRoom(user.Character.RoomId)
		if resolveRoom != nil {
			resolveSpell(user, cs, spellData, resolveRoom)
		}
		user.Character.TrackSpellCast(cs.SpellId)
		// Fire progression for the correct skill based on spell school.
		// Difficulty scaling: harder spells give proportionally more progression.
		spellBonus := 1.0
		if spellData != nil {
			bal := configs.GetBalanceConfig()
			spellBonus = 1.0 + float64(spellData.Difficulty)*float64(bal.SpellDifficultyProgressionScale)

			// Self-cast penalty: HelpSingle targeting only self gets reduced progression
			if spellData.Type == spells.HelpSingle &&
				len(cs.TargetMobInstanceIds) == 0 &&
				len(cs.TargetUserIds) == 1 && cs.TargetUserIds[0] == userId {
				spellBonus *= float64(bal.SelfCastProgressionMultiplier)
			}

			// AoE guard: HarmArea/HarmMulti with no targets hit skips progression
			if (spellData.Type == spells.HarmArea || spellData.Type == spells.HarmMulti) &&
				len(cs.TargetUserIds) == 0 && len(cs.TargetMobInstanceIds) == 0 {
				spellBonus = 0
			}
		}

		if spellBonus > 0 {
			// OnSkillUseScaled already rolls the skill's primary stat --
			// manifestation maps to charisma, spellcasting to willpower -- so an
			// explicit OnStatUse beside it double-rolled every cast. The stat a
			// spell trains now comes from its primarystat (Task 13).
			castSkill := skills.Spellcasting
			if spellData != nil && spellData.HasSchool(spells.SchoolManifestation) {
				castSkill = skills.Manifestation
			}
			user.Character.OnSkillUseScaled(string(castSkill), userId, spellBonus, false)

			// primarystat overrides the skill's default stat. Manifestation
			// already maps to charisma and spellcasting to willpower, so for
			// every shipped file this is a no-op -- it exists so a spell that
			// declares something else actually trains it.
			if spellData != nil {
				if st := spellData.PrimaryStat; st != "" && st != skills.GetSkillPrimaryStat(string(castSkill)) {
					user.Character.OnStatUse(st, userId)
				}
			}
		}

		// Phase 25.1: Spell discovery — traditional schools.
		castSkillLevel := user.Character.GetSkillLevel(skills.Spellcasting)
		knownCount := len(user.Character.SpellBook)
		bal := configs.GetBalanceConfig()
		perception := user.Character.Stats.Perception.ValueAdj
		traditionalChance := configs.DiscoveryChance(configs.DiscoveryParams{
			Base:       float64(bal.SpellDiscoveryBaseChance),
			Decay:      float64(bal.SpellDiscoveryDecayRate),
			Known:      knownCount,
			Perception: perception,
			Skill:      castSkillLevel,
		})
		if util.Rand(100) < int(traditionalChance) {
			eligible := spells.GetEligibleSpells(user.Character.SpellBook, castSkillLevel,
				spells.SchoolElemental, spells.SchoolEnhancement, spells.SchoolMental, spells.SchoolVital)
			if len(eligible) > 0 {
				pick := eligible[util.Rand(len(eligible))]
				if user.Character.LearnSpell(pick) {
					if newSpell := spells.GetSpell(pick); newSpell != nil {
						user.SendText(messaging.CategorySkillProgress, fmt.Sprintf(
							`<ansi fg="magenta-bold">A new pattern crystallizes in your mind: <ansi fg="cyan-bold">%s</ansi></ansi>`,
							newSpell.Name))
					}
				}
			}
		}
		// Phase 25.1: Spell discovery — manifestation school.
		// Only runs if the player has any manifestation skill.
		manifestSkillLevel := user.Character.GetSkillLevel(skills.Manifestation)
		if manifestSkillLevel > 0 {
			manifestChance := configs.DiscoveryChance(configs.DiscoveryParams{
				Base:       float64(bal.SpellDiscoveryBaseChance),
				Decay:      float64(bal.SpellDiscoveryDecayRate),
				Known:      knownCount,
				Perception: perception,
				Skill:      manifestSkillLevel,
			})
			if util.Rand(100) < int(manifestChance) {
				eligible := spells.GetEligibleSpells(user.Character.SpellBook, manifestSkillLevel,
					spells.SchoolManifestation)
				if len(eligible) > 0 {
					pick := eligible[util.Rand(len(eligible))]
					if user.Character.LearnSpell(pick) {
						if newSpell := spells.GetSpell(pick); newSpell != nil {
							user.SendText(messaging.CategorySkillProgress, fmt.Sprintf(
								`<ansi fg="magenta-bold">A manifestation reveals itself: <ansi fg="cyan-bold">%s</ansi></ansi>`,
								newSpell.Name))
						}
					}
				}
			}
		}

	case result.StillCasting:
		cs := result.CastingData
		// Send YAML wait text (if defined).
		waitSpellInfo := spells.GetSpell(cs.SpellId)
		if waitSpellInfo != nil && (waitSpellInfo.WaitUserText != "" || waitSpellInfo.WaitRoomText != "") {
			tCtx := textutil.TokenContext{
				SourceName:      user.Character.GetCharacterName(true),
				SourcePlainName: user.Character.GetCharacterName(false),
			}
			cfg := textutil.SendTextConfig{
				UserSendFunc: func(msg string) { user.SendText(messaging.CategorySpellFold, msg) },
				RoomSendFunc: func(msg string, skip ...int) {
					if r := rooms.LoadRoom(user.Character.RoomId); r != nil {
						r.SendText(messaging.CategorySpellFold, msg, skip...)
					}
				},
				ExcludeId: user.UserId,
			}
			textutil.SendPhaseText(waitSpellInfo.WaitUserText, waitSpellInfo.WaitRoomText, tCtx, "pink", cfg)
		}
		// "cast_continuing", not "cast_started": this fires once per round while
		// the folds are still being laid down, so the start pool announced the
		// cast as beginning again every round, and duplicated the real start
		// line (sent by the cast command) on the round right after initiation.
		//
		// The name must be the DISPLAY name. Passing cs.SpellId here showed the
		// player the raw identifier, e.g. "the folds of conviction-spike".
		foldName := cs.SpellId
		if waitSpellInfo != nil && waitSpellInfo.Name != "" {
			foldName = waitSpellInfo.Name
		}
		user.SendText(messaging.CategorySpellFold, spells.GetCastMessage("cast_continuing", foldName))
	}

	return true
}

// handleMobFoldCasting processes fold spell casting for a mob.
// Returns true if the mob is casting and should skip combat.
func handleMobFoldCasting(mob *mobs.Mob, mobRoom *rooms.Room) bool {
	if mob.Character.Activity == nil || !mob.Character.Activity.IsCasting() {
		return false
	}

	// Capture state before processFoldRound clears it on terminal conditions.
	csBeforeProcess, _ := mob.Character.Activity.CastingData()

	result := processFoldRound(&mob.Character)

	switch {
	case result.ProneBroke:
		recordConcentrationFailure(combat.Mob, combat.User, &mob.Character, castingTargetChar(csBeforeProcess))
		mobRoom.SendText(messaging.CategorySpellDisruption, fmt.Sprintf(
			`%s's concentration breaks.`, mobDisplayName(mob, mobRoom, 0)))

	case result.GrappleBroke:
		// Chunk 4e T4: grapple breaks concentration same as Prone (spec §4.2).
		recordConcentrationFailure(combat.Mob, combat.User, &mob.Character, castingTargetChar(csBeforeProcess))
		mobRoom.SendText(messaging.CategorySpellDisruption, fmt.Sprintf(
			`%s's concentration breaks.`, mobDisplayName(mob, mobRoom, 0)))

	case result.TargetGone:
		recordConcentrationFailure(combat.Mob, combat.User, &mob.Character, castingTargetChar(csBeforeProcess))
		mobRoom.SendText(messaging.CategorySpellDisruption, fmt.Sprintf(
			`%s's spell fizzles.`, mobDisplayName(mob, mobRoom, 0)))

	case result.SpellDataMissing:
		// Silent failure — no message for missing spell data on mobs.

	case result.InsufficientConviction:
		recordConcentrationFailure(combat.Mob, combat.User, &mob.Character, castingTargetChar(csBeforeProcess))
		mobRoom.SendText(messaging.CategorySpellDisruption, fmt.Sprintf(
			`%s's spell falters.`, mobDisplayName(mob, mobRoom, 0)))

	case result.CastComplete:
		cs := result.CastingData
		spellData := result.SpellData
		if resolveRoom := rooms.LoadRoom(mob.Character.RoomId); resolveRoom != nil {
			resolveMobSpell(mob, cs, spellData, resolveRoom)
		}
		// Stage 38.3: Mob spellcasting progression — difficulty-scaled
		spellBonus := 1.0
		if spellData != nil {
			bal := configs.GetBalanceConfig()
			spellBonus = 1.0 + float64(spellData.Difficulty)*float64(bal.SpellDifficultyProgressionScale)
		}
		// OnSkillUseScaled already rolls the skill's primary stat -- see the
		// identical fix in handlePlayerFoldCasting above for why the explicit
		// OnStatUse calls here double-rolled every mob cast.
		castSkill := skills.Spellcasting
		if spellData != nil && spellData.HasSchool(spells.SchoolManifestation) {
			castSkill = skills.Manifestation
		}
		mob.Character.OnSkillUseScaled(string(castSkill), 0, spellBonus, false)

		// primarystat overrides the skill's default stat -- see the identical
		// override in handlePlayerFoldCasting above.
		if spellData != nil {
			if st := spellData.PrimaryStat; st != "" && st != skills.GetSkillPrimaryStat(string(castSkill)) {
				mob.Character.OnStatUse(st, 0)
			}
		}

		// Task 6: Spell discovery for caster mobs.
		// Only mobs that started with spells or have archetype="casting" can discover new ones.
		isCaster := mob.Archetype == "casting" || len(mob.Character.SpellBook) > 0
		if isCaster {
			castSkillLevel := mob.Character.GetSkillLevel(skills.Spellcasting)
			knownCount := len(mob.Character.SpellBook)
			bal := configs.GetBalanceConfig()
			perception := mob.Character.Stats.Perception.ValueAdj
			traditionalChance := configs.DiscoveryChance(configs.DiscoveryParams{
				Base:       float64(bal.SpellDiscoveryBaseChance),
				Decay:      float64(bal.SpellDiscoveryDecayRate),
				Known:      knownCount,
				Perception: perception,
				Skill:      castSkillLevel,
			})
			// Traditional school discovery.
			if util.Rand(100) < int(traditionalChance) {
				eligible := spells.GetEligibleSpells(mob.Character.SpellBook, castSkillLevel,
					spells.SchoolElemental, spells.SchoolEnhancement, spells.SchoolMental, spells.SchoolVital)
				if len(eligible) > 0 {
					pick := eligible[util.Rand(len(eligible))]
					mob.Character.LearnSpell(pick)
				}
			}
			// Manifestation school discovery — only if mob has manifestation skill.
			manifestSkillLevel := mob.Character.GetSkillLevel(skills.Manifestation)
			if manifestSkillLevel > 0 {
				manifestChance := configs.DiscoveryChance(configs.DiscoveryParams{
					Base:       float64(bal.SpellDiscoveryBaseChance),
					Decay:      float64(bal.SpellDiscoveryDecayRate),
					Known:      knownCount,
					Perception: perception,
					Skill:      manifestSkillLevel,
				})
				if util.Rand(100) < int(manifestChance) {
					eligible := spells.GetEligibleSpells(mob.Character.SpellBook, manifestSkillLevel,
						spells.SchoolManifestation)
					if len(eligible) > 0 {
						pick := eligible[util.Rand(len(eligible))]
						mob.Character.LearnSpell(pick)
					}
				}
			}
		}

	case result.StillCasting:
		mobRoom.SendText(messaging.CategorySpellFold, fmt.Sprintf(
			`%s weaves magic with focused intent.`, mobDisplayName(mob, mobRoom, 0)))
	}

	return true
}

// handlePlayerFlee processes a player's flee attempt.
// Returns true if the player is fleeing and should skip combat.
func handlePlayerFlee(user *users.UserRecord, uRoom *rooms.Room, userId int) bool {
	// Task 15: IsDisengaging() reads CombatPhase.State() == Disengaging,
	// set by TransitionToDisengaging in flee.go. This replaces the legacy
	// Aggro.Type == Flee sentinel check.
	// TODO Task 18: remove legacy Aggro.Type fallback once Aggro is gone.
	phaseFleeing := user.Character.IsDisengaging()
	isFleeing := phaseFleeing
	if !phaseFleeing && user.Character.Aggro != nil && user.Character.Aggro.Type == characters.Flee {
		// Legacy path: Aggro-only set (no CombatPhase wired). Still handled.
		isFleeing = true
	}
	if !isFleeing {
		// A terminal transition can cancel Disengaging before this asynchronous
		// round runs. Atomically retract that orphan; an absent handoff is a
		// harmless no-op for ordinary non-flee combat rounds.
		usercommands.TakeFleeAdmission(user)
		return false
	}
	// Consume admission before any resolution branch. A phase-based flee can
	// only come from the command, so missing admission means another/reentrant
	// resolver already owns it. A true legacy Aggro sentinel has no handoff and
	// retains historical full-skill behavior.
	includeSkill := true
	if phaseFleeing {
		var admitted bool
		includeSkill, admitted = usercommands.TakeFleeAdmission(user)
		if !admitted {
			return true
		}
	}

	// Revert to Default combat regardless of outcome (legacy path only).
	// When CombatPhase is wired, ResolveFlee(false) handles the revert.
	if user.Character.Aggro != nil && user.Character.Aggro.Type == characters.Flee {
		user.Character.SetAggro(user.Character.Aggro.UserId, user.Character.Aggro.MobInstanceId, characters.DefaultAttack)
	}

	// Can't flee while in any grapple state. CombatPhase position veto
	// also blocks TransitionToDisengaging, but the message still needs
	// to fire here for UX. Chunk 4b R3: FSM-driven — IsStandingGrapple
	// || IsGroundGrapple covers all 11 grapple states.
	if user.Character.IsStandingGrapple() || user.Character.IsGroundGrapple() {
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You can't flee while grappled!</ansi>`)
		if user.Character.CombatPhase != nil {
			user.Character.CombatPhase.ResolveFlee(false)
		}
		return true
	}

	// Shared opposed-roll blocker resolution (combat.ResolveFleeBlockers).
	// Replaces two duplicated loops; also corrects the prior
	// variable-shadowing in the player-blockers loop (the inner
	// `for _, userId := range` shadowed the outer fleer's id, so PvP
	// players never blocked each other from fleeing). Perspective-
	// specific messaging stays here.
	blocker, contested := combat.ResolveFleeBlockers(user.Character, uRoom, includeSkill)
	// Skullduggery practice is awarded HERE, by the wrapper, and only when an
	// opposed roll actually happened. Two conditions, both load-bearing:
	// includeSkill is false when the fleer was too spent to pay in full and
	// therefore never brought the skill to the contest (practising a skill you
	// did not use is not practice), and contested is false when nothing in the
	// room was targeting the fleer, so there was no contest to learn from.
	if contested && includeSkill {
		user.Character.OnSkillUse(string(skills.Skullduggery), user.UserId)
	}
	if blocker != nil {
		var targetTag string
		if blocker.IsPlayer() {
			targetTag = "username"
		} else {
			targetTag = "mobname"
		}
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="red-bold"><ansi fg="%s">%s</ansi> blocks you from fleeing!</ansi>`, targetTag, blocker.Name))
		excludes := []int{user.UserId}
		if blocker.IsPlayer() {
			excludes = append(excludes, blocker.UserId)
		}
		uRoom.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="username">%s</ansi> is blocked from fleeing by <ansi fg="%s">%s</ansi>!`, user.Character.Name, targetTag, blocker.Name), excludes...)
		// Task 15: flee failure — restore Engaged state in CombatPhase.
		if user.Character.CombatPhase != nil {
			user.Character.CombatPhase.ResolveFlee(false)
		}
		return true
	}

	// Success!
	exitName, exitRoomId := uRoom.GetRandomExit()

	if exitName == `` {
		user.SendText(messaging.CategorySystem, `You can't find an exit!`)
		// No exit found — treat as blocked (flee failure).
		if user.Character.CombatPhase != nil {
			user.Character.CombatPhase.ResolveFlee(false)
		}
		return true
	}

	user.SendText(messaging.CategoryRoomExit, fmt.Sprintf(`You flee to the <ansi fg="exit">%s</ansi> exit!`, exitName))
	uRoom.SendText(messaging.CategoryRoomExit, fmt.Sprintf(`<ansi fg="username">%s</ansi> flees to the <ansi fg="exit">%s</ansi> exit!`, user.Character.Name, exitName), user.UserId)

	// Task 15: flee success — EndAggro clears legacy Aggro; ResolveFlee
	// transitions CombatPhase Disengaging → Idle.
	user.Character.EndAggro()
	if user.Character.CombatPhase != nil {
		user.Character.CombatPhase.ResolveFlee(true)
	}

	if err := rooms.MoveToRoom(user.UserId, exitRoomId); err == nil {

		for _, instId := range uRoom.GetMobs(rooms.FindCharmed) {
			if mob := mobs.GetInstance(instId); mob != nil {
				if mob.Character.IsCharmed(userId) {
					mob.Command(exitName)
				}
			}
		}

		newRoom := rooms.LoadRoom(exitRoomId)
		usercommands.Look(``, user, newRoom, events.CmdSecretly)

		// Fire the room behavior tree's room_enter event for the destination,
		// exactly as walked movement (go.go) does. Without this, fleeing INTO a
		// room silently skips its on-entry effects — e.g. the newcomer tutorial's
		// final room (6467), reached by fleeing the effigy, never delivered its
		// guide's "talk to me" instruction, stranding the player (2026-07-17
		// playtest). Correct in general: entering a room by any means should
		// trigger its entry hooks.
		behaviortree.TryRoomBehavior(exitRoomId, behaviortree.EventContext{
			EventType: "room_enter",
			UserId:    user.UserId,
			RoomId:    exitRoomId,
		})
	}

	return true
}

// handleCompanionOwnerAssist triggers a companion's owner (and the owner's other
// companions) to fight back when the companion is attacked.
// attackerDesc is the attack-command argument that identifies the attacker
// (e.g. "#42" for a mob instance or "@7" for a player).
func handleCompanionOwnerAssist(defMob *mobs.Mob, attackerDesc string) {
	ownerId := defMob.Character.GetCharmedUserId()
	if ownerId == 0 {
		return
	}
	owner := users.GetByUserId(ownerId)
	if owner == nil {
		return
	}

	// Find the companion entry to check AutoAssist.
	comp := owner.Character.GetCompanionByInstanceId(defMob.InstanceId)
	if comp == nil || !comp.AutoAssist {
		return
	}

	// Owner fights back if not already in combat.
	if owner.Character.Aggro == nil {
		owner.Command(fmt.Sprintf("attack %s", attackerDesc))
	}

	// Other companions of the same owner also assist.
	ownerRoom := rooms.LoadRoom(owner.Character.RoomId)
	if ownerRoom != nil {
		handleCharmedMobAssist(ownerRoom, ownerId, attackerDesc)
	}
}

// handleCharmedMobAssist triggers charmed mobs to assist their owner when attacked.
// Only engages companions that have AutoAssist enabled.
func handleCharmedMobAssist(room *rooms.Room, defId int, targetDesc string) {
	defUser := users.GetByUserId(defId)
	if defUser == nil {
		return
	}
	for _, instanceId := range room.GetMobs(rooms.FindCharmed) {
		if charmedMob := mobs.GetInstance(instanceId); charmedMob != nil {
			if charmedMob.Character.IsCharmed(defId) && charmedMob.Character.Aggro == nil {
				comp := defUser.Character.GetCompanionByInstanceId(instanceId)
				if comp != nil && comp.AutoAssist {
					charmedMob.Command(fmt.Sprintf("attack %s", targetDesc))
				}
			}
		}
	}
}

// handleOffhandBreakUserDef handles offhand item breakage when a player defender is hit.
func handleOffhandBreakUserDef(roundResult combat.AttackResult, defUser *users.UserRecord, defRoom *rooms.Room) {
	br := tryWeaponBreak(defUser.Character, roundResult, defRoom)
	if !br.Broke {
		return
	}

	defUser.SendText(messaging.CategoryEquipment, `<ansi fg="202">***</ansi>`)
	defUser.SendText(messaging.CategoryEquipment, fmt.Sprintf(`<ansi fg="214"><ansi fg="202">***</ansi> Your <ansi fg="item">%s</ansi> breaks! <ansi fg="202">***</ansi></ansi>`, br.BrokenItemName))
	defUser.SendText(messaging.CategoryEquipment, `<ansi fg="202">***</ansi>`)

	defRoom.SendText(messaging.CategoryEquipment, fmt.Sprintf(`<ansi fg="214"><ansi fg="202">***</ansi> The <ansi fg="item">%s</ansi> <ansi fg="username">%s</ansi> was carrying breaks! <ansi fg="202">***</ansi></ansi>`, br.BrokenItemName, defUser.Character.Name), defUser.UserId)

	events.AddToQueue(events.ItemOwnership{
		UserId: defUser.UserId,
		Item:   br.BrokenItem,
		Gained: false,
	})

	events.AddToQueue(events.ItemOwnership{
		UserId: defUser.UserId,
		Item:   br.ReplacementItem,
		Gained: true,
	})
}

// handleOffhandBreakMobDef handles offhand item breakage when a mob defender is hit.
func handleOffhandBreakMobDef(roundResult combat.AttackResult, defMob *mobs.Mob) {
	defRoom := rooms.LoadRoom(defMob.Character.RoomId)
	br := tryWeaponBreak(&defMob.Character, roundResult, defRoom)
	if !br.Broke {
		return
	}

	if defRoom != nil {
		defRoom.SendText(messaging.CategoryEquipment, fmt.Sprintf(`<ansi fg="214"><ansi fg="202">***</ansi> The <ansi fg="item">%s</ansi> <ansi fg="mobname">%s</ansi> was carrying breaks! <ansi fg="202">***</ansi></ansi>`, br.BrokenItemName, defMob.Character.Name))
	}

	events.AddToQueue(events.ItemOwnership{
		MobInstanceId: defMob.InstanceId,
		Item:          br.BrokenItem,
		Gained:        false,
	})

	events.AddToQueue(events.ItemOwnership{
		MobInstanceId: defMob.InstanceId,
		Item:          br.ReplacementItem,
		Gained:        true,
	})
}

// handlePlayerConcentrationBreak checks if a caster's concentration breaks when hit,
// and hard-cancels any in-progress crafting or salvaging activity on the same damage hit.
// The two helpers are independent: a Casting character gets a willpower roll;
// a Crafting/Salvaging character is always interrupted (no roll).
func handlePlayerConcentrationBreak(defUser *users.UserRecord, roundResult combat.AttackResult, defRoom *rooms.Room) {
	// Hard-cancel craft/salvage on any damage (no roll needed).
	if roundResult.DamageToTarget > 0 {
		cancelCraftOrSalvageOnDamage(defUser.Character)
	}
	if checkConcentrationBreak(defUser.Character, roundResult.DamageToTarget) {
		csSnap, _ := defUser.Character.Activity.CastingData()
		recordConcentrationFailure(combat.User, combat.Mob, defUser.Character, castingTargetChar(csSnap))
		clearCastingActivity(defUser.Character, activity.TriggerConcentrationBreak)
		events.AddToQueue(events.CastInterrupted{UserId: defUser.UserId, SpellId: csSnap.SpellId})
		defUser.SendText(messaging.CategorySpellDisruption, `<ansi fg="red">The pain shatters your concentration!</ansi>`)
		defRoom.SendText(messaging.CategorySpellDisruption, fmt.Sprintf(
			`<ansi fg="username">%s</ansi>'s concentration breaks.`,
			defUser.Character.Name), defUser.UserId)
	}
}

// handleMobAIDecision processes mob AI decisions (spell casting, special moves, combat commands).
// Returns true if the mob executed an AI action and should skip normal combat.
func handleMobAIDecision(mob *mobs.Mob, c configs.Config) bool {
	// Aggro can be cleared mid-round before this AI pass runs — e.g. a kiting
	// archer whose target left the room (keep_distance repositions it, and the
	// unified combat handler drops a mob's aggro when its target isn't present).
	// The unified dispatch below guards on Aggro != nil; mirror that here so we
	// never deref a nil Aggro (was a server-crashing nil-pointer panic).
	if mob.Character.Aggro == nil {
		return false
	}
	if mob.Character.Aggro.Type != characters.DefaultAttack {
		return false
	}

	// Stage 11.5: Caster AI decision - try spell first, then special move
	var chosenMove string
	if util.Rand(100) < mob.ActivityLevel {
		var targetChar *characters.Character
		if mob.Character.Aggro.UserId > 0 {
			if u := users.GetByUserId(mob.Character.Aggro.UserId); u != nil {
				targetChar = u.Character
			}
		} else if mob.Character.Aggro.MobInstanceId > 0 {
			if tm := mobs.GetInstance(mob.Character.Aggro.MobInstanceId); tm != nil {
				targetChar = &tm.Character
			}
		}
		if targetChar != nil {
			chosenMove = combat.ChooseCastAction(mob)
			if chosenMove == "" {
				chosenMove = combat.ChooseSpecialMove(mob, targetChar)
			}
		}
	}

	// Execute AI-chosen move or fall back to CombatCommands
	if chosenMove != "" {
		mob.Command(chosenMove, 0)
		return true
	}

	// If they have combat commands, maybe do one of them?
	cmdCt := len(mob.CombatCommands)
	if cmdCt > 0 {
		if util.Rand(100) < mob.ActivityLevel {
			combatAction := mob.CombatCommands[util.Rand(cmdCt)]

			if combatAction == `` {
				return true
			}

			var waitTime float64 = 0.0
			allCmds := strings.Split(combatAction, `;`)
			if len(allCmds) >= c.Timing.TurnsPerRound() {
				mob.Command(`say I have a CombatAction that is too long. Please notify an admin.`)
			} else {
				for _, action := range strings.Split(combatAction, `;`) {
					mob.Command(action, waitTime)
					waitTime += 0.1
				}
			}
			return true
		}
	}

	return false
}

// handleMobTargetSwitch processes mob target switching AI.
// Returns true if the mob switched targets and should skip this round.
func handleMobTargetSwitch(mob *mobs.Mob, mobRoom *rooms.Room) bool {
	if util.Rand(100) >= 10 || mob.Character.Aggro.Type != characters.DefaultAttack {
		return false
	}

	combatSkill := mob.Character.GetCombatSkillLevel()
	if combatSkill < 30 {
		return false
	}

	potentialTargets := []int{}
	for _, userId := range mobRoom.GetPlayers() {
		if userId == mob.Character.Aggro.UserId {
			continue
		}
		if u := users.GetByUserId(userId); u != nil {
			if u.Character.Health > 0 && !u.Character.IsHidden() {
				if u.Character.Aggro != nil && u.Character.Aggro.MobInstanceId == mob.InstanceId {
					potentialTargets = append(potentialTargets, userId)
				}
			}
		}
	}

	if len(potentialTargets) == 0 {
		return false
	}

	switchChance := combat.ChanceToSwitchTarget(&mob.Character)
	roll := util.Rand(100)
	util.LogRoll("Mob Target Switch", roll, switchChance)

	if roll < switchChance {
		newTargetId := potentialTargets[util.Rand(len(potentialTargets))]
		mob.Character.SetAggro(newTargetId, 0, mob.Character.Aggro.Type, 1)

		if newTarget := users.GetByUserId(newTargetId); newTarget != nil {
			mobRoom.SendText(messaging.CategoryMobEmote,
				fmt.Sprintf("%s shifts focus to <ansi fg=\"username\">%s</ansi>!", mobDisplayName(mob, mobRoom, 0), newTarget.Character.Name),
			)
		}
		return true
	}

	return false
}

// handleMobWeaponPickup tries to equip a weapon from inventory when disarmed.
func handleMobWeaponPickup(mob *mobs.Mob) {
	if mob.Character.Equipment.Weapon.ItemId != 0 || len(mob.Character.Items) == 0 {
		return
	}

	roll := util.Rand(100)
	util.LogRoll(`Look for weapon`, roll, mob.Character.Stats.Charisma.ValueAdj)

	if roll < mob.Character.Stats.Charisma.ValueAdj {
		possibleWeapons := []string{}
		for _, itm := range mob.Character.Items {
			iSpec := itm.GetSpec()
			if iSpec.Type == items.Weapon {
				possibleWeapons = append(possibleWeapons, itm.DisplayName())
			}
		}

		if len(possibleWeapons) > 0 {
			mob.Command(fmt.Sprintf("equip %s", possibleWeapons[util.Rand(len(possibleWeapons))]))
		}
	}
}

// handlePartyAutoAttack triggers auto-attack for party members when one is attacked by a mob.
// Uses the persistent per-character "autoattack" setting instead of the party-level list.
func handlePartyAutoAttack(mob *mobs.Mob, defUser *users.UserRecord) {
	if party := parties.Get(defUser.UserId); party != nil {
		for _, memberId := range party.UserIds {
			if memberId == defUser.UserId {
				continue
			}
			if memberUser := users.GetByUserId(memberId); memberUser != nil {
				if memberUser.Character.RoomId == defUser.Character.RoomId &&
					memberUser.Character.GetSetting("autoattack") != "off" &&
					memberUser.Character.Aggro == nil {
					memberUser.Command(fmt.Sprintf(`attack #%d`, mob.InstanceId))
				}
			}
		}
	}
}
