package questengine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	yamlv2 "gopkg.in/yaml.v2"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
)

// The martial spoke of Pothole Coulee: First Blood (32, Drillmaster Vorn),
// Hold the Wash (33), Take the Tower (34) and Cull the Wash (53, all three
// Caravan Guard Garve). Owner ruling 2026-10-17: one command per step, the
// steps in order, each step's trigger gated on the step before it, and the
// givers' greetings saying the one command of the step the player is on.

const (
	martialDummy     = 9109
	martialScout     = 9110
	martialSquatter  = 9111
	martialCaptain   = 9115
	martialCudgel    = 10043
	martialDrillYard = 5227
)

// martialLoadDialogue parses a shipped Pothole Coulee dialogue file the way
// the loader does (yaml.v2), so a bare-scalar list field fails here too
// instead of muting the NPC in game.
func martialLoadDialogue(t *testing.T, file string) *dialogue.DialogueFile {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "dialogue", "pothole_coulee", file)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var df dialogue.DialogueFile
	require.NoError(t, yamlv2.Unmarshal(data, &df), "%s must parse or the NPC goes mute", file)
	require.NotNil(t, df.Tree, "%s has no tree", file)
	return &df
}

// done marks earlier quests finished, so the giver's prerequisite variants
// (Opened, trained, and so on) step aside.
func (w *questWalker) martialDone(tokens ...string) *questWalker {
	for _, tok := range tokens {
		w.p.fullMockPlayer.quests[tok] = true
	}
	return w
}

func (w *questWalker) martialState() *dialogue.PlayerState {
	carries := func(id int) bool { return w.p.items[id] }
	return &dialogue.PlayerState{
		HasQuest:     w.p.HasQuest,
		HasItem:      carries,
		CarriesItem:  carries,
		GetQuestFlag: func(k string) string { return w.p.flags[k] },
	}
}

// greetingHints is what `talk <giver>` shows the player at their current step.
func (w *questWalker) martialGreeting(t *testing.T, df *dialogue.DialogueFile) string {
	t.Helper()
	_, hints, ok := dialogue.Greet(df, 970001, 1, w.martialState())
	require.True(t, ok)
	return hints
}

// ask answers `ask <giver> <topic>` the way usercommands.deliverDialogue
// does: tree nodes first, then patterns. fallback is true when only the
// empty-keyword catch-all answered.
func (w *questWalker) martialAsk(df *dialogue.DialogueFile, topic string) (text, hints string, fallback bool) {
	if txt, h, _, ok := dialogue.TreeAdvance(df, 970002, 1, topic, w.martialState()); ok {
		return txt, h, false
	}
	resp, _, ok, usedFallback := dialogue.MatchWithFallbackInfo(df, 970002, topic, w.martialState())
	if !ok {
		return "", "", true
	}
	return resp, "", usedFallback
}

// martialRequireGreeting checks the giver's greeting at one step names exactly
// one command, and that it is the step's command.
func martialRequireGreeting(t *testing.T, w *questWalker, df *dialogue.DialogueFile, step, command string) {
	t.Helper()
	hints := w.martialGreeting(t, df)
	n := len(typedCommand.FindAllString(strings.ReplaceAll(hints, darkAside, ""), -1))
	require.Equal(t, 1, n, "greeting on step %q must name one command: %q", step, hints)
	require.Contains(t, strings.ToLower(hints), command, "greeting on step %q must name %q: %q", step, command, hints)
}

// martialRequireAnswer checks that a mid-quest question gets a real answer, not
// the giver's catch-all filler.
func martialRequireAnswer(t *testing.T, w *questWalker, df *dialogue.DialogueFile, step, topic string) {
	t.Helper()
	text, _, fallback := w.martialAsk(df, topic)
	require.False(t, fallback, "on step %q, ask %q fell through to the catch-all (%q)", step, topic, text)
	require.NotEmpty(t, text)
	require.NotContains(t, text, "no task for you", "on step %q, ask %q", step, topic)
}

