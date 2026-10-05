package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/gather"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// WearUsedTool records one finished job on the tool a job used and tells the
// actor when it gives out (wilderness trades review: tools wear and break).
// has is false when no tool was used; nothing happens then.
func WearUsedTool(actor Actor, t gather.Tool, has bool) {
	if !has || actor == nil {
		return
	}
	name := gather.WearTool(actor.GetCharacter(), t)
	if name == `` {
		return
	}
	actor.SendText(messaging.CategoryWarning, fmt.Sprintf(
		`<ansi fg="red">Your <ansi fg="itemname">%s</ansi> gives out: the edge is gone and the haft has split. It is no use until it is repaired.</ansi> (<ansi fg="command">help repair</ansi>)`, name))
}
