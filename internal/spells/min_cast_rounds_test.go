package spells

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
)

// min_cast_rounds (#480) is validated at load: a negative count is a typo, and
// a player's cast ignores the field, so on a spell players can learn it would
// read as a promise the engine does not keep.
func TestValidate_MinCastRounds(t *testing.T) {
	base := func(minRounds int, mobOnly bool) *SpellData {
		return &SpellData{SpellId: "blast", PrimaryStat: "willpower", AttackType: combatvocab.AttackSpell,
			DamageType: combatvocab.DamagePhysical, Targeting: combatvocab.TargetArea,
			MobOnly: mobOnly, MinCastRounds: minRounds}
	}
	if err := base(2, true).Validate(); err != nil {
		t.Errorf("min_cast_rounds 2 on a mob_only spell must load: %v", err)
	}
	if err := base(0, false).Validate(); err != nil {
		t.Errorf("an absent min_cast_rounds must load on any spell: %v", err)
	}
	for _, tc := range []struct {
		name    string
		sd      *SpellData
		mention string
	}{
		{"negative", base(-1, true), "min_cast_rounds"},
		{"player-castable", base(2, false), "mob_only"},
	} {
		err := tc.sd.Validate()
		if err == nil || !strings.Contains(err.Error(), tc.mention) {
			t.Errorf("%s: Validate() = %v, want an error naming %q", tc.name, err, tc.mention)
		}
	}
}
