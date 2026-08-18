package combat

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/skills"
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

// defenceEffectiveness returns the config multiplier applied to a defence's
// score before it enters a contest.
//
// runBestOfAllDefense reads the same three physical knobs inline; this is the
// same table for the channels that do not run through it. An unrecognised
// defence gets 1.0 and not 0, so a defence added to DefenceSetFor without a knob
// enters the contest at face value instead of silently losing every roll.
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
	switch defenceType {
	case characters.DefenseDodge:
		c.OnSkillUse(string(skills.UnarmedCombat), userId)
		c.OnStatUse("dexterity", userId)
	case characters.DefenseParry:
		c.OnSkillUse(string(skills.WeaponCombat), userId)
		c.OnStatUse("dexterity", userId)
		c.OnStatUse("strength", userId)
	case characters.DefenseBlock:
		c.OnSkillUse(string(skills.WeaponCombat), userId)
		c.OnStatUse("strength", userId)
	case characters.DefenseQuell:
		c.OnSkillUse(string(skills.Spellcasting), userId)
		c.OnStatUse("willpower", userId)
	case characters.DefenseDefy:
		c.OnSkillUse(string(skills.Rhetoric), userId)
		c.OnStatUse("willpower", userId)
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

// ResolveChannelDefence runs ONE opposed contest for a channel that does not go
// through the melee hitroll and returns its structured outcome.
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
// DefenceSetFor sends dodge and block to this function for ChannelRanged and
// ChannelSpellPhysical, and eleven shipped spells declare
// target_defense_type: physical. QuoteDefenseCost retains fractional carry and
// the distinct physical modifiers, so this path must not fall back to the lossy
// integer compatibility price. An earlier comment here asserted the physical
// three never reached this site; that was never true.
func ResolveChannelDefence(channel AttackChannel, attacker, defender *characters.Character) ChannelDefenceResult {
	return resolveChannelDefenceWithRunner(channel, attacker, defender, RunContest)
}

func resolveChannelDefenceWithRunner(channel AttackChannel, attacker, defender *characters.Character, runner defenceContestRunner) ChannelDefenceResult {
	out := ChannelDefenceResult{
		DamageMultiplier: 1.0,
		Cost:             characters.CostCommitResult{Status: characters.CostNoCharge},
	}
	if attacker == nil || defender == nil {
		return out
	}

	defences := DefenceSetFor(channel)
	if len(defences) == 0 {
		// No defence answers this channel. Uncontested is an attack win, which
		// is what a full multiplier says.
		return out
	}

	atkScore := ChannelAttackScore(channel, attacker)

	entries := make([]contest.Entry, 0, len(defences))
	candidates := make([]quotedDefenceCandidate, 0, len(defences))
	for _, d := range defences {
		quote, quoted := defender.QuoteDefenseCost(d)
		includeSkill := !quoted || quote.Affordable()
		entry := contest.Entry{
			Name:  d,
			Score: defender.GetDefenseScoreFor(d, includeSkill) * defenceEffectiveness(d),
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
	out.DamageMultiplier = defenceDamageMultiplier(res)

	out.DefenceType = res.Winner
	out.Cost = commitDefenceWinner(defender, candidates, res)
	for _, candidate := range candidates {
		if candidate.entry.Name == res.Winner {
			AwardDefenceProgression(defender, defender.GetUserId(), res.Winner)
			break
		}
	}

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
