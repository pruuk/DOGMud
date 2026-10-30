package actions

import (
	"slices"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
)

// A MobActor's Skip list (the listeners an ambient line is over budget for,
// #317) joins the room line's excludes; a player actor still excludes only
// itself.
func TestRoomLineExcludes(t *testing.T) {
	if got := roomLineExcludes(&MobActor{Skip: []int{4, 5}}); !slices.Equal(got, []int{0, 4, 5}) {
		t.Errorf("mob with Skip: %v, want [0 4 5]", got)
	}
	if got := roomLineExcludes(&MobActor{}); !slices.Equal(got, []int{0}) {
		t.Errorf("mob without Skip: %v, want [0]", got)
	}
	u := users.NewTestUser(7, "excl", "Excl", 0)
	if got := roomLineExcludes(NewUserActor(u)); !slices.Equal(got, []int{7}) {
		t.Errorf("player: %v, want [7]", got)
	}
}
