package conversation

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Give resolves `give <item> <npc>` against the NPC's topics before any
// transfer (R8). An eligible topic wanting the item plays exactly as if
// chosen from the menu (resolve): an offer pitches and shows its Accept /
// Leave card, taking the item only on Accept; any other topic plays with the
// same speech, handover and private lines, then the menu is re-shown (which
// OpenMenu itself withholds while a gate rule holds or a prompt is open). A wanted item with no eligible topic is handed
// back and never leaves the player. It returns false, changing nothing, when
// no topic on the NPC wants the item or the NPC is charmed (charmed NPCs
// offer no topics), and the caller runs the ordinary give.
//
// give is a plain command with its own refusals in give.go (sleep, sight),
// so an open menu or prompt does not stop it.
func Give(user *users.UserRecord, room *rooms.Room, mob *mobs.Mob, itm items.Item) bool {
	if mob == nil || mob.Character.IsCharmed() {
		return false
	}
	df := dialogue.Load(int(mob.MobId), mob.Zone)
	if df == nil {
		return false
	}
	buf := &lineBuffer{user: user}
	ps := talkState(user, buf)
	t, wanted := df.ItemTopic(itm.ItemId, ps)
	if !wanted {
		return false
	}
	if t == nil {
		sight := messaging.ParticipantSight(user.Character, room)
		user.SendText(messaging.CategorySystem, messaging.HideNames(fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> looks at the <ansi fg="item">%s</ansi> and hands it back.`,
			mob.Character.Name, itm.DisplayName()), []string{mob.Character.Name}, sight))
		return true
	}
	resolve(user, room, mob, df, t, ps, buf)
	return true
}
