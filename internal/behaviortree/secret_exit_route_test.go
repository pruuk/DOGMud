package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// #781, owner ruling 13: a planner's pathto is the mob's own errand, so it is
// sent as `pathto found ...`; any other command passes as written.
func TestUnscriptedRoute(t *testing.T) {
	for in, want := range map[string]string{
		"pathto 12":    "pathto " + mobs.PathtoFoundOnly + " 12",
		"pathto 12 13": "pathto " + mobs.PathtoFoundOnly + " 12 13",
		"wander":       "wander",
		"pathto":       "pathto",
		"pathtoo 12":   "pathtoo 12",
		"go north":     "go north",
	} {
		if got := unscriptedRoute(in); got != want {
			t.Errorf("unscriptedRoute(%q) = %q, want %q", in, got, want)
		}
	}
}

// A walk-away (actFlee out of combat) heads home only through exits the mob
// knows.
func TestPickRetreatExit_SkipsAnUnfoundSecretExit(t *testing.T) {
	room := &rooms.Room{RoomId: 31, Exits: map[string]exit.RoomExit{
		"west": {RoomId: 40, Secret: true},
		"east": {RoomId: 41},
	}}
	mob := &mobs.Mob{HomeRoomId: 40}
	if got := pickRetreatExit(mob, room); got != "east" {
		t.Fatalf("retreat %q, want east: west is an unfound secret exit", got)
	}
	mob.Character.LearnSecretExit(31, "west")
	if got := pickRetreatExit(mob, room); got != "west" {
		t.Fatalf("retreat %q, want west (home, found)", got)
	}
}
