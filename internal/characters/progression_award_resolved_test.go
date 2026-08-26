package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/progression"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// The three assertions the U10b-1 firing convention rests on, in one test: a
// win advances the skill, a loss advances it STRICTLY LESS, and a two-candidate
// call advances ONLY the Best-of winner.
//
// weapon-combat is the probe skill because it is the one that makes the win
// half EXACT rather than statistical. Under pinCertainStatProgressionForTest,
// BaseProgressionChance is 1.0, so at a fresh character's rank 1 the curve
// yields exp(-3.0/50) = 0.9418, and weapon-combat's 1.27 progression multiplier
// pushes that past the 1.0 clamp -- a win is CERTAIN. At the 0.35 failure
// fraction the same product is 0.42, which is not. The two preconditions below
// assert exactly that, so if a default ever moves the test fails loudly on the
// precondition instead of flaking on the counts.
//
// The loss half cannot be made exact and that is inherent, not laziness: a
// fraction strictly between "never" and "always" IS a probability. Only
// frac == 0 or a frac large enough to re-clamp at 1.0 would be deterministic,
// and both are the degenerate cases the shipped 0.35 is not. Over 200 fresh
// characters at p = 0.42, a false failure needs all 200 to advance, which is
// about 1e-75.
func TestAwardResolved_FiresOnceForTheBestOfWinner(t *testing.T) {
	pinCertainStatProgressionForTest(t)

	const winner = "weapon-combat"
	const loser = "search"
	const trials = 200

	frac := float64(configs.GetBalanceConfig().ProgressionFailureFraction)
	if frac <= 0 || frac >= 1.0 {
		t.Fatalf("ProgressionFailureFraction is %v; this test needs a fraction strictly between 0 and 1 to have anything to measure", frac)
	}

	probe := newProgressionTestCharacter(t)
	if got := probe.ProgressionChanceForSkill(winner, 1.0); got < 1.0 {
		t.Fatalf("precondition: a winning %s roll has chance %v, not the pinned certainty of 1.0; the exact assertions below would be statistical", winner, got)
	}
	if got := probe.ProgressionChanceForSkill(winner, frac); got >= 1.0 {
		t.Fatalf("precondition: a LOST %s roll also has chance %v; at that fraction a loss is as certain as a win and 'strictly less' can never hold", winner, got)
	}

	// 1. A win advances the skill. Exact: every one of them, or the path is dead.
	winAdvanced := 0
	for i := 0; i < trials; i++ {
		c := newProgressionTestCharacter(t)
		before := c.Skills[winner]
		c.AwardResolved(0, true, progression.Candidate{Skill: winner, Roll: 100, Level: before})
		if c.Skills[winner] > before {
			winAdvanced++
		}
	}
	if winAdvanced != trials {
		t.Fatalf("a WON action advanced %s on %d of %d fresh characters, want all %d; the chance is pinned to 1.0 so anything less means the event never fired", winner, winAdvanced, trials, trials)
	}

	// 2. A loss advances STRICTLY LESS. The non-zero guard above is what stops
	// an implementation that advances nothing at all from passing this half.
	lossAdvanced := 0
	for i := 0; i < trials; i++ {
		c := newProgressionTestCharacter(t)
		before := c.Skills[winner]
		c.AwardResolved(0, false, progression.Candidate{Skill: winner, Roll: 100, Level: before})
		if c.Skills[winner] > before {
			lossAdvanced++
		}
	}
	if lossAdvanced >= winAdvanced {
		t.Errorf("a LOST action advanced %s on %d of %d characters against a win's %d; a loss must be worth strictly less", winner, lossAdvanced, trials, winAdvanced)
	}

	// 3. Two candidates, ONE event, for the Best-of winner only. Asserted on
	// the LOSER as well as the winner: "the winner advanced" alone would pass
	// against an implementation that fires an event for every candidate.
	c := newProgressionTestCharacter(t)
	beforeLoser := c.Skills[loser]
	c.AwardResolved(0, true,
		progression.Candidate{Skill: loser, Roll: 1, Level: c.Skills[loser]},
		progression.Candidate{Skill: winner, Roll: 100, Level: c.Skills[winner]},
	)
	if got := c.GetSkillUseCount(winner); got != 1 {
		t.Errorf("the Best-of winner %s tracked %d uses, want exactly 1", winner, got)
	}
	if got := c.GetSkillUseCount(loser); got != 0 {
		t.Errorf("the Best-of LOSER %s tracked %d uses; a resolved action produces ONE event, for the winner only", loser, got)
	}
	if got := c.Skills[loser]; got != beforeLoser {
		t.Errorf("the Best-of LOSER %s advanced from %d to %d; it lost its roll and must not progress", loser, beforeLoser, got)
	}
}

