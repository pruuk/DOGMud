package dialogue

import (
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
	require.Equal(t, "If Teels is waiting on it, take it to her. To use it yourself, type quaff purge.", y.FindTopic("detox").Hints)
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
