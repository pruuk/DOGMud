package questengine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The Alchemy spoke of Pothole Coulee (quests 38, 39 and 55) under the owner
// ruling of 2026-10-17: one command per step, steps in order, each trigger
// gated on the step before it. These walk the shipped YAML with a player whose
// progress behaves like a real character's (see linearPlayer).

const (
	glassVial    = 40006
	healersRoot  = 40004
	firebombItem = 30058
	marshLeech   = 9130
	sheltered    = 5265 // Birna's pool, an alchemy bench
	dryingRacks  = 5267 // the second alchemy bench
	sunkenPath   = 5270
	blackPool    = 5273
)

func TestQuest38FirstBrew_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "38-first_brew.yaml", 38)
	steps := newLinearPlayer(q).steps
	for _, old := range []string{"start", "brew", "drink", "end"} {
		require.Contains(t, steps, old, "step %q must survive so a player mid-quest resumes", old)
	}
	turnIn := turnInRequires(t, "9128.yaml", "brew_turnin")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")

		w.notify("item_gain", EventDetails{ItemId: glassVial, RoomId: sheltered})
		require.Equal(t, "start", w.p.current, "a vial bought before listing must not skip the list step")

		w.notify("command", EventDetails{Command: "list", RoomId: sheltered})
		require.Equal(t, "list", w.p.current)

		w.notify("item_gain", EventDetails{ItemId: healersRoot, RoomId: sheltered})
		require.Equal(t, "list", w.p.current, "a root before the vial must not skip the vial step")

		w.notify("item_gain", EventDetails{ItemId: glassVial, RoomId: sheltered})
		require.Equal(t, "vial", w.p.current)

		w.notify("command", EventDetails{Command: "drink", RoomId: sheltered})
		require.Equal(t, "vial", w.p.current, "a drink before the brew must not count")

		w.notify("item_gain", EventDetails{ItemId: healersRoot, RoomId: sheltered})
		require.Equal(t, "roots", w.p.current)
		require.False(t, holdsAll(w.p, turnIn))

		w.notify("command", EventDetails{Command: "craft", RoomId: sheltered})
		require.Equal(t, "brew", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the drink")

		w.notify("command", EventDetails{Command: "drink", RoomId: sheltered})
		require.Equal(t, "drink", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the last step")
	})

	// Old saves: a player on start may already carry the vial and roots (the
	// old start step asked for all three purchases at once). Crafting needs
	// them in hand, so the brew grant is lenient on 38-start.
	t.Run("player saved on start with materials can craft", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "craft", RoomId: dryingRacks})
		require.Equal(t, "brew", w.p.current)
	})

	t.Run("player saved on start without materials lists first", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "list", RoomId: sheltered})
		require.Equal(t, "list", w.p.current)
	})

	t.Run("player saved on brew resumes with the drink", func(t *testing.T) {
		w := newQuestWalker(q).at("brew")
		w.notify("command", EventDetails{Command: "drink", RoomId: 5269})
		require.Equal(t, "drink", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("player saved on drink can turn in", func(t *testing.T) {
		w := newQuestWalker(q).at("drink")
		require.True(t, holdsAll(w.p, turnIn))
	})
}

func TestQuest39WadeTheMarsh_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "39-wade_the_marsh.yaml", 39)
	require.Equal(t, []string{"start", "leech", "throw", "forage", "end"}, newLinearPlayer(q).steps,
		"the step ids are what a saved character stores; keep them so a player mid-quest resumes")
	turnIn := turnInRequires(t, "9129.yaml", "marsh_turnin")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")

		w.notify("command", EventDetails{Command: "forage", RoomId: sunkenPath})
		require.Equal(t, "start", w.p.current, "a forage before the leech must not skip ahead")
		w.notify("command", EventDetails{Command: "throw", RoomId: 5269})
		require.Equal(t, "start", w.p.current, "a throw before the leech is down must not skip the leech step")

		w.notify("mob_death", EventDetails{MobId: marshLeech, RoomId: 5269})
		require.Equal(t, "leech", w.p.current)

		w.notify("command", EventDetails{Command: "forage", RoomId: blackPool})
		require.Equal(t, "leech", w.p.current, "a forage before the throw must not skip the throw step")

		w.notify("command", EventDetails{Command: "throw", RoomId: 5272})
		require.Equal(t, "throw", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the forage")

		w.notify("command", EventDetails{Command: "forage", RoomId: sunkenPath})
		require.Equal(t, "forage", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the last step")
	})

	t.Run("player saved on leech resumes with the throw", func(t *testing.T) {
		w := newQuestWalker(q).at("leech")
		w.notify("command", EventDetails{Command: "throw", RoomId: blackPool})
		require.Equal(t, "throw", w.p.current)
	})

	t.Run("player saved on throw resumes with the forage", func(t *testing.T) {
		w := newQuestWalker(q).at("throw")
		w.notify("command", EventDetails{Command: "forage", RoomId: blackPool})
		require.Equal(t, "forage", w.p.current)
	})

	t.Run("player saved on forage can turn in", func(t *testing.T) {
		w := newQuestWalker(q).at("forage")
		require.True(t, holdsAll(w.p, turnIn))
	})
}