func martialStepsKept(t *testing.T, q *QuestDef, old []string) []string {
	t.Helper()
	steps := newLinearPlayer(q).steps
	last := -1
	for _, s := range old {
		i := (&linearPlayer{steps: steps}).stepIndex(s)
		require.GreaterOrEqual(t, i, 0, "step %q must survive so a player saved on it resumes", s)
		require.Greater(t, i, last, "old step %q must keep its place in the order", s)
		last = i
	}
	return steps
}

func TestQuest32FirstBlood_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "32-first_blood.yaml", 32)
	steps := martialStepsKept(t, q, []string{"start", "wield", "strike", "special", "consider", "verbosity", "end"})
	// Consider comes before the fight (owner 2026-10-19): sized_up is the
	// new consider step. The old consider step is no longer granted and stays
	// only so a player saved on it resumes.
	require.Equal(t, []string{"start", "wield", "sized_up", "strike", "special", "consider", "verbosity", "end"}, steps)
	turnIn := turnInRequires(t, "9108.yaml", "train_turnin")
	require.Equal(t, []string{"32-verbosity"}, turnIn, "the turn-in gates on the last step's token")
	vorn := martialLoadDialogue(t, "9108.yaml")

	cmd := func(c string) EventDetails { return EventDetails{Command: c, RoomId: martialDrillYard} }
	kill := EventDetails{MobId: martialDummy, RoomId: martialDrillYard}

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")

		w.notify("mob_death", kill)
		require.Equal(t, "start", w.p.current, "a kill before wielding must not skip the wield step")
		// The equip event names its item (2026-10-17 newbie-duo playtest):
		// wielding the sling is not wielding the cudgel.
		w.notify("command", EventDetails{Command: "equip", RoomId: martialDrillYard, ItemId: 10038})
		require.Equal(t, "start", w.p.current, "wielding the sling must not finish the cudgel step")
		w.notify("command", EventDetails{Command: "equip", RoomId: martialDrillYard, ItemId: martialCudgel})
		require.Equal(t, "wield", w.p.current)

		w.notify("mob_death", kill)
		require.Equal(t, "wield", w.p.current, "a kill before consider must not skip it")
		w.notify("command", cmd("kick"))
		require.Equal(t, "wield", w.p.current, "a kick before consider must not skip it")
		w.notify("command", cmd("consider"))
		require.Equal(t, "sized_up", w.p.current)

		w.notify("command", cmd("kick"))
		require.Equal(t, "sized_up", w.p.current, "a kick before the kill must not skip the fight")
		w.notify("mob_death", kill)
		require.Equal(t, "strike", w.p.current)

		w.notify("command", cmd("combatverbosity"))
		require.Equal(t, "strike", w.p.current, "verbosity before the special move must not skip it")
		w.notify("command", cmd("kick"))
		require.Equal(t, "special", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the verbosity step")

		w.notify("command", cmd("consider"))
		require.Equal(t, "special", w.p.current, "a second consider grants nothing")
		w.notify("command", cmd("combatverbosity"))
		require.Equal(t, "verbosity", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the last step")
	})

	t.Run("trip counts as the special move", func(t *testing.T) {
		w := newQuestWalker(q).at("strike")
		w.notify("command", cmd("trip"))
		require.Equal(t, "special", w.p.current)
	})

	t.Run("a refused kick or consider does not count", func(t *testing.T) {
		w := newQuestWalker(q).at("strike")
		w.notify("command_issued", cmd("kick"))
		w.notify("command_issued", cmd("trip"))
		require.Equal(t, "strike", w.p.current, "command_issued fires on refusals too")
		w = newQuestWalker(q).at("wield")
		w.notify("command_issued", cmd("consider"))
		require.Equal(t, "wield", w.p.current)
	})

	t.Run("players saved on each old step resume", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "equip", RoomId: martialDrillYard, ItemId: martialCudgel})
		require.Equal(t, "wield", w.p.current, "saved on start: the hint says wield, and wielding advances")

		// Saved on wield before consider moved first: the hint now says
		// consider, and the fight waits for it.
		w = newQuestWalker(q).at("wield")
		w.notify("command", cmd("consider"))
		require.Equal(t, "sized_up", w.p.current)

		w = newQuestWalker(q).at("strike")
		w.notify("command", cmd("kick"))
		require.Equal(t, "special", w.p.current)

		w = newQuestWalker(q).at("special")
		w.notify("command", cmd("combatverbosity"))
		require.Equal(t, "verbosity", w.p.current)

		w = newQuestWalker(q).at("consider")
		w.notify("command", cmd("combatverbosity"))
		require.Equal(t, "verbosity", w.p.current)

		w = newQuestWalker(q).at("verbosity")
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("Vorn's greeting names the step's one command", func(t *testing.T) {
		for _, c := range []struct{ step, command string }{
			{"start", "wield cudgel"},
			{"wield", "consider dummy"},
			{"sized_up", "attack dummy"},
			{"strike", "kick dummy"},
			{"special", "set combatverbosity"},
			{"consider", "set combatverbosity"},
			{"verbosity", "ask vorn report"},
		} {
			martialRequireGreeting(t, newQuestWalker(q).at(c.step).martialDone("30-end"), vorn, c.step, c.command)
		}
	})

	t.Run("mid-quest questions get a quest answer", func(t *testing.T) {
		for _, step := range []string{"start", "wield", "sized_up", "strike", "special", "consider"} {
			for _, topic := range []string{"kick", "trip", "dummy", "cudgel", "help", "how", "report", "quest", "consider", "attack"} {
				w := newQuestWalker(q).at(step).martialDone("30-end")
				w.p.items[martialCudgel] = true
				martialRequireAnswer(t, w, vorn, step, topic)
			}
		}
	})

	t.Run("a lost cudgel is replaced once", func(t *testing.T) {
		var replace, already *dialogue.TreeNode
		for i := range vorn.Tree.Nodes {
			n := &vorn.Tree.Nodes[i]
			if n.GivesItem == martialCudgel && n.GrantsQuest == "" {
				replace = n
				if i+1 < len(vorn.Tree.Nodes) {
					already = &vorn.Tree.Nodes[i+1]
				}
			}
		}
		require.NotNil(t, replace, "Vorn must replace a lost cudgel mid-quest")
		require.Equal(t, martialCudgel, replace.ItemExcluded, "no second cudgel for a player who carries one")
		require.NotNil(t, already)
		require.Equal(t, replace.Triggers, already.Triggers, "the already-have-one answer follows on the same words")
		require.Zero(t, already.GivesItem)
	})
}

