package questengine

import (
	"errors"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
)

// Quest 76, The Disc, interim home (#375 phase 7, owner option A, #625): the
// Inert Disc (40167) is found by a real search in Maren's Cottage (4023); the
// same search reads the journal; casting Chrysalis Glow there wakes the disc
// (40168 for 40167); Brennan (9516) takes the hand-in. The Reliquary keeps no
// disc. A finished player who lost the woken disc can search and wake another.
func TestQuest76TheDisc_InterimHome(t *testing.T) {
	const cottage, reliquary, inert, woken = 4023, 5343, 40167, 40168
	q := loadShippedQuest(t, "76-the_disc.yaml", 76)
	require.Equal(t, []string{"start", "journal", "attune", "end"}, newLinearPlayer(q).steps,
		"the step ids are what a saved character stores; they stay")

	brennan := loadShippedDialogue(t, "greenford", "9516.yaml")
	in := brennan.FindTopic("disc_in")
	require.NotNil(t, in, "Brennan takes the hand-in")
	require.Equal(t, dialogue.KindHandIn, in.Kind)
	require.Equal(t, []string{"76-attune"}, in.QuestRequired)
	require.Equal(t, "76-end", in.GrantsQuest)
	require.Equal(t, woken, in.CarriesItem, "his line and the reward both look at the woken disc")

	search := func(room int) EventDetails { return EventDetails{Command: "search", RoomId: room} }
	// `cast glow` with no target aims at the caster (InitiateCast, TargetSelf).
	glow := func(room int) EventDetails {
		return EventDetails{Command: "cast", Noun: "chrysalis-glow", RoomId: room, TargetKind: "self"}
	}

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", glow(cottage))
		require.Equal(t, "start", w.p.current, "the hearth waits for the journal")
		require.Empty(t, w.ctx.givenItems)
		w.notify("command", search(reliquary))
		require.Equal(t, "start", w.p.current, "the Reliquary holds no disc any more")
		require.Empty(t, w.ctx.givenItems)
		w.notify("command", search(cottage))
		require.Equal(t, "journal", w.p.current)
		require.Equal(t, []int{inert}, w.ctx.givenItems, "the search turns up the disc")
		w.p.items[inert] = true
		w.notify("command", EventDetails{Command: "cast", Noun: "conviction-spike", RoomId: cottage, TargetKind: "self"})
		require.Equal(t, "journal", w.p.current, "only the glow wakes it")
		w.notify("command", glow(reliquary))
		require.Equal(t, "journal", w.p.current, "only at the marked hearth")
		w.notify("command", glow(cottage))
		require.Equal(t, "attune", w.p.current)
		require.Equal(t, []int{inert, woken}, w.ctx.givenItems)
		require.Contains(t, w.ctx.consumedItems, inert)
		require.False(t, w.p.HasQuest("76-end"), "Brennan takes the hand-in")
	})

	t.Run("a glow aimed at someone else wakes nothing", func(t *testing.T) {
		for _, kind := range []string{"mob", "player"} {
			w := newQuestWalker(q).at("journal")
			w.p.items[inert] = true
			w.notify("command", EventDetails{Command: "cast", Noun: "chrysalis-glow", RoomId: cottage, TargetKind: kind})
			require.Equal(t, "journal", w.p.current, "aimed at a %s", kind)
			require.Empty(t, w.ctx.givenItems)
		}
	})

	t.Run("a player who already carries a disc gets no second one", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.p.items[inert] = true
		w.notify("command", search(cottage))
		require.Equal(t, "journal", w.p.current)
		require.Empty(t, w.ctx.givenItems)
		w.notify("command", search(cottage))
		require.Empty(t, w.ctx.givenItems, "nor on a second search")
	})

	t.Run("a player on the journal step with no disc finds one by searching", func(t *testing.T) {
		w := newQuestWalker(q).at("journal")
		w.notify("command", search(cottage))
		require.Equal(t, "journal", w.p.current)
		require.Equal(t, []int{inert}, w.ctx.givenItems)
	})

	t.Run("a player on the journal step with no disc is handed one before the wake", func(t *testing.T) {
		w := newQuestWalker(q).at("journal")
		w.notify("command", glow(cottage))
		require.Equal(t, "attune", w.p.current)
		require.Equal(t, []int{inert, woken}, w.ctx.givenItems,
			"the wake line says the disc goes on the hearth, so one must exist")
		require.Equal(t, []int{inert}, w.ctx.consumedItems)
	})

	t.Run("a refused woken disc holds the step", func(t *testing.T) {
		w := newQuestWalker(q).at("journal")
		w.p.items[inert] = true
		ctx := &refuseGiveCtx{linearCtx: w.ctx}
		w.e.Notify("command", EventDetails{UserId: 1, Command: "cast", Noun: "chrysalis-glow", RoomId: cottage, TargetKind: "self"}, w.p, ctx)
		require.Equal(t, "journal", w.p.current, "no step moves on an undelivered disc")
		require.Empty(t, w.ctx.sentTexts, "and the wake story is not told")
	})

	t.Run("a finished player who lost the woken disc can wake another", func(t *testing.T) {
		w := newQuestWalker(q).at("end")
		w.notify("command", search(cottage))
		require.Equal(t, []int{inert}, w.ctx.givenItems)
		require.Empty(t, w.ctx.consumedItems, "nothing to swap")
		w.p.items[inert] = true
		w.notify("command", glow(cottage))
		require.Equal(t, []int{inert, woken}, w.ctx.givenItems)
		require.Equal(t, []int{inert}, w.ctx.consumedItems)
		require.Equal(t, "end", w.p.current)
	})

	t.Run("searching again with a grey disc in hand leaves one disc", func(t *testing.T) {
		w := newQuestWalker(q).at("end")
		w.p.items[inert] = true
		w.notify("command", search(cottage))
		require.Equal(t, []int{inert}, w.ctx.consumedItems)
		require.Equal(t, []int{inert}, w.ctx.givenItems,
			"one taken, one given: searching cannot pile up discs")
	})

	t.Run("a finished player who still holds the woken disc finds nothing more", func(t *testing.T) {
		w := newQuestWalker(q).at("end")
		w.p.items[woken] = true
		w.notify("command", search(cottage))
		require.Empty(t, w.ctx.givenItems)
		require.Empty(t, w.ctx.consumedItems)
		w.notify("command", glow(cottage))
		require.Empty(t, w.ctx.givenItems)
	})

	t.Run("no step listens for a typed noun", func(t *testing.T) {
		for _, tr := range q.Triggers {
			if tr.Event != "room_interact" {
				continue
			}
			require.Equal(t, "stone", tr.Noun, "only Maren's letter, which is no step, still listens for a noun")
			for _, a := range tr.Actions {
				require.Empty(t, a.Grant, "the letter trigger grants no step")
			}
		}
	})

	t.Run("hints fit the cap and name one command", func(t *testing.T) {
		for _, s := range q.Steps {
			if s.Id == "end" {
				continue
			}
			require.LessOrEqual(t, len([]rune(squash(s.Hint))), 200, "step %s hint is over the cap", s.Id)
			require.Equal(t, 1, typedCount(s.Hint), "step %s hint names one command: %q", s.Id, s.Hint)
		}
	})

	// Farmland sky with no lamp: below full sight on most nights. A search still
	// runs in the dark, so the hint carries the one allowed aside.
	t.Run("the search hint says what to do in the dark", func(t *testing.T) {
		require.Equal(t, "start", q.Steps[0].Id)
		require.Contains(t, squash(q.Steps[0].Hint), squash(darkAside))
	})

	t.Run("nothing still sends the player to the Reliquary for the disc", func(t *testing.T) {
		for _, f := range [][]string{{"greenford", "9516.yaml"}, {"eastern_highlands", "9553.yaml"}} {
			raw := strings.ToLower(string(repoFile(t, "_datafiles", "world", "dogmud", "dialogue", f[0], f[1])))
			require.NotContains(t, raw, "reliquary", "%s/%s", f[0], f[1])
			require.NotContains(t, raw, "look disc", "%s/%s", f[0], f[1])
		}
		require.NotContains(t, strings.ToLower(readQuestFile(t, "76-the_disc.yaml")), "reliquary")
	})
}

// refuseGiveCtx is a player too laden to take anything: GiveItem refuses, as
// GameBridge.GiveItem does past the carry limit, and the engine then abandons
// the rest of the trigger's actions.
type refuseGiveCtx struct{ *linearCtx }

func (c *refuseGiveCtx) GiveItem(int) error { return errors.New("carrying too much") }
