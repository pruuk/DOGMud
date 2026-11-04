package dialogue

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func carrying(ps *PlayerState, ids ...int) *PlayerState {
	has := func(id int) bool {
		for _, i := range ids {
			if i == id {
				return true
			}
		}
		return false
	}
	ps.HasItem, ps.CarriesItem = has, has
	return ps
}

func withFlags(ps *PlayerState, flags map[string]string) *PlayerState {
	ps.GetQuestFlag = func(k string) string { return flags[k] }
	return ps
}

func TestTier5DockRatHandInsFollowTheBook(t *testing.T) {
	for _, file := range []string{"new_plymouth_docks/9300.yaml", "new_plymouth_docks/9316.yaml"} {
		df := loadShippedDialogue(t, file)
		require.NotContains(t, menuKeys(df, carrying(heldState("63-start"), 40100)), "tallyhandoff", file)
		require.Contains(t, menuKeys(df, carrying(heldState("63-start", "63-evidence"), 40100)), "tallyhandoff", file)
	}
}

func TestTier5YsoldeReplacesALostPurge(t *testing.T) {
	y := loadShippedDialogue(t, "new_plymouth_common/9323.yaml")
	text, _ := y.OpenerFor(heldState("66-start", "66-dose"))
	require.Contains(t, text, "Where is the cup")
	require.Contains(t, menuKeys(y, heldState("66-start", "66-dose")), "detox")
	text, _ = y.OpenerFor(carrying(heldState("66-start", "66-dose"), 40109))
	require.Contains(t, text, "You have the cup")
	require.NotContains(t, menuKeys(y, carrying(heldState("66-start", "66-dose"), 40109)), "detox", "one cup at a time")
}

// The detox hint reads for a patient and for a player on Teels's errand alike.
func TestTier5DetoxHintDoesNotSendTheErrandToDrinkIt(t *testing.T) {
	y := loadShippedDialogue(t, "new_plymouth_common/9323.yaml")
	require.Equal(t, "If this cup is for someone else, take it to them. To drink it yourself, type quaff purge.", y.FindTopic("detox").Hints)
}

// A hand-in's opener says "choose X below" only when X is on the menu: both read
// the step token and what a topic can take. A player handed the item by someone
// else, or holding it where a topic cannot take it, reads no such promise.
func TestTier5OpenersPromiseOnlyTheHandInOnTheMenu(t *testing.T) {
	for _, c := range []struct {
		file, topic, promise string
		before               []string // holding the item short of the step, when that can happen
		onStep               []string
		item                 int
	}{
		{"new_plymouth_docks/9316.yaml", "tallyhandoff", "Choose Dock Rat below", []string{"63-start"}, []string{"63-start", "63-evidence"}, 40100},
		{"new_plymouth_docks/9300.yaml", "tallyhandoff", "Choose Dock Rat below", []string{"63-start"}, []string{"63-start", "63-evidence"}, 40100},
		{"new_plymouth_docks/9316.yaml", "casefilehandoff", "Choose The Bloom Trail below", []string{"67-start"}, []string{"67-start", "67-witness"}, 40111},
		{"new_plymouth_common/9320.yaml", "sweeperhandoff", "Choose The Street Sweeper's Secret below", []string{"65-start"}, []string{"65-start", "65-discover"}, 40101},
		{"new_plymouth_common/9323.yaml", "wrapperhandoff", "Choose the line about the wrapper below", nil, []string{"66-start"}, 40110},
		{"new_plymouth_docks/9392.yaml", "purge_handoff", "Choose The Addict's Plight below", []string{"66-start"}, []string{"66-start", "66-dose"}, 40109},
	} {
		df := loadShippedDialogue(t, c.file)
		state := func(held []string, has, carries bool) *PlayerState {
			ps := heldState(held...)
			ps.HasItem = func(id int) bool { return has && id == c.item }
			ps.CarriesItem = func(id int) bool { return carries && id == c.item }
			return ps
		}
		agree := func(name string, ps *PlayerState, want bool) {
			_, hints := df.OpenerFor(ps)
			require.Equal(t, want, strings.Contains(hints, c.promise), "%s %s %s opener: %q", c.file, c.topic, name, hints)
			if want {
				require.Contains(t, menuKeys(df, ps), c.topic, "%s %s", c.file, name)
			} else {
				require.NotContains(t, menuKeys(df, ps), c.topic, "%s %s", c.file, name)
			}
		}
		agree("on the step and takeable", state(c.onStep, true, true), true)
		agree("on the step, carried where no topic can take it", state(c.onStep, false, true), false)
		agree("on the step with nothing", state(c.onStep, false, false), false)
		if c.before != nil {
			agree("handed the item short of the step", state(c.before, true, true), false)
		}
	}
}

