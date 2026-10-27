package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/assert"
)

func TestSpellLandedDetails(t *testing.T) {
	d := SpellLandedDetails(1, 5305, "heal", "heal", TargetActor(1, 1, 0))
	assert.Equal(t, EventDetails{UserId: 1, RoomId: 5305, Noun: "heal", Effect: "heal", TargetKind: quests.TargetSelf}, d)

	d = SpellLandedDetails(1, 5305, "identify", "identify", Target{})
	assert.Equal(t, quests.TargetNone, d.TargetKind)
	assert.Equal(t, "identify", d.Noun)
	assert.Empty(t, d.Command, "spell_landed is not a command event")
}

// seedMendQuest: one step, any heal that lands on mob 9112.
func seedMendQuest() *Engine {
	e := NewEngine()
	e.RegisterQuest(&QuestDef{
		QuestId: 310, Name: "Mend the Scout",
		Steps: []QuestStep{{Id: "310-start"}, {Id: "310-mended"}},
		Triggers: []TriggerDef{{
			Event: "spell_landed", Effect: "heal", Mob: 9112,
			Conditions: Conditions{Has: []string{"310-start"}, Missing: []string{"310-mended"}},
			Actions:    []ActionDef{{Grant: "310-mended"}},
		}},
	})
	return e
}

func TestSpellLandedNeedsTheEffectAndTheMob(t *testing.T) {
	e := seedMendQuest()
	p := newFullMockPlayer(100)
	p.quests["310-start"] = true

	// A damage spell on the scout does not mend it.
	e.Notify("spell_landed", EventDetails{UserId: 1, Noun: "conviction-spike", Effect: "damage", TargetKind: quests.TargetMob, MobId: 9112}, p, newFullMockActionContext(1, p))
	assert.False(t, p.HasQuest("310-mended"))
	// A heal on someone else does not either.
	e.Notify("spell_landed", EventDetails{UserId: 1, Noun: "heal", Effect: "heal", TargetKind: quests.TargetMob, MobId: 7}, p, newFullMockActionContext(1, p))
	assert.False(t, p.HasQuest("310-mended"))
	// Any heal on the scout does.
	e.Notify("spell_landed", EventDetails{UserId: 1, Noun: "mend-wounds", Effect: "heal", TargetKind: quests.TargetMob, MobId: 9112}, p, newFullMockActionContext(1, p))
	assert.True(t, p.HasQuest("310-mended"))
}
