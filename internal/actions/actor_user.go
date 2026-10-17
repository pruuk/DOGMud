package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/progression"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// UserActor adapts a *users.UserRecord so it satisfies the Actor interface.
type UserActor struct {
	User *users.UserRecord
	Room *rooms.Room
}

var _ Actor = (*UserActor)(nil)

// NewUserActor wraps a UserRecord in an Actor for polymorphic combat and
// target-resolution code paths. The returned Actor has Room == nil; callers
// that need GetRoom() to return non-nil should use NewUserActorInRoom or
// set the Room field directly on the resulting concrete *UserActor.
func NewUserActor(u *users.UserRecord) Actor {
	return &UserActor{User: u}
}

// NewUserActorInRoom is NewUserActor with a pre-populated room reference.
// Use this at sites where downstream code calls GetRoom() on the returned
// Actor.
func NewUserActorInRoom(u *users.UserRecord, room *rooms.Room) Actor {
	return &UserActor{User: u, Room: room}
}

func (a *UserActor) GetCharacter() *characters.Character {
	return a.User.Character
}

func (a *UserActor) GetRoom() *rooms.Room {
	return a.Room
}

func (a *UserActor) SendText(cat messaging.Category, msg string) {
	a.User.SendText(cat, msg)
}

func (a *UserActor) SendBlankLine(cat messaging.Category) {
	a.User.SendBlankLine(cat)
}

func (a *UserActor) GetName() string {
	return a.User.Character.Name
}

func (a *UserActor) IsPlayer() bool {
	return true
}

func (a *UserActor) GetUserId() int {
	return a.User.UserId
}

func (a *UserActor) GetMobInstanceId() int {
	return 0
}

func (a *UserActor) AddCondition(conditionId int, source string) {
	a.User.AddCondition(conditionId, source)
}

// AddConditionScaled is the DrinkActor door for a duration-scaled condition;
// it queues through the event path so the holder reads the start notice.
func (a *UserActor) AddConditionScaled(conditionId int, durationMult float64, source string) {
	a.User.AddConditionScaled(conditionId, durationMult, source)
}

// AddConditionMagnitude is the DrinkActor door for an exact trigger count and
// magnitude; it queues through the event path.
func (a *UserActor) AddConditionMagnitude(conditionId int, triggers int, magnitude float64, source string) {
	a.User.AddConditionMagnitude(conditionId, triggers, magnitude, source)
}

func (a *UserActor) OnSkillUse(skillName string) bool {
	return a.User.Character.OnSkillUse(skillName, a.User.UserId)
}

func (a *UserActor) OnStatUse(statName string) bool {
	return a.User.Character.OnStatUse(statName, a.User.UserId)
}

// AwardResolved fires the Best-of progression event for one resolved action on
// the player's own character, carrying the user id so the SkillUsed quest event
// still reaches the quest engine.
func (a *UserActor) AwardResolved(won bool, cands ...progression.Candidate) {
	a.User.Character.AwardResolved(a.User.UserId, won, cands...)
}
