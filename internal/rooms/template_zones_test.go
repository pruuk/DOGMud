package rooms

import "testing"

// A room added to memory joins the template zone index; an ephemeral id
// never does.
func TestTemplateZoneIndex(t *testing.T) {
	room := &Room{RoomId: 987650, Zone: "Index Test Zone"}
	if err := addRoomToMemory(room); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		removeRoomFromMemory(room)
		templateZoneMu.Lock()
		delete(templateZone, room.RoomId)
		templateZoneMu.Unlock()
	})
	if got := TemplateRoomZone(987650); got != "Index Test Zone" {
		t.Errorf("TemplateRoomZone = %q", got)
	}
	if got := TemplateRoomZones()[987650]; got != "Index Test Zone" {
		t.Errorf("TemplateRoomZones()[987650] = %q", got)
	}
	setTemplateZone(ephemeralRoomIdMinimum+1, "Index Test Zone")
	if got := TemplateRoomZone(ephemeralRoomIdMinimum + 1); got != "" {
		t.Errorf("ephemeral id indexed as %q", got)
	}
}
