package questengine

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
)

// The archery line under #375 phase 7: Toke offers First Shot (50) and Iden
// takes it; Iden offers Across the Canyon (51) and Bryn takes it; Bryn offers
// The Overlook (52), takes its report, and keeps Range Practice (59) and the
// bridge back to Toke. One command per step, in order; handouts give only to
// a player who does not already carry one.

const (
	canyonMouth    = 5353
	canyonNarrows  = 5354 // Echoing Narrows: the shot is taken FROM here
	soundingFloor  = 5356 // The Sounding Floor: where the scout stands
	canyonHead     = 5358
	overlookRoom   = 5364
	canyonScout    = 9165
	scoutBryn      = 9162
	bowmasterSkell = 9169
	pouchOfShot    = 30064
	caseOfBolts    = 30063
	slingItem      = 10038
)

// atScout is a shot at the Canyon Scout from the Echoing Narrows. A miss
// counts too: the command event goes out when the shot resolves, hit or not.
var atScout = EventDetails{Command: "fire", RoomId: canyonNarrows, TargetKind: "mob", MobId: canyonScout}

// squash lowercases and collapses whitespace, so a folded YAML line compares
// the way the player reads it.
func squash(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func readQuestFile(t *testing.T, file string) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "quests", file))
	require.NoError(t, err)
	return string(raw)
}

// playerOn builds a linear player for quest q at step, holding the given
// tokens from other quests (the prerequisites a real character would carry).
func playerOn(q *QuestDef, step string, other ...string) *linearPlayer {
	p := newLinearPlayer(q)
	p.current = step
	for _, tok := range other {
		p.fullMockPlayer.quests[tok] = true
	}
	return p
}

// questHintFor returns the step's own hint, squashed.
func questHintFor(t *testing.T, q *QuestDef, step string) string {
	t.Helper()
	for _, s := range q.Steps {
		if s.Id == step {
			return squash(s.Hint)
		}
	}
	t.Fatalf("quest %d has no step %q", q.QuestId, step)
	return ""
}

// The shots in 51 and 59 are taken at the Canyon Scout from the Narrows. Only
// that counts: not a shot at nothing, at Bryn, at a player, at Skell, or from
// the Sounding Floor itself.
func checkOnlyAScoutShotCounts(t *testing.T, q *QuestDef, step string, others ...string) {
	t.Helper()
	wrong := map[string]EventDetails{
		"no target":      {Command: "fire", RoomId: canyonNarrows},
		"Bryn":           {Command: "fire", RoomId: canyonNarrows, TargetKind: "mob", MobId: scoutBryn},
		"Skell":          {Command: "fire", RoomId: canyonNarrows, TargetKind: "mob", MobId: bowmasterSkell},
		"another player": {Command: "fire", RoomId: canyonNarrows, TargetKind: "player"},
		"from the floor": {Command: "fire", RoomId: soundingFloor, TargetKind: "mob", MobId: canyonScout},
	}
	for name, ev := range wrong {
		w := newQuestWalker(q).at(step)
		for _, tok := range others {
			w.p.fullMockPlayer.quests[tok] = true
		}
		w.notify("command", ev)
		require.Equal(t, step, w.p.current, "quest %d: a shot %s must not count", q.QuestId, name)
	}
}

