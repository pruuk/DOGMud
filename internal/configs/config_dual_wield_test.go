package configs

import "testing"

// The five dual-wield knobs ship at the values the penalty had as literals, and
// the Go defaults match, because a test binary never loads config.yaml.
func TestDualWieldKnobs_ShippedMatchTheDefaults(t *testing.T) {
	shipped := shippedBalance(t)
	var absent Balance
	absent.validateCombatDualWield()
	cases := []struct {
		name            string
		shipped, absent ConfigInt
		want            ConfigInt
	}{
		{"DualWieldPenaltyBase", shipped.DualWieldPenaltyBase, absent.DualWieldPenaltyBase, 50},
		{"DualWieldPenaltyReduction", shipped.DualWieldPenaltyReduction, absent.DualWieldPenaltyReduction, 40},
		{"DualWieldPenaltyRankDivisor", shipped.DualWieldPenaltyRankDivisor, absent.DualWieldPenaltyRankDivisor, 50},
		{"DualWieldPenaltyFloor", shipped.DualWieldPenaltyFloor, absent.DualWieldPenaltyFloor, 10},
		{"DualWieldPenaltyPerExtraArm", shipped.DualWieldPenaltyPerExtraArm, absent.DualWieldPenaltyPerExtraArm, 20},
	}
	for _, tc := range cases {
		if tc.shipped != tc.want {
			t.Errorf("%s ships at %d, want %d", tc.name, tc.shipped, tc.want)
		}
		if tc.absent != tc.want {
			t.Errorf("%s absent reads %d, want %d", tc.name, tc.absent, tc.want)
		}
	}
}

// Below 1 takes the default, a set value is kept, and the floor never passes the
// base. validateCombat is what the balance load runs, so the test goes through it.
func TestDualWieldKnobs_Validation(t *testing.T) {
	negative := Balance{
		DualWieldPenaltyBase:        -1,
		DualWieldPenaltyReduction:   -1,
		DualWieldPenaltyRankDivisor: -1,
		DualWieldPenaltyFloor:       -1,
		DualWieldPenaltyPerExtraArm: -1,
	}
	negative.validateCombat()
	if negative.DualWieldPenaltyBase != 50 || negative.DualWieldPenaltyReduction != 40 ||
		negative.DualWieldPenaltyRankDivisor != 50 || negative.DualWieldPenaltyFloor != 10 ||
		negative.DualWieldPenaltyPerExtraArm != 20 {
		t.Fatalf("negative values must take the defaults: %+v", []ConfigInt{
			negative.DualWieldPenaltyBase, negative.DualWieldPenaltyReduction,
			negative.DualWieldPenaltyRankDivisor, negative.DualWieldPenaltyFloor,
			negative.DualWieldPenaltyPerExtraArm,
		})
	}

	kept := Balance{
		DualWieldPenaltyBase:        70,
		DualWieldPenaltyReduction:   30,
		DualWieldPenaltyRankDivisor: 25,
		DualWieldPenaltyFloor:       5,
		DualWieldPenaltyPerExtraArm: 7,
	}
	kept.validateCombat()
	if kept.DualWieldPenaltyBase != 70 || kept.DualWieldPenaltyReduction != 30 ||
		kept.DualWieldPenaltyRankDivisor != 25 || kept.DualWieldPenaltyFloor != 5 ||
		kept.DualWieldPenaltyPerExtraArm != 7 {
		t.Fatalf("set values must be kept: %+v", kept.DualWieldPenaltyBase)
	}

	capped := Balance{DualWieldPenaltyBase: 8, DualWieldPenaltyFloor: 30}
	capped.validateCombat()
	if capped.DualWieldPenaltyFloor != 8 {
		t.Fatalf("a floor above the base must be capped at it, got %d", capped.DualWieldPenaltyFloor)
	}
}
