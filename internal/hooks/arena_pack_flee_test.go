package hooks

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/stretchr/testify/require"
)

// 2026-10-17 pre-deploy playtest: every arena pit fighter is species 1, so
// PackFlee's species scatter treated them as one pack and printed "Sensing
// the death of their packmate, the others scatter!" when one fell, followed
// by a blocked flee. The pit fighters are rivals, so every one carries
// pack_flee_immune. Drives PackFlee on the SHIPPED templates: a fighter in
// combat, another dies, no scatter. The scatter line is the observable:
// PackFlee prints it only when it queued at least one flee.
func TestArenaFighters_ARivalsDeathDoesNotScatterThem(t *testing.T) {
	const deadId = 316 // Arena Brute
	for _, tc := range []struct {
		name string
		id   int
	}{
		{"Arena Brute", 316},
		{"Arena Cutthroat", 317},
		{"Arena Veteran", 323},
		{"Arena Champion", 324},
		{"Arena Veteran Heavy", 325},
	} {
		t.Run(tc.name, func(t *testing.T) {
			drainEventQueueOnCleanup(t) // PackFlee queues mob commands
			room := seedFallbackRoom(t, 60, heatEyesConditionId)
			loadShippedArenaFighters(t)

			fighter := mobs.NewMobByIdFresh(mobs.MobId(tc.id), room.RoomId)
			require.NotNil(t, fighter)
			require.True(t, fighter.PackFleeImmune, "%s must carry pack_flee_immune", tc.name)
			fighter.Character.RoomId = room.RoomId
			room.AddMob(fighter.InstanceId)
			fighter.Character.SetAggro(1, 0, characters.DefaultAttack)
			require.True(t, fighter.Character.IsInCombat(), "fixture: the fighter is fighting")

			dead := mobs.NewMobByIdFresh(mobs.MobId(deadId), room.RoomId)
			require.NotNil(t, dead)
			require.Equal(t, fighter.Character.SpeciesId, dead.Character.SpeciesId,
				"fixture: the dead fighter shares the species, which is what made it a packmate")
			drainPlain(1)

			PackFlee(events.MobDeath{MobId: deadId, InstanceId: dead.InstanceId, RoomId: room.RoomId})

			require.Zero(t, countContaining(drainPlain(1), "the others scatter"),
				"a rival's death must not make %s flee", tc.name)
		})
	}
}

func loadShippedArenaFighters(t *testing.T) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "_datafiles", "world", "dogmud", "mobs", "instance_arena")
	ms, err := fileloader.LoadAllFlatFiles[int, *mobs.Mob](dir)
	if err != nil {
		t.Fatalf("loading arena mobs: %v", err)
	}
	t.Cleanup(mobs.SeedMobsForTest(ms, map[int]*mobs.Mob{}))
}
