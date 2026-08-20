package combat

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/progression"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// DefenceMitigation maps a defender's NORMALIZED margin onto the fraction of
// damage removed.
//
// Before U6 a defensive win was a clean miss and a spell deflection was a flat
// 0.5, which is two mechanisms answering one question. A bare win now mitigates
// 50%, rising linearly to 100% at ContestCritThreshold. Skill raises the margin,
// so skill raises mitigation continuously rather than in a step.
//
// A defensive CRIT is not this curve: it fully negates and fires the
// counterattack, and is resolved before this is reached.
//
// Applied AFTER item mitigation. There is no double count: a crit bypasses item
// mitigation and never receives a defence multiplier, because an attack crit
// beats a non-crit defence outright.
//
// The 0.5 and the crit threshold are STRUCTURAL, not tunables. 0.5 is the value
// that makes a bare win worth exactly half a swing, and the threshold is the
// point the curve has to meet so a defensive crit's full negation is continuous
// with it rather than a cliff. Moving either independently reintroduces the
// discontinuity this replaces, so neither is a config knob.
func DefenceMitigation(normalizedDefenceMargin float64) float64 {
	if normalizedDefenceMargin <= 0 {
		// Not a defensive win at all. Clamp to the bare floor rather than
		// extrapolating below it: a mis-signed margin must not be able to
		// AMPLIFY damage past 100%.
		return 0.5
	}
	if normalizedDefenceMargin >= ContestCritThreshold {
		return 1.0
	}
	return 0.5 + 0.5*(normalizedDefenceMargin/ContestCritThreshold)
}

// ChannelAttackScore builds the attacker's score for a channel that does not go
// through the melee hitroll.
//
// Note both spell channels share one score. The channel decides what DEFENDS
// against the attack, not what powers it: a physical-flavoured spell is still
// cast with willpower and spellcasting, it is simply dodged rather than quelled.
//
// The physical channels return 0 rather than guessing. Melee and ranged build
// their attack score in calcAttackScore with weapon, reach, position and
// resource terms this cannot see, and a plausible-looking wrong number here
// would be worse than an obviously wrong one.
func ChannelAttackScore(channel AttackChannel, attacker *characters.Character) float64 {
	if attacker == nil {
		return 0
	}
	skillWeight := float64(configs.GetBalanceConfig().SkillWeight)

	switch channel {
	case ChannelSpellMental, ChannelSpellPhysical:
		return float64(attacker.Stats.Willpower.ValueAdj) +
			float64(attacker.GetSkillLevel(skills.Spellcasting))*skillWeight
	case ChannelSocial:
		return float64(attacker.Stats.Charisma.ValueAdj) +
			float64(attacker.GetSkillLevel(skills.Rhetoric))*skillWeight
	default:
		return 0
	}
}

// AttackSide is the attacker's half of a channel contest, made explicit.
//
// Before U6b the seam derived the attack score internally (ChannelAttackScore,
// hardcoded Willpower+Spellcasting / Charisma+Rhetoric), which meant a spell's
// primarystat (U9) could not reach the hit contest and the progression naming
// had to mirror the hardcode. Callers now say exactly what attacks:
//
//	Stat      fully-modified stat VALUE feeding the score
//	StatName  which stat that is — progression events carry it
//	Skill     governing skill — progression, cost and crit-rank input carry it
//	SkillRank RAW rank. It multiplies by SkillWeight for the score, and feeds
//	          CritDamageMultiplier UNWEIGHTED (Assumption 8 — taunt used to
//	          pass the weighted value, a x15.75-vs-x4.6 outlier, corrected).
//	Mult      situational multiplier on the whole score (1.0 default; taunt's
//	          conviction-depletion factor and Task 17's shared modifiers land
//	          here)
type AttackSide struct {
	Stat      int
	StatName  string
	Skill     skills.SkillTag
	SkillRank int
	Mult      float64
}

// score is the attack score the side enters the contest with. Mult 0 is the
// zero value, not an annihilator: it reads as "unset", 1.0.
func (s AttackSide) score() float64 {
	m := s.Mult
	if m == 0 {
		m = 1.0
	}
	return (float64(s.Stat) + float64(s.SkillRank)*float64(configs.GetBalanceConfig().SkillWeight)) * m
}

