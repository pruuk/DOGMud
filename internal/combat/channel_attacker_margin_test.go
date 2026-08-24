package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
)

// AttackerNormalizedMargin is the attack-positive twin of
// NormalizedDefenceMargin. The two populate on OPPOSITE paths and neither
// substitutes for the other -- contest.Result.Margin's own docs record why that
// matters: mixing the conventions compiles cleanly and silently puts the
// outcome on the losing side.
//
// defenceAdmissionCharacters is used rather than defenceFixture because it is
// the only shared fixture that gives the defender a rhetoric rank, and defy is
// the sole entry in ChannelSocial's defence set.
func TestAttackerNormalizedMargin_PopulatedOnAttackWin(t *testing.T) {
	attacker, defender := defenceAdmissionCharacters()

	restore := SetChannelAttackContestRunnerForTest(
		func(atk float64, entries []contest.Entry) contest.Result {
			name := ""
			if len(entries) > 0 {
				name = entries[0].Name
			}
			return contest.Result{
				Success: true, Contested: true, Winner: name,
				Margin:      30,
				AttackRoll:  dice.RollResult{StdDev: 10},
				DefenseRoll: dice.RollResult{StdDev: 10},
			}
		})
	defer restore()

	out := ResolveChannelAttack(ChannelSocial, AttackSide{Stat: 100}, attacker, defender)

	want := 30.0 / (10.0 * math.Sqrt2)
	if math.Abs(out.AttackerNormalizedMargin-want) > 1e-9 {
		t.Errorf("AttackerNormalizedMargin = %v, want %v", out.AttackerNormalizedMargin, want)
	}
	// The sign assertion is the point of this test. A decisive ATTACK win must
	// read positive; if it reads negative the two margin conventions have been
	// crossed and every consumer scales the wrong way.
	if out.AttackerNormalizedMargin <= 0 {
		t.Error("a decisive attack win produced a non-positive margin; the sign is inverted")
	}
	if out.NormalizedDefenceMargin != 0 {
		t.Errorf("NormalizedDefenceMargin = %v, want 0 on an attack win", out.NormalizedDefenceMargin)
	}
}

func TestAttackerNormalizedMargin_ZeroWhenDefenceWon(t *testing.T) {
	attacker, defender := defenceAdmissionCharacters()

	restore := SetChannelAttackContestRunnerForTest(
		func(atk float64, entries []contest.Entry) contest.Result {
			name := ""
			if len(entries) > 0 {
				name = entries[0].Name
			}
			return contest.Result{
				Success: false, Contested: true, Winner: name,
				Margin:      -30,
				AttackRoll:  dice.RollResult{StdDev: 10},
				DefenseRoll: dice.RollResult{StdDev: 10},
			}
		})
	defer restore()

	out := ResolveChannelAttack(ChannelSocial, AttackSide{Stat: 100}, attacker, defender)

	if out.AttackerNormalizedMargin != 0 {
		t.Errorf("AttackerNormalizedMargin = %v, want 0 when the defence won", out.AttackerNormalizedMargin)
	}
	// Its defence-positive twin is the field that carries meaning here.
	if out.NormalizedDefenceMargin <= 0 {
		t.Errorf("NormalizedDefenceMargin = %v, want positive on a defence win", out.NormalizedDefenceMargin)
	}
}

// A floored outcome stamps a +-1 SENTINEL margin rather than a roll. Reporting
// it would let a mercy-granted win read as dominance, which is the opposite of
// what a floor means.
func TestAttackerNormalizedMargin_ZeroWhenFloored(t *testing.T) {
	attacker, defender := defenceAdmissionCharacters()

	restore := SetChannelAttackContestRunnerForTest(
		func(atk float64, entries []contest.Entry) contest.Result {
			name := ""
			if len(entries) > 0 {
				name = entries[0].Name
			}
			return contest.Result{
				Success: true, Contested: true, Floored: true, Winner: name,
				Margin:      1,
				AttackRoll:  dice.RollResult{StdDev: 10},
				DefenseRoll: dice.RollResult{StdDev: 10},
			}
		})
	defer restore()

	out := ResolveChannelAttack(ChannelSocial, AttackSide{Stat: 100}, attacker, defender)

	if out.AttackerNormalizedMargin != 0 {
		t.Errorf("AttackerNormalizedMargin = %v, want 0 on a floored win", out.AttackerNormalizedMargin)
	}
}

// The ForceCrit forced win returns ABOVE the res.Success exit, so it leaves the
// margin at zero even though it is the most decisive outcome the system can
// produce. This test pins that as KNOWN rather than discovered.
//
// It matters to any consumer that scales an effect by decisiveness: a
// sleeping victim is exactly the case where a caller would expect a maximal
// margin and will instead read the minimum. Handle ForceCrit explicitly at the
// call site until this field is given a defined value on that path.
func TestAttackerNormalizedMargin_ZeroOnForcedCritWin_KNOWN(t *testing.T) {
	attacker, defender := defenceAdmissionCharacters()

	restore := SetChannelAttackContestRunnerForTest(
		func(atk float64, entries []contest.Entry) contest.Result {
			name := ""
			if len(entries) > 0 {
				name = entries[0].Name
			}
			return contest.Result{
				Success: false, Contested: true, Winner: name,
				Margin:      -30,
				AttackRoll:  dice.RollResult{StdDev: 10},
				DefenseRoll: dice.RollResult{StdDev: 10},
			}
		})
	defer restore()

	out := ResolveChannelAttack(ChannelSocial, AttackSide{Stat: 100, ForceCrit: true}, attacker, defender)

	if !out.AttackerCrit {
		t.Fatal("precondition: ForceCrit must produce an attacker crit")
	}
	if out.AttackerNormalizedMargin != 0 {
		t.Errorf("AttackerNormalizedMargin = %v; this path is documented as leaving it zero. "+
			"If that changed deliberately, update the field's doc comment and every consumer "+
			"that special-cases ForceCrit", out.AttackerNormalizedMargin)
	}
}
