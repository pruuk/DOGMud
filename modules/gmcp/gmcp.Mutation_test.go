package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mutations"
)

func TestBuildMutationPayload(t *testing.T) {
	restore := mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"chameleon-skin": {
			MutationId:  "chameleon-skin",
			Name:        "Chameleon Skin",
			Description: "Your skin drinks the colors around it.",
		},
	})
	defer restore()

	p, ok := buildMutationPayload(mutations.Gained{
		UserId: 1, MutationId: "chameleon-skin", Rank: 2, IsNew: false,
	})
	if !ok {
		t.Fatal("expected payload for a registered mutation")
	}
	if p.Id != "chameleon-skin" || p.Name != "Chameleon Skin" || p.Rank != 2 || p.IsNew {
		t.Errorf("payload fields wrong: %+v", p)
	}
	if p.Art != "/static/images/mutations/chameleon-skin.png" {
		t.Errorf("art path = %q", p.Art)
	}
	if p.Description == "" {
		t.Error("description must ship")
	}

	if _, ok := buildMutationPayload(mutations.Gained{MutationId: "nope"}); ok {
		t.Error("unknown mutation must not build a payload")
	}
}

// A silent grant (Bloom Wafer, Phial of Second Birth) narrates itself, so the
// web client gets no mutation card for it, just as the terminal gets no
// reveal (#680). A loud one still pushes the card.
func TestSilentMutationGainPushesNoCard(t *testing.T) {
	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"chameleon-skin": {MutationId: "chameleon-skin", Name: "Chameleon Skin"},
	}))
	events.DrainQueuedByTypeForTest(`GMCPOut`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`GMCPOut`) })
	g := GMCPMutationModule{}

	g.onMutationGained(mutations.Gained{UserId: 6801, MutationId: "chameleon-skin", Rank: 1, IsNew: true, Silent: true})
	if sent := drainGMCPOut(t); len(sent) != 0 {
		t.Fatalf("a silent grant pushed %d frames: %+v", len(sent), sent)
	}

	g.onMutationGained(mutations.Gained{UserId: 6801, MutationId: "chameleon-skin", Rank: 1, IsNew: true})
	sent := drainGMCPOut(t)
	if len(sent) != 1 || sent[0].Module != `Char.Mutation` {
		t.Fatalf("a loud grant must push one Char.Mutation frame, got %+v", sent)
	}
}