func TestQuest55AnotherBrew_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "55-another_brew.yaml", 55)
	require.Equal(t, []string{"start", "brew", "end"}, newLinearPlayer(q).steps)

	w := newQuestWalker(q).at("start")
	w.notify("command", EventDetails{Command: "drink", RoomId: sheltered})
	require.Equal(t, "start", w.p.current, "a drink before the brew must not finish the quest")

	// Both benches in the coulee are alchemy benches.
	w.notify("command", EventDetails{Command: "craft", RoomId: dryingRacks})
	require.Equal(t, "brew", w.p.current, "a craft at the Drying Racks bench counts")

	w.notify("command", EventDetails{Command: "drink", RoomId: 5271})
	require.Equal(t, "end", w.p.current)
}

// Falv hands out a replacement firebomb while quest 39 is open. A player who
// still carries one must not get a second (S8): the handout topic carries
// itemExcluded, and its reminder (carriesItem, same gates) hands nothing out.
// Every firebomb handout on Falv is checked, so a new one cannot skip the
// rule.
func TestFalvFirebombHandoutGivesOnlyOne(t *testing.T) {
	falv := loadCouleeTopics(t, "9129.yaml")
	found := 0
	for i := range falv.Topics {
		tp := &falv.Topics[i]
		if tp.GivesItem != firebombItem || tp.GrantsQuest != "" {
			continue
		}
		found++
		require.Equal(t, firebombItem, tp.ItemExcluded, "topic %s hands a firebomb to a player who already carries one", tp.Key)
	}
	require.Equal(t, 1, found, "expected exactly one firebomb handout topic; the check below names it")

	q := loadShippedQuest(t, "39-wade_the_marsh.yaml", 39)
	n := newNpcTalker(falv, playerOn(q, "start", "38-end"))
	checkHandout(t, n, "firebomb_recovery", "firebomb_already_have", firebombItem)
	require.Contains(t, squash(falv.FindTopic("firebomb_already_have").Text), "already have one")
}

// S5: Another Brew finishes on the drink, so nothing may send the player
// back to Falv to report it.
func TestQuest55AnotherBrew_NoReportBack(t *testing.T) {
	q := loadShippedQuest(t, "55-another_brew.yaml", 55)
	lines := []string{q.Description, q.Rewards.PlayerMessage}
	for _, s := range q.Steps {
		lines = append(lines, s.Hint, s.Description)
	}
	for _, tr := range q.Triggers {
		for _, a := range tr.Actions {
			lines = append(lines, a.SendText)
		}
	}
	for _, l := range lines {
		l = squash(l)
		require.NotContains(t, l, "come back", "quest 55 ends on the drink: %q", l)
		require.NotContains(t, l, "report back", "quest 55 ends on the drink: %q", l)
		require.NotContains(t, l, "tell him", "quest 55 ends on the drink: %q", l)
	}
}
