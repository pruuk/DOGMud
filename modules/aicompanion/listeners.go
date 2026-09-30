package aicompanion

import (
	"fmt"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Everything in this file runs under the mud lock (event listeners and the
// ask hook). Nothing here calls the model; it records what the companion
// perceived and queues stimuli for the next dispatch.

// speakerOf names a player the way a companion in the room would: their
// name, or "someone" if they are hidden.
// speakerOf names a player as this companion perceives them: by name when
// it can make them out, and otherwise as a voice in the dark. Its own
// senses decide, not the speaker's hiding alone.
func speakerOf(u *users.UserRecord, mob *mobs.Mob) string {
	if mob != nil && !mob.Character.Perceives(u.Character) {
		return `someone`
	}
	if mob == nil && u.Character.IsHidden() {
		return `someone`
	}
	return u.Character.Name
}

// companionsInRoom returns the standing controllers whose companion is in
// the room.
func (m *AICompanionModule) companionsInRoom(roomId int) []*controller {
	var out []*controller
	for _, c := range m.ctrls {
		if c.instanceId == 0 {
			continue
		}
		if mob := mobs.GetInstance(c.instanceId); mob != nil && mob.Character.RoomId == roomId {
			out = append(out, c)
		}
	}
	return out
}

// onCommunication records speech the companion can hear and queues a
// response when it is addressed.
func (m *AICompanionModule) onCommunication(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.Communication)
	if !ok || !m.cfg.Enabled || evt.CommType != `say` || len(m.ctrls) == 0 {
		return events.Continue
	}

	// A companion's own speech comes back through here; ignore it.
	if evt.SourceMobInstanceId > 0 && m.controllerForInstance(evt.SourceMobInstanceId) != nil {
		return events.Continue
	}

	roomId := 0
	speaker := evt.Name
	speakerUserId := 0

	if evt.SourceUserId > 0 {
		u := users.GetByUserId(evt.SourceUserId)
		if u == nil || u.Character == nil {
			return events.Continue
		}
		roomId = u.Character.RoomId
		speakerUserId = u.UserId
		speaker = u.Character.Name // named per companion below
	} else if evt.SourceMobInstanceId > 0 {
		if sm := mobs.GetInstance(evt.SourceMobInstanceId); sm != nil {
			roomId = sm.Character.RoomId
			speaker = sm.Character.Name
			if sm.Character.IsHidden() {
				speaker = `someone`
			}
		}
	}
	if roomId == 0 || strings.TrimSpace(evt.Message) == `` {
		return events.Continue
	}

	room := rooms.LoadRoom(roomId)
	now := time.Now().Unix()

	// Calling her name when she is elsewhere brings her back on her own
	// feet. She does not have to be in earshot for this: a companion who
	// cannot be called is a companion an owner has to go and find.
	if speakerUserId > 0 {
		if c, ok := m.ctrls[speakerUserId]; ok && m.cfg.Enabled {
			if mob := mobs.GetInstance(c.instanceId); mob != nil && mob.Character.RoomId != roomId &&
				mentionsName(evt.Message, c.profile.Name) {
				m.calledBack(c, mob, users.GetByUserId(speakerUserId))
			}
		}
	}

	for _, c := range m.companionsInRoom(roomId) {
		mob := mobs.GetInstance(c.instanceId)
		others := 0
		heardFrom := speaker
		if mob != nil {
			others = otherPlayersPresent(room, speakerUserId, mob)
			if u := users.GetByUserId(speakerUserId); speakerUserId > 0 && u != nil {
				heardFrom = speakerOf(u, mob)
			}
		}
		direct := false
		fromOwner := speakerUserId > 0 && speakerUserId == c.ownerUserId
		if speakerUserId > 0 {
			direct = isAddressed(evt.Message, c.profile.Name, fromOwner, others, m.cfg.RespondWhenAlone)
		}
		m.hearSaid(c, users.GetByUserId(speakerUserId), heardFrom, evt.Message, roomId, direct, now)
	}
	return events.Continue
}

// hearSaid is one companion hearing one line of speech: u is the player who
// said it (nil for a mob), speaker how she makes them out, direct whether
// it was meant for her.
func (m *AICompanionModule) hearSaid(c *controller, u *users.UserRecord, speaker string, text string, roomId int, direct bool, now int64) {
	speakerUserId := 0
	if u != nil {
		speakerUserId = u.UserId
	}
	fromOwner := speakerUserId > 0 && speakerUserId == c.ownerUserId

	// A literal "i agree" or "i decline" while the question is open is an
	// answer, not conversation: it is not written down or answered, and it
	// is read whether or not she was named, because the question asks for
	// exactly those words and nothing else.
	if fromOwner && m.answerConsent(c, u, text) {
		return
	}
	// Nothing anyone says is written into her mind until her owner has
	// agreed that what is said may leave the server. Her memory is the
	// thing that gets sent, so recording first and gating later is the same
	// as not gating at all. She still hears it, and still answers with her
	// set lines (dispatch sends an unconsented owner to the fallback), so
	// only the writing down waits.
	//
	// Speech that was not for her is remembered only when the server allows
	// it: it is what lets her overhear, and it is also other people's
	// conversation going to the API.
	if m.consented(c.ownerUserId) && (direct || m.cfg.RecordBystanderSpeech) {
		line := Line{Speaker: speaker, Kind: `said`, ToMe: direct, Text: text, Unix: now}
		c.mind.addLine(line, m.cfg.WorkingMemoryLines)
		c.dirty = true
		if direct {
			m.noteConversation(c, roomId, speaker, speakerUserId, line)
		}
	}
	if speakerUserId > 0 {
		c.lastSocialUnix = now
	}
	if fromOwner && !direct {
		c.ownerTalkedAway = now
	}
	if !direct {
		return
	}
	if fromOwner {
		m.interruptErrand(c, speaker)
		// Spoken to by an owner who is sneaking, she hears it and holds her
		// tongue: answering aloud is what gets people caught.
		if m.sneaking(c, u) {
			return
		}
	} else if !m.strangerMayAsk(u, c) {
		// A passer-by speaking to her by name is asking her something as
		// surely as one who uses `ask`, and is paced the same way. Heard,
		// and remembered above if that is allowed, but not answered.
		return
	}
	c.push(stimulus{Kind: `heard`, Speaker: speaker, Text: text,
		FromOwner: fromOwner, AskerUserId: speakerUserId})
}

// onEmote notices emotes. An emote is something seen, so a companion that
// cannot see, in the dark or blind, or cannot make the actor out, never
// learns of it. One that names the companion is aimed at it and gets a
// response; any other is only remembered as something seen.
func (m *AICompanionModule) onEmote(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.Emote)
	if !ok || !m.cfg.Enabled || evt.UserId == 0 || len(m.ctrls) == 0 {
		return events.Continue
	}
	u := users.GetByUserId(evt.UserId)
	text := strings.TrimSpace(evt.Text)
	if u == nil || u.Character == nil || text == `` {
		return events.Continue
	}
	for _, c := range m.companionsInRoom(evt.RoomId) {
		mob := mobs.GetInstance(c.instanceId)
		room := rooms.LoadRoom(evt.RoomId)
		if mob == nil || room == nil || cannotSee(mob, room) || !mob.Character.Perceives(u.Character) {
			continue
		}
		m.seeEmote(c, u, speakerOf(u, mob), text, evt.RoomId, mentionsName(text, c.profile.Name), time.Now().Unix())
	}
	return events.Continue
}

