package questengine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// forgeDialogue loads a shipped Pothole Coulee dialogue file.
func forgeDialogue(t *testing.T, file string) *dialogue.DialogueFile {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "dialogue", "pothole_coulee", file))
	require.NoError(t, err)
	var df dialogue.DialogueFile
	require.NoError(t, yaml.Unmarshal(raw, &df), "dialogue %s must parse, or the NPC goes mute", file)
	require.NotNil(t, df.Tree)
	return &df
}

var forgeTalkInstance = 71000

// forgeAsk answers a topic the way usercommands/ask.go does: tree nodes
// first, then patterns. catchAll is true when the empty-keyword pattern
// answered.
func forgeAsk(df *dialogue.DialogueFile, p *linearPlayer, topic string) (hints string, fromTree, catchAll bool) {
	forgeTalkInstance++
	ps := &dialogue.PlayerState{HasQuest: p.HasQuest, HasItem: func(int) bool { return false }}
	if _, h, _, ok := dialogue.TreeAdvance(df, forgeTalkInstance, 1, topic, ps); ok {
		return h, true, false
	}
	_, _, _, fallback := dialogue.MatchWithFallbackInfo(df, forgeTalkInstance, topic, ps)
	return "", false, fallback
}

func forgeGreetHints(df *dialogue.DialogueFile, p *linearPlayer) string {
	forgeTalkInstance++
	ps := &dialogue.PlayerState{HasQuest: p.HasQuest, HasItem: func(int) bool { return false }}
	_, h, _ := dialogue.Greet(df, forgeTalkInstance, 1, ps)
	return h
}

func forgeSquash(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

// forgeGiverMatchesSteps: on each step the giver's greeting hint and the
// answer to every quest word name that step's own command, never the
// catch-all.
func forgeGiverMatchesSteps(t *testing.T, q *QuestDef, file string, prereqs []string, commands map[string]string, words []string) {
	t.Helper()
	df := forgeDialogue(t, file)
	for step, cmd := range commands {
		p := newLinearPlayer(q)
		p.current = step
		for _, tok := range prereqs {
			p.fullMockPlayer.quests[tok] = true
		}
		require.Contains(t, forgeSquash(forgeGreetHints(df, p)), cmd,
			"%s greeting on step %s must name the step's command", file, step)
		for _, w := range words {
			p2 := newLinearPlayer(q)
			p2.current = step
			for _, tok := range prereqs {
				p2.fullMockPlayer.quests[tok] = true
			}
			hints, fromTree, catchAll := forgeAsk(df, p2, w)
			require.False(t, catchAll, "%s: ask %q on step %s fell through to the catch-all", file, w, step)
			require.True(t, fromTree, "%s: ask %q on step %s must get a quest answer", file, w, step)
			require.Contains(t, forgeSquash(hints), "type ", "%s: ask %q on step %s names no command", file, w, step)
			require.Equal(t, step, p2.current, "%s: ask %q on step %s must not move the quest", file, w, step)
		}
	}
}

func TestRusk9116_MidQuestGreetsAndAnswersEachStep(t *testing.T) {
	q := loadShippedQuest(t, "35-first_heat.yaml", 35)
	forgeGiverMatchesSteps(t, q, "9116.yaml", []string{"30-end"},
		map[string]string{
			"start": "type list",
			"list":  "type buy iron ingot",
			"ingot": "type buy leather strip",
			"strip": "type craft iron dagger",
			"craft": "type ask rusk done",
		},
		[]string{"ingot", "strip", "dagger", "craft", "list", "help", "how"})
}

func TestOvell9117_MidQuestGreetsAndAnswersEachStep(t *testing.T) {
	q36 := loadShippedQuest(t, "36-pick_the_vein.yaml", 36)
	forgeGiverMatchesSteps(t, q36, "9117.yaml", []string{"30-end", "35-end"},
		map[string]string{
			"start":     "type attack scavenger",
			"scavenger": "type attack lurker",
			"lurker":    "type forage",
		},
		[]string{"vein", "lurker", "scavenger", "forage", "help", "how"})

	// On the last step every quest word reaches the turn-in or names it.
	df := forgeDialogue(t, "9117.yaml")
	p := newLinearPlayer(q36)
	p.current = "ore"
	p.fullMockPlayer.quests["30-end"], p.fullMockPlayer.quests["35-end"] = true, true
	require.Contains(t, forgeSquash(forgeGreetHints(df, p)), "type ask ovell done")

	q37 := loadShippedQuest(t, "37-into_the_vein.yaml", 37)
	forgeGiverMatchesSteps(t, q37, "9117.yaml", []string{"30-end", "35-end", "36-end"},
		map[string]string{"start": "type attack beast"},
		[]string{"beast", "den", "help", "how"})

	q54 := loadShippedQuest(t, "54-back_to_the_forge.yaml", 54)
	forgeGiverMatchesSteps(t, q54, "9117.yaml", []string{"30-end", "35-end", "36-end", "37-end"},
		map[string]string{"start": "type craft iron dagger"},
		[]string{"forge", "craft", "work", "help", "how"})
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

	t.Run("player saved on craft can turn in by handing over the blade", func(t *testing.T) {
		w := newQuestWalker(q).at("craft")
		require.True(t, holdsAll(w.p, turnIn))
		w.notify("item_give", EventDetails{MobId: forgeSmithRusk, ItemId: forgeIronDagger, RoomId: forgeSmithy})
		require.Equal(t, "end", w.p.current)
	})

	t.Run("handing over a blade before the craft does not finish the quest", func(t *testing.T) {
		w := newQuestWalker(q).at("strip")
		w.notify("item_give", EventDetails{MobId: forgeSmithRusk, ItemId: forgeIronDagger, RoomId: forgeSmithy})
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