// Ysolde does not name who sells it (she says she does not know). The draper
// comes from the player, who had it from Teels, and only until Marn is past.
func TestTier5YsoldeTakesTheDraperFromThePlayer(t *testing.T) {
	y := loadShippedDialogue(t, "new_plymouth_common/9323.yaml")
	text, _ := y.OpenerFor(heldState("66-end", "67-start"))
	require.Equal(t, "Teels gave you the draper, then. Start there.", text)
	text, hints := y.OpenerFor(heldState("66-end", "67-start", "67-front"))
	require.NotContains(t, text, "Start there")
	require.NotContains(t, hints, "Marn")
}

// Every hint on the tier 5 quests' NPCs fits the cap, with no schedule in it.
func TestTier5HintsAreShort(t *testing.T) {
	for _, file := range []string{
		"new_plymouth_common/9320.yaml", "new_plymouth_common/9323.yaml",
		"new_plymouth_docks/9300.yaml", "new_plymouth_docks/9305.yaml", "new_plymouth_docks/9316.yaml", "new_plymouth_docks/9392.yaml",
		"new_plymouth_noble/9369.yaml", "new_plymouth_old_quarter/9379.yaml", "new_plymouth_old_quarter/9381.yaml",
	} {
		df := loadShippedDialogue(t, file)
		hints := []string{df.Opener.Hints}
		for _, v := range df.Opener.Variants {
			hints = append(hints, v.Hints)
		}
		for _, tp := range df.Topics {
			hints = append(hints, tp.Hints)
		}
		for _, h := range hints {
			require.LessOrEqual(t, len(h), 200, "%s: %q", file, h)
			require.NotContains(t, h, "late afternoon", file)
			require.NotContains(t, h, "by night", file)
		}
	}
}

func TestTier5TeelsTakesThePurge(t *testing.T) {
	teels := loadShippedDialogue(t, "new_plymouth_docks/9392.yaml")
	dose := heldState("66-start", "66-dose")
	require.NotContains(t, menuKeys(teels, dose), "purge_handoff")
	text, hints := teels.OpenerFor(dose)
	require.Contains(t, text, "Where is it?")
	require.Contains(t, hints, "Ysolde has more")

	withCup := carrying(heldState("66-start", "66-dose"), 40109)
	require.Contains(t, menuKeys(teels, withCup), "purge_handoff")
	text, _ = teels.OpenerFor(withCup)
	require.Contains(t, text, "Give it here")

	// A purge from Ysolde's cure before the wrapper reached her is handed back.
	require.NotContains(t, menuKeys(teels, carrying(heldState("66-start"), 40109)), "purge_handoff")

	done := heldState("66-start", "66-dose", "66-end")
	require.Contains(t, menuKeys(teels, done), "q67_grant")
	require.NotContains(t, menuKeys(teels, heldState("66-start", "66-dose", "66-end", "67-start")), "q67_grant")
}

func TestTier5CaseFileHandInFollowsTheWitness(t *testing.T) {
	c := loadShippedDialogue(t, "new_plymouth_docks/9316.yaml")
	require.NotContains(t, menuKeys(c, carrying(heldState("67-start"), 40111)), "casefilehandoff")
	require.Contains(t, menuKeys(c, carrying(heldState("67-start", "67-witness"), 40111)), "casefilehandoff")
	require.NotContains(t, c.FindTopic("casefilehandoff").Text, "wrapper")
	d := loadShippedDialogue(t, "new_plymouth_old_quarter/9379.yaml")
	require.NotContains(t, d.FindTopic("operation").Text, "no longer runs")
	for _, g := range d.Greetings {
		require.NotContains(t, g.Text, "already closed")
	}
}

func TestTier5GrittaDoesNotSendColl(t *testing.T) {
	gritta := loadShippedDialogue(t, "new_plymouth_old_quarter/9381.yaml")
	require.NotContains(t, gritta.FindTopic("founding").Text, "Coll")
	require.Contains(t, gritta.FindTopic("finds").Text, "gutters")
	coll := loadShippedDialogue(t, "new_plymouth_common/9320.yaml")
	require.NotContains(t, menuKeys(coll, carrying(heldState("65-start"), 40101)), "sweeperhandoff")
	require.Contains(t, menuKeys(coll, carrying(heldState("65-start", "65-discover"), 40101)), "sweeperhandoff")
}
