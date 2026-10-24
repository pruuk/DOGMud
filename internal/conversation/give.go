package conversation

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
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
// OpenMenu itself withholds while a gate rule holds or a prompt is open). A
// wanted item with no eligible topic is handed back and never leaves the
// player (handBack, which lets the mob's tree say why). The topic takes the
// instance the player named. It returns false, changing nothing, when
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
	// The topic takes the instance the player named (`give 2.purse`).
	buf := &lineBuffer{user: user, prefer: itm.UUID}
	ps := talkState(user, buf)
	t, wanted := df.ItemTopic(itm.ItemId, ps)
	if !wanted {
		return false
	}
	if t == nil {
		handBack(user, room, mob, itm)
		return true
	}
	resolve(user, room, mob, df, t, ps, buf)
	return true
}

// handBack returns a wanted item no topic can take now. The mob's tree hears
// the give first as a player_give hand-back (behaviortree.GiveHandBack), so
// a refusal written for that item plays ("See Drunn first, then come
// back."); the tree can only speak and say the item came back, never
// credit, and a catch-all give branch stays silent. When no branch
// ran return_item, the player reads the generic hand-back line, so they
// always learn the item is still theirs. The item never moves.
func handBack(user *users.UserRecord, room *rooms.Room, mob *mobs.Mob, itm items.Item) {
	hb := &behaviortree.GiveHandBack{}
	behaviortree.TryMobBehavior(mob.InstanceId, behaviortree.EventContext{
		EventType: "player_give",
		UserId:    user.UserId,
		ItemId:    itm.ItemId,
		ItemUUID:  itm.UUID,
		RoomId:    room.RoomId,
		HandBack:  hb,
	})
	if hb.Returned {
		return
	}
	sight := messaging.ParticipantSight(user.Character, room)
	user.SendText(messaging.CategorySystem, messaging.HideNames(fmt.Sprintf(
		`<ansi fg="mobname">%s</ansi> looks at the <ansi fg="item">%s</ansi> and hands it back.`,
		mob.Character.Name, itm.DisplayName()), []string{mob.Character.Name}, sight))
}
