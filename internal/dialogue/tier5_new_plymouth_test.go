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
		"new_plymouth_crafting/9332.yaml", "new_plymouth_crafting/9338.yaml",
		"new_plymouth_docks/9300.yaml", "new_plymouth_docks/9305.yaml", "new_plymouth_docks/9316.yaml", "new_plymouth_docks/9392.yaml",
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
// naming the hand-in names its real label and says where the survey must be.
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
	inABag := heard(heldState(onChoice...))
	inABag.CarriesItem = func(id int) bool { return id == survey }
	require.Contains(t, menuKeys(orin, inABag), "circle_terms")
	require.NotContains(t, menuKeys(orin, inABag), "circle_choice")
	terms := orin.FindTopic("circle_terms").Hints
	require.Contains(t, terms, orin.FindTopic("circle_choice").Label, "the hint names the real line")
	require.Contains(t, terms, "in your pack", "the hint says what opens the line")
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
