package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// mendWorld is "mend the scout": Garve offers quest 60, any heal that lands
// on mob 9113 moves it on, Garve takes the hand-in.
func mendWorld() questloop.World {
	q := &quests.Quest{
		QuestId: 60, Name: "Mend the Scout",
		Steps: []quests.QuestStep{{Id: "start", Hint: "h"}, {Id: "mended", Hint: "h"}, {Id: "end"}},
		Triggers: []quests.TriggerDef{{
			Event: "spell_landed", Effect: "heal", Mob: 9113,
			Conditions: quests.Conditions{Has: []string{"60-start"}, Missing: []string{"60-mended"}},
			Actions:    []quests.ActionDef{{Grant: "60-mended"}},
		}},
	}
	df := &dialogue.DialogueFile{MobId: 9112, Topics: []dialogue.Topic{
		{Key: "mend_offer", Kind: dialogue.KindOffer, Quest: 60, Label: "Mend", Text: "t",
			Effects: dialogue.Effects{GrantsQuest: "60-start"}},
		{Key: "mend_in", Kind: dialogue.KindHandIn, Quest: 60, Label: "Mend", Text: "t",
			Gates: dialogue.Gates{QuestRequired: []string{"60-mended"}}, Effects: dialogue.Effects{GrantsQuest: "60-end"}},
	}}
	return questloop.World{Quests: []*quests.Quest{q}, Dialogue: []*dialogue.DialogueFile{df}}
}

func spellSrc() questloop.Sources {
	effects := map[string]string{"heal": "heal", "conviction-spike": "damage"}
	return questloop.Sources{
		SpellEffect:   func(id string) string { return effects[id] },
		StarterSpells: []string{"conviction-spike"},
	}
}

func noSpell(fs []questloop.Finding) []questloop.Finding {
	var out []questloop.Finding
	for _, f := range fs {
		if f.Rule == "walk-no-spell" {
			out = append(out, f)
		}
	}
	return out
}

func TestWalkSpell_NoHeldSpellCanLand(t *testing.T) {
	fs := Walk(mendWorld(), spellSrc())
	ns := noSpell(fs)
	require.Len(t, ns, 1, "%v", fs)
	require.Equal(t, 60, ns[0].QuestId)
	require.Equal(t, "quest 60 trigger spell_landed mob=9113 effect=heal grants=60-mended", ns[0].Where)
	require.Contains(t, ns[0].Msg, "60-mended")
	got := keys(fs)
	require.Contains(t, got, "walk-dead-end 60 step mended", "the quest must not complete")
	require.NotContains(t, got, "walk-dead-end 60 quest 60 trigger spell_landed mob=9113 effect=heal grants=60-mended",
		"the Way is reported once, as walk-no-spell")
}

func TestWalkSpell_TeachSpellOnThePathHoldsIt(t *testing.T) {
	w := mendWorld()
	w.Quests[0].Triggers = append(w.Quests[0].Triggers, quests.TriggerDef{
		Event: "quest_granted", QuestToken: "60-start",
		Actions: []quests.ActionDef{{TeachSpell: "heal"}},
	})
	require.Empty(t, Walk(w, spellSrc()))

	// The same teacher with a spell of another effect does not help.
	w.Quests[0].Triggers[1].Actions[0].TeachSpell = "conviction-spike"
	require.Len(t, noSpell(Walk(w, spellSrc())), 1)
}

// One branch learns the spell and the other does not: the Way works from
// the first, so it is no finding.
func TestWalkSpell_OneBranchHoldingTheSpellIsEnough(t *testing.T) {
	w := mendWorld()
	q := w.Quests[0]
	q.Steps = []quests.QuestStep{{Id: "start", Hint: "h"}, {Id: "taught", Hint: "h"}, {Id: "mended", Hint: "h"}, {Id: "end"}}
	q.Triggers[0].Conditions = quests.Conditions{Has: []string{"60-taught"}, Missing: []string{"60-mended"}}
	// Both branches end in room 5, so only the spell tells the states apart.
	for i, a := range [][]quests.ActionDef{{{Grant: "60-taught"}}, {{Grant: "60-taught"}, {TeachSpell: "heal"}}} {
		q.Triggers = append(q.Triggers, quests.TriggerDef{
			Event: []string{"room_enter", "room_interact"}[i], Room: 5,
			Conditions: quests.Conditions{Has: []string{"60-start"}, Missing: []string{"60-taught"}},
			Actions:    a,
		})
	}
	require.Empty(t, Walk(w, spellSrc()))

	// Neither branch teaches it: walk-no-spell.
	q.Triggers[2].Actions = q.Triggers[2].Actions[:1]
	require.Len(t, noSpell(Walk(w, spellSrc())), 1)
}

func TestWalkSpell_PrerequisiteRewardSpellHoldsIt(t *testing.T) {
	w := mendWorld()
	w.Quests = append(w.Quests, &quests.Quest{QuestId: 59,
		Steps:   []quests.QuestStep{{Id: "start"}, {Id: "end"}},
		Rewards: quests.QuestReward{SpellId: "heal"}})
	w.Dialogue[0].Topics[0].QuestRequired = []string{"59-end"}
	require.Empty(t, noSpell(Walk(w, spellSrc())))

	// Only quest 59's final step holds the reward: needing 59-start is not
	// enough.
	w.Dialogue[0].Topics[0].QuestRequired = []string{"59-start"}
	require.Len(t, noSpell(Walk(w, spellSrc())), 1)
}

func TestWalkSpell_NilSpellEffectSkipsTheCheck(t *testing.T) {
	src := spellSrc()
	src.SpellEffect = nil
	require.Empty(t, Walk(mendWorld(), src))
}

// A spell taught on a later step does not let an earlier state jump to the
// step that needs it, and the skip probe's refusal is no finding.
func TestWalkSpell_SpellTaughtLaterIsNoSkipAndNoFinding(t *testing.T) {
	w := mendWorld()
	q := w.Quests[0]
	q.Steps = []quests.QuestStep{{Id: "start", Hint: "h"}, {Id: "taught", Hint: "h"}, {Id: "mended", Hint: "h"}, {Id: "end"}}
	q.Triggers[0].Conditions = quests.Conditions{Missing: []string{"60-mended"}}
	q.Triggers = append(q.Triggers, quests.TriggerDef{
		Event: "room_enter", Room: 5,
		Conditions: quests.Conditions{Has: []string{"60-start"}, Missing: []string{"60-taught"}},
		Actions:    []quests.ActionDef{{Grant: "60-taught"}, {TeachSpell: "heal"}},
	})
	require.Empty(t, Walk(w, spellSrc()))
}
