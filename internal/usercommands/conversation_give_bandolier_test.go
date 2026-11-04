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
	"github.com/GoMudEngine/GoMud/internal/pets"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
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

// A potion in the bandolier reads the same target refusal as one in the
// backpack: the sight gate at shapes and in the dark, and "not here" for a target
// that is away. Whether the NPC wants it does not show, and the potion stays put.
func TestBandolierGive_TargetRefusalsReadAsFromTheBackpack(t *testing.T) {
	away := func(room *rooms.Room) { room.RemoveMob(100) }
	for _, c := range []struct {
		name, yaml, who string
		band            aimBand
		setup           func(*rooms.Room)
		want            string
	}{
		{"shapes, wanted", potionHandInYAML, "skeleton", aimShapes, nil, "You can only make out shapes here."},
		{"shapes, unwanted", talkFixtureYAML, "skeleton", aimShapes, nil, "You can only make out shapes here."},
		{"dark, wanted", potionHandInYAML, "skeleton", aimDark, nil, refusal.NotHereCreatureLine},
		{"dark, unwanted", talkFixtureYAML, "skeleton", aimDark, nil, refusal.NotHereCreatureLine},
		{"dark, a two word name", potionHandInYAML, "smith rusk", aimDark, nil, refusal.NotHereCreatureLine},
		{"lit, target away", potionHandInYAML, "skeleton", aimFull, away, refusal.NotHereCreatureLine},
		{"lit, nobody by that name", potionHandInYAML, "ghost", aimFull, nil, refusal.NotHereCreatureLine},
	} {
		t.Run(c.name, func(t *testing.T) {
			told := map[string]string{}
			for _, where := range []string{"backpack", "bandolier"} {
				_, mob, done := fixtureWith(t, c.yaml)
				user, room := aimSceneSeeded(t, c.band)
				wearBandolier(t, user)
				if where == "backpack" {
					user.Character.Items = append(user.Character.Items, items.New(30001))
				} else {
					bandolierPotion(t, user, 30001)
				}
				if c.setup != nil {
					c.setup(room)
				}

				_, err := Give("healing potion "+c.who, user, room, 0)
				require.NoError(t, err)

				told[where] = aimTold(1)
				require.Equal(t, 1, countBackpack(user, 30001)+countItemsById(user.Character.PotionItems, 30001), where)
				require.False(t, mobHolds(mob, 30001), where)
				require.Empty(t, questTokensFor(1), where)
				done()
			}
			require.Contains(t, told["bandolier"], c.want)
			require.NotContains(t, told["bandolier"], "aren't carrying")
			require.Equal(t, told["backpack"], told["bandolier"])
		})
	}
}

// At shapes a figure resolves, so the wanted hand-in plays and a figure that
// is a player keeps today's refusal.
func TestBandolierGive_AtShapesAFigureResolves(t *testing.T) {
	_, _, done := fixtureWith(t, potionHandInYAML)
	defer done()
	user, room := aimSceneSeeded(t, aimShapes)
	wearBandolier(t, user)
	bandolierPotion(t, user, 30001)

	_, err := Give("healing potion 1.shape", user, room, 0)
	require.NoError(t, err)
	require.Contains(t, aimTold(1), "aren't carrying", "shape 1 is Bobrick")
	require.Equal(t, 1, countItemsById(user.Character.PotionItems, 30001))

	_, err = Give("healing potion 2.shape", user, room, 0)
	require.NoError(t, err)
	require.Equal(t, 0, countItemsById(user.Character.PotionItems, 30001), "shape 2 is the Skeleton, who wants it")
	require.Equal(t, []string{"9-end"}, questTokensFor(1))
}

