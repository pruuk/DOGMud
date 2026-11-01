package questengine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The brewing line of Pothole Coulee (#375 phase 7): Crier Toke offers First
// Brew (38) and Herbalist Birna takes it; Birna offers Wade the Marsh (39)
// and Fenwalker Falv takes it; Falv offers Heart of the Mire (40), then
// Report to Toke (Brewing) (92) and the repeatable Another Brew (55). Potions
// are taken with quaff (plan 1). One command per step, every quest ends at a
// hand-in.

const (
	glassVial    = 40006
	healersRoot  = 40004
	firebombItem = 30058
	marshLeech   = 9130
	reedLurker   = 9131
	birnaMob     = 9128
	falvMob      = 9129
	sheltered    = 5265 // Birna's pool, an alchemy bench
	dryingRacks  = 5267 // the second alchemy bench
	reedBeds     = 5269
	sunkenPath   = 5270
	cattailStand = 5271 // Falv's camp
	mudflat      = 5272
	blackPool    = 5273
)

// countingLinearCtx keeps count: progress, as the game's bridge does, so a
// count: N trigger needs N matches (the plain walker context reaches any
// count at once).
type countingLinearCtx struct {
	*linearCtx
	counts map[string]int
}

func (c *countingLinearCtx) BumpStepCount(questId int, step, key string) (int, bool) {
	c.counts[key]++
	return c.counts[key], true
}

func notifyCounting(w *questWalker, ctx *countingLinearCtx, event string, d EventDetails) {
	d.UserId = 1
	w.p.roomId = d.RoomId
	w.e.Notify(event, d, w.p, ctx)
}

func TestQuest38FirstBrew_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "38-first_brew.yaml", 38)
	require.Equal(t, []string{"start", "met", "list", "vial", "roots", "brew", "drink", "end"}, newLinearPlayer(q).steps,
		"the old ids keep their places; met is inserted after start")
	turnIn := turnInRequires(t, "9128.yaml", "brew_turnin")
	require.Equal(t, []string{"38-drink"}, turnIn)
	heard := EventDetails{MobId: birnaMob, Topic: "brew_arrive", RoomId: sheltered}

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		ctx := &countingLinearCtx{linearCtx: w.ctx, counts: map[string]int{}}

		notifyCounting(w, ctx, "command", EventDetails{Command: "list", RoomId: sheltered})
		require.Equal(t, "start", w.p.current, "listing before Birna has spoken must not skip the talk")
		notifyCounting(w, ctx, "topic_heard", heard)
		require.Equal(t, "met", w.p.current)

		notifyCounting(w, ctx, "item_gain", EventDetails{ItemId: glassVial, RoomId: sheltered})
		require.Equal(t, "met", w.p.current, "a vial bought before listing must not skip the list step")
		notifyCounting(w, ctx, "command", EventDetails{Command: "list", RoomId: sheltered})
		require.Equal(t, "list", w.p.current)

		notifyCounting(w, ctx, "item_gain", EventDetails{ItemId: healersRoot, RoomId: sheltered})
		require.Equal(t, "list", w.p.current, "a root before the vial must not skip the vial step")
		notifyCounting(w, ctx, "item_gain", EventDetails{ItemId: glassVial, RoomId: sheltered})
		require.Equal(t, "vial", w.p.current)

		notifyCounting(w, ctx, "command", EventDetails{Command: "quaff", RoomId: sheltered})
		require.Equal(t, "vial", w.p.current, "a quaff before the brew must not count")
		notifyCounting(w, ctx, "command", EventDetails{Command: "craft", RoomId: sheltered})
		require.Equal(t, "vial", w.p.current, "a craft before the roots must not count (no lenient grant)")

		notifyCounting(w, ctx, "item_gain", EventDetails{ItemId: healersRoot, RoomId: sheltered})
		require.Equal(t, "vial", w.p.current, "#695: one root is not the salve's two")
		notifyCounting(w, ctx, "item_gain", EventDetails{ItemId: healersRoot, RoomId: sheltered})
		require.Equal(t, "roots", w.p.current, "the second root finishes the step")
		require.False(t, holdsAll(w.p, turnIn))

		notifyCounting(w, ctx, "command", EventDetails{Command: "craft", RoomId: dryingRacks})
		require.Equal(t, "brew", w.p.current, "the Drying Racks bench counts")
		notifyCounting(w, ctx, "command", EventDetails{Command: "drink", RoomId: sheltered})
		require.Equal(t, "brew", w.p.current, "drink is for food and drink now; a potion is taken with quaff")
		notifyCounting(w, ctx, "command", EventDetails{Command: "quaff", RoomId: sheltered})
		require.Equal(t, "drink", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the last step")
	})

	t.Run("the vial hint names a buy command that works", func(t *testing.T) {
		require.Contains(t, questHintFor(t, q, "vial"), "type buy 2 healer's root", "#695")
	})

	t.Run("Birna's talk step", func(t *testing.T) {
		checkArriveTopic(t, "birna", "9128.yaml", q, birnaMob, "brew_arrive", []string{"30-end", "31-end"})
	})
}

