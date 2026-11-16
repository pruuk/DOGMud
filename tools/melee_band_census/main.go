// Command melee_band_census measures how often a melee defensive win lands in
// each narration band under the OLD rule (defender's self-relative z-score) and
// the NEW rule (defensive crit, then normalized contest margin), across a grid
// of attacker/defender score matchups.
//
// One-off: run before and after the M4c flip, paste the table into
// docs/superpowers/audits/2026-09-20-melee-defence-band-shares.md. Not wired
// into CI -- internal/combat is already ~200s of the 300s -race cap.
package main

import (
	"fmt"
	"math"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/contest"
)

const samples = 50000

type shares struct{ weak, normal, heavy int }

func (s shares) row(label string, total int) string {
	if total == 0 {
		return fmt.Sprintf("| %s | - | - | - | 0 |", label)
	}
	pct := func(n int) float64 { return 100 * float64(n) / float64(total) }
	return fmt.Sprintf("| %s | %.1f%% | %.1f%% | %.1f%% | %d |",
		label, pct(s.weak), pct(s.normal), pct(s.heavy), total)
}

func main() {
	matchups := []struct {
		name               string
		atkScore, defScore float64
	}{
		{"defender outclassed (100 vs 60)", 100, 60},
		{"defender behind (100 vs 85)", 100, 85},
		{"even (100 vs 100)", 100, 100},
		{"defender ahead (85 vs 100)", 85, 100},
		{"defender dominant (60 vs 100)", 60, 100},
	}

	fmt.Println("| matchup | rule | weak | normal | heavy | defensive wins |")
	fmt.Println("|---|---|---|---|---|---|")
	for _, m := range matchups {
		var oldS, newS shares
		wins := 0
		for range samples {
			res := combat.RunContest(m.atkScore, []contest.Entry{{Name: "dodge", Score: m.defScore}})
			if res.Success { // the attacker won; no defence narration is sent
				continue
			}
			wins++

			// OLD rule: the defender's own roll, self-relative.
			switch {
			case res.DefenseRoll.ZScore >= 2.0:
				oldS.heavy++
			case res.DefenseRoll.ZScore >= 0.5:
				oldS.normal++
			default:
				oldS.weak++
			}

			// NEW rule: defensive crit, else the normalized margin. A floored
			// outcome carries a sentinel margin, never crits, and reads 0.
			norm := 0.0
			if !res.Floored && res.DefenseRoll.StdDev > 0 {
				norm = -res.Margin / (res.DefenseRoll.StdDev * math.Sqrt2)
			}
			switch {
			case !res.Floored && norm >= combat.ContestCritThreshold:
				newS.heavy++
			case norm >= 0.5:
				newS.normal++
			default:
				newS.weak++
			}
		}
		fmt.Println(oldS.row(m.name+" | old (z-score)", wins))
		fmt.Println(newS.row(m.name+" | new (crit+margin)", wins))
	}
}
