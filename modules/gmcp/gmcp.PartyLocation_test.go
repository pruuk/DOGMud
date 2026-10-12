package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// #492: a party member's Party.Vitals Location follows that member's own
// sight, like Room.Info (#252). Pitch dark gives nothing to tell.
func TestVitalsLocation_FollowsTheMembersOwnSight(t *testing.T) {
	cases := []struct {
		name string
		lamp int
		want string
	}{
		{"pitch dark", 0, ``},
		{"shapes", 30, `Cellar`},
		{"lit", 60, `Cellar`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viewer := roomSightFixture(t, tc.lamp)
			room := rooms.LoadRoom(viewer.Character.RoomId)
			require.NotNil(t, room)
			require.Equal(t, tc.want, vitalsLocation(viewer.Character, room))
		})
	}
}

// A room that failed to load gives no title and must not panic on the
// typed-nil visibility trap.
func TestVitalsLocation_NilRoom(t *testing.T) {
	viewer := roomSightFixture(t, 60)
	require.Equal(t, ``, vitalsLocation(viewer.Character, nil))
}
