package guards

import (
	"sort"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
)

// TestEveryMobSpawnsWithFullPools spawns every mob template in the shipped
// world and requires each pool to be full, which for a reserved pool means its
// ceiling. A spawn that fills its pools before something later raises the max
// reads as already wounded to the first player who sees it (#598).
func TestEveryMobSpawnsWithFullPools(t *testing.T) {
	templates := loadFightWorld(t)
	if len(templates) < 100 {
		t.Fatalf("loaded only %d mob templates; the world did not load", len(templates))
	}

	ids := make([]int, 0, len(templates))
	for id := range templates {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	short := 0
	for _, id := range ids {
		m := mobs.NewMobByIdFresh(mobs.MobId(id), 1)
		if m == nil {
			t.Errorf("mob %d did not spawn", id)
			continue
		}
		c := &m.Character
		if m.HealthMaxOverride > 0 && c.HealthMax.Value != m.HealthMaxOverride {
			t.Errorf("mob %d %q authors health_max %d and spawns with max %d", id, c.Name, m.HealthMaxOverride, c.HealthMax.Value)
		}
		if m.StaminaMaxOverride > 0 && c.StaminaMax.Value != m.StaminaMaxOverride {
			t.Errorf("mob %d %q authors stamina_max %d and spawns with max %d", id, c.Name, m.StaminaMaxOverride, c.StaminaMax.Value)
		}
		for _, p := range []characters.Pool{characters.PoolHealth, characters.PoolStamina, characters.PoolConviction} {
			if got, want := c.PoolValue(p), c.PoolCeiling(p); got != want {
				short++
				t.Errorf("mob %d %q spawns with %s %d of %d (%d%%), mutations %v",
					id, c.Name, p, got, want, 100*got/max(want, 1), c.Mutations)
			}
		}
	}
	t.Logf("spawned %d templates, %d pools short", len(ids), short)
}
