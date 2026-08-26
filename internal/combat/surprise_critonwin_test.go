package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// attackWinBest is a settled ATTACK win below the crit bar, so only critOnWin
// can make it crit.
//
// hitRoll.StdDev is set deliberately. defenceWinBest leaves it at zero, which
// makes normalizedAttackMargin return ok == false and sends the crit check down
// the legacy self-relative fallback rather than the margin path production
// actually uses. With StdDev == defStdDev the normalised attack margin is
// -(-15*sqrt2) / (15*sqrt2) == 1.0 -- a real, margin-derived z, comfortably
// under the 2.0 bar. So the fixture is sub-crit on the PATH IT CLAIMS TO BE ON,
// and nothing but critOnWin can promote it.
func attackWinBest() bestDefenseResult {
	best := defenceWinBest(-15*math.Sqrt2, 15)
	best.hitRoll.StdDev = 15
	return best
}

func TestCritOnWin_UpgradesWinButNeverRescuesALoss(t *testing.T) {
	src, tgt := defenceFixture(1000)

	t.Run("clean attack win becomes a crit", func(t *testing.T) {
		res := resolveDefenseOutcomeCore(&AttackResult{}, attackWinBest(), src, tgt, 2.0, false, false, true)
		if !res.hit || res.defended {
			t.Fatalf("precondition: expected a clean attack win")
		}
		if !res.crit {
			t.Fatalf("critOnWin must upgrade a won contest")
		}
	})

	t.Run("defence win stays a defence win", func(t *testing.T) {
		best := defenceWinBest(15*math.Sqrt2, 15) // positive == defence won
		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false, true)
		if res.crit {
			t.Fatalf("critOnWin must never rescue a lost contest")
		}
	})

	t.Run("critOnWin false is unchanged behaviour", func(t *testing.T) {
		res := resolveDefenseOutcomeCore(&AttackResult{}, attackWinBest(), src, tgt, 2.0, false, false, false)
		if res.crit {
			t.Fatalf("an ordinary win must not crit merely from winning")
		}
	})

	t.Run("a FLOORED win must not crit", func(t *testing.T) {
		best := attackWinBest()
		best.floored = true
		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false, true)
		if res.crit {
			t.Fatalf("a sentinel margin must never be promoted to a crit")
		}
	})

	// The guard's `best.margin <= 0` term is the only one that is not implied by
	// the others, and this is the case that separates it. A defence FUMBLE exits
	// the inner resolver early, BEFORE attackWon is ever computed, and hands back
	// hit == true / defended == false / fumble == false on a swing the attack
	// LOST on margin. Without the margin term the guard would fire here and turn
	// the defender's stumble into an opening-strike assassination.
	t.Run("a defence FUMBLE the attack lost on margin must not crit", func(t *testing.T) {
		best := defenceWinBest(15*math.Sqrt2, 15) // positive == the DEFENCE took the margin
		best.defRoll.ZScore = -3.0                // ... and then fumbled it away

		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false, true)

		if !res.hit || res.defended || res.fumble {
			t.Fatalf("precondition: expected the defence-fumble exit "+
				"(hit=true, defended=false, fumble=false), got hit=%v defended=%v fumble=%v",
				res.hit, res.defended, res.fumble)
		}
		if best.margin <= 0 {
			t.Fatalf("precondition: the ATTACK must have lost the margin, got %v", best.margin)
		}
		if res.crit {
			t.Fatalf("critOnWin must not promote a swing the attack lost on margin " +
				"and won only because the defender fumbled")
		}
	})
}

// --- U10d Task 9: the same capability on the CHANNEL seam. ---
//
// The melee half above parameterises resolveDefenseOutcomeCore. Ranged and
// every special move resolve through ResolveChannelAttack instead, so CritOnWin
// has to exist on both paths. These tests drive the REAL seam --
// resolveChannelAttackWithRunner, which is what SetChannelAttackContestRunnerForTest
// repoints for out-of-package callers -- so the defence cost, the ordinary
// progression award and the crit/fumble bonus tier all still run, against a
// deterministic contest.

// pinChannelCritOnWinConfig pins the defence economy AND the crit bar, so the
// sub-crit fixture below is sub-crit for a stated reason rather than by luck.
func pinChannelCritOnWinConfig(t *testing.T) {
	t.Helper()
	pinDefenceAdmissionConfig(t)
	cfg := configs.GetConfig()
	cfg.Balance.CritBarSkillSlope = 0.05
	cfg.Balance.CritBarFloor = 1.5
	cfg.Balance.CritBarCeiling = 3.0
	configs.SetConfigForTest(t, cfg)
}

