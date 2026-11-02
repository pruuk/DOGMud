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
	t.Run("Lore", func(t *testing.T) {
		checkBridge(t, toke, 95, "Lore", "95-report_to_toke__lore_.yaml",
			"9160.yaml", "report_lore_offer", "report_lore_in", "49-end")
		q := loadShippedQuest(t, "95-report_to_toke__lore_.yaml", 95)
		require.Contains(t, squash(questHintFor(t, q, "start")), loreBridgeWords)
		require.Equal(t, foldingHubSquare, foldingWalk(t, 5341, loreBridgeWalk...),
			"the way back to Hub Square walks")
		offer := loadCouleeTopics(t, "9160.yaml").FindTopic("report_lore_offer")
		require.Contains(t, squash(offer.Hints), loreBridgeWords, "Sere's offer gives the same way")
	})
	t.Run("Archery", func(t *testing.T) {
		checkBridge(t, toke, 96, "Archery", "96-report_to_toke__archery_.yaml",
			"9162.yaml", "report_archery_offer", "report_archery_in", "52-end")
		q := loadShippedQuest(t, "96-report_to_toke__archery_.yaml", 96)
		require.Contains(t, squash(questHintFor(t, q, "start")), archeryBridgeWords)
		require.Equal(t, foldingHubSquare, foldingWalk(t, canyonMouth, archeryBridgeWalk...),
			"the way back to Hub Square walks")
		offer := loadCouleeTopics(t, "9162.yaml").FindTopic("report_archery_offer")
		require.Contains(t, squash(offer.Hints), archeryBridgeWords, "Bryn's offer gives the same way")
	})
	t.Run("Shadows", func(t *testing.T) {
		checkBridge(t, toke, 97, "Shadows", "97-report_to_toke__shadows_.yaml",
			"9126.yaml", "report_shadows_offer", "report_shadows_in", "100-end")
		q := loadShippedQuest(t, "97-report_to_toke__shadows_.yaml", 97)
		require.Contains(t, squash(questHintFor(t, q, "start")), shadowsBridgeWords)
		require.Equal(t, foldingHubSquare, foldingWalk(t, reedJetty, "south", "east", "south"),
			"the way back to Hub Square walks")
		offer := loadCouleeTopics(t, "9126.yaml").FindTopic("report_shadows_offer")
		require.Contains(t, squash(offer.Hints), shadowsBridgeWords, "Dace's offer gives the same way")
	})
}

const (
	reedJetty          = 5218
	mineHead           = 5255
	timberedDrift      = 5256
	floodedSump        = 5257
	shadowsBridgeWords = "from the reed jetty go south, east to the awakening pool, then south to hub square"
)

// The Shadows line's lines give these ways, so they are walked here: plan text
// once named a route that did not exist.
func TestShadowsLine_RoutesWalk(t *testing.T) {
	toHead := []string{"north", "west", "west", "west", "west", "west", "north",
		"west", "west", "west", "west", "west", "down"}
	require.Equal(t, mineHead, foldingWalk(t, foldingHubSquare, toHead...), "Hub Square to the Mine Head")
	require.Equal(t, timberedDrift, foldingWalk(t, mineHead, "west"))
	require.Equal(t, floodedSump, foldingWalk(t, timberedDrift, "west"))
	require.Equal(t, mineHead, foldingWalk(t, floodedSump, "east", "east"))

	headToJetty := []string{"up", "east", "east", "east", "east", "east", "south",
		"east", "east", "east", "east", "north"}
	require.Equal(t, reedJetty, foldingWalk(t, mineHead, headToJetty...), "Mine Head to the Reed Jetty")
	jettyToDrift := []string{"south", "west", "west", "west", "west", "north",
		"west", "west", "west", "west", "west", "down", "west"}
	require.Equal(t, timberedDrift, foldingWalk(t, reedJetty, jettyToDrift...), "Reed Jetty to the drift")

	toke := loadCouleeTopics(t, "9106.yaml")
	require.Contains(t, squash(toke.FindTopic("send_light").Hints),
		"from hub square go north, west five times to the coulee smithy, north, west five times to the mine mouth, then down")

	const headWords = "from the mine head go up, east five times, south to the smithy, east four times, then north to the reed jetty"
	require.Contains(t, squash(loadCouleeTopics(t, "9125.yaml").FindTopic("quiet_hands_offer").Hints), headWords)
	require.Contains(t, squash(questHintFor(t, loadShippedQuest(t, "99-quiet_hands.yaml", 99), "start")), headWords)
	require.Contains(t, squash(questHintFor(t, loadShippedQuest(t, "100-from_the_dark.yaml", 100), "kill")), headWords)

	const jettyWords = "from the reed jetty go south, west four times, north, west five times, down, then west into the timbered drift"
	require.Contains(t, squash(loadCouleeTopics(t, "9126.yaml").FindTopic("from_the_dark_offer").Hints), jettyWords)
	require.Contains(t, squash(questHintFor(t, loadShippedQuest(t, "100-from_the_dark.yaml", 100), "start")), jettyWords)
}

