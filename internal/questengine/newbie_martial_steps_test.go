package questengine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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

	// The opener names each step's one command and a quest-aware topic on
	// Vorn's menu answers it; the verbosity step's hand-in completes the
	// quest. The retired consider step still answers with the verbosity
	// command, so a player saved on it is not stranded.
	t.Run("Vorn's greeting names the step's one command", func(t *testing.T) {
		f := "32-first_blood.yaml"
		pre := []string{"30-end"}
		carriesCudgel := func(p *linearPlayer) { p.items[martialCudgel] = true }
		checkTopicStepsWith(t, "vorn", "9108.yaml", carriesCudgel, []topicStep{
			{f, 32, "start", "type wield cudgel", pre, "progress_start", false},
			{f, 32, "wield", "type consider dummy", pre, "progress_wield", false},
			{f, 32, "sized_up", "type attack dummy", pre, "progress_sized_up", false},
			{f, 32, "strike", "type kick dummy", pre, "progress_strike", false},
			{f, 32, "special", "type set combatverbosity", pre, "progress_special", false},
			{f, 32, "consider", "type set combatverbosity", pre, "progress_special", false},
			{f, 32, "verbosity", "", pre, "train_turnin", true},
		})
	})

	t.Run("a lost cudgel is replaced once", func(t *testing.T) {
		vorn := loadCouleeTopics(t, "9108.yaml")
		found := 0
		for i := range vorn.Topics {
			if tp := &vorn.Topics[i]; tp.GivesItem == martialCudgel && tp.GrantsQuest == "" {
				found++
			}
		}
		require.Equal(t, 1, found, "Vorn replaces a lost cudgel mid-quest through exactly one topic")
		n := newNpcTalker(vorn, playerOn(q, "start", "30-end"))
		checkHandout(t, n, "cudgel_replace", "cudgel_have", martialCudgel)
	})
}

func TestQuest33HoldTheWash_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "33-hold_the_wash.yaml", 33)
	steps := martialStepsKept(t, q, []string{"start", "scout", "squatter", "end"})
	require.Equal(t, []string{"start", "scout", "squatter", "end"}, steps)
	turnIn := turnInRequires(t, "9112.yaml", "wash_turnin")
	require.Equal(t, []string{"33-squatter"}, turnIn, "the turn-in gates on the last step's token")

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
		f := "33-hold_the_wash.yaml"
		pre := []string{"30-end", "32-end"}
		checkTopicSteps(t, "garve", "9112.yaml", []topicStep{
			{f, 33, "start", "type attack scout", pre, "wash_progress_start", false},
			{f, 33, "scout", "type attack squatter", pre, "wash_progress_scout", false},
			{f, 33, "squatter", "", pre, "wash_turnin", true},
		})
	})
}

func TestQuest34TakeTheTower_OneCommand(t *testing.T) {
	q := loadShippedQuest(t, "34-take_the_tower.yaml", 34)
	require.Equal(t, []string{"start", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, q.Steps[0].Hint, darkAside)
	require.Contains(t, strings.ToLower(q.Steps[0].Hint), "type attack captain")

	for _, boon := range []string{"might", "finesse"} {
		w := newQuestWalker(q).at("start")
		w.p.flags["34-boon"] = boon
		w.notify("mob_death", EventDetails{MobId: martialCaptain, RoomId: 5242})
		require.Equal(t, "end", w.p.current, "the %s path completes on the captain's death", boon)
	}

	might := func(p *linearPlayer) { p.flags["34-boon"] = "might" }
	checkTopicStepsWith(t, "garve", "9112.yaml", might, []topicStep{
		{"34-take_the_tower.yaml", 34, "start", "type attack captain", []string{"30-end", "32-end", "33-end"}, "tower_progress", false},
	})
}

func TestQuest53CullTheWash_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "53-cull_the_wash.yaml", 53)
	require.Equal(t, []string{"start", "firstkill", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, q.Steps[0].Hint, darkAside)

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
		f := "53-cull_the_wash.yaml"
		pre := []string{"30-end", "32-end", "33-end", "34-end"}
		checkTopicSteps(t, "garve", "9112.yaml", []topicStep{
			{f, 53, "start", "type attack scout", pre, "cull_progress_start", false},
			{f, 53, "firstkill", "type attack squatter", pre, "cull_progress_firstkill", false},
		})
	})
}
