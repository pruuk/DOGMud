package spells

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

func boltSpec() *SpellData {
	// PrimaryStat is required by Validate (U9 made it load-bearing).
	return &SpellData{SpellId: "bolt", Name: "Bolt", PrimaryStat: "willpower", CastUserText: "You gather a bolt.", CastRoomText: "{source} gathers a bolt at {target}.", WaitUserText: "You hold the bolt."}
}

func TestNarrationCastPutsTheCasterInActor(t *testing.T) {
	v := boltSpec().Narration(PhaseCast)
	if len(v.Actor) != 1 || v.Actor[0] != "You gather a bolt." {
		t.Fatalf("Actor: %v", v.Actor)
	}
	if len(v.Observer) != 1 || v.Observer[0] != "{source} gathers a bolt at {target}." {
		t.Fatalf("Observer: %v", v.Observer)
	}
	if len(v.Actee) != 0 {
		t.Fatalf("Actee must be empty (M6 authors it), got %v", v.Actee)
	}
}

func TestNarrationWaitAndMagic(t *testing.T) {
	s := boltSpec()
	if v := s.Narration(PhaseWait); len(v.Actor) != 1 || v.Actor[0] != "You hold the bolt." || len(v.Observer) != 0 {
		t.Fatalf("wait: %+v", v)
	}
	if v := s.Narration(PhaseMagic); v.Len() != 0 {
		t.Fatalf("magic has no text, got %+v", v)
	}
}

func TestNarrateSubstitutesSourceAndTarget(t *testing.T) {
	roles := boltSpec().Narrate(PhaseCast, textutil.TokenContext{SourceName: "Kael", TargetName: "Goblin"})
	if roles.Actor != "You gather a bolt." || roles.Observer != "Kael gathers a bolt at Goblin." {
		t.Fatalf("roles: %+v", roles)
	}
	if roles := boltSpec().Narrate(PhaseMagic, textutil.TokenContext{}); roles != (narration.Roles{}) {
		t.Fatalf("magic should render nothing, got %+v", roles)
	}
}

func TestValidateRefusesAWhitespaceOnlyLine(t *testing.T) {
	s := boltSpec()
	s.WaitRoomText = " "
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "wait") {
		t.Fatalf("expected a wait-phase validation error, got %v", err)
	}
	s.WaitRoomText = ""
	if err := s.Validate(); err != nil {
		t.Fatalf("ordinary text must validate, got %v", err)
	}
}