// defenceEffectiveness returns the config multiplier applied to a defence's
// score before it enters a contest.
//
// Both melee and channel defence candidate builders use this centralized
// mapping. An unrecognised defence gets 1.0 and not 0, so a defence added to
// DefenceSetFor without a knob enters the contest at face value instead of
// silently losing every roll.
func defenceEffectiveness(defenceType string) float64 {
	bal := configs.GetBalanceConfig()
	switch defenceType {
	case characters.DefenseDodge:
		return float64(bal.DodgeEffectiveness)
	case characters.DefenseParry:
		return float64(bal.ParryEffectiveness)
	case characters.DefenseBlock:
		return float64(bal.BlockEffectiveness)
	case characters.DefenseQuell:
		return float64(bal.QuellEffectiveness)
	case characters.DefenseDefy:
		return float64(bal.DefyEffectiveness)
	}
	return 1.0
}

// DefenceSkillAndStat is THE mapping from a defence to what it trains, in one
// place, for all five defences. AwardDefenceProgression and the crit/fumble
// bonus tier both read it, so the five rows exist once.
//
// Note the asymmetry with AwardDefenceProgression: parry deliberately awards
// BOTH dexterity and strength there, while this returns the single stat the
// bonus tier wants. That is intentional. Do not "simplify" the two into one by
// dropping parry's second stat.
//
// An unrecognised defence returns two empty strings rather than guessing.
// Passing an empty skill on is not inert: CheckSkillProgression("") takes the
// roll and a success banners no skill at all.
func DefenceSkillAndStat(defenceType string) (skill, stat string) {
	switch defenceType {
	case characters.DefenseDodge:
		return string(skills.UnarmedCombat), "dexterity"
	case characters.DefenseParry:
		return string(skills.WeaponCombat), "dexterity"
	case characters.DefenseBlock:
		return string(skills.WeaponCombat), "strength"
	case characters.DefenseQuell:
		return string(skills.Spellcasting), "willpower"
	case characters.DefenseDefy:
		return string(skills.Rhetoric), "willpower"
	}
	return "", ""
}

// AwardDefenceProgression fires the skill and stat progression a defence earns
// the character who mounted it. It is THE mapping, in one place, for all five
// defences.
//
// It exists because U6 Task 12 deleted avoidance.go, which awarded the
// non-physical rows itself while hooks.processDefenderProgression awarded the
// physical three from a switch with no default arm. Deleting the file without
// this would have silently deleted defender progression on both non-physical
// channels: no compile error, no failing test, just a stat that stops growing.
//
// Note the change of stat on the spell row. TrySpellDeflection awarded
// PERCEPTION because perception was what it contested. Quell contests
// WILLPOWER, so willpower is what it trains. Losing perception as a spell-defence
// stat is the intended outcome of the unification, not a cost of it.
//
// An unrecognised defence awards nothing rather than guessing a skill. An empty
// skill name is not inert: TrackSkillUse("") and CheckSkillProgression("", ...)
// take the roll and a success sends a levelup banner naming no skill at all.
func AwardDefenceProgression(c *characters.Character, userId int, defenceType string) {
	if c == nil {
		return
	}
	skill, stat := DefenceSkillAndStat(defenceType)
	if skill == "" {
		return // unrecognised defence awards nothing rather than guessing
	}
	c.OnSkillUse(skill, userId)
	c.OnStatUse(stat, userId)
	// Parry is the one two-stat defence: it takes both the timing and the
	// force to turn a blade. Preserved from pre-U9 behaviour verbatim.
	if defenceType == characters.DefenseParry {
		c.OnStatUse("strength", userId)
	}
}

// ChannelDefenceResult is the canonical mechanical outcome of one channel
// defence. Damage callers consume DamageMultiplier; later narration consumes
// the defence identity, normalized opposed margin, and crit flag.
type ChannelDefenceResult struct {
	DamageMultiplier        float64
	DefenceType             string
	DefenseRollZScore       float64
	NormalizedDefenceMargin float64
	Defended                bool
	DefensiveCrit           bool
	Cost                    characters.CostCommitResult

	// U6b: the attacker's half of the same contest. Crit is margin-derived
	// against CritBarFor and NEVER set on a Floored outcome; Fumble is
	// self-relative (AttackRoll.ZScore <= -DefenseCritBar()) and callers must
	// resolve it BEFORE success — a fumbled attack aborts even when the roll
	// won (Assumption 7, uniform across channels; it caps hit at the ceiling
	// minus the fumble rate, which is the pre-U6b spell/taunt behaviour made
	// universal and documented).
	AttackerCrit   bool
	AttackerFumble bool

	// AttackRollZScore is the attacker's own roll quality from the ONE contest,
	// surfaced for analytics (combat.RecordSpell) so callers that no longer run
	// their own contest can still record the roll that decided the cast. Zero
	// when the contest never ran (nil participants or an empty defence set).
	AttackRollZScore float64
}

