package rifts

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const (
	testSword  = 50001
	testHelm   = 50002
	testBread  = 50003
	testDoorKy = 50004
)

// lossWorld seeds the shipped profiles, a user in a run, and a few items:
// a sword and bread in the pack, a house key, and a worn helm.
func lossWorld(t *testing.T) (*users.UserRecord, *Run) {
	t.Helper()
	u := setupRuntime(t)
	p := GetProfile(`obelisk`)
	specs := map[int]*items.ItemSpec{
		p.KeyItemId:   {ItemId: p.KeyItemId, Name: `Facet Key`, Type: items.Object, Subtype: items.Mundane},
		p.LightItemId: {ItemId: p.LightItemId, Name: `Glowstone`, Type: items.Light, Subtype: items.Wearable},
		testSword:     {ItemId: testSword, Name: `Plain Sword`, Type: items.Weapon, Subtype: items.Generic},
		testHelm:      {ItemId: testHelm, Name: `Iron Helm`, Type: items.Head, Subtype: items.Wearable},
		testBread:     {ItemId: testBread, Name: `Loaf of Bread`, Type: items.Food, Subtype: items.Edible},
		testDoorKy:    {ItemId: testDoorKy, Name: `House Key`, Type: items.Key, Subtype: items.Mundane},
	}
	t.Cleanup(items.SeedItemsForTest(specs))
	saved := lostItems
	lostItems = nil
	t.Cleanup(func() { lostItems = saved; SetLostChanged(nil) })

	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	walk(t, u, run.EntryRoomId)
	for _, id := range []int{testSword, testBread, testDoorKy} {
		require.True(t, u.Character.StoreItem(items.New(id)))
	}
	_, worn, why := u.Character.Wear(items.New(testHelm))
	require.True(t, worn, why)
	u.Character.Gold = 120
	return u, run
}

func withRng(t *testing.T, fn func(int) int) {
	t.Helper()
	saved := rng
	rng = fn
	t.Cleanup(func() { rng = saved })
}

func names(list []items.Item) string {
	var out []string
	for _, i := range list {
		out = append(out, i.GetSpec().Name)
	}
	return strings.Join(out, `,`)
}

// Death keeps the pack, the gold and (at the chance) the worn pieces; keys
// stay with the player; everything taken goes into the record.
func TestLosses_Death(t *testing.T) {
	u, run := lossWorld(t)
	changed := 0
	SetLostChanged(func() { changed++ })
	withRng(t, func(n int) int { return 0 }) // every worn piece is lost

	OnPlayerDeath(u.UserId, u.Character.RoomId)

	assert.Equal(t, `House Key`, names(u.Character.GetAllBackpackItems()), `only the key stays`)
	assert.Empty(t, u.Character.GetAllWornItems(), `the helm went too`)
	assert.Zero(t, u.Character.Gold)
	require.Len(t, lostItems, 3)
	for _, li := range lostItems {
		assert.Equal(t, u.Character.Name, li.Owner)
		assert.Equal(t, `death`, li.How)
		assert.Equal(t, run.Profile.Id, li.Profile)
	}
	assert.Equal(t, 1, changed, `saved once`)
}

func TestLosses_DeathSparesWornAtTheChance(t *testing.T) {
	u, _ := lossWorld(t)
	withRng(t, func(n int) int { return n - 1 }) // never under the chance
	OnPlayerDeath(u.UserId, u.Character.RoomId)
	assert.Equal(t, `Iron Helm`, names(u.Character.GetAllWornItems()))
	assert.Len(t, lostItems, 2, `the sword and the bread`)
}

// Logging out inside keeps what is carried loose (not gold, not worn), and
// says so at the next login. Outside a rift nothing is taken.
func TestLosses_Logout(t *testing.T) {
	u, run := lossWorld(t)
	room := u.Character.RoomId
	OnPlayerDespawn(u.UserId, room)
	assert.Equal(t, `House Key`, names(u.Character.GetAllBackpackItems()))
	assert.Equal(t, `Iron Helm`, names(u.Character.GetAllWornItems()))
	assert.Equal(t, 120, u.Character.Gold)
	assert.Len(t, lostItems, 2)
	assert.False(t, run.Members[u.UserId])

	notice, _ := u.Character.GetMiscData(logoutNoticeKey()).(string)
	assert.Contains(t, notice, `Sword and Loaf`)
	tellLogoutLosses(u)
	assert.Nil(t, u.Character.GetMiscData(logoutNoticeKey()), `told once`)

	// Out in the world, a despawn takes nothing.
	require.True(t, u.Character.StoreItem(items.New(testSword)))
	OnPlayerDespawn(u.UserId, testOrigin)
	assert.Len(t, lostItems, 2)
}