// Once all eight bridges are in, the last [!] on Toke's menu is Esk's Road, and
// his opener names it.
func TestToke_AllBridgesInOpenerNamesEsksRoad(t *testing.T) {
	toke := loadCouleeTopics(t, "9106.yaml")
	q := loadShippedQuest(t, "98-first_light.yaml", 98)
	held := []string{"30-end", "31-end", "88-end", "32-start", "35-start", "38-start", "41-start",
		"44-start", "47-start", "50-start"}
	for id := 90; id <= 97; id++ {
		held = append(held, fmt.Sprintf("%d-end", id))
	}
	n := newNpcTalker(toke, playerOn(q, "end", held...))
	_, hints := n.opener()
	require.Contains(t, squash(hints), "choose esk's road")
	require.NotNil(t, n.eligible("road_esk"))

	_, hints = newNpcTalker(toke, playerOn(q, "end", held[:len(held)-1]...)).opener()
	require.NotContains(t, squash(hints), "choose esk's road", "a bridge is still out")
}

// The routes the leaving lines give, walked: Hub Square to the Threshold House,
// and the Awakening Pool to the same room for Hadwen's another_way.
func TestEsksRoad_RoutesWalk(t *testing.T) {
	const pool, threshold = 5200, 5215
	require.Equal(t, threshold, foldingWalk(t, foldingHubSquare, "south", "south"))
	require.Equal(t, threshold, foldingWalk(t, pool, "south", "south", "south"))
	require.Contains(t, squash(questHintFor(t, loadShippedQuest(t, "89-esks_road.yaml", 89), "start")),
		"from hub square go south twice")
	require.Contains(t, squash(loadCouleeTopics(t, "9106.yaml").FindTopic("road_esk").Hints),
		"from hub square go south twice")
	require.Contains(t, squash(loadCouleeTopics(t, "9100.yaml").FindTopic("another_way").Hints),
		"go south three times")
}

// A holder of Esk's Road who crosses the arch keeps it: the hand-in at Esk is
// its only end, and the temple portal leads back to him.
func TestEsksRoad_CrossingTheArchDoesNotEndIt(t *testing.T) {
	const thornwallTemple = 468
	w := newQuestWalker(loadShippedQuest(t, "89-esks_road.yaml", 89)).at("start")
	w.notify("room_enter", EventDetails{RoomId: thornwallTemple})
	require.Equal(t, "start", w.p.current, "arriving through the arch moved quest 89")
}

// The hand-over for lost gear: a player with no lantern or picks is given more,
// and one who carries them is told so. equip reads only the pack, and "lantern"
// alone can match an oil lantern, so First Light names the hooded lantern.
func TestShadowsLine_LostGearAndHoodLines(t *testing.T) {
	gethin := loadCouleeTopics(t, "9125.yaml")
	spare, have := gethin.FindTopic("lantern_spare"), gethin.FindTopic("lantern_have")
	require.Equal(t, 20097, spare.ItemExcluded)
	require.Equal(t, 20097, spare.GivesItem)
	require.Equal(t, 20097, have.CarriesItem)
	require.Equal(t, spare.QuestRequired, have.QuestRequired)
	require.Equal(t, spare.QuestExcluded, have.QuestExcluded)

	dace := loadCouleeTopics(t, "9126.yaml")
	spare, have = dace.FindTopic("lockpicks_spare"), dace.FindTopic("lockpicks_have")
	require.Equal(t, 33, spare.ItemExcluded)
	require.Equal(t, 33, spare.GivesItem)
	require.Equal(t, 33, have.CarriesItem)
	require.Equal(t, spare.QuestRequired, have.QuestRequired)
	require.Equal(t, spare.QuestExcluded, have.QuestExcluded)

	q98 := loadShippedQuest(t, "98-first_light.yaml", 98)
	lantern := squash(questHintFor(t, q98, "lantern"))
	require.Contains(t, lantern, "type equip hooded lantern")
	require.Contains(t, lantern, "remove lantern")
	require.Equal(t, 1, typedCount(questHintFor(t, q98, "lantern")))
	require.Contains(t, squash(questHintFor(t, q98, "drift")), "unhood")
}

// The archery bridge's walk from the Canyon Mouth, where Bryn gives it, to
// Hub Square, and the words that say it.
var archeryBridgeWalk = []string{"south", "west", "west", "down", "south", "south", "down", "south", "west", "west", "south"}

const archeryBridgeWords = "from the canyon mouth, go south, west, west, down, south, south, down, south, west, west and south"

// The lore bridge's walk from the Old Shrine, where Sere gives it, to Hub
// Square, and the words that say it.
var loreBridgeWalk = []string{"south", "south", "east", "south", "south", // the Stone Circle
	"south", "east", "south", "south", "east", "south", "south", "south", "south", "south", "south", // the shore
	"west", "south"}

const loreBridgeWords = "go south twice, east, and south twice to the stone circle. then south, east, " +
	"south twice, east, south six times, west, south."

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
	require.Contains(t, squash(handin.Text), "any mouth you have not walked yet is still open to you",
		"the hand-in's ending stays true once every trade is taken")
	newNpcTalker(toke, n.p).choose(t, handinKey)
	require.True(t, n.p.HasQuest(fmt.Sprintf("%d-end", id)), "Toke's hand-in completes the bridge")
}

// A player holding a bridge is told, above the hub, which line on Toke's menu
// reports it, and that line sits above every trade offer. The hub's own hint
// names only the [!] lines.
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
		{95, "Lore", "95-report_to_toke__lore_.yaml", "report_lore_in"},
		{96, "Archery", "96-report_to_toke__archery_.yaml", "report_archery_in"},
		{97, "Shadows", "97-report_to_toke__shadows_.yaml", "report_shadows_in"},
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

// With all eight trades taken the hub's "[!] below" describes lines that are
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
	require.Len(t, trades, 8, "Toke starts eight trades")
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
	t.Run("Shadows", func(t *testing.T) { checkTokeSends(t, toke, "send_light", 98, "gethin") })
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
