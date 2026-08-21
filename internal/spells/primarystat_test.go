package spells

import "testing"

func TestPrimaryStat_RequiredAndValid(t *testing.T) {
	cases := []struct {
		name    string
		stat    string
		wantErr bool
	}{
		{"valid willpower", "willpower", false},
		{"valid charisma", "charisma", false},
		{"empty is now an error", "", true},
		{"typo", "willpwer", true},
		{"not a stat", "manifestation", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := SpellData{SpellId: "test", Name: "Test", PrimaryStat: tc.stat}
			err := s.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
