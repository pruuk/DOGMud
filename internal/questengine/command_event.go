package questengine

import (
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Target is what a command was aimed at, for the command event's target
// filters (#375 phase 3b slice 2). Build it with the constructors below; the
// zero value reads as NoTarget.
type Target struct {
	Kind   string // a quests.Target* kind; "" reads as quests.TargetNone
	MobId  int    // mob template id: the mob aimed at, or the counterparty (sell's shopkeeper)
	ItemId int    // item template id: the item used, aimed at or traded
	Noun   string // the command's own noun: cast's canonical spell id, a container's name
}

// NoTarget is a command aimed at nothing (help, sleep, a grenade that hit no one).
func NoTarget() Target { return Target{Kind: quests.TargetNone} }

// TargetSelf is a command aimed at its own actor (a self-cast).
func TargetSelf() Target { return Target{Kind: quests.TargetSelf} }

// TargetPlayer is a command aimed at another player.
func TargetPlayer() Target { return Target{Kind: quests.TargetPlayer} }

// TargetMob is a command aimed at a mob, by its template id. A nil mob (it
// died or left before the event) is no target.
func TargetMob(m *mobs.Mob) Target {
	if m == nil {
		return NoTarget()
	}
	return Target{Kind: quests.TargetMob, MobId: int(m.MobId)}
}

// TargetItem is a command aimed at, using or trading an item, by its
// template id. Zero (a condition sale, no item) is no target.
func TargetItem(itemId int) Target {
	if itemId <= 0 {
		return NoTarget()
	}
	return Target{Kind: quests.TargetItem, ItemId: itemId}
}

// TargetContainer is a command aimed at a room container, by its name.
func TargetContainer(name string) Target {
	return Target{Kind: quests.TargetContainer, Noun: name}
}

// TargetActor resolves the {userId, mobInstanceId} pair a combat or move
// result carries: the actor's own id is self, another user is a player, a
// live mob instance is that mob's template, and nothing is no target.
func TargetActor(actorUserId, userId, mobInstanceId int) Target {
	switch {
	case userId > 0 && userId == actorUserId:
		return TargetSelf()
	case userId > 0:
		return TargetPlayer()
	case mobInstanceId > 0:
		return TargetMob(mobs.GetInstance(mobInstanceId))
	}
	return NoTarget()
}

// WithMob adds a counterparty mob (sell's shopkeeper) to a target.
func (t Target) WithMob(m *mobs.Mob) Target {
	if m != nil {
		t.MobId = int(m.MobId)
	}
	return t
}

// WithItem adds the item a command used on its target (the item planted).
func (t Target) WithItem(itemId int) Target {
	if itemId > 0 {
		t.ItemId = itemId
	}
	return t
}

// WithNoun sets the command's own noun (cast's canonical spell id).
func (t Target) WithNoun(noun string) Target {
	t.Noun = noun
	return t
}

// EventAt starts an event in a room: the user and the template room
// (rooms.OriginalRoomId, a no-op for a normal room). Every event source
// matches on the template room, so room, area and zone filters hold inside
// an instance; the source's bridge keeps the real room (#375 phase 3b
// slice 4).
func EventAt(userId, roomId int) EventDetails {
	matchRoom, _ := rooms.OriginalRoomId(roomId)
	return EventDetails{UserId: userId, RoomId: matchRoom}
}

// CommandDetails builds the command event for one target. RoomId is the
// template room (rooms.OriginalRoomId), as command_issued and flee already
// match, so room-scoped triggers work in ephemeral instances.
func CommandDetails(userId, roomId int, command string, t Target) EventDetails {
	kind := t.Kind
	if kind == "" {
		kind = quests.TargetNone
	}
	matchRoom, _ := rooms.OriginalRoomId(roomId)
	return EventDetails{
		UserId:     userId,
		RoomId:     matchRoom,
		Command:    command,
		MobId:      t.MobId,
		ItemId:     t.ItemId,
		Noun:       t.Noun,
		TargetKind: kind,
	}
}

// NotifyCommand is THE command event: every production `command` site calls
// it (internal/guards/command_event_guard_test.go holds them to it). It
// raises one event per target under one guard (NotifyAll), so a trigger
// fires at most once per command; no targets means one event aimed at
// nothing. roomId is the room the command happened in; the bridge keeps the
// real id for mob and item resolution.
func NotifyCommand(user *users.UserRecord, roomId int, command string, targets ...Target) NotifyResult {
	if user == nil {
		return NotifyResult{}
	}
	if len(targets) == 0 {
		targets = []Target{NoTarget()}
	}
	details := make([]EventDetails, 0, len(targets))
	for _, t := range targets {
		details = append(details, CommandDetails(user.UserId, roomId, command, t))
	}
	bridge := NewGameBridge(user, roomId)
	return GetEngine().NotifyAll("command", details, bridge, bridge)
}

// SpellLandedDetails builds the spell_landed event for one target: Noun is
// the canonical spell id and Effect the spell's effect_type (#375 phase 3b
// slice 3). The target's own Noun is replaced by the spell id.
func SpellLandedDetails(userId, roomId int, spellId, effect string, t Target) EventDetails {
	d := CommandDetails(userId, roomId, "", t.WithNoun(spellId))
	d.Effect = effect
	return d
}

// NotifySpellLanded is THE spell_landed event: one detail per target the
// spell landed on, under one guard (NotifyAll), so a trigger fires at most
// once per cast. A cast that landed on nothing raises nothing; a spell with
// no creature target that succeeded passes NoTarget. Raised only for player
// casters: the event is matched against the caster's quests.
func NotifySpellLanded(user *users.UserRecord, roomId int, spellId, effect string, targets ...Target) NotifyResult {
	if user == nil || len(targets) == 0 {
		return NotifyResult{}
	}
	details := make([]EventDetails, 0, len(targets))
	for _, t := range targets {
		details = append(details, SpellLandedDetails(user.UserId, roomId, spellId, effect, t))
	}
	bridge := NewGameBridge(user, roomId)
	return GetEngine().NotifyAll("spell_landed", details, bridge, bridge)
}

// NotifyRoomMapped is raised when a room joined the player's map
// (actions.MarkRoomMappedIfSeen returned true): it checks the explore steps
// of every area holding the room (#375 phase 3b slice 4). It raises no event
// name; CheckExplore reads explore triggers from their own index. roomId is
// the room the player stands in; the bridge keeps it and CheckExplore maps
// it to its template.
func NotifyRoomMapped(user *users.UserRecord, roomId int) NotifyResult {
	if user == nil {
		return NotifyResult{}
	}
	bridge := NewGameBridge(user, roomId)
	return GetEngine().CheckExplore(roomId, bridge, bridge)
}

// TopicHeardDetails builds the topic_heard event: the NPC's template id and
// the topic key, in the template room (#375 phase 3b slice 1).
func TopicHeardDetails(userId, roomId, mobId int, topic string) EventDetails {
	matchRoom, _ := rooms.OriginalRoomId(roomId)
	return EventDetails{UserId: userId, RoomId: matchRoom, MobId: mobId, Topic: topic}
}

// NotifyTopicHeard is THE topic_heard event, raised by
// conversation.applyWithHandover once a played topic's effects have landed (a
// menu choice, an accepted offer, a companion's ask for its owner; a refused
// charge raises nothing).
func NotifyTopicHeard(user *users.UserRecord, roomId, mobId int, topic string) NotifyResult {
	if user == nil || mobId <= 0 || topic == "" {
		return NotifyResult{}
	}
	bridge := NewGameBridge(user, roomId)
	return GetEngine().NotifyAll("topic_heard", []EventDetails{TopicHeardDetails(user.UserId, roomId, mobId, topic)}, bridge, bridge)
}