// ChannelDefenceIdentities carries actor-aware, display-ready identities into
// the shared defence renderer. Callers supply username/mobname ANSI tags (and
// duplicate-mob suffixes) before neutral message placeholders are replaced.
type ChannelDefenceIdentities struct {
	Attacker string
	Defender string
}

// RenderChannelDefenceMessages renders the canonical channel outcome without
// rolling or deriving a second result. Attack wins return an empty triad.
func RenderChannelDefenceMessages(out ChannelDefenceResult, identities ChannelDefenceIdentities, attack string, indexOverride ...int) items.DefenseMessageTriad {
	if !out.Defended {
		return items.DefenseMessageTriad{}
	}
	return items.RenderDefenseMessage(items.DefenseType(out.DefenceType), out.DefensiveCrit, out.NormalizedDefenceMargin, map[items.TokenName]string{
		items.TokenAttacker: identities.Attacker,
		items.TokenDefender: identities.Defender,
		items.TokenAttack:   attack,
		items.TokenWeapon:   attack,
	}, indexOverride...)
}

// ResolveChannelAttack is THE channel resolution entry point (U6b Task 3): it
// runs ONE opposed contest for a channel that does not go through the melee
// hitroll and returns its structured outcome, with the attacker's half made
// explicit by the caller-supplied AttackSide.
//
// It replaces TrySpellDeflection and TryStoicResolve, which each ran a SECOND
// independent contest on top of the channel's primary roll, on different stats,
// and returned a flat configured multiplier. Same contest, same floor, same
// mitigation curve as the physical pipeline; only the defence set differs. That
// is what let both be deleted rather than ported.
//
// The selected defence is CHARGED and PROGRESSED whether or not it wins,
// matching both the melee path and the two deleted functions (which awarded
// progression unconditionally). Every eligible defence is quoted before the
// contest and only Result.Winner's paired quote is committed.
//
// THE PHYSICAL DEFENCES DO ARRIVE HERE, so the FLOAT entry point is mandatory.
// DefenceEntriesFor sends dodge (and, for shielded defenders, block) to this
// function for ChannelRanged and ChannelSpellPhysical, and eleven shipped
// spells declare
// target_defense_type: physical. QuoteDefenseCost retains fractional carry and
// the distinct physical modifiers, so this path must not fall back to the lossy
// integer compatibility price. An earlier comment here asserted the physical
// three never reached this site; that was never true.
func ResolveChannelAttack(channel AttackChannel, side AttackSide, attacker, defender *characters.Character) ChannelDefenceResult {
	return resolveChannelAttackWithRunner(channel, side, attacker, defender, channelAttackContestRunner)
}

// channelAttackContestRunner is the contest core behind ResolveChannelAttack.
// Production never repoints it; SetChannelAttackContestRunnerForTest swaps it
// so out-of-package tests (internal/hooks) can drive the FULL seam — cost
// admission, progression, the bonus tier — against a deterministic contest
// outcome instead of stubbing the seam away and losing those side effects.
var channelAttackContestRunner defenceContestRunner = RunContest

// SetChannelAttackContestRunnerForTest swaps the contest runner behind
// ResolveChannelAttack and returns a restore func for t.Cleanup.
func SetChannelAttackContestRunnerForTest(runner func(float64, []contest.Entry) contest.Result) (restore func()) {
	prev := channelAttackContestRunner
	channelAttackContestRunner = runner
	return func() { channelAttackContestRunner = prev }
}

// ResolveChannelDefence remains for the taunt/spell callers until Tasks 4-5
// migrate them onto ResolveChannelAttack, building the legacy default side
// from the old ChannelAttackScore semantics. DELETED in Task 5 once the last
// caller moves; ChannelAttackScore goes with it.
func ResolveChannelDefence(channel AttackChannel, attacker, defender *characters.Character) ChannelDefenceResult {
	return resolveChannelDefenceWithRunner(channel, attacker, defender, RunContest)
}

// resolveChannelDefenceWithRunner is the legacy shim behind
// ResolveChannelDefence: it builds the default AttackSide and delegates.
// Deleted with the wrapper in Task 5.
func resolveChannelDefenceWithRunner(channel AttackChannel, attacker, defender *characters.Character, runner defenceContestRunner) ChannelDefenceResult {
	return resolveChannelAttackWithRunner(channel, legacyChannelAttackSide(channel, attacker), attacker, defender, runner)
}

