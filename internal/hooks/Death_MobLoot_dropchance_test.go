package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
)

// dropChancePtr is a set itemdropchance for a test mob.
func dropChancePtr(v int) *int { return &v }

// TestDropMobLoot_ItemDropChanceZeroNeverDrops pins the #585 ruling at the
// death hook: a mob that authors itemdropchance 0 never drops what it wears,
// an absent key keeps the old default (every worn item drops), a positive
// chance is unchanged, and an item's own dropchance still overrides the
// mob-wide 0. Carried items and gold are not gated by itemdropchance.
// Corpses are off in tests, so loot lands on the room floor.
func TestDropMobLoot_ItemDropChanceZeroNeverDrops(t *testing.T) {
	cleanup := items.SeedItemsForTest(map[int]*items.ItemSpec{
		2: {ItemId: 2, Name: "Ordinary Helm", Type: items.Head},
		3: {ItemId: 3, Name: "Reward Trinket"},
		4: {ItemId: 4, Name: "Own-Chance Boots", Type: items.Feet},
	})
	defer cleanup()
	biomesCleanup := rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"default": {BiomeId: "default", Name: "Default"},
	})
	defer biomesCleanup()
	defer util.SetRandForTest(585)()

	// kill drops one mob wearing a helm (no own chance) and boots with their
	// own 100% chance, carrying a trinket, and reports what reached the floor.
	kill := func(chance *int) (helm, boots, trinket int, gold int) {
		m := &mobs.Mob{ItemDropChance: chance}
		m.Character = characters.Character{}
		m.Character.Equipment.Head = items.Item{ItemId: 2}
		m.Character.Equipment.Feet = items.Item{ItemId: 4, DropChance: 100}
		m.Character.Items = []items.Item{{ItemId: 3}}
		m.Character.Gold = 7
		room := &rooms.Room{RoomId: 12345, Biome: "default"}
		dropMobLootAndSetCorpse(m, room)
		for _, itm := range room.Items {
			switch itm.ItemId {
			case 2:
				helm++
			case 3:
				trinket++
			case 4:
				boots++
			}
		}
		return helm, boots, trinket, room.Gold
	}

	const kills = 400
	totals := map[string][4]int{}
	for name, chance := range map[string]*int{
		"explicit 0": dropChancePtr(0),
		"absent":     nil,
		"50":         dropChancePtr(50),
		"100":        dropChancePtr(100),
	} {
		var sum [4]int
		for i := 0; i < kills; i++ {
			h, b, tr, g := kill(chance)
			sum[0] += h
			sum[1] += b
			sum[2] += tr
			sum[3] += g
		}
		totals[name] = sum
	}

	assert.Equal(t, 0, totals["explicit 0"][0], "itemdropchance 0 must never drop a worn item (#585)")
	assert.Equal(t, kills, totals["absent"][0], "an absent itemdropchance keeps the default: every worn item drops")
	assert.Equal(t, kills, totals["100"][0], "itemdropchance 100 drops every worn item")
	half := totals["50"][0]
	assert.True(t, half > kills*35/100 && half < kills*65/100,
		"itemdropchance 50 must drop about half the time, got %d of %d", half, kills)

	// A mob that is or was a companion may wear its owner's gear, and the
	// companion death path clears the record expecting the corpse to take it,
	// so its authored 0 keeps the old reading: everything worn drops.
	companion := &mobs.Mob{ItemDropChance: dropChancePtr(0)}
	companion.Character = characters.Character{EverCharmed: true}
	companion.Character.Equipment.Head = items.Item{ItemId: 2}
	room := &rooms.Room{RoomId: 12346, Biome: "default"}
	dropMobLootAndSetCorpse(companion, room)
	assert.Len(t, room.Items, 1, "a companion's worn gear must still drop at itemdropchance 0, or its owner's gear is destroyed")

	for name, sum := range totals {
		assert.Equal(t, kills, sum[1], "%s: an item's own dropchance overrides the mob-wide chance", name)
		assert.Equal(t, kills, sum[2], "%s: carried items are not gated by itemdropchance", name)
		assert.Equal(t, kills*7, sum[3], "%s: gold always drops", name)
	}
}
