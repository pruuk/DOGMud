package rooms

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// A builder's move takes a template room's zone with it.
func TestTemplateZoneIndex_FollowsMoveToZone(t *testing.T) {
	const roomId = 910101
	restore := setupPrepareRoom(t, roomId)
	defer restore()
	base := configs.GetFilePathsConfig().DataFiles.String()
	for _, d := range []string{"rooms", "rooms.instances"} {
		if err := os.MkdirAll(filepath.Join(base, d, "movezone"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	prevOld, prevNew := roomManager.zones["preparezone"], roomManager.zones["movezone"]
	roomManager.zones["preparezone"] = &ZoneConfig{Name: "preparezone", RoomId: 1, RoomIds: map[int]struct{}{roomId: {}}}
	roomManager.zones["movezone"] = &ZoneConfig{Name: "movezone", RoomId: 2, RoomIds: map[int]struct{}{}}
	t.Cleanup(func() {
		restoreZone("preparezone", prevOld)
		restoreZone("movezone", prevNew)
		deleteTemplateZone(roomId)
	})
	setTemplateZone(roomId, "preparezone")

	if err := MoveToZone(roomId, "movezone"); err != nil {
		t.Fatalf("MoveToZone: %v", err)
	}
	if got := TemplateRoomZone(roomId); got != "movezone" {
		t.Errorf("after the move TemplateRoomZone = %q, want movezone", got)
	}
}

func restoreZone(name string, z *ZoneConfig) {
	if z == nil {
		delete(roomManager.zones, name)
		return
	}
	roomManager.zones[name] = z
}

// A deleted template room leaves the index.
func TestTemplateZoneIndex_FollowsDeleteRoomTemplate(t *testing.T) {
	const roomId = 910102
	restore := setupPrepareRoom(t, roomId)
	defer restore()
	setTemplateZone(roomId, "preparezone")
	t.Cleanup(func() { deleteTemplateZone(roomId) })

	if err := DeleteRoomTemplate(roomId); err != nil {
		t.Fatalf("DeleteRoomTemplate: %v", err)
	}
	if got := TemplateRoomZone(roomId); got != "" {
		t.Errorf("a deleted room is still indexed in %q", got)
	}
}

// A zone rename moves every template room of the old name, and only those.
func TestTemplateZoneIndex_Rename(t *testing.T) {
	setTemplateZone(910103, "Old Index Zone")
	setTemplateZone(910104, "Old Index Zone")
	setTemplateZone(910105, "Other Index Zone")
	t.Cleanup(func() {
		for _, id := range []int{910103, 910104, 910105} {
			deleteTemplateZone(id)
		}
	})
	renameTemplateZone("Old Index Zone", "New Index Zone")
	for id, want := range map[int]string{910103: "New Index Zone", 910104: "New Index Zone", 910105: "Other Index Zone"} {
		if got := TemplateRoomZone(id); got != want {
			t.Errorf("room %d: zone %q, want %q", id, got, want)
		}
	}
}

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