// seeEmote is one companion seeing one emote she could make out.
func (m *AICompanionModule) seeEmote(c *controller, u *users.UserRecord, speaker string, text string, roomId int, direct bool, now int64) {
	fromOwner := u.UserId == c.ownerUserId
	if !direct && !m.cfg.RecordBystanderSpeech {
		return // other people's business, by the server's choice
	}
	// Nothing is written down before they have agreed; she still sees it,
	// and still answers with her set lines.
	if m.consented(c.ownerUserId) {
		emoteLine := Line{Speaker: speaker, Kind: `emoted`, ToMe: direct, Text: text, Unix: now}
		c.mind.addLine(emoteLine, m.cfg.WorkingMemoryLines)
		c.dirty = true
		if direct {
			m.noteConversation(c, roomId, speaker, u.UserId, emoteLine)
		}
	}
	c.lastSocialUnix = now
	if !direct {
		return
	}
	if fromOwner {
		m.interruptErrand(c, speaker)
	} else if !m.strangerMayAsk(u, c) {
		return // seen, but a gesture is paced like a question
	}
	c.push(stimulus{Kind: `emote`, Speaker: speaker, Text: text, FromOwner: fromOwner, AskerUserId: u.UserId})
}

// onGiftAccepted reacts to an item given to the companion. The engine has
// already moved the item; this only records what it meant.
func (m *AICompanionModule) onGiftAccepted(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.GiftAccepted)
	if !ok || !m.cfg.Enabled {
		return events.Continue
	}
	c := m.controllerForInstance(evt.MobInstanceId)
	u := users.GetByUserId(evt.UserId)
	if c == nil || u == nil || u.Character == nil {
		return events.Continue
	}

	// items.GetItemSpec(evt.ItemId) reads only the carrier's own catalog
	// spec (its base item type, e.g. "Curious Trinket"): a bauble's own
	// name lives on the *items.Item instance (its Bauble record id,
	// resolved through items.Item.ModelName/Name), and events.GiftAccepted
	// carries no instance, only the ItemId, so a bauble given to the
	// companion is always named by its generic carrier here, never by its
	// catalog text (model-safe by construction, if more generic than it
	// need be for a server-moderated bauble). If this event ever grows to
	// carry the item instance, name it with item.ModelName() instead
	// (internal/items/bauble_model.go), never Name(), so a player-key
	// bauble's own text still cannot reach the model.
	itemName := `something`
	if spec := items.GetItemSpec(evt.ItemId); spec != nil && spec.Name != `` {
		itemName = spec.Name
	}
	giver := speakerOf(u, mobs.GetInstance(c.instanceId))
	fromOwner := u.UserId == c.ownerUserId
	now := time.Now().Unix()

	if m.mayRemember(c) {
		c.mind.addLine(Line{Speaker: giver, Kind: `event`, Text: fmt.Sprintf(`%s gave you %s.`, giver, itemName)}, m.cfg.WorkingMemoryLines)
		c.mind.addMemory(Memory{
			Unix: now, Kind: `gift`, Text: fmt.Sprintf(`%s gave me %s.`, giver, itemName),
			Importance: 5, Emotion: `gratitude`, People: []string{giver}, PlaceId: u.Character.RoomId,
		}, m.cfg.MaxMemories)
	}

	// A gift from the owner is something she keeps (F9.5).
	if fromOwner {
		c.mind.protectItem(evt.ItemId, `a gift from `+giver)
		// And the exact thing, not merely one of its kind.
		if mob := mobs.GetInstance(c.instanceId); mob != nil {
			for i := range mob.Character.Items {
				if mob.Character.Items[i].ItemId == evt.ItemId {
					c.mind.protectInstance(mob.Character.Items[i].UUID.String())
				}
			}
		}
	}

	// A small, certain warmth for a gift from the owner, at most three
	// times a day so gifts cannot be farmed (the model may add a little
	// more within the gift envelope).
	if fromOwner && c.mind.ruleChangesSince(`gift`, now-86400) < 3 {
		c.mind.applyOpinion(Opinion{Affection: 1}, `gift`, `rule`, itemName, false)
	}

	c.dirty = true
	c.snapshotDue = true
	c.lastSocialUnix = now
	// A passer-by pressing things on her is paced like one asking her
	// things: the gift is hers and remembered, but she need not stop and
	// think about every one.
	if !fromOwner && !m.strangerMayAsk(u, c) {
		return events.Continue
	}
	c.push(stimulus{Kind: `gift`, Speaker: giver, Text: itemName, FromOwner: fromOwner, AskerUserId: u.UserId})
	return events.Continue
}