func TestQuest33HoldTheWash_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "33-hold_the_wash.yaml", 33)
	steps := martialStepsKept(t, q, []string{"start", "scout", "squatter", "end"})
	require.Equal(t, []string{"start", "scout", "squatter", "end"}, steps)
	turnIn := turnInRequires(t, "9112.yaml", "wash_turnin")
	require.Equal(t, []string{"33-squatter"}, turnIn, "the turn-in gates on the last step's token")
	garve := martialLoadDialogue(t, "9112.yaml")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("mob_death", EventDetails{MobId: martialSquatter, RoomId: 5235})
		require.Equal(t, "start", w.p.current, "the squatter before the scout must not skip the scout")
		require.False(t, holdsAll(w.p, turnIn))

		w.notify("mob_death", EventDetails{MobId: martialScout, RoomId: 5232})
		require.Equal(t, "scout", w.p.current)
		w.notify("mob_death", EventDetails{MobId: martialScout, RoomId: 5233})
		require.Equal(t, "scout", w.p.current, "a second scout is not the squatter")
		require.False(t, holdsAll(w.p, turnIn))

		w.notify("mob_death", EventDetails{MobId: martialSquatter, RoomId: 5235})
		require.Equal(t, "squatter", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("players saved on each old step resume", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("mob_death", EventDetails{MobId: martialScout, RoomId: 5232})
		require.Equal(t, "scout", w.p.current)

		w = newQuestWalker(q).at("scout")
		w.notify("mob_death", EventDetails{MobId: martialSquatter, RoomId: 5236})
		require.Equal(t, "squatter", w.p.current)

		w = newQuestWalker(q).at("squatter")
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("start hint carries the dark aside", func(t *testing.T) {
		require.Contains(t, q.Steps[0].Hint, darkAside)
	})

	t.Run("Garve's greeting names the step's one command", func(t *testing.T) {
		for _, c := range []struct{ step, command string }{
			{"start", "attack scout"},
			{"scout", "attack squatter"},
			{"squatter", "ask garve report"},
		} {
			martialRequireGreeting(t, newQuestWalker(q).at(c.step).martialDone("30-end", "32-end"), garve, c.step, c.command)
		}
	})

	t.Run("mid-quest questions get a quest answer", func(t *testing.T) {
		for _, step := range []string{"start", "scout"} {
			for _, topic := range []string{"scout", "squatter", "help", "how", "report", "quest", "wash", "flee", "cistern"} {
				w := newQuestWalker(q).at(step).martialDone("30-end", "32-end")
				martialRequireAnswer(t, w, garve, step, topic)
			}
		}
	})
}

func TestQuest34TakeTheTower_OneCommand(t *testing.T) {
	q := loadShippedQuest(t, "34-take_the_tower.yaml", 34)
	require.Equal(t, []string{"start", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, q.Steps[0].Hint, darkAside)
	require.Contains(t, strings.ToLower(q.Steps[0].Hint), "type attack captain")
	garve := martialLoadDialogue(t, "9112.yaml")

	for _, boon := range []string{"might", "finesse"} {
		w := newQuestWalker(q).at("start")
		w.p.flags["34-boon"] = boon
		w.notify("mob_death", EventDetails{MobId: martialCaptain, RoomId: 5242})
		require.Equal(t, "end", w.p.current, "the %s path completes on the captain's death", boon)
	}

	w := newQuestWalker(q).at("start").martialDone("30-end", "32-end", "33-end")
	w.p.flags["34-boon"] = "might"
	martialRequireGreeting(t, w, garve, "start", "attack captain")
	for _, topic := range []string{"captain", "tower", "help", "how", "report", "quest"} {
		martialRequireAnswer(t, w, garve, "start", topic)
	}
}

func TestQuest53CullTheWash_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "53-cull_the_wash.yaml", 53)
	require.Equal(t, []string{"start", "firstkill", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, q.Steps[0].Hint, darkAside)
	garve := martialLoadDialogue(t, "9112.yaml")

	// The quest finishes on the squatter's death; no step may send the
	// player back to Garve for a turn-in that does not exist.
	for _, s := range q.Steps {
		low := strings.ToLower(s.Hint + " " + s.Description)
		require.NotContains(t, low, "come back", "step %q: the quest ends on the kill", s.Id)
		require.NotContains(t, low, "report", "step %q: the quest ends on the kill", s.Id)
	}
	require.Contains(t, strings.ToLower(q.Steps[1].Hint), "squatter", "the second kill is the squatter")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("mob_death", EventDetails{MobId: martialSquatter, RoomId: 5235})
		require.Equal(t, "start", w.p.current, "the squatter first must not count")
		w.notify("mob_death", EventDetails{MobId: martialScout, RoomId: 5232})
		require.Equal(t, "firstkill", w.p.current)
		w.notify("mob_death", EventDetails{MobId: martialScout, RoomId: 5233})
		require.Equal(t, "firstkill", w.p.current, "a second scout is not the squatter")
		w.notify("mob_death", EventDetails{MobId: martialSquatter, RoomId: 5235})
		require.Equal(t, "end", w.p.current)
	})

	t.Run("Garve's greeting names the step's one command", func(t *testing.T) {
		for _, c := range []struct{ step, command string }{
			{"start", "attack scout"},
			{"firstkill", "attack squatter"},
		} {
			w := newQuestWalker(q).at(c.step).martialDone("30-end", "32-end", "33-end", "34-end")
			martialRequireGreeting(t, w, garve, c.step, c.command)
			for _, topic := range []string{"scout", "squatter", "help", "how", "report", "quest", "work"} {
				martialRequireAnswer(t, w, garve, c.step, topic)
			}
		}
	})
}
