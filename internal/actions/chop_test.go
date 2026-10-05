package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/timber"
)

func TestChopTargetAndRounds_RiseWithTier(t *testing.T) {
	if !(ChopTarget(1) < ChopTarget(2) && ChopTarget(2) < ChopTarget(4)) {
		t.Error("rarer trees are harder to fell")
	}
	if ChopRounds(4) <= ChopRounds(1) {
		t.Error("rarer trees take longer")
	}
}

func TestLogsFor(t *testing.T) {
	if n := LogsFor(100, items.QualityStandard); n != 1 {
		t.Errorf("a baseline woodcutter gets one log, got %d", n)
	}
	if LogsFor(200, items.QualityFine) <= LogsFor(100, items.QualityFine) {
		t.Error("strength adds logs")
	}
	if n := LogsFor(10000, items.QualityPristine); n > 4 {
		t.Errorf("capped by TimberMaxLogs, got %d", n)
	}
}

func TestRoomStand_SeedsAndSkipsNonTimberRooms(t *testing.T) {
	d, err := timber.Parse([]byte("species:\n  - {id: pine, name: pine, log: 1, tier: 1}\nbiomes:\n  forest: [{species: pine, weight: 1}]\n"), timber.World{})
	if err != nil {
		t.Fatal(err)
	}
	timber.Install(d)
	defer timber.Install(nil)

	city := newSalvageTestRoom(t, 9601)
	city.Biome = `city_thoroughfare`
	if _, _, ok := RoomStand(city, 100); ok {
		t.Error("a city street has no stand")
	}

	wood := newSalvageTestRoom(t, 9602)
	wood.Biome = `forest`
	st, sp, ok := RoomStand(wood, 100)
	if !ok || sp == nil || sp.Id != `pine` || st.Stock < 1 {
		t.Fatalf("forest room should seed a pine stand, got %+v %+v %v", st, sp, ok)
	}
	again, _, _ := RoomStand(wood, 100)
	if again != st {
		t.Error("the stand persists on the room between calls")
	}
}