// won == false must become Defended: TRUE, so the ACTOR is the side that gets
// the scaled consolation award.
//
// Inverting this would be silent and total, and no count-based test would see
// it: an inverted AwardResolved marks the DEFENDER side, which ApplyProgression
// then never applies for SideAttacker, so the actor would get a full weight,
// unlost event on a loss. The SkillUsed quest event is the exact observable,
// because OnSkillUseScaled gates it on the loss and nothing else.
func TestAwardResolved_ALossIsTheActorsLoss(t *testing.T) {
	pinConfigForTest(t)
	events.DrainQueuedSkillUsedForTest(0)

	lost := newProgressionTestCharacter(t)
	lost.AwardResolved(41, false, progression.Candidate{Skill: "spellcasting", Roll: 100})
	if got := events.DrainQueuedSkillUsedForTest(41); len(got) != 0 {
		t.Errorf("a LOST action emitted %d SkillUsed events, want 0; Defended must be !won so the actor is the scaled, Lost side", len(got))
	}

	won := newProgressionTestCharacter(t)
	won.AwardResolved(42, true, progression.Candidate{Skill: "spellcasting", Roll: 100})
	if got := events.DrainQueuedSkillUsedForTest(42); len(got) != 1 {
		t.Errorf("a WON action emitted %d SkillUsed events, want exactly 1", len(got))
	}
}

// The two inert entries: a nil receiver must not panic, and nothing to award
// must award nothing.
func TestAwardResolved_NilReceiverAndNothingToAward(t *testing.T) {
	pinCertainStatProgressionForTest(t)

	var nilChar *Character
	nilChar.AwardResolved(0, true, progression.Candidate{Skill: "weapon-combat", Roll: 100})

	// No candidates at all.
	none := newProgressionTestCharacter(t)
	none.AwardResolved(0, true)
	if got := none.GetSkillUseCount("weapon-combat"); got != 0 {
		t.Errorf("a call with no candidates tracked %d uses of weapon-combat, want 0", got)
	}

	// One candidate that names neither a skill nor a stat. BestOf reports false
	// for it, and firing it anyway would burn a roll and banner no skill.
	inert := newProgressionTestCharacter(t)
	inert.AwardResolved(0, true, progression.Candidate{Roll: 100})
	if got := inert.GetSkillUseCount(""); got != 0 {
		t.Errorf("an award-nothing candidate tracked %d uses under the empty skill name, want 0", got)
	}
}

