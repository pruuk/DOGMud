package questengine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Rusk's opener names each First Heat step's command and a quest-aware topic
// answers it; on the craft step the hand-in completes the quest. A player who
// sold the dagger reports on their word (craft_turnin_told); the shown-dagger
// hand-in is pinned by TestRusk9116_HandInWithOrWithoutTheDagger.
func TestRusk9116_MidQuestGreetsAndAnswersEachStep(t *testing.T) {
	f := "35-first_heat.yaml"
	pre := []string{"30-end"}
	checkTopicSteps(t, "rusk", "9116.yaml", []topicStep{
		{f, 35, "start", "type list", pre, "heat_list_help", false},
		{f, 35, "list", "type buy iron ingot", pre, "heat_ingot_help", false},
		{f, 35, "ingot", "type buy leather strip", pre, "heat_strip_help", false},
		{f, 35, "strip", "type craft iron dagger", pre, "heat_craft_help", false},
		{f, 35, "craft", "", pre, "craft_turnin_told", true},
	})
}

func TestOvell9117_MidQuestGreetsAndAnswersEachStep(t *testing.T) {
	f36 := "36-pick_the_vein.yaml"
	pre := []string{"30-end", "35-end"}
	checkTopicSteps(t, "ovell", "9117.yaml", []topicStep{
		{f36, 36, "start", "type attack scavenger", pre, "vein_scavenger_help", false},
		{f36, 36, "scavenger", "type attack lurker", pre, "vein_lurker_help", false},
		{f36, 36, "lurker", "type forage", pre, "vein_forage_help", false},
		{f36, 36, "ore", "", pre, "vein_turnin", true},
		{"37-into_the_vein.yaml", 37, "start", "type attack beast", []string{"30-end", "35-end", "36-end"}, "beast_help", false},
		{"54-back_to_the_forge.yaml", 54, "start", "type craft iron dagger", []string{"30-end", "35-end", "36-end", "37-end"}, "forge_loop_help", false},
	})
}

// The forge spoke of Pothole Coulee (quests 35, 36, 37 and 54) under the owner
// ruling of 2026-10-17: one command per step, steps in order, each trigger
// gated on the step before it. These walk the shipped YAML with a player whose
// progress is one current step per quest, as a real character's is.
//
// Identifiers carry a forge prefix: sibling newbie_*_steps_test.go files share
// this package.

const (
	forgeSmithy     = 5245 // The Coulee Smithy: Rusk and the forge
	forgeOreStall   = 5246 // Ore Stall: no merchant, no forge
	forgeLowerTalus = 5249
	forgeScreeField = 5250
	forgeOrePocket  = 5252
	forgeUpperTalus = 5253
	forgeLowerDrift = 5258
	forgeDen        = 5260

	forgeIronIngot    = 40001
	forgeLeatherStrip = 40002
	forgeIronDagger   = 10009

	forgeSmithRusk  = 9116
	forgeScavenger  = 9118
	forgeLurker     = 9119
	forgeDriftMob   = 9122
	forgeStoneBeast = 9123
)

func TestQuest35FirstHeat_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "35-first_heat.yaml", 35)
	require.Equal(t, []string{"start", "list", "ingot", "strip", "craft", "end"}, newLinearPlayer(q).steps,
		"start and craft are the ids a saved character may hold today; the new steps sit between them")
	turnIn := turnInRequires(t, "9116.yaml", "craft_turnin")
	require.Equal(t, []string{"35-craft"}, turnIn, "the turn-in gates on the last step's token")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")

		// Buying before browsing does not skip the list step.
		w.notify("item_gain", EventDetails{ItemId: forgeIronIngot, RoomId: forgeSmithy})
		require.Equal(t, "start", w.p.current, "a purchase out of order must not skip the list step")

		w.notify("command", EventDetails{Command: "list", RoomId: forgeSmithy})
		require.Equal(t, "list", w.p.current)

		// The strip before the ingot does not skip the ingot step.
		w.notify("item_gain", EventDetails{ItemId: forgeLeatherStrip, RoomId: forgeSmithy})
		require.Equal(t, "list", w.p.current, "the strip out of order must not skip the ingot step")

		w.notify("item_gain", EventDetails{ItemId: forgeIronIngot, RoomId: forgeSmithy})
		require.Equal(t, "ingot", w.p.current)

		w.notify("item_gain", EventDetails{ItemId: forgeLeatherStrip, RoomId: forgeSmithy})
		require.Equal(t, "strip", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the craft")

		// Crafting away from the forge is not the forge lesson.
		w.notify("command", EventDetails{Command: "craft", RoomId: forgeOreStall})
		require.Equal(t, "strip", w.p.current)

		w.notify("command", EventDetails{Command: "craft", RoomId: forgeSmithy})
		require.Equal(t, "craft", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the craft")
	})

	t.Run("player saved on start with materials in hand can craft", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "craft", RoomId: forgeSmithy})
		require.Equal(t, "craft", w.p.current, "crafting proves the materials were bought (lenient grant)")
	})

	// #375 Task 10: `give dagger rusk` resolves as the topic that wants the
	// dagger (conversation.Give over DialogueFile.ItemTopic), not as a quest
	// item_give trigger.
	rusk := loadCouleeTopics(t, "9116.yaml")
	require.Equal(t, forgeSmithRusk, rusk.MobId)

	t.Run("player saved on craft can turn in by handing over the blade", func(t *testing.T) {
		w := newQuestWalker(q).at("craft")
		w.p.fullMockPlayer.quests["30-end"] = true
		w.p.items[forgeIronDagger] = true
		require.True(t, holdsAll(w.p, turnIn))
		n := newNpcTalker(rusk, w.p)
		tp, wanted := rusk.ItemTopic(forgeIronDagger, n.state())
		require.True(t, wanted, "Rusk wants the dagger")
		require.NotNil(t, tp, "giving the dagger on the craft step must play a hand-in")
		require.Equal(t, "craft_turnin", tp.Key)
		require.True(t, rusk.Apply(tp, n.state()))
		require.Equal(t, "end", w.p.current)
		require.True(t, w.p.items[forgeIronDagger], "Rusk looks the dagger over and hands it back")
	})

	t.Run("handing over a blade before the craft does not finish the quest", func(t *testing.T) {
		w := newQuestWalker(q).at("strip")
		w.p.fullMockPlayer.quests["30-end"] = true
		w.p.items[forgeIronDagger] = true
		tp, wanted := rusk.ItemTopic(forgeIronDagger, newNpcTalker(rusk, w.p).state())
		require.True(t, wanted, "Rusk wants the dagger, so give hands it back instead of keeping it")
		require.Nil(t, tp, "no hand-in is open before the craft")
		require.Equal(t, "strip", w.p.current)
	})
}

