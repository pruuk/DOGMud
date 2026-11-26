package dialogue

import (
	"os"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func loadShippedDialogue(t *testing.T, path string) *DialogueFile {
	t.Helper()
	data, err := os.ReadFile("../../_datafiles/world/dogmud/dialogue/" + path)
	require.NoError(t, err)
	var df DialogueFile
	require.NoError(t, yaml.Unmarshal(data, &df))
	return &df
}

// seedShippedQuest11 seeds the shipped quest 11 file, so a real character
// resolves its branched path the way the game does.
func seedShippedQuest11(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile("../../_datafiles/world/dogmud/quests/11-the_windwardens_dilemma.yaml")
	require.NoError(t, err)
	var q quests.Quest
	require.NoError(t, yaml.Unmarshal(data, &q))
	t.Cleanup(quests.SeedQuestsForTest(&q))
}

// quest11State is a player on quest 11's step, built from a real character:
// HasQuest and GetQuestFlag read the character, so what the player holds is
// what the engine says they hold. side is the 11-branch flag ("" for none).
func quest11State(step, side string, carrying bool) *PlayerState {
	c := &characters.Character{
		QuestProgress: map[int]string{11: step},
		QuestFlags:    map[string]string{},
	}
	if side != "" {
		c.QuestFlags["11-branch"] = side
	}
	ps := heldState()
	ps.HasQuest = c.HasQuest
	ps.GetQuestFlag = c.GetQuestFlag
	ps.HasHeard = func(string) bool { return true }
	ps.HasItem = func(int) bool { return carrying }
	ps.CarriesItem = ps.HasItem
	return ps
}

// #613: a player who takes quest 11 from Kael first holds 11-start with no
// side. Each side's menu must still lead to the choice: the terms topic opens,
// and hearing it opens the stand topic. Once the side is chosen, that side's
// hand-in opens at its return step and the other side's topics stay closed.
func TestQuest11CanBeFinishedAfterTalkingToKaelFirst(t *testing.T) {
	seedShippedQuest11(t)
	kael := loadShippedDialogue(t, "ironwind_steppe/240.yaml")
	require.Contains(t, menuKeys(kael, heldState()), "the_problem", "Kael offers the quest")

	sides := []struct {
		file, cause, terms, stand, handin, value, fetch, ret       string
		otherFile, otherCause, otherTerms, otherStand, otherHandin string
	}{
		{"ironwind_steppe/241.yaml", "her_cause", "sylara_terms", "stand_with_sylara", "totem_return", "sylara", "sylara", "totem",
			"ironwind_steppe/242.yaml", "his_cause", "rhett_terms", "stand_with_rhett", "sample_return"},
		{"ironwind_steppe/242.yaml", "his_cause", "rhett_terms", "stand_with_rhett", "sample_return", "rhett", "rhett", "samples",
			"ironwind_steppe/241.yaml", "her_cause", "sylara_terms", "stand_with_sylara", "totem_return"},
	}
	for _, s := range sides {
		t.Run(s.value, func(t *testing.T) {
			df := loadShippedDialogue(t, s.file)
			other := loadShippedDialogue(t, s.otherFile)

			unsided := heldState("11-start")
			keys := menuKeys(df, unsided)
			require.NotContains(t, keys, s.cause, "the offer is closed once the quest is held")
			require.Contains(t, keys, s.terms, "the side's terms are open with no side chosen")
			require.NotContains(t, keys, s.stand, "the choice waits until the terms are heard")

			heard := heldState("11-start")
			heard.HasHeard = func(string) bool { return true }
			require.Contains(t, menuKeys(df, heard), s.stand, "hearing the terms opens the choice")

			// On the fetch step the item is carried but the hand-in is not
			// open yet; on the return step it is, and only there.
			for _, step := range []struct {
				id         string
				handInOpen bool
			}{{s.fetch, false}, {s.ret, true}} {
				ps := quest11State(step.id, s.value, true)
				keys := menuKeys(df, ps)
				if step.handInOpen {
					require.Contains(t, keys, s.handin, "the hand-in is open at the return step")
				} else {
					require.NotContains(t, keys, s.handin, "the hand-in waits for the return step")
				}
				require.NotContains(t, keys, s.stand, "the stand is not offered twice, at "+step.id)
				otherKeys := menuKeys(other, ps)
				require.NotContains(t, otherKeys, s.otherStand, "the other side's choice is closed at "+step.id)
				require.NotContains(t, otherKeys, s.otherTerms, "the other side's terms are closed at "+step.id)
				require.NotContains(t, otherKeys, s.otherHandin, "the other side's hand-in is closed at "+step.id)
			}

			// A finished side holds only its own steps, and no hand-in is left.
			done := quest11State("end", s.value, true)
			require.NotContains(t, menuKeys(df, done), s.handin, "the hand-in closes at the end")
			require.NotContains(t, menuKeys(other, done), s.otherHandin)

			// A save from before the side steps: the flag is set, the player is
			// still on 11-start, and the terms may never have been heard.
			flagged := quest11State("start", s.value, true)
			flagged.HasHeard = func(string) bool { return false }
			keys = menuKeys(df, flagged)
			require.Contains(t, keys, s.terms, "the flagged save can hear its side's terms")
			require.NotContains(t, keys, s.stand, "the stand waits for the terms")
			require.NotContains(t, keys, s.handin, "the hand-in follows the side's return step")
			require.NotContains(t, menuKeys(other, flagged), s.otherTerms, "the other side's terms stay hidden")
			require.NotContains(t, menuKeys(other, flagged), s.otherStand)
			flagged.HasHeard = func(string) bool { return true }
			keys = menuKeys(df, flagged)
			require.Contains(t, keys, s.stand, "with the terms heard the flagged save can stand again")
			require.NotContains(t, keys, s.handin, "the hand-in follows the side's return step")

			// An old save that finished with no side holds only 11-start (and
			// 11-end), and the flag is empty, so the terms stay open.
			finished := quest11State("end", "", false)
			require.Contains(t, menuKeys(df, finished), s.terms, "the unsided finished save still hears the terms")
			require.Contains(t, menuKeys(df, finished), "late_stand_with_"+s.value)

			// #958 playtest: on the return step without the item, the NPC
			// sends the player back to fetch it instead of asking for it.
			carrying, _ := df.OpenerFor(quest11State(s.ret, s.value, true))
			dropped, _ := df.OpenerFor(quest11State(s.ret, s.value, false))
			fetching, _ := df.OpenerFor(quest11State(s.fetch, s.value, false))
			require.NotEqual(t, carrying, dropped, "the 'you have it' opener needs the item carried")
			require.Equal(t, fetching, dropped, "without the item the fetch opener plays")

			// #958 playtest: a player who chose this side and set the quest
			// aside is not pitched the other side's offer; their own still
			// takes it back up.
			shelved := heldState()
			shelved.GetQuestFlag = func(key string) string {
				if key == "11-branch" {
					return s.value
				}
				return ""
			}
			require.Contains(t, menuKeys(df, shelved), s.cause, "the chosen side's offer resumes the quest")
			require.NotContains(t, menuKeys(other, shelved), s.otherCause, "the other side does not pitch again")
		})
	}
}

// #290: Marek hands over a replacement notice only to a player without one.
func TestMarekOnlyReplacesALostNotice(t *testing.T) {
	const notice = 30
	marek := loadShippedDialogue(t, "thornwall_city/96.yaml")

	lost := heldState("10-start")
	require.Contains(t, menuKeys(marek, lost), "lost_notice")

	carrying := heldState("10-start")
	carrying.CarriesItem = func(id int) bool { return id == notice }
	require.NotContains(t, menuKeys(marek, carrying), "lost_notice")
}

// A held item on the first step: new players go through the carrying offer and
// Hook 1, and a save from before this shipped gets have_it (spec, "Old saves").
func TestGiversHandleAHeldItem(t *testing.T) {
	for _, c := range []struct {
		file, offer, carrying, handin string
		item                          int
		start, deliver, end           string
	}{
		{"dustwalk_road/83.yaml", "help_quest", "help_quest_carrying", "evidence_handoff", 16, "4-start", "4-evidence", "4-end"},
		{"thornwall_outskirts/89.yaml", "help_quest", "help_quest_carrying", "dornhandoff", 24, "7-start", "7-deliver", "7-end"},
		{"marches_spur_road/250.yaml", "quest_start", "quest_start_carrying", "quest_complete_dialogue", 40039, "15-start", "15-return", "15-end"},
	} {
		df := loadShippedDialogue(t, c.file)
		holding := heldState()
		holding.CarriesItem = func(id int) bool { return id == c.item }
		holding.HasItem = func(id int) bool { return id == c.item }

		keys := menuKeys(df, holding)
		require.Contains(t, keys, c.carrying, c.file)
		require.NotContains(t, keys, c.offer, c.file)
		keys = menuKeys(df, heldState())
		require.Contains(t, keys, c.offer, c.file)
		require.NotContains(t, keys, c.carrying, c.file)
		require.NotEmpty(t, df.FindTopic(c.carrying).Objective, c.file)

		// have_it shows only on the start step with the item.
		onStart := heldState(c.start)
		onStart.CarriesItem = holding.CarriesItem
		onStart.HasItem = holding.HasItem
		require.Contains(t, menuKeys(df, onStart), "have_it", c.file)
		require.NotContains(t, menuKeys(df, onStart), c.handin, "the hand-in follows the last step: "+c.file)
		require.NotContains(t, menuKeys(df, heldState(c.start)), "have_it", c.file)

		delivered := heldState(c.start, c.deliver)
		delivered.CarriesItem = holding.CarriesItem
		delivered.HasItem = holding.HasItem
		require.Contains(t, menuKeys(df, delivered), c.handin, c.file)
		require.NotContains(t, menuKeys(df, delivered), "have_it", c.file)
	}

	// The ledgers cannot be held before their quests, so they have no second
	// offer, only have_it for an old save.
	for _, c := range []struct {
		file, handin   string
		item           int
		start, deliver string
	}{
		{"watchers_crossing/84.yaml", "tolvahandoff", 21, "5-start", "5-deliver"},
		{"thornwall_city/95.yaml", "olenhandoff", 29, "9-start", "9-deliver"},
	} {
		df := loadShippedDialogue(t, c.file)
		onStart := heldState(c.start)
		onStart.CarriesItem = func(id int) bool { return id == c.item }
		onStart.HasItem = onStart.CarriesItem
		require.Contains(t, menuKeys(df, onStart), "have_it", c.file)
		require.NotContains(t, menuKeys(df, onStart), c.handin, c.file)
	}
}

// Tier 2 (owner, 2026-11-02): Delia's herbs hand-in is gone, so quest 16 can
// only be finished by meeting the forager. Holding the herbs adds no hand-in.
func TestTier2DeliaHerbsAreNoHandIn(t *testing.T) {
	df := loadShippedDialogue(t, "ashwick/259.yaml")
	withHerbs := func(tok string) *PlayerState {
		ps := heldState(tok)
		ps.HasItem = func(id int) bool { return id == 40040 }
		ps.CarriesItem = ps.HasItem
		return ps
	}
	require.NotContains(t, menuKeys(df, withHerbs("16-start")), "quest_complete_herbs")
	require.NotContains(t, menuKeys(df, withHerbs("16-start")), "quest_complete_forager")
	at := menuKeys(df, withHerbs("16-forager"))
	require.NotContains(t, at, "quest_complete_herbs")
	handIns := 0
	for _, e := range df.Menu("npc", withHerbs("16-forager")).Entries {
		if strings.HasPrefix(e.Id, "quest_complete") {
			handIns++
		}
	}
	require.Equal(t, 1, handIns)
	require.Contains(t, at, "quest_complete_forager")
}

// Tier 2: a hand-in follows the quest's last step.
func TestTier2HandInsFollowTheLastStep(t *testing.T) {
	for _, c := range []struct {
		file, topic, onPrev, onLast string
		item                        int
	}{
		{"ashwick/259.yaml", "quest_complete_forager", "16-start", "16-forager", 0},
		{"new_plymouth_outskirts/9208.yaml", "guardhandoff", "60-waybill", "60-manifest", 40073},
		{"stillwater/347.yaml", "ullahandoff", "20-kingfisher_found", "20-vella_journal", 40061},
		{"stillwater/355.yaml", "vellahandoff", "20-brindle_confession", "20-kingfisher_found", 40060},
	} {
		df := loadShippedDialogue(t, c.file)
		state := func(tok string) *PlayerState {
			ps := heldState(tok)
			ps.HasItem = func(id int) bool { return id == c.item }
			ps.CarriesItem = ps.HasItem
			return ps
		}
		require.NotContains(t, menuKeys(df, state(c.onPrev)), c.topic, c.file)
		require.Contains(t, menuKeys(df, state(c.onLast)), c.topic, c.file)
	}
}

// Tier 3: Quist offers The Undercroft at his hand-in, through the chain.
func TestQuistOffersTheUndercroft(t *testing.T) {
	quist := loadShippedDialogue(t, "the_confluence/9441.yaml")
	require.Contains(t, menuKeys(quist, heldState("73-end")), "undercroft_offer")
	require.NotContains(t, menuKeys(quist, heldState("73-map")), "undercroft_offer")
	require.NotContains(t, menuKeys(quist, heldState("73-end", "74-start")), "undercroft_offer")

	cael := loadShippedDialogue(t, "the_confluence/9465.yaml")
	require.Nil(t, cael.FindTopic("q74_grant"), "the offer moved to Quist")
	require.Contains(t, menuKeys(cael, heldState("74-start")), "q74_work")
	require.NotContains(t, menuKeys(cael, heldState("74-start", "74-ledger")), "q74_work")
}

// Tier 3: the choice of who gets the rubbing opens at the reveal, the step
// that gives the rubbing, not the step before it.
func TestTheUndercroftsEndingsFollowTheReveal(t *testing.T) {
	for _, c := range []struct{ file, topic, terms string }{
		{"the_confluence/9464.yaml", "aldrichandoff", "aldric_terms"},
		{"the_confluence/9454.yaml", "marginhandoff", "margin_terms"},
	} {
		df := loadShippedDialogue(t, c.file)
		state := func(tok string) *PlayerState {
			ps := heldState(tok)
			ps.HasItem = func(id int) bool { return id == 40142 }
			ps.CarriesItem = ps.HasItem
			// The terms topic was heard, so only the quest gate is under test.
			ps.HasHeard = func(string) bool { return true }
			return ps
		}
		require.NotContains(t, menuKeys(df, state("74-descent")), c.topic, c.file)
		require.Contains(t, menuKeys(df, state("74-reveal")), c.topic, c.file)
		// The terms topic that explains the choice opens with it, not before.
		require.NotContains(t, menuKeys(df, state("74-descent")), c.terms, c.file)
		require.Contains(t, menuKeys(df, state("74-reveal")), c.terms, c.file)
	}
}