func TestQuest51AcrossTheCanyon_GiverToGiver(t *testing.T) {
	q := loadShippedQuest(t, "51-across_the_canyon.yaml", 51)
	require.Equal(t, []string{"start", "met", "shoot", "kill", "end"}, newLinearPlayer(q).steps,
		"the old ids survive so a player mid-quest resumes; met is the meeting with Bryn")
	turnIn := turnInRequires(t, "9162.yaml", "canyon_turnin")
	require.Equal(t, []string{"51-kill"}, turnIn)
	require.Equal(t, "canyon_offer", foldingOffers(t, "9161.yaml")[51], "Iden offers the canyon")
	require.NotContains(t, foldingOffers(t, "9162.yaml"), 51, "Bryn no longer offers it")

	require.Contains(t, questHintFor(t, q, "start"), "east twice to the high terrace, then north to the canyon mouth")
	require.Contains(t, questHintFor(t, q, "start"), "type talk bryn")
	require.Equal(t, canyonMouth, foldingWalk(t, 5350, "east", "east", "north"))
	require.Contains(t, questHintFor(t, q, "met"), "type fire scout north")
	require.Contains(t, q.Steps[1].Hint, darkAside)
	require.Contains(t, questHintFor(t, q, "shoot"), "type fire scout")
	require.Contains(t, questHintFor(t, q, "kill"), "type talk bryn")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", atScout)
		require.Equal(t, "start", w.p.current, "a shot before Bryn's lesson does not count")
		w.notify("topic_heard", EventDetails{MobId: scoutBryn, Topic: "canyon_meet"})
		require.Equal(t, "met", w.p.current)

		w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: soundingFloor})
		require.Equal(t, "met", w.p.current, "a kill before the cross-room shot must not skip the shot")
		w.notify("command", EventDetails{Command: "fire", RoomId: soundingFloor, TargetKind: "mob", MobId: canyonScout})
		require.Equal(t, "met", w.p.current, "only a shot from the Echoing Narrows counts")
		w.notify("command", atScout)
		require.Equal(t, "shoot", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in waits for the kill")
		w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: canyonNarrows})
		require.Equal(t, "kill", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("only a shot at the scout counts", func(t *testing.T) {
		checkOnlyAScoutShotCounts(t, q, "met")
	})

	t.Run("only the scout's death counts", func(t *testing.T) {
		w := newQuestWalker(q).at("shoot")
		w.notify("mob_death", EventDetails{MobId: bowmasterSkell, RoomId: canyonNarrows})
		require.Equal(t, "shoot", w.p.current)
	})

	t.Run("player saved on shoot resumes with the kill", func(t *testing.T) {
		w := newQuestWalker(q).at("shoot")
		w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: canyonNarrows})
		require.Equal(t, "kill", w.p.current)
	})

	t.Run("player saved on kill can turn in", func(t *testing.T) {
		require.True(t, holdsAll(newQuestWalker(q).at("kill").p, turnIn))
	})

	// No line may promise what a shot cannot promise: a shot can miss.
	t.Run("no line promises a hit", func(t *testing.T) {
		text := squash(readQuestFile(t, "51-across_the_canyon.yaml"))
		for _, claim := range []string{"finds the scout", "it is hit", "bit.", "and bit", "it is coming"} {
			require.NotContains(t, text, claim)
		}
	})
}

func TestQuest52TheOverlook_KillThenReport(t *testing.T) {
	q := loadShippedQuest(t, "52-the_overlook.yaml", 52)
	require.Equal(t, []string{"start", "kill", "end"}, newLinearPlayer(q).steps)
	turnIn := turnInRequires(t, "9162.yaml", "overlook_turnin")
	require.Equal(t, []string{"52-kill"}, turnIn)
	require.Contains(t, questHintFor(t, q, "start"), "type fire skell")
	require.Contains(t, q.Steps[0].Hint, darkAside, "the Overlook goes dark at night; the start hint carries the dark aside")
	require.NotContains(t, questHintFor(t, q, "start"), "ammo",
		"ammo advice is not the step's command; it belongs in the description")
	require.Contains(t, squash(q.Steps[0].Description), "ammo")

	// Bryn gives the climb from the Canyon Mouth: the start hint walks it.
	require.Contains(t, questHintFor(t, q, "start"),
		"from the canyon mouth, go north four times, up, then north three times to the overlook")
	require.Equal(t, overlookRoom, foldingWalk(t, canyonMouth, "north", "north", "north", "north", "up", "north", "north", "north"))

	back := questHintFor(t, q, "kill")
	require.Contains(t, back, "south three times, down, then south four times to the canyon mouth")
	require.Contains(t, back, "type talk bryn")
	require.Equal(t, canyonMouth, foldingWalk(t, overlookRoom, "south", "south", "south", "down", "south", "south", "south", "south"))

	w := newQuestWalker(q).at("start")
	w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: canyonNarrows})
	require.Equal(t, "start", w.p.current, "only Skell's death moves the Overlook on")
	w.notify("mob_death", EventDetails{MobId: bowmasterSkell, RoomId: overlookRoom})
	require.Equal(t, "kill", w.p.current, "the boss kill moves to the report; Bryn takes the hand-in")
	require.False(t, w.p.HasQuest("52-end"))
	require.True(t, holdsAll(w.p, turnIn))

	// Skell comes back on his room's timer, so a player who finds him gone
	// is told to wait, in the description and in Bryn's answer.
	require.Contains(t, squash(q.Steps[0].Description), "wait")
}

