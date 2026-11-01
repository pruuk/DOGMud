package questengine

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
)

// heldState is a dialogue state holding exactly these tokens. A caller lists a
// step's earlier tokens too, as Character.HasQuest would answer for them.
func heldState(tokens ...string) *dialogue.PlayerState {
	held := map[string]bool{}
	for _, tok := range tokens {
		held[tok] = true
	}
	return &dialogue.PlayerState{
		QuestOpen:    func(int) bool { return true },
		HasQuest:     func(tok string) bool { return held[tok] },
		HasItem:      func(int) bool { return false },
		CarriesItem:  func(int) bool { return false },
		GetQuestFlag: func(string) string { return "" },
		HasHeard:     func(string) bool { return false },
		MarkHeard:    func(string) {},
	}
}

// Crier Toke's lines read true for each state on the way to Find Your Footing:
// no footing task is named before its offer is on the menu, the player Hadwen
// sent is pointed at the [?] line, and the breadcrumb holds whether or not
// Hadwen has sent them yet.
func TestTokeNamesWhatIsOnTheMenu(t *testing.T) {
	df := loadShippedDialogue(t, "pothole_coulee", "9106.yaml")
	opened := []string{"30-start", "30-end"}
	sent := append([]string{"88-start"}, opened...)
	done := append([]string{"88-end"}, sent...)

	for _, c := range []struct {
		name    string
		tokens  []string
		offer   bool     // Find Your Footing is on the menu
		hasCopy []string // the hint names each of these
	}{
		{"opened, no road yet", opened, false, []string{"Cleric Hadwen", "Toke's Road"}},
		{"Hadwen has sent them", sent, false, []string{"Toke's Road", "[?]"}},
		{"road handed in", done, true, []string{"Find Your Footing", "[!]"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			ps := heldState(c.tokens...)
			require.Equal(t, c.offer, df.Eligible(df.FindTopic("footing_offer"), ps))
			_, hints := df.OpenerFor(ps)
			for _, want := range c.hasCopy {
				require.Contains(t, hints, want)
			}
			if !c.offer {
				require.NotContains(t, strings.ToLower(hints), "footing")
				require.NotContains(t, strings.ToLower(hints), "task to")
				crumbs := strings.Join(df.Breadcrumbs(ps), " ")
				require.Contains(t, crumbs, "Hadwen")
				require.NotContains(t, crumbs, "Once he has", "said to someone Hadwen may have sent already")
			}
		})
	}

	require.NotContains(t, df.FindTopic("news").Hints, "task",
		"the news topic is open before any task is on the menu")
}

// Garve sends a player with no fighting trade to the one who starts it. Crier
// Toke starts every trade, and Vorn himself sends a newcomer back to Toke, so
// Garve names Vorn only once First Blood is under way.
func TestGarveSendsANewcomerToToke(t *testing.T) {
	df := loadShippedDialogue(t, "pothole_coulee", "9112.yaml")

	text, hints := df.OpenerFor(heldState("30-start", "30-end"))
	require.Contains(t, hints, "Toke")
	require.NotContains(t, hints, "Vorn")
	require.Contains(t, text, "Toke")
	require.NotContains(t, text, "Vorn")

	text, hints = df.OpenerFor(heldState("30-start", "30-end", "32-start"))
	require.Contains(t, hints, "Vorn")
	require.Contains(t, text, "Vorn")
}

// Find Your Footing (quest 31) under the owner's one-command-per-step ruling
// (2026-10-17): the shop step asked for `list` and then `buy cotton shirt`
// in one hint. It is now two steps, list then buy, each its own trigger,
// with the old step ids kept so a saved character resumes.
func TestQuest31FindYourFooting_OneCommandPerStep(t *testing.T) {
	const (
		hubSquare   = 5203
		marketRow   = 5204
		lantern     = 5205
		provisions  = 5207
		schoolHall  = 5213
		cottonShirt = 20008
	)
	q := loadShippedQuest(t, "31-find_your_footing.yaml", 31)
	steps := newLinearPlayer(q).steps
	for _, old := range []string{"start", "school", "shop", "equip", "lantern", "end"} {
		require.Contains(t, steps, old, "step %q must survive so a player mid-quest resumes", old)
	}
	turnIn := turnInRequires(t, "9106.yaml", "footing_turnin")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")

		w.notify("room_enter", EventDetails{RoomId: schoolHall})
		require.Equal(t, "school", w.p.current)

		// Buying before listing must not skip the list step.
		w.notify("item_gain", EventDetails{ItemId: cottonShirt, RoomId: provisions})
		require.Equal(t, "school", w.p.current, "a purchase before list must not skip the list step")

		// A list anywhere but Onna's shop is not this lesson.
		w.notify("command", EventDetails{Command: "list", RoomId: marketRow})
		require.Equal(t, "school", w.p.current, "list outside Coulee Provisions must not count")

		w.notify("command", EventDetails{Command: "list", RoomId: provisions})
		require.Equal(t, "list", w.p.current)

		// Equipping before buying must not skip the buy step.
		w.notify("command", EventDetails{Command: "equip", RoomId: provisions})
		require.Equal(t, "list", w.p.current, "equip before the purchase must not skip the buy step")

		w.notify("item_gain", EventDetails{ItemId: cottonShirt, RoomId: provisions})
		require.Equal(t, "shop", w.p.current)

		// Walking into the inn before equipping must not skip the equip step.
		w.notify("room_enter", EventDetails{RoomId: lantern})
		require.Equal(t, "shop", w.p.current, "the inn before equip must not skip the equip step")

		// Putting on anything but the shirt is not this lesson (the equip
		// event names its item; 2026-10-17 newbie-duo playtest).
		w.notify("command", EventDetails{Command: "equip", RoomId: provisions, ItemId: 10043})
		require.Equal(t, "shop", w.p.current, "equipping the cudgel must not finish the shirt step")

		w.notify("command", EventDetails{Command: "equip", RoomId: provisions, ItemId: cottonShirt})
		require.Equal(t, "equip", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the inn")

		w.notify("room_enter", EventDetails{RoomId: lantern})
		require.Equal(t, "lantern", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the last step")
	})

	t.Run("player saved on start resumes at the school", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("room_enter", EventDetails{RoomId: schoolHall})
		require.Equal(t, "school", w.p.current)
	})

	t.Run("player saved on school resumes with list", func(t *testing.T) {
		w := newQuestWalker(q).at("school")
		require.False(t, holdsAll(w.p, turnIn))
		w.notify("command", EventDetails{Command: "list", RoomId: provisions})
		require.Equal(t, "list", w.p.current)
		w.notify("item_gain", EventDetails{ItemId: cottonShirt, RoomId: provisions})
		require.Equal(t, "shop", w.p.current)
	})

	t.Run("player saved on shop resumes with equip", func(t *testing.T) {
		w := newQuestWalker(q).at("shop")
		w.notify("command", EventDetails{Command: "equip", RoomId: hubSquare, ItemId: cottonShirt})
		require.Equal(t, "equip", w.p.current)
	})

	t.Run("player saved on equip resumes at the inn", func(t *testing.T) {
		w := newQuestWalker(q).at("equip")
		w.notify("room_enter", EventDetails{RoomId: lantern})
		require.Equal(t, "lantern", w.p.current)
	})

	t.Run("player saved on lantern can turn in", func(t *testing.T) {
		w := newQuestWalker(q).at("lantern")
		require.True(t, holdsAll(w.p, turnIn))
	})
}
