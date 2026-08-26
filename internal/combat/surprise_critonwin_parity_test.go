package combat

// surprise_critonwin_parity_test.go — U10d Task 16.
//
// crit-on-win exists TWICE, because the arc has two attack paths:
//
//   melee    resolveDefenseOutcomeCore's critOnWin parameter
//            (combat_helpers.go), guarded by
//            critOnWin && res.hit && !res.defended && !best.floored &&
//            !res.fumble && best.margin <= 0
//
//   channel  AttackSide.CritOnWin in resolveChannelAttackWithRunner
//            (defence_multiplier.go), guarded by
//            side.CritOnWin && res.Success && !res.Floored && !out.AttackerFumble
//
// Ranged and every special move resolve through the channel seam; ordinary
// melee swings do not. A player cannot tell which seam answered their ambush,
// so the two guards must agree on every input or the same fiction produces two
// different outcomes depending on the weapon in hand.
//
// The two guards are written from different vocabularies (hit/defended/margin
// versus Success), and they are NOT trivially equivalent: melee's
// `best.margin <= 0` term exists solely because melee's defence-FUMBLE exit
// returns hit == true / defended == false on a swing the ATTACK LOST on margin,
// which the channel seam's res.Success would refuse. Drop that one term and the
// paths diverge on exactly one input — the defence-fumble variant below.
//
// This file therefore drives BOTH paths with the same abstract input and
// asserts the SAME verdict. It deliberately does NOT assert each path's own
// behaviour separately: two independent single-path assertions cannot detect a
// divergence, which is the only thing this guard is for.

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/contest"
)

// critOnWinVariant is the abstract contest situation, shared by both paths.
// Each path expresses it in its OWN representation (melee: a bestDefenseResult
// with a defence-positive margin; channel: a contest.Result with Success and a
// normalised margin), because there is no common input type — that asymmetry is
// precisely why the two guards drifted apart in the first place.
type critOnWinVariant string

const (
	// The attack cleanly took the margin. Sub-crit on both paths without
	// critOnWin, so only critOnWin can promote it.
	variantCleanWin critOnWinVariant = "clean attack win"
	// The DEFENCE took the margin.
	variantDefenceWin critOnWinVariant = "defence win"
	// The defence took the margin AND then fumbled it away. This is the one
	// input the two guards are written differently for: melee reaches its
	// defence-fumble exit (hit, undefended, not a fumble) on a swing the attack
	// LOST, while the channel seam still reports res.Success == false.
	variantDefenceFumble critOnWinVariant = "defence fumble on a margin the attack lost"
	// The attack won only because the contest floor granted it — a sentinel
	// margin, not a roll.
	variantFlooredWin critOnWinVariant = "floored win"
	// The attack took the margin and then fumbled its own roll.
	variantAttackerFumble critOnWinVariant = "attacker fumble on a margin the attack won"
)

// meleeBestFor builds the melee path's fixture for a variant.
//
// attackWinBest / defenceWinBest are the package's existing settled-contest
// fixtures (see surprise_critonwin_test.go and defence_multiplier_test.go);
// both carry a real normaliser on the roll, so the crit check runs down the
// margin path production actually uses rather than the legacy self-relative
// fallback.
func meleeBestFor(variant critOnWinVariant) bestDefenseResult {
	switch variant {
	case variantCleanWin:
		return attackWinBest()
	case variantDefenceWin:
		return defenceWinBest(15*math.Sqrt2, 15)
	case variantDefenceFumble:
		best := defenceWinBest(15*math.Sqrt2, 15) // positive == the DEFENCE took the margin
		best.defRoll.ZScore = -3.0                // ... and then fumbled it away
		return best
	case variantFlooredWin:
		best := attackWinBest()
		best.floored = true
		return best
	case variantAttackerFumble:
		best := attackWinBest()
		best.hitRoll.ZScore = -3.0
		return best
	}
	panic("unknown critOnWin variant: " + string(variant))
}

// channelRunnerFor builds the channel path's fixture for the same variant.
//
// channelContestAt(normZ) yields a settled contest at a chosen NORMALIZED
// margin; |0.5| is far under the crit-bar floor pinned by
// pinChannelCritOnWinConfig, so the win is sub-crit for a stated reason rather
// than by luck.
func channelRunnerFor(variant critOnWinVariant) func(float64, []contest.Entry) contest.Result {
	switch variant {
	case variantCleanWin:
		return channelContestAt(0.5)
	case variantDefenceWin:
		return channelContestAt(-0.5)
	case variantDefenceFumble:
		return func(atkScore float64, entries []contest.Entry) contest.Result {
			res := channelContestAt(-0.5)(atkScore, entries)
			res.DefenseRoll.ZScore = -3.0
			return res
		}
	case variantFlooredWin:
		return func(atkScore float64, entries []contest.Entry) contest.Result {
			res := channelContestAt(0.5)(atkScore, entries)
			res.Floored = true
			res.Margin = 1 // the +-1 sentinel a floored outcome carries
			return res
		}
	case variantAttackerFumble:
		return func(atkScore float64, entries []contest.Entry) contest.Result {
			res := channelContestAt(0.5)(atkScore, entries)
			res.AttackRoll.ZScore = -3.0
			return res
		}
	}
	panic("unknown critOnWin variant: " + string(variant))
}

