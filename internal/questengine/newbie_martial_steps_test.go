package questengine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The fighting line of Pothole Coulee (#375 phase 7): Crier Toke offers
// First Blood (32) and Drillmaster Vorn takes its hand-in; Vorn offers Hold
// the Wash (33) and Caravan Guard Garve takes it; Garve offers Take the
// Tower (34), then Report to Toke (Fighting) (90) and the repeatable Cull
// the Wash (53). One command per step, each trigger gated on the step before
// it (owner ruling 2026-10-17), and every quest ends at a hand-in.

const (
	martialDummy     = 9109
	martialScout     = 9110
	martialSquatter  = 9111
	martialCaptain   = 9115
	martialCudgel    = 10043
	martialDrillYard = 5227
	martialHollow    = 5234
	martialVorn      = 9108
	martialGarve     = 9112
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
	steps := martialStepsKept(t, q, []string{"start", "wield", "strike", "special", "verbosity", "end"})
	require.Equal(t, []string{"start", "met", "wield", "sized_up", "strike", "special", "verbosity", "end"}, steps)
	turnIn := turnInRequires(t, "9108.yaml", "train_turnin")
	require.Equal(t, []string{"32-verbosity"}, turnIn, "the turn-in gates on the last step's token")

	cmd := func(c string) EventDetails { return EventDetails{Command: c, RoomId: martialDrillYard} }
	kill := EventDetails{MobId: martialDummy, RoomId: martialDrillYard}
	equipCudgel := EventDetails{Command: "equip", RoomId: martialDrillYard, ItemId: martialCudgel}
	heard := EventDetails{MobId: martialVorn, Topic: "drill_arrive", RoomId: martialDrillYard}

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", equipCudgel)
		require.Equal(t, "start", w.p.current, "wielding before Vorn has spoken must not skip the talk")
		w.notify("topic_heard", EventDetails{MobId: martialGarve, Topic: "drill_arrive", RoomId: martialHollow})
		require.Equal(t, "start", w.p.current, "only Vorn's topic is the talk step")
		w.notify("topic_heard", heard)
		require.Equal(t, "met", w.p.current)

		w.notify("mob_death", kill)
		require.Equal(t, "met", w.p.current, "a kill before wielding must not skip the wield step")
		w.notify("command", EventDetails{Command: "equip", RoomId: martialDrillYard, ItemId: 10038})
		require.Equal(t, "met", w.p.current, "wielding the sling must not finish the cudgel step")
		w.notify("command", equipCudgel)
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

	t.Run("players saved on each step resume", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("topic_heard", heard)
		require.Equal(t, "met", w.p.current, "saved on start: the hint says talk to Vorn, and hearing him advances")

		w = newQuestWalker(q).at("met")
		w.notify("command", equipCudgel)
		require.Equal(t, "wield", w.p.current)

		w = newQuestWalker(q).at("wield")
		w.notify("command", cmd("consider"))
		require.Equal(t, "sized_up", w.p.current)

		w = newQuestWalker(q).at("strike")
		w.notify("command", cmd("kick"))
		require.Equal(t, "special", w.p.current)

		w = newQuestWalker(q).at("special")
		w.notify("command", cmd("combatverbosity"))
		require.Equal(t, "verbosity", w.p.current)

		w = newQuestWalker(q).at("verbosity")
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("the old consider step moves saved players onto special", func(t *testing.T) {
		raw := string(repoFile(t, "_datafiles", "world", "dogmud", "quests", "step-ids.txt"))
		require.Contains(t, raw, "32-consider -> 32-special")
	})

	t.Run("Vorn's talk step", func(t *testing.T) {
		checkArriveTopic(t, "vorn", "9108.yaml", q, martialVorn, "drill_arrive", []string{"30-end", "31-end"})
	})

	t.Run("Vorn's greeting names the step's one command", func(t *testing.T) {
		f := "32-first_blood.yaml"
		pre := []string{"30-end", "31-end"}
		carriesCudgel := func(p *linearPlayer) { p.items[martialCudgel] = true }
		checkTopicStepsWith(t, "vorn", "9108.yaml", carriesCudgel, []topicStep{
			{f, 32, "met", "type wield cudgel", pre, "progress_start", false},
			{f, 32, "wield", "type consider dummy", pre, "progress_wield", false},
			{f, 32, "sized_up", "type attack dummy", pre, "progress_sized_up", false},
			{f, 32, "strike", "type kick dummy", pre, "progress_strike", false},
			{f, 32, "special", "type set combatverbosity", pre, "progress_special", false},
			{f, 32, "verbosity", "", pre, "train_turnin", true},
		})
	})

	// The trigger cannot see an early wield, and equip reads only the pack, so
	// every line that tells a player to wield names the remove as well.
	t.Run("a player already holding the cudgel is told to remove it first", func(t *testing.T) {
		vorn := loadCouleeTopics(t, "9108.yaml")
		_, openerHint := newNpcTalker(vorn, playerOn(q, "met", "30-end", "31-end")).opener()
		var heardLine string
		for _, trig := range q.Triggers {
			if trig.Topic == "drill_arrive" {
				for _, a := range trig.Actions {
					heardLine += a.SendText
				}
			}
		}
		for name, line := range map[string]string{
			"met hint":           questHintFor(t, q, "met"),
			"met description":    q.Steps[1].Description,
			"talk step line":     heardLine,
			"Vorn opener":        openerHint,
			"drill_arrive hints": vorn.FindTopic("drill_arrive").Hints,
			"progress_start":     vorn.FindTopic("progress_start").Hints,
		} {
			require.Contains(t, squash(line), "remove cudgel", name)
			require.Contains(t, squash(line), "wield cudgel", name)
		}
		require.Contains(t, squash(vorn.FindTopic("cudgel_have").Hints), "type inventory",
			"the reminder shows with the cudgel in hand, where a wield hint would be refused")
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
	require.Equal(t, []string{"start", "met", "scout", "squatter", "end"}, steps)
	turnIn := turnInRequires(t, "9112.yaml", "wash_turnin")
	require.Equal(t, []string{"33-squatter"}, turnIn, "the turn-in gates on the last step's token")
	heard := EventDetails{MobId: martialGarve, Topic: "wash_arrive", RoomId: martialHollow}

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("mob_death", EventDetails{MobId: martialScout, RoomId: 5232})
		require.Equal(t, "start", w.p.current, "a scout before Garve has spoken must not skip the talk")
		w.notify("topic_heard", heard)
		require.Equal(t, "met", w.p.current)

		w.notify("mob_death", EventDetails{MobId: martialSquatter, RoomId: 5235})
		require.Equal(t, "met", w.p.current, "the squatter before the scout must not skip the scout")
		w.notify("mob_death", EventDetails{MobId: martialScout, RoomId: 5232})
		require.Equal(t, "scout", w.p.current)
		w.notify("mob_death", EventDetails{MobId: martialScout, RoomId: 5233})
		require.Equal(t, "scout", w.p.current, "a second scout is not the squatter")
		require.False(t, holdsAll(w.p, turnIn))

		w.notify("mob_death", EventDetails{MobId: martialSquatter, RoomId: 5235})
		require.Equal(t, "squatter", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("players saved on each step resume", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("topic_heard", heard)
		require.Equal(t, "met", w.p.current)

		w = newQuestWalker(q).at("met")
		w.notify("mob_death", EventDetails{MobId: martialScout, RoomId: 5232})
		require.Equal(t, "scout", w.p.current)

		w = newQuestWalker(q).at("scout")
		w.notify("mob_death", EventDetails{MobId: martialSquatter, RoomId: 5236})
		require.Equal(t, "squatter", w.p.current)

		w = newQuestWalker(q).at("squatter")
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("the first fight carries the dark aside", func(t *testing.T) {
		require.Contains(t, q.Steps[1].Hint, darkAside)
	})

	// A bandit's worn gear drops only sometimes, and the squatter is a woman
	// (9111-bandit_squatter.yaml).
	t.Run("the loot lines promise no gear, and the squatter is she", func(t *testing.T) {
		var lines []string
		lines = append(lines, q.Steps[2].Description)
		for _, trig := range q.Triggers {
			for _, a := range trig.Actions {
				lines = append(lines, a.SendText)
			}
		}
		all := squash(strings.Join(lines, " "))
		for _, promise := range []string{"stays on the corpse", "stays on the body", "worth having"} {
			require.NotContains(t, all, promise)
		}
		require.Contains(t, all, "type loot corpse")
		require.Contains(t, all, "her gear")

		garve := loadCouleeTopics(t, "9112.yaml")
		require.Contains(t, garve.FindTopic("wash_progress_scout").Text, "She holds the old cistern")
		text, _ := newNpcTalker(garve, playerOn(q, "scout", "30-end", "32-end")).opener()
		require.Contains(t, text, "She has dug into the old cistern")
	})

	t.Run("Vorn offers it and Garve no longer does", func(t *testing.T) {
		checkOfferMoved(t, "9108.yaml", "wash_offer", q, "32-end", "9112.yaml")
	})

	t.Run("Garve's talk step", func(t *testing.T) {
		checkArriveTopic(t, "garve", "9112.yaml", q, martialGarve, "wash_arrive", []string{"30-end", "32-end"})
	})

	t.Run("Garve's greeting names the step's one command", func(t *testing.T) {
		f := "33-hold_the_wash.yaml"
		pre := []string{"30-end", "32-end"}
		checkTopicSteps(t, "garve", "9112.yaml", []topicStep{
			{f, 33, "met", "type attack scout", pre, "wash_progress_start", false},
			{f, 33, "scout", "type attack squatter", pre, "wash_progress_scout", false},
			{f, 33, "squatter", "", pre, "wash_turnin", true},
		})
	})
}

func TestQuest34TakeTheTower_KillThenReport(t *testing.T) {
	q := loadShippedQuest(t, "34-take_the_tower.yaml", 34)
	require.Equal(t, []string{"start", "slain", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, q.Steps[0].Hint, darkAside)
	require.Contains(t, strings.ToLower(q.Steps[0].Hint), "type attack captain")
	turnIn := turnInRequires(t, "9112.yaml", "tower_in")
	require.Equal(t, []string{"34-slain"}, turnIn)

	// The Watch-Captain's Blade (10044) arrives only as the hand-in's reward, and
	// the captain carries a different sword, so no line may put it in the
	// player's hands before Garve gives it.
	t.Run("the blade comes from Garve, not from the kill", func(t *testing.T) {
		require.Equal(t, 10044, q.Rewards.ItemId)
		for _, trig := range q.Triggers {
			for _, a := range trig.Actions {
				require.NotContains(t, a.SendText, "blade", "the kill line must not hand over a blade")
			}
		}
		require.NotContains(t, q.Rewards.PlayerMessage, "from you", "the player never had the blade to hand back")
		require.NotContains(t, q.Rewards.PlayerMessage, "gives it back")
		require.NotContains(t, q.Rewards.RoomMessage, "back")
		require.Contains(t, q.Rewards.PlayerMessage, "blade", "the reward line still names what it pays")
	})

	// The kill is one action with no flag condition; the boon the player
	// chose still lands at the kill, through the quest_granted 34-slain
	// triggers.
	for boon, stat := range map[string]string{"might": "strength", "finesse": "dexterity"} {
		w := newQuestWalker(q).at("start")
		w.p.flags["34-boon"] = boon
		w.notify("mob_death", EventDetails{MobId: martialCaptain, RoomId: 5242})
		require.Equal(t, "slain", w.p.current, "the %s path: the captain's death moves the player onto slain", boon)
		require.Equal(t, []StatIncreaseCall{{Stat: stat, Amount: 3}}, w.ctx.increasedStats,
			"the %s path trains %s once, at the kill", boon, stat)
		require.False(t, holdsAll(w.p, []string{"34-end"}), "the kill no longer ends the quest")
	}

	pre := []string{"30-end", "32-end", "33-end"}
	might := func(p *linearPlayer) { p.flags["34-boon"] = "might" }
	checkTopicStepsWith(t, "garve", "9112.yaml", might, []topicStep{
		{"34-take_the_tower.yaml", 34, "start", "type attack captain", pre, "tower_progress", false},
		{"34-take_the_tower.yaml", 34, "slain", "", pre, "tower_in", true},
	})
}

func TestQuest53CullTheWash_EndsAtGarve(t *testing.T) {
	q := loadShippedQuest(t, "53-cull_the_wash.yaml", 53)
	require.Equal(t, []string{"start", "firstkill", "culled", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, q.Steps[0].Hint, darkAside)
	require.Contains(t, strings.ToLower(q.Steps[1].Hint), "squatter", "the second kill is the squatter")
	turnIn := turnInRequires(t, "9112.yaml", "cull_in")
	require.Equal(t, []string{"53-culled"}, turnIn)

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("mob_death", EventDetails{MobId: martialSquatter, RoomId: 5235})
		require.Equal(t, "start", w.p.current, "the squatter first must not count")
		w.notify("mob_death", EventDetails{MobId: martialScout, RoomId: 5232})
		require.Equal(t, "firstkill", w.p.current)
		w.notify("mob_death", EventDetails{MobId: martialScout, RoomId: 5233})
		require.Equal(t, "firstkill", w.p.current, "a second scout is not the squatter")
		w.notify("mob_death", EventDetails{MobId: martialSquatter, RoomId: 5235})
		require.Equal(t, "culled", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
		require.False(t, holdsAll(w.p, []string{"53-end"}), "the kill no longer ends the run")
	})

	t.Run("Garve's greeting names the step's one command", func(t *testing.T) {
		f := "53-cull_the_wash.yaml"
		pre := []string{"30-end", "32-end", "33-end", "34-end"}
		checkTopicSteps(t, "garve", "9112.yaml", []topicStep{
			{f, 53, "start", "type attack scout", pre, "cull_progress_start", false},
			{f, 53, "firstkill", "type attack squatter", pre, "cull_progress_firstkill", false},
			{f, 53, "culled", "", pre, "cull_in", true},
		})
	})
}