// legacyChannelAttackSide reproduces ChannelAttackScore's hardcoded pairs as
// an explicit AttackSide, so the legacy wrapper's callers keep the exact score
// and bar inputs they had: side.score() equals ChannelAttackScore for every
// channel (the physical channels score 0 there and the zero side scores 0
// here), and SkillRank is the same real rank the U9 bonus tier used to read
// through its per-channel hardcode (deleted this task). Stays in lockstep
// with ChannelAttackScore until Task 5 deletes both.
func legacyChannelAttackSide(channel AttackChannel, attacker *characters.Character) AttackSide {
	if attacker == nil {
		return AttackSide{Mult: 1.0}
	}
	switch channel {
	case ChannelSpellMental, ChannelSpellPhysical:
		return AttackSide{
			Stat:      attacker.Stats.Willpower.ValueAdj,
			StatName:  "willpower",
			Skill:     skills.Spellcasting,
			SkillRank: attacker.GetSkillLevel(skills.Spellcasting),
			Mult:      1.0,
		}
	case ChannelSocial:
		return AttackSide{
			Stat:      attacker.Stats.Charisma.ValueAdj,
			StatName:  "charisma",
			Skill:     skills.Rhetoric,
			SkillRank: attacker.GetSkillLevel(skills.Rhetoric),
			Mult:      1.0,
		}
	}
	return AttackSide{Mult: 1.0}
}

func resolveChannelAttackWithRunner(channel AttackChannel, side AttackSide, attacker, defender *characters.Character, runner defenceContestRunner) ChannelDefenceResult {
	out := ChannelDefenceResult{
		DamageMultiplier: 1.0,
		Cost:             characters.CostCommitResult{Status: characters.CostNoCharge},
	}
	if attacker == nil || defender == nil {
		return out
	}

	// U6b Task 2: the set comes from the equipment-gated name builder, not the
	// bare channel table — a shieldless bare-handed defender no longer rolls
	// block against a bolt or a physical spell.
	defences := DefenceEntriesFor(channel, defender, DefenceEntryOpts{})
	if len(defences) == 0 {
		// No defence answers this channel. Uncontested is an attack win, which
		// is what a full multiplier says.
		return out
	}

	atkScore := side.score()

	bal := configs.GetBalanceConfig()
	proneDefender := defender.IsProne() || defender.IsSupine()

	entries := make([]contest.Entry, 0, len(defences))
	candidates := make([]quotedDefenceCandidate, 0, len(defences))
	for _, d := range defences {
		quote, quoted := defender.QuoteDefenseCost(d)
		includeSkill := !quoted || quote.Affordable()
		score := defender.GetDefenseScoreFor(d, includeSkill) * defenceEffectiveness(d)

		// U6b Task 2: prone defence penalties now apply on every channel —
		// before this a prone defender dodged a bolt at full score while
		// dodging a sword at penalty. Same knobs as melee's candidate loop;
		// quell and defy have no prone knobs and are deliberately unpenalised
		// (matching melee's disclosed gap in combat_helpers.go).
		if proneDefender {
			switch d {
			case characters.DefenseDodge:
				score *= float64(bal.ProneDodgePenalty)
			case characters.DefenseParry:
				score *= float64(bal.ProneParryPenalty)
			case characters.DefenseBlock:
				score *= float64(bal.ProneBlockPenalty)
			}
		}

		entry := contest.Entry{
			Name:  d,
			Score: score,
		}
		entries = append(entries, entry)
		candidates = append(candidates, quotedDefenceCandidate{
			entry:  entry,
			quote:  quote,
			quoted: quoted,
		})
	}

	res := runner(atkScore, entries)
	if !res.Contested {
		return out
	}
	out.AttackRollZScore = res.AttackRoll.ZScore
	out.DamageMultiplier = defenceDamageMultiplier(res)

	// U6b Task 3: the attacker's verdicts, derived ONCE from this contest.
	// The bar is the CHANNEL's skill pair — the attack's governing rank
	// against the WINNING defence's governing rank. A floored outcome carries
	// the +-1 sentinel margin and cannot be a crit (the same rule
	// applyCritFloors declares in melee). Fumble is self-relative and callers
	// resolve it BEFORE success — a fumbled attack aborts even a winning roll.
	bar := CritBarFor(side.SkillRank, defenderRankOf(defender, res.Winner))
	out.AttackerCrit = !res.Floored && AttackContestCritAt(res.Margin, res.AttackRoll, bar)
	out.AttackerFumble = res.AttackRoll.ZScore <= -DefenseCritBar()

	out.DefenceType = res.Winner
	out.Cost = commitDefenceWinner(defender, candidates, res)
	// U9: the ordinary defence award is unchanged in WHEN it fires -- whenever
	// the contest ran, win or lose, which is what this path has always done and
	// is deliberately different from melee's defence-used gate. That divergence
	// is recorded in the firing audit and is U10b's to reconcile.
	//
	// What is new is the bonus tier: a defensive crit or fumble now pays the
	// defender, and the attacker observes it.
	for _, candidate := range candidates {
		if candidate.entry.Name == res.Winner {
			AwardDefenceProgression(defender, defender.GetUserId(), res.Winner)
			break
		}
	}

	awardChannelDefenceBonus(channel, side, attacker, defender, res, out.AttackerCrit, out.AttackerFumble)

	// A floor changes the outcome without changing the underlying rolls. Keep
	// the winner and cost, but expose zero statistical sentinels so later prose
	// cannot mistake the original roll for the strength of a floor-granted save.
	if !res.Floored {
		out.DefenseRollZScore = res.DefenseRoll.ZScore
	}
	if res.Success {
		return out
	}

	out.Defended = true
	if res.Floored {
		return out
	}
	out.DefensiveCrit = out.DamageMultiplier == 0
	if res.DefenseRoll.StdDev > 0 {
		out.NormalizedDefenceMargin = -res.Margin / (res.DefenseRoll.StdDev * math.Sqrt2)
	}
	return out
}

