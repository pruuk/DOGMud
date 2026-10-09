// Round ticks for players
package hooks

import (
	"strconv"
	"time"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/conversationadapter"
	"github.com/GoMudEngine/GoMud/internal/conversations"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state/presence"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

//
// Handle mobs that are bored
//

// endStrandedFold ends a fold mob holds through fizzleMobFold, told at each
// reader's sight in the mob's room. A no-op when the mob is not casting.
func endStrandedFold(mob *mobs.Mob) {
	cs, ok := mob.Character.CastingData()
	if !ok {
		return
	}
	if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
		fizzleMobFold(mob, room, cs)
	}
}

func IdleMobs(e events.Event) events.ListenerReturn {

	mc := configs.GetMemoryConfig()

	allMobInstances := mobs.GetAllMobInstanceIds()

	allowedUnloadCt := len(allMobInstances) - int(mc.MobUnloadThreshold)
	if allowedUnloadCt < 0 {
		allowedUnloadCt = 0
	}

	// Handle idle mob behavior
	tStart := time.Now()
	// Chunk 6.4: per-tick sub-timers, broken out of the lumped IdleMobs()
	// total so 6.6 can attribute growth. Accumulated across all mobs, recorded
	// once after the loop (same denominator as the IdleMobs() parent).
	var schedDur, patrolDur, convDur time.Duration
	for _, mobId := range allMobInstances {

		mob := mobs.GetInstance(mobId)
		if mob == nil {
			allowedUnloadCt--
			continue
		}

		// Chunk 5 (Presence): despawn driven by Presence state, not BoredomCounter.
		// PresenceTick (T4) has already transitioned the mob to Despawning;
		// this hook fires the actual removal on the next tick.
		if mob.Character.Presence != nil && mob.Character.Presence.State() == presence.Despawning {
			if allowedUnloadCt > 0 {
				mob.Command(`despawn presence_despawning`)
				allowedUnloadCt--
			}
			continue
		}

		// If they are doing some sort of combat thing,
		// Don't do idle actions
		if mob.Character.IsInCombat() {
			if mob.Character.CurrentCombatTarget().UserId > 0 {
				user := users.GetByUserId(mob.Character.CurrentCombatTarget().UserId)
				if user == nil || user.Character.RoomId != mob.Character.RoomId {
					// A fold in progress ends first (#242), so its fizzle
					// reads before the mumble; endStrandedFold below would
					// only catch it a round later.
					endStrandedFold(mob)
					mob.Command(`emote mumbles about losing their quarry.`)
					targeting.Release(&mob.Character, targeting.ReasonDisengage)
				}
			}
			continue
		}

		// Out of combat, a mob's fold step never runs (it lives in the
		// combat round), so a harmful fold it still holds would hang
		// unspoken. One sweep for every way a caster is released mid-fold
		// (#242): a mob target that walked or fled out
		// (actions.ClearRoomAggroOnDeparture) and any other release. A help
		// fold (an idle self-buff) is left alone: it never resolves out of
		// combat either, but fizzling it would turn every idle buff into a
		// fizzle line, which is a separate fix.
		if cs, ok := mob.Character.CastingData(); ok {
			if spell := spells.GetSpell(cs.SpellId); spell != nil && spell.IsHarm() {
				endStrandedFold(mob)
			}
		}

		// Chunk 5.14: a mob someone is swinging at must not wander off before
		// the swing resolves. The check above is not enough on its own because
		// it reads the mob's OWN combat state, and the mob does not have one
		// yet -- see mobIsTargetedInRoom for the full sequence.
		if mobIsTargetedInRoom(mob.InstanceId, mob.Character.RoomId, roomPlayerIds, userCombatTarget) {
			continue
		}

		// Chunk 3.2: schedule executor. Runs before the path-walker so it can
		// clear stale paths on segment transitions and queue new pathtos before
		// the walker consumes them.
		if mob.ScheduleId != "" {
			tSched := time.Now()
			plan := scheduleTickPlan(mob, gametime.GetDate().Hour24)
			applySchedulePlan(mob, plan)
			schedDur += time.Since(tSched)
		}

		// Chunk 3.4: patrol executor. Reads active_patrol_id stamped by the
		// schedule branch above (if the current schedule segment has
		// activity: patrol), otherwise falls back to mob.PatrolId for
		// standalone patrols. Reads-and-clears the stamp so it doesn't
		// linger across ticks.
		{
			var activePatrolId string
			if id := getMiscDataString(&mob.Character, "active_patrol_id"); id != "" {
				activePatrolId = id
				mob.Character.SetMiscData("active_patrol_id", "")
			}
			if activePatrolId == "" && mob.PatrolId != "" {
				activePatrolId = mob.PatrolId
			}
			if activePatrolId != "" {
				tPatrol := time.Now()
				plan := patrolTickPlan(mob, activePatrolId)
				applyPatrolPlan(mob, plan, activePatrolId)
				patrolDur += time.Since(tPatrol)
			}
		}

		// Chunk 3.6: NPC↔NPC idle conversations.
		// Phase 1: if this mob is already in a conversation, advance the
		// state machine one tick (fires the next line or finalises/aborts).
		// Chunk 6.4: IdleMobs::conversation covers both Phase 1 and Phase 2 below.
		tConv := time.Now()
		if partnerId, ok := mob.Character.GetMiscData(conversations.MiscDataPartnerId).(int); ok && partnerId > 0 {
			conversations.TickConversation(conversationadapter.AdaptMob(mob), partnerId)
		}

		// Phase 2: if fully idle and not on cooldown, roll for a new
		// conversation. ConversationBaseChancePct is a float percent
		// (1.0 = 1 %), so multiply by 100 and roll out of 10 000 to
		// preserve fractional precision.
		if conversationsTriggerEligible(mob) {
			cfg := configs.GetBalanceConfig()
			if util.Rand(10000) < int(float64(cfg.ConversationBaseChancePct)*100) {
				if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
					conversations.TryStart(conversationadapter.AdaptMob(mob), room.GetMobs())
				}
			}
		}
		convDur += time.Since(tConv)

		// Check whether they are currently in the middle of a path, or have one waiting to start.
		// This comes after checks for whether they are currently in a conversation, or in combat, etc.
		if advanceMobPath(mob) {
			continue
		}

		events.AddToQueue(events.MobIdle{MobInstanceId: mobId})

	}

	util.TrackTime(`IdleMobs::schedule`, schedDur.Seconds())
	util.TrackTime(`IdleMobs::patrol`, patrolDur.Seconds())
	util.TrackTime(`IdleMobs::conversation`, convDur.Seconds())
	util.TrackTime(`IdleMobs()`, time.Since(tStart).Seconds())

	return events.Continue
}

