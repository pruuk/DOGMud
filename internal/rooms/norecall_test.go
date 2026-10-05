package rooms

import "testing"

func TestNoRecall(t *testing.T) {
	restore := SeedRoomsForTest(map[int]*Room{71: {RoomId: 71}, 72: {RoomId: 72}}, map[string]*ZoneConfig{})
	defer restore()
	LoadRoom(71).SetTempData(`allow_recall`, false)
	if !NoRecall(71) {
		t.Fatal("a room with allow_recall false keeps its occupants in")
	}
	if NoRecall(72) || NoRecall(99999) {
		t.Fatal("an ordinary or missing room does not")
	}
}
