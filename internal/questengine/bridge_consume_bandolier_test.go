package questengine

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
)

// #794: a scripted take reaches a potion in the bandolier, where StoreItem
// routes one by itself, and tells the player when it leaves the Vitalis
// Bandolier soured.

const (
	bridgeVitalisId = 975111
	bridgeSalveId   = 975112
)

func TestGameBridgeConsumeItem_TakesFromTheVitalisAndSaysItSoured(t *testing.T) {
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		bridgeVitalisId: {ItemId: bridgeVitalisId, Name: "Vitalis Bandolier", Type: items.Belt, Subtype: items.Wearable,
			IsBandolier: true, BandolierCapacity: 4, AmbientPotions: true, PreservesContents: true},
		bridgeSalveId: {ItemId: bridgeSalveId, Name: "quest salve", Type: items.Potion, Subtype: items.Drinkable,
			ConditionIds: []int{54}, Weight: 0.1},
	}))
	u := users.NewTestUser(9752, "bridgetake", "Bridgetake", 97520)
	u.Character.Equipment.Belt = items.New(bridgeVitalisId)
	u.Character.StoreItem(items.New(bridgeSalveId))
	if len(u.Character.PotionItems) != 1 {
		t.Fatal("fixture: the salve should slot")
	}
	events.DrainQueuedMessagesForTest(u.UserId)

	NewGameBridge(u, 1).ConsumeItem(bridgeSalveId)

	assert.Empty(t, u.Character.PotionItems, "the scripted take reaches the bandolier")
	told := strings.Join(events.DrainQueuedMessagesForTest(u.UserId), " ")
	assert.Contains(t, told, "sours the moment it leaves")
}
