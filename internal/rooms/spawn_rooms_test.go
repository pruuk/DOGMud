package rooms

import (
	"reflect"
	"testing"
)

func TestSpawnRooms_IndexReplaceAndDrop(t *testing.T) {
	t.Cleanup(func() {
		deleteTemplateSpawns(920001)
		deleteTemplateSpawns(920002)
	})
	setTemplateSpawns(920001, []SpawnInfo{{MobId: 9901}, {MobId: 9902}, {MobId: 9901}, {ItemId: 5}})
	setTemplateSpawns(920002, []SpawnInfo{{MobId: 9901}})
	if got := SpawnRooms(9901); !reflect.DeepEqual(got, []int{920001, 920002}) {
		t.Fatalf("SpawnRooms(9901) = %v", got)
	}
	shared := SpawnRooms(9901)

	// A builder save replaces the room's spawns.
	setTemplateSpawns(920001, []SpawnInfo{{MobId: 9902}})
	if got := SpawnRooms(9901); !reflect.DeepEqual(got, []int{920002}) {
		t.Errorf("after the replace SpawnRooms(9901) = %v", got)
	}
	if !reflect.DeepEqual(shared, []int{920001, 920002}) {
		t.Errorf("a slice handed out earlier changed under its reader: %v", shared)
	}

	deleteTemplateSpawns(920002)
	if got := SpawnRooms(9901); got != nil {
		t.Errorf("after the delete SpawnRooms(9901) = %v, want nil", got)
	}
	if got := SpawnRooms(9902); !reflect.DeepEqual(got, []int{920001}) {
		t.Errorf("SpawnRooms(9902) = %v", got)
	}
}

func TestSpawnRooms_SkipsEphemeralRooms(t *testing.T) {
	id := ephemeralRoomIdMinimum + 7
	setTemplateSpawns(id, []SpawnInfo{{MobId: 9903}})
	t.Cleanup(func() { deleteTemplateSpawns(id) })
	if got := SpawnRooms(9903); got != nil {
		t.Errorf("an ephemeral room was indexed: %v", got)
	}
}

func TestSpawnRooms_FollowsDeleteRoomTemplate(t *testing.T) {
	const roomId = 920003
	restore := setupPrepareRoom(t, roomId)
	defer restore()
	setTemplateZone(roomId, "preparezone")
	setTemplateSpawns(roomId, []SpawnInfo{{MobId: 9904}})
	t.Cleanup(func() { deleteTemplateZone(roomId); deleteTemplateSpawns(roomId) })

	if err := DeleteRoomTemplate(roomId); err != nil {
		t.Fatalf("DeleteRoomTemplate: %v", err)
	}
	if got := SpawnRooms(9904); got != nil {
		t.Errorf("a deleted room still spawns: %v", got)
	}
}
