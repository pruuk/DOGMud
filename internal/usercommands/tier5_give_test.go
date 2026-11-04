package usercommands

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

const tier5World = "../../_datafiles/world/dogmud"

// tier5NPC stands the fixture's Skeleton in for a shipped NPC: its real dialogue
// file, the real quest and item files, so these tests read the names a player
// types against the names the game ships.
func tier5NPC(t *testing.T, dialogueFile string, mobId int, zone string, questFiles []string, itemIds ...int) (*users.UserRecord, *mobs.Mob) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(tier5World, "dialogue", dialogueFile))
	require.NoError(t, err)
	user, mob, done := fixtureWith(t, string(src))
	t.Cleanup(done)
	mob.MobId = mobs.MobId(mobId)
	mob.Zone = zone

	specs := map[int]*items.ItemSpec{}
	for _, id := range itemIds {
		matches, err := filepath.Glob(filepath.Join(tier5World, "items", "materials-40000", fmt.Sprintf("%d-*.yaml", id)))
		require.NoError(t, err)
		require.Len(t, matches, 1, "item %d", id)
		data, err := os.ReadFile(matches[0])
		require.NoError(t, err)
		var spec items.ItemSpec
		require.NoError(t, yaml.Unmarshal(data, &spec))
		specs[id] = &spec
	}
	t.Cleanup(items.SeedItemsForTest(specs))

	var qs []*quests.Quest
	for _, f := range questFiles {
		data, err := os.ReadFile(filepath.Join(tier5World, "quests", f))
		require.NoError(t, err)
		var q quests.Quest
		require.NoError(t, yaml.Unmarshal(data, &q))
		qs = append(qs, &q)
	}
	t.Cleanup(quests.SeedQuestsForTest(qs...))
	return user, mob
}

func tier5Hold(t *testing.T, u *users.UserRecord, ids ...int) {
	t.Helper()
	for _, id := range ids {
		require.True(t, u.Character.StoreItem(items.New(id)), "item %d", id)
	}
}

const (
	benchMark = 40112
	survey    = 40113
	mapCopy   = 40114
	cipher    = 40115
	reading   = 40116
	lintel    = 40118
	dross     = 9360
	orin      = 9332
	drossZone = "New Plymouth Temple"
	orinZone  = "New Plymouth Crafting"
	drossFile = "new_plymouth_temple/9360.yaml"
	orinFile  = "new_plymouth_crafting/9332.yaml"
	quest68   = "68-the_cooperage_circle.yaml"
	quest69   = "69-the_gallery_cipher.yaml"
	quest70   = "70-the_pre_founding_web.yaml"
	skeleton  = " skeleton" // the fixture's mob, as a give's recipient
)

// `give rubbing dross` reaches the rubbing Dross wants, wherever Toby's lies in
// the pack: the hand-in plays, with the other papers left alone.
func TestTier5GiveRubbingReachesDrossPastTobysRubbing(t *testing.T) {
	for _, order := range [][]int{{benchMark, cipher, reading}, {cipher, benchMark, reading}, {lintel, cipher, reading}} {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			user, mob := tier5NPC(t, drossFile, dross, drossZone, []string{quest69}, benchMark, cipher, reading, lintel)
			user.Character.QuestProgress = map[int]string{69: "reading"}
			tier5Hold(t, user, order...)

			_, err := Give("rubbing"+skeleton, user, mustRoom(t, user), 0)
			require.NoError(t, err)
			require.Contains(t, drained(1), "You reached her")
			require.Equal(t, []string{"69-end"}, questTokensFor(1))
			require.Equal(t, 0, countBackpack(user, reading), "the topic takes the reading")
			require.Equal(t, 1, countBackpack(user, order[0]), "the first paper in the pack stays")
			require.False(t, mobHolds(mob, order[0]))
		})
	}
}

// The same at Orin: the lintel rubbing goes in, past Toby's and past the cipher.
func TestTier5GiveRubbingReachesOrinPastTheOtherRubbings(t *testing.T) {
	for _, order := range [][]int{{benchMark, lintel}, {lintel, benchMark}, {cipher, lintel}} {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			user, mob := tier5NPC(t, orinFile, orin, orinZone, []string{quest68, quest70}, benchMark, cipher, lintel)
			user.Character.QuestProgress = map[int]string{70: "witness"}
			tier5Hold(t, user, order...)

			_, err := Give("rubbing"+skeleton, user, mustRoom(t, user), 0)
			require.NoError(t, err)
			require.Contains(t, drained(1), "A rubbing from the lintel")
			require.Equal(t, []string{"70-end"}, questTokensFor(1))
			require.Equal(t, 0, countBackpack(user, lintel))
			other := order[0]
			if other == lintel {
				other = order[1]
			}
			require.Equal(t, 1, countBackpack(user, other), "the other rubbing stays")
			require.False(t, mobHolds(mob, other))
		})
	}
}

