package questengine

import (
	"fmt"
	"strings"
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
	t.Run("Smithing", func(t *testing.T) {
		checkBridge(t, toke, 91, "Smithing", "91-report_to_toke__smithing_.yaml",
			"9117.yaml", "report_smithing_offer", "report_smithing_in", "37-end")
	})
	t.Run("Brewing", func(t *testing.T) {
		checkBridge(t, toke, 92, "Brewing", "92-report_to_toke__brewing_.yaml",
			"9129.yaml", "report_brewing_offer", "report_brewing_in", "40-end")
	})
	t.Run("Tracking", func(t *testing.T) {
		checkBridge(t, toke, 93, "Tracking", "93-report_to_toke__tracking_.yaml",
			"9137.yaml", "report_tracking_offer", "report_tracking_in", "43-end")
	})
	t.Run("Casting", func(t *testing.T) {
		checkBridge(t, toke, 94, "Casting", "94-report_to_toke__casting_.yaml",
			"9145.yaml", "report_casting_offer", "report_casting_in", "46-end")
		q := loadShippedQuest(t, "94-report_to_toke__casting_.yaml", 94)
		require.Contains(t, squash(questHintFor(t, q, "start")),
			"go south three times, east, south, down, down, south three times and east to hub square")
		require.Equal(t, foldingHubSquare, foldingWalk(t, foldingOrrinRoom,
			"south", "south", "south", "east", "south", "down", "down", "south", "south", "south", "east"),
			"the way back to Hub Square walks")
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

// A player holding a bridge is told, above the hub, which line on Toke's menu
// reports it, and that line sits above every trade offer. The hub's own hint
// names only the [!] lines. Plans 3b and 4 add bridges 94 to 97: add each here.
func TestToke_BridgeOpenerNamesTheHandInAndItComesFirst(t *testing.T) {
	toke := loadCouleeTopics(t, "9106.yaml")
	pre := []string{"30-end", "31-end", "88-end"}
	for _, b := range []struct {
		id        int
		line      string
		file      string
		handinKey string
	}{
		{90, "Fighting", "90-report_to_toke__fighting_.yaml", "report_fighting_in"},
		{91, "Smithing", "91-report_to_toke__smithing_.yaml", "report_smithing_in"},
		{92, "Brewing", "92-report_to_toke__brewing_.yaml", "report_brewing_in"},
		{93, "Tracking", "93-report_to_toke__tracking_.yaml", "report_tracking_in"},
		{94, "Casting", "94-report_to_toke__casting_.yaml", "report_casting_in"},
	} {
		t.Run(b.line, func(t *testing.T) {
			q := loadShippedQuest(t, b.file, b.id)
			n := newNpcTalker(toke, playerOn(q, "start", pre...))

			_, hints := n.opener()
			require.Contains(t, squash(hints), "choose report to toke ("+strings.ToLower(b.line)+")")
			require.Contains(t, hints, "[?]")
			require.NotContains(t, hints, "[!]", "the hub's hint must not stand in for the report")

			entries := toke.Menu("Crier Toke", n.state()).Entries
			require.NotEmpty(t, entries)
			require.Equal(t, b.handinKey, entries[0].Id, "the report comes before every trade offer")

			n.choose(t, b.handinKey)
			_, hints = n.opener()
			require.Contains(t, hints, "[!]", "handed in, the opener goes back to the hub")
		})
	}
}

// With all seven trades taken the hub's "[!] below" describes lines that are
// not there. The trade list is read from Toke's own send_ offers, so a trade
// added to the menu without joining the variant's gate fails here.
func TestToke_NoTradesLeftOpener(t *testing.T) {
	toke := loadCouleeTopics(t, "9106.yaml")
	q := loadShippedQuest(t, "90-report_to_toke__fighting_.yaml", 90)
	var trades, sendKeys []string
	for i := range toke.Topics {
		if tp := &toke.Topics[i]; strings.HasPrefix(tp.Key, "send_") {
			trades = append(trades, tp.GrantsQuest)
			sendKeys = append(sendKeys, tp.Key)
		}
	}
	require.Len(t, trades, 7, "Toke starts seven trades")
	pre := []string{"30-end", "31-end", "88-end"}

	all := newNpcTalker(toke, playerOn(q, "", append(append([]string{}, pre...), trades...)...))
	_, hints := all.opener()
	require.NotContains(t, hints, "[!]", "no trade is left to start")
	for _, key := range sendKeys {
		require.Nil(t, all.eligible(key), "no offer stays open")
	}

	for skip := range trades {
		held := append([]string{}, pre...)
		for i, tp := range trades {
			if i != skip {
				held = append(held, tp)
			}
		}
		_, hints := newNpcTalker(toke, playerOn(q, "", held...)).opener()
		require.Contains(t, hints, "[!]", "%s is still open, so the hub stands", trades[skip])
	}

	// A held bridge outranks it: the report is the thing to do next.
	bridged := newNpcTalker(toke, playerOn(q, "start", append(append([]string{}, pre...), trades...)...))
	_, hints = bridged.opener()
	require.Contains(t, squash(hints), "choose report to toke (fighting)")
}

// Toke's quest 1 offers send the player to the line's first giver (#375
// phase 7, controller ruling): the hint names `talk <giver>` and no other
// command, because the quest now begins with that talk.
func TestTokeQuestOneOffers_SendToTheGiver(t *testing.T) {
	toke := loadCouleeTopics(t, "9106.yaml")
	t.Run("Fighting", func(t *testing.T) { checkTokeSends(t, toke, "send_drill", 32, "vorn") })
	t.Run("Smithing", func(t *testing.T) { checkTokeSends(t, toke, "send_forge", 35, "rusk") })
	t.Run("Brewing", func(t *testing.T) { checkTokeSends(t, toke, "send_brew", 38, "birna") })
	t.Run("Tracking", func(t *testing.T) { checkTokeSends(t, toke, "send_steppe", 41, "tarn") })
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
