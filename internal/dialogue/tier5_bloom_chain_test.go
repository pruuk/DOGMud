package dialogue

import (
	"os"
	"strings"
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

// A player who found her, then shelved the quest, is not told to go and find her.
// The flag that tells them apart is set by her own story.
func TestTier5HorstResumeReadsTrueToAPlayerWhoFoundHer(t *testing.T) {
	h := loadShippedDialogue(t, "new_plymouth_merchant/9344.yaml")
	o := loadShippedDialogue(t, "new_plymouth_common/9801.yaml")
	require.Equal(t, "64-seen", o.FindTopic("q64_found").SetsQuestFlag.Key)

	flags := map[string]string{"67-entry": "undercover"}
	fresh := menuKeys(h, withFlags(heldState(), flags))
	require.Contains(t, fresh, "q64_offer")
	require.NotContains(t, fresh, "q64_resume")

	flags["64-seen"] = "found"
	found := menuKeys(h, withFlags(heldState(), flags))
	require.NotContains(t, found, "q64_offer")
	require.Contains(t, found, "q64_resume")

	resume := h.FindTopic("q64_resume")
	for _, line := range []string{resume.Text, resume.Hints, resume.Objective} {
		require.NotContains(t, line, "Find her")
		require.NotContains(t, line, "Common Quarter")
	}
	require.Contains(t, h.FindTopic("q64_offer").Text, "Find her", "the first offer still says it")
}

// Horst's openers: he never names a floor the player did not name, stops sending a
// customer to Marn, and the office is unbothered by the bust.
func TestTier5HorstOpenersReadTrue(t *testing.T) {
	h := loadShippedDialogue(t, "new_plymouth_merchant/9344.yaml")
	reported := map[string]string{"64-outcome": "reported"}

	ran, _ := h.OpenerFor(withFlags(heldState("64-start", "64-found", "64-end"), map[string]string{"64-outcome": "ran"}))
	require.NotContains(t, ran, "floor", "the player never told him which floor")
	require.Contains(t, ran, "You were slow, or you were something else.")

	_, hints := h.OpenerFor(withFlags(heldState("64-start", "64-found", "64-end"), reported))
	require.Contains(t, hints, "Marn", "not yet a customer")

	_, hints = h.OpenerFor(withFlags(heldState("64-start", "64-found", "64-end", "102-start"), reported))
	require.NotContains(t, hints, "Marn", "already paid him")

	bust, _ := h.OpenerFor(heldState("67-start", "67-end"))
	require.Contains(t, bust, "not troubled")
	plain, _ := h.OpenerFor(heldState())
	require.NotContains(t, plain, "not troubled", "no bust, nothing to mention")
}

// The stake is named while the offer is hidden, and it survives the bust.
func TestTier5TheKnockCosts(t *testing.T) {
	m := loadShippedDialogue(t, "new_plymouth_docks/9305.yaml")
	offer := m.FindTopic("q102_offer")
	require.Equal(t, 500, offer.ChargesGold)
	require.Equal(t, 500, offer.GoldRequired)
	require.Contains(t, offer.Text, "Five hundred gold")
	require.Contains(t, offer.Objective, "five hundred gold")
	require.Contains(t, offer.Hints, "takes the gold now")
	require.NotContains(t, offer.Text, "Three short", "the knock is taught on Accept, after the gold")

	state := func(flag string, gold bool, held ...string) *PlayerState {
		ps := withFlags(heldState(held...), map[string]string{"64-outcome": flag})
		ps.HasGold = func(int) bool { return gold }
		return ps
	}
	require.NotContains(t, menuKeys(m, state("", true)), "q102_offer", "no skip, no knock")
	require.NotContains(t, menuKeys(m, state("ran", true)), "q102_offer", "she ran, the chain closes")
	require.NotContains(t, menuKeys(m, state("reported", false)), "q102_offer", "the price in hand")
	require.Contains(t, menuKeys(m, state("reported", true)), "q102_offer")
	require.Contains(t, menuKeys(m, state("reported", true, "67-start", "67-end")), "q102_offer", "survives the bust")
	require.NotContains(t, menuKeys(m, state("reported", true, "102-start")), "q102_offer")

	text, _ := m.OpenerFor(state("reported", false))
	require.Contains(t, text, "Five hundred gold", "the price is named while the offer is hidden")
	plain, _ := m.OpenerFor(state("", true))
	require.NotContains(t, plain, "Five hundred")
	ran, _ := m.OpenerFor(state("ran", true))
	require.NotContains(t, ran, "Five hundred")
	bust, _ := m.OpenerFor(state("reported", false, "67-start", "67-end"))
	require.Contains(t, bust, "Five hundred gold", "the price is named after the bust too")
	require.Contains(t, bust, "Lintel Street")

	s := loadShippedDialogue(t, "new_plymouth_docks/9802.yaml")
	in := s.FindTopic("q102_in")
	require.Equal(t, []string{"102-behind"}, in.QuestRequired)
	require.Contains(t, in.Text, "never on that paper", "this cell survives quest 67")
	require.Contains(t, s.FindTopic("what_it_does").Hints, "Ysolde")
}

// A payer who shelved the quest reads what is true for them, gold or none: no
// price, no "accepting takes the gold", and a free line that really is listed.
func TestTier5TheKnockShelvedPayerReadsTrue(t *testing.T) {
	m := loadShippedDialogue(t, "new_plymouth_docks/9305.yaml")
	for _, gold := range []bool{true, false} {
		ps := withFlags(heldState(), map[string]string{"64-outcome": "reported", "102-stake": "paid"})
		ps.HasGold = func(int) bool { return gold }
		keys := menuKeys(m, ps)
		require.Contains(t, keys, "q102_resume")
		require.NotContains(t, keys, "q102_offer", "the gold is taken once")
		text, hints := m.OpenerFor(ps)
		require.NotContains(t, text, "Five hundred")
		require.NotContains(t, hints, "five hundred")
		require.Contains(t, hints, "Choose "+m.FindTopic("q102_resume").Label+" below")
	}

	resume := m.FindTopic("q102_resume")
	require.Equal(t, "The Knock", resume.Label)
	require.Zero(t, resume.ChargesGold)
	require.Zero(t, resume.GoldRequired)
	require.NotContains(t, resume.Hints, "takes the gold")
	require.Equal(t, "102-start", resume.GrantsQuest)
	require.NotContains(t, menuKeys(m, withFlags(heldState(), map[string]string{"64-outcome": "reported"})), "q102_resume", "no stake paid, no free line")

	// The one offer setting the flag is the one taking the gold.
	offer := m.FindTopic("q102_offer")
	require.Equal(t, "102-stake", offer.SetsQuestFlag.Key)
	require.Equal(t, "paid", offer.SetsQuestFlag.Value)

	// Whatever a holder reads, no hint promises a line that is not there.
	for _, held := range [][]string{{"102-start"}, {"102-start", "102-behind"}, {"102-start", "102-behind", "102-end"}} {
		ps := withFlags(heldState(held...), map[string]string{"64-outcome": "reported", "102-stake": "paid"})
		_, hints := m.OpenerFor(ps)
		require.NotContains(t, hints, "The Knock", "%v", held)
		require.NotContains(t, menuKeys(m, ps), "q102_resume", "%v", held)
		require.NotContains(t, menuKeys(m, ps), "q102_offer", "%v", held)
	}

	// The price's hint names the real line, and it is there once the gold is.
	rich := withFlags(heldState(), map[string]string{"64-outcome": "reported"})
	rich.HasGold = func(int) bool { return true }
	_, hints := m.OpenerFor(rich)
	require.Contains(t, hints, "choose "+m.FindTopic("q102_offer").Label+" below")
	require.Contains(t, hints, "five hundred gold")
	require.Contains(t, menuKeys(m, rich), "q102_offer")
}

// "Storage" is Marn's lie to a stranger. He does not tell it to someone he is
// selling the knock to.
func TestTier5MarnStopsLyingToACustomer(t *testing.T) {
	m := loadShippedDialogue(t, "new_plymouth_docks/9305.yaml")
	require.Contains(t, menuKeys(m, withFlags(heldState(), map[string]string{})), "backroom")
	require.Contains(t, menuKeys(m, withFlags(heldState(), map[string]string{"64-outcome": "ran"})), "backroom")
	require.NotContains(t, menuKeys(m, withFlags(heldState(), map[string]string{"64-outcome": "reported"})), "backroom")
}

// A holder of the knock meets the holding line after the bust, in Sefton's voice,
// and this room is the separate thing. It waits for the bust, and it never gets
// in front of the hand-in. It names no quest and no number.
func TestTier5SeftonOutlastsTheBust(t *testing.T) {
	s := loadShippedDialogue(t, "new_plymouth_docks/9802.yaml")
	holder := []string{"102-start", "102-behind", "102-end"}

	before, _ := s.OpenerFor(heldState(holder...))
	require.NotContains(t, before, "Lintel Street")

	after, _ := s.OpenerFor(heldState(append(holder, "67-start", "67-end")...))
	require.Contains(t, after, "Lintel Street")
	require.Contains(t, after, "never on that paper")
	require.NotContains(t, strings.ToLower(after), "bloom trail")

	_, hints := s.OpenerFor(heldState("102-start", "102-behind", "67-start", "67-end"))
	require.Contains(t, hints, "Choose The Knock below", "the hand-in comes first")
	require.Contains(t, menuKeys(s, heldState("102-start", "102-behind", "67-start", "67-end")), "q102_in")
	_, hints = s.OpenerFor(heldState(holder...))
	require.NotContains(t, hints, "The Knock")
	require.NotContains(t, menuKeys(s, heldState(holder...)), "q102_in")
}

// A player reaches the Bloom only by deliberate choice, and the cure stays open
// to everyone. So Sefton says plainly what it does, with no numbers, and the hint
// says where the cure is, as quest 66's does, without walking the player there: a
// playtest seat with a craving searched about twenty rooms for her. Ysolde's cup
// asks for no quest.
func TestTier5SeftonWarnsPlainly(t *testing.T) {
	s := loadShippedDialogue(t, "new_plymouth_docks/9802.yaml")
	warn := s.FindTopic("what_it_does")
	require.True(t, warn.Marked)
	require.Empty(t, warn.QuestRequired, "the warning is there for anyone who can reach him")
	require.Empty(t, warn.QuestFlagRequired)
	require.Contains(t, warn.Text, "you will need them")
	require.Regexp(t, `^[^0-9]*$`, warn.Text+warn.Hints)
	require.Contains(t, warn.Hints, "Type talk ysolde")
	lower := strings.ToLower(warn.Hints)
	require.Contains(t, lower, "common quarter")
	for _, banned := range []string{"north", "south", " east", " west", "temple", "up ", "down "} {
		require.NotContains(t, lower, banned)
	}
	require.Contains(t, menuKeys(s, heldState()), "what_it_does")

	require.Contains(t, s.FindTopic("q102_in").Hints, "type buy wafer")
	require.Contains(t, s.FindTopic("q102_in").Hints, "Ask him what it does")
	require.Contains(t, s.Opener.Hints, "type list")

	y := loadShippedDialogue(t, "new_plymouth_common/9323.yaml")
	detox := y.FindTopic("detox")
	require.Empty(t, detox.QuestRequired)
	require.Empty(t, detox.QuestFlagRequired)
	require.Contains(t, menuKeys(y, heldState()), "detox", "the cure is open to everyone")

	// Sefton does say what it costs later, so the wafer cannot say no seller will.
	data, err := os.ReadFile("../../_datafiles/world/dogmud/items/materials-40000/40108-bloom_wafer.yaml")
	require.NoError(t, err)
	require.NotContains(t, string(data), "Nobody who sells it")
}