// defenceDamageMultiplier converts a finished opposed contest into the
// attacker's damage multiplier: 1.0 when the attack won, 0.0 on a defensive
// crit, exactly 0.5 on a floored save, and 0.0-0.5 along the DefenceMitigation
// curve for a rolled defensive win.
//
// This is the exact tail of ResolveChannelDefence's derivation, extracted so
// skill_moves.go's maneuvers (bash/trip/kick) can share it rather than
// hand-copy the sign negation, the floored sentinel, and the sqrt(2)
// normaliser below -- context.md records the sign and .Margin traps as
// having NEARLY shipped once already, caught only in review, which is
// reason enough to keep them in exactly one place.
func defenceDamageMultiplier(res contest.Result) float64 {
	if res.Success {
		return 1.0
	}

	// A FLOORED save is an outcome the contest did not produce. RunWithFloors
	// stamps a +-1 sentinel margin in RAW SCORE UNITS, not sigma, so feeding it
	// to the curve below would read as a near-zero margin by accident rather than
	// by rule. It takes the bare win and cannot crit -- the same rule
	// applyCritFloors applies in melee.
	if res.Floored {
		return 1.0 - DefenceMitigation(0)
	}

	// SIGN. contest.Result.Margin is ATTACK-positive and everything from here
	// down is the DEFENDER's, so it is negated exactly here and nowhere else.
	// Do NOT copy the negation from normalizedDefenseMargin: that reads
	// bestDefenseResult.margin, which runBestOfAllDefense has already built
	// DEFENCE-positive. The two conventions are opposites, mixing them compiles
	// cleanly, and the result is a crit awarded to the losing side.
	//
	// It is Result.Margin and not res.DefenseRoll.Margin. contest.Run rolls via
	// dice.Roll, which never populates a RollResult's Margin field, so that read
	// is a silent constant zero and nothing ever fully negates again.
	if DefenseContestCrit(-res.Margin, res.DefenseRoll) {
		return 0.0
	}

	stdDev := res.DefenseRoll.StdDev
	if stdDev <= 0 {
		// No scale to normalise against. Bare win rather than a divide by zero.
		return 1.0 - DefenceMitigation(0)
	}

	// NORMALISER. Both sides rolled with the attacker's stdDev, so their
	// difference has standard deviation stdDev*sqrt(2). Dividing by stdDev alone
	// inflates the margin by about 41% and silently over-mitigates everything.
	defMargin := -res.Margin / (stdDev * math.Sqrt2)

	return 1.0 - DefenceMitigation(defMargin)
}

