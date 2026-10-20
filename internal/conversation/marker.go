package conversation

import (
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func init() {
	rooms.NpcMarker = Marker
}

// Marker is the talk marker viewerUserId sees on mob: [?], [!], [*] or "".
// The roster and GMCP call it only for viewers who see the mob by name.
func Marker(viewerUserId int, mob *mobs.Mob) string {
	user := users.GetByUserId(viewerUserId)
	if user == nil || mob == nil || mob.Character.IsCharmed() {
		return ""
	}
	return dialogue.Load(int(mob.MobId), mob.Zone).Marker(PlayerState(user))
}
