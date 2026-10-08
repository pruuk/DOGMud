package hooks

import (
	"fmt"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/progression"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// processAttackerProgression fires the round's ONE ordinary attacker award: the
// SKILL that rolled best across every swing the attacker threw, at full weight
// when the round landed a clean hit and at Balance.ProgressionFailureFraction
// when it did not.
//
// One award per ROUND, mirroring processDefenderProgression. U10b-1 Task 10
// first removed the `if !wh.CleanHit { continue }` gate so a missed swing
// trained something, then Task 11 collapsed the per-weapon loop that gate sat
// in. Both halves matter and the second is the one that fixes a real
// distortion.
//
// WHY NOT PER WEAPON. AttackResult.WeaponHits carries one entry per HAND SLOT,
// not per swing: collectAttackWeapons contributes a fist for every empty hand
// (CombatSkillTagForItem maps ItemId 0 to unarmed-combat) and the extra-arms
// mutation adds up to four more slots. Paying per entry therefore paid per
// hand, which produced a six-to-one spread that tracked nothing a player would
// recognise as effort:
//
//	two-handed weapon      1 entry   -> 1 award
//	one-handed + empty off 2 entries -> 2 awards (one of them unarmed-combat)
//	dual wield             2 entries -> 2 awards
//	bare hands             2 entries -> 2 awards
//	extra arms L4          6 entries -> 6 awards
//
// The most committed weapon in the game came last, a sword-and-nothing fighter
// silently trained unarmed-combat every round off the empty hand, and
// weapon SPEED -- ws.swingCount, the thing a player would actually call "more
// swings" -- contributed nothing at all, because every swing of one weapon
// folds into that weapon's single entry.
//
// CANDIDATES ARE KEYED BY SKILL, NOT BY WEAPON. Three weapon-combat weapons
// collapse to ONE weapon-combat candidate carrying the best roll among them.
// Keying by entry would rebuild per-hand payment inside the Best-of and put the
// spread straight back.
//
// MORE ARMS STILL HELP, through the WIN RATE rather than the award count.
// progression.Candidate.Roll only SELECTS which skill earns the event; what
// sizes the event is won/lost. A six-armed attacker has six chances for one of
// them to clean-hit, so they take full weight far more often than a two-handed
// attacker does. At the measured 0.3856 per-entry clean-hit rate that is an
// expected award of roughly 0.60 at one entry against 0.97 at six -- about
// 1.6x, tapering, rather than 6x flat.
//
// THE RATE CHANGE IS STILL AN INCREASE. Per round the attacker goes from
// P(clean hit) = 0.3856 awards to exactly 1. The owed re-solve of
// SkillProgressionMultipliers must use 0.3856 and NOT the 0.5752 in the design
// spec's risk table, which is the HIT rate mislabelled as the clean-hit rate.
//
// SKULLDUGGERY JOINS THE SAME CONTEST on a surprise attack rather than taking a
// second award beside it. See surpriseCandidate for the one caveat there: it is
// the only candidate whose roll did not actually happen.
//
// NOTE: there is no `len(WeaponHits) == 0` fallback. One was deleted here, and
// it was DEAD: collectAttackWeapons cannot return empty (every hand slot
// contributes, and a final fallback appends a bare fist when nothing else did),
// buildAttackPlan filters none of it, calcSwingCount has a minimum of 1, and
// calculateCombat appends exactly one entry per plan weapon unconditionally. Its
// second condition made it doubly unreachable: result.CleanHit can only be set
// inside the swing loop, which cannot run without a plan weapon. Do not
// reintroduce a round-level consolation award beside the loop -- everything this
// function pays comes from WeaponHits.
func processAttackerProgression(c *characters.Character, userId int, result combat.AttackResult) {
	if c == nil {
		return
	}
	cands, cleanBySkill := attackerCandidates(c, result)
	if sc, ok := surpriseCandidate(c, result); ok {
		cands = append(cands, sc)
		// A surprise candidate is only built for a round that CLEAN-HIT, so
		// its own outcome is a win by construction.
		cleanBySkill[sc.Skill] = true
	}
	best, ok := progression.BestOf(cands)
	if !ok {
		return
	}
	// won is the WINNING SKILL's own outcome, not the round aggregate.
	//
	// This deliberately matches processDefenderProgression, which passes the
	// selected defence's own best.Won. An earlier draft passed
	// AttackResult.CleanHit -- "did anything land this round" -- and that is a
	// materially different rule, not a rephrasing. For a one-handed weapon
	// beside an empty hand the two skills are different, weapon-combat almost
	// always wins selection, and P(the fist landed cleanly while the weapon
	// did not) is roughly 0.24 at the measured rates. About one round in four
	// would have paid weapon-combat FULL weight for a round in which the sword
	// never won a contest.
	//
	// Full weight has to mean "you succeeded with THIS skill", or the fraction
	// stops being a statement about the skill being trained.
	c.AwardResolved(userId, cleanBySkill[best.Skill], best)
}

// attackerCandidates folds the round's weapon entries into ONE candidate per
// SKILL, carrying that skill's best actual attack roll, and reports whether
// each skill landed a clean hit anywhere in the round.
//
// Deterministic order, which BestOf's full-tie rule requires: candidates come
// out in first-appearance order of WeaponHits, never in map order. Two skills
// tying on both Roll and Level is vanishingly unlikely with real float rolls,
// but "vanishingly unlikely" is not "never", and a map here would rotate the
// winner between rounds for no visible reason. That determinism is the WHOLE
// reason to dedup by skill: an earlier draft claimed entry-keying would
// "rebuild per-hand payment", which is false -- payment count is fixed at one
// by the single AwardResolved call regardless of how many candidates BestOf is
// handed. Entry-keying and skill-keying pick the same winner in every case
// except an exact float tie between two different skills.
//
// An unrecognised SkillTag is dropped rather than carried: characters.
// CandidateFor returns the zero Candidate for one, and a zero candidate that
// happened to out-roll the real ones would make BestOf report false and the
// whole round train nothing. Dropping it costs only the unknown skill.
//
// ✅ THE ROLL DISCRIMINATES BY SKILL as of the 2026-08-27 offhand fix.
// calcAttackScore used to build every swing's score from
// characters.GetCombatSkillLevel, which resolves the MAIN-HAND weapon's tag for
// every entry in the plan -- so an offhand fist's attack roll was centred on a
// score containing WEAPON-combat's rank, not unarmed-combat's, and which of two
// DIFFERENT skills trained was decided by the dual-wield penalty rather than by
// either skill's own rank. calcAttackScore now calls GetCombatSkillLevelFor
// with the weapon being swung, so each candidate's roll carries its own skill.
//
// ⚠️ ONE PIECE OF THE SAME DEFECT IS STILL LIVE, and it is a SWING COUNT rather
// than a score: calcSwingCount (combat_helpers.go:170) still reads main-hand-
// only GetCombatSkillLevel, so an offhand fist beside a sword gets a swing
// count derived from weapon-combat's rank. More swings means more chances to
// set BestRoll, so it still tilts selection toward the offhand, just far less
// than the score term did.
func attackerCandidates(c *characters.Character, result combat.AttackResult) ([]progression.Candidate, map[string]bool) {
	order := make([]string, 0, len(result.WeaponHits))
	bestRoll := make(map[string]float64, len(result.WeaponHits))
	clean := make(map[string]bool, len(result.WeaponHits))

	for _, wh := range result.WeaponHits {
		// Drop unknown skills, not merely empty ones. GetSkillPrimaryStat
		// returns "" for anything not in skills.SkillPrimaryStats, which is the
		// same test characters.CandidateFor applies before it returns the zero
		// Candidate. Carrying one would be worse than dropping it: it arrives
		// with a REAL attack roll, so it can out-roll the genuine candidates,
		// and BestOf then reports false on a winner that awards nothing -- the
		// whole round would train nothing because one hand held something
		// unrecognised.
		if skills.GetSkillPrimaryStat(wh.SkillTag) == "" {
			continue
		}
		// A skill counts as having won the round if ANY of its entries clean-hit,
		// mirroring how AttackResult.CleanHit aggregates across a weapon's
		// swings. Two fists are one skill, so either landing is that skill
		// landing.
		clean[wh.SkillTag] = clean[wh.SkillTag] || wh.CleanHit

		if _, seen := bestRoll[wh.SkillTag]; !seen {
			order = append(order, wh.SkillTag)
			bestRoll[wh.SkillTag] = wh.BestRoll
			continue
		}
		if wh.BestRoll > bestRoll[wh.SkillTag] {
			bestRoll[wh.SkillTag] = wh.BestRoll
		}
	}

	cands := make([]progression.Candidate, 0, len(order)+1)
	for _, skill := range order {
		// Stat stays empty, meaning "the skill's primary". That is EQUIVALENT
		// to the explicit AttackerStat: skills.GetSkillPrimaryStat(...) the
		// pre-U10b-1 code passed, because ApplyProgression only pays a separate
		// stat roll when an ordinary event names a stat DIFFERENT from the
		// skill's primary. It is also safer: a populated Stat that DID differ
		// would silently double-roll, which is the block/defy shape that
		// U10b-1 Task 4 had to fix elsewhere.
		cands = append(cands, progression.Candidate{
			Skill: skill,
			Roll:  bestRoll[skill],
			Level: c.GetSkillLevel(skills.SkillTag(skill)),
		})
	}
	return cands, clean
}

// processDefenderProgression fires ONE skill-and-stat progression award for the
// defender per melee round: the defence that ROLLED BEST across the round's
// swings, at full weight if that defence won and at
// Balance.ProgressionFailureFraction if it lost.
//
// U10b-1 Task 9 changed both halves of that sentence. Before it this looped
// combat.AwardDefenceProgression once per defence TYPE that had WON -- keyed on
// AttackResult.SwingEvents' DefenseUsed, which sendDefenseMessages stamps only
// on a defensive win -- so a round in which every defence lost trained nothing
// at all, and a defender with dodge, parry and block took up to three rolls
// where a bare-handed one took one. The firing convention is now Best-of: one
// resolved action, one event, for the single highest-rolling candidate.
//
// The REDISTRIBUTION is deliberate and is not a uniform gain. A defender with
// one defence gains (a lost round now trains at the fraction). A shield user
// loses: parry and block both train weapon-combat, so a round that quoted both
// used to be able to take two weapon-combat rolls and now takes one.
//
// An UNCONTESTED round awards nothing. runBestOfAllDefense leaves the defence
// name empty when the defender had no defence available, the swing loop appends
// no SwingDefence for it, and BestOf on an empty slice reports false.
//
// The de-duplication that used to be this function's only job beyond
// AwardDefenceProgression is now BestOf's: a defender who dodges four swings
// still gets one dodge award, because four dodge candidates collapse to the one
// that rolled highest.
//
// Quell and defy still cannot reach this function -- neither is in melee's
// defence set -- but AwardDefenceProgression covers both, so wiring either into
// melee stays a row in combatvocab's eligibility table and nothing else.
func processDefenderProgression(c *characters.Character, userId int, result combat.AttackResult) {
	best, ok := bestSwingDefence(c, result.SwingDefences)
	if !ok {
		return
	}
	combat.AwardDefenceProgression(c, userId, best.Defence, best.Won)
}

// bestSwingDefence picks the ONE swing defence that earns the round's defender
// award, and reports false when there is nothing to award.
//
// The choice is DELEGATED to progression.BestOf rather than reimplemented here:
// that function is the arc's single definition of "which candidate earns the
// event", down to the tiebreaks (highest roll, then highest level, then slice
// order), and a second copy would drift from it the first time the rule
// changed.
//
// Three things are worth stating about the mapping.
//
// The Roll handed to BestOf is the defence's ACTUAL contest roll, already made
// during the swing, not a fresh characters.CandidateFor roll. Re-rolling would
// add a second source of randomness on top of the one that already decided the
// swing.
//
// These rolls do NOT all share one scale, and the comparison does not need them
// to. contest.Run rolls every defence with dice.StdDevFor(atkScore), but
// atkScore is recomputed PER SWING (combat.go, calcAttackScore) and subtracts
// ws.penalty, which differs per weapon -- so a dual-wielder's mainhand and
// offhand swings roll their defences with different spreads. Two more sources
// drift within a round: a defence's own governing-skill addend is dropped when
// its immutable quote stops being affordable as stamina drains
// (includeSkill in runBestOfAllDefense), and the prone/clinch/grounded
// penalties are per-defence. What survives all of that is the only property
// this selection needs: every roll is centred on its OWN defence's score, so no
// defence is systematically favoured by the choice of scale.
//
// The Candidates built here are SELECTION-ONLY. None of them reaches
// ApplyProgression -- BestOf's winner is thrown away except for the index it
// identifies, and the award goes out through combat.AwardDefenceProgression,
// which re-derives skill and stat from the defence type. That is why populating
// Stat here is safe: it sharpens the by-value recovery below without incurring
// the double stat roll characters.CandidateFor warns a caller about. Handing
// one of these to AwardResolved instead WOULD incur it.
//
// The winner is recovered by VALUE rather than by index because BestOf returns
// the Candidate, not its position. That is sound here: each defence type
// produces a distinct (Skill, Stat) pair -- dodge is unarmed-combat/dexterity,
// parry weapon-combat/dexterity, block weapon-combat/strength -- so two
// candidates can only compare equal when they name the same defence, and the
// walk below takes the first match in the same slice order BestOf's full-tie
// rule uses.
func bestSwingDefence(c *characters.Character, quoted []combat.SwingDefence) (combat.SwingDefence, bool) {
	if c == nil || len(quoted) == 0 {
		return combat.SwingDefence{}, false
	}

	cands := make([]progression.Candidate, 0, len(quoted))
	for _, q := range quoted {
		skill, stat := combat.DefenceSkillAndStat(q.Defence)
		cands = append(cands, progression.Candidate{
			Skill: skill,
			Stat:  stat,
			Roll:  q.Roll,
			Level: c.GetSkillLevel(skills.SkillTag(skill)),
		})
	}

	winner, ok := progression.BestOf(cands)
	if !ok {
		return combat.SwingDefence{}, false
	}
	for i, cand := range cands {
		if cand == winner {
			return quoted[i], true
		}
	}
	return combat.SwingDefence{}, false
}

// defenceTypesUsed returns the set of defences that registered this round, in
// the same fixed order processDefenderProgression uses. Extracted so the seam
// and the ordinary award read one definition of "which defences happened".
func defenceTypesUsed(result combat.AttackResult) []combatvocab.Defence {
	used := make(map[combatvocab.Defence]bool, 3)
	for _, se := range result.SwingEvents {
		if se.DefenseUsed != combatvocab.DefenceNone {
			used[se.DefenseUsed] = true
		}
	}
	out := make([]combatvocab.Defence, 0, 3)
	for _, d := range []combatvocab.Defence{combatvocab.DefenceDodge, combatvocab.DefenceParry, combatvocab.DefenceBlock} {
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
func defenceSkillFor(used []combatvocab.Defence) string {
	if len(used) == 0 {
		return ""
	}
	skill, _ := combat.DefenceSkillAndStat(used[0])
	return skill
}

// defenceStatFor is defenceSkillFor's stat counterpart, from the same mapping.
func defenceStatFor(used []combatvocab.Defence) string {
	if len(used) == 0 {
		return ""
	}
	_, stat := combat.DefenceSkillAndStat(used[0])
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
//
// A mob hidden from the viewer reads as "something" (lowercase: the line may
// put it mid-sentence; mobSubjectName is the line-opening form). Viewer 0 is
// a room-wide line, which has no one reader to ask Perceives of, so a hidden
// mob is unseen in it by every reader, as a hidden caster is in its
// spell-channel lines (#274, owner R3). GetMobNameIndexed alone named it to
// every faces reader as "Skeleton (hidden)" (#382).
func mobDisplayName(mob *mobs.Mob, room *rooms.Room, viewingUserId int) string {
	if mobHiddenFrom(mob, viewingUserId) {
		return messaging.UnseenFigure(messaging.SightNone)
	}
	dupIdx := room.GetMobDuplicateIndex(mob.InstanceId)
	return mob.Character.GetMobNameIndexed(viewingUserId, dupIdx).String()
}

// mobPlainName is mobDisplayName's untagged twin, for a condition line's
// {actee_plain}: the bare name, or "something" for a hidden mob. A room line
// is the only place it is read, so it never asks a viewer.
func mobPlainName(mob *mobs.Mob) string {
	if mobHiddenFrom(mob, 0) {
		return messaging.UnseenNoun(messaging.SightNone)
	}
	return mob.Character.GetCharacterName(false)
}

// mobHiddenFrom reports whether a line about mob must not name it to
// viewingUserId: the mob is hidden and the viewer does not Perceive it. Viewer
// 0 (a room-wide line) perceives no hidden mob.
func mobHiddenFrom(mob *mobs.Mob, viewingUserId int) bool {
	if !mob.Character.IsHidden() {
		return false
	}
	if viewingUserId == 0 {
		return true
	}
	u := users.GetByUserId(viewingUserId)
	return u == nil || !u.Character.Perceives(&mob.Character)
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

// sendDarkRoomCombatFallback sends a one-time "You hear fighting close by."
// line in a dark room to every player who cannot follow the fight by eye: one the
// visual pipeline delivers nothing to (messaging.CanSeeShapes is false).
// It used to test the nightvision FLAG, which sent the sound to an
// infravision holder reading shapes and withheld it from a nightvision
// holder whose window reads the room as blind (lighting plan 5c).
func sendDarkRoomCombatFallback(room *rooms.Room, excludeUserIds ...int) {
	if room == nil || room.IsLit() {
		return
	}
	for _, uid := range room.GetPlayers() {
		if isExcludedUser(uid, excludeUserIds) {
			continue
		}
		u := users.GetByUserId(uid)
		if u != nil && !messaging.CanSeeShapes(u.Character, room) {
			// #216: every caller passes the fight's own room, so the
			// reader is IN the fight's room; "nearby" said otherwise.
			u.SendText(messaging.CategoryDefault, `<ansi fg="yellow">You hear fighting close by.</ansi>`)
		}
	}
}

// sendVisualElseAudible sends visualMsg through the visual pipeline, which
// delivers it to every reader who makes out at least shapes (anonymising
// names for a shapes reader), and soundMsg to every player the pipeline
// skipped. Every player in the room reads exactly one of the two, except
// excludeUserIds, who read neither (a caster reads its own line).
func sendVisualElseAudible(room *rooms.Room, cat messaging.Category, visualMsg, soundMsg string, excludeUserIds ...int) {
	if room == nil {
		return
	}
	room.SendTextVisual(cat, visualMsg, excludeUserIds...)
	for _, uid := range room.GetPlayers() {
		if isExcludedUser(uid, excludeUserIds) {
			continue
		}
		u := users.GetByUserId(uid)
		if u != nil && !messaging.CanSeeShapes(u.Character, room) {
			u.SendText(cat, soundMsg)
		}
	}
}

// The sound lines a reader who sees nothing gets for a spell-channel
// disruption (#242, owner ruling R4). Mob and player casters share them.
const (
	spellChantBreaksOffSound = `Someone's chant breaks off.`
	spellSputtersOutSound    = `A half-formed spell sputters out.`
)

// mobSubjectName is a mob's name at the start of a room-wide line (a caster's
// spell-channel line, a mob emote): mobDisplayName for viewer 0, capitalized.
// A hidden mob is unseen by every reader, whatever they can see, and reads
// "Something", as sendSpoken does for a speaker still hidden and as a hidden
// actor's emote is silent (#274, owner R3). The capital is spelled here, not
// left to the pipeline, because the mob emote categories skip
// sentence-start capitalization.
func mobSubjectName(mob *mobs.Mob, room *rooms.Room) string {
	if mobHiddenFrom(mob, 0) {
		name := mob.Character.Name
		return messaging.HideNames(`<ansi fg="mobname">`+name+`</ansi>`, []string{name}, messaging.SightNone)
	}
	return mobDisplayName(mob, room, 0)
}

// sendMobConcentrationBroke narrates a mob caster's broken concentration:
// seen by sight (a figure at shapes), heard by a reader who sees nothing.
// It used plain Room.SendText, the unfiltered channel, which named the
// caster to everyone (#242).
func sendMobConcentrationBroke(mob *mobs.Mob, room *rooms.Room) {
	sendVisualElseAudible(room, messaging.CategorySpellDisruption, fmt.Sprintf(
		`%s's concentration breaks.`, mobSubjectName(mob, room)),
		spellChantBreaksOffSound)
}

// sendMobSpellFailed narrates a mob's spell that fizzles (target gone) or
// falters (not enough conviction); verb is "fizzles" or "falters".
func sendMobSpellFailed(mob *mobs.Mob, room *rooms.Room, verb string) {
	sendVisualElseAudible(room, messaging.CategorySpellDisruption, fmt.Sprintf(
		`%s's spell %s.`, mobSubjectName(mob, room), verb),
		spellSputtersOutSound)
}

// sendMobWeaving narrates a mob still holding its fold. Sight only: a quiet
// weave makes no sound (owner ruling R4).
func sendMobWeaving(mob *mobs.Mob, room *rooms.Room) {
	sendVisualRoomText(room, messaging.CategorySpellFold, fmt.Sprintf(
		`%s weaves magic with focused intent.`, mobSubjectName(mob, room)))
}

// sendPlayerConcentrationBroke narrates a player caster's broken
// concentration to the rest of the room, as sendMobConcentrationBroke does
// for a mob: seen by sight, heard by a reader who sees nothing. The caster
// reads its own line. The pain-of-a-hit path used plain Room.SendText and
// named the caster to everyone (#242).
func sendPlayerConcentrationBroke(caster *users.UserRecord, room *rooms.Room) {
	sendVisualElseAudible(room, messaging.CategorySpellDisruption, fmt.Sprintf(
		`<ansi fg="username">%s</ansi>'s concentration breaks.`, caster.Character.Name),
		spellChantBreaksOffSound, caster.UserId)
}

// sendMobShiftsFocus narrates a mob switching its attack to a new player.
// Sight only (owner ruling R4); each reader sees both names at its own
// sight, as target.go's player shift-focus line does.
func sendMobShiftsFocus(mob *mobs.Mob, room *rooms.Room, newTarget *users.UserRecord) {
	room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
		fmt.Sprintf("%s shifts focus to <ansi fg=\"username\">%s</ansi>!", mobSubjectName(mob, room), newTarget.Character.Name),
		[]string{mob.Character.Name, newTarget.Character.Name},
	)
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
		sendPlayerConcentrationBroke(user, rooms.LoadRoom(user.Character.RoomId))

	case result.GrappleBroke:
		// Chunk 4e T4: grapple breaks concentration same as Prone (spec §4.2).
		recordConcentrationFailure(combat.User, combat.Mob, user.Character, castingTargetChar(csBeforeProcess))
		user.SendText(messaging.CategorySpellDisruption, `<ansi fg="red">Your concentration shatters. You cannot hold the fold while grappled!</ansi>`)
		sendPlayerConcentrationBroke(user, rooms.LoadRoom(user.Character.RoomId))

	case result.TargetGone:
		recordConcentrationFailure(combat.User, combat.Mob, user.Character, castingTargetChar(csBeforeProcess))
		user.SendText(messaging.CategorySpellDisruption, `<ansi fg="red">Your spell fizzles. The target is gone.</ansi>`)

	case result.SpellDataMissing:
		user.SendText(messaging.CategorySpellDisruption, `<ansi fg="red">The spell dissipates. Its data cannot be found.</ansi>`)

	case result.InsufficientConviction:
		recordConcentrationFailure(combat.User, combat.Mob, user.Character, castingTargetChar(csBeforeProcess))
		user.SendText(messaging.CategorySpellDisruption, `<ansi fg="red">Your conviction wavers, and the fold collapses.</ansi>`)

	case result.CastComplete:
		cs := result.CastingData
		spellData := result.SpellData
		// Send YAML wait text (if defined).
		if spellData != nil && spellData.Narration(spells.PhaseWait).Len() > 0 {
			roles := spellData.Narrate(spells.PhaseWait, textutil.TokenContext{
				ActorName:      user.Character.GetCharacterName(true),
				ActorPlainName: user.Character.GetCharacterName(false),
			})
			if roles.Actor != "" {
				user.SendText(messaging.CategorySpellFold, roles.Actor)
			}
			// Audio channel, as before this refactor: filed, not changed here.
			if roles.Observer != "" {
				if r := rooms.LoadRoom(user.Character.RoomId); r != nil {
					r.SendText(messaging.CategorySpellFold, roles.Observer, user.UserId)
				}
			}
		}

		resolveRoom := rooms.LoadRoom(user.Character.RoomId)
		castLanded := false
		if resolveRoom != nil {
			castLanded = resolveSpell(user, cs, spellData, resolveRoom)
		}
		user.Character.TrackSpellCast(cs.SpellId)
		// Fire progression for the correct skill based on spell school.
		//
		// U10b-3: the DIFFICULTY multiplier that used to open this expression
		// (1 + Difficulty * SpellDifficultyProgressionScale) is gone. Difficulty
		// now decides what you can DISCOVER -- it is a spell's discovery skill
		// minimum, and it biases which candidate a discovery roll draws -- not
		// how fast casting trains. Practising a hard spell no longer trains
		// faster than practising an easy one; earning access to it is the
		// reward instead.
		//
		// ⚠️ spellBonus SURVIVES, and the two riders below are why. Deleting it
		// wholesale would take the self-cast reduction AND the AoE guard with
		// it, and the AoE guard is what stops a caster farming progression by
		// casting area spells at an empty room.
		spellBonus := 1.0
		if spellData != nil {
			bal := configs.GetBalanceConfig()

			// Self-cast penalty: a single-target help spell targeting only self gets reduced progression
			if !spellData.IsHarm() && spellData.Targeting == combatvocab.TargetSingle &&
				len(cs.TargetMobInstanceIds) == 0 &&
				len(cs.TargetUserIds) == 1 && cs.TargetUserIds[0] == userId {
				spellBonus *= float64(bal.SelfCastProgressionMultiplier)
			}

			// AoE guard: an area or multi harm spell with no targets hit skips progression
			if spellData.IsHarm() && (spellData.Targeting == combatvocab.TargetArea || spellData.Targeting == combatvocab.TargetMulti) &&
				len(cs.TargetUserIds) == 0 && len(cs.TargetMobInstanceIds) == 0 {
				spellBonus = 0
			}
		}

		if spellBonus > 0 {
			// U10b-1 Task 13: ONE CAST IS ONE RESOLVED ACTION. This already
			// fired once per cast rather than once per target; what it lacked
			// was an outcome. It now pays full weight when ANY target's contest
			// was won and ProgressionFailureFraction when every one of them was
			// defended -- a caster who watched three enemies all shrug off the
			// same spell learned something, and used to be paid as if the cast
			// had never resolved.
			//
			// spellBonus is a separate axis from the win/lose weight. Since
			// U10b-3 it carries only the self-cast reduction (the difficulty
			// multiplier moved to discovery), and that reduction is a WINNING
			// multiplier below 1.0 -- which is exactly why AwardResolved takes
			// `won` rather than inferring a loss from a small multiplier.
			castSkill := skills.Spellcasting
			if spellData != nil && spellData.HasSchool(spells.SchoolManifestation) {
				castSkill = skills.Manifestation
			}
			user.Character.AwardResolvedScaled(userId, castLanded, spellBonus,
				user.Character.CandidateFor(string(castSkill)))

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
				pick := eligible[configs.WeightedDiscoveryPick(spells.DifficultiesFor(eligible), util.Rand)]
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
					pick := eligible[configs.WeightedDiscoveryPick(spells.DifficultiesFor(eligible), util.Rand)]
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
		if waitSpellInfo != nil && waitSpellInfo.Narration(spells.PhaseWait).Len() > 0 {
			roles := waitSpellInfo.Narrate(spells.PhaseWait, textutil.TokenContext{
				ActorName:      user.Character.GetCharacterName(true),
				ActorPlainName: user.Character.GetCharacterName(false),
			})
			if roles.Actor != "" {
				user.SendText(messaging.CategorySpellFold, roles.Actor)
			}
			// Audio channel, as before this refactor: filed, not changed here.
			if roles.Observer != "" {
				if r := rooms.LoadRoom(user.Character.RoomId); r != nil {
					r.SendText(messaging.CategorySpellFold, roles.Observer, user.UserId)
				}
			}
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
		sendMobConcentrationBroke(mob, mobRoom)

	case result.GrappleBroke:
		// Chunk 4e T4: grapple breaks concentration same as Prone (spec §4.2).
		recordConcentrationFailure(combat.Mob, combat.User, &mob.Character, castingTargetChar(csBeforeProcess))
		sendMobConcentrationBroke(mob, mobRoom)

	case result.TargetGone:
		recordConcentrationFailure(combat.Mob, combat.User, &mob.Character, castingTargetChar(csBeforeProcess))
		sendMobSpellFailed(mob, mobRoom, "fizzles")

	case result.SpellDataMissing:
		// Silent failure — no message for missing spell data on mobs.

	case result.InsufficientConviction:
		recordConcentrationFailure(combat.Mob, combat.User, &mob.Character, castingTargetChar(csBeforeProcess))
		sendMobSpellFailed(mob, mobRoom, "falters")

	case result.CastComplete:
		cs := result.CastingData
		spellData := result.SpellData
		castLanded := false
		if resolveRoom := rooms.LoadRoom(mob.Character.RoomId); resolveRoom != nil {
			castLanded = resolveMobSpell(mob, cs, spellData, resolveRoom)
		}
		// Stage 38.3: Mob spellcasting progression.
		//
		// U10b-3 removed the difficulty multiplier that was this block's only
		// content. Unlike the player path above, the mob path never carried the
		// self-cast reduction or the AoE guard, so there is no bonus left to
		// compute and the plain AwardResolved is now the honest call.
		// U10b-1 Task 13: one cast is one resolved action, paid at full weight
		// when ANY target's contest was won and at ProgressionFailureFraction
		// when every one was defended. See the identical conversion in
		// handlePlayerFoldCasting above.
		castSkill := skills.Spellcasting
		if spellData != nil && spellData.HasSchool(spells.SchoolManifestation) {
			castSkill = skills.Manifestation
		}
		mob.Character.AwardResolved(0, castLanded,
			mob.Character.CandidateFor(string(castSkill)))

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
				eligible := spells.GetEligibleSpellsForMob(mob.Character.SpellBook, castSkillLevel,
					spells.SchoolElemental, spells.SchoolEnhancement, spells.SchoolMental, spells.SchoolVital)
				if len(eligible) > 0 {
					pick := eligible[configs.WeightedDiscoveryPick(spells.DifficultiesFor(eligible), util.Rand)]
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
					eligible := spells.GetEligibleSpellsForMob(mob.Character.SpellBook, manifestSkillLevel,
						spells.SchoolManifestation)
					if len(eligible) > 0 {
						pick := eligible[configs.WeightedDiscoveryPick(spells.DifficultiesFor(eligible), util.Rand)]
						mob.Character.LearnSpell(pick)
					}
				}
			}
		}

	case result.StillCasting:
		sendMobWeaving(mob, mobRoom)
	}

	return true
}

// handlePlayerFlee resolves a player's flee on its round through the shared
// actions.ResolveFlee and renders the player's lines. Returns true when the
// player is fleeing and should skip combat this round.
func handlePlayerFlee(user *users.UserRecord, uRoom *rooms.Room, userId int) bool {
	out := actions.ResolveFlee(actions.NewUserActorInRoom(user, uRoom), uRoom)
	if !out.Fleeing {
		return false
	}
	if !out.Resolved {
		return true
	}

	switch {
	case out.Grappled:
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You can't flee while grappled!</ansi>`)
		return true
	case out.Blocker != nil:
		blocker := out.Blocker
		targetTag := "mobname"
		if blocker.IsPlayer() {
			targetTag = "username"
		}
		// The fleer reads the blocker at their own sight, as the room line
		// below does for everyone else. The literal stays INLINE for the root
		// viewpoint guard.
		user.SendText(messaging.CategorySystem, messaging.HideNames(
			fmt.Sprintf(`<ansi fg="red-bold"><ansi fg="%s">%s</ansi> blocks you from fleeing!</ansi>`, targetTag, blocker.Name),
			[]string{blocker.Name},
			messaging.ParticipantSight(user.Character, uRoom)))
		excludes := []int{user.UserId}
		if blocker.IsPlayer() {
			excludes = append(excludes, blocker.UserId)
		}
		// Visual (owner ruling 2026-09-21): a reader who cannot see does not
		// witness a blocked escape at all, and one who makes out shapes reads
		// neither name.
		uRoom.SendTextVisualHidingNames(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="username">%s</ansi> is blocked from fleeing by <ansi fg="%s">%s</ansi>!`, user.Character.Name, targetTag, blocker.Name), []string{user.Character.Name, blocker.Name}, excludes...)
		return true
	case out.NoExit:
		user.SendText(messaging.CategorySystem, `You can't find an exit!`)
		return true
	}

	user.SendText(messaging.CategoryRoomExit, fmt.Sprintf(`You flee to the <ansi fg="exit">%s</ansi> exit!`, out.ExitName))
	// Visual (owner ruling 2026-09-21): seeing someone break away and which
	// way they went is sight, so a reader who cannot see learns nothing.
	uRoom.SendTextVisualHidingNames(messaging.CategoryRoomExit, fmt.Sprintf(`<ansi fg="username">%s</ansi> flees to the <ansi fg="exit">%s</ansi> exit!`, user.Character.Name, out.ExitName), []string{user.Character.Name}, user.UserId)

	if err := rooms.MoveToRoom(user.UserId, out.ExitRoomId); err == nil {

		for _, instId := range uRoom.GetMobs(rooms.FindCharmed) {
			if mob := mobs.GetInstance(instId); mob != nil {
				if mob.Character.IsCharmed(userId) {
					mob.Command(out.ExitName)
				}
			}
		}

		newRoom := rooms.LoadRoom(out.ExitRoomId)
		usercommands.Look(``, user, newRoom, events.CmdSecretly)

		// Fire the room behavior tree's room_enter event for the destination,
		// exactly as walked movement (go.go) does. Without this, fleeing INTO a
		// room silently skips its on-entry effects — e.g. the newcomer tutorial's
		// final room (6467), reached by fleeing the effigy, never delivered its
		// guide's "talk to me" instruction, stranding the player (2026-07-17
		// playtest). Correct in general: entering a room by any means should
		// trigger its entry hooks.
		behaviortree.TryRoomBehavior(out.ExitRoomId, behaviortree.EventContext{
			EventType: "room_enter",
			UserId:    user.UserId,
			RoomId:    out.ExitRoomId,
		})
	}

	return true
}

// handleMobFlee is the mob twin of handlePlayerFlee: the same
// actions.ResolveFlee, the mob's own room lines, and on success an uncharged
// actions.RelocateMob (a player's flee pays no movement cost either) and the
// mob_flee behaviour event. Returns true when the mob is fleeing and should
// skip combat this round.
func handleMobFlee(mob *mobs.Mob, room *rooms.Room) bool {
	out := actions.ResolveFlee(actions.NewMobActorInRoom(mob, room), room)
	if !out.Fleeing {
		return false
	}
	if !out.Resolved {
		return true
	}

	name := mob.Character.Name
	switch {
	case out.Grappled:
		// The grappler sees the hold working, as at command time.
		room.SendTextVisual(messaging.CategoryGrappleFlow,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to break free but you've got them locked down!`, name))
		return true
	case out.Blocker != nil:
		// Passing the NAME matters: SendTextVisual alone falls back to the
		// tag-based "a figure", uncapitalised at a sentence start.
		room.SendTextVisualHidingNames(messaging.CategoryRoomExit,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to flee but is blocked!`, name),
			[]string{name})
		return true
	case out.NoExit:
		// Cornered: the mob stays in the fight, and the room sees it try.
		room.SendTextVisual(messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> looks around frantically for an escape but finds none!`, name))
		return true
	}

	room.SendTextVisual(messaging.CategoryRoomExit,
		fmt.Sprintf(`<ansi fg="mobname">%s</ansi> flees!`, name))
	if dest := rooms.LoadRoom(out.ExitRoomId); dest != nil {
		actions.RelocateMob(mob, room, out.ExitName, dest, actions.MobIsSneaking(mob))
	}
	behaviortree.TryMobBehavior(mob.InstanceId, behaviortree.EventContext{
		EventType: "mob_flee",
		RoomId:    mob.Character.RoomId,
	})
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

	// Owner fights back if not already in combat. The round claim stops this
	// duplicating the reactive path in CombatPhase_CompanionAssist.go, which
	// fires a round earlier and leaves IsInCombat() still false.
	if !owner.Character.IsInCombat() &&
		owner.Character.TryClaimAssistCommand(util.GetRoundCount()) {
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
			if charmedMob.Character.IsCharmed(defId) && !charmedMob.Character.IsInCombat() {
				comp := defUser.Character.GetCompanionByInstanceId(instanceId)
				if comp != nil && comp.AutoAssist &&
					charmedMob.Character.TryClaimAssistCommand(util.GetRoundCount()) {
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

	// Visual (owner ruling 2026-09-21).
	defRoom.SendTextVisualHidingNames(messaging.CategoryEquipment, fmt.Sprintf(`<ansi fg="214"><ansi fg="202">***</ansi> The <ansi fg="item">%s</ansi> <ansi fg="username">%s</ansi> was carrying breaks! <ansi fg="202">***</ansi></ansi>`, br.BrokenItemName, defUser.Character.Name), []string{defUser.Character.Name}, defUser.UserId)

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
		// Visual (owner ruling 2026-09-21).
		defRoom.SendTextVisualHidingNames(messaging.CategoryEquipment, fmt.Sprintf(`<ansi fg="214"><ansi fg="202">***</ansi> The <ansi fg="item">%s</ansi> <ansi fg="mobname">%s</ansi> was carrying breaks! <ansi fg="202">***</ansi></ansi>`, br.BrokenItemName, defMob.Character.Name), []string{defMob.Character.Name})
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
		sendPlayerConcentrationBroke(defUser, defRoom)
	}
}

// ordinaryMeleeEngagement reports whether the actor is in the plain
// toe-to-toe attack state the two mob-AI passes below were written for.
//
// U12c-2: this was `Aggro.Type == characters.DefaultAttack`, and it is a
// COMPOUND question, not a simple one. DefaultAttack was the odd one out of
// five types, so the test excluded all four others at once: a ranged
// engagement, an unspent ambush opening, a cast, and a flee. Each of those now
// lives on the machine that models it, and Engagement composes them, so the
// question is asked in one place rather than reconstructed at each site.
//
// Getting this wrong is silent: dropping the ranged half alone would let kiting
// archers start running melee target-switch AI, and nothing would fail.
func ordinaryMeleeEngagement(c *characters.Character) bool {
	e := targeting.EngagementOf(c)
	if e.Phase != combatphase.Engaging && e.Phase != combatphase.Engaged {
		return false // Idle, or Disengaging (the old Flee type)
	}
	return !e.Ranged && !e.OpeningUnspent && !e.Casting
}

// handleMobAIDecision processes mob AI decisions (spell casting, special moves, combat commands).
// Returns true if the mob executed an AI action and should skip normal combat.
func handleMobAIDecision(mob *mobs.Mob, c configs.Config) bool {
	// Aggro can be cleared mid-round before this AI pass runs — e.g. a kiting
	// archer whose target left the room (keep_distance repositions it, and the
	// unified combat handler drops a mob's aggro when its target isn't present).
	// The unified dispatch below guards on Aggro != nil; mirror that here so we
	// never deref a nil Aggro (was a server-crashing nil-pointer panic).
	if !mob.Character.IsInCombat() {
		return false
	}
	if !ordinaryMeleeEngagement(&mob.Character) {
		return false
	}

	// Stage 11.5: Caster AI decision - try spell first, then special move
	var chosenMove string
	if util.Rand(100) < mob.ActivityLevel {
		var targetChar *characters.Character
		aiTarget := mob.Character.CurrentCombatTarget()
		if aiTarget.UserId > 0 {
			if u := users.GetByUserId(aiTarget.UserId); u != nil {
				targetChar = u.Character
			}
		} else if aiTarget.MobInstanceId > 0 {
			if tm := mobs.GetInstance(aiTarget.MobInstanceId); tm != nil {
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
	if util.Rand(100) >= 10 || !ordinaryMeleeEngagement(&mob.Character) {
		return false
	}

	combatSkill := mob.Character.GetCombatSkillLevel()
	if combatSkill < 30 {
		return false
	}

	potentialTargets := []int{}
	for _, userId := range mobRoom.GetPlayers() {
		if userId == mob.Character.CurrentCombatTarget().UserId {
			continue
		}
		if u := users.GetByUserId(userId); u != nil {
			if u.Character.Health > 0 && !u.Character.IsHidden() {
				if u.Character.CurrentCombatTarget().MobInstanceId == mob.InstanceId {
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
		// U12c-2: the prevType hoist is gone. ordinaryMeleeEngagement above
		// already refused every non-DefaultAttack engagement, so the type it
		// carried was ALWAYS DefaultAttack and ReasonAttack is exact. Commit
		// re-derives the ranged flavour from the equipped weapon anyway.
		targeting.CommitAfter(&mob.Character,
			state.ActorRef{UserId: newTargetId},
			targeting.ReasonAttack, 1)

		if newTarget := users.GetByUserId(newTargetId); newTarget != nil {
			sendMobShiftsFocus(mob, mobRoom, newTarget)
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
					!memberUser.Character.IsInCombat() {
					memberUser.Command(fmt.Sprintf(`attack #%d`, mob.InstanceId))
				}
			}
		}
	}
}

// surpriseCandidate builds the skullduggery candidate a landed surprise attack
// contributes to the round's attacker contest, or reports false.
//
// U10d gave the ambush its own award beside the weapon one, which under the
// Best-of convention would be a second event for a single resolved action. It
// now competes in the SAME contest: an ambush that rolled better than the blade
// trains skullduggery, otherwise the blade takes it.
//
// Gated on CleanHit, not merely on WasSurpriseAttack. An ambush that was seen
// and answered is not an ambush that worked, and skullduggery here means "the
// approach succeeded", not "the approach was attempted". This is the one place
// the round's candidate set is narrower on a loss than on a win, and it is why
// a fully-defended surprise round still trains the weapon skill at the failure
// fraction rather than training skullduggery.
//
// ⚠️ IT IS ALSO OUT-DRAWN BY CONSTRUCTION, which matters more than the scale
// caveat below and was missing from an earlier draft. A weapon candidate's roll
// is the MAX across that weapon's swings (1 to 4), while this is a single draw.
// At equal scores that order statistic alone gives the weapon roughly 65% at
// two swings and 80% at four. Measured against the scale effects together,
// skullduggery needs to sit about four skill levels above the combat skill to
// reach a coin flip. A dedicated ambusher clears that; a fighter who happened to
// open from stealth does not, which is roughly the intended shape -- but it is a
// property of the arithmetic rather than a decision anyone made.
//
// ⚠️ ITS ROLL IS SYNTHESISED, and it is the only one in the set that is.
// Skullduggery is never rolled during a surprise attack -- crit_damage.go reads
// it as a LEVEL, not a contest -- so there is no roll that already happened to
// carry. characters.CandidateFor rolls dice.RollStat(dexterity +
// skullduggeryLevel*SkillWeight), which is the same SHAPE as an attack roll and
// shares dexterity with the melee weapon skills, but NOT the same scale: a real
// attack score also carries weapon, position, encumbrance and third-party
// modifiers that a bare stat-plus-skill roll does not. So the comparison is
// approximate, and which side it favours depends on how those modifiers happen
// to sit. Recorded as an open item for U10b-1b alongside the melee/channel
// fumble divergence; do not read this function as evidence the two rolls are
// commensurable.
func surpriseCandidate(c *characters.Character, result combat.AttackResult) (progression.Candidate, bool) {
	if c == nil || !result.WasSurpriseAttack || !result.CleanHit {
		return progression.Candidate{}, false
	}
	return c.CandidateFor(string(skills.Skullduggery)), true
}