// defenderRankOf resolves the WINNING defence's governing skill rank — the
// defender half of CritBarFor's pair. Uncontested/static outcomes (an empty or
// unrecognised winner) use rank 0 rather than guessing a skill.
func defenderRankOf(defender *characters.Character, winner string) int {
	skillName, _ := DefenceSkillAndStat(winner)
	if skillName == "" || defender == nil {
		return 0
	}
	return defender.GetSkillLevel(skills.SkillTag(skillName))
}

// awardChannelDefenceBonus pays the crit/fumble tier for a channel contest.
//
// The ORDINARY events are left to AwardDefenceProgression and to the attacker's
// own call site, so Outcome carries only the skill and stat names the bonus
// cells need -- populating the ordinary fields here would double-award.
//
// The attacker's crit and fumble verdicts are CONSUMED, not re-derived: the
// seam already decided them once against CritBarFor's pair bar, and a second
// derivation here (the pre-U6b const-bar AttackContestCrit call) would let a
// skill-advantaged attacker crit at the floored bar for narration and damage
// while the progression bonus still demanded 2.0 — two verdicts for one
// contest. The attacker's skill and stat likewise come FROM the AttackSide the
// caller passed, not a per-channel hardcode.
func awardChannelDefenceBonus(channel AttackChannel, side AttackSide, attacker, defender *characters.Character, res contest.Result, attackCrit, attackFumble bool) {
	if !res.Contested || res.Floored {
		return
	}

	// The DEFENDER's crit is still decided here, by NORMALIZED MARGIN against
	// the constant defender bar (see DefenseCritBar for why it does not move
	// with the skill pair).
	//
	// Note the sign: Result.Margin is ATTACK-positive, so the defence side
	// negates it, exactly as defenceDamageMultiplier does at
	// defence_multiplier.go:307.
	defenceCrit := DefenseContestCrit(-res.Margin, res.DefenseRoll)

	// Fumble stays self-relative: it is a property of one bad roll, not of the
	// gap between two. ContestCritThreshold is the same magnitude in both
	// directions.
	defenceFumble := res.DefenseRoll.ZScore <= -ContestCritThreshold

	exceptional := progression.Classify(attackCrit, defenceCrit, attackFumble, defenceFumble)
	if exceptional == progression.ExcNone {
		return
	}

	atkSkill, atkStat := string(side.Skill), side.StatName
	defSkill, defStat := DefenceSkillAndStat(res.Winner)

	out := progression.Outcome{
		AttackerSkill: atkSkill,
		AttackerStat:  atkStat,
		DefenderSkill: defSkill,
		DefenderStat:  defStat,
		ToughenStat:   characters.ToughenStatFor(channelDamageChannel(channel)),
		Exceptional:   exceptional,
	}

	bal := configs.GetBalanceConfig()
	bonuses := progression.Bonuses{
		Doing:     float64(bal.CritProgressionBonus),
		Observing: float64(bal.ObservedCritProgressionBonus),
	}

	// BonusEvents, not EventsForContest: the ordinary events on this path are
	// already awarded by AwardDefenceProgression above and by the attacker's
	// own call site, so asking for them here would double-award.
	evs := progression.BonusEvents(out, bonuses)

	round := util.GetRoundCount()
	attacker.ApplyProgression(evs, progression.SideAttacker, attacker.GetUserId(), round)
	defender.ApplyProgression(evs, progression.SideDefender, defender.GetUserId(), round)
}

// channelDamageChannel maps an AttackChannel onto the "physical"/"magical"/
// "conviction" damage-channel string ToughenStatFor expects.
//
// Both spell channels answer "magical" here, NOT "physical" for
// ChannelSpellPhysical. TargetDefenseType: physical only changes which
// defence answers the spell (dodge/block instead of quell); the damage
// itself is still cast off willpower and always goes through
// combat.ChannelMagical in calcSpellDamageForCharacter
// (internal/hooks/combat_shared_helpers.go). Mapping ChannelSpellPhysical to
// "physical" here would toughen the wrong stat (vitality instead of
// willpower) on a defensive crit.
//
// ChannelMelee and ChannelRanged both answer "physical": U6b routes the
// special-attack and ranged resolutions through this seam, and a "" fallthrough
// would make ToughenStatFor("") silently toughen the wrong stat on a bash or
// bolt crit.
func channelDamageChannel(channel AttackChannel) string {
	switch channel {
	case ChannelMelee, ChannelRanged:
		return "physical"
	case ChannelSpellPhysical, ChannelSpellMental:
		return "magical"
	case ChannelSocial:
		return "conviction"
	default:
		return ""
	}
}
