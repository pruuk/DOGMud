package questengine

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

// nounEngine registers one quest per (id, noun) pair, each with a single
// look_at trigger on room 100 that grants "<id>-done".
func nounEngine(nouns map[int]string) *Engine {
	e := NewEngine()
	for id, noun := range nouns {
		tok := questTokenFor(id)
		e.RegisterQuest(&QuestDef{
			QuestId: id,
			Name:    "noun number test",
			Steps:   []QuestStep{{Id: tok, Description: "done"}},
			Triggers: []TriggerDef{{
				Event:   "look_at",
				Room:    100,
				Noun:    noun,
				Actions: []ActionDef{{Grant: tok}},
			}},
		})
	}
	return e
}

func questTokenFor(id int) string {
	return string(rune('a'+id)) + "-done"
}

func notifyLook(e *Engine, player *fullMockPlayer, typed string) NotifyResult {
	ctx := newFullMockActionContext(1, player)
	return e.Notify("look_at", EventDetails{
		UserId: 1, RoomId: 100, Noun: typed,
	}, player, ctx)
}

func TestTriggerNoun_PluralTypedFiresSingularTrigger(t *testing.T) {
	e := nounEngine(map[int]string{1: "slab"})
	p := newFullMockPlayer(100)
	assert.True(t, notifyLook(e, p, "slabs").Handled, "look slabs should fire a noun: slab trigger")
	assert.True(t, p.HasQuest(questTokenFor(1)))
}

func TestTriggerNoun_SingularTypedFiresPluralTrigger(t *testing.T) {
	e := nounEngine(map[int]string{2: "wardstones"})
	p := newFullMockPlayer(100)
	assert.True(t, notifyLook(e, p, "wardstone").Handled, "look wardstone should fire a noun: wardstones trigger")
	assert.True(t, p.HasQuest(questTokenFor(2)))
}

func TestTriggerNoun_UnrelatedNounStillDoesNotFire(t *testing.T) {
	e := nounEngine(map[int]string{1: "slab", 3: "moss"})
	p := newFullMockPlayer(100)
	assert.False(t, notifyLook(e, p, "mos").Handled, "mos must not reach moss")
	assert.False(t, notifyLook(e, p, "slate").Handled)
}

func TestTriggerNoun_ExactPreferredOverNumberVariant(t *testing.T) {
	// Two quests, one per spelling: the exact spelling fires and its
	// number variant does not.
	e := nounEngine(map[int]string{4: "pillar", 5: "pillars"})

	p := newFullMockPlayer(100)
	assert.True(t, notifyLook(e, p, "pillar").Handled)
	assert.True(t, p.HasQuest(questTokenFor(4)))
	assert.False(t, p.HasQuest(questTokenFor(5)), "pillars trigger must not fire on exact 'pillar'")

	p2 := newFullMockPlayer(100)
	assert.True(t, notifyLook(e, p2, "pillars").Handled)
	assert.True(t, p2.HasQuest(questTokenFor(5)))
	assert.False(t, p2.HasQuest(questTokenFor(4)), "pillar trigger must not fire on exact 'pillars'")
}

// TestTriggerNoun_Quest20PillarsOneTrigger pins the merged pair in quest 20
// (Ulla's Silence): the pillars once had one trigger for `pillars` and one for
// `pillar`; now `look pillar` and `look pillars` each match exactly one. The
// trigger sits on the Temple Approach (4122), the room whose `pillars` noun
// describes the carved spiral; the sanctuary (4123) has no pillars and
// matches none (#375 batch 3 zone pass).
func TestTriggerNoun_Quest20PillarsOneTrigger(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "_datafiles", "world", "dogmud", "quests", "20-ullas_silence.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read quest 20: %v", err)
	}
	var q QuestDef
	if err := yaml.Unmarshal(raw, &q); err != nil {
		t.Fatalf("parse quest 20: %v", err)
	}
	// Tier 2: the pillars are found by `search`, not a typed noun, so the
	// Temple Approach carries exactly one search trigger and no
	// look_at or search_at trigger remains.
	count := func(roomId int) int {
		d := EventDetails{UserId: 1, RoomId: roomId, Command: "search"}
		matches := 0
		for i := range q.Triggers {
			tr := &q.Triggers[i]
			assert.NotContains(t, []string{"look_at", "search_at"}, tr.Event, "quest 20 has no typed-noun trigger")
			if tr.Event == "command" && matchTriggerFields(tr, d, nounNumber) {
				matches++
			}
		}
		return matches
	}
	assert.Equal(t, 1, count(4122), "search in room 4122 should match exactly one quest 20 trigger")
	assert.Equal(t, 0, count(4123), "search in room 4123 (no pillars there) should match no quest 20 trigger")
}

func TestTriggerNoun_VariantWorksWhenNoExactTriggerExists(t *testing.T) {
	e := nounEngine(map[int]string{4: "pillar", 6: "column"})
	p := newFullMockPlayer(100)
	assert.True(t, notifyLook(e, p, "pillars").Handled)
	assert.True(t, p.HasQuest(questTokenFor(4)))
	assert.False(t, p.HasQuest(questTokenFor(6)))
}