func TestQuest36PickTheVein_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "36-pick_the_vein.yaml", 36)
	require.Equal(t, []string{"start", "scavenger", "lurker", "ore", "end"}, newLinearPlayer(q).steps,
		"the step ids are what a saved character stores; keep them so a player mid-quest resumes")
	turnIn := turnInRequires(t, "9117.yaml", "vein_turnin")
	require.Equal(t, []string{"36-ore"}, turnIn, "the turn-in gates on the last step's token")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")

		w.notify("command", EventDetails{Command: "forage", RoomId: forgeScreeField})
		require.Equal(t, "start", w.p.current, "a forage first must not skip both fights")
		w.notify("mob_death", EventDetails{MobId: forgeLurker, RoomId: forgeOrePocket})
		require.Equal(t, "start", w.p.current, "the lurker first must not skip the scavenger")

		w.notify("mob_death", EventDetails{MobId: forgeScavenger, RoomId: forgeLowerTalus})
		require.Equal(t, "scavenger", w.p.current)

		w.notify("command", EventDetails{Command: "forage", RoomId: forgeScreeField})
		require.Equal(t, "scavenger", w.p.current, "a forage must not skip the lurker")

		w.notify("mob_death", EventDetails{MobId: forgeLurker, RoomId: forgeUpperTalus})
		require.Equal(t, "lurker", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the forage")

		// Foraging off the vein does not count.
		w.notify("command", EventDetails{Command: "forage", RoomId: forgeLowerTalus})
		require.Equal(t, "lurker", w.p.current)

		w.notify("command", EventDetails{Command: "forage", RoomId: forgeOrePocket})
		require.Equal(t, "ore", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the forage")
	})

	t.Run("player saved on scavenger resumes with the lurker", func(t *testing.T) {
		w := newQuestWalker(q).at("scavenger")
		w.notify("mob_death", EventDetails{MobId: forgeLurker, RoomId: forgeOrePocket})
		require.Equal(t, "lurker", w.p.current)
	})

	t.Run("player saved on lurker resumes with the forage", func(t *testing.T) {
		w := newQuestWalker(q).at("lurker")
		w.notify("command", EventDetails{Command: "forage", RoomId: forgeScreeField})
		require.Equal(t, "ore", w.p.current)
	})

	t.Run("player saved on ore can turn in", func(t *testing.T) {
		w := newQuestWalker(q).at("ore")
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("forage lines do not claim ore was found", func(t *testing.T) {
		for _, tr := range q.Triggers {
			if tr.Command != "forage" {
				continue
			}
			for _, a := range tr.Actions {
				low := strings.ToLower(a.SendText)
				for _, claim := range []string{"you pry", "you work a chunk", "pull ore", "you pulled"} {
					require.NotContains(t, low, claim,
						"forage can roll and find nothing, so its line must not claim ore: %q", a.SendText)
				}
			}
		}
	})
}

func TestQuest37IntoTheVein_OneCommand(t *testing.T) {
	q := loadShippedQuest(t, "37-into_the_vein.yaml", 37)
	require.Equal(t, []string{"start", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, q.Steps[0].Hint, darkAside, "the mine is pitch black: the start hint carries the dark aside")

	w := newQuestWalker(q).at("start")
	w.notify("mob_death", EventDetails{MobId: forgeDriftMob, RoomId: forgeLowerDrift})
	require.Equal(t, "start", w.p.current, "a lesser mine mob does not finish the quest")
	w.notify("mob_death", EventDetails{MobId: forgeStoneBeast, RoomId: forgeDen})
	require.Equal(t, "end", w.p.current)
}

func TestQuest54BackToTheForge_OneCommand(t *testing.T) {
	q := loadShippedQuest(t, "54-back_to_the_forge.yaml", 54)
	require.Equal(t, []string{"start", "end"}, newLinearPlayer(q).steps)

	hint := strings.ToLower(q.Steps[0].Hint)
	require.Contains(t, hint, "type craft iron dagger", "the hint names the recipe to craft")
	require.NotContains(t, hint, "come back", "the quest finishes on the craft; there is nothing to come back for")
	for _, tr := range q.Triggers {
		for _, a := range tr.Actions {
			low := strings.ToLower(a.SendText)
			require.NotContains(t, low, "report back", "the quest finishes on the craft: %q", a.SendText)
		}
	}

	w := newQuestWalker(q).at("start")
	w.notify("command", EventDetails{Command: "craft", RoomId: forgeOrePocket})
	require.Equal(t, "start", w.p.current, "crafting away from the forge does not count")
	w.notify("command", EventDetails{Command: "craft", RoomId: forgeSmithy})
	require.Equal(t, "end", w.p.current)
}
