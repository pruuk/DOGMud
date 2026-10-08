package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/require"
)

// #272: appraise and offer are dealing, so they take the shop sight gate that
// list, buy and sell already take (lighting plan 5b): below the faces band
// they refuse with the one shared line, name no merchant and take no gold.
// The fixture is list_sight_test.go's listSightRoom (merchant "Keeper").

func TestAppraise_DarkRoom_RefusesAndNamesNoMerchant(t *testing.T) {
	for _, biome := range []string{"cave", "shapes"} {
		user, room := listSightRoom(t, biome)
		user.Character.StoreItem(items.New(84001))
		user.Character.Gold = 100

		handled, err := Appraise("tin cup", user, room, 0)
		require.NoError(t, err)
		require.True(t, handled)

		lines := listSightPlain(events.DrainQueuedMessagesForTest(8411))
		require.Equal(t, []string{actions.ShopSightRefusalText}, lines, "biome %s", biome)
		require.Equal(t, 100, user.Character.Gold, "biome %s: no fee in the dark", biome)
	}
}

func TestOffer_DarkRoom_Refuses(t *testing.T) {
	for _, biome := range []string{"cave", "shapes"} {
		user, room := listSightRoom(t, biome)
		user.Character.StoreItem(items.New(84001))

		handled, err := Offer("tin cup", user, room, 0)
		require.NoError(t, err)
		require.True(t, handled)

		lines := listSightPlain(events.DrainQueuedMessagesForTest(8411))
		require.Equal(t, []string{actions.ShopSightRefusalText}, lines, "biome %s", biome)
	}
}

// Control: in a lit shop both verbs deal as before and never refuse.
func TestAppraiseOffer_LitRoom_Deal(t *testing.T) {
	user, room := listSightRoom(t, "city")
	user.Character.StoreItem(items.New(84001))
	user.Character.Gold = 100

	_, err := Appraise("tin cup", user, room, 0)
	require.NoError(t, err)
	_, err = Offer("tin cup", user, room, 0)
	require.NoError(t, err)

	joined := strings.Join(listSightPlain(events.DrainQueuedMessagesForTest(8411)), "\n")
	require.NotContains(t, joined, actions.ShopSightRefusalText)
	require.Contains(t, joined, "Keeper")
}