// onGoldGiven is a player giving her gold with `give`: the engine names the
// giver, so the right person is thanked and, for a passer-by, paced and
// charged (receiveGold). noticeGold matches the purse against it.
func (m *AICompanionModule) onGoldGiven(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.GoldGiven)
	if !ok || !m.cfg.Enabled || evt.Amount <= 0 {
		return events.Continue
	}
	c := m.controllerForInstance(evt.MobInstanceId)
	u := users.GetByUserId(evt.UserId)
	mob := mobs.GetInstance(evt.MobInstanceId)
	if c == nil || u == nil || u.Character == nil || mob == nil {
		return events.Continue
	}
	c.goldByEvent += evt.Amount
	m.receiveGold(c, mob, u, evt.Amount)
	return events.Continue
}

// onPlayerAttackedMob reacts to someone attacking the companion. The combat
// system handles the fight itself; this records the betrayal or threat.
func (m *AICompanionModule) onPlayerAttackedMob(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.PlayerAttackedMob)
	if !ok || !m.cfg.Enabled {
		return events.Continue
	}
	c := m.controllerForInstance(evt.MobInstanceId)
	u := users.GetByUserId(evt.UserId)
	if c == nil {
		// Not an attack on her. It may still be one she watched her owner
		// make on somebody who had done nothing.
		m.witnessAttack(evt.UserId, evt.MobInstanceId)
		return events.Continue
	}
	if u == nil || u.Character == nil {
		return events.Continue
	}

	// One reaction per attacker per ten minutes; a fight fires this often.
	now := time.Now().Unix()
	if last, seen := c.lastAttackBy[u.UserId]; seen && now-last < 600 {
		return events.Continue
	}
	c.lastAttackBy[u.UserId] = now

	attacker := speakerOf(u, mobs.GetInstance(c.instanceId))
	fromOwner := u.UserId == c.ownerUserId

	remember := m.mayRemember(c)
	if remember {
		c.mind.addLine(Line{Speaker: attacker, Kind: `event`, Text: fmt.Sprintf(`%s attacked you.`, attacker)}, m.cfg.WorkingMemoryLines)
		importance := 6
		if fromOwner {
			importance = 9
		}
		c.mind.addMemory(Memory{
			Unix: now, Kind: `attack`, Text: fmt.Sprintf(`%s attacked me.`, attacker),
			Importance: importance, Emotion: `anger`, People: []string{attacker}, PlaceId: u.Character.RoomId,
		}, m.cfg.MaxMemories)
	}

	// Being attacked by the person you travel with costs trust and
	// affection whatever the model says (F5.4); the model may add more
	// within the attack envelope, never less.
	if fromOwner {
		c.mind.applyOpinion(Opinion{Trust: -5, Affection: -5}, `attacked`, `rule`, `attacked me`, false)
		// Being struck by the person you have come to love is one of the
		// few things that changes what you are to each other.
		if remember && romanceRank(c.mind.Romance.Stage) > 0 {
			m.recordCore(c, attacker, c.mind.Romance.Stage, false)
		}
	}

	c.dirty = true
	c.lastSocialUnix = now
	// An attack pre-empts anything that was waiting.
	c.pending = nil
	// A stranger's attack is paced by the ten-minute rule above, and what
	// she makes of it is paid for from their allowance, not her owner's.
	c.push(stimulus{Kind: `attacked`, Speaker: attacker, FromOwner: fromOwner, AskerUserId: u.UserId})
	return events.Continue
}

