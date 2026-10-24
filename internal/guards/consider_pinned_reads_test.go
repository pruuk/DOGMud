package guards

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// Drift guard 4 (#301): reads the owner named, pinned.

// The Straw Effigy has vast health and pillow hits (species 19, zero base
// damage). It must never read as a quick kill either way: your pace is an
// age, and the verdict is "won't lay a finger" or a stalemate, never "zero
// chance" and never a loss.
func TestPinnedRead_StrawEffigy(t *testing.T) {
	loadFightWorld(t)
	effigy := mobs.NewMobByIdFresh(mobs.MobId(9614), 1)
	if effigy == nil {
		t.Fatal("Straw Effigy (9614) failed to spawn")
	}
	novice := fixtureAdventurer(t, "Novice", 100, 10)

	r := actions.ReadFight(novice, &effigy.Character)
	t.Logf("novice vs effigy: verdict %v, your pace %v, their pace %v (prediction %+v)",
		r.Verdict, r.YourPace, r.TheirPace, r.Prediction)
	if r.Verdict != actions.VerdictUntouched && r.Verdict != actions.VerdictStalemate {
		t.Errorf("effigy verdict %v, want untouched or stalemate (prediction %+v)", r.Verdict, r.Prediction)
	}
	if r.YourPace != actions.PaceAge {
		t.Errorf("effigy: your pace %v, want an age", r.YourPace)
	}
}

// fixtureAscended is the owner's "zero chance" yardstick: species 99
// (Ascended, base 1000 in every stat, hydrated by Validate), every skill at
// 100, wielding the Blackrazor (item 40183).
func fixtureAscended(t *testing.T) *characters.Character {
	t.Helper()
	c := characters.New()
	c.Name = "Ascended"
	c.RoomId = 1
	c.SpeciesId = 99
	// characters.New rolls random stats, and Validate hydrates only a zero
	// base from the species, so clear the six bases first.
	for _, s := range []*int{&c.Stats.Strength.Base, &c.Stats.Dexterity.Base, &c.Stats.Perception.Base,
		&c.Stats.Vitality.Base, &c.Stats.Willpower.Base, &c.Stats.Charisma.Base} {
		*s = 0
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("validate Ascended: %v", err)
	}
	if c.Stats.Strength.ValueAdj < 1000 {
		t.Fatalf("Ascended strength %d: species stats did not hydrate", c.Stats.Strength.ValueAdj)
	}
	for _, s := range skills.GetAllSkillNames() {
		c.Skills[string(s)] = 100
	}
	c.Equipment.Weapon = items.New(40183)
	if c.Equipment.Weapon.ItemId != 40183 {
		t.Fatal("the Blackrazor (40183) did not load")
	}
	c.Health = c.HealthMax.Value
	c.Stamina = c.StaminaMax.Value
	c.Conviction = c.ConvictionMax.Value
	return c
}

// An Ascended with every skill at 100 and the Blackrazor against a novice:
// zero chance for the novice, and moments for the Ascended.
func TestPinnedRead_Hopeless(t *testing.T) {
	loadFightWorld(t)
	novice := fixtureAdventurer(t, "Novice", 60, 1)
	pinnacle := fixtureAscended(t)

	r := actions.ReadFight(novice, pinnacle)
	t.Logf("novice vs pinnacle: verdict %v, your pace %v, their pace %v (prediction %+v)",
		r.Verdict, r.YourPace, r.TheirPace, r.Prediction)
	if r.Verdict != actions.VerdictHopeless {
		t.Errorf("novice vs pinnacle: %v, want hopeless (prediction %+v)", r.Verdict, r.Prediction)
	}
	r = actions.ReadFight(pinnacle, novice)
	t.Logf("pinnacle vs novice: verdict %v, your pace %v, their pace %v (prediction %+v)",
		r.Verdict, r.YourPace, r.TheirPace, r.Prediction)
	if r.Verdict != actions.VerdictUntouched || r.YourPace != actions.PaceMoments {
		t.Errorf("pinnacle vs novice: %v / %v, want untouched in moments (prediction %+v)", r.Verdict, r.YourPace, r.Prediction)
	}
}
