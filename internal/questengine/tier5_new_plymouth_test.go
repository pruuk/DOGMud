package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

func tier5Search(room int) EventDetails { return EventDetails{Command: "search", RoomId: room} }

// Rule 14: a step completes on something the player does, never a typed noun.
func TestTier5QuestsHaveNoTypedNouns(t *testing.T) {
	for file, id := range map[string]int{
		"63-dock_rat.yaml":                   63,
		"65-the_street_sweepers_secret.yaml": 65,
		"66-the_addicts_plight.yaml":         66,
		"67-the_bloom_trail.yaml":            67,
		"68-the_cooperage_circle.yaml":       68,
		"69-the_gallery_cipher.yaml":         69,
		"70-the_pre_founding_web.yaml":       70,
	} {
		for _, tr := range loadShippedQuest(t, file, id).Triggers {
			require.NotEqual(t, "room_interact", tr.Event, "quest %d", id)
		}
	}
}

func TestQuest63TheBookIsFoundBySearch(t *testing.T) {
	const office, book = 5526, 40100
	q := loadShippedQuest(t, "63-dock_rat.yaml", 63)
	require.Equal(t, []string{"start", "evidence", "end"}, newLinearPlayer(q).steps)
	require.NoError(t, q.Validate())

	w := newQuestWalker(q).at("start")
	w.notify("command", tier5Search(office+1))
	require.Equal(t, "start", w.p.current)
	w.notify("command", tier5Search(office))
	require.Equal(t, "evidence", w.p.current)
	require.Equal(t, []int{book}, w.ctx.givenItems, "the search gives one book")

	lost := newQuestWalker(q).at("evidence")
	lost.notify("command", tier5Search(office))
	require.Equal(t, []int{book}, lost.ctx.givenItems, "a lost book has a spare")

	held := newQuestWalker(q).at("evidence")
	held.p.items[book] = true
	held.notify("command", tier5Search(office))
	require.Empty(t, held.ctx.givenItems, "no second book while one is held")
}

// C3: a save on a removed step must keep going, so each leaves by rename.
func TestTier5RemovedStepsAreRenamed(t *testing.T) {
	m, err := quests.ParseStepManifest(repoFile(t, "_datafiles", "world", "dogmud", "quests", "step-ids.txt"))
	require.NoError(t, err)
	require.Equal(t, "63-evidence", m.Renamed["63-report"])
	require.Equal(t, "65-discover", m.Renamed["65-report"])
	require.Equal(t, "66-dose", m.Renamed["66-escort"])
	require.Equal(t, "66-dose", m.Renamed["66-detox"])
	require.Equal(t, "67-witness", m.Renamed["67-report"])

	// The migration a character load runs (users.LoadUser), on a save resting on
	// each old step, active and shelved. It lands on a step the quest still has.
	for _, c := range []struct {
		file          string
		qid           int
		oldStep, step string
	}{
		{"63-dock_rat.yaml", 63, "report", "evidence"},
		{"65-the_street_sweepers_secret.yaml", 65, "report", "discover"},
		{"66-the_addicts_plight.yaml", 66, "escort", "dose"},
		{"66-the_addicts_plight.yaml", 66, "detox", "dose"},
		{"67-the_bloom_trail.yaml", 67, "report", "witness"},
	} {
		save := &characters.Character{
			QuestProgress: map[int]string{c.qid: c.oldStep},
			ShelvedQuests: map[int]string{c.qid: c.oldStep},
		}
		save.RenameQuestSteps(m.Renamed)
		require.Equal(t, c.step, save.QuestProgress[c.qid], "%d-%s", c.qid, c.oldStep)
		require.Equal(t, c.step, save.ShelvedQuests[c.qid], "%d-%s on the shelf", c.qid, c.oldStep)
		require.Contains(t, newLinearPlayer(loadShippedQuest(t, c.file, c.qid)).steps, c.step)
	}
}

// Quest 66 is deliver, return, heal (owner ruling 2): Ysolde takes the wrapper
// and gives her purge, and Teels takes the purge.
func TestQuest66DeliverReturnHeal(t *testing.T) {
	const ysolde = 9323
	q := loadShippedQuest(t, "66-the_addicts_plight.yaml", 66)
	require.Equal(t, []string{"start", "dose", "end"}, newLinearPlayer(q).steps)
	require.NoError(t, q.Validate())
	require.Empty(t, q.Flags, "66-addict-fate went with escort_close")

	w := newQuestWalker(q).at("start")
	w.notify("topic_heard", EventDetails{MobId: ysolde, Topic: "bloom"})
	require.Equal(t, "start", w.p.current)
	w.notify("topic_heard", EventDetails{MobId: ysolde, Topic: "wrapperhandoff"})
	require.Equal(t, "dose", w.p.current)

	y := loadShippedDialogue(t, "new_plymouth_common", "9323.yaml")
	in := y.FindTopic("wrapperhandoff")
	require.Equal(t, 40110, in.RequiresItem)
	require.Equal(t, 40109, in.GivesItem)
	require.Empty(t, in.GrantsQuest, "a mid step is a topic_heard trigger")
	require.Nil(t, y.FindTopic("escort_close"))
	require.NotNil(t, y.FindTopic("detox"), "her info topic stays, and replaces a lost purge")

	teels := loadShippedDialogue(t, "new_plymouth_docks", "9392.yaml")
	out := teels.FindTopic("purge_handoff")
	require.Equal(t, 40109, out.RequiresItem)
	require.Equal(t, []string{"66-dose"}, out.QuestRequired)
	require.Equal(t, "66-end", out.GrantsQuest)
	require.Equal(t, []string{"66-end"}, teels.FindTopic("q67_grant").QuestRequired)
	require.Nil(t, y.FindTopic("q67_grant"), "C1: the offer moved to Teels")
}

