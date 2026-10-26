package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/splash"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type notAGainedEvent struct{}

func (notAGainedEvent) Type() string { return "dummy" }

func TestMutationGainedRevealWrongEventType(t *testing.T) {
	assert.Equal(t, events.Continue, MutationGained_Reveal(notAGainedEvent{}))
}

func TestDeepenFlourishText(t *testing.T) {
	got := deepenFlourishText("Chameleon Skin", 2, 3)
	assert.Contains(t, got, "Chameleon Skin")
	assert.Contains(t, got, "Level 2")
	assert.NotContains(t, got, "fully matured")

	got = deepenFlourishText("Chameleon Skin", 3, 3)
	assert.Contains(t, got, "fully matured")
}

func TestRevealCaption(t *testing.T) {
	got := revealCaption("Chameleon Skin", "Your skin drinks the colors around it.")
	// The caption is the screen-reader / degraded path — plain text, no ansi tags.
	assert.Contains(t, got, "A mutation emerges: Chameleon Skin.")
	assert.Contains(t, got, "Your skin drinks the colors around it.")
	assert.NotContains(t, got, "<ansi")
}

// displacedGearFixture seeds a tail mutation and greaves, puts the greaves on
// user 1, registers the listeners the real server wires for this flow
// (reveal, splash delivery, the gear notice) and captures every message.
// gained says whether the grant queues mutations.Gained, as the round tick,
// pinnacle tick and quest grants do; the Bloom Wafer and the Phial of Second
// Birth do not.
func displacedGearFixture(t *testing.T, splashes bool) (*users.UserRecord, *[]events.Message) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"tail": {MutationId: "tail", Name: "Tail", Description: "A tail grows.",
			Pros: []mutations.MutationEffect{{Type: "flag", Target: "tail", Value: 1}},
			Cons: []mutations.MutationEffect{{Type: "flag", Target: "disable-legs", Value: 1}},
		},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		999482: {ItemId: 999482, Name: "iron greaves", Type: items.Legs, Weight: 1},
	}))
	cfg := configs.GetConfig()
	cfg.GamePlay.SplashesEnabled = configs.ConfigBool(splashes)
	configs.SetConfigForTest(t, cfg)

	u := users.GetByUserId(1)
	require.NotNil(t, u)
	origLegs, origMuts := u.Character.Equipment.Legs, u.Character.Mutations
	t.Cleanup(func() {
		u.Character.Equipment.Legs, u.Character.Mutations = origLegs, origMuts
		u.Character.RemoveItem(items.New(999482))
		u.Character.TakeDisplacedGear()
	})
	u.Character.SetUserId(u.UserId) // as every production load path does
	u.Character.Equipment.Legs = items.New(999482)

	events.ProcessEvents() // start from an empty queue
	ids := []struct {
		e  events.Event
		id events.ListenerId
	}{
		{mutations.Gained{}, events.RegisterListener(mutations.Gained{}, MutationGained_Reveal)},
		{splash.Splash{}, events.RegisterListener(splash.Splash{}, Splash_Deliver)},
		{displacedGearNotice{}, events.RegisterListener(displacedGearNotice{}, DisplacedGear_Notify)},
	}
	t.Cleanup(func() {
		for _, l := range ids {
			events.UnregisterListener(l.e, l.id)
		}
	})
	captured, capCleanup := captureMessages(t)
	t.Cleanup(capCleanup)
	return u, captured
}

// indexOfMessage returns the index of the first captured message to user 1
// containing needle, or -1.
func indexOfMessage(msgs []events.Message, needle string) int {
	for i, m := range msgs {
		if m.UserId == 1 && strings.Contains(m.Text, needle) {
			return i
		}
	}
	return -1
}

const gearNeedle = "Your new tail leaves no room for"

func countMessages(msgs []events.Message, needle string) int {
	n := 0
	for _, m := range msgs {
		if m.UserId == 1 && strings.Contains(m.Text, needle) {
			n++
		}
	}
	return n
}