// A name that fits nothing the NPC wants still reaches the item the player named.
func TestTier5GiveFallsBackToTheNamedItem(t *testing.T) {
	user, _ := tier5NPC(t, orinFile, orin, orinZone, []string{quest68, quest70}, cipher, lintel)
	user.Character.QuestProgress = map[int]string{70: "witness"}
	tier5Hold(t, user, cipher, lintel)

	_, err := Give("cipher rubbing"+skeleton, user, mustRoom(t, user), 0)
	require.NoError(t, err)
	mine := drained(1)
	require.NotContains(t, mine, "A rubbing from the lintel")
	require.Contains(t, mine, "Cipher Rubbing")
	require.Empty(t, questTokensFor(1))
	require.Equal(t, 1, countBackpack(user, lintel))
}

// A player at 68-evidence holding only Toby's rubbing does not lose it to Orin:
// no topic wants it, so it takes the ordinary give, and the archetype his mob file
// names hands it straight back.
func TestTier5OrinHandsBackTobysRubbing(t *testing.T) {
	user, mob := tier5NPC(t, orinFile, orin, orinZone, []string{quest68, quest70}, benchMark)
	user.Character.QuestProgress = map[int]string{68: "evidence"}

	mobFile, err := os.ReadFile(filepath.Join(tier5World, "mobs", "new_plymouth_crafting", "9332-orin_the_bookseller.yaml"))
	require.NoError(t, err)
	var orinMob struct {
		BehaviorArchetype string `yaml:"behavior_archetype"`
	}
	require.NoError(t, yaml.Unmarshal(mobFile, &orinMob))
	require.NotEmpty(t, orinMob.BehaviorArchetype)
	tree, err := os.ReadFile(filepath.Join(tier5World, "behaviors", "archetypes", orinMob.BehaviorArchetype+".yaml"))
	require.NoError(t, err)
	node, err := behaviortree.LoadTreeFromBytes(tree)
	require.NoError(t, err)
	t.Cleanup(behaviortree.GetEngine().SetMobTreeForTest(orin, node))

	tier5Hold(t, user, benchMark)
	for _, word := range []string{"rubbing", "bench mark"} {
		_, err = Give(word+skeleton, user, mustRoom(t, user), 0)
		require.NoError(t, err)
		require.Contains(t, drained(1), "hands back", word)
		require.Equal(t, 1, countBackpack(user, benchMark), word)
		require.False(t, mobHolds(mob, benchMark), word)
	}
}

// Every hint says survey, so `give survey orin` works, and so does the word on
// the item: one give warns, the next commits.
func TestTier5SurveyAnswersToBothWords(t *testing.T) {
	for _, word := range []string{"survey", "map-fragment"} {
		t.Run(word, func(t *testing.T) {
			user, mob := tier5NPC(t, orinFile, orin, orinZone, []string{quest68}, survey, mapCopy)
			user.Character.QuestProgress = map[int]string{68: "survey"}
			tier5Hold(t, user, survey)
			room := mustRoom(t, user)

			_, err := Give(word+skeleton, user, room, 0)
			require.NoError(t, err)
			require.Contains(t, drained(1), "So it is true")
			require.Equal(t, 1, countBackpack(user, survey), "shown, not taken")

			// The game's trigger moves the step on hearing that topic.
			user.Character.QuestProgress = map[int]string{68: "choice"}
			_, err = Give(word+skeleton, user, room, 0)
			require.NoError(t, err)
			require.Contains(t, drained(1), "Plainly, then")
			require.Equal(t, 1, countBackpack(user, survey), "the warning takes nothing")
			require.Empty(t, questTokensFor(1))

			_, err = Give(word+skeleton, user, room, 0)
			require.NoError(t, err)
			require.Contains(t, drained(1), "Then it is done")
			require.Equal(t, 0, countBackpack(user, survey), "the second give commits")
			require.Equal(t, 1, countBackpack(user, mapCopy))
			require.Equal(t, []string{"68-end"}, questTokensFor(1))
			require.False(t, mobHolds(mob, survey))
		})
	}
}
