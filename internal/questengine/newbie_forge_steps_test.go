package questengine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The smithing line of Pothole Coulee (#375 phase 7): Crier Toke offers First
// Heat (35) and Smith Rusk takes it; Rusk offers Pick the Vein (36) and
// Survivor Ovell takes it; Ovell offers Into the Vein (37), then Report to
// Toke (Smithing) (91) and the repeatable Back to the Forge (54). One command
// per step, each trigger gated on the step before it, every quest ends at a
// hand-in. The forge prefix keeps identifiers apart from the sibling
// newbie_*_steps_test.go files in this package.

const (
	forgeSmithy     = 5245 // The Coulee Smithy: Rusk and the forge
	forgeOreStall   = 5246 // Ore Stall: no merchant, no forge
	forgeLowerTalus = 5249
	forgeScreeField = 5250
	forgeCollapsed  = 5251 // Collapsed Cut: Ovell
	forgeOrePocket  = 5252
	forgeUpperTalus = 5253
	forgeLowerDrift = 5258
	forgeDen        = 5260

	forgeIronIngot    = 40001
	forgeLeatherStrip = 40002
	forgeIronDagger   = 10009

	forgeSmithRusk  = 9116
	forgeOvell      = 9117
	forgeScavenger  = 9118
	forgeLurker     = 9119
	forgeDriftMob   = 9122
	forgeStoneBeast = 9123
)

func TestRusk9116_MidQuestGreetsAndAnswersEachStep(t *testing.T) {
	f := "35-first_heat.yaml"
	pre := []string{"30-end", "31-end"}
	checkTopicSteps(t, "rusk", "9116.yaml", []topicStep{
		{f, 35, "met", "type list", pre, "heat_list_help", false},
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
		{f36, 36, "met", "type attack scavenger", pre, "vein_scavenger_help", false},
		{f36, 36, "scavenger", "type attack lurker", pre, "vein_lurker_help", false},
		{f36, 36, "lurker", "type forage", pre, "vein_forage_help", false},
		{f36, 36, "ore", "", pre, "vein_turnin", true},
		{"37-into_the_vein.yaml", 37, "start", "type attack beast", []string{"30-end", "35-end", "36-end"}, "beast_help", false},
		{"37-into_the_vein.yaml", 37, "slain", "", []string{"30-end", "35-end", "36-end"}, "mine_in", true},
		{"54-back_to_the_forge.yaml", 54, "start", "type craft iron dagger", []string{"30-end", "35-end", "36-end", "37-end"}, "forge_loop_help", false},
		{"54-back_to_the_forge.yaml", 54, "crafted", "", []string{"30-end", "35-end", "36-end", "37-end"}, "forge_loop_in", true},
	})
}

func TestQuest35FirstHeat_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "35-first_heat.yaml", 35)
	require.Equal(t, []string{"start", "met", "list", "ingot", "strip", "craft", "end"}, newLinearPlayer(q).steps,
		"start, list, ingot, strip and craft are ids a saved character may hold; met is inserted after start")
	turnIn := turnInRequires(t, "9116.yaml", "craft_turnin")
	require.Equal(t, []string{"35-craft"}, turnIn, "the turn-in gates on the last step's token")
	heard := EventDetails{MobId: forgeSmithRusk, Topic: "heat_arrive", RoomId: forgeSmithy}

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "list", RoomId: forgeSmithy})
		require.Equal(t, "start", w.p.current, "listing before Rusk has spoken must not skip the talk")
		w.notify("topic_heard", heard)
		require.Equal(t, "met", w.p.current)

		w.notify("item_gain", EventDetails{ItemId: forgeIronIngot, RoomId: forgeSmithy})
		require.Equal(t, "met", w.p.current, "a purchase out of order must not skip the list step")
		w.notify("command", EventDetails{Command: "list", RoomId: forgeSmithy})
		require.Equal(t, "list", w.p.current)

		w.notify("item_gain", EventDetails{ItemId: forgeLeatherStrip, RoomId: forgeSmithy})
		require.Equal(t, "list", w.p.current, "the strip out of order must not skip the ingot step")
		w.notify("item_gain", EventDetails{ItemId: forgeIronIngot, RoomId: forgeSmithy})
		require.Equal(t, "ingot", w.p.current)

		w.notify("command", EventDetails{Command: "craft", RoomId: forgeSmithy})
		require.Equal(t, "ingot", w.p.current, "a craft before the strip must not skip it (no lenient grant)")
		w.notify("item_gain", EventDetails{ItemId: forgeLeatherStrip, RoomId: forgeSmithy})
		require.Equal(t, "strip", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the craft")

		w.notify("command", EventDetails{Command: "craft", RoomId: forgeOreStall})
		require.Equal(t, "strip", w.p.current, "crafting away from the forge is not the forge lesson")
		w.notify("command", EventDetails{Command: "craft", RoomId: forgeSmithy})
		require.Equal(t, "craft", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the craft")
	})

	t.Run("the craft line names the sell route and the talk", func(t *testing.T) {
		found := false
		for _, tr := range q.Triggers {
			if tr.Command != "craft" {
				continue
			}
			for _, a := range tr.Actions {
				if a.SendText == "" {
					continue
				}
				found = true
				low := squash(a.SendText)
				require.Contains(t, low, "sell", "#697: the post-craft line mentions the sell route")
				require.Contains(t, low, "type talk rusk")
			}
		}
		require.True(t, found, "the craft trigger has a line")
	})

	t.Run("Rusk's talk step", func(t *testing.T) {
		checkArriveTopic(t, "rusk", "9116.yaml", q, forgeSmithRusk, "heat_arrive", []string{"30-end", "31-end"})
	})

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
	require.Equal(t, []string{"start", "met", "scavenger", "lurker", "ore", "end"}, newLinearPlayer(q).steps)
	turnIn := turnInRequires(t, "9117.yaml", "vein_turnin")
	require.Equal(t, []string{"36-ore"}, turnIn, "the turn-in gates on the last step's token")
	heard := EventDetails{MobId: forgeOvell, Topic: "vein_arrive", RoomId: forgeCollapsed}

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("mob_death", EventDetails{MobId: forgeScavenger, RoomId: forgeLowerTalus})
		require.Equal(t, "start", w.p.current, "a kill before Ovell has spoken must not skip the talk")
		w.notify("topic_heard", heard)
		require.Equal(t, "met", w.p.current)

		w.notify("command", EventDetails{Command: "forage", RoomId: forgeScreeField})
		require.Equal(t, "met", w.p.current, "a forage first must not skip both fights")
		w.notify("mob_death", EventDetails{MobId: forgeLurker, RoomId: forgeOrePocket})
		require.Equal(t, "met", w.p.current, "the lurker first must not skip the scavenger")

		w.notify("mob_death", EventDetails{MobId: forgeScavenger, RoomId: forgeLowerTalus})
		require.Equal(t, "scavenger", w.p.current)
		w.notify("command", EventDetails{Command: "forage", RoomId: forgeScreeField})
		require.Equal(t, "scavenger", w.p.current, "a forage must not skip the lurker")

		w.notify("mob_death", EventDetails{MobId: forgeLurker, RoomId: forgeUpperTalus})
		require.Equal(t, "lurker", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the forage")

		w.notify("command", EventDetails{Command: "forage", RoomId: forgeLowerTalus})
		require.Equal(t, "lurker", w.p.current, "foraging off the vein does not count")
		w.notify("command", EventDetails{Command: "forage", RoomId: forgeOrePocket})
		require.Equal(t, "ore", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the forage")
	})

	t.Run("the first fight carries the dark aside", func(t *testing.T) {
		require.Contains(t, q.Steps[1].Hint, darkAside)
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

	t.Run("Rusk offers it and Ovell no longer does", func(t *testing.T) {
		checkOfferMoved(t, "9116.yaml", "vein_offer", q, "35-end", "9117.yaml")
	})

	t.Run("Ovell's talk step", func(t *testing.T) {
		checkArriveTopic(t, "ovell", "9117.yaml", q, forgeOvell, "vein_arrive", []string{"30-end", "35-end"})
	})
}

func TestQuest37IntoTheVein_KillThenReport(t *testing.T) {
	q := loadShippedQuest(t, "37-into_the_vein.yaml", 37)
	require.Equal(t, []string{"start", "slain", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, q.Steps[0].Hint, darkAside, "the mine is pitch black: the start hint carries the dark aside")
	turnIn := turnInRequires(t, "9117.yaml", "mine_in")
	require.Equal(t, []string{"37-slain"}, turnIn)

	w := newQuestWalker(q).at("start")
	w.notify("mob_death", EventDetails{MobId: forgeDriftMob, RoomId: forgeLowerDrift})
	require.Equal(t, "start", w.p.current, "a lesser mine mob does not count")
	w.notify("mob_death", EventDetails{MobId: forgeStoneBeast, RoomId: forgeDen})
	require.Equal(t, "slain", w.p.current)
	require.False(t, holdsAll(w.p, []string{"37-end"}), "the kill no longer ends the quest")
}

func TestQuest54BackToTheForge_EndsAtOvell(t *testing.T) {
	q := loadShippedQuest(t, "54-back_to_the_forge.yaml", 54)
	require.Equal(t, []string{"start", "crafted", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, strings.ToLower(q.Steps[0].Hint), "type craft iron dagger", "the hint names the recipe to craft")
	require.Contains(t, questHintFor(t, q, "crafted"), "type talk ovell")
	turnIn := turnInRequires(t, "9117.yaml", "forge_loop_in")
	require.Equal(t, []string{"54-crafted"}, turnIn)

	w := newQuestWalker(q).at("start")
	w.notify("command", EventDetails{Command: "craft", RoomId: forgeOrePocket})
	require.Equal(t, "start", w.p.current, "crafting away from the forge does not count")
	w.notify("command", EventDetails{Command: "craft", RoomId: forgeSmithy})
	require.Equal(t, "crafted", w.p.current)
	require.False(t, holdsAll(w.p, []string{"54-end"}), "the craft no longer ends the run")
}
