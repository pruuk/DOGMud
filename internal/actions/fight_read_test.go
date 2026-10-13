package actions

import (
	"math"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
)

var inf = math.Inf(1)

func TestPaceFor(t *testing.T) {
	bal := configs.GetBalanceConfig() // Go defaults: 2 / 6 / 20
	cases := []struct {
		rounds float64
		want   Pace
	}{{1, PaceMoments}, {2, PaceMoments}, {2.01, PaceShort}, {6, PaceShort}, {20, PaceLong}, {20.5, PaceAge}, {inf, PaceNever}}
	for _, c := range cases {
		if got := paceFor(c.rounds, bal); got != c.want {
			t.Errorf("paceFor(%v) = %v, want %v", c.rounds, got, c.want)
		}
	}
}

func TestClassifyFight(t *testing.T) {
	bal := configs.GetBalanceConfig()
	p := func(you, them, cost, dent float64) combat.FightPrediction {
		return combat.FightPrediction{TheirDamagePerRound: 1, RoundsToKillThem: you, RoundsToKillYou: them, Cost: cost, Dent: dent}
	}
	cases := []struct {
		name string
		in   combat.FightPrediction
		want Verdict
	}{
		{"no incoming damage", combat.FightPrediction{TheirDamagePerRound: 0, RoundsToKillThem: 3, RoundsToKillYou: inf}, VerdictUntouched},
		{"tiny cost", p(3, 60, 0.04, 20), VerdictUntouched},
		// The effigy: both sides an age; dent is tiny only because it is slow.
		{"effigy stalemate before hopeless", p(5000, 400, 12, 0.01), VerdictStalemate},
		{"megalomania", p(inf, 1, inf, 0.001), VerdictHopeless},
		{"scratches", p(3, 30, 0.1, 10), VerdictScratches},
		{"hurt", p(4, 10, 0.4, 2.5), VerdictHurt},
		{"barely", p(8, 9, 0.9, 1.1), VerdictBarely},
		{"likely lose", p(10, 5, 2, 0.5), VerdictLikelyLose},
		{"lose badly", p(12, 2, 6, 0.2), VerdictLoseBadly},
	}
	for _, c := range cases {
		if got := classifyFight(c.in, bal).Verdict; got != c.want {
			t.Errorf("%s: verdict %v, want %v", c.name, got, c.want)
		}
	}
}

func TestMuddySlide(t *testing.T) {
	want := map[Verdict]Verdict{
		VerdictUntouched: VerdictUntouched, VerdictScratches: VerdictHurt, VerdictHurt: VerdictBarely,
		VerdictBarely: VerdictBarely, VerdictLikelyLose: VerdictBarely, VerdictLoseBadly: VerdictLikelyLose,
		VerdictHopeless: VerdictHopeless, VerdictStalemate: VerdictStalemate,
	}
	for in, out := range want {
		if got := muddySlide(in); got != out {
			t.Errorf("muddySlide(%v) = %v, want %v", in, got, out)
		}
	}
}

func TestReadQualityFor(t *testing.T) {
	bal := configs.GetBalanceConfig()
	sd := dice.StdDevFor(100)
	if got := readQualityFor(contest.Result{Success: false}, 100, bal); got != ReadMuddy {
		t.Errorf("lost roll: %v, want muddy", got)
	}
	if got := readQualityFor(contest.Result{Success: true, Margin: 0.5 * sd}, 100, bal); got != ReadFair {
		t.Errorf("narrow win: %v, want fair", got)
	}
	if got := readQualityFor(contest.Result{Success: true, Margin: 1.5 * sd}, 100, bal); got != ReadClear {
		t.Errorf("wide win: %v, want clear", got)
	}
}

// Player copy: every line a read can produce fits 80 columns with a long name,
// and carries no digits.
func TestFightReadCopy_FitsAndHasNoNumbers(t *testing.T) {
	name := "Captain of the Watch Halix" // 26 characters
	var lines []string
	for _, q := range []ReadQuality{ReadClear, ReadFair, ReadMuddy} {
		lines = append(lines, openerText(q, name))
	}
	for v := VerdictUntouched; v <= VerdictStalemate; v++ {
		lines = append(lines, verdictText[v])
	}
	for yp := PaceMoments; yp <= PaceNever; yp++ {
		for tp := PaceMoments; tp <= PaceNever; tp++ {
			lines = append(lines, paceText(FightRead{YourPace: yp, TheirPace: tp}))
		}
	}
	for _, l := range lines {
		if len(l) > 80 {
			t.Errorf("%d columns: %q", len(l), l)
		}
		if strings.ContainsAny(l, "0123456789") {
			t.Errorf("a number in player copy: %q", l)
		}
		if l == "" {
			t.Error("an empty line in the copy tables")
		}
	}
}
