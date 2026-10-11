package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/stretchr/testify/require"
)

// #480 playtest, owner ruling 2026-10-10: a crash-site boss's adds share its
// species (45), so PackFlee's species scatter made the boss flee its vault,
// dropping its windup, whenever an add died. Both bosses now carry
// pack_flee_immune. This drives PackFlee on the SHIPPED templates: a boss in
// combat, its add dies, and no scatter happens. The scatter line is the
// observable: PackFlee prints it only when it queued at least one flee.
func TestCrashSiteBosses_AnAddsDeathDoesNotScatterTheBoss(t *testing.T) {
	for _, tc := range []struct {
		name          string
		bossId, addId int
	}{
		{"Core Guardian", coreDischargeGuardianId, 9586},
		{"Warden-Prime", coreDischargeWardenId, 9585},
	} {
		t.Run(tc.name, func(t *testing.T) {
			drainEventQueueOnCleanup(t) // PackFlee queues mob commands
			room := seedFallbackRoom(t, 60, heatEyesConditionId)
			loadShippedCrashSiteBosses(t)

			boss := mobs.NewMobByIdFresh(mobs.MobId(tc.bossId), room.RoomId)
			require.NotNil(t, boss)
			require.True(t, boss.PackFleeImmune, "%s must carry pack_flee_immune", tc.name)
			boss.Character.RoomId = room.RoomId
			room.AddMob(boss.InstanceId)
			boss.Character.SetAggro(1, 0, characters.DefaultAttack)
			require.True(t, boss.Character.IsInCombat(), "fixture: the boss is fighting")

			add := mobs.NewMobByIdFresh(mobs.MobId(tc.addId), room.RoomId)
			require.NotNil(t, add)
			require.Equal(t, boss.Character.SpeciesId, add.Character.SpeciesId,
				"fixture: the add shares the boss's species, which is what made it a packmate")
			drainPlain(1)

			PackFlee(events.MobDeath{MobId: tc.addId, InstanceId: add.InstanceId, RoomId: room.RoomId})

			require.Zero(t, countContaining(drainPlain(1), "the others scatter"),
				"an add's death must not make %s flee", tc.name)
		})
	}
}