// #481: a new tail pushes worn leg armour into the pack, and the player is
// told which item and why, once, AFTER the reveal: on the splash path (which
// ships on) the reveal is a queued splash, so the gear line must not jump it.
// The flow is the round tick's: the mutation lands and Gained is queued,
// then the end-of-tick validate displaces the greaves and queues the notice.
func TestDisplacedGear_TailToldOnceAfterTheReveal(t *testing.T) {
	for _, splashes := range []bool{true, false} {
		t.Run(map[bool]string{true: "splash", false: "no splash"}[splashes], func(t *testing.T) {
			u, captured := displacedGearFixture(t, splashes)

			u.Character.Mutations = map[string]int{"tail": 1}
			events.AddToQueue(mutations.Gained{UserId: 1, MutationId: "tail", Rank: 1, IsNew: true})
			endOfRoundValidate(u)
			events.ProcessEvents()

			// The splash path delivers the art (category splash); the
			// no-splash path sends the classic announcement.
			revealNeedle := "A mutation emerges"
			if splashes {
				revealNeedle = `<ansi fg="splash">`
			}
			reveal := indexOfMessage(*captured, revealNeedle)
			gear := indexOfMessage(*captured, gearNeedle)
			require.GreaterOrEqual(t, reveal, 0, "the reveal was sent")
			require.GreaterOrEqual(t, gear, 0, "the gear line was sent")
			assert.Greater(t, gear, reveal, "the gear line must follow the reveal")
			assert.Equal(t, 1, countMessages(*captured, gearNeedle))
			assert.Contains(t, strings.ToLower((*captured)[gear].Text), "iron greaves")
			_, inPack := u.Character.FindInBackpack("iron greaves")
			assert.True(t, inPack, "the greaves are in the pack, not destroyed")
			assert.True(t, u.Character.Equipment.Legs.IsDisabled())
		})
	}
}

// A grant that queues no Gained event (the Bloom Wafer's
// BloomSeedNewMutation, the Phial of Second Birth's GrantRandomMutationRare)
// still tells the player: the end-of-tick validate every player reaches is
// the drain point, not the reveal.
func TestDisplacedGear_ToldOnAGrantWithNoGainedEvent(t *testing.T) {
	u, captured := displacedGearFixture(t, true)

	u.Character.Mutations = map[string]int{"tail": 1} // no Gained queued
	endOfRoundValidate(u)
	events.ProcessEvents()

	assert.Equal(t, 1, countMessages(*captured, gearNeedle))
	assert.Equal(t, -1, indexOfMessage(*captured, `<ansi fg="splash">`), "no reveal on this path")

	// Nothing stale is left to be told at a later, unrelated reveal.
	endOfRoundValidate(u)
	events.ProcessEvents()
	assert.Equal(t, 1, countMessages(*captured, gearNeedle))
}

// Each cause has its own wording.
func TestDisplacedGearText_NamesTheCause(t *testing.T) {
	cases := map[string]string{
		characters.DisplacedByDisabledLegs:  "Your new tail leaves no room for",
		characters.DisplacedByLostTail:      "Your tail is gone",
		characters.DisplacedByLostArm:       "Your extra arm is gone",
		characters.DisplacedBySpecies:       "Your body has no place for",
		characters.DisplacedByTwoHandedGrip: "Your weapon needs both hands again",
	}
	for reason, want := range cases {
		got := displacedGearText(characters.DisplacedGear{ItemName: "Iron Greaves", Reason: reason})
		assert.Contains(t, got, want, reason)
		assert.Contains(t, got, "Iron Greaves", reason)
	}
}

func TestFlattenDescription(t *testing.T) {
	in := "Your breathing reorganizes into a lattice of insectile spiracles --\ntireless,\nand indifferent to bad air.\n"
	got := flattenDescription(in)
	assert.NotContains(t, got, "\n")
	assert.Contains(t, got, "spiracles -- tireless, and indifferent")
}