func TestQuest59RangePractice_ReportsToBryn(t *testing.T) {
	q := loadShippedQuest(t, "59-range_practice.yaml", 59)
	require.True(t, q.Repeatable)
	require.Equal(t, []string{"start", "shoot", "kill", "end"}, newLinearPlayer(q).steps)
	turnIn := turnInRequires(t, "9162.yaml", "range_loop_turnin")
	require.Equal(t, []string{"59-kill"}, turnIn)
	require.Contains(t, questHintFor(t, q, "start"), "type fire scout north")
	require.Contains(t, questHintFor(t, q, "shoot"), "type fire scout")
	require.Contains(t, questHintFor(t, q, "kill"), "type talk bryn")

	// N13: Bryn is "he" (mobs/pothole_coulee/9162-scout_bryn.yaml).
	require.NotRegexp(t, regexp.MustCompile(`(?i)\b(she|her|hers|herself)\b`), readQuestFile(t, "59-range_practice.yaml"),
		"Bryn is a he")

	w := newQuestWalker(q).at("start")
	w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: soundingFloor})
	require.Equal(t, "start", w.p.current, "a kill before the shot does not count")
	w.notify("command", atScout)
	require.Equal(t, "shoot", w.p.current)
	w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: canyonNarrows})
	require.Equal(t, "kill", w.p.current)
	require.True(t, holdsAll(w.p, turnIn))

	t.Run("only a shot at the scout counts", func(t *testing.T) {
		checkOnlyAScoutShotCounts(t, q, "start")
	})

	t.Run("no line promises a hit", func(t *testing.T) {
		text := squash(readQuestFile(t, "59-range_practice.yaml"))
		for _, claim := range []string{"finds the scout", "it is hit", "and bites", "crosses the slot and bites", "now it will come"} {
			require.NotContains(t, text, claim)
		}
	})
}

// Bryn greets each step with its command, a quest-aware topic answers it, and
// every hand-in he takes completes its quest.
func TestBryn9162_MidQuestGreetsAndAnswersEachStep(t *testing.T) {
	f51, f52, f59 := "51-across_the_canyon.yaml", "52-the_overlook.yaml", "59-range_practice.yaml"
	checkMeetStep(t, "bryn", "9162.yaml", f51, 51, "canyon_meet", "type fire scout north", "30-end", "50-end")
	all := []string{"50-end", "51-end", "52-end"}
	checkTopicSteps(t, "bryn", "9162.yaml", []topicStep{
		{f51, 51, "met", "type fire scout north", []string{"50-end"}, "coach_51_start", false},
		{f51, 51, "shoot", "type fire scout", []string{"50-end"}, "coach_51_shoot", false},
		{f51, 51, "kill", "", []string{"50-end"}, "canyon_turnin", true},
		{f52, 52, "start", "type fire skell", []string{"50-end", "51-end"}, "coach_52_start", false},
		{f52, 52, "kill", "", []string{"50-end", "51-end"}, "overlook_turnin", true},
		{f59, 59, "start", "type fire scout north", all, "coach_59_start", false},
		{f59, 59, "shoot", "type fire scout", all, "coach_59_shoot", false},
		{f59, 59, "kill", "", all, "range_loop_turnin", true},
	})
}

// A character saved before this pass can hold 51-start with no 51-met (the old
// first step), or 51-shoot or 51-kill without it. Each must hear the right
// thing from Bryn and from Iden, never be sent back to the lesson or be left
// with no way forward.
func TestBryn9162_OldSavesOnQuest51(t *testing.T) {
	// The linear player derives every token of its own quest from its step, so
	// it cannot hold 51-shoot without 51-met. An old save is raw tokens, which
	// a player standing on another quest's linear model keeps exactly as set.
	q52 := loadShippedQuest(t, "52-the_overlook.yaml", 52)
	old := func(tokens ...string) *linearPlayer {
		return playerOn(q52, "", append([]string{"30-end", "50-end"}, tokens...)...)
	}
	bryn := loadCouleeTopics(t, "9162.yaml")
	iden := loadCouleeTopics(t, "9161.yaml")

	// Saved on the old start step: no 51-met yet. Bryn's lesson is on the menu.
	n := newNpcTalker(bryn, old("51-start"))
	require.NotNil(t, n.eligible("canyon_meet"), "a player saved on start hears the lesson first")
	_, hints := n.opener()
	require.Contains(t, squash(hints), "hear the canyon lesson")

	// Saved on shoot or kill: the lesson is behind them and must not reopen, or
	// its topic_heard grant would hand 51-met to a player past it.
	for name, tokens := range map[string][]string{
		"shoot": {"51-start", "51-shoot"},
		"kill":  {"51-start", "51-shoot", "51-kill"},
	} {
		n = newNpcTalker(bryn, old(tokens...))
		require.Nil(t, n.eligible("canyon_meet"), "saved on %s: the lesson stays shut", name)
		_, hints = n.opener()
		require.NotContains(t, squash(hints), "canyon lesson", "saved on %s: Bryn does not send them back", name)

		n = newNpcTalker(iden, old(tokens...))
		_, hints = n.opener()
		require.NotContains(t, squash(hints), "type talk bryn", "saved on %s: Iden does not send them back to Bryn", name)
	}

	// Saved on kill, the hand-in is open and completes the quest.
	n = newNpcTalker(bryn, old("51-start", "51-shoot", "51-kill"))
	require.NotNil(t, n.eligible("canyon_turnin"))
	n.choose(t, "canyon_turnin")
	require.True(t, n.p.HasQuest("51-end"))
}

