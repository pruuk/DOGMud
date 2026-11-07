package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #794: a potion whose effect the Vitalis Bandolier already sustains is not
// drunk (the hold would absorb it), and a potion it holds is not sold (it
// would sour leaving, and the merchant must not call it spoiled).

const (
	vdsVitalisId = 999960
	vdsSalveId   = 999961 // condition 54
	vdsCond      = 54
)

func vdsScene(t *testing.T) *characters.Character {
	t.Helper()
	seedParityWorld(t)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		vdsVitalisId: {ItemId: vdsVitalisId, Name: "Vitalis Bandolier", Type: items.Belt, Subtype: items.Wearable,
			IsBandolier: true, BandolierCapacity: 4, AmbientPotions: true, PreservesContents: true},
		vdsSalveId: {ItemId: vdsSalveId, Name: "test salve", Type: items.Potion, Subtype: items.Drinkable,
			ConditionIds: []int{vdsCond}, Uses: 1, Value: 50, VendorCategories: []string{"alchemy"}},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		vdsCond: {ConditionId: vdsCond, Name: "Salve", TriggerCount: 150, RoundInterval: 1},
	}))
	c := newParityChar()
	c.Equipment.Belt = items.New(vdsVitalisId)
	return c
}

func TestVitalis_DrinkOfASustainedEffectIsRefusedAndKept(t *testing.T) {
	c := vdsScene(t)
	storeItem(t, c, items.New(vdsSalveId)) // slotted
	// The bandolier has attuned and holds the effect.
	c.SetMiscData(characters.AmbientBandolierMiscKey, []int{vdsCond})
	require.NoError(t, c.AddCondition(vdsCond, true))
	c.Items = append(c.Items, items.New(vdsSalveId)) // a second, in the pack

	u := &users.UserRecord{UserId: parityUserId, Character: c}
	events.DrainQueuedMessagesForTest(parityUserId)
	res := Quaff(NewUserActorInRoom(u, newEmptyTestRoom(t)).(DrinkActor), "test salve")
	told := strings.Join(events.DrainQueuedMessagesForTest(parityUserId), " ")

	assert.False(t, res.Drank, "the drink must be refused")
	assert.Equal(t, DrinkRefuseSustained, res.Refusal)
	assert.Contains(t, told, "already sustains that effect")
	assert.Len(t, c.PotionItems, 1, "the slotted potion is kept")
	assert.Len(t, c.Items, 1, "the pack potion is kept")
}

func TestVitalis_APotionItHoldsIsNotForSale(t *testing.T) {
	c := vdsScene(t)
	storeItem(t, c, items.New(vdsSalveId))
	held := c.PotionItems[0]
	require.True(t, held.Preserved)

	q := QuoteSale(c, nil, &mobs.Mob{}, nil, held, true)
	assert.Zero(t, q.Price)
	assert.Contains(t, q.Tell, "held at its best", "the seller is told the potion is held")
	assert.NotContains(t, q.Tell+q.Say, "gone bad", "a held potion is not called spoiled")
}
