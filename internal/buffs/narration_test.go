package buffs

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

func glowSpec() *BuffSpec {
	return &BuffSpec{BuffId: 500, Name: "Glow", StartRoomText: "A glow surrounds {source}.", EndUserText: "The glow fades.", TriggerUserText: "You shimmer."}
}

func TestNarrationStartPutsTheHolderInActeeAndUsesTheNotice(t *testing.T) {
	v := glowSpec().Narration(PhaseStart)
	if len(v.Actor) != 0 {
		t.Fatalf("Actor must stay empty (reserved for the caster), got %v", v.Actor)
	}
	if len(v.Actee) != 1 || v.Actee[0] != "Glow takes effect." {
		t.Fatalf("Actee should be the generic notice, got %v", v.Actee)
	}
	if len(v.Observer) != 1 || v.Observer[0] != "A glow surrounds {source}." {
		t.Fatalf("Observer should be the raw room line, got %v", v.Observer)
	}
}

func TestNarrationTriggerAndEnd(t *testing.T) {
	s := glowSpec()
	if v := s.Narration(PhaseTrigger); len(v.Actee) != 1 || v.Actee[0] != "You shimmer." || len(v.Observer) != 0 {
		t.Fatalf("trigger: %+v", v)
	}
	if v := s.Narration(PhaseEnd); len(v.Actee) != 1 || v.Actee[0] != "The glow fades." || len(v.Observer) != 0 {
		t.Fatalf("end: %+v", v)
	}
}

func TestNarrationSecretBuffHasNoHolderLine(t *testing.T) {
	s := glowSpec()
	s.Secret = true
	if v := s.Narration(PhaseStart); len(v.Actee) != 0 {
		t.Fatalf("a secret buff must not narrate to its holder, got %v", v.Actee)
	}
}

func TestNarrateSubstitutesTheHolderName(t *testing.T) {
	roles := glowSpec().Narrate(PhaseStart, textutil.TokenContext{SourceName: "Aliceia", SourcePlainName: "Aliceia"})
	if roles.Actee != "Glow takes effect." || roles.Observer != "A glow surrounds Aliceia." || roles.Actor != "" {
		t.Fatalf("roles: %+v", roles)
	}
}

func TestNarrateAPhaseWithNoTextRendersNothing(t *testing.T) {
	s := &BuffSpec{BuffId: 501, Name: "Quiet", Secret: true}
	if v := s.Narration(PhaseStart); v.Len() != 0 {
		t.Fatalf("expected no variants, got %+v", v)
	}
	if roles := s.Narrate(PhaseStart, textutil.TokenContext{}); roles != (narration.Roles{}) {
		t.Fatalf("expected zero roles, got %+v", roles)
	}
}

func TestAuthoredStartLineIgnoresTheSilentStartRule(t *testing.T) {
	s := &BuffSpec{BuffId: 502, Name: "Sleeping", Flags: []Flag{SilentStart}, StartUserText: "You lie down, {source_plain}."}
	if got := s.StartUserNotice(); got != "" {
		t.Fatalf("notice should be silent for silent-start, got %q", got)
	}
	if got := s.AuthoredStartLine(textutil.TokenContext{SourcePlainName: "Aliceia"}); got != "You lie down, Aliceia." {
		t.Fatalf("AuthoredStartLine = %q", got)
	}
}

func TestValidateRefusesAWhitespaceOnlyLine(t *testing.T) {
	s := glowSpec()
	s.TriggerRoomText = "   "
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "trigger") {
		t.Fatalf("expected a trigger-phase validation error, got %v", err)
	}
	s.TriggerRoomText = ""
	if err := s.Validate(); err != nil {
		t.Fatalf("a spec with ordinary text must validate, got %v", err)
	}
}

func TestValidateReadsTheRawStartLineEvenWhenTheNoticeIsSilent(t *testing.T) {
	s := &BuffSpec{BuffId: 503, Name: "Sleeping", Flags: []Flag{SilentStart}, StartUserText: "   "}
	if got := s.StartUserNotice(); got != "" {
		t.Fatalf("precondition: the notice must be silent for silent-start, got %q", got)
	}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "start") {
		t.Fatalf("a whitespace-only start line must be refused even when the notice hides it, got %v", err)
	}
}
