package conversation

import (
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func init() {
	rooms.NpcMarker = Marker
}

// Marker is the talk marker viewerUserId sees on mob: [?], [!], [*] or "".
// Callers ask only for viewers who see the mob by name (sight Full). A mob
// with no dialogue file costs one cache lookup. Charmed NPCs offer no
// topics, so they carry no marker.
func Marker(viewerUserId int, mob *mobs.Mob) string {
	if mob == nil || mob.Character.IsCharmed() {
		return ""
	}
	df := dialogue.Load(int(mob.MobId), mob.Zone)
	if df == nil {
		return ""
	}
	user := users.GetByUserId(viewerUserId)
	if user == nil {
		return ""
	}
	return df.Marker(PlayerState(user))
}

// RefreshMarkers tells the player's GMCP client to re-read the NPC list in
// its room, because the markers on it may have changed.
func RefreshMarkers(userId int) {
	if userId < 1 {
		return
	}
	events.AddToQueue(events.NpcMarkersChanged{UserId: userId})
}
