package questengine

import (
	"errors"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
)

// Quest 76, The Disc (#667): the trail runs from the cottage east to the
// man's last camp. The disc is on his remains and wakes at his own
// hearthstone. The Sentinel is the readiness test; the Keeper takes the hand-in.
func TestQuest76TheDisc_Trail(t *testing.T) {
	const cottage, cairn, surveyCamp, lastCamp = 4023, 6349, 6351, 6365
	const survivor, alpha, sentinel = 9535, 9547, 9552
	const inert, woken = 40167, 40168
	q := loadShippedQuest(t, "76-the_disc.yaml", 76)
	require.Equal(t,
		[]string{"start", "journal", "shelter", "cairn", "camp", "alpha", "remains", "attune", "sentinel", "end"},
		newLinearPlayer(q).steps)

	keeper := loadShippedDialogue(t, "eastern_highlands", "9553.yaml")
	in := keeper.FindTopic("disc_in")
	require.NotNil(t, in, "the Keeper takes the hand-in")
	require.Equal(t, dialogue.KindHandIn, in.Kind)
	require.Equal(t, []string{"76-sentinel"}, in.QuestRequired)
	require.Equal(t, "76-end", in.GrantsQuest)
	require.Equal(t, woken, in.CarriesItem)

	search := func(room int) EventDetails { return EventDetails{Command: "search", RoomId: room} }
	// `cast glow` with no target aims at the caster (InitiateCast, TargetSelf).
	glow := func(room int) EventDetails {
		return EventDetails{Command: "cast", Noun: "chrysalis-glow", RoomId: room, TargetKind: "self"}
	}

	t.Run("the file passes the loader, same-command rule included", func(t *testing.T) {
		require.NoError(t, loadShippedQuest(t, "76-the_disc.yaml", 76).Validate())
	})

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", search(lastCamp))
		require.Equal(t, "start", w.p.current, "the camp waits for the trail")
		require.Empty(t, w.ctx.givenItems)
		w.notify("command", search(cottage))
		require.Equal(t, "journal", w.p.current)
		require.Empty(t, w.ctx.givenItems, "the disc went east with him")
		w.notify("topic_heard", EventDetails{MobId: survivor, Topic: "grey_man"})
		require.Equal(t, "shelter", w.p.current)
		w.notify("command", search(cairn))
		require.Equal(t, "cairn", w.p.current)
		w.notify("command", search(surveyCamp))
		require.Equal(t, "camp", w.p.current)
		w.notify("mob_death", EventDetails{MobId: alpha, RoomId: 6353})
		require.Equal(t, "alpha", w.p.current)
		w.notify("command", search(lastCamp))
		require.Equal(t, "remains", w.p.current)
		require.Equal(t, []int{inert}, w.ctx.givenItems)
		w.p.items[inert] = true
		w.notify("command", glow(cottage))
		require.Equal(t, "remains", w.p.current, "his own hearthstone, not the cottage")
		w.notify("command", glow(lastCamp))
		require.Equal(t, "attune", w.p.current)
		require.Contains(t, w.ctx.givenItems, woken)
		require.Contains(t, w.ctx.consumedItems, inert)
		w.notify("mob_death", EventDetails{MobId: sentinel, RoomId: 6371})
		require.Equal(t, "sentinel", w.p.current)
	})

	t.Run("each step waits for the one before it", func(t *testing.T) {
		w := newQuestWalker(q).at("journal")
		w.notify("command", search(cairn))
		require.Equal(t, "journal", w.p.current)
		w.notify("command", search(surveyCamp))
		require.Equal(t, "journal", w.p.current)
		w.notify("mob_death", EventDetails{MobId: alpha, RoomId: 6353})
		require.Equal(t, "journal", w.p.current)
		w.notify("mob_death", EventDetails{MobId: sentinel, RoomId: 6371})
		require.Equal(t, "journal", w.p.current)
		w.notify("command", glow(lastCamp))
		require.Equal(t, "journal", w.p.current)
		require.Empty(t, w.ctx.givenItems)
	})

	t.Run("a glow aimed at someone else wakes nothing", func(t *testing.T) {
		for _, kind := range []string{"mob", "player"} {
			w := newQuestWalker(q).at("remains")
			w.p.items[inert] = true
			w.notify("command", EventDetails{Command: "cast", Noun: "chrysalis-glow", RoomId: lastCamp, TargetKind: kind})
			require.Equal(t, "remains", w.p.current, "aimed at a %s", kind)
		}
	})

	t.Run("a player on an old save carrying a disc gets no second one", func(t *testing.T) {
		w := newQuestWalker(q).at("alpha")
		w.p.items[inert] = true
		w.notify("command", search(lastCamp))
		require.Equal(t, "remains", w.p.current)
		require.Empty(t, w.ctx.givenItems)
	})

	t.Run("an old save at journal holding a disc walks the trail", func(t *testing.T) {
		w := newQuestWalker(q).at("journal")
		w.p.items[inert] = true
		w.notify("topic_heard", EventDetails{MobId: survivor, Topic: "grey_man"})
		require.Equal(t, "shelter", w.p.current)
	})

	t.Run("an old save at attune goes on to the Sentinel", func(t *testing.T) {
		w := newQuestWalker(q).at("attune")
		require.True(t, w.p.HasQuest("76-remains"), "a saved later step counts as past the new ones")
		w.notify("mob_death", EventDetails{MobId: sentinel, RoomId: 6371})
		require.Equal(t, "sentinel", w.p.current)
	})

	t.Run("recovery at the camp grants nothing and leaves one disc", func(t *testing.T) {
		w := newQuestWalker(q).at("end")
		w.notify("command", search(lastCamp))
		require.Equal(t, "end", w.p.current)
		require.Equal(t, []int{inert}, w.ctx.givenItems)
		w.p.items[inert] = true
		w.ctx.givenItems = nil
		w.notify("command", search(lastCamp))
		require.Contains(t, w.ctx.consumedItems, inert, "a second search swaps, it does not pile up")
		w.p.items[inert] = true
		w.notify("command", glow(lastCamp))
		require.Equal(t, "end", w.p.current)
		require.Contains(t, w.ctx.givenItems, woken)
	})

	t.Run("nothing at the cottage gives a disc any more", func(t *testing.T) {
		w := newQuestWalker(q).at("end")
		w.notify("command", search(cottage))
		require.Empty(t, w.ctx.givenItems)
		w.notify("command", glow(cottage))
		require.Empty(t, w.ctx.givenItems)
	})

	// The tests below run against a pack whose items really move, because the
	// triggers on one event are judged in file order against live state: a disc
	// given by an earlier trigger is in hand when a later trigger is judged.
	t.Run("the remains search gives one disc, and does nothing more that command", func(t *testing.T) {
		w, c := newPackWalker(q, "alpha")
		c.notify(w, "command", search(lastCamp))
		require.Equal(t, "remains", w.p.current)
		require.Equal(t, 1, c.held[inert])
		require.Empty(t, w.ctx.consumedItems)
	})

	t.Run("searching the camp again finds nothing more", func(t *testing.T) {
		w, c := newPackWalker(q, "alpha")
		c.notify(w, "command", search(lastCamp))
		c.notify(w, "command", search(lastCamp))
		require.Equal(t, "remains", w.p.current)
		require.Equal(t, 1, c.held[inert])
		require.Len(t, w.ctx.givenItems, 1)
	})

	t.Run("the wake leaves one woken disc and no grey one, and fires no recovery", func(t *testing.T) {
		w, c := newPackWalker(q, "alpha")
		c.notify(w, "command", search(lastCamp))
		c.notify(w, "command", glow(lastCamp))
		require.Equal(t, "attune", w.p.current)
		require.Zero(t, c.held[inert])
		require.Equal(t, 1, c.held[woken])
		require.Equal(t, []int{inert, woken}, w.ctx.givenItems, "nothing handed over twice")
		require.Equal(t, []int{inert}, w.ctx.consumedItems)
	})

	t.Run("no disc in hand at remains: his satchel hands it over, then it wakes", func(t *testing.T) {
		w, c := newPackWalker(q, "remains")
		c.notify(w, "command", glow(lastCamp))
		require.Equal(t, "attune", w.p.current)
		require.Equal(t, []int{inert, woken}, w.ctx.givenItems)
		require.Equal(t, []int{inert}, w.ctx.consumedItems)
		require.Zero(t, c.held[inert])
		require.Equal(t, 1, c.held[woken])
		require.Contains(t, strings.Join(w.ctx.sentTexts, " "), "still in his satchel")
	})

	t.Run("a pack too heavy for the grey disc still moves to remains, and the wake recovers it", func(t *testing.T) {
		w, c := newPackWalker(q, "alpha")
		c.refuse = true
		c.notify(w, "command", search(lastCamp))
		require.Equal(t, "remains", w.p.current, "reading the remains is the step, the disc can wait")
		require.Zero(t, c.held[inert])
		c.refuse = false
		c.notify(w, "command", glow(lastCamp))
		require.Equal(t, "attune", w.p.current)
		require.Equal(t, 1, c.held[woken])
	})

	t.Run("a refused woken disc holds the step, and the story is not told", func(t *testing.T) {
		w, c := newPackWalker(q, "remains")
		c.held[inert], w.p.items[inert] = 1, true
		c.refuse = true
		c.notify(w, "command", glow(lastCamp))
		require.Equal(t, "remains", w.p.current, "no step moves on an undelivered disc")
		require.Empty(t, w.ctx.sentTexts)
		c.refuse = false
		c.notify(w, "command", glow(lastCamp))
		require.Equal(t, "attune", w.p.current, "the cast can be tried again")
		require.Equal(t, 1, c.held[woken])
	})

	t.Run("recovery never piles up discs, however often the camp is searched", func(t *testing.T) {
		for _, step := range []string{"attune", "sentinel", "end"} {
			w, c := newPackWalker(q, step)
			for range 4 {
				c.notify(w, "command", search(lastCamp))
				require.Equal(t, step, w.p.current, "recovery grants nothing")
				require.Equal(t, 1, c.held[inert], "at %s", step)
			}
			c.notify(w, "command", glow(lastCamp))
			require.Equal(t, step, w.p.current, "recovery grants nothing")
			require.Zero(t, c.held[inert], "at %s", step)
			require.Equal(t, 1, c.held[woken], "at %s", step)
		}
	})

	t.Run("a finished player who still holds the woken disc finds nothing more", func(t *testing.T) {
		w, c := newPackWalker(q, "end")
		c.held[woken], w.p.items[woken] = 1, true
		c.notify(w, "command", search(lastCamp))
		c.notify(w, "command", glow(lastCamp))
		require.Empty(t, w.ctx.givenItems)
		require.Empty(t, w.ctx.consumedItems)
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

	t.Run("hints fit the cap and name at most one command", func(t *testing.T) {
		for _, s := range q.Steps {
			if s.Id == "end" {
				continue
			}
			require.LessOrEqual(t, len([]rune(squash(s.Hint))), 200, "step %s hint is over the cap", s.Id)
			require.LessOrEqual(t, typedCount(s.Hint), 1, "step %s hint names one command: %q", s.Id, s.Hint)
		}
	})

	// Farmland sky with no lamp: below full sight on most nights. A search still
	// runs in the dark, so the hint carries the one allowed aside.
	t.Run("the search hint says what to do in the dark", func(t *testing.T) {
		require.Equal(t, "start", q.Steps[0].Id)
		require.Contains(t, squash(q.Steps[0].Hint), squash(darkAside))
	})

	// The cairn, the survey camp and his last camp go fully dark for a bare
	// character at midwinter midnight, and a search in total darkness moves no
	// step (owner ruling 2026-11-02), so the step sending the player there says so.
	t.Run("the steps that send a player to a dark search say to bring a light", func(t *testing.T) {
		for _, s := range q.Steps {
			if s.Id == "shelter" || s.Id == "cairn" || s.Id == "alpha" {
				require.Contains(t, squash(s.Hint), squash("Bring a light after dark."), "step %s", s.Id)
			}
		}
	})

	t.Run("nothing still sends the player to the Reliquary or the cottage for the disc", func(t *testing.T) {
		for _, f := range [][]string{{"greenford", "9516.yaml"}, {"eastern_highlands", "9553.yaml"}} {
			raw := strings.ToLower(string(repoFile(t, "_datafiles", "world", "dogmud", "dialogue", f[0], f[1])))
			require.NotContains(t, raw, "reliquary", "%s/%s", f[0], f[1])
			require.NotContains(t, raw, "look disc", "%s/%s", f[0], f[1])
			require.NotContains(t, raw, "marked hearth", "%s/%s", f[0], f[1])
			require.NotContains(t, raw, "maren's cottage in ashwick for", "%s/%s", f[0], f[1])
		}
		require.NotContains(t, strings.ToLower(readQuestFile(t, "76-the_disc.yaml")), "reliquary")
	})
}

// packCtx is a pack whose items really move: GiveItem and ConsumeItem change
// what the player's next condition reads, as GameBridge does, and held counts
// each item, so a second disc shows where the player's bool item set would hide
// it. refuse is a player too laden to take anything: GiveItem errors, as
// GameBridge.GiveItem does past the carry limit, and the engine then abandons
// the rest of the trigger's actions.
type packCtx struct {
	*linearCtx
	held   map[int]int
	refuse bool
}

func newPackWalker(q *QuestDef, step string) (*questWalker, *packCtx) {
	w := newQuestWalker(q).at(step)
	return w, &packCtx{linearCtx: w.ctx, held: map[int]int{}}
}

func (c *packCtx) GiveItem(id int) error {
	if c.refuse {
		return errors.New("carrying too much")
	}
	if err := c.linearCtx.GiveItem(id); err != nil {
		return err
	}
	c.held[id]++
	c.p.items[id] = true
	return nil
}

func (c *packCtx) ConsumeItem(id int) {
	c.linearCtx.ConsumeItem(id)
	if c.held[id] > 0 {
		c.held[id]--
	}
	if c.held[id] == 0 {
		delete(c.p.items, id)
	}
}

func (c *packCtx) notify(w *questWalker, event string, d EventDetails) {
	d.UserId = 1
	w.p.roomId = d.RoomId
	w.e.Notify(event, d, w.p, c)
}
