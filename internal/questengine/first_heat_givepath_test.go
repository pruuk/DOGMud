package questengine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestQuest35_GiveDaggerCompletesFirstHeat locks the fix for the newbie-area
// "First Heat stalls at 66%" bug (Malia playtest, 2026-06-29): a player who
// did the intuitive thing, physically GIVE the finished iron dagger (item
// 10009) to Smith Rusk (mob 9116), got it handed back with no quest progress,
// leaving the quest parked at 35-craft.
//
// The 2026-06-29 fix was a quest item_give trigger. Since #375 (Task 10) a
// give resolves as the topic that wants the item (conversation.Give over
// DialogueFile.ItemTopic), and the trigger is gone (Task 16). This test loads
// Rusk's REAL converted dialogue and asserts that, for a player on 35-craft
// who carries the dagger, the dagger resolves to the craft_turnin hand-in and
// playing it grants 35-end. Before the craft the dagger is wanted but no
// hand-in is open, so give hands it back instead of keeping it.
func TestQuest35_GiveDaggerCompletesFirstHeat(t *testing.T) {
	const (
		ruskMobId    = 9116
		ironDaggerId = 10009
	)
	rusk := loadCouleeTopics(t, "9116.yaml")
	require.Equal(t, ruskMobId, rusk.MobId)
	q := loadShippedQuest(t, "35-first_heat.yaml", 35)

	p := playerOn(q, "craft", "30-end")
	p.items[ironDaggerId] = true
	n := newNpcTalker(rusk, p)
	tp, wanted := rusk.ItemTopic(ironDaggerId, n.state())
	require.True(t, wanted, "Rusk must want the dagger")
	require.NotNil(t, tp, "giving the finished dagger to Rusk must play a hand-in")
	require.Equal(t, "craft_turnin", tp.Key)
	require.Equal(t, ironDaggerId, tp.CarriesItem, "the dagger is shown, never taken")
	require.True(t, rusk.Apply(tp, n.state()))
	require.True(t, p.HasQuest("35-end"), "giving the finished dagger to Rusk must complete First Heat (grant 35-end)")
	require.True(t, p.items[ironDaggerId], "the player keeps the dagger")

	early := playerOn(q, "strip", "30-end")
	early.items[ironDaggerId] = true
	tp, wanted = rusk.ItemTopic(ironDaggerId, newNpcTalker(rusk, early).state())
	require.True(t, wanted)
	require.Nil(t, tp, "no hand-in plays before the dagger is crafted")
}

// TestQuest69_GiveRubbingAdvancesGalleryStep locks the same fix for the Gallery
// Cipher quest (F3 audit, 2026-06-29). The quest log says "Take the rubbing to
// Dross", and a player who physically gave the rubbing (item 40115) to Dross
// (mob 9360) must advance rubbing to gallery. Since #375 the give resolves as
// Dross's q69_gallery topic (carriesItem 40115), which leaves the rubbing with
// the player, who needs it at the gallery. Since #375 phase 3b the quest's
// topic_heard trigger on that topic grants 69-gallery once it is heard.
func TestQuest69_GiveRubbingAdvancesGalleryStep(t *testing.T) {
	const (
		drossMobId  = 9360
		rubbingItem = 40115
	)
	dross := loadShippedDialogue(t, "new_plymouth_temple", "9360.yaml")
	require.Equal(t, drossMobId, dross.MobId)
	q := loadShippedQuest(t, "69-the_gallery_cipher.yaml", 69)

	p := playerOn(q, "rubbing")
	p.items[rubbingItem] = true
	n := newNpcTalker(dross, p)
	tp, wanted := dross.ItemTopic(rubbingItem, n.state())
	require.True(t, wanted, "Dross must want the rubbing")
	require.NotNil(t, tp, "giving the rubbing to Dross must play a topic")
	require.Equal(t, "q69_gallery", tp.Key)
	require.True(t, dross.Apply(tp, n.state()))
	require.False(t, p.HasQuest("69-gallery"), "the topic grants nothing itself; the quest's topic_heard trigger does")

	e := NewEngine()
	e.RegisterQuest(q)
	ctx := &linearCtx{mockActionContext: newMockActionContext(1), p: p}
	e.Notify("topic_heard", EventDetails{UserId: 1, MobId: drossMobId, Topic: tp.Key}, p, ctx)
	require.True(t, p.HasQuest("69-gallery"), "giving the rubbing to Dross must advance the quest (grant 69-gallery)")
	require.True(t, p.items[rubbingItem], "the player must keep a rubbing (needed at the gallery and at turn-in)")
	require.Empty(t, n.removed, "Dross takes nothing")
}
