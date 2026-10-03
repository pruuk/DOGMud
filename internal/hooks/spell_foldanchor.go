package hooks

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

func resolveFoldAnchor(actor actions.Actor) {
	char := actor.GetCharacter()
	if char == nil {
		return
	}
	roomId := char.RoomId
	if rooms.NoRecall(roomId) {
		// An anchor here would outlive the place (a rift's rooms are reused
		// by others' runs): it will not take.
		actor.SendText(messaging.CategorySpellFold, `The anchor will not take hold here. This place does not keep anything of yours.`)
		return
	}
	char.SetMiscData("fold-anchor-room", roomId)

	actor.SendText(messaging.CategorySpellFold, `A Chrysalis anchor locks into place here. `+
		`Cast <ansi fg="command">fold-recall</ansi> from elsewhere to return.`)

	if room := actor.GetRoom(); room != nil {
		room.SendTextVisual(messaging.CategorySpellFold, fmt.Sprintf(
			`A faint shimmer marks where <ansi fg="username">%s</ansi> has set an anchor.`,
			actor.GetName()), actor.GetUserId())
	}
}
