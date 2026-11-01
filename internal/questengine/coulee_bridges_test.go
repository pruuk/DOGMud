package questengine

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
)

// The Report to Toke bridges (#375 phase 7): a line's second giver offers its
// bridge only once the finale is handed in, and Crier Toke (9106) takes it.
// Quest names follow the shared naming; the file name is the quest name
// through ConvertForFilename, as the loader demands.
func TestCouleeBridges_GiverTwoOffersTokeTakes(t *testing.T) {
	toke := loadCouleeTopics(t, "9106.yaml")
	t.Run("Fighting", func(t *testing.T) {
		checkBridge(t, toke, 90, "Fighting", "90-report_to_toke__fighting_.yaml",
			"9112.yaml", "report_fighting_offer", "report_fighting_in", "34-end")
	})
}

// checkBridge walks one bridge: the quest's shape, the giver's offer gated
// on the finale's end token, and Toke's hand-in completing it.
func checkBridge(t *testing.T, toke *dialogue.DialogueFile, id int, line, file, giverFile, offerKey, handinKey, after string) {
	t.Helper()
	q := loadShippedQuest(t, file, id)
	require.Equal(t, "Report to Toke ("+line+")", q.Name)
	require.Equal(t, []string{"start", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, questHintFor(t, q, "start"), "type talk toke")

	giver := loadCouleeTopics(t, giverFile)
	offer := giver.FindTopic(offerKey)
	require.NotNil(t, offer, "%s has no topic %q", giverFile, offerKey)
	require.Equal(t, dialogue.KindOffer, offer.Kind)
	require.Equal(t, id, offer.Quest)
	require.Equal(t, []string{after}, offer.QuestRequired, "the bridge opens on the finale's hand-in")
	require.Nil(t, newNpcTalker(giver, playerOn(q, "", "30-end")).eligible(offerKey),
		"no bridge before the finale is handed in")

	n := newNpcTalker(giver, playerOn(q, "", "30-end", after))
	n.choose(t, offerKey)
	require.Equal(t, "start", n.p.current)

	handin := toke.FindTopic(handinKey)
	require.NotNil(t, handin, "Toke has no topic %q", handinKey)
	require.Equal(t, dialogue.KindHandIn, handin.Kind)
	require.Equal(t, []string{fmt.Sprintf("%d-start", id)}, handin.QuestRequired)
	newNpcTalker(toke, n.p).choose(t, handinKey)
	require.True(t, n.p.HasQuest(fmt.Sprintf("%d-end", id)), "Toke's hand-in completes the bridge")
}

// Toke's quest 1 offers send the player to the line's first giver (#375
// phase 7, controller ruling): the hint names `talk <giver>` and no other
// command, because the quest now begins with that talk.
func TestTokeQuestOneOffers_SendToTheGiver(t *testing.T) {
	toke := loadCouleeTopics(t, "9106.yaml")
	t.Run("Fighting", func(t *testing.T) { checkTokeSends(t, toke, "send_drill", 32, "vorn") })
}

func checkTokeSends(t *testing.T, toke *dialogue.DialogueFile, key string, quest int, npc string) {
	t.Helper()
	tp := toke.FindTopic(key)
	require.NotNil(t, tp, "Toke has no topic %q", key)
	require.Equal(t, dialogue.KindOffer, tp.Kind)
	require.Equal(t, quest, tp.Quest)
	require.Contains(t, squash(tp.Hints), "type talk "+npc, "%s sends the player to %s", key, npc)
	require.Equal(t, 1, typedCount(tp.Hints), "%s names one command: %q", key, tp.Hints)
}
