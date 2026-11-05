package dialogue

import (
	"os"
	"slices"
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
		flags                map[string]string
	}{
		{"new_plymouth_docks/9316.yaml", "tallyhandoff", "Choose Dock Rat below", []string{"63-start"}, []string{"63-start", "63-evidence"}, 40100, nil},
		{"new_plymouth_docks/9300.yaml", "tallyhandoff", "Choose Dock Rat below", []string{"63-start"}, []string{"63-start", "63-evidence"}, 40100, nil},
		{"new_plymouth_docks/9316.yaml", "casefilehandoff", "Choose The Bloom Trail below", []string{"67-start"}, []string{"67-start", "67-witness"}, 40111, nil},
		{"new_plymouth_common/9320.yaml", "sweeperhandoff", "Choose The Street Sweeper's Secret below", []string{"65-start"}, []string{"65-start", "65-discover"}, 40101, nil},
		{"new_plymouth_common/9323.yaml", "wrapperhandoff", "Choose the line about the wrapper below", nil, []string{"66-start"}, 40110, nil},
		{"new_plymouth_docks/9392.yaml", "purge_handoff", "Choose The Addict's Plight below", []string{"66-start"}, []string{"66-start", "66-dose"}, 40109, nil},
		{"new_plymouth_crafting/9332.yaml", "orinhandoff", "Choose The Pre-Founding Web below", []string{"70-start"}, []string{"70-start", "70-witness"}, 40118, nil},
		{"new_plymouth_crafting/9332.yaml", "orinhandoff", "Choose The Pre-Founding Web below", []string{"70-start"}, []string{"70-start", "70-witness", "69-end"}, 40118, nil},
		{"new_plymouth_merchant/9347.yaml", "ledgerhandoff", "Choose The Tribute below", []string{"71-horst"}, []string{"71-horst", "71-ledger"}, 40119, nil},
		{"new_plymouth_crafting/9332.yaml", "orinhandoff_cold", "Choose The Pre-Founding Web below", []string{"70-start", "68-end"}, []string{"70-start", "70-witness", "68-end"}, 40118, map[string]string{"68-allegiance": "bloodline"}},
	} {
		df := loadShippedDialogue(t, c.file)
		state := func(held []string, has, carries bool) *PlayerState {
			ps := withFlags(heldState(held...), c.flags)
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

	// A hand-in that takes no item: the promise follows its step and what must be
	// heard first. Hearing Odile's story is what grants the step her opener reads.
	// Horst's terms have no step of their own, so his opener names the terms.
	for _, c := range []struct {
		file, topic, promise string
		onStep               []string
		openerPromises       bool
	}{
		{"new_plymouth_common/9801.yaml", "q64_run", "choose The Skip below", []string{"64-start", "64-found"}, true},
		{"new_plymouth_merchant/9344.yaml", "q64_report", "choose The Skip below", []string{"64-start", "64-found"}, false},
	} {
		df := loadShippedDialogue(t, c.file)
		_, hints := df.OpenerFor(heldState(c.onStep...))
		require.Equal(t, c.openerPromises, strings.Contains(hints, c.promise), "%s opener: %q", c.file, hints)
		heard := heldState(c.onStep...)
		heard.HasHeard = func(string) bool { return true }
		require.Contains(t, menuKeys(df, heard), c.topic, c.file)
		require.NotContains(t, menuKeys(df, heldState(c.onStep...)), c.topic, "%s: gated on hearing first", c.file)
		require.NotContains(t, menuKeys(df, heldState(c.onStep[:1]...)), c.topic, "%s: gated on the step", c.file)
	}

	// Sefton's hand-in takes no item and asks to hear nothing: going through the
	// door is the step, and it follows that step alone.
	sefton := loadShippedDialogue(t, "new_plymouth_docks/9802.yaml")
	behind := heldState("102-start", "102-behind")
	_, hints := sefton.OpenerFor(behind)
	require.Contains(t, hints, "Choose The Knock below")
	require.Contains(t, menuKeys(sefton, behind), "q102_in")
	require.Equal(t, "The Knock", sefton.FindTopic("q102_in").Label)
	for _, held := range [][]string{{"102-start"}, {"102-start", "102-behind", "102-end"}, nil} {
		ps := heldState(held...)
		_, hints = sefton.OpenerFor(ps)
		require.NotContains(t, hints, "Choose The Knock below", "%v", held)
		require.NotContains(t, menuKeys(sefton, ps), "q102_in", "%v", held)
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
		"new_plymouth_common/9320.yaml", "new_plymouth_common/9323.yaml", "new_plymouth_common/9325.yaml", "new_plymouth_common/9801.yaml",
		"new_plymouth_crafting/9332.yaml", "new_plymouth_crafting/9338.yaml",
		"new_plymouth_docks/9300.yaml", "new_plymouth_docks/9305.yaml", "new_plymouth_docks/9316.yaml", "new_plymouth_docks/9392.yaml", "new_plymouth_docks/9802.yaml",
		"new_plymouth_merchant/9344.yaml", "new_plymouth_merchant/9347.yaml", "new_plymouth_merchant/9349.yaml",
		"new_plymouth_noble/9369.yaml", "new_plymouth_noble/9371.yaml", "new_plymouth_temple/9360.yaml", "new_plymouth_old_quarter/9379.yaml", "new_plymouth_old_quarter/9381.yaml",
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

func TestTier5TheMarkIsReCutWhereNobodyLooks(t *testing.T) {
	orin := loadShippedDialogue(t, "new_plymouth_crafting/9332.yaml")
	require.Contains(t, orin.FindTopic("circle_choice").Text, "inner door")
	require.Contains(t, orin.FindTopic("q68_choice").Text, "rubbing")
	text, _ := orin.OpenerFor(withFlags(heldState("68-start", "68-evidence", "68-survey", "68-choice", "68-end"), map[string]string{"68-allegiance": "circle"}))
	require.Contains(t, text, "inner door")
	text, _ = orin.OpenerFor(carrying(heldState("68-start", "68-evidence", "68-survey"), 40112, 40113))
	require.Contains(t, text, "Asha")
	toby := loadShippedDialogue(t, "new_plymouth_crafting/9338.yaml")
	text, _ = toby.OpenerFor(withFlags(heldState("68-start", "68-evidence", "68-survey", "68-choice", "68-end"), map[string]string{"68-allegiance": "circle"}))
	require.Contains(t, text, "inner door")
}

// Orin's lines about the choice follow the survey, as Vell's do: "You have it"
// and "choose his line below" are said only to a player holding it, and the hint
// naming the hand-in names its real label.
func TestTier5OrinsChoiceFollowsTheSurvey(t *testing.T) {
	const survey = 40113
	orin := loadShippedDialogue(t, "new_plymouth_crafting/9332.yaml")
	onSurvey := []string{"68-start", "68-evidence", "68-survey"}
	onChoice := append(append([]string{}, onSurvey...), "68-choice")

	// At the survey step.
	require.Contains(t, menuKeys(orin, carrying(heldState(onSurvey...), survey)), "q68_choice")
	text, hints := orin.OpenerFor(carrying(heldState(onSurvey...), survey))
	require.Contains(t, text, "You have it")
	require.Contains(t, hints, "Choose to tell Orin")
	require.NotContains(t, menuKeys(orin, heldState(onSurvey...)), "q68_choice")
	text, hints = orin.OpenerFor(heldState(onSurvey...))
	require.NotContains(t, text, "You have")
	require.Contains(t, hints, "Type search there")

	// At the choice.
	require.Contains(t, menuKeys(orin, carrying(heldState(onChoice...), survey)), "circle_terms")
	_, hints = orin.OpenerFor(carrying(heldState(onChoice...), survey))
	require.Contains(t, hints, "choose his line below")
	require.NotContains(t, menuKeys(orin, heldState(onChoice...)), "circle_terms")
	_, hints = orin.OpenerFor(heldState(onChoice...))
	require.NotContains(t, hints, "choose his line below")
	require.Contains(t, hints, "Type search there")

	// The hand-in opens after the terms, and only for a survey a topic can take.
	heard := func(ps *PlayerState) *PlayerState {
		ps.HasHeard = func(k string) bool { return k == HeardKey(orin.MobId, "circle_terms") }
		return ps
	}
	require.Contains(t, menuKeys(orin, heard(carrying(heldState(onChoice...), survey))), "circle_choice")
	terms := orin.FindTopic("circle_terms").Hints
	require.Contains(t, terms, orin.FindTopic("circle_choice").Label, "the hint names the real line")
	require.NotContains(t, terms, "in your pack", "the survey cannot leave the pack")
}

// The choice that cannot be undone reads the same by give and by menu, for both
// sides: the first give warns, and only a player who heard the warning commits.
// The menu lists the hand-in above its warning, as every menu lists hand-ins.
func TestTier5GiveWarnsThenCommitsOnBothSides(t *testing.T) {
	const survey = 40113
	onChoice := []string{"68-start", "68-evidence", "68-survey", "68-choice"}
	for _, c := range []struct{ file, terms, commit string }{
		{"new_plymouth_crafting/9332.yaml", "circle_terms", "circle_choice"},
		{"new_plymouth_merchant/9349.yaml", "q68_vell_report", "vellhandoff"},
	} {
		df := loadShippedDialogue(t, c.file)
		unheard := carrying(heldState(onChoice...), survey)
		tp, wanted := df.ItemTopic(survey, unheard)
		require.True(t, wanted, c.file)
		require.NotNil(t, tp, c.file)
		require.Equal(t, c.terms, tp.Key, "%s: the warning comes first", c.file)
		require.NotContains(t, menuKeys(df, unheard), c.commit, c.file)

		heard := carrying(heldState(onChoice...), survey)
		heard.HasHeard = func(k string) bool { return k == HeardKey(df.MobId, c.terms) }
		tp, wanted = df.ItemTopic(survey, heard)
		require.True(t, wanted, c.file)
		require.NotNil(t, tp, c.file)
		require.Equal(t, c.commit, tp.Key, "%s: the second give commits", c.file)
		keys := menuKeys(df, heard)
		require.Contains(t, keys, c.terms, "%s: the warning can be heard again", c.file)
		require.Less(t, slices.Index(keys, c.commit), slices.Index(keys, c.terms), c.file)
	}
}

// A player at the choice who lost the survey reads why the line is not there, at
// either end of it, and where to find another. The same at the survey step at
// Toby's: "You have it" is said only to a player who does.
func TestTier5ALostSurveyIsToldSo(t *testing.T) {
	const survey = 40113
	vell := loadShippedDialogue(t, "new_plymouth_merchant/9349.yaml")
	onChoice := []string{"68-start", "68-evidence", "68-survey", "68-choice"}
	text, hints := vell.OpenerFor(heldState(onChoice...))
	require.Contains(t, text, "no document")
	require.Contains(t, hints, "Type search there")
	require.NotContains(t, menuKeys(vell, heldState(onChoice...)), "q68_vell_report")
	text, hints = vell.OpenerFor(carrying(heldState(onChoice...), survey))
	require.Equal(t, vell.Opener.Text, text, "holding it, she reads the plain office")
	require.Equal(t, vell.Opener.Hints, hints)
	text, _ = vell.OpenerFor(heldState(append(onChoice, "68-end")...))
	require.Equal(t, vell.Opener.Text, text, "the matter is closed")

	toby := loadShippedDialogue(t, "new_plymouth_crafting/9338.yaml")
	onSurvey := onChoice[:3]
	text, hints = toby.OpenerFor(heldState(onSurvey...))
	require.NotContains(t, text, "You have it")
	require.Contains(t, text, "not got the survey")
	require.Contains(t, hints, "Type search there")
	text, hints = toby.OpenerFor(carrying(heldState(onSurvey...), survey))
	require.Contains(t, text, "You have it")
	require.Contains(t, hints, "Type talk orin")
	text, _ = toby.OpenerFor(heldState(onChoice...))
	require.Contains(t, text, "not got the survey")
}

// After the bloodline ending Toby still answers, and his opener allows for it.
func TestTier5TobyAnswersAfterTheBloodlineEnding(t *testing.T) {
	toby := loadShippedDialogue(t, "new_plymouth_crafting/9338.yaml")
	text, _ := toby.OpenerFor(withFlags(heldState("68-start", "68-evidence", "68-survey", "68-choice", "68-end"), map[string]string{"68-allegiance": "bloodline"}))
	require.Contains(t, text, "Ask, if you must")
	require.NotContains(t, text, "I don't have anything to say")
}

// The copy Orin hands over and the survey he keeps cannot both be "the original
// below": what lies below is what the map charts.
func TestTier5OrinKeepsTheOriginalInHand(t *testing.T) {
	orin := loadShippedDialogue(t, "new_plymouth_crafting/9332.yaml")
	require.NotContains(t, orin.FindTopic("circle_choice").Text, "The original is below")
	require.Contains(t, orin.FindTopic("circle_choice").Text, "What it charts is under the docks")
	copyItem, err := os.ReadFile("../../_datafiles/world/dogmud/items/materials-40000/40114-copy_of_edvars_map.yaml")
	require.NoError(t, err)
	require.NotContains(t, string(copyItem), "original is below")
	require.Contains(t, string(copyItem), "What it charts is under the docks")
}

// "Mind the canal" says why, in the words a new reader needs.
func TestTier5TheCanalWarningGivesItsReason(t *testing.T) {
	gritta := loadShippedDialogue(t, "new_plymouth_old_quarter/9381.yaml")
	hint := gritta.FindTopic("q70_start").Hints
	require.Contains(t, hint, "Something waits in the water")
	require.NotContains(t, hint, "Mind the canal")
	require.LessOrEqual(t, len(hint), 200)
	require.Contains(t, hint, "Bring a light")
	require.Contains(t, hint, "Type search there")
}

// Ostry is worried, in words a new reader knows.
func TestTier5OstrysRootHintIsPlain(t *testing.T) {
	o := loadShippedDialogue(t, "new_plymouth_merchant/9347.yaml")
	require.Contains(t, o.Opener.Hints, "She seems worried about")
	require.NotContains(t, o.Opener.Hints, "tight about")
}

// A step's hint says what to do where the player stands. The quest log's guide
// line gives the way, and a sleeping Toby is read from the talk refusal.
func TestTier5CooperageHintsGiveNoRouteAndAskForASearch(t *testing.T) {
	for _, file := range []string{"new_plymouth_crafting/9332.yaml", "new_plymouth_crafting/9338.yaml"} {
		df := loadShippedDialogue(t, file)
		hints := []string{df.Opener.Hints}
		for _, v := range df.Opener.Variants {
			hints = append(hints, v.Hints)
		}
		for _, tp := range df.Topics {
			hints = append(hints, tp.Hints)
		}
		for _, h := range hints {
			for _, banned := range []string{"twice", "three times", "by day", "look floorboard"} {
				require.NotContains(t, h, banned, file)
			}
		}
	}
}

// Dross takes Lysha's reading and keeps the rubbing with it, so he asks for both
// papers: the reading is handed over, the rubbing only has to be carried.
func TestTier5DrossNeedsBothPapers(t *testing.T) {
	d := loadShippedDialogue(t, "new_plymouth_temple/9360.yaml")
	held := []string{"69-start", "69-rubbing", "69-gallery", "69-reading"}
	require.Contains(t, menuKeys(d, carrying(heldState(held...), 40115, 40116)), "give_40116")
	require.NotContains(t, menuKeys(d, carrying(heldState(held...), 40116)), "give_40116", "no rubbing")
	require.NotContains(t, menuKeys(d, carrying(heldState("69-start", "69-rubbing", "69-gallery"), 40115, 40116)), "give_40116", "the step before")
	text, _ := d.OpenerFor(carrying(heldState(held...), 40116))
	require.Contains(t, text, "where is the rubbing")
	require.NotContains(t, d.FindTopic("q69_gallery").Text, "Get past him")
}

// The give command and the menu agree. A rubbing alone, or a reading alone, is
// handed back; both open the hand-in; the opener promises the hand-in only then,
// and says how to get another rubbing when it is the rubbing that is missing.
func TestTier5DrossGiveAndMenuAgree(t *testing.T) {
	d := loadShippedDialogue(t, "new_plymouth_temple/9360.yaml")
	held := []string{"69-start", "69-rubbing", "69-gallery", "69-reading"}
	const promise = "Choose The Gallery Cipher below"

	both := carrying(heldState(held...), 40115, 40116)
	tp, wanted := d.ItemTopic(40116, both)
	require.True(t, wanted)
	require.NotNil(t, tp)
	require.Equal(t, "give_40116", tp.Key)
	_, hints := d.OpenerFor(both)
	require.Contains(t, hints, promise)

	reading := carrying(heldState(held...), 40116)
	tp, wanted = d.ItemTopic(40116, reading)
	require.True(t, wanted, "give is refused, not ignored")
	require.Nil(t, tp)
	_, hints = d.OpenerFor(reading)
	require.NotContains(t, hints, promise)
	require.Contains(t, hints, "Type search there")

	rubbing := carrying(heldState(held...), 40115)
	tp, _ = d.ItemTopic(40115, rubbing)
	require.Nil(t, tp, "the rubbing alone opens nothing at this step")
	_, hints = d.OpenerFor(rubbing)
	require.NotContains(t, hints, promise)

	// A reading where a topic cannot take it (carried, not in reach): no
	// promise, no topic.
	pouch := heldState(held...)
	pouch.HasItem = func(id int) bool { return id == 40115 }
	pouch.CarriesItem = func(id int) bool { return id == 40115 || id == 40116 }
	require.NotContains(t, menuKeys(d, pouch), "give_40116")
	_, hints = d.OpenerFor(pouch)
	require.NotContains(t, hints, promise)

	// Applying the hand-in takes the reading, leaves the rubbing to the quest's
	// own trigger, and grants the end once.
	var removed []int
	var granted []string
	both.RemoveItem = func(id int) bool { removed = append(removed, id); return true }
	both.GiveQuest = func(tok string) { granted = append(granted, tok) }
	require.True(t, d.Apply(d.FindTopic("give_40116"), both))
	require.Equal(t, []int{40116}, removed, "the dialogue takes the reading only")
	require.Equal(t, []string{"69-end"}, granted)

	// After the end the hand-in is gone, with or without the papers.
	require.NotContains(t, menuKeys(d, carrying(heldState(append(held, "69-end")...), 40115, 40116)), "give_40116")
}

// Lysha's hint for a lost rubbing sends the player to a search.
func TestTier5LyshaAsksForASearch(t *testing.T) {
	l := loadShippedDialogue(t, "new_plymouth_noble/9371.yaml")
	_, hints := l.OpenerFor(heldState("69-start", "69-rubbing", "69-gallery"))
	require.Contains(t, hints, "Type search there")
	require.NotContains(t, hints, "look reliefs")
}

// Orin remembers the 68 ending and nothing else changes the hand-in: every other
// player gets the warm line, and the bloodline's gets the cold one and no circle
// reputation. Exactly one is open for each, by the menu and by give alike.
func TestTier5OrinRemembersTheCooperage(t *testing.T) {
	const rubbing = 40118
	orin := loadShippedDialogue(t, "new_plymouth_crafting/9332.yaml")
	const (
		warm = "orinhandoff"
		cold = "orinhandoff_cold"
	)
	cooperage := []string{"68-start", "68-evidence", "68-survey", "68-choice", "68-end"}
	for _, c := range []struct {
		name    string
		held    []string
		flags   map[string]string
		topic   string
		other   string
		circle  int
		opening string
	}{
		{"no quest 68 at all", nil, nil, warm, cold, 5, "What have you brought me up from below?"},
		{"68 just started", []string{"68-start"}, nil, warm, cold, 5, "What have you brought me up from below?"},
		{"68 at the choice", cooperage[:4], nil, warm, cold, 5, "What have you brought me up from below?"},
		{"the circle ending", cooperage, map[string]string{"68-allegiance": "circle"}, warm, cold, 5, "Toby's mark holds, and now this. What have you brought me up from below?"},
		{"68 ended on an old save with no flag", cooperage, nil, warm, cold, 5, "What have you brought me up from below?"},
		{"the bloodline ending", cooperage, map[string]string{"68-allegiance": "bloodline"}, cold, warm, 0, "put it on the counter and go"},
	} {
		held := append([]string{"70-start", "70-witness"}, c.held...)
		ps := withFlags(carrying(heldState(held...), rubbing), c.flags)

		keys := menuKeys(orin, ps)
		require.Contains(t, keys, c.topic, c.name)
		require.NotContains(t, keys, c.other, c.name)
		text, hints := orin.OpenerFor(ps)
		require.Contains(t, text, c.opening, c.name)
		require.Contains(t, hints, "Choose The Pre-Founding Web below", c.name)

		// give rubbing orin plays the same topic.
		tp, wanted := orin.ItemTopic(rubbing, ps)
		require.True(t, wanted, c.name)
		require.NotNil(t, tp, c.name)
		require.Equal(t, c.topic, tp.Key, c.name)

		rep := map[string]int{}
		var removed []int
		var granted []string
		ps.BumpRep = func(f string, d int) { rep[f] += d }
		ps.RemoveItem = func(id int) bool { removed = append(removed, id); return true }
		ps.GiveQuest = func(tok string) { granted = append(granted, tok) }
		require.True(t, orin.Apply(tp, ps), c.name)
		require.Equal(t, c.circle, rep["cooperage_circle"], c.name)
		require.Equal(t, []int{rubbing}, removed, c.name)
		require.Equal(t, []string{"70-end"}, granted, c.name)

		// No rubbing, or the quest over: neither hand-in is open.
		none := withFlags(heldState(held...), c.flags)
		require.NotContains(t, menuKeys(orin, none), warm, c.name)
		require.NotContains(t, menuKeys(orin, none), cold, c.name)
		over := withFlags(carrying(heldState(append(held, "70-end")...), rubbing), c.flags)
		require.NotContains(t, menuKeys(orin, over), warm, c.name)
		require.NotContains(t, menuKeys(orin, over), cold, c.name)
	}
	require.NotContains(t, orin.FindTopic(cold).Text, "glad")
}

// A lost rubbing is told where the next comes from, and not told to choose a
// hand-in that is not on the menu. This comes before Orin's lines about 68, for a
// player at any point in it.
func TestTier5OrinOnALostRubbing(t *testing.T) {
	orin := loadShippedDialogue(t, "new_plymouth_crafting/9332.yaml")
	cooperage := []string{"68-start", "68-evidence", "68-survey", "68-choice", "68-end"}
	for _, flags := range []map[string]string{nil, {"68-allegiance": "circle"}, {"68-allegiance": "bloodline"}} {
		held := []string{"70-start", "70-witness"}
		if flags != nil {
			held = append(held, cooperage...)
		}
		lost := withFlags(heldState(held...), flags)
		text, hints := orin.OpenerFor(lost)
		require.Contains(t, text, "brought nothing up")
		require.Contains(t, hints, "Type search there")
		require.Contains(t, hints, "Bring a light")
		require.NotContains(t, hints, "Choose The Pre-Founding Web below")
	}
}

// Gritta and Orin nod toward 68 and 69 (owner ruling): for a player who did them,
// a line of their own, and for one who did not, a mention they can pass over. No
// nod names a quest, gives a direction or tells the player to do anything.
func TestTier5QuietNods(t *testing.T) {
	gritta := loadShippedDialogue(t, "new_plymouth_old_quarter/9381.yaml")
	orin := loadShippedDialogue(t, "new_plymouth_crafting/9332.yaml")
	circle := map[string]string{"68-allegiance": "circle"}
	bloodline := map[string]string{"68-allegiance": "bloodline"}

	templeNod, hints := gritta.OpenerFor(heldState("69-end"))
	require.Contains(t, templeNod, "temple wall")
	require.Equal(t, gritta.Opener.Hints, hints, "the nod keeps her offer's trail")
	cooperageNod, hints := gritta.OpenerFor(withFlags(heldState("68-end"), circle))
	require.Contains(t, cooperageNod, "cooperage")
	require.Equal(t, gritta.Opener.Hints, hints)

	root, _ := gritta.OpenerFor(heldState())
	bloody, _ := gritta.OpenerFor(withFlags(heldState("68-end"), bloodline))
	require.Equal(t, root, bloody, "no nod to the one who sold them out")
	both, _ := gritta.OpenerFor(withFlags(heldState("69-end", "68-end"), bloodline))
	require.Equal(t, templeNod, both, "the temple is still theirs")

	// Once the quest is under way the quest's own lines lead.
	for _, step := range [][]string{{"70-start"}, {"70-start", "70-witness"}, {"70-start", "70-witness", "70-end"}} {
		text, _ := gritta.OpenerFor(heldState(append(step, "69-end")...))
		require.NotEqual(t, templeNod, text)
	}

	onHandIn := func(held ...string) *PlayerState {
		return carrying(heldState(append([]string{"70-start", "70-witness"}, held...)...), 40118)
	}
	orinNod, _ := orin.OpenerFor(onHandIn("69-end"))
	require.Contains(t, orinNod, "temple")
	coldNod, _ := orin.OpenerFor(withFlags(onHandIn("69-end", "68-end"), bloodline))
	require.NotContains(t, coldNod, "temple", "the cold opener wins over the temple")
	circleNod, _ := orin.OpenerFor(withFlags(onHandIn("68-end"), circle))
	require.Contains(t, circleNod, "Toby's mark holds", "he remembers who stood with it")
	circleAndTemple, _ := orin.OpenerFor(withFlags(onHandIn("69-end", "68-end"), circle))
	require.Equal(t, circleNod, circleAndTemple, "the cooperage is the closer memory")
	circleLost, _ := orin.OpenerFor(withFlags(heldState("70-start", "70-witness", "68-end"), circle))
	require.NotContains(t, circleLost, "charcoal", "no rubbing, no charcoal")

	// The mention for a player who did neither, heard by every warm hand-in and
	// after the quest at Gritta's. The cold hand-in has no such mention.
	warmHandIn := orin.FindTopic("orinhandoff").Text
	require.Contains(t, warmHandIn, "temple")
	require.Contains(t, warmHandIn, "Edvar")
	after, _ := gritta.OpenerFor(heldState("70-start", "70-witness", "70-end"))
	require.Contains(t, after, "temple wall keeps it too")
	require.Contains(t, after, "shuttered shop")
	require.NotContains(t, orin.FindTopic("orinhandoff_cold").Text, "temple")

	// The one who sold the cooperage out hears no praise of a shuttered shop.
	soldOut, _ := gritta.OpenerFor(withFlags(heldState("70-start", "70-witness", "70-end", "68-end"), bloodline))
	require.True(t, strings.HasSuffix(soldOut, "That was the point."), soldOut)
	require.NotContains(t, soldOut, "shuttered")
	standBy, _ := gritta.OpenerFor(withFlags(heldState("70-start", "70-witness", "70-end", "68-end"), circle))
	require.Equal(t, after, standBy, "the circle's player hears the whole line")

	for _, line := range []string{templeNod, cooperageNod, orinNod, circleNod, warmHandIn, after, soldOut} {
		lower := strings.ToLower(line)
		for _, banned := range []string{"gallery cipher", "cooperage circle", "pre-founding web", "type ", "north", "south", " east", "west", "go to", "go and", "you should"} {
			require.NotContains(t, lower, banned, line)
		}
	}
}

// Gritta sends her finds to Orin. Coll picks small pieces out of the gutters and
// has never met her (TestTier5GrittaDoesNotSendColl), so her schedule agrees.
func TestTier5GrittasScheduleSendsNothingToColl(t *testing.T) {
	data, err := os.ReadFile("../../_datafiles/world/dogmud/schedules/new_plymouth_old_quarter/np_oldquarter_gritta.yaml")
	require.NoError(t, err)
	require.Contains(t, string(data), "bookseller", "the file is the one that names her finds")
	require.NotContains(t, string(data), "Coll")
	require.NotContains(t, string(data), "sweeper")
}

// Ostry's hand-in pays no circle reputation: the quest's reward pays the common
// folk, and the 69-end trigger keeps its own bonus.
func TestTier5OstryPaysNoCircleRep(t *testing.T) {
	o := loadShippedDialogue(t, "new_plymouth_merchant/9347.yaml")
	in := o.FindTopic("ledgerhandoff")
	require.Empty(t, in.BumpsRep, "the 69-end trigger keeps its bonus, the hand-in pays none")
	require.Equal(t, []string{"71-ledger"}, in.QuestRequired)
	require.Equal(t, 40119, in.RequiresItem)
	require.Equal(t, "The Tribute", in.Label, "the openers say Choose The Tribute below")
}

// Each Ostry opener says only what is true for the step: the hand-in when the
// page can be taken, a search for a lost one, and a search at the parlor before
// the page is found. Every hint names a search, never a typed noun.
func TestTier5OstryOpenersFollowTheLedger(t *testing.T) {
	o := loadShippedDialogue(t, "new_plymouth_merchant/9347.yaml")
	steps := []string{"71-start", "71-vell", "71-horst", "71-ledger"}
	_, hints := o.OpenerFor(carrying(heldState(steps...), 40119))
	require.Contains(t, hints, "Choose The Tribute below")
	_, hints = o.OpenerFor(heldState(steps...))
	require.Contains(t, hints, "Type search there", "a lost page is searched for again")
	require.NotContains(t, hints, "Choose The Tribute below")
	_, hints = o.OpenerFor(heldState(steps[:3]...))
	require.Contains(t, hints, "Type search there")
	for _, tok := range [][]string{steps[:1], steps[:2], steps[:3]} {
		_, hints = o.OpenerFor(carrying(heldState(tok...), 40119))
		require.NotContains(t, hints, "Choose The Tribute below", "%v", tok)
	}
	all := []string{o.Opener.Hints}
	for _, v := range o.Opener.Variants {
		all = append(all, v.Hints)
	}
	for _, tp := range o.Topics {
		all = append(all, tp.Hints)
	}
	for _, h := range all {
		require.NotContains(t, h, "look paper scrap")
	}
	h := loadShippedDialogue(t, "new_plymouth_merchant/9344.yaml").FindTopic("q71_horst")
	require.Contains(t, h.Hints, "Type search there")
	require.NotContains(t, h.Hints, "look paper scrap")
}

// No Dross line sends the player to look at the reliefs.
func TestTier5DrossNeverSaysLookReliefs(t *testing.T) {
	d := loadShippedDialogue(t, "new_plymouth_temple/9360.yaml")
	all := []string{d.Opener.Hints}
	for _, v := range d.Opener.Variants {
		all = append(all, v.Hints)
	}
	for _, tp := range d.Topics {
		all = append(all, tp.Hints)
	}
	for _, h := range all {
		require.NotContains(t, h, "look reliefs")
	}
}

func TestTier5NewcomerTopicsReadTheFlag(t *testing.T) {
	newFlag := map[string]string{"30-newcomer": "new"}

	toke := loadShippedDialogue(t, "pothole_coulee/9106.yaml")
	require.Contains(t, menuKeys(toke, withFlags(heldState(), newFlag)), "newcomer_players")
	require.NotContains(t, menuKeys(toke, heldState()), "newcomer_players")
	require.Nil(t, toke.FindTopic("newcomer_players").SetsQuestFlag, "hearing Toke must not close Hadwen's lesson")
	require.Empty(t, toke.FindTopic("newcomer_players").GrantsQuest)

	hadwen := loadShippedDialogue(t, "pothole_coulee/9100.yaml")
	require.Contains(t, menuKeys(hadwen, withFlags(heldState("30-end"), newFlag)), "newcomer_mutations")
	require.NotContains(t, menuKeys(hadwen, withFlags(heldState(), newFlag)), "newcomer_mutations")
	require.NotContains(t, menuKeys(hadwen, heldState("30-end")), "newcomer_mutations")
}
