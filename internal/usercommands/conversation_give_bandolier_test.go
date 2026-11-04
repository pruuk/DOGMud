package usercommands

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// A potion lands in a worn bandolier by itself, so a hand-in wanting a potion
// has to read it there: the NPC and the engine handle a held item, never the
// player moving it first (owner ruling, 2026-11-02).
const potionHandInYAML = `mobid: 1
zone: TestZone
topics:
  - key: potion
    kind: handin
    quest: 9
    label: "The Lost Potion"
    playerLine: "Here is your potion."
    text: "My thanks."
    questExcluded: [9-end]
    requiresItem: 30001
    grantsQuest: 9-end
`

const potionShownYAML = `mobid: 1
zone: TestZone
topics:
  - key: potion
    kind: info
    label: "Look at this potion."
    text: "Fine work, that."
    carriesItem: 30001
`

// wearBandolier belts the user with the seeded bandolier for this test.
func wearBandolier(t *testing.T, u *users.UserRecord) {
	t.Helper()
	belt, potions := u.Character.Equipment.Belt, u.Character.PotionItems
	u.Character.Equipment.Belt = items.New(40182)
	u.Character.PotionItems = nil
	t.Cleanup(func() { u.Character.Equipment.Belt, u.Character.PotionItems = belt, potions })
}

// bandolierPotion stores a potion, which must land in the bandolier.
func bandolierPotion(t *testing.T, u *users.UserRecord, itemId int) items.Item {
	t.Helper()
	itm := items.New(itemId)
	require.True(t, u.Character.StoreItem(itm))
	require.Equal(t, 0, countBackpack(u, itemId), "the potion must not be in the backpack")
	require.Equal(t, 1, countItemsById(u.Character.PotionItems, itemId), "the potion must be in the bandolier")
	return itm
}

func TestBandolierHandIn_MenuTakesThePotion(t *testing.T) {
	user, mob, done := fixtureWith(t, potionHandInYAML)
	defer done()
	wearBandolier(t, user)
	bandolierPotion(t, user, 30001)

	conversation.Talk(user, mustRoom(t, user), mob)
	require.True(t, user.HasOpenMenu(), "the hand-in is on the menu with the potion in the bandolier")
	drained(1)
	require.True(t, user.RouteMenuInput("1", true))

	require.Contains(t, drained(1), "You give the")
	require.Equal(t, 0, countItemsById(user.Character.PotionItems, 30001), "taken from the bandolier")
	require.False(t, mobHolds(mob, 30001), "the topic takes the item; the mob does not keep it")
	require.Equal(t, []string{"9-end"}, questTokensFor(1))
}

