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
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// Talk opens a conversation with mob: the approach line and the NPC's
// opener and breadcrumbs as speech the room hears, then the private menu.
// The caller has already resolved mob by sight and refused a sleeping one.
// When the menu could not open, or the player cannot see the NPC by name (a
// figure at shapes sight; spec 4 item 1), the player gets one private line
// and nobody speaks.
func Talk(user *users.UserRecord, room *rooms.Room, mob *mobs.Mob) {
	if line := blockedLine(user); line != "" {
		user.SendText(messaging.CategoryRefusal, line)
		return
	}
	if !seesByName(user, room, mob) {
		user.SendText(messaging.CategoryRefusal, SightRefusal(user, room))
		return
	}
	room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(
		`<ansi fg="username">%s</ansi> approaches <ansi fg="mobname">%s</ansi> for a conversation.`,
		user.Character.Name, mob.Character.Name), user.UserId)

	df := dialogue.Load(int(mob.MobId), mob.Zone)
	// Charmed NPCs offer no topics (the marker skips them too).
	if df == nil || mob.Character.IsCharmed() {
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

// blockedLines name why a conversation cannot start, one per menu gate rule.
var blockedLines = map[menu.CloseReason]string{
	menu.CloseCombat:     "You can't start a conversation in the middle of a fight.",
	menu.CloseBusy:       "You are too busy to start a conversation.",
	menu.CloseAsleep:     "You can't start a conversation while you are asleep.",
	menu.CloseControlled: "You can't start a conversation while you are held.",
	menu.CloseQuitting:   "You can't start a conversation while you meditate.",
	menu.CloseDead:       "You can't start a conversation while you are dead.",
}

const promptOpenLine = "Finish what you have open before you start a conversation."

// TalkSightLine refuses a conversation with an NPC the player cannot see by
// name (spec 4 item 1): one aimed at as a figure, or one the light no longer
// shows. It names no one, so a figure's name never leaks, and follows the
// shop verbs' "You can't make out ... well enough to ..." wording.
const TalkSightLine = "You can't make out who that is well enough to start a conversation."

// TalkShapesLine refuses every talk aim from a player who makes out shapes
// only. It says what the player needs, more light, rather than the shared aim
// hint's "Try talk figure", which only reaches TalkSightLine (playtest seat
// C, #375). Two lines: refusals are not wrapped.
const TalkShapesLine = "You can only make out shapes here.\n" +
	"You need more light to see who someone is before you can talk to them."

// SightRefusal is the line for a talk the player's sight refuses:
// TalkShapesLine at shapes, TalkSightLine otherwise.
func SightRefusal(user *users.UserRecord, room *rooms.Room) string {
	if room != nil && messaging.ParticipantSight(user.Character, room) == messaging.SightShapes {
		return TalkShapesLine
	}
	return TalkSightLine
}

// RefusedAtShapes sends TalkShapesLine and reports true when the player
// makes out shapes only, so no talk aim there is resolved or hinted at.
func RefusedAtShapes(user *users.UserRecord, room *rooms.Room) bool {
	if room == nil || messaging.ParticipantSight(user.Character, room) != messaging.SightShapes {
		return false
	}
	user.SendText(messaging.CategoryRefusal, TalkShapesLine)
	return true
}

// blockedLine is the refusal when the talk menu could not open now, or "".
func blockedLine(user *users.UserRecord) string {
	reason, promptOpen := user.MenuBlockedReason()
	if reason != menu.CloseNone {
		if line, ok := blockedLines[reason]; ok {
			return line
		}
		return "You can't start a conversation right now."
	}
	if promptOpen {
		return promptOpenLine
	}
	return ""
}

// lineBuffer holds the private lines the PlayerState callbacks send during a
// choice (a delivery, gold received or paid, a refusal), so they land after
// the handover line and before the NPC's reply. It also carries the choice's
// item: prefer is the instance a give named (zero from the menu), and taken
// is the display name of the item the choice's effects took, for the
// handover line.
type lineBuffer struct {
	user   *users.UserRecord
	lines  []bufferedLine
	prefer uuid.UUID
	taken  string
}

type bufferedLine struct {
	cat  messaging.Category
	text string
}

func (b *lineBuffer) add(cat messaging.Category, text string) {
	b.lines = append(b.lines, bufferedLine{cat, text})
}

func (b *lineBuffer) flush() {
	for _, l := range b.lines {
		b.user.SendText(l.cat, l.text)
	}
	b.lines = nil
}

// talkState is PlayerState for a choice: its private lines go to buf, a
// price the player cannot pay says so (the shared callback is silent), and
// a taken item is buf.prefer's instance when it names one, recorded in
// buf.taken.
func talkState(user *users.UserRecord, buf *lineBuffer) *dialogue.PlayerState {
	ps := playerState(user, buf.add)
	ps.RemoveItem = func(itemId int) bool {
		itm, ok := removeBackpackItem(user, itemId, buf.prefer)
		if ok {
			buf.taken = itm.DisplayName()
		}
		return ok
	}
	charge := ps.ChargeGold
	ps.ChargeGold = func(amount int) bool {
		if charge(amount) {
			return true
		}
		buf.add(messaging.CategoryRefusal, "You don't have enough gold for that.")
		return false
	}
	return ps
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
	if !seesByName(user, room, mob) {
		return nil, nil, false
	}
	return mob, room, true
}

// seesByName reports whether the player can see mob by name in room: full
// sight there, and the mob perceived (spec 4 item 1).
func seesByName(user *users.UserRecord, room *rooms.Room, mob *mobs.Mob) bool {
	return room != nil && messaging.ParticipantSight(user.Character, room) == messaging.SightFull &&
		user.Character.Perceives(&mob.Character)
}

func session(user *users.UserRecord, mob *mobs.Mob, onChoose func(menu.Entry)) menu.Session {
	inst := mob.InstanceId
	return menu.Session{
		Topic:      "talk with " + mob.Character.Name,
		TopicNames: []string{mob.Character.Name},
		StillValid: func() bool { _, _, ok := present(user, inst); return ok },
		OnChoose:   onChoose,
		// The talk menu and the Accept / Leave card come after the lines of
		// the choice that opened them, quest banners and rewards included.
		ShowLast:      true,
		MobInstanceId: inst,
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
	buf := &lineBuffer{user: user}
	ps := talkState(user, buf)
	t := df.FindTopic(key)
	if t == nil || !df.Eligible(t, ps) {
		openMenu(user, mobInstanceId, "")
		return
	}
	resolve(user, room, mob, df, t, ps, buf)
}

// resolve runs an eligible topic, the one path for a menu choice and for a
// give (R8): an offer pitches and shows its Accept / Leave card (R6), and
// takes nothing until Accept; any other topic plays and the menu is
// re-shown, except a behavior topic whose effects landed, which closes it.
func resolve(user *users.UserRecord, room *rooms.Room, mob *mobs.Mob, df *dialogue.DialogueFile, t *dialogue.Topic, ps *dialogue.PlayerState, buf *lineBuffer) {
	if t.Kind == dialogue.KindOffer {
		playerSay(user, room, t.PlayerLine)
		npcSay(mob, room, t.Text)
		sendHints(user, t.Hints)
		offerCard(user, mob, t, buf.prefer)
		return
	}
	granted, applied := play(user, room, mob, df, t, ps, buf)
	if applied && t.Behavior {
		return // a timed scene is under way: the menu closes (R18)
	}
	openMenu(user, mob.InstanceId, granted)
}

// play speaks and applies an info or hand-in topic: the player's line, the
// handover when it takes an item, the callbacks' private lines, the NPC's
// reply, then mood and any behavior. When Apply refuses (a price or a
// delivery failed) the player reads the refusal and the NPC says nothing
// more. It returns the quest token the topic granted, or "", and whether the
// topic's effects landed.
func play(user *users.UserRecord, room *rooms.Room, mob *mobs.Mob, df *dialogue.DialogueFile, t *dialogue.Topic, ps *dialogue.PlayerState, buf *lineBuffer) (string, bool) {
	line := t.Label
	if t.Kind == dialogue.KindHandIn {
		line = t.PlayerLine
	}
	playerSay(user, room, line)
	if !applyWithHandover(user, room, mob, df, t, ps, buf) {
		return "", false
	}
	npcSay(mob, room, t.Text)
	sendHints(user, t.Hints)
	after(user, room, mob, df, t)
	return t.GrantsQuest, true
}

// applyWithHandover applies t and, when it took an item, prints the handover
// lines; then it flushes the callbacks' private lines, refused or not.
func applyWithHandover(user *users.UserRecord, room *rooms.Room, mob *mobs.Mob, df *dialogue.DialogueFile, t *dialogue.Topic, ps *dialogue.PlayerState, buf *lineBuffer) bool {
	buf.taken = ""
	ok := df.Apply(t, ps)
	if ok && buf.taken != "" {
		handoverLines(user, room, mob, buf.taken)
	}
	buf.flush()
	return ok
}

// after runs once per choice whose effects landed: it refreshes the
// player's NPC markers (the heard set or a grant may have changed them),
// shifts the NPC's mood and fires a behavior topic's tree.
func after(user *users.UserRecord, room *rooms.Room, mob *mobs.Mob, df *dialogue.DialogueFile, t *dialogue.Topic) {
	RefreshMarkers(user.UserId)
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
// the talk menu comes back. prefer is the item instance a give named, which
// Accept takes in place of the first match (zero from the menu).
func offerCard(user *users.UserRecord, mob *mobs.Mob, t *dialogue.Topic, prefer uuid.UUID) {
	inst, key := mob.InstanceId, t.Key
	card := menu.Confirm(t.Label, "Accept", "Leave")
	user.OpenMenu(card, session(user, mob, func(e menu.Entry) {
		granted, closes := "", false
		if e.Id == "yes" {
			granted, closes = accept(user, inst, key, prefer)
		}
		if closes {
			return
		}
		openMenu(user, inst, granted)
	}))
}

// accept grants an offer after re-checking it, and returns the token it
// granted, or "", and whether the accepted offer was a behavior topic (R18),
// which closes the menu.
func accept(user *users.UserRecord, mobInstanceId int, key string, prefer uuid.UUID) (string, bool) {
	mob, room, ok := present(user, mobInstanceId)
	if !ok {
		return "", false
	}
	df := dialogue.Load(int(mob.MobId), mob.Zone)
	buf := &lineBuffer{user: user, prefer: prefer}
	ps := talkState(user, buf)
	t := df.FindTopic(key)
	if t == nil || !df.Eligible(t, ps) {
		user.SendText(messaging.CategoryRefusal, `That offer is no longer open.`)
		return "", false
	}
	if !applyWithHandover(user, room, mob, df, t, ps, buf) {
		return "", false
	}
	after(user, room, mob, df, t)
	return t.GrantsQuest, t.Behavior
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