// handleAsk claims `ask <companion> <text>` for a companion this module
// drives. Installed into internal/companionai.
func (m *AICompanionModule) handleAsk(userId int, mobInstanceId int, text string) bool {
	if !m.cfg.Enabled {
		return false
	}
	c := m.controllerForInstance(mobInstanceId)
	if c == nil {
		return false
	}
	u := users.GetByUserId(userId)
	mob := mobs.GetInstance(mobInstanceId)
	text = strings.TrimSpace(text)
	if u == nil || u.Character == nil || mob == nil || text == `` || u.Muted {
		return false
	}

	safe := util.EscapeAnsiTags(text)
	u.SendText(messaging.CategorySpeech, fmt.Sprintf(
		`You ask <ansi fg="mobname">%s</ansi>, "<ansi fg="saytext">%s</ansi>"`, mob.Character.Name, safe))
	if room := rooms.LoadRoom(u.Character.RoomId); room != nil {
		room.SendTextCommunication(fmt.Sprintf(
			`<ansi fg="username">%s</ansi> asks <ansi fg="mobname">%s</ansi>, "<ansi fg="saytext">%s</ansi>"`,
			u.Character.Name, mob.Character.Name, safe), u.UserId)
	}

	m.hearAsked(c, u, speakerOf(u, mob), text, time.Now().Unix())
	return true
}