func TestBandolierHandIn_GiveCommandPlaysTheTopic(t *testing.T) {
	user, mob, done := fixtureWith(t, potionHandInYAML)
	defer done()
	wearBandolier(t, user)
	bandolierPotion(t, user, 30001)

	_, err := Give("healing potion skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)

	require.Contains(t, drained(1), "My thanks.")
	require.Equal(t, 0, countItemsById(user.Character.PotionItems, 30001))
	require.False(t, mobHolds(mob, 30001))
	require.Equal(t, []string{"9-end"}, questTokensFor(1))
}

// A two word name resolves for a bandolier potion as for a backpack item.
func TestBandolierHandIn_GiveCommandTakesATwoWordRecipient(t *testing.T) {
	user, mob, done := fixtureWith(t, potionHandInYAML)
	defer done()
	wearBandolier(t, user)
	bandolierPotion(t, user, 30001)
	name := mob.Character.Name
	mob.Character.Name = "Smith Rusk"
	t.Cleanup(func() { mob.Character.Name = name })

	_, err := Give("healing potion smith rusk", user, mustRoom(t, user), 0)
	require.NoError(t, err)

	require.Equal(t, 0, countItemsById(user.Character.PotionItems, 30001))
	require.Equal(t, []string{"9-end"}, questTokensFor(1))
}

// With one in each place, the menu takes the backpack's first.
func TestBandolierHandIn_MenuPrefersTheBackpack(t *testing.T) {
	user, mob, done := fixtureWith(t, potionHandInYAML)
	defer done()
	wearBandolier(t, user)
	bandolierPotion(t, user, 30001)
	user.Character.Items = append(user.Character.Items, items.New(30001))

	conversation.Talk(user, mustRoom(t, user), mob)
	require.True(t, user.RouteMenuInput("1", true))

	require.Equal(t, 0, countBackpack(user, 30001), "the backpack's goes first")
	require.Equal(t, 1, countItemsById(user.Character.PotionItems, 30001), "the bandolier's stays")
}

// With one in each place, the instance the player named is the one taken.
func TestBandolierHandIn_TakesTheNamedInstance(t *testing.T) {
	user, mob, done := fixtureWith(t, potionHandInYAML)
	defer done()
	wearBandolier(t, user)
	named := bandolierPotion(t, user, 30001)
	user.Character.Items = append(user.Character.Items, items.New(30001))

	require.True(t, conversation.Give(user, mustRoom(t, user), mob, named))

	require.Equal(t, 1, countBackpack(user, 30001), "the backpack's stays")
	require.Equal(t, 0, countItemsById(user.Character.PotionItems, 30001), "the named one went")
}

// A shown item reads the bandolier too, and stays there.
func TestBandolierShownItem_PlaysAndIsKept(t *testing.T) {
	user, _, done := fixtureWith(t, potionShownYAML)
	defer done()
	wearBandolier(t, user)
	bandolierPotion(t, user, 30001)

	_, err := Give("healing potion skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)

	mine := drained(1)
	require.Contains(t, mine, "Fine work, that.")
	require.NotContains(t, mine, "You give the")
	require.Equal(t, 1, countItemsById(user.Character.PotionItems, 30001))
}

// A topic that wants the potion but is closed hands it back, and it never moves.
func TestBandolierHandIn_IneligibleIsHandedBack(t *testing.T) {
	user, mob, done := fixtureWith(t, potionHandInYAML)
	defer done()
	wearBandolier(t, user)
	bandolierPotion(t, user, 30001)
	user.Character.QuestProgress = map[int]string{9: "end"}

	_, err := Give("healing potion skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)

	require.Contains(t, drained(1), "hands it back")
	require.Equal(t, 1, countItemsById(user.Character.PotionItems, 30001))
	require.False(t, mobHolds(mob, 30001))
}

// With no topic wanting it, give stays backpack only: the refusal is the very
// line a player carrying no such potion reads.
func TestGive_OrdinaryGiveStaysBackpackOnly(t *testing.T) {
	for name, who := range map[string]func(*users.UserRecord) string{
		"mob with no topic for it": func(*users.UserRecord) string { return "skeleton" },
		"another player":           func(*users.UserRecord) string { return users.GetByUserId(2).Character.Name },
	} {
		t.Run(name, func(t *testing.T) {
			user, mob, done := talkFixture(t)
			defer done()
			wearBandolier(t, user)
			target := who(user)

			_, err := Give("healing potion "+target, user, mustRoom(t, user), 0)
			require.NoError(t, err)
			refused := drained(1)
			require.NotEmpty(t, refused)

			bandolierPotion(t, user, 30001)
			_, err = Give("healing potion "+target, user, mustRoom(t, user), 0)
			require.NoError(t, err)

			require.Equal(t, refused, drained(1), "the same refusal with or without the potion")
			require.Equal(t, 1, countItemsById(user.Character.PotionItems, 30001), "it stays in the bandolier")
			require.False(t, mobHolds(mob, 30001))
			require.Empty(t, drained(2), "nothing happened in the room")
		})
	}
}

// A charmed NPC offers no topics, so it wants nothing from a bandolier.
func TestWantsItem_ReadsTheTopicsAndSkipsACharmedNPC(t *testing.T) {
	user, mob, done := fixtureWith(t, potionHandInYAML)
	defer done()
	require.True(t, conversation.WantsItem(user, mob, 30001))
	require.False(t, conversation.WantsItem(user, mob, 10001))

	user.Character.QuestProgress = map[int]string{9: "end"}
	require.True(t, conversation.WantsItem(user, mob, 30001), "a closed topic still wants it, so give hands it back")

	mob.Character.Charmed = characters.NewCharm(2, characters.CharmPermanent, "")
	require.False(t, conversation.WantsItem(user, mob, 30001))
}

// shippedYAML reads a file under the shipped world, anchored on this test file
// because a shared test binary's working directory is not the package's.
func shippedYAML(t *testing.T, out any, parts ...string) {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(append([]string{filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud"}, parts...)...)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(data, out))
}

// Quest 2, The Warren Compact, on its shipped dialogue and items: the shaman
// takes a healing salve from a shipped leather bandolier, by menu and by give.
func TestQuest2SalveInABandolier_Shipped(t *testing.T) {
	const salveId, bandolierId = 30036, 20059
	for name, handIn := range map[string]func(t *testing.T, u *users.UserRecord){
		"menu": func(t *testing.T, u *users.UserRecord) {
			conversation.Talk(u, mustRoom(t, u), mobs.GetInstance(100))
			require.True(t, u.HasOpenMenu())
			require.True(t, u.RouteMenuInput("1", true))
		},
		"give": func(t *testing.T, u *users.UserRecord) {
			_, err := Give("salve skeleton", u, mustRoom(t, u), 0)
			require.NoError(t, err)
		},
	} {
		t.Run(name, func(t *testing.T) {
			user, _, done := fixtureWith(t, talkFixtureYAML)
			defer done()

			var salve, belt items.ItemSpec
			shippedYAML(t, &salve, "items", "consumables-30000", "30036-healing_salve.yaml")
			shippedYAML(t, &belt, "items", "armor-20000", "belt", "20059-leather_bandolier.yaml")
			defer items.SeedItemsForTest(map[int]*items.ItemSpec{salveId: &salve, bandolierId: &belt})()

			var shaman dialogue.DialogueFile
			shippedYAML(t, &shaman, "dialogue", "labyrinth_of_low_tunnels", "74.yaml")
			require.Equal(t, salveId, shaman.FindTopic("salve_delivery").RequiresItem)
			shaman.MobId, shaman.Zone = 1, "TestZone"
			defer dialogue.InstallForTest(&shaman)()

			beltWas, potionsWas := user.Character.Equipment.Belt, user.Character.PotionItems
			t.Cleanup(func() { user.Character.Equipment.Belt, user.Character.PotionItems = beltWas, potionsWas })
			user.Character.Equipment.Belt = items.New(bandolierId)
			user.Character.PotionItems = nil
			user.Character.QuestProgress = map[int]string{2: "start"}
			bandolierPotion(t, user, salveId)

			handIn(t, user)

			require.Equal(t, 0, countItemsById(user.Character.PotionItems, salveId), "the shaman took the salve")
			require.Equal(t, []string{"2-end"}, questTokensFor(1))
		})
	}
}
