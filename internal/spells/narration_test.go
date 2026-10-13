package spells

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

func boltSpec() *SpellData {
	// PrimaryStat is required by Validate (U9 made it load-bearing), as are
	// the three axes (validateAxes).
	return &SpellData{
		SpellId: "bolt", Name: "Bolt", PrimaryStat: "willpower",
		AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamageMental, Targeting: combatvocab.TargetSingle,
		CastUserText: "You gather a bolt.", CastRoomText: "{actor} gathers a bolt at {actee}.", CastActeeText: "{actor} gathers a bolt at you.", WaitUserText: "You hold the bolt.",
	}
}

func TestNarrationCastPutsTheCasterInActor(t *testing.T) {
	v := boltSpec().Narration(PhaseCast)
	if len(v.Actor) != 1 || v.Actor[0] != "You gather a bolt." {
		t.Fatalf("Actor: %v", v.Actor)
	}
	if len(v.Observer) != 1 || v.Observer[0] != "{actor} gathers a bolt at {actee}." {
		t.Fatalf("Observer: %v", v.Observer)
	}
	if len(v.Actee) != 1 || v.Actee[0] != "{actor} gathers a bolt at you." {
		t.Fatalf("Actee: %v", v.Actee)
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
	roles := boltSpec().Narrate(PhaseCast, textutil.TokenContext{ActorName: "Kael", ActeeName: "Goblin"})
	if roles.Actor != "You gather a bolt." || roles.Observer != "Kael gathers a bolt at Goblin." {
		t.Fatalf("roles: %+v", roles)
	}
	if roles.Actee != "Kael gathers a bolt at you." {
		t.Fatalf("actee: %q", roles.Actee)
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

// The target's own line is a cast-phase line only (messaging M6 slice 4).
func TestNarrationActeeIsCastPhaseOnly(t *testing.T) {
	if v := boltSpec().Narration(PhaseWait); len(v.Actee) != 0 {
		t.Fatalf("wait carries an actee: %v", v.Actee)
	}
}

// A whitespace-only cast_actee is refused even when the phase has no other
// line, so it cannot slip past the "phase not authored" skip.
func TestValidateRefusesAWhitespaceOnlyActeeLine(t *testing.T) {
	s := boltSpec()
	s.CastUserText, s.CastRoomText, s.CastActeeText = "", "", " "
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "cast") {
		t.Fatalf("expected a cast-phase validation error, got %v", err)
	}
}

// An unknown token is a boot failure, not a warning (messaging arc M4a).
// Before the promotion this spell loaded and the player saw "{actae}" raw.
func TestValidateRefusesAnUnknownToken(t *testing.T) {
	s := boltSpec()
	s.CastRoomText = "{actae} gathers a bolt."
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "{actae}") {
		t.Fatalf("expected an unknown-token validation error naming {actae}, got %v", err)
	}
}
