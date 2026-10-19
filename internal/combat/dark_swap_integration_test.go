package combat

import (
	"slices"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/darkcombat"
)

// darkPools returns every line of every outcome pool for viewpoint v.
func darkPools(v darkcombat.Viewpoint) []string {
	var all []string
	for _, o := range darkcombat.Outcomes() {
		all = append(all, darkcombat.Pool(v, o)...)
	}
	return all
}

// The dark swap is unit-tested against hand-built results. This drives the
// REAL composers (calculateCombat over a bare-handed attacker, so a
// multi-swing round) and asserts, round by round, that a reader who cannot
// see reads one dark-pool line per swing that is not a double fumble, and
// that every other line is the double-fumble line the composer kept.
//
// A swing composer that bypasses sendSwingToSource / sendSwingToTarget
// leaves its composed line in place of the dark line; the count check then
// fails.
func TestDarkSwap_RealComposersBlindReader(t *testing.T) {
	loadDarkCombatForTest(t)
	cases := []struct {
		name        string
		blindAttack bool
		viewpoint   darkcombat.Viewpoint
		lines       func(r *AttackResult) []TaggedMessage
	}{
		{"blind attacker reads MessagesToSource", true, darkcombat.Attacker,
			func(r *AttackResult) []TaggedMessage { return r.MessagesToSource }},
		{"blind defender reads MessagesToTarget", false, darkcombat.Defender,
			func(r *AttackResult) []TaggedMessage { return r.MessagesToTarget }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			atk, def := swingDefencePopulationCombatants()
			blind := verdictObserver(t, false, false, false, true)
			// The fixture characters carry stats and an Aggro; the blind one
			// borrows the blinded perception machine from the verdict observer.
			if c.blindAttack {
				atk.Perception = blind.Perception
			} else {
				def.Perception = blind.Perception
			}
			atk.Name, def.Name = "Grimwald", "Shade"
			pool := darkPools(c.viewpoint)

			sawMultiSwing, sawDark := false, false
			for i := 0; i < 400; i++ {
				atk.Stamina, def.Stamina, def.Health = 1000, 500, 5000
				ctx := newCombatContext(atk, def, verdictLight(100), false)
				plan := buildAttackPlan(atk, def)
				res := calculateCombat(atk, def, User, Mob, plan, ctx)

				wantDark := 0
				doubleFumble := false
				for _, se := range res.SwingEvents {
					if se.DoubleFumble {
						doubleFumble = true
					} else {
						wantDark++
					}
				}
				gotDark := 0
				for _, m := range c.lines(&res) {
					// A defender's damaging swing ends with wound words (#572)
					// after its pool line.
					if slices.Contains(pool, stripWoundWords(m.Text)) {
						gotDark++
						continue
					}
					if !doubleFumble {
						t.Fatalf("round %d: line %q (%v) is neither a %v dark line nor part of a double fumble", i, m.Text, m.Category, c.viewpoint)
					}
				}
				if gotDark != wantDark {
					t.Fatalf("round %d: %d dark lines, want %d (one per non-double-fumble swing of %d): %v",
						i, gotDark, wantDark, len(res.SwingEvents), c.lines(&res))
				}
				sawMultiSwing = sawMultiSwing || len(res.SwingEvents) >= 2
				sawDark = sawDark || gotDark > 0
			}
			if !sawMultiSwing || !sawDark {
				t.Fatalf("fixture no longer exercises the swap: multiSwing=%v dark=%v", sawMultiSwing, sawDark)
			}
		})
	}
}