// A rubble search has the find chance to turn up 1 to find_max lost items,
// which leave the record for good; otherwise nothing.
func TestLosses_FoundInRubble(t *testing.T) {
	u, run := lossWorld(t)
	p := run.Profile
	for _, id := range []int{testSword, testBread, testHelm} {
		lostItems = append(lostItems, LostItem{Item: items.New(id), Owner: `Someone`, Profile: p.Id})
	}
	before := len(u.Character.GetAllBackpackItems())

	withRng(t, func(n int) int { return n - 1 }) // the chance fails
	assert.Empty(t, findLost(u, p))
	assert.Len(t, lostItems, 3)

	withRng(t, func(n int) int { return 0 }) // the chance passes; one item
	got := findLost(u, p)
	assert.Len(t, got, 1)
	assert.Len(t, lostItems, 2, `gone from the record`)
	assert.Len(t, u.Character.GetAllBackpackItems(), before+1)

	calls := 0
	withRng(t, func(n int) int { // passes, then two items
		calls++
		if calls == 2 {
			return n - 1
		}
		return 0
	})
	assert.Len(t, findLost(u, p), 2)
	assert.Empty(t, lostItems)
	assert.Empty(t, findLost(u, p), `nothing left to find`)
}

// The record keeps at most `keep` items, the oldest dropping off, and
// survives a save and load.
func TestLosses_RecordCapAndPersistence(t *testing.T) {
	u, run := lossWorld(t)
	run.Profile.Losses.Keep = 2
	t.Cleanup(func() { run.Profile.Losses.Keep = 1000 })
	lostItems = []LostItem{{Item: items.New(testHelm), Owner: `Old`}}
	OnPlayerDespawn(u.UserId, u.Character.RoomId)
	require.Len(t, lostItems, 2)
	assert.NotEqual(t, `Old`, lostItems[0].Owner, `the oldest dropped off`)

	b, err := yaml.Marshal(LostSnapshot())
	require.NoError(t, err)
	var st LostState
	require.NoError(t, yaml.Unmarshal(b, &st))
	lostItems = nil
	RestoreLost(st)
	assert.Equal(t, 2, LostCount())
	assert.Equal(t, testSword, lostItems[0].Item.ItemId)
}

// Potions in the bandolier and components in the bag are carried loose
// too; an item bound to an account or a house key never leaves its owner.
func TestLosses_EveryPoolAndBoundItems(t *testing.T) {
	u, _ := lossWorld(t)
	u.Character.PotionItems = append(u.Character.PotionItems, items.New(testBread))
	u.Character.ComponentItems = append(u.Character.ComponentItems, items.New(testBread))
	deed := items.New(testSword)
	deed.BoundUserId = u.UserId
	require.True(t, u.Character.StoreItem(deed))
	withRng(t, func(n int) int { return n - 1 })

	OnPlayerDeath(u.UserId, u.Character.RoomId)
	assert.Empty(t, u.Character.PotionItems)
	assert.Empty(t, u.Character.ComponentItems)
	left := u.Character.GetAllBackpackItems()
	assert.Len(t, left, 2, `the house key and the bound deed stay`)
	for _, itm := range lostItems {
		assert.Zero(t, itm.Item.BoundUserId)
	}
}

// A found item carries no old stolen-goods marks.
func TestLosses_FoundIsNotStolen(t *testing.T) {
	u, run := lossWorld(t)
	p := run.Profile
	stolen := items.New(testHelm)
	stolen.StolenFrom, stolen.StolenBy = `a merchant`, 77
	lostItems = LostRecord{{Item: stolen, Owner: `Someone`}}
	withRng(t, func(n int) int { return 0 })
	got := findLost(u, p)
	require.Len(t, got, 1)
	for _, itm := range u.Character.GetAllBackpackItems() {
		if itm.ItemId == testHelm {
			assert.False(t, itm.IsStolen(), `the finder is no thief`)
		}
	}

}
