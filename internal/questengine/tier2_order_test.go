package questengine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func runKills(t *testing.T, questId string, start string, mobs ...int) (*fullMockPlayer, *fullMockActionContext) {
	t.Helper()
	e := NewEngine()
	e.RegisterQuest(loadTier2Quest(t, questId))
	p := newFullMockPlayer(100)
	p.quests[start] = true
	ctx := newFullMockActionContext(1, p)
	for _, m := range mobs {
		e.Notify("mob_death", EventDetails{UserId: 1, MobId: m, RoomId: 100}, p, ctx)
	}
	return p, ctx
}

// Tier 2 ruling: the hedgerow bandit first, then the Reach-Touched.
func TestQuest61InOrder(t *testing.T) {
	p, _ := runKills(t, "61", "61-start", 9173, 9178)
	assert.True(t, p.HasQuest("61-bandit"))
	assert.True(t, p.HasQuest("61-reach"))
}

func TestQuest61ReachFirstGrantsNothingAndSaysSo(t *testing.T) {
	p, ctx := runKills(t, "61", "61-start", 9178)
	assert.False(t, p.HasQuest("61-bandit"))
	assert.False(t, p.HasQuest("61-reach"))
	assert.Len(t, ctx.sentTexts, 1)
	assert.Contains(t, ctx.sentTexts[0], "Pemberton wanted the bandit")

	// The bandit, then the Reach-Touched again, still finishes.
	e := NewEngine()
	e.RegisterQuest(loadTier2Quest(t, "61"))
	e.Notify("mob_death", EventDetails{UserId: 1, MobId: 9173, RoomId: 100}, p, ctx)
	e.Notify("mob_death", EventDetails{UserId: 1, MobId: 9178, RoomId: 100}, p, ctx)
	assert.True(t, p.HasQuest("61-reach"))
}

// Quest 19: a hunter killed before a swarm grants nothing, and says so.
func TestQuest19HunterAfterShrimp(t *testing.T) {
	p, ctx := runKills(t, "19", "19-start", 331)
	assert.False(t, p.HasQuest("19-hunter"))
	if assert.Len(t, ctx.sentTexts, 1) {
		assert.Contains(t, ctx.sentTexts[0], "starts with the shrimp swarms")
	}
	p, _ = runKills(t, "19", "19-start", 330, 331)
	assert.True(t, p.HasQuest("19-hunter"))
}

// The signing needs only the hunter, which holds the shrimp by HasQuest.
func TestQuest19SignBountyNeedsTheHunter(t *testing.T) {
	q := loadTier2Quest(t, "19")
	for _, tr := range q.Triggers {
		if tr.Event == "topic_heard" && tr.Topic == "sign_bounty" {
			assert.Equal(t, []string{"19-hunter"}, tr.Conditions.Has)
			return
		}
	}
	t.Fatal("no sign_bounty trigger")
}

// Quest 61's first step points at the bandit's lair, the Mire Cutoff.
func TestQuest61StartPointsAtTheBandit(t *testing.T) {
	q := loadTier2Quest(t, "61")
	assert.Equal(t, "start", q.Steps[0].Id)
	assert.Equal(t, []int{9173}, q.Steps[0].Destination.Mobs)
	assert.Equal(t, 5376, q.Steps[0].Destination.Room)
}

// The reach-first flag and Pemberton's flagged opener are gone from the data.
func TestQuest61HasNoReachFirstFlag(t *testing.T) {
	root := "../../_datafiles/world/dogmud"
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return err
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		assert.NotContains(t, string(data), "reach-first", path)
		return nil
	})
	assert.NoError(t, err)
	data, err := os.ReadFile(root + "/dialogue/north_road_north/9171.yaml")
	assert.NoError(t, err)
	assert.NotContains(t, string(data), "questFlagRequired")
}

// Quest 20: a find makes its hidden noun known, so `look <noun>` works after.
func TestQuest20SearchDiscoversItsHiddenNoun(t *testing.T) {
	q := loadTier2Quest(t, "20")
	want := map[int]string{4124: "buried marker", 4130: "cache", 4140: "disturbance"}
	got := map[int]string{}
	for _, tr := range q.Triggers {
		for _, a := range tr.Actions {
			if a.Discover != "" {
				got[tr.Room] = a.Discover
			}
		}
	}
	assert.Equal(t, want, got)
}

// Quest 18's only way to the 18-barn flag is a search, so the room says so.
func TestQuest18BedrollTextTellsThePlayerToSearch(t *testing.T) {
	data, err := os.ReadFile("../../_datafiles/world/dogmud/rooms/north_road/4050.yaml")
	assert.NoError(t, err)
	assert.Contains(t, strings.Join(strings.Fields(string(data)), " "), "Not long ago. Type search.")
}