// channelSurpriseSide is the ranged AttackSide the Task 10 surprise shot will
// carry. Nothing in production sets CritOnWin yet.
func channelSurpriseSide(critOnWin bool) AttackSide {
	return AttackSide{
		Stat: 100, StatName: "perception",
		Skill: skills.RangedCombat, SkillRank: 10, Mult: 1.0,
		CritOnWin: critOnWin,
	}
}

// channelContestAt builds a contested result at a chosen NORMALIZED margin
// (Margin / (StdDev * sqrt2), the normaliser ContestCritAt uses).
//
// A magnitude of 0.5 is a settled outcome far under the 1.5 crit-bar floor
// pinned above, and both rolls sit mildly POSITIVE so neither side's
// self-relative fumble verdict fires and no defensive crit materialises.
// progression.Classify therefore sees nothing exceptional unless CritOnWin
// promotes the attacker, which is what makes the bonus-tier assertion below a
// real placement guard rather than a coincidence.
func channelContestAt(normZ float64) func(float64, []contest.Entry) contest.Result {
	return func(atkScore float64, entries []contest.Entry) contest.Result {
		stdDev := dice.StdDevFor(atkScore)
		defScore := 0.0
		name := ""
		if len(entries) > 0 {
			defScore, name = entries[0].Score, entries[0].Name
		}
		return contest.Result{
			Contested: true,
			Success:   normZ > 0,
			Winner:    name,
			Margin:    normZ * stdDev * math.Sqrt2,
			AttackRoll: dice.RollResult{
				Mean: atkScore, StdDev: stdDev,
				ZScore: 0.25, Value: atkScore + 0.25*stdDev,
			},
			DefenseRoll: dice.RollResult{
				Mean: defScore, StdDev: stdDev,
				ZScore: 0.25, Value: defScore + 0.25*stdDev,
			},
		}
	}
}

func TestCritOnWin_ChannelSeamUpgradesWinButNeverRescuesALoss(t *testing.T) {
	pinChannelCritOnWinConfig(t)

	t.Run("clean attack win becomes a crit", func(t *testing.T) {
		attacker, defender := defenceAdmissionCharacters()
		out := resolveChannelAttackWithRunner(ChannelRanged, channelSurpriseSide(true),
			attacker, defender, channelContestAt(0.5))
		if out.Defended {
			t.Fatalf("precondition: expected a clean attack win, got %+v", out)
		}
		if !out.AttackerCrit {
			t.Fatal("CritOnWin must upgrade a won channel contest to a crit")
		}
	})

	t.Run("defence win stays a defence win", func(t *testing.T) {
		attacker, defender := defenceAdmissionCharacters()
		out := resolveChannelAttackWithRunner(ChannelRanged, channelSurpriseSide(true),
			attacker, defender, channelContestAt(-0.5))
		if !out.Defended {
			t.Fatalf("precondition: expected a defence win, got %+v", out)
		}
		if out.AttackerCrit {
			t.Fatal("CritOnWin must never rescue a lost channel contest")
		}
	})

	t.Run("CritOnWin false is unchanged behaviour", func(t *testing.T) {
		attacker, defender := defenceAdmissionCharacters()
		out := resolveChannelAttackWithRunner(ChannelRanged, channelSurpriseSide(false),
			attacker, defender, channelContestAt(0.5))
		if out.Defended {
			t.Fatalf("precondition: expected a clean attack win, got %+v", out)
		}
		if out.AttackerCrit {
			t.Fatal("an ordinary win must not crit merely from winning")
		}
	})

	// The seam's own AttackerCrit line already refuses a floored outcome: the
	// +-1 sentinel is not a roll, so a mercy-granted win must not read as
	// dominance. The CritOnWin clause mirrors that gate rather than bypassing it.
	t.Run("a FLOORED win must not crit", func(t *testing.T) {
		attacker, defender := defenceAdmissionCharacters()
		out := resolveChannelAttackWithRunner(ChannelRanged, channelSurpriseSide(true),
			attacker, defender, func(atkScore float64, entries []contest.Entry) contest.Result {
				res := channelContestAt(0.5)(atkScore, entries)
				res.Floored = true
				res.Margin = 1
				return res
			})
		if out.AttackerCrit {
			t.Fatal("a sentinel margin must never be promoted to a crit")
		}
	})
}