func TestQuest67HasNoReportStep(t *testing.T) {
	q := loadShippedQuest(t, "67-the_bloom_trail.yaml", 67)
	require.Equal(t, []string{"start", "front", "lintel", "delivery", "source", "witness", "end"}, newLinearPlayer(q).steps)
	require.NoError(t, q.Validate())
	require.Contains(t, q.Steps[3].Hint, "bring a light", "the Old Quarter is dark")
}

func TestQuest65TheSeamIsFoundBySearch(t *testing.T) {
	const foundation, lump = 5624, 40101
	q := loadShippedQuest(t, "65-the_street_sweepers_secret.yaml", 65)
	require.Equal(t, []string{"start", "discover", "end"}, newLinearPlayer(q).steps)
	require.NoError(t, q.Validate())
	w := newQuestWalker(q).at("start")
	w.notify("command", tier5Search(foundation))
	require.Equal(t, "discover", w.p.current)
	require.Equal(t, []int{lump}, w.ctx.givenItems)
	lost := newQuestWalker(q).at("discover")
	lost.notify("command", tier5Search(foundation))
	require.Equal(t, []int{lump}, lost.ctx.givenItems)
}

// A search on the Cooperage Floor finds the survey at any hour: Toby may be
// asleep on the cot beside it. Old saves need no catch-up: a save holding the
// survey at 68-evidence takes a harmless spare, and one at 68-choice without it
// takes the recovery.
func TestQuest68TheSurveyIsFoundBySearch(t *testing.T) {
	const floor, survey = 5720, 40113
	q := loadShippedQuest(t, "68-the_cooperage_circle.yaml", 68)
	require.NoError(t, q.Validate())
	w := newQuestWalker(q).at("start")
	w.notify("command", tier5Search(floor))
	require.Equal(t, "start", w.p.current, "Toby's word comes first")
	require.Empty(t, w.ctx.givenItems)
	w = newQuestWalker(q).at("evidence")
	w.notify("command", tier5Search(floor))
	require.Equal(t, "survey", w.p.current)
	require.Equal(t, []int{survey}, w.ctx.givenItems)
	old := newQuestWalker(q).at("choice")
	old.notify("command", tier5Search(floor))
	require.Equal(t, "choice", old.p.current)
	require.Equal(t, []int{survey}, old.ctx.givenItems)
	done := newQuestWalker(q).at("end")
	done.notify("command", tier5Search(floor))
	require.Empty(t, done.ctx.givenItems, "no survey after the choice")

	// The save the removed catch-up triggers were for: it took the survey before
	// the survey step existed, and one search still moves it on.
	early := newQuestWalker(q).at("evidence")
	early.p.items[survey] = true
	early.notify("command", tier5Search(floor))
	require.Equal(t, "survey", early.p.current)

	held := newQuestWalker(q).at("choice")
	held.p.items[survey] = true
	held.notify("command", tier5Search(floor))
	require.Empty(t, held.ctx.givenItems, "no spare once the step is past and one is held")
}

// Hints are at most 200 characters and name no route: the quest log's guide line
// gives the way.
func TestQuest68StepHintsAreShortAndGiveNoRoute(t *testing.T) {
	q := loadShippedQuest(t, "68-the_cooperage_circle.yaml", 68)
	for _, s := range q.Steps {
		require.LessOrEqual(t, len(s.Hint), 200, s.Id)
		require.NotContains(t, s.Hint, "twice", s.Id)
		require.NotContains(t, s.Hint, "three times", s.Id)
		require.NotContains(t, s.Hint, "by day", s.Id)
		require.NotContains(t, s.Hint, "look floorboard", s.Id)
	}
}