// hearAsked is the companion being put a question directly with `ask`.
func (m *AICompanionModule) hearAsked(c *controller, u *users.UserRecord, speaker string, text string, now int64) {
	fromOwner := u.UserId == c.ownerUserId
	if fromOwner {
		// "ask <her> i agree" is as good an answer to the question as saying
		// it aloud, and is not conversation either.
		if m.answerConsent(c, u, text) {
			return
		}
		m.interruptErrand(c, speaker)
	}
	// Heard, and answered below, but written down only once her owner has
	// agreed that her mind may be sent. Before that, dispatch answers with
	// her set lines.
	if m.consented(c.ownerUserId) {
		askLine := Line{Speaker: speaker, Kind: `asked`, ToMe: true, Text: text, Unix: now}
		c.mind.addLine(askLine, m.cfg.WorkingMemoryLines)
		m.noteConversation(c, u.Character.RoomId, speaker, u.UserId, askLine)
		c.dirty = true
	}
	c.lastSocialUnix = now
	// A stranger's question is asked aloud, heard, and remembered like any
	// other, but she stops what she is doing to answer only as often as
	// strangerMayAsk allows: a passer-by cannot make her think on demand.
	if !fromOwner && !m.strangerMayAsk(u, c) {
		return
	}
	c.push(stimulus{Kind: `asked`, Speaker: speaker, Text: text,
		FromOwner: fromOwner, AskerUserId: u.UserId})
}

// interruptErrand stops a trip when the owner speaks to the companion
// (F11.6): the owner comes first, and the model decides what next. A walk
// back to the owner is not interrupted.
func (m *AICompanionModule) interruptErrand(c *controller, ownerName string) {
	if c.travel == nil || c.travel.Purpose == `return` {
		return
	}
	dest := c.travel.DestName
	c.travel = nil
	if m.mayRemember(c) {
		c.mind.addLine(Line{Kind: `event`, Text: `You stopped on your way to ` + dest + ` because ` + ownerName + ` spoke to you.`}, m.cfg.WorkingMemoryLines)
		c.dirty = true
	}
}

// onHealed reacts to someone healing the companion with magic.
func (m *AICompanionModule) onHealed(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.Healed)
	if !ok || !m.cfg.Enabled {
		return events.Continue
	}
	c := m.controllerForInstance(evt.MobInstanceId)
	u := users.GetByUserId(evt.HealerUserId)
	mob := mobs.GetInstance(evt.MobInstanceId)
	if c == nil || u == nil || u.Character == nil {
		return events.Continue
	}
	now := time.Now().Unix()
	healer := speakerOf(u, mobs.GetInstance(c.instanceId))
	fromOwner := u.UserId == c.ownerUserId

	if m.mayRemember(c) {
		c.mind.addLine(Line{Speaker: healer, Kind: `event`, Text: fmt.Sprintf(`%s healed you.`, healer)}, m.cfg.WorkingMemoryLines)
		c.mind.addMemory(Memory{
			Unix: now, Kind: `event`, Text: fmt.Sprintf(`%s healed my wounds.`, healer),
			Importance: 5, Emotion: `gratitude`, People: []string{healer}, PlaceId: u.Character.RoomId,
		}, m.cfg.MaxMemories)
	}

	// Being tended by the person you travel with earns a little trust and
	// warmth whatever the model says, three times a day at most.
	if fromOwner && c.mind.ruleChangesSince(`healed`, now-86400) < 3 {
		c.mind.applyOpinion(Opinion{Trust: 1, Affection: 1}, `healed`, `rule`, `healed me`, false)
	}
	if fromOwner && mob != nil && healthPct(&mob.Character) < 25 {
		m.noteMilestone(c, `revived`)
	}

	c.dirty = true
	c.lastSocialUnix = now
	if !fromOwner && !m.strangerMayAsk(u, c) {
		return events.Continue // tended, and remembered, but paced like a question
	}
	c.push(stimulus{Kind: `healed`, Speaker: healer, FromOwner: fromOwner, AskerUserId: u.UserId})
	return events.Continue
}

