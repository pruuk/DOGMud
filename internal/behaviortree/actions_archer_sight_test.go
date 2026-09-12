package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// archerMeleeEngaged reports that a fight ALREADY EXISTS: either this mob's
// aggro target stands here, or someone here is aggroed onto this mob. It
// acquires no target, so slice F deliberately does NOT gate it on sight.
//
// Gating it would mean a blind archer that has been closed on never notices it
// is pinned and keeps shooting point-blank instead of kiting, which contradicts
// owner ruling 4 (fights already under way continue) rather than implementing
// it. Same shape as the companion sweep, which is also not target acquisition.
func TestArcherMeleeEngagedIsNotSightGated(t *testing.T) {
	m, room := sightScene(t, "cave")
	u := users.NewTestUser(8140, "kesh", "Kesh", 98140)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8140: u}))
	room.AddPlayer(8140)
	u.Character.SetAggro(0, m.InstanceId, characters.DefaultAttack)

	require.True(t, archerMeleeEngaged(m),
		"a blind archer must still know it is pinned in melee")
}
