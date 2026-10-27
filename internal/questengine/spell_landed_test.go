package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/assert"
)

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