// `2.healing potion` counts over the bandolier's potions, oldest first, as it
// would over a backpack.
func TestBandolierGive_HonoursTheOrdinal(t *testing.T) {
	user, _, done := fixtureWith(t, potionHandInYAML)
	defer done()
	wearBandolier(t, user)
	bandolierPotion(t, user, 30001)
	user.Character.PotionItems = append(user.Character.PotionItems, items.New(30001))
	pots := user.Character.PotionItems
	require.Len(t, pots, 2)
	pots[0].CraftedRound, pots[1].CraftedRound = 5, 9
	first, second := pots[0].UUID, pots[1].UUID

	_, err := Give("3.healing potion skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	require.Contains(t, drained(1), "aren't carrying", "there is no third")
	require.Len(t, user.Character.PotionItems, 2)

	_, err = Give("2.healing potion skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	require.Len(t, user.Character.PotionItems, 1)
	require.Equal(t, first, user.Character.PotionItems[0].UUID, "the second went, the first stayed")
	require.NotEqual(t, second, user.Character.PotionItems[0].UUID)
	require.Equal(t, []string{"9-end"}, questTokensFor(1))
}

// The NPC is asked before the potion is chosen: an older potion matching the
// name that nobody asked for does not stand in front of the one wanted.
func TestBandolierGive_ChoosesAmongWantedPotions(t *testing.T) {
	user, mob, done := fixtureWith(t, potionHandInYAML)
	defer done()
	const greater = 30002
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		30001: items.GetItemSpec(30001),
		40182: items.GetItemSpec(40182),
		greater: {ItemId: greater, Name: "Greater Healing Potion", Description: "A vial of dark red liquid.",
			Type: items.Potion, Subtype: items.Usable, Uses: 3, Value: 50},
	})()
	wearBandolier(t, user)
	bandolierPotion(t, user, greater)
	bandolierPotion(t, user, 30001)
	user.Character.PotionItems = append(user.Character.PotionItems, items.New(30001))
	pots := user.Character.PotionItems
	pots[0].CraftedRound, pots[1].CraftedRound, pots[2].CraftedRound = 1, 8, 3
	youngest := pots[1].UUID

	_, err := Give("healing potion skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)

	require.Contains(t, drained(1), "My thanks.")
	require.Equal(t, 1, countItemsById(user.Character.PotionItems, greater), "the unwanted one stays")
	require.False(t, mobHolds(mob, greater))
	require.Equal(t, 1, countItemsById(user.Character.PotionItems, 30001))
	remaining := -1
	for i, p := range user.Character.PotionItems {
		if p.ItemId == 30001 {
			remaining = i
		}
	}
	require.Equal(t, youngest, user.Character.PotionItems[remaining].UUID, "the oldest wanted one went")
	require.Equal(t, []string{"9-end"}, questTokensFor(1))
}

// A pet never takes a potion from the bandolier, the giver's own (by the word
// and by its name) or another player's: the refusal is the line of a player
// carrying no such potion.
func TestBandolierGive_APetTakesNothing(t *testing.T) {
	for _, who := range []string{"pet", "fang", "rex"} {
		t.Run(who, func(t *testing.T) {
			user, _, done := fixtureWith(t, potionHandInYAML)
			defer done()
			wearBandolier(t, user)
			other := users.GetByUserId(2)
			mine, theirs := user.Character.Pet, other.Character.Pet
			t.Cleanup(func() { user.Character.Pet, other.Character.Pet = mine, theirs })
			user.Character.Pet = pets.Pet{Name: "Fang", Type: "dog", Capacity: 5}
			other.Character.Pet = pets.Pet{Name: "Rex", Type: "dog", Capacity: 5}

			_, err := Give("healing potion "+who, user, mustRoom(t, user), 0)
			require.NoError(t, err)
			refused := drained(1)
			require.Contains(t, refused, "aren't carrying")

			bandolierPotion(t, user, 30001)
			_, err = Give("healing potion "+who, user, mustRoom(t, user), 0)
			require.NoError(t, err)

			require.Equal(t, refused, drained(1))
			require.Equal(t, 1, countItemsById(user.Character.PotionItems, 30001))
			require.Empty(t, user.Character.Pet.Items)
			require.Empty(t, other.Character.Pet.Items)
			require.Empty(t, questTokensFor(1))
		})
	}
}

// A hidden mob is not there to the giver: it takes nothing, wanted or not, and
// the refusal is the one an empty room gives, so nothing tells it apart.
func TestBandolierGive_AHiddenMobIsNotThere(t *testing.T) {
	for name, src := range map[string]string{"wanted": potionHandInYAML, "unwanted": talkFixtureYAML} {
		t.Run(name, func(t *testing.T) {
			told := map[bool]string{}
			for _, hidden := range []bool{true, false} {
				user, mob, done := fixtureWith(t, src)
				room := mustRoom(t, user)
				wearBandolier(t, user)
				bandolierPotion(t, user, 30001)
				if hidden {
					hideForAmbushTest(t, &mob.Character)
				} else {
					room.RemoveMob(100)
				}

				_, err := Give("healing potion skeleton", user, room, 0)
				require.NoError(t, err)

				told[hidden] = drained(1)
				require.Equal(t, 1, countItemsById(user.Character.PotionItems, 30001))
				require.False(t, mobHolds(mob, 30001))
				require.Empty(t, questTokensFor(1))
				done()
			}
			require.Contains(t, told[true], refusal.NotHereCreatureLine)
			require.Equal(t, told[false], told[true])
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