// assertMeleePrecondition proves the melee fixture reached the branch the
// variant names. Without this the whole table could pass while every fixture
// silently collapsed onto the same code path.
func assertMeleePrecondition(t *testing.T, variant critOnWinVariant, best bestDefenseResult, res hitResolution) {
	t.Helper()
	switch variant {
	case variantCleanWin:
		if !res.hit || res.defended || res.fumble {
			t.Fatalf("precondition: expected a clean attack win, got hit=%v defended=%v fumble=%v",
				res.hit, res.defended, res.fumble)
		}
	case variantDefenceWin:
		if !res.defended {
			t.Fatalf("precondition: expected a defence win, got hit=%v defended=%v", res.hit, res.defended)
		}
	case variantDefenceFumble:
		// The exit that makes melee's `best.margin <= 0` term necessary.
		if !res.hit || res.defended || res.fumble {
			t.Fatalf("precondition: expected the defence-fumble exit "+
				"(hit=true, defended=false, fumble=false), got hit=%v defended=%v fumble=%v",
				res.hit, res.defended, res.fumble)
		}
		if best.margin <= 0 {
			t.Fatalf("precondition: the ATTACK must have LOST the margin, got %v", best.margin)
		}
	case variantFlooredWin:
		if !best.floored {
			t.Fatal("precondition: the fixture must carry the floored sentinel")
		}
	case variantAttackerFumble:
		if !res.fumble || res.hit {
			t.Fatalf("precondition: expected an attack fumble, got hit=%v fumble=%v", res.hit, res.fumble)
		}
	}
}

// assertChannelPrecondition is the channel-side mirror.
func assertChannelPrecondition(t *testing.T, variant critOnWinVariant, out ChannelDefenceResult) {
	t.Helper()
	switch variant {
	case variantCleanWin:
		if out.Defended {
			t.Fatalf("precondition: expected a clean attack win, got %+v", out)
		}
	case variantDefenceWin, variantDefenceFumble:
		if !out.Defended {
			t.Fatalf("precondition: expected a defence win, got %+v", out)
		}
	case variantAttackerFumble:
		if !out.AttackerFumble {
			t.Fatalf("precondition: expected an attacker fumble, got %+v", out)
		}
	}
}

// TestCritOnWin_MeleeAndChannelAgree is the parity guard the CritOnWin field
// comment (defence_multiplier.go) names. One table, two paths, one verdict.
func TestCritOnWin_MeleeAndChannelAgree(t *testing.T) {
	pinChannelCritOnWinConfig(t)

	cases := []struct {
		name      string
		variant   critOnWinVariant
		critOnWin bool
		wantCrit  bool
	}{
		{"win with critOnWin crits", variantCleanWin, true, true},
		{"win without critOnWin does not", variantCleanWin, false, false},
		{"loss with critOnWin does not", variantDefenceWin, true, false},
		{"loss without critOnWin does not", variantDefenceWin, false, false},

		// The three inputs the two guards phrase differently. Each is a place
		// where one path could crit and the other refuse.
		{"defence fumble with critOnWin does not", variantDefenceFumble, true, false},
		{"defence fumble without critOnWin does not", variantDefenceFumble, false, false},
		{"floored win with critOnWin does not", variantFlooredWin, true, false},
		{"floored win without critOnWin does not", variantFlooredWin, false, false},
		{"attacker fumble with critOnWin does not", variantAttackerFumble, true, false},
		{"attacker fumble without critOnWin does not", variantAttackerFumble, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// ── melee ────────────────────────────────────────────────────
			src, tgt := defenceFixture(1000)
			best := meleeBestFor(tc.variant)
			meleeRes := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false, tc.critOnWin)
			assertMeleePrecondition(t, tc.variant, best, meleeRes)

			// ── channel ──────────────────────────────────────────────────
			attacker, defender := defenceAdmissionCharacters()
			channelOut := resolveChannelAttackWithRunner(ChannelRanged,
				channelSurpriseSide(tc.critOnWin), attacker, defender, channelRunnerFor(tc.variant))
			assertChannelPrecondition(t, tc.variant, channelOut)

			// ── the whole point ──────────────────────────────────────────
			if meleeRes.crit != channelOut.AttackerCrit {
				t.Fatalf("DIVERGENCE on %q (critOnWin=%v): melee crit=%v, channel crit=%v.\n"+
					"The two crit-on-win guards no longer agree, so the same ambush crits with one "+
					"weapon and not another. Fix the PRODUCTION divergence — do not split this into "+
					"two per-path assertions, which is exactly the shape that cannot detect it.",
					tc.variant, tc.critOnWin, meleeRes.crit, channelOut.AttackerCrit)
			}
			if meleeRes.crit != tc.wantCrit {
				t.Errorf("%q (critOnWin=%v): both paths agree on crit=%v, want %v — "+
					"they have drifted TOGETHER, away from the specified rule",
					tc.variant, tc.critOnWin, meleeRes.crit, tc.wantCrit)
			}
		})
	}
}