// CandidateFor must roll on the SKILL'S PRIMARY STAT, at stat + level*SkillWeight.
//
// Asserted by separation rather than by re-deriving the formula: charisma is
// driven far above dexterity, so a rhetoric candidate (primary charisma) must
// out-roll a skullduggery one (primary dexterity) by a margin no amount of
// dice spread can close. A CandidateFor that rolled a fixed stat, no stat, or
// the wrong one collapses that gap.
func TestCandidateFor_RollsTheSkillsPrimaryStat(t *testing.T) {
	pinConfigForTest(t)

	c := newProgressionTestCharacter(t)
	c.Stats.Charisma.Base = 2000
	c.Stats.Charisma.Recalculate()
	c.Stats.Dexterity.Base = 10
	c.Stats.Dexterity.Recalculate()

	if skills.GetSkillPrimaryStat("rhetoric") != "charisma" || skills.GetSkillPrimaryStat("skullduggery") != "dexterity" {
		t.Fatal("precondition: rhetoric/skullduggery no longer key on charisma/dexterity; pick two other skills whose primaries diverge")
	}

	rhet := c.CandidateFor("rhetoric")
	if rhet.Skill != "rhetoric" {
		t.Errorf("CandidateFor named skill %q, want rhetoric", rhet.Skill)
	}
	// Empty Stat means "the skill's primary", which is what every current
	// caller wants. A populated Stat here would ALSO make ApplyProgression pay
	// a second, redundant stat roll.
	if rhet.Stat != "" {
		t.Errorf("CandidateFor set Stat to %q; empty means the skill's primary and that is the contract", rhet.Stat)
	}
	if want := c.GetSkillLevel(skills.SkillTag("rhetoric")); rhet.Level != want {
		t.Errorf("CandidateFor set Level %d, want the skill level %d", rhet.Level, want)
	}

	sku := c.CandidateFor("skullduggery")
	// charisma 2000 against dexterity 10: the spread is proportional to the
	// mean, so even a 6-sigma low charisma roll clears a 6-sigma high dexterity
	// one by an enormous margin.
	if rhet.Roll <= sku.Roll {
		t.Errorf("rhetoric rolled %v against skullduggery's %v on a character with charisma 2000 and dexterity 10; CandidateFor is not rolling the skill's primary stat", rhet.Roll, sku.Roll)
	}
	if rhet.Roll < 1000 {
		t.Errorf("a rhetoric candidate on a charisma-2000 character rolled %v; CandidateFor is not reading the stat value at all", rhet.Roll)
	}
	if sku.Roll > 500 {
		t.Errorf("a skullduggery candidate on a dexterity-10 character rolled %v; CandidateFor is rolling something other than dexterity", sku.Roll)
	}
}

// An unknown skill has no primary stat, so there is no stat value to roll. It
// must come back INERT rather than as a candidate rolling GetStatValue("") == 0
// while still naming a skill CheckSkillProgression would banner raw.
func TestCandidateFor_UnknownSkillIsInert(t *testing.T) {
	pinCertainStatProgressionForTest(t)

	c := newProgressionTestCharacter(t)
	cand := c.CandidateFor("not-a-real-skill")
	if cand.Skill != "" || cand.Stat != "" || cand.Roll != 0 || cand.Level != 0 {
		t.Errorf("CandidateFor on an unknown skill returned %+v, want the zero Candidate", cand)
	}
	if _, ok := progression.BestOf([]progression.Candidate{cand}); ok {
		t.Error("BestOf accepted the unknown-skill candidate; it awards nothing and must report false")
	}

	c.AwardResolved(0, true, cand)
	if got := c.GetSkillUseCount("not-a-real-skill"); got != 0 {
		t.Errorf("an unknown-skill candidate tracked %d uses, want 0", got)
	}

	// And it must LOSE to a real candidate rather than shadow it.
	known := c.CandidateFor("weapon-combat")
	c.AwardResolved(0, true, cand, known)
	if got := c.GetSkillUseCount("weapon-combat"); got != 1 {
		t.Errorf("weapon-combat tracked %d uses beside an inert candidate, want 1", got)
	}
}

// CandidateFor and AwardResolved must compose: the standard helper's output is
// what production sites will hand to the entry point, so the pair is pinned
// end to end and not only in isolation.
func TestCandidateFor_FeedsAwardResolved(t *testing.T) {
	pinCertainStatProgressionForTest(t)

	c := newProgressionTestCharacter(t)
	before := c.Skills["weapon-combat"]
	c.AwardResolved(0, true, c.CandidateFor("weapon-combat"))
	if got := c.Skills["weapon-combat"]; got <= before {
		t.Errorf("weapon-combat stayed at %d through CandidateFor -> AwardResolved, want an advance; the chance is pinned to 1.0", got)
	}
	if got := c.GetSkillUseCount("weapon-combat"); got != 1 {
		t.Errorf("weapon-combat tracked %d uses, want exactly 1", got)
	}
}
