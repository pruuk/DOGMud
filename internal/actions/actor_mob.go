package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/progression"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// MobActor adapts a *mobs.Mob so it satisfies the Actor interface.
type MobActor struct {
	Mob  *mobs.Mob
	Room *rooms.Room
	// Skip lists players this actor's room line leaves out: the listeners an
	// ambient line is over budget for (#317, mobcommands.Ambient). Empty
	// for every other line.
	Skip []int
	// EchoExclude, when set, names the players to leave out of a say's
	// "You hear someone talking." echo in each neighbouring room. Only an
	// ambient say sets it (#317); every other say echoes to everyone.
	EchoExclude func(*rooms.Room) []int
}

var _ Actor = (*MobActor)(nil)

// NewMobActor wraps a Mob in an Actor for polymorphic combat and
// target-resolution code paths. The returned Actor has Room == nil; callers
// that need GetRoom() to return non-nil should use NewMobActorInRoom or set
// the Room field directly on the resulting concrete *MobActor.
func NewMobActor(m *mobs.Mob) Actor {
	return &MobActor{Mob: m}
}

// NewMobActorInRoom is NewMobActor with a pre-populated room reference.
// Use this at sites where downstream code calls GetRoom() on the returned
// Actor.
func NewMobActorInRoom(m *mobs.Mob, room *rooms.Room) Actor {
	return &MobActor{Mob: m, Room: room}
}

func (a *MobActor) GetCharacter() *characters.Character {
	return &a.Mob.Character
}

func (a *MobActor) GetRoom() *rooms.Room {
	return a.Room
}

// SendText is a no-op for mobs — they have no player connection.
func (a *MobActor) SendText(cat messaging.Category, msg string) {}

// SendBlankLine is a no-op for mobs, as SendText is.
func (a *MobActor) SendBlankLine(cat messaging.Category) {}

func (a *MobActor) GetName() string {
	return a.Mob.Character.Name
}

func (a *MobActor) IsPlayer() bool {
	return false
}

func (a *MobActor) GetUserId() int {
	return 0
}

func (a *MobActor) GetMobInstanceId() int {
	return a.Mob.InstanceId
}

func (a *MobActor) AddCondition(conditionId int, source string) {
	a.Mob.AddCondition(conditionId, source)
}

// AddConditionScaled is the DrinkActor door for a duration-scaled condition;
// it queues through the event path, the same as the player door.
func (a *MobActor) AddConditionScaled(conditionId int, durationMult float64, source string) {
	a.Mob.AddConditionScaled(conditionId, durationMult, source)
}

// AddConditionMagnitude is the DrinkActor door for an exact trigger count and
// magnitude; it queues through the event path.
func (a *MobActor) AddConditionMagnitude(conditionId int, triggers int, magnitude float64, source string) {
	a.Mob.AddConditionMagnitude(conditionId, triggers, magnitude, source)
}

func (a *MobActor) OnSkillUse(skillName string) bool {
	return a.Mob.Character.OnSkillUse(skillName, 0)
}

func (a *MobActor) OnStatUse(statName string) bool {
	return a.Mob.Character.OnStatUse(statName, 0)
}

// AwardResolved fires the Best-of progression event for one resolved action on
// the mob's own character. User id 0: a mob has no user record, and a non-zero
// id here would queue a SkillUsed quest event against an unrelated player.
func (a *MobActor) AwardResolved(won bool, cands ...progression.Candidate) {
	a.Mob.Character.AwardResolved(0, won, cands...)
}
