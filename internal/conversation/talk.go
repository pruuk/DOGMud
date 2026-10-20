package conversation

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Talk opens a conversation with mob: the approach line and the NPC's
// opener and breadcrumbs as speech the room hears, then the private menu.
// The caller has already resolved mob by sight and refused a sleeping one.
func Talk(user *users.UserRecord, room *rooms.Room, mob *mobs.Mob) {
	room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(
		`<ansi fg="username">%s</ansi> approaches <ansi fg="mobname">%s</ansi> for a conversation.`,
		user.Character.Name, mob.Character.Name), user.UserId)

	df := dialogue.Load(int(mob.MobId), mob.Zone)
	if df == nil {
		mob.Command(`emote nods.`)
		return
	}
	ps := PlayerState(user)
	text, hints := df.OpenerFor(ps)
	if text != "" {
		npcSay(mob, room, text)
	}
	for _, b := range df.Breadcrumbs(ps) {
		npcSay(mob, room, b)
	}
	sendHints(user, hints)
	openMenu(user, mob.InstanceId, "")
}

// heldAlso is ps with granted counted as held. GiveQuest only queues the
// token and the quest engine records it on a later event pass, so the menu
// re-shown straight after a grant would otherwise still list the topic the
// player just took (an accepted offer, a hand-in or a step whose
// questExcluded names its own grant).
func heldAlso(ps *dialogue.PlayerState, granted string) *dialogue.PlayerState {
	if granted == "" || ps == nil || ps.HasQuest == nil {
		return ps
	}
	has := ps.HasQuest
	ps.HasQuest = func(token string) bool { return token == granted || has(token) }
	return ps
}

// present returns the mob and room while the player can still talk to it:
// same room, full sight, and the mob perceived. It backs StillValid.
func present(user *users.UserRecord, mobInstanceId int) (*mobs.Mob, *rooms.Room, bool) {
	mob := mobs.GetInstance(mobInstanceId)
	if mob == nil || mob.Character.RoomId != user.Character.RoomId {
		return nil, nil, false
	}
	room := rooms.LoadRoom(user.Character.RoomId)
	if room == nil || messaging.ParticipantSight(user.Character, room) != messaging.SightFull ||
		!user.Character.Perceives(&mob.Character) {
		return nil, nil, false
	}
	return mob, room, true
}

func session(user *users.UserRecord, mob *mobs.Mob, onChoose func(menu.Entry)) menu.Session {
	inst := mob.InstanceId
	return menu.Session{
		Topic:      "talk with " + mob.Character.Name,
		TopicNames: []string{mob.Character.Name},
		StillValid: func() bool { _, _, ok := present(user, inst); return ok },
		OnChoose:   onChoose,
	}
}

// openMenu shows (or re-shows) the talk menu, recomputed for this moment.
// Nothing opens when the NPC is gone or has nothing to talk about. granted
// names the quest token the choice just granted, if any (see heldAlso).
func openMenu(user *users.UserRecord, mobInstanceId int, granted string) {
	mob, _, ok := present(user, mobInstanceId)
	if !ok {
		return
	}
	df := dialogue.Load(int(mob.MobId), mob.Zone)
	m := df.Menu(mob.Character.Name, heldAlso(PlayerState(user), granted))
	if len(m.Entries) == 0 {
		return
	}
	user.OpenMenu(m, session(user, mob, func(e menu.Entry) { choose(user, mobInstanceId, e.Id) }))
}

func choose(user *users.UserRecord, mobInstanceId int, key string) {
	mob, room, ok := present(user, mobInstanceId)
	if !ok {
		return
	}
	df := dialogue.Load(int(mob.MobId), mob.Zone)
	ps := PlayerState(user)
	t := df.FindTopic(key)
	if t == nil || !df.Eligible(t, ps) {
		openMenu(user, mobInstanceId, "")
		return
	}
	if t.Kind == dialogue.KindOffer {
		playerSay(user, room, t.PlayerLine)
		npcSay(mob, room, t.Text)
		sendHints(user, t.Hints)
		offerCard(user, mob, t)
		return
	}
	granted := play(user, room, mob, df, t, ps)
	openMenu(user, mobInstanceId, granted)
}

