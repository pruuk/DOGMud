package items

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

// The two slice 1 keys load from item YAML (lighting 5e, Rules 1 and 10).
func TestItemSpecLoadsBehaviorAndFixture(t *testing.T) {
	var spec ItemSpec
	if err := yaml.Unmarshal([]byte("itemid: 55\nname: Arch Lantern\nbehavior: dusk_to_dawn\nfixture: light\n"), &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Behavior != "dusk_to_dawn" || spec.Fixture != FixtureLight {
		t.Errorf("Behavior=%q Fixture=%q, want dusk_to_dawn, light", spec.Behavior, spec.Fixture)
	}
}

func TestValidateRefusesAnUnknownFixtureKind(t *testing.T) {
	for _, ok := range []string{"", FixtureLight, FixtureDarkness} {
		spec := ItemSpec{ItemId: 1, Name: "x", Value: 1, Fixture: ok}
		if err := spec.Validate(); err != nil {
			t.Errorf("fixture %q refused: %v", ok, err)
		}
	}
	spec := ItemSpec{ItemId: 1, Name: "x", Value: 1, Fixture: "lamp"}
	if err := spec.Validate(); err == nil || !strings.Contains(err.Error(), "fixture") {
		t.Errorf("fixture lamp: err = %v, want a fixture refusal", err)
	}
}

// IsFixture and HasBehavior read the template, so an item instance carrying
// an override spec (an enchanted or renamed copy) cannot shed either.
func TestIsFixtureAndHasBehaviorReadTheTemplate(t *testing.T) {
	t.Cleanup(SeedItemsForTest(map[int]*ItemSpec{
		55: {ItemId: 55, Name: "Arch Lantern", Fixture: FixtureLight, Behavior: "dusk_to_dawn"},
		56: {ItemId: 56, Name: "Plain Stone"},
	}))
	fixture := Item{ItemId: 55, Spec: &ItemSpec{ItemId: 55, Name: "Renamed"}}
	if !fixture.IsFixture() || !fixture.HasBehavior() {
		t.Errorf("an overridden fixture reads IsFixture=%v HasBehavior=%v, want both true", fixture.IsFixture(), fixture.HasBehavior())
	}
	plain := Item{ItemId: 56}
	if plain.IsFixture() || plain.HasBehavior() {
		t.Error("a plain item reads as a fixture or as treed")
	}
	if (Item{}).IsFixture() || (Item{}).HasBehavior() {
		t.Error("an empty item reads as a fixture or as treed")
	}
}

// The holder index (Rule 5) holds mobs and rooms, not items: each enters
// once, leaves when dropped, and lists in id order.
func TestHolderIndex(t *testing.T) {
	t.Cleanup(ResetHolderIndexForTest())
	IndexMobHolder(9)
	IndexMobHolder(3)
	IndexMobHolder(9)
	IndexMobHolder(0) // never a holder
	if got := MobHolders(); len(got) != 2 || got[0] != 3 || got[1] != 9 {
		t.Errorf("MobHolders = %v, want [3 9]", got)
	}
	DropMobHolder(3)
	if got := MobHolders(); len(got) != 1 || got[0] != 9 {
		t.Errorf("after DropMobHolder(3), MobHolders = %v, want [9]", got)
	}

	var indexed []int
	OnRoomHolderIndexed = func(roomId int) { indexed = append(indexed, roomId) }
	t.Cleanup(func() { OnRoomHolderIndexed = nil })
	IndexRoomHolder(4111)
	IndexRoomHolder(4111)
	IndexRoomHolder(5000)
	if len(indexed) != 2 || indexed[0] != 4111 || indexed[1] != 5000 {
		t.Errorf("OnRoomHolderIndexed saw %v, want [4111 5000]: once per room entering the index", indexed)
	}
	if got := RoomHolders(); len(got) != 2 || got[0] != 4111 || got[1] != 5000 {
		t.Errorf("RoomHolders = %v, want [4111 5000]", got)
	}
	DropRoomHolder(4111)
	IndexRoomHolder(4111)
	if len(indexed) != 3 {
		t.Errorf("a room re-entering the index was not reported again: %v", indexed)
	}
}
