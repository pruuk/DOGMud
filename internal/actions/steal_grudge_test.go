package actions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/goals"
	_ "github.com/GoMudEngine/GoMud/internal/goals/catalog" // main.go's registration
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The steal's success point reaches the grudge. takeFromMob is where a mob
// thief's steal and a player's pickpocket reveal both land.

const grudgeMobId = 97400

func setupStealGrudge(t *testing.T, thiefUserId int) *mobs.Mob {
	t.Helper()
	prev, had := os.LookupEnv("DOGMUD_GOALS_DIR_OVERRIDE")
	os.Setenv("DOGMUD_GOALS_DIR_OVERRIDE", filepath.Join(t.TempDir(), "goals"))
	t.Cleanup(func() {
		if had {
			os.Setenv("DOGMUD_GOALS_DIR_OVERRIDE", prev)
		} else {
			os.Unsetenv("DOGMUD_GOALS_DIR_OVERRIDE")
		}
	})
	goals.ClearCache()
	t.Cleanup(goals.ClearCache)

	u := users.NewTestUser(thiefUserId, "thief", "Nimble", 0)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{thiefUserId: u}))
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{}, map[string]*rooms.ZoneConfig{}))

	m := newStealTestMob(97401, 40, 1)
	m.MobId = mobs.MobId(grudgeMobId)
	return m
}

func TestTakeFromMob_APlayersTheftGivesTheMarkItsOwnGrudge(t *testing.T) {
	thief := newStealPlayerActor(150, 5)
	m := setupStealGrudge(t, thief.userId)

	if res := takeFromMob(thief, m, []items.Item{}); !res.Succeeded {
		t.Fatalf("the theft did not land")
	}

	own := goals.IndividualGoalsOf(m)
	if len(own) != 1 || own[0].Type != "revenge-mob" {
		t.Fatalf("own goals = %+v, want one revenge-mob", own)
	}
	if id, _ := own[0].Params["target_id"].(int); id != thief.userId {
		t.Fatalf("grudge against %d, want the thief %d", id, thief.userId)
	}
	for _, g := range goals.GoalsOf(grudgeMobId, "bandit") {
		if g.Type == "revenge-mob" {
			t.Fatalf("the type's file holds the grudge: %+v", g)
		}
	}
}

// A pocket with nothing in it to take names no theft, so it starts no grudge.
func TestTakeFromMob_AnEmptyPocketStartsNoGrudge(t *testing.T) {
	thief := newStealPlayerActor(150, 5)
	m := setupStealGrudge(t, thief.userId)
	m.Character.Gold = 0

	takeFromMob(thief, m, nil)

	if own := goals.IndividualGoalsOf(m); len(own) != 0 {
		t.Fatalf("own goals = %+v, want none", own)
	}
}

// OnTheft is reached for a player thief only: a grudge is against a user id, and
// a mob thief has none. Stated here so a change to that is a deliberate one.
func TestTakeFromMob_AMobThiefStartsNoGrudge(t *testing.T) {
	m := setupStealGrudge(t, 1001)

	if res := takeFromMob(newStealMobActor(150, 5), m, nil); !res.Succeeded {
		t.Fatalf("the theft did not land")
	}

	if own := goals.IndividualGoalsOf(m); len(own) != 0 {
		t.Fatalf("own goals = %+v, want none", own)
	}
}
