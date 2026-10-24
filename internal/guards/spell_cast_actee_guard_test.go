package guards

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// castActeeExempt are the single-target spells that author no cast_actee:
// four act on the caster or summon (they target no one), and heal authors no
// cast text at all (messaging M6 slice 4).
var castActeeExempt = map[string]bool{
	"fold-anchor":          true,
	"fold-recall":          true,
	"summon-hive-swarm":    true,
	"summon-steppe-spirit": true,
	"heal":                 true,
}

// TestCastActeeCoversEveryAimedSpell: every single-target spell that can land
// on someone else authors cast_actee, and no other spell does.
func TestCastActeeCoversEveryAimedSpell(t *testing.T) {
	loaded, err := fileloader.LoadAllFlatFiles[string, *spells.SpellData](shippedWorldRoot + "/spells")
	if err != nil {
		t.Fatalf("load spells: %v", err)
	}
	authored := 0
	for id, s := range loaded {
		want := s.Targeting == combatvocab.TargetSingle && !castActeeExempt[id]
		has := len(s.Narration(spells.PhaseCast).Actee) > 0
		switch {
		case want && !has:
			t.Errorf("spell %q is aimed at one target but authors no cast_actee", id)
		case !want && has:
			t.Errorf("spell %q authors cast_actee but has no one target to address", id)
		}
		if has {
			authored++
		}
	}
	for id := range castActeeExempt {
		if _, ok := loaded[id]; !ok {
			t.Errorf("exempt spell %q no longer exists; drop it from castActeeExempt", id)
		}
	}
	if authored == 0 {
		t.Fatal("no spell authors cast_actee: the guard is checking nothing")
	}
}
