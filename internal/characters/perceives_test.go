package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
)

const perceivesVeilBuffId = 7301

func perceivesChar(t *testing.T, name string) *Character {
	t.Helper()
	c := New()
	c.Name = name
	c.Awareness = awareness.NewMachine()
	return c
}

// perceivesHide puts c into the Awareness Hidden state, the only thing
// IsHidden reads. Concealing then resolving is the only route into it.
func perceivesHide(t *testing.T, c *Character) {
	t.Helper()
	reason := state.TransitionReason{Trigger: "perceives_test"}
	if err := c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason); err != nil {
		t.Fatalf("concealing: %v", err)
	}
	c.Awareness.ResolveConcealment(true, reason)
	if !c.IsHidden() {
		t.Fatal("precondition: the fixture should now be hidden")
	}
}

func TestPerceives(t *testing.T) {
	t.Run("a creature that is not hidden", func(t *testing.T) {
		if !perceivesChar(t, "Viewer").Perceives(perceivesChar(t, "Kesh")) {
			t.Fatal("anyone perceives a creature that is not hidden")
		}
	})

	t.Run("a hidden creature, no see-hidden", func(t *testing.T) {
		kesh := perceivesChar(t, "Kesh")
		perceivesHide(t, kesh)
		if perceivesChar(t, "Viewer").Perceives(kesh) {
			t.Fatal("a hidden creature must not be perceived without see-hidden")
		}
	})

	t.Run("yourself, while hidden", func(t *testing.T) {
		kesh := perceivesChar(t, "Kesh")
		perceivesHide(t, kesh)
		if !kesh.Perceives(kesh) {
			t.Fatal("a hider always perceives themselves")
		}
	})

	t.Run("see-hidden from a buff, with no pet", func(t *testing.T) {
		t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
			perceivesVeilBuffId: {BuffId: perceivesVeilBuffId, Name: "Test Veil", Flags: []buffs.Flag{buffs.SeeHidden}},
		}))
		viewer := perceivesChar(t, "Viewer")
		if err := viewer.AddBuff(perceivesVeilBuffId, true); err != nil {
			t.Fatalf("applying see-hidden: %v", err)
		}
		if viewer.Pet.Exists() {
			t.Fatal("precondition: the viewer must have no pet")
		}
		kesh := perceivesChar(t, "Kesh")
		perceivesHide(t, kesh)
		if !viewer.Perceives(kesh) {
			t.Fatal("see-hidden alone reveals a hidden creature; no pet is involved")
		}
	})

	t.Run("see-hidden from a mutation", func(t *testing.T) {
		t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
			"test-eyes": {MutationId: "test-eyes", Name: "Test Eyes",
				Pros: []mutations.MutationEffect{{Type: "flag", Target: string(buffs.SeeHidden), Value: 1}}},
		}))
		viewer := perceivesChar(t, "Viewer")
		viewer.Mutations = map[string]int{"test-eyes": 1}
		kesh := perceivesChar(t, "Kesh")
		perceivesHide(t, kesh)
		if !viewer.Perceives(kesh) {
			t.Fatal("a see-hidden mutation reveals a hidden creature")
		}
	})

	t.Run("nothing", func(t *testing.T) {
		if perceivesChar(t, "Viewer").Perceives(nil) {
			t.Fatal("a nil creature is not perceived")
		}
	})
}