// play speaks and applies an info or hand-in topic: the player's line, the
// handover when it takes an item, the NPC's reply, then mood and any
// behavior. When Apply refuses (a price or a delivery failed) the NPC says
// nothing more; the refusal line came from the PlayerState callback. It
// returns the quest token the topic granted, or "".
func play(user *users.UserRecord, room *rooms.Room, mob *mobs.Mob, df *dialogue.DialogueFile, t *dialogue.Topic, ps *dialogue.PlayerState) string {
	line := t.Label
	if t.Kind == dialogue.KindHandIn {
		line = t.PlayerLine
	}
	playerSay(user, room, line)
	// Read before Apply, which takes the item.
	itemName := backpackItemName(user, t.RequiresItem)
	if !df.Apply(t, ps) {
		return ""
	}
	if itemName != "" {
		handoverLines(user, room, mob, itemName)
	}
	npcSay(mob, room, t.Text)
	sendHints(user, t.Hints)
	after(user, room, mob, df, t)
	return t.GrantsQuest
}

// after shifts the NPC's mood and fires a behavior topic's tree.
func after(user *users.UserRecord, room *rooms.Room, mob *mobs.Mob, df *dialogue.DialogueFile, t *dialogue.Topic) {
	dialogue.ShiftMood(mob.InstanceId, t.MoodChange, df.DefaultMood)
	if t.Behavior {
		behaviortree.TryMobBehavior(mob.InstanceId, behaviortree.EventContext{
			EventType: "player_topic",
			UserId:    user.UserId,
			Text:      t.Key,
			RoomId:    room.RoomId,
		})
	}
}

// offerCard is the private Accept / Leave after an offer's pitch (R6).
// Enter is Leave. Accept re-checks eligibility before granting; either way
// the talk menu comes back.
func offerCard(user *users.UserRecord, mob *mobs.Mob, t *dialogue.Topic) {
	inst, key := mob.InstanceId, t.Key
	card := menu.Confirm(t.Label, "Accept", "Leave")
	user.OpenMenu(card, session(user, mob, func(e menu.Entry) {
		granted := ""
		if e.Id == "yes" {
			granted = accept(user, inst, key)
		}
		openMenu(user, inst, granted)
	}))
}

// accept grants an offer after re-checking it, and returns the token it
// granted, or "".
func accept(user *users.UserRecord, mobInstanceId int, key string) string {
	mob, room, ok := present(user, mobInstanceId)
	if !ok {
		return ""
	}
	df := dialogue.Load(int(mob.MobId), mob.Zone)
	ps := PlayerState(user)
	t := df.FindTopic(key)
	if t == nil || !df.Eligible(t, ps) {
		user.SendText(messaging.CategoryRefusal, `That offer is no longer open.`)
		return ""
	}
	if !df.Apply(t, ps) {
		return ""
	}
	after(user, room, mob, df, t)
	return t.GrantsQuest
}

// playerSay speaks a topic line as the player: the room hears it through
// actions.Say (sight-aware names, deafen filter), the player reads their
// own line. A muted player's line stays private.
func playerSay(user *users.UserRecord, room *rooms.Room, text string) {
	if text == "" {
		return
	}
	if !user.Muted {
		actions.Say(&actions.UserActor{User: user, Room: room}, text)
	}
	user.SendText(messaging.CategorySpeech, util.SplitStringNL(
		fmt.Sprintf(`You say, "<ansi fg="saytext">%s</ansi>"`, text), 80))
}

func npcSay(mob *mobs.Mob, room *rooms.Room, text string) {
	if text != "" {
		actions.Say(&actions.MobActor{Mob: mob, Room: room}, text)
	}
}

func sendHints(user *users.UserRecord, hints string) {
	if hints != "" {
		user.SendText(messaging.CategoryDialogueHint, fmt.Sprintf(`<ansi fg="181">  [%s]</ansi>`, hints))
	}
}

func backpackItemName(user *users.UserRecord, itemId int) string {
	if itemId <= 0 {
		return ""
	}
	for _, itm := range user.Character.GetAllBackpackItems() {
		if itm.ItemId == itemId {
			return itm.DisplayName()
		}
	}
	return ""
}

// handoverLines are give.go's own lines for an item handed to a mob.
func handoverLines(user *users.UserRecord, room *rooms.Room, mob *mobs.Mob, itemName string) {
	sight := messaging.ParticipantSight(user.Character, room)
	user.SendText(messaging.CategorySystem, messaging.HideNames(
		fmt.Sprintf(`You give the <ansi fg="item">%s</ansi> to <ansi fg="mobname">%s</ansi>.`, itemName, mob.Character.Name),
		[]string{mob.Character.Name}, sight))
	room.SendTextVisual(messaging.CategoryLoot, fmt.Sprintf(
		`<ansi fg="username">%s</ansi> gave their <ansi fg="item">%s</ansi> to <ansi fg="mobname">%s</ansi>.`,
		user.Character.Name, itemName, mob.Character.Name), user.UserId)
}
