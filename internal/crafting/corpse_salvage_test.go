package crafting

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/species"
)

func TestLookupCorpseSalvage_Animal(t *testing.T) {
	got := LookupCorpseSalvage([]string{"animal", "canine", "predator"})
	want := []items.SalvageReturn{
		{ItemTag: "raw-meat", Quantity: 1},
		{ItemTag: "leather-strip", Quantity: 2},
		{ItemTag: "sinew", Quantity: 1},
	}
	if !equalReturns(got, want) {
		t.Errorf("animal: got %+v, want %+v", got, want)
	}
}

func TestLookupCorpseSalvage_Humanoid(t *testing.T) {
	got := LookupCorpseSalvage([]string{"bandit", "humanoid"})
	want := []items.SalvageReturn{
		{ItemTag: "cloth-strip", Quantity: 2},
		{ItemTag: "leather-strip", Quantity: 1},
	}
	if !equalReturns(got, want) {
		t.Errorf("humanoid: got %+v, want %+v", got, want)
	}
}

func TestLookupCorpseSalvage_NoMatch(t *testing.T) {
	got := LookupCorpseSalvage([]string{"chrysalis", "elemental"})
	if got != nil {
		t.Errorf("no-match: got %+v, want nil", got)
	}
}

func TestLookupCorpseSalvage_EmptyGroups(t *testing.T) {
	got := LookupCorpseSalvage(nil)
	if got != nil {
		t.Errorf("nil groups: got %+v, want nil", got)
	}
	got = LookupCorpseSalvage([]string{})
	if got != nil {
		t.Errorf("empty groups: got %+v, want nil", got)
	}
}

func TestLookupCorpseSalvage_FirstTableEntryWins(t *testing.T) {
	// table order: rodent, animal, humanoid — if a mob has both
	// animal and humanoid groups, the animal entry wins.
	got := LookupCorpseSalvage([]string{"humanoid", "animal"})
	want := []items.SalvageReturn{
		{ItemTag: "raw-meat", Quantity: 1},
		{ItemTag: "leather-strip", Quantity: 2},
		{ItemTag: "sinew", Quantity: 1},
	}
	if !equalReturns(got, want) {
		t.Errorf("multi-group: got %+v, want %+v", got, want)
	}
}

func TestLookupCorpseSalvage_AnimalYieldsRawMeat(t *testing.T) {
	got := LookupCorpseSalvage([]string{"animal", "predator"})
	tags := map[string]int{}
	for _, r := range got {
		tags[r.ItemTag] = r.Quantity
	}
	if tags["raw-meat"] < 1 {
		t.Errorf("animal corpse should yield raw-meat, got %v", got)
	}
}

func TestLookupCorpseSalvage_SmallGameYieldsHareMeat(t *testing.T) {
	got := LookupCorpseSalvage([]string{"animal", "rodent", "prey"})
	tags := map[string]int{}
	for _, r := range got {
		tags[r.ItemTag] = r.Quantity
	}
	if tags["wild-hare-meat"] < 1 {
		t.Errorf("small-game corpse should yield wild-hare-meat, got %v", got)
	}
}

func equalReturns(a, b []items.SalvageReturn) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ItemTag != b[i].ItemTag || a[i].Quantity != b[i].Quantity {
			return false
		}
	}
	return true
}

// A steppe wolf is grouped `steppe-wolf`/`canine`, which no table row names.
// The species fallback must still give it the game returns.
func TestLookupCorpseSalvageFor_SpeciesFallback(t *testing.T) {
	got := LookupCorpseSalvageFor([]string{"steppe-wolf", "canine"}, "Canine")
	if !equalReturns(got, corpseSalvageTable[1].Returns) {
		t.Errorf("canine fallback = %v, want animal returns", got)
	}
}

// Groups win over species: an authored `humanoid` tag on a canine-species mob
// (a werewolf, say) keeps the humanoid returns.
func TestLookupCorpseSalvageFor_GroupsWin(t *testing.T) {
	got := LookupCorpseSalvageFor([]string{"humanoid"}, "canine")
	if !equalReturns(got, corpseSalvageTable[2].Returns) {
		t.Errorf("groups should win, got %v", got)
	}
}

// Cold-blooded and chitinous species stay unsalvageable until phase 2.
func TestLookupCorpseSalvageFor_NoFallbackForInsects(t *testing.T) {
	for _, sp := range []string{"insectoid", "arachnid", "reptile", "fish", "worm", ""} {
		if got := LookupCorpseSalvageFor([]string{"vermin"}, sp); got != nil {
			t.Errorf("species %q should have no fallback, got %v", sp, got)
		}
	}
}

func TestLookupCorpseSalvageForMob_ResolvesSpeciesId(t *testing.T) {
	restore := species.SeedSpeciesForTest(map[int]*species.Species{
		2:  {SpeciesId: 2, Name: "canine"},
		12: {SpeciesId: 12, Name: "insectoid"},
	})
	defer restore()

	if got := LookupCorpseSalvageForMob([]string{"beast", "pack"}, 2); len(got) == 0 {
		t.Error("beast-grouped canine should be salvageable")
	}
	if got := LookupCorpseSalvageForMob([]string{"beast", "cave"}, 12); got != nil {
		t.Errorf("beast-grouped insectoid should not be salvageable yet, got %v", got)
	}
	if got := LookupCorpseSalvageForMob([]string{"beast"}, 999); got != nil {
		t.Errorf("unknown species should not be salvageable, got %v", got)
	}
}