// A search of the Sanctuary finds the rubbing; a player who lost it takes another
// while the hand-in is open; Dross keeps the rubbing when he takes the reading.
func TestQuest69DrossKeepsTheRubbing(t *testing.T) {
	const sanctuary, rubbing = 5901, 40115
	q := loadShippedQuest(t, "69-the_gallery_cipher.yaml", 69)
	require.NoError(t, q.Validate())
	w := newQuestWalker(q).at("start")
	w.notify("command", tier5Search(sanctuary))
	require.Equal(t, "rubbing", w.p.current)
	require.Equal(t, []int{rubbing}, w.ctx.givenItems, "one search, one rubbing")
	lost := newQuestWalker(q).at("gallery")
	lost.notify("command", tier5Search(sanctuary))
	require.Equal(t, []int{rubbing}, lost.ctx.givenItems)
	lostLate := newQuestWalker(q).at("reading")
	lostLate.notify("command", tier5Search(sanctuary))
	require.Equal(t, []int{rubbing}, lostLate.ctx.givenItems, "lost between Lysha and Dross")

	// Holding the rubbing, a search gives no second one.
	held := newQuestWalker(q).at("reading")
	held.p.items[rubbing] = true
	held.notify("command", tier5Search(sanctuary))
	require.Empty(t, held.ctx.givenItems)

	// After the end, a search gives nothing.
	done := newQuestWalker(q).at("end")
	done.notify("command", tier5Search(sanctuary))
	require.Empty(t, done.ctx.givenItems, "no rubbing after the end")

	end := newQuestWalker(q).at("end")
	end.p.items[rubbing] = true
	end.notify("quest_granted", EventDetails{QuestToken: "69-end"})
	require.Equal(t, []int{rubbing}, end.ctx.consumedItems, "the rubbing leaves exactly once")

	// A rubbing the player does not hold in a backpack is not consumed, and
	// another token's grant never takes it.
	none := newQuestWalker(q).at("end")
	none.notify("quest_granted", EventDetails{QuestToken: "69-end"})
	require.Empty(t, none.ctx.consumedItems)
	other := newQuestWalker(q).at("reading")
	other.p.items[rubbing] = true
	other.notify("quest_granted", EventDetails{QuestToken: "69-reading"})
	require.Empty(t, other.ctx.consumedItems)
}

// Hints are at most 200 characters and name no route.
func TestQuest69StepHintsAreShortAndGiveNoRoute(t *testing.T) {
	q := loadShippedQuest(t, "69-the_gallery_cipher.yaml", 69)
	for _, s := range q.Steps {
		require.LessOrEqual(t, len(s.Hint), 200, s.Id)
		for _, banned := range []string{"west six times", "four times", "look reliefs", "morning", "evening"} {
			require.NotContains(t, s.Hint, banned, s.Id)
		}
	}
}

// The lintel is dark, so the step says to bring a light. One search gives one
// rubbing and moves the step; a player who lost it searches again; a player
// still holding it gets no spare; after the end the lintel gives nothing.
func TestQuest70TheLintelIsFoundBySearch(t *testing.T) {
	const lintel, rubbing = 6037, 40118
	q := loadShippedQuest(t, "70-the_pre_founding_web.yaml", 70)
	require.NoError(t, q.Validate())
	require.Contains(t, q.Steps[0].Hint, "bring a light")
	w := newQuestWalker(q).at("start")
	w.notify("command", tier5Search(lintel+1))
	require.Equal(t, "start", w.p.current)
	w.notify("command", tier5Search(lintel))
	require.Equal(t, "witness", w.p.current)
	require.Equal(t, []int{rubbing}, w.ctx.givenItems, "one search, one rubbing")
	lost := newQuestWalker(q).at("witness")
	lost.notify("command", tier5Search(lintel))
	require.Equal(t, []int{rubbing}, lost.ctx.givenItems)
	held := newQuestWalker(q).at("witness")
	held.p.items[rubbing] = true
	held.notify("command", tier5Search(lintel))
	require.Empty(t, held.ctx.givenItems)
	done := newQuestWalker(q).at("end")
	done.notify("command", tier5Search(lintel))
	require.Empty(t, done.ctx.givenItems, "no rubbing after the end")
}

// The quest stands alone (owner ruling): nothing a first timer reads leans on 68
// or 69, the hints fit and give no route, and the reward no longer admits the
// player to anything, since Orin may have just shown them the door.
func TestQuest70ReadsForAFirstTimerAndForTheColdEnding(t *testing.T) {
	q := loadShippedQuest(t, "70-the_pre_founding_web.yaml", 70)
	for _, s := range q.Steps {
		require.LessOrEqual(t, len(s.Hint), 200, s.Id)
		for _, banned := range []string{"Type look", "afternoon", "north eight", "up the stair"} {
			require.NotContains(t, s.Hint, banned, s.Id)
		}
		for _, banned := range []string{"painting", "cipher", "panel"} {
			require.NotContains(t, s.Description, banned, s.Id)
		}
	}
	require.Contains(t, q.Steps[0].Hint, "north of the stonework", "6038 is north of 6036, not of the lintel")
	for _, reward := range []string{q.Rewards.PlayerMessage, q.Rewards.RoomMessage} {
		for _, banned := range []string{"part of the quiet", "nod", "painting", "cipher"} {
			require.NotContains(t, reward, banned)
		}
	}
}
