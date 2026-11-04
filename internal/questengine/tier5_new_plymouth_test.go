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