// Iden's canyon offer and her opener, which names the real step.
func TestIden9161_CanyonOfferAndOpener(t *testing.T) {
	q := loadShippedQuest(t, "51-across_the_canyon.yaml", 51)
	iden := loadCouleeTopics(t, "9161.yaml")
	offer := iden.FindTopic("canyon_offer")
	require.NotNil(t, offer)
	require.Equal(t, dialogue.KindOffer, offer.Kind)
	require.Equal(t, 51, offer.Quest)
	require.Equal(t, "Across the Canyon", offer.Label)
	require.Equal(t, "51-start", offer.GrantsQuest)
	require.Equal(t, []string{"50-end"}, offer.QuestRequired)
	require.Contains(t, offer.QuestExcluded, "51-end", "a finished quest is not offered again")
	require.Contains(t, squash(offer.Hints), "east twice to the high terrace, then north to the canyon mouth")
	require.Equal(t, 1, typedCount(offer.Hints))

	// Before the offer is taken Iden's opener points at it; once taken, at
	// Bryn; once Bryn's lesson is heard, no longer at the lesson.
	n := newNpcTalker(iden, playerOn(q, "", "30-end", "50-end"))
	_, hints := n.opener()
	require.Contains(t, squash(hints), "choose across the canyon below")
	require.NotNil(t, n.eligible("canyon_offer"))
	n.choose(t, "canyon_offer")
	require.True(t, n.p.HasQuest("51-start"))

	n = newNpcTalker(iden, playerOn(q, "start", "30-end", "50-end"))
	_, hints = n.opener()
	require.Contains(t, squash(hints), "type talk bryn", "taken, not yet met: Bryn is waiting")
	n = newNpcTalker(iden, playerOn(q, "met", "30-end", "50-end", "51-met"))
	_, hints = n.opener()
	require.NotContains(t, squash(hints), "type talk bryn", "met: Iden does not send them to Bryn again")
}

// S8: a handout topic gives its item once. A player who already carries one
// sees the reminder instead, and gets nothing.
func TestRangedGivers_HandoutsGiveOnlyWhenTheItemIsMissing(t *testing.T) {
	q50 := loadShippedQuest(t, "50-first_shot.yaml", 50)
	q51 := loadShippedQuest(t, "51-across_the_canyon.yaml", 51)
	q52 := loadShippedQuest(t, "52-the_overlook.yaml", 52)
	iden := loadCouleeTopics(t, "9161.yaml")
	bryn := loadCouleeTopics(t, "9162.yaml")

	cases := []struct {
		name         string
		df           *dialogue.DialogueFile
		p            func() *linearPlayer
		supply, have string
		item         int
	}{
		{"iden shot", iden, func() *linearPlayer { return playerOn(q50, "equip", "30-end") }, "shot_supply", "shot_have", pouchOfShot},
		{"iden sling", iden, func() *linearPlayer { return playerOn(q50, "equip", "30-end") }, "sling_recovery", "sling_have", slingItem},
		{"bryn shot during 51", bryn, func() *linearPlayer { return playerOn(q51, "start", "50-end") }, "canyon_shot_supply", "canyon_shot_have", pouchOfShot},
		{"bryn bolts during 52", bryn, func() *linearPlayer { return playerOn(q52, "start", "50-end", "51-end") }, "ammo_resupply", "ammo_have", caseOfBolts},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			checkHandout(t, newNpcTalker(c.df, c.p()), c.supply, c.have, c.item)
		})
	}
}
