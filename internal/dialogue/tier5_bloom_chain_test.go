package dialogue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTier5HorstHiresTheUndercoverOnly(t *testing.T) {
	h := loadShippedDialogue(t, "new_plymouth_merchant/9344.yaml")
	require.NotContains(t, menuKeys(h, heldState()), "q64_offer")
	require.NotContains(t, menuKeys(h, withFlags(heldState("67-start"), map[string]string{"67-entry": "evidence"})), "q64_offer")
	require.Contains(t, menuKeys(h, withFlags(heldState("67-start"), map[string]string{"67-entry": "undercover"})), "q64_offer")
	require.Empty(t, h.Breadcrumbs(heldState()), "an honest player hears nothing of this chain")
	shelved := withFlags(heldState(), map[string]string{"67-entry": "undercover"})
	require.Contains(t, menuKeys(h, shelved), "q64_offer", "a player who shelved 67 keeps the shady line")
	finished := withFlags(heldState("67-start", "67-end"), map[string]string{"67-entry": "undercover"})
	require.Contains(t, menuKeys(h, finished), "q64_offer", "the shady line outlasts the trail")
}

func TestTier5TheSkipHasTwoEndings(t *testing.T) {
	h := loadShippedDialogue(t, "new_plymouth_merchant/9344.yaml")
	o := loadShippedDialogue(t, "new_plymouth_common/9801.yaml")
	report, run := h.FindTopic("q64_report"), o.FindTopic("q64_run")
	for _, tp := range []*Topic{report, run} {
		require.Equal(t, KindHandIn, tp.Kind)
		require.Equal(t, "64-end", tp.GrantsQuest)
		require.Equal(t, []string{"64-found"}, tp.QuestRequired)
	}
	require.Equal(t, "reported", report.SetsQuestFlag.Value)
	require.Equal(t, 150, report.GivesGold)
	require.Equal(t, 40108, report.GivesItem)
	require.Equal(t, "ran", run.SetsQuestFlag.Value)
	require.Zero(t, run.GivesGold)
	require.Zero(t, run.GivesItem)

	found := heldState("64-start", "64-found")
	require.NotContains(t, menuKeys(h, found), "q64_report", "hear the terms first")
	require.NotContains(t, menuKeys(o, found), "q64_run", "hear her first")
	heard := heldState("64-start", "64-found")
	heard.HasHeard = func(string) bool { return true }
	require.Contains(t, menuKeys(h, heard), "q64_report")
	require.Contains(t, menuKeys(o, heard), "q64_run")
	ended := heldState("64-start", "64-found", "64-end")
	ended.HasHeard = func(string) bool { return true }
	require.NotContains(t, menuKeys(o, ended), "q64_run")
	require.NotContains(t, menuKeys(h, ended), "q64_report")
	require.NotContains(t, menuKeys(h, withFlags(ended, map[string]string{"67-entry": "undercover"})), "q64_offer")
}

func TestTier5MarnLetsItSlipOnlyToACustomer(t *testing.T) {
	m := loadShippedDialogue(t, "new_plymouth_docks/9305.yaml")
	require.Contains(t, m.FindTopic("bloom_undercover").Hints, "Horst")
	require.NotContains(t, m.FindTopic("bloom_evidence").Hints, "Horst")
}

// The Skip's openers and hints name lines the player can choose at that moment.
// Odile's hand-in needs her story heard, which is what grants the step her opener
// reads. Horst's needs his terms heard, which no opener variant can read, so his
// opener sends the player to the terms and only they promise the hand-in.
func TestTier5TheSkipPromisesOnlyWhatIsOnTheMenu(t *testing.T) {
	heardAll := func(ps *PlayerState) *PlayerState {
		ps.HasHeard = func(string) bool { return true }
		return ps
	}

	o := loadShippedDialogue(t, "new_plymouth_common/9801.yaml")
	_, hints := o.OpenerFor(heldState("64-start", "64-found"))
	require.Contains(t, hints, "choose The Skip below")
	require.Contains(t, menuKeys(o, heardAll(heldState("64-start", "64-found"))), "q64_run")
	require.Equal(t, "The Skip", o.FindTopic("q64_run").Label)
	require.Contains(t, o.FindTopic("q64_found").Hints, "choose The Skip below", "hearing her opens the line")
	_, hints = o.OpenerFor(heldState("64-start"))
	require.NotContains(t, hints, "The Skip", "her line waits for her story")
	require.NotContains(t, menuKeys(o, heldState("64-start")), "q64_run")

	h := loadShippedDialogue(t, "new_plymouth_merchant/9344.yaml")
	terms := h.FindTopic("q64_terms")
	_, hints = h.OpenerFor(heldState("64-start", "64-found"))
	require.Contains(t, hints, "Choose "+terms.Label+" below", "the opener names the line on the menu")
	require.Contains(t, menuKeys(h, heldState("64-start", "64-found")), "q64_terms")
	require.NotContains(t, menuKeys(h, heldState("64-start", "64-found")), "q64_report")
	require.NotContains(t, hints, "choose The Skip below", "not yet on the menu")
	require.Contains(t, terms.Hints, "choose The Skip below")
	require.Contains(t, menuKeys(h, heardAll(heldState("64-start", "64-found"))), "q64_report", "hearing the terms opens it")
	require.Equal(t, "The Skip", h.FindTopic("q64_report").Label)
}
