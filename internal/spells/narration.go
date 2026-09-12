package spells

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

// Phase selects which of a spell's three narrated moments to render: the cast
// command, each round of the channel, and the moment the magic resolves. One
// line per phase and audience, so the phase IS the selector.
type Phase uint8

const (
	PhaseCast Phase = iota
	PhaseWait
	PhaseMagic
)

// Narration assembles the variants for one phase: the caster's line is the
// Actor, the room's line the Observer. Actee is empty; the target's own line
// is authored in M6.
func (s *SpellData) Narration(p Phase) narration.Variants {
	var caster, room string
	switch p {
	case PhaseCast:
		caster, room = s.CastUserText, s.CastRoomText
	case PhaseWait:
		caster, room = s.WaitUserText, s.WaitRoomText
	case PhaseMagic:
		caster, room = s.MagicUserText, s.MagicRoomText
	}
	return narration.Variants{Actor: textutil.Pool(caster), Observer: textutil.Pool(room)}
}

// Narrate renders one phase with the caster as {source} and the first target,
// if any, as {target}.
func (s *SpellData) Narrate(p Phase, ctx textutil.TokenContext) narration.Roles {
	return textutil.Narrate(s.Narration(p), ctx)
}

// validateNarration refuses a phase whose authored text is whitespace only.
func (s *SpellData) validateNarration() error {
	phases := []struct {
		name string
		p    Phase
	}{{"cast", PhaseCast}, {"wait", PhaseWait}, {"magic", PhaseMagic}}
	for _, ph := range phases {
		v := s.Narration(ph.p)
		if len(v.Actor) == 0 && len(v.Observer) == 0 {
			continue
		}
		if err := narration.ValidateVariants(v, 1); err != nil {
			return fmt.Errorf("spell %s %s text: %w", s.SpellId, ph.name, err)
		}
	}
	return nil
}