// PLACEMENT GUARD. This is the whole reason the clause sits where it does.
//
// awardChannelDefenceBonus runs BEFORE the res.Success branch and receives
// out.AttackerCrit BY VALUE. A CritOnWin clause written inside that later branch
// produces a byte-identical ChannelDefenceResult while the progression tier has
// already run with the old false, so the surprise shot would silently earn no
// crit progression at all. Only a side-effect assertion can tell the two
// placements apart.
//
// ClaimedBonusThisRound is the observable: bonus events deliberately do not
// track use counts, and characters.ClaimedBonusThisRound is exported for
// precisely this cross-package need.
func TestCritOnWin_ChannelCritReachesTheProgressionTier(t *testing.T) {
	pinChannelCritOnWinConfig(t)

	t.Run("the crit pays the attacker's bonus tier", func(t *testing.T) {
		attacker, defender := defenceAdmissionCharacters()
		out := resolveChannelAttackWithRunner(ChannelRanged, channelSurpriseSide(true),
			attacker, defender, channelContestAt(0.5))
		if !out.AttackerCrit {
			t.Fatalf("precondition: CritOnWin must have produced a crit, got %+v", out)
		}
		if !attacker.ClaimedBonusThisRound(string(skills.RangedCombat)) {
			t.Fatal("the CritOnWin crit never reached awardChannelDefenceBonus: the clause " +
				"has been moved AFTER it (into the res.Success branch), so the surprise " +
				"shot lands as a crit but earns no crit progression")
		}
	})

	// Control. Without it the assertion above would still pass if the tier fired
	// on every contest, which would make it no guard at all.
	t.Run("control: the same contest without CritOnWin pays nothing", func(t *testing.T) {
		attacker, defender := defenceAdmissionCharacters()
		out := resolveChannelAttackWithRunner(ChannelRanged, channelSurpriseSide(false),
			attacker, defender, channelContestAt(0.5))
		if out.AttackerCrit {
			t.Fatalf("precondition: this fixture must be sub-crit without CritOnWin, got %+v", out)
		}
		if attacker.ClaimedBonusThisRound(string(skills.RangedCombat)) {
			t.Fatal("an unexceptional contest paid the bonus tier; the assertion above " +
				"proves nothing")
		}
	})
}

// Both early returns are attack WINS by the seam's own comment ("Uncontested is
// an attack win, which is what a full multiplier says"). Omitting CritOnWin
// there would mean a defender with NO available defence denies the ambush bonus
// while a fully-defended one grants it -- an inversion of the fiction that would
// read as a bug in play.
func TestCritOnWin_ChannelEarlyReturnsAreAttackWins(t *testing.T) {
	pinChannelCritOnWinConfig(t)

	// An unknown channel has no DefenceSetFor row, which is how this package
	// reaches the len(defences) == 0 exit.
	t.Run("empty defence set", func(t *testing.T) {
		attacker, defender := defenceAdmissionCharacters()
		ran := false
		runner := func(float64, []contest.Entry) contest.Result {
			ran = true
			return contest.Result{}
		}
		out := resolveChannelAttackWithRunner(AttackChannel("unknown"), channelSurpriseSide(true),
			attacker, defender, runner)
		if ran {
			t.Fatal("precondition: the empty-defence exit must return before the contest runs")
		}
		if !out.AttackerCrit {
			t.Fatal("CritOnWin must crit when no defence answers the channel")
		}
	})

	t.Run("uncontested roll", func(t *testing.T) {
		attacker, defender := defenceAdmissionCharacters()
		out := resolveChannelAttackWithRunner(ChannelRanged, channelSurpriseSide(true),
			attacker, defender, func(float64, []contest.Entry) contest.Result {
				return contest.Result{Contested: false}
			})
		if !out.AttackerCrit {
			t.Fatal("CritOnWin must crit on an uncontested roll, which the seam treats as an attack win")
		}
	})

	// Controls: neither early return may hand out a crit unconditionally.
	t.Run("control: CritOnWin false leaves the empty defence set uncritted", func(t *testing.T) {
		attacker, defender := defenceAdmissionCharacters()
		out := resolveChannelAttackWithRunner(AttackChannel("unknown"), channelSurpriseSide(false),
			attacker, defender, func(float64, []contest.Entry) contest.Result {
				return contest.Result{}
			})
		if out.AttackerCrit {
			t.Fatal("the empty-defence exit crit with neither ForceCrit nor CritOnWin set")
		}
	})

	t.Run("control: CritOnWin false leaves the uncontested roll uncritted", func(t *testing.T) {
		attacker, defender := defenceAdmissionCharacters()
		out := resolveChannelAttackWithRunner(ChannelRanged, channelSurpriseSide(false),
			attacker, defender, func(float64, []contest.Entry) contest.Result {
				return contest.Result{Contested: false}
			})
		if out.AttackerCrit {
			t.Fatal("the uncontested exit crit with neither ForceCrit nor CritOnWin set")
		}
	})
}