// advanceMobPath moves a mob one step along its path. It returns true when the
// mob is busy with its path this round (a step issued, a re-path queued, or a
// tired wait) and the caller must not treat it as idle.
//
// Movement parity 4b: the next step is quoted before it is taken. A mob that
// cannot pay yet keeps its path and waits, re-quoting each round but never
// re-pathing; before, Next() advanced the queue first, so a step the mob never
// took forced a re-path on the next tick. A step it could never pay clears the
// path, which hands the mob to the schedule and patrol fallbacks rather than
// parking it forever. A step it can pay is taken exactly as before.
func advanceMobPath(mob *mobs.Mob) bool {
	currentStep := mob.Path.Current()
	if currentStep == nil && mob.Path.Len() == 0 {
		return false
	}

	// If their currentStep isn't actually the room they are in, they've
	// somehow been moved. Recalculate a new path.
	if currentStep != nil && currentStep.RoomId() != mob.Character.RoomId {
		reDoWaypoints := mob.Path.Waypoints()
		if len(reDoWaypoints) > 0 {
			newCommand := `pathto`
			for _, wpInt := range reDoWaypoints {
				newCommand += ` ` + strconv.Itoa(wpInt)
			}
			mob.Command(newCommand)
			return true
		}
		// if we were unable to come up with a new path, send them home.
		mob.Command(`pathto home`)
		return true
	}

	if nextStep, quote, ok := quoteMobPathStep(mob); ok {
		if quote.OK() {
			mob.Path.Next()
			mob.Command(nextStep.ExitName())
			// Stage 2 caravan: pace caravan crews a shade slower than
			// default mob walking. The noop pushes lastCommandTurn forward
			// so the next path step waits ~1.5s. ~1 step per 5.5s real
			// instead of per 4s, visible in flavor without being painful.
			for _, g := range mob.Groups {
				if g == "caravan" {
					mob.Command("noop", 1.5)
					break
				}
			}
			return true
		}
		if !quote.Never {
			return true // tired: keep the path, try again next round
		}
	}

	mob.Path.Clear()
	if mob.HomeRoomId == mob.Character.RoomId {
		mob.WanderCount = 0
	}
	return false
}

// quoteMobPathStep peeks the mob's next path step and, when it is a step the
// walker would take from where the mob stands (it has not been moved off its
// path, and the room has that exit leading to the step's room), quotes it.
// ok is false when the walker would not take the step at all.
func quoteMobPathStep(mob *mobs.Mob) (mobs.PathRoom, actions.MoveCharge, bool) {
	if cur := mob.Path.Current(); cur != nil && cur.RoomId() != mob.Character.RoomId {
		return nil, actions.MoveCharge{}, false
	}
	nextStep := mob.Path.Peek()
	if nextStep == nil {
		return nil, actions.MoveCharge{}, false
	}
	room := rooms.LoadRoom(mob.Character.RoomId)
	if room == nil {
		return nil, actions.MoveCharge{}, false
	}
	exitInfo, found := room.Exits[nextStep.ExitName()]
	if !found || exitInfo.RoomId != nextStep.RoomId() {
		return nil, actions.MoveCharge{}, false
	}
	return nextStep, actions.QuoteMobStep(mob, nextStep.ExitName()), true
}

// mobPathStepWaiting reports whether the walker will wait on the mob's next
// path step: a step it would take, which the mob cannot afford yet but could
// once rested. The patrol executor reads it so a rest does not count as a
// failed path.
func mobPathStepWaiting(mob *mobs.Mob) bool {
	_, q, ok := quoteMobPathStep(mob)
	return ok && !q.OK() && !q.Never
}

// conversationsTriggerEligible gates the per-tick conversation trigger for a
// mob. It duplicates the conversations package's isFullyIdle logic so the hot
// path doesn't allocate an adapter just to peek at eligibility.
func conversationsTriggerEligible(mob *mobs.Mob) bool {
	if mob == nil {
		return false
	}
	// Already in a conversation.
	if partnerId, ok := mob.Character.GetMiscData(conversations.MiscDataPartnerId).(int); ok && partnerId > 0 {
		return false
	}
	// On cooldown from a recently completed conversation.
	if until, ok := mob.Character.GetMiscData(conversations.MiscDataCooldownUntilRound).(uint64); ok && uint64(util.GetRoundCount()) < until {
		return false
	}
	// Combat or pending aggro.
	if mob.Character.IsInCombat() {
		return false
	}
	// Sleeping mob shouldn't start chatting.
	if mob.Character.HasConditionFlag(conditions.Sleeping) {
		return false
	}
	// Mid-walk on a path — let the mob arrive before striking up a chat.
	if mob.Path.Len() > 0 || mob.Path.Current() != nil {
		return false
	}
	return true
}
