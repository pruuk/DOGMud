package items

import "testing"

// Procs are validated with the item's tree now (behaviortree checkProcNodes,
// item behaviour slice 3).
func TestItemReserveValidation(t *testing.T) {
	badReserve := &ItemSpec{ItemId: 999904, Name: "bad reserve", Type: Weapon, ReserveHealthPct: 1.5}
	if err := badReserve.Validate(); err == nil {
		t.Fatal("reserve pct > 1 accepted")
	}
}

func TestItemSpecBoundsValidation(t *testing.T) {
	valid := func(mut func(*ItemSpec)) *ItemSpec {
		spec := &ItemSpec{ItemId: 999910, Name: "bounds test", Type: Weapon}
		mut(spec)
		return spec
	}

	tests := []struct {
		name    string
		mut     func(*ItemSpec)
		wantErr bool
	}{
		// Reserve pct boundaries
		{"reserve 0 valid", func(s *ItemSpec) { s.ReserveHealthPct = 0 }, false},
		{"reserve 0.99 valid", func(s *ItemSpec) { s.ReserveHealthPct = 0.99 }, false},
		{"reserve exactly 1.0 invalid", func(s *ItemSpec) { s.ReserveHealthPct = 1.0 }, true},
		{"reserve negative invalid", func(s *ItemSpec) { s.ReserveHealthPct = -0.1 }, true},
		// Hunger fields
		{"hunger rounds -1 invalid", func(s *ItemSpec) { s.HungerRounds = -1 }, true},
		{"hunger rounds positive valid", func(s *ItemSpec) { s.HungerRounds = 50 }, false},
		{"hunger drain pct 1.0 invalid", func(s *ItemSpec) { s.HungerDrainPct = 1.0 }, true},
		{"hunger drain pct negative invalid", func(s *ItemSpec) { s.HungerDrainPct = -0.05 }, true},
		{"hunger drain pct 0.02 valid", func(s *ItemSpec) { s.HungerDrainPct = 0.02 }, false},
		// Mutation tick fields
		{"mutation interval -1 invalid", func(s *ItemSpec) { s.MutationTickInterval = -1 }, true},
		{"mutation chance negative invalid (interval 0)", func(s *ItemSpec) { s.MutationTickChance = -1 }, true},
		{"mutation chance 101 invalid (interval 0)", func(s *ItemSpec) { s.MutationTickChance = 101 }, true},
		{"mutation interval set, chance 0 invalid", func(s *ItemSpec) {
			s.MutationTickInterval = 10
			s.MutationTickChance = 0
		}, true},
		{"mutation interval set, chance 1 valid", func(s *ItemSpec) {
			s.MutationTickInterval = 10
			s.MutationTickChance = 1
		}, false},
		{"mutation rarity floor 10 valid", func(s *ItemSpec) { s.MutationRarityFloor = 10 }, false},
		{"mutation rarity floor 11 invalid", func(s *ItemSpec) { s.MutationRarityFloor = 11 }, true},
		{"mutation rarity floor negative invalid", func(s *ItemSpec) { s.MutationRarityFloor = -1 }, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := valid(tc.mut).Validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected valid, got: %v", err)
			}
		})
	}
}
