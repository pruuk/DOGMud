package combat

import (
	"maps"

	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/narration"
)

// grappleMoveIdentities carries the PLAIN display forms grapple's two
// combat-package events interpolate. Both events' source lines in
// grapple.yaml name their parties bare (no identity ansi tag): crit_failure's
// actor line names nobody at all, and disarm's original Sprintf calls used
// target.Name / source.Name untagged. There is no tagged-name field here
// because neither event needs one.
type grappleMoveIdentities struct {
	ActorPlain string
	ActeePlain string
}

// renderGrappleEvent renders one grapple event from the shipped
// movenarration store.
//
// This duplicates the shape of mobcommands.renderMoveEvent rather than
// calling it: internal/mobcommands already imports internal/combat (for
// GrappleMoveResult and its siblings), so the reverse import would be a
// cycle, and renderMoveEvent is unexported besides.
func renderGrappleEvent(event movenarration.EventKey, ids grappleMoveIdentities, extra map[string]string) (narration.Roles, bool) {
	g := movenarration.GetMove("grapple")
	if g == nil {
		mudlog.Error("movenarration", "error", "store not loaded", "verb", "grapple", "event", string(event))
		return narration.Roles{}, false
	}
	v, ok := g.Variants(event)
	if !ok {
		mudlog.Error("movenarration", "error", "unknown event", "verb", "grapple", "event", string(event))
		return narration.Roles{}, false
	}
	tokens := map[string]string{
		narration.TokenActorPlain: ids.ActorPlain,
		narration.TokenActeePlain: ids.ActeePlain,
	}
	maps.Copy(tokens, extra)
	return narration.Render(v, tokens, narration.DefaultPicker), true
}
