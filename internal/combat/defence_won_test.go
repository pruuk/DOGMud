package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
)

// U10b-1 Task 9: hitResolution.defenceWon is the melee half of the defender
// award's win/lose predicate, and it is the UNION of the two defensive-win
// shapes rather than either flag alone.
//
// This test exists because "simplifying" it to r.defended compiles, passes every
// other test in the package, and silently stops paying defensive crits -- the
// most decisive defensive outcome the system produces -- their full-weight
// progression. Verified by temporarily making that reduction: the
// DefensiveCrit and FloorPromoted rows below fail and nothing else does.
func TestDefenceWon_IsTheUnionOfBothDefensiveWinShapes(t *testing.T) {
	cases := []struct {
		name string
		res  hitResolution
		want bool
	}{
		// The Task 10 deflection: the defence won on margin and the swing still
		// lands for partial damage.
		{"deflection", hitResolution{hit: true, defended: true, damageMult: 0.6}, true},
		// A defensive crit fully negates and DECLARES defended false, so reading
		// that flag alone would score the strongest defensive result as a loss.
		{"defensive crit", hitResolution{hit: false, defenseCrit: true, damageMult: 0.0}, true},
		// Both set is not a shape resolveDefenseOutcomeInner produces, but the
		// union must not care.
		{"both flags", hitResolution{defended: true, defenseCrit: true}, true},

		{"clean attack win", hitResolution{hit: true, damageMult: 1.0}, false},
		{"attack crit", hitResolution{hit: true, crit: true, damageMult: 1.0}, false},
		// A forced crit against a sleeping victim: the attack win is decided
		// before the roll, so the defence lost however it rolled. Shape taken
		// from the crit branch of resolveDefenseOutcomeInner under forceCrit.
		{"forced crit", hitResolution{hit: true, crit: true, damageMult: 1.0}, false},

		// All THREE fumble paths. Melee treats a fumble as absolute and returns
		// before attackWon is computed, so none of them can report a defensive
		// win whatever the margin said. This is a KNOWN divergence from the
		// channel seam, which has no fumble branch before its award and pays
		// that defence as a win; U10b-1b owns reconciling it. These rows pin
		// melee's side so the divergence cannot change by accident.
		{"double fumble", hitResolution{fumble: true, doubleFumble: true, hit: false}, false},
		{"attack fumble", hitResolution{fumble: true, hit: false}, false},
		// The defence-fumble branch sets only hit and damageMult -- res.fumble
		// is the ATTACKER's flag -- so its shape is deliberately identical to a
		// clean attack win. Kept as its own row so all three branches are
		// enumerated rather than argued about.
		{"defence fumble", hitResolution{hit: true, damageMult: 1.0}, false},

		{"zero value", hitResolution{}, false},
	}

	for _, tc := range cases {
		if got := tc.res.defenceWon(); got != tc.want {
			t.Errorf("%s: defenceWon() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The floor promotion is the REASON the union exists, and it is the one case a
// table of hand-built structs cannot pin: applyCritFloors turns a deflection
// into a defensive crit and CLEARS defended as it goes, so a reduction to
// r.defended would flip this swing from a defensive win to a loss purely
// because the floor was generous to it.
//
// defenseFloor is 1.0 so the promotion is certain rather than a 1% coin flip;
// attackFloor is 0 so the attack side cannot interfere. best.floored must stay
// false, since applyCritFloors returns early on a sentinel margin.
func TestDefenceWon_SurvivesTheFloorPromotionThatClearsDefended(t *testing.T) {
	res := hitResolution{hit: true, defended: true, damageMult: 0.6}
	if !res.defenceWon() {
		t.Fatalf("precondition: the deflection did not read as a defensive win before the promotion")
	}

	applyCritFloors(&res, &AttackResult{}, dodgeBestWon(defenceWinMargin), 0.0, 1.0)

	if !res.defenseCrit {
		t.Fatalf("precondition: defenseFloor 1.0 did not promote the deflection; the fixture is not exercising the promotion")
	}
	if res.defended {
		t.Fatalf("precondition: the promotion did not clear defended, so this test is not covering the case it exists for")
	}
	if !res.defenceWon() {
		t.Error("defenceWon() = false after a floor-promoted defensive crit; the union has been reduced to r.defended and a promoted save now trains as a loss")
	}
}

// Guard the mapping this file's fixtures depend on: dodgeBestWon names a real
// defence, which is what applyCritFloors keys its defence branch on.
func TestDefenceWon_FloorFixtureNamesARealDefence(t *testing.T) {
	if dodgeBestWon(defenceWinMargin).defenseType != characters.DefenseDodge {
		t.Fatal("the floor fixture stopped naming a real defence; applyCritFloors would return before promoting")
	}
}
