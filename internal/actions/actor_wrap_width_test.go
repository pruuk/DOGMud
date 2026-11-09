package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func TestActorWrapWidth(t *testing.T) {
	u := users.NewTestUser(8890, "reader", "Reader", 98890)
	u.LineWidth = 50
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8890: u}))

	require.Equal(t, 50, actorWrapWidth(&UserActor{User: u}))
	require.Equal(t, 80, actorWrapWidth(&MobActor{Mob: &mobs.Mob{}}), "a mob has no reader; its SendText is a no-op")
}