// witnessAttack is her owner setting about someone who was not fighting:
// a shopkeeper, a local, a child. The game calls that a crime, her
// faction standing pays for it, and so does what she thinks of him.
func (m *AICompanionModule) witnessAttack(userId int, mobInstanceId int) {
	c, ok := m.ctrls[userId]
	if !ok || c.instanceId == 0 {
		return
	}
	victim := mobs.GetInstance(mobInstanceId)
	mob := mobs.GetInstance(c.instanceId)
	u := users.GetByUserId(userId)
	if victim == nil || mob == nil || u == nil || u.Character == nil {
		return
	}
	if victim.Character.RoomId != mob.Character.RoomId || !mob.Character.Perceives(&victim.Character) {
		return // she did not see it
	}
	if !refusesToFight(c.profile, victim) {
		return // a fight, not a crime
	}
	now := time.Now().Unix()
	if last, seen := c.lastAttackBy[-mobInstanceId]; seen && now-last < 600 {
		return
	}
	c.lastAttackBy[-mobInstanceId] = now

	name := victim.Character.Name
	if m.mayRemember(c) {
		c.mind.addLine(Line{Kind: `event`, Text: u.Character.Name + ` set about ` + name + `, who had done nothing.`}, m.cfg.WorkingMemoryLines)
		c.mind.addMemory(Memory{Unix: now, Kind: `event`, Text: u.Character.Name + ` attacked ` + name + `, who had done nothing to anyone.`,
			Importance: 8, Emotion: `disgust`, People: []string{u.Character.Name, name}, PlaceId: mob.Character.RoomId}, m.cfg.MaxMemories)
	}
	c.mind.applyOpinion(Opinion{Trust: -4, Respect: -5, Affection: -4}, `witnessed_crime`, `rule`, `set about `+name, false)
	c.dirty = true
	c.push(stimulus{Kind: `witnessed`, Speaker: u.Character.Name, Text: name, FromOwner: true})
}

// strangerMayAsk paces what a passer-by can prompt of somebody else's
// companion: speaking to her by name, `ask`, a gesture aimed at her, a gift
// or healing. Without it, anyone could stand beside a companion and drive
// model calls until the day's budget was gone. The day's allowance is read
// first, because the cooldown is spent by trying it: a stranger with
// nothing left does not also start a fresh wait. The cooldown lives on the
// asker's own character, so it persists with them, and the day's count is
// kept per asker (dispatch reserves each call against it).
//
// Call it once per thing said or done, and only when a stimulus is about
// to be queued: every call that passes spends the cooldown.
func (m *AICompanionModule) strangerMayAsk(u *users.UserRecord, c *controller) bool {
	if u == nil || u.Character == nil {
		return false
	}
	// With strangers off nothing they prompt is paid for, so their day's
	// allowance does not stop her set-line answer; the cooldown still
	// paces it.
	if m.cfg.StrangerDailyTokens > 0 && !m.strangersOff(c.ownerUserId) &&
		m.fw().Allowance(apiframework.DimCompanionStranger, u.UserId) >= m.cfg.StrangerDailyTokens {
		return false
	}
	// Nor when passers-by together have spent all they may of this owner's
	// companion today (StrangerTokensPerOwner).
	if m.cfg.StrangerTokensPerOwner > 0 && !m.strangersOff(c.ownerUserId) &&
		m.fw().Allowance(apiframework.DimCompanionStrangersFor, c.ownerUserId) >= m.cfg.StrangerTokensPerOwner {
		return false
	}
	if m.cfg.StrangerAskSeconds > 0 {
		tag := fmt.Sprintf(`aicompanion-ask-%d`, c.instanceId)
		if !u.Character.TryCooldown(tag, fmt.Sprintf(`%d real seconds`, m.cfg.StrangerAskSeconds)) {
			return false
		}
	}
	return true
}

// calledBack is the companion hearing her own name from her owner while she
// is somewhere else: she breaks off whatever she was at and walks back. No
// model call, no teleport, and nothing for the owner to type but her name.
func (m *AICompanionModule) calledBack(c *controller, mob *mobs.Mob, u *users.UserRecord) {
	if u == nil || u.Character == nil || c.calledAt == util.GetRoundCount() {
		return
	}
	c.calledAt = util.GetRoundCount()
	if c.travel != nil && c.travel.Purpose == `return` {
		return // already on her way
	}
	c.travel = nil
	remember := m.mayRemember(c)
	if reason := m.startTravel(c, mob, u.Character.RoomId, `return`, false); reason != `` {
		if remember {
			c.mind.addLine(Line{Kind: `event`, Text: u.Character.Name + ` called you, and you could not find the way back.`},
				m.cfg.WorkingMemoryLines)
			c.dirty = true
		}
		return
	}
	if remember {
		c.mind.addLine(Line{Kind: `event`, Text: u.Character.Name + ` called you by name; you started back.`},
			m.cfg.WorkingMemoryLines)
		c.dirty = true
	}
}
