package mobs

// MobEventTryMobBehavior dispatches a behaviour-tree event on a mob instance.
// behaviortree sets it at init (mobs cannot import behaviortree); the default
// does nothing, for tests and before registration. Tests swap it.
var MobEventTryMobBehavior TryMobBehaviorFunc = func(mobInstanceId int, ctx EventContext) bool {
	return false
}

// FireCaughtPlanting fires caught_planting on a mob that caught someone
// planting an item on it (#375 phase 7). userId is the planter, 0 for a mob.
func FireCaughtPlanting(mob *Mob, userId int) {
	if mob == nil {
		return
	}
	MobEventTryMobBehavior(mob.InstanceId, EventContext{EventType: "caught_planting", UserId: userId})
}

// FireAmbushFoiled fires ambush_foiled on a mob that was missed by, or turned
// aside, an opening strike from hiding, melee or a shot from cover (#375
// phase 7). userId is the striker, 0 for a mob.
func FireAmbushFoiled(mob *Mob, userId int) {
	if mob == nil {
		return
	}
	MobEventTryMobBehavior(mob.InstanceId, EventContext{EventType: "ambush_foiled", UserId: userId})
}
