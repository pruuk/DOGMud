package buffs

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

// Phase selects which of a buff's three narrated moments to render. A buff
// holds one line per phase and audience, so the phase IS the selector: there
// is no pool and nothing to pick.
type Phase uint8

const (
	PhaseStart Phase = iota
	PhaseTrigger
	PhaseEnd
)

// Narration assembles the variants for one phase.
//
// The holder's line is the ACTEE: the buff happens to them. The room's line is
// the Observer. Actor is empty and reserved for the caster, which M6 authors
// once events.Buff carries a caster (owner ruling, 2026-09-12). Start and End
// go through StartUserNotice / EndUserNotice, so the secret, hidden,
// silent-start and generic-fallback rules stay in their one door.
func (b *BuffSpec) Narration(p Phase) narration.Variants {
	var holder, room string
	switch p {
	case PhaseStart:
		holder, room = b.StartUserNotice(), b.StartRoomText
	case PhaseTrigger:
		holder, room = b.TriggerUserText, b.TriggerRoomText
	case PhaseEnd:
		holder, room = b.EndUserNotice(), b.EndRoomText
	}
	return narration.Variants{Actee: textutil.Pool(holder), Observer: textutil.Pool(room)}
}

// Narrate renders one phase for its audiences with the holder as {source}.
func (b *BuffSpec) Narrate(p Phase, ctx textutil.TokenContext) narration.Roles {
	return textutil.Narrate(b.Narration(p), ctx)
}

// AuthoredStartLine renders start_user_text as written, ignoring the notice
// rules. It is the door for the applier of a silent-start buff, which narrates
// the start itself because the buff never travels the event that would:
// sleep (15), arrest (88), stun (84) and broken limb (83).
func (b *BuffSpec) AuthoredStartLine(ctx textutil.TokenContext) string {
	return textutil.SubstituteTokens(b.StartUserText, ctx)
}

// validateNarration refuses a phase whose authored text cannot be rendered:
// a whitespace-only line, which ValidateVariants reports as an empty variant.
// It checks the RAW fields, not the notices, so a silent-start buff's hidden
// start line is checked too.
func (b *BuffSpec) validateNarration() error {
	phases := []struct{ name, user, room string }{
		{"start", b.StartUserText, b.StartRoomText},
		{"trigger", b.TriggerUserText, b.TriggerRoomText},
		{"end", b.EndUserText, b.EndRoomText},
	}
	for _, ph := range phases {
		if ph.user == "" && ph.room == "" {
			continue
		}
		v := narration.Variants{Actee: textutil.Pool(ph.user), Observer: textutil.Pool(ph.room)}
		if err := narration.ValidateVariants(v, 1); err != nil {
			return fmt.Errorf("buffId %d (%s) %s text: %w", b.BuffId, b.Name, ph.name, err)
		}
	}
	return nil
}
