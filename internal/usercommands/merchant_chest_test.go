package usercommands

import (
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/merchantchests"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Merchant chests through the real get command: a take from an unwatched
// chest succeeds and keeps the stolen mark; a take under a sharp-eyed
// merchant's nose is caught, takes nothing, and the chest is slammed shut.

const chestOwnerInstId = 401

// seedMerchantChest puts an open "strongbox" belonging to mob 2 (the seeded
// "Merchant" template) in room 1 holding a stolen sword and 10 gold, empties
// the room of every other observer, and returns the user and room. Undone
// by t.Cleanup.
func seedMerchantChest(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)

	t.Cleanup(merchantchests.SetForTest(merchantchests.Catalog{
		Settings: merchantchests.Settings{RestockInterval: `1 day`, RelockInterval: `2 hours`,
			SleepingPerceptionMult: 0.5, LockMin: 6, LockMax: 20, ItemsMin: 1, ItemsMax: 3},
		Generic: merchantchests.Chest{Name: `chest`, Description: `A chest.`},
		Chests:  []merchantchests.Chest{{MobId: 2, RoomId: room.RoomId, Name: `strongbox`}},
	}))

	for _, id := range room.GetMobs() {
		room.RemoveMob(id)
		id := id
		t.Cleanup(func() { room.AddMob(id) })
	}
	for _, uid := range room.GetPlayers() {
		if uid != user.UserId {
			room.RemovePlayer(uid)
			uid := uid
			t.Cleanup(func() { room.AddPlayer(uid) })
		}
	}

	// Lit, so eyes work on both sides of the contest (messaging.SightMult).
	origLamp := room.Lamp
	room.Lamp = rooms.LampPtr(60)
	t.Cleanup(func() { room.Lamp = origLamp })

	// SetUnlocked stamps the current round, and round 0 reads as "never
	// unlocked"; pin a real round for the open chest.
	origRound := util.GetRoundCount()
	util.SetRoundCount(10000)
	t.Cleanup(func() { util.SetRoundCount(origRound) })

	sword := items.New(10001) // Iron Sword
	sword.StolenFrom, sword.StolenFromMob = `Merchant`, 2
	lock := gamelock.Lock{Difficulty: 6, RotationSeed: 1, RelockInterval: `100 rounds`}
	lock.SetUnlocked()
	room.Containers = map[string]rooms.Container{
		`strongbox`: {Lock: lock, Items: []items.Item{sword}, Gold: 10},
	}
	user.Character.Items = nil
	user.Character.Gold = 0
	return user, room
}

// addWatchfulOwner stands the chest's merchant in the room with Perception
// no thief can beat, and turns off the contest floor (Balance.ContestFloor
// gives every thief a small chance however outmatched) so the catch is
// certain.
func addWatchfulOwner(t *testing.T, room *rooms.Room) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.ContestFloor = 0
	configs.SetConfigForTest(t, cfg)
	owner := &mobs.Mob{
		MobId:        2,
		InstanceId:   chestOwnerInstId,
		HomeRoomId:   room.RoomId,
		Zone:         `TestZone`,
		NonCombatant: true,
		Character: characters.Character{
			Name:         `Merchant`,
			RoomId:       room.RoomId,
			Conditions:   conditions.New(),
			Cooldowns:    map[string]int{},
			NonCombatant: true,
			Shop:         characters.Shop{{ItemId: 10001}},
		},
	}
	owner.Character.HealthMax.Value = 100
	owner.Character.Health = 100
	owner.Character.Stats.Perception.ValueAdj = 1000000
	mobs.SetInstanceForTest(chestOwnerInstId, owner)
	room.AddMob(chestOwnerInstId)
	t.Cleanup(func() {
		room.RemoveMob(chestOwnerInstId)
		mobs.SetInstanceForTest(chestOwnerInstId, nil)
	})
}

func TestGet_MerchantChest_UnwatchedTakeKeepsStolenMark(t *testing.T) {
	user, room := seedMerchantChest(t)

	handled, err := Get(`sword strongbox`, user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Len(t, user.Character.Items, 1, "nobody watching: the take succeeds")
	taken := user.Character.Items[0]
	assert.True(t, taken.IsStolen(), "taken out of the chest, the merchant's goods are stolen")
	assert.Equal(t, `Merchant`, taken.StolenFrom)
	assert.Equal(t, user.UserId, taken.StolenBy, "the thief is on record")
	assert.Equal(t, room.Zone, taken.StolenZone, "hot in the area it was taken in")
	assert.Positive(t, taken.StolenAt)

	_, _ = Get(`gold strongbox`, user, room, 0)
	assert.Equal(t, 10, user.Character.Gold)
}

func TestGet_MerchantChest_CaughtTakesNothingAndChestIsSlammedShut(t *testing.T) {
	user, room := seedMerchantChest(t)
	addWatchfulOwner(t, room)

	handled, err := Get(`all strongbox`, user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	assert.Empty(t, user.Character.Items, "caught: nothing taken, and the rest of `get all` is refused")
	assert.Equal(t, 0, user.Character.Gold)
	c := room.Containers[`strongbox`]
	assert.Len(t, c.Items, 1)
	assert.Equal(t, 10, c.Gold)
	assert.True(t, c.Lock.IsLocked(), "the merchant slams the chest shut and locks it")
	assert.Equal(t, uint64(2), c.Lock.RotationSeed, "locking rotates the combination")
}

func TestGet_PlainContainerIsNeverWatched(t *testing.T) {
	user, room := seedMerchantChest(t)
	addWatchfulOwner(t, room)
	room.Containers[`crate`] = rooms.Container{Items: []items.Item{items.New(10001)}}

	_, _ = Get(`sword crate`, user, room, 0)
	assert.Len(t, user.Character.Items, 1, "only merchant chests are watched")
}

func TestPut_IntoAMerchantChestIsRefused(t *testing.T) {
	user, room := seedMerchantChest(t)
	user.Character.Items = []items.Item{items.New(10001)}

	handled, err := Put(`sword strongbox`, user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	assert.Len(t, user.Character.Items, 1, "a merchant's chest takes nothing from strangers")
	assert.Len(t, room.Containers[`strongbox`].Items, 1)
}

func TestGet_FromAnOrdinaryContainerMarksNothing(t *testing.T) {
	user, room := seedMerchantChest(t)
	goods := items.New(10001)
	goods.StolenFrom, goods.StolenFromMob = `Merchant`, 2 // somehow outside its chest
	room.Containers[`crate`] = rooms.Container{Items: []items.Item{goods}}

	_, _ = Get(`sword crate`, user, room, 0)
	require.Len(t, user.Character.Items, 1)
	assert.False(t, user.Character.Items[0].IsStolen(), "whoever picks it up elsewhere is not made its thief")
}

func TestStorage_ComponentBagHotGoodsAreRefused(t *testing.T) {
	user, room := seedMerchantChest(t)
	ingot := items.New(10001)
	ingot.StolenFrom, ingot.StolenFromMob = `Merchant`, 2
	ingot.MarkTaken(user.UserId, room.Zone, time.Now().Add(-time.Hour))
	user.Character.ComponentItems = []items.Item{ingot}

	_, found, hot := storageFindAddable(user, `sword`, room)
	assert.False(t, found, "a hot component is not storable where it is hot")
	assert.Equal(t, ingot.ItemId, hot.ItemId, "and it is named in the refusal")
}
