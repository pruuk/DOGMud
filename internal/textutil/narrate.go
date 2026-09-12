package textutil

import "github.com/GoMudEngine/GoMud/internal/narration"

// Pool is a single-variant pool: nil for an empty string, otherwise the one
// line. Whitespace is kept, not trimmed, so narration.ValidateVariants can
// refuse a whitespace-only authored line at load instead of a site sending it.
func Pool(text string) []string {
	if text == "" {
		return nil
	}
	return []string{text}
}

// Narrate renders one single-variant event for its audiences.
//
// It is the ONLY way the Kind B stores (buffs, spells, quests) reach
// narration.Render, and it always passes narration.FirstPicker: these stores
// hold one line per phase, so there is nothing to choose, and the default
// picker would consume a global random draw per narrated phase (see
// FirstPicker). The root guard narration_render_callers_guard_test.go pins
// both facts. An empty pool renders nothing without building the token map;
// the buff tick calls this every round for every buffed character, and most
// buffs have no trigger text.
func Narrate(v narration.Variants, ctx TokenContext) narration.Roles {
	if v.Len() == 0 {
		return narration.Roles{}
	}
	return narration.Render(v, ctx.Tokens(), narration.FirstPicker)
}
