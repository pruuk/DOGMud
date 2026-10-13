package crafting

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

// Phase selects which of a recipe's two narrated outcomes to render. One line
// per outcome and audience, so the phase IS the selector.
type Phase uint8

const (
	PhaseSuccess Phase = iota
	PhaseFailure
	// PhaseStart is a craft beginning (messaging M6 row 7). It has no recipe
	// text: Narration returns empty variants for it, and the room's start line
	// comes from the crafting-messages store.
	PhaseStart
	// PhaseCancel is a craft that ended without finishing (R4): cancelled,
	// walked away from, or interrupted by damage. Like PhaseStart it has no
	// recipe text; its lines come from the crafting-messages store.
	PhaseCancel
)

// Narration assembles the variants for one outcome: the crafter's line is the
// Actor, the room's line the Observer. There is no Actee: a craft has no second
// party today. Enchanting another player's gear would add one here, as one
// optional key and one more field in this literal.
func (r *RecipeSpec) Narration(p Phase) narration.Variants {
	var crafter, room string
	switch p {
	case PhaseSuccess:
		crafter, room = r.SuccessMessage, r.SuccessRoomMessage
	case PhaseFailure:
		crafter, room = r.FailureMessage, r.FailureRoomMessage
	}
	return narration.Variants{Actor: textutil.Pool(crafter), Observer: textutil.Pool(room)}
}

// Narrate renders one outcome with the crafter as the actor. A craft has no
// actee, so {actee} renders empty.
func (r *RecipeSpec) Narrate(p Phase, ctx textutil.TokenContext) narration.Roles {
	return textutil.Narrate(r.Narration(p), ctx)
}

// ValidateNarrationTokens reports unknown tokens in this recipe's four
// narration fields. Crafting was the only store with no token check at all
// (messaging arc M4a), so a typo shipped silently and rendered raw to the
// player.
func (r RecipeSpec) ValidateNarrationTokens() []string {
	var problems []string
	for _, text := range []string{
		r.SuccessMessage, r.SuccessRoomMessage,
		r.FailureMessage, r.FailureRoomMessage,
	} {
		problems = append(problems, textutil.ValidateTokens(text)...)
	}
	return problems
}

// validateNarration is called from Validate, so a violation fails the load.
//
// Both crafter lines are required: every site sends the Actor line
// unconditionally, as it did before the door existed, and all 126 shipped
// recipes set both. A room line must name the crafter with {actor}, the same
// rule quests.RoomTextProblems applies to a quest's observer line: the room is
// watching someone work, and a subjectless line cannot say who.
func (r *RecipeSpec) validateNarration() error {
	if r.SuccessMessage == "" {
		return fmt.Errorf("recipe %q: success_actor cannot be empty", r.RecipeId)
	}
	if r.FailureMessage == "" {
		return fmt.Errorf("recipe %q: failure_actor cannot be empty", r.RecipeId)
	}
	phases := []struct {
		name string
		p    Phase
		room string
	}{
		{"success", PhaseSuccess, r.SuccessRoomMessage},
		{"failure", PhaseFailure, r.FailureRoomMessage},
	}
	for _, ph := range phases {
		// No expected role set: the Observer is deliberately optional until M6
		// authors it, so no fixed shape exists to declare. The blank-variant
		// check is what this call is for.
		if err := narration.ValidateVariants(r.Narration(ph.p), 1); err != nil {
			return fmt.Errorf("recipe %q %s text: %w", r.RecipeId, ph.name, err)
		}
		if ph.room != "" && !strings.Contains(ph.room, narration.TokenActor) {
			return fmt.Errorf("recipe %q: %s_observer must name the crafter with %s (the room is watching them work)", r.RecipeId, ph.name, narration.TokenActor)
		}
	}
	// An unknown token fails the load rather than warning: the loader turns this
	// error into a boot panic (crafting.LoadRecipeFiles), so a typo cannot reach
	// a player as raw text.
	if problems := r.ValidateNarrationTokens(); len(problems) > 0 {
		return fmt.Errorf("recipe %q: %s", r.RecipeId, strings.Join(problems, "; "))
	}
	return nil
}
