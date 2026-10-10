package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mutations"
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

// #481: a new tail pushes worn leg armour into the pack, and the player is
// told which item and why, once.
func TestMutationGainedReveal_TellsThePlayerTheTailDisplacedTheirLegArmour(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"tail": {MutationId: "tail", Name: "Tail", Description: "A tail grows.",
			Pros: []mutations.MutationEffect{{Type: "flag", Target: "tail", Value: 1}},
			Cons: []mutations.MutationEffect{{Type: "flag", Target: "disable-legs", Value: 1}},
		},
	})()
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999482: {ItemId: 999482, Name: "iron greaves", Type: items.Legs, Weight: 1},
	})()

	u := users.GetByUserId(1)
	require.NotNil(t, u)
	origLegs, origMuts := u.Character.Equipment.Legs, u.Character.Mutations
	t.Cleanup(func() {
		u.Character.Equipment.Legs, u.Character.Mutations = origLegs, origMuts
		u.Character.RemoveItem(items.New(999482))
	})
	u.Character.SetUserId(u.UserId) // as every production load path does
	u.Character.Equipment.Legs = items.New(999482)
	// As the round tick does: the mutation lands, then the Gained event.
	u.Character.Mutations = map[string]int{"tail": 1}

	captured, capCleanup := captureMessages(t)
	MutationGained_Reveal(mutations.Gained{UserId: 1, MutationId: "tail", Rank: 1, IsNew: true})
	capCleanup()

	told := 0
	for _, m := range *captured {
		if m.UserId == 1 && strings.Contains(m.Text, "Your new tail leaves no room for") &&
			strings.Contains(strings.ToLower(m.Text), "iron greaves") && strings.Contains(m.Text, "so you take it off.") {
			told++
		}
	}
	assert.Equal(t, 1, told, "the player is told once which item the tail displaced")
	_, inPack := u.Character.FindInBackpack("iron greaves")
	assert.True(t, inPack, "the greaves are in the pack, not destroyed")
	assert.True(t, u.Character.Equipment.Legs.IsDisabled())
}

func TestFlattenDescription(t *testing.T) {
	in := "Your breathing reorganizes into a lattice of insectile spiracles --\ntireless,\nand indifferent to bad air.\n"
	got := flattenDescription(in)
	assert.NotContains(t, got, "\n")
	assert.Contains(t, got, "spiracles -- tireless, and indifferent")
}