func TestQuest39WadeTheMarsh_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "39-wade_the_marsh.yaml", 39)
	require.Equal(t, []string{"start", "met", "leech", "throw", "forage", "end"}, newLinearPlayer(q).steps)
	turnIn := turnInRequires(t, "9129.yaml", "marsh_turnin")
	require.Equal(t, []string{"39-forage"}, turnIn)
	heard := EventDetails{MobId: falvMob, Topic: "marsh_arrive", RoomId: cattailStand}

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("mob_death", EventDetails{MobId: marshLeech, RoomId: reedBeds})
		require.Equal(t, "start", w.p.current, "a leech before Falv has spoken must not skip the talk")
		w.notify("topic_heard", heard)
		require.Equal(t, "met", w.p.current)

		w.notify("command", EventDetails{Command: "forage", RoomId: sunkenPath})
		require.Equal(t, "met", w.p.current, "a forage before the leech must not skip ahead")
		w.notify("mob_death", EventDetails{MobId: marshLeech, RoomId: reedBeds})
		require.Equal(t, "leech", w.p.current)

		// #694: a throw that reached nobody does not count.
		w.notify("command", EventDetails{Command: "throw", RoomId: sunkenPath, TargetKind: "none"})
		require.Equal(t, "leech", w.p.current, "a throw at nobody must not finish the throw step")
		w.notify("command", EventDetails{Command: "forage", RoomId: blackPool})
		require.Equal(t, "leech", w.p.current, "a forage before the throw must not skip it")
		w.notify("command", EventDetails{Command: "throw", RoomId: mudflat, TargetKind: "mob", MobId: reedLurker})
		require.Equal(t, "throw", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the forage")

		w.notify("command", EventDetails{Command: "forage", RoomId: sunkenPath})
		require.Equal(t, "forage", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("the first fight carries the dark aside", func(t *testing.T) {
		require.Contains(t, q.Steps[1].Hint, darkAside)
	})

	t.Run("Birna offers it with the firebomb and Falv no longer does", func(t *testing.T) {
		checkOfferMoved(t, "9128.yaml", "marsh_offer", q, "38-end", "9129.yaml")
		require.Equal(t, firebombItem, loadCouleeTopics(t, "9128.yaml").FindTopic("marsh_offer").GivesItem)
	})

	t.Run("Falv's talk step", func(t *testing.T) {
		checkArriveTopic(t, "falv", "9129.yaml", q, falvMob, "marsh_arrive", []string{"30-end", "38-end"})
	})
}

func TestQuest40HeartOfTheMire_KillThenReport(t *testing.T) {
	q := loadShippedQuest(t, "40-heart_of_the_mire.yaml", 40)
	require.Equal(t, []string{"start", "slain", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, q.Steps[0].Hint, darkAside)
	turnIn := turnInRequires(t, "9129.yaml", "mire_in")
	require.Equal(t, []string{"40-slain"}, turnIn)

	w := newQuestWalker(q).at("start")
	w.notify("mob_death", EventDetails{MobId: marshLeech, RoomId: blackPool})
	require.Equal(t, "start", w.p.current, "a leech is not the spirit")
	w.notify("mob_death", EventDetails{MobId: 9135, RoomId: 5280})
	require.Equal(t, "slain", w.p.current)
	require.False(t, holdsAll(w.p, []string{"40-end"}), "the kill no longer ends the quest")
}

func TestQuest55AnotherBrew_EndsAtFalv(t *testing.T) {
	q := loadShippedQuest(t, "55-another_brew.yaml", 55)
	require.Equal(t, []string{"start", "brew", "quaffed", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, questHintFor(t, q, "brew"), "type quaff healing salve")
	require.Contains(t, questHintFor(t, q, "quaffed"), "type talk falv")
	turnIn := turnInRequires(t, "9129.yaml", "brew_loop_in")
	require.Equal(t, []string{"55-quaffed"}, turnIn)

	w := newQuestWalker(q).at("start")
	w.notify("command", EventDetails{Command: "quaff", RoomId: sheltered})
	require.Equal(t, "start", w.p.current, "a quaff before the brew must not count")
	w.notify("command", EventDetails{Command: "craft", RoomId: dryingRacks})
	require.Equal(t, "brew", w.p.current, "a craft at the Drying Racks bench counts")
	w.notify("command", EventDetails{Command: "quaff", RoomId: 5271})
	require.Equal(t, "quaffed", w.p.current)
	require.False(t, holdsAll(w.p, []string{"55-end"}), "the quaff no longer ends the run")
}

// Falv hands out a replacement firebomb while quest 39 is open. A player who
// still carries one must not get a second (S8): the handout topic carries
// itemExcluded, and its reminder (carriesItem, same gates) hands nothing out.
// Every firebomb handout on Falv is checked, so a new one cannot skip the
// rule. Birna's offer hands over the first firebomb.
func TestFalvFirebombHandoutGivesOnlyOne(t *testing.T) {
	falv := loadCouleeTopics(t, "9129.yaml")
	found := 0
	for i := range falv.Topics {
		tp := &falv.Topics[i]
		if tp.GivesItem != firebombItem || tp.GrantsQuest != "" {
			continue
		}
		found++
		require.Equal(t, firebombItem, tp.ItemExcluded, "topic %s hands a firebomb to a player who already carries one", tp.Key)
	}
	require.Equal(t, 1, found, "expected exactly one firebomb handout topic; the check below names it")

	q := loadShippedQuest(t, "39-wade_the_marsh.yaml", 39)
	n := newNpcTalker(falv, playerOn(q, "start", "38-end"))
	checkHandout(t, n, "firebomb_recovery", "firebomb_already_have", firebombItem)
	require.Contains(t, squash(falv.FindTopic("firebomb_already_have").Text), "already have one")
}
