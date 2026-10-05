package housing

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/fileloader"
)

// The shipped buildings load and pass their own validation. World checks
// (rooms, mob and faction exist; units lead back to the door) run in the
// real-data boot smoke test, which calls LoadDataFiles through
// loadAllDataFiles.
func TestShippedBuildingsValidate(t *testing.T) {
	loaded, err := fileloader.LoadAllFlatFiles[string, Building](`../../_datafiles/world/dogmud/housing_buildings`)
	if err != nil {
		t.Fatalf("shipped buildings: %v", err)
	}
	b, ok := loaded[`back_court_lodgings`]
	if !ok {
		t.Fatal("back_court_lodgings not found")
	}
	simple, ok := b.Tier(`simple`)
	if !ok || simple.Price != 500 || simple.Rooms != 1 {
		t.Errorf("simple tier = %+v, want 500 gold for one room", simple)
	}
	if b.Faction != `np_commonfolk` || b.MinRepTier != `warm` {
		t.Errorf("standing gate = %s/%s", b.Faction, b.MinRepTier)
	}
}
