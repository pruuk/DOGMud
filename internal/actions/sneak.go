package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// SneakResult holds the outcome of a Sneak call.
type SneakResult struct {
	// Cost reports whether shared admission paid or refused the attempt.
	Cost characters.CostCommitResult
	// Success is true when the actor entered the hidden state.
	Success bool
	// SpottedBy is the observer who detected the actor; nil when Success is
	// true. It is the observer itself, not a name, so the caller words it at
	// the sneaker's own sight (SpottedLine, #215).
	SpottedBy *characters.Character
	// AlreadyHidden is true when the actor already has the hidden condition.
	AlreadyHidden bool
	// InCombat is true when the actor could not attempt the action because
	// they were engaged in combat.
	InCombat bool
	// RollHappened is true when at least one opposed roll was made against an
	// observer. Callers use this to gate skill progression — no roll means no
	// practice (room was empty).
	RollHappened bool
}

// sneakObserverScore is an observer's side of a sneak contest, the sneak
// command's and a sneaking arrival's alike, and the sight it was priced at.
// An observer who sees nothing hears for the sneaker (CalcHearingScore); one
// who makes out anything looks (CalcDetectionScore). Sight gates close-out,
// #333, owner 2026-10-08.
func sneakObserverScore(observer *characters.Character, room messaging.RoomVisibility) (float64, messaging.SightDecision) {
	sight := messaging.ParticipantSight(observer, room)
	if sight == messaging.SightNone {
		return CalcHearingScore(observer), sight
	}
	return CalcDetectionScore(observer, room), sight
}

// sneakNoticeLine is what an observer who wins a sneak contest reads. seen
// is the line with the sneaker's name tagged (moverName); the name follows
// the observer's sight (HideNames), and an observer who sees nothing only
// heard someone (#215, #333).
func sneakNoticeLine(seen, sneakerName string, sight messaging.SightDecision) string {
	if sight == messaging.SightNone {
		return `You hear someone trying to move quietly.`
	}
	return messaging.HideNames(seen, []string{sneakerName}, sight)
}

// SpottedLine is what a sneaker reads when spotter catches the attempt, the
// spotter named only as far as the SNEAKER's own sight allows (#215): the
// name at clear sight, "a figure" at shapes, "something" when the sneaker
// sees nothing or cannot perceive the spotter (a hidden spotter is never
// named).
func SpottedLine(sneaker *characters.Character, room messaging.RoomVisibility, spotter *characters.Character) string {
	return spottedLineFrom(spottedOpeningHide, sneaker, room, spotter)
}

// The openings of the two spotted lines: the sneak command's attempt, and a
// sneaking arrival caught at the door (EntryDetection).
const (
	spottedOpeningHide   = `You try to blend into the shadows`
	spottedOpeningArrive = `You slip into the room`
)

// spottedLineFrom is SpottedLine with its opening, so the sneak command and a
// sneaking arrival name the spotter by one rule.
func spottedLineFrom(opening string, sneaker *characters.Character, room messaging.RoomVisibility, spotter *characters.Character) string {
	tag := `mobname`
	if spotter.GetUserId() > 0 {
		tag = `username`
	}
	line := opening + ` but <ansi fg="` + tag + `">` +
		spotter.Name + `</ansi> notices you.`
	sight := messaging.ParticipantSight(sneaker, room)
	if !sneaker.Perceives(spotter) {
		sight = messaging.SightNone
	}
	return messaging.HideNames(line, []string{spotter.Name}, sight)
}

// MobIsSneaking derives a mob's sneaking state the same way both mob
// movement call sites must: hidden (the Awareness-backed condition), or the
// misc-data "sneaking" flag set while not yet hidden (Sneak sets it
// synchronously, ahead of the hidden condition's event landing). Walking
// (internal/mobcommands/go.go) and a successful flee
// (internal/hooks/NewRound_DoCombat_helpers.go, handleMobFlee) both call this
// rather than each re-deriving it, so they cannot drift the way flee once
// did by checking only IsHidden. Mirrors the player derivation in
// internal/usercommands/go.go without sharing code with it, since that path
// stays player-only.
func MobIsSneaking(mob *mobs.Mob) bool {
	if mob == nil {
		return false
	}
	if mob.Character.IsHidden() {
		return true
	}
	if flag, ok := mob.Character.GetMiscData(`sneaking`).(bool); ok && flag {
		return true
	}
	return false
}

// Sneak attempts to put actor into the hidden (sneaking) state.
//
// It rolls the actor's sneak score against every observer in the room. A
// player actor's party members and own charmed mobs and companions are
// excluded from the observer checks (alliesOf). If any observer wins the
// opposed roll the attempt fails and SpottedBy is set.
//
// On success the hidden condition (id 9) is applied via the event queue and the
// "sneaking" misc-data key is set immediately so other systems can react
// before the condition processes on the next tick.
//
// Callers are responsible for:
//   - Skill-gate checks (player side only)
//   - Failure cooldowns (player side only)
//   - Skill progression and quest engine notifications
//   - Player-facing messaging
func Sneak(actor Actor) SneakResult {
	char := actor.GetCharacter()

	// Already hidden — nothing to do.
	if char.IsHidden() {
		return SneakResult{AlreadyHidden: true}
	}

	// Can't hide while fighting.
	if char.IsInCombat() {
		return SneakResult{InCombat: true}
	}
	if !char.IsFree() || char.Awareness == nil || char.Awareness.State() != awareness.Visible {
		return SneakResult{}
	}

	room := actor.GetRoom()
	if room == nil {
		return SneakResult{}
	}

	cfg := configs.GetBalanceConfig()
	// The room's light is invariant across every occupant checked below, so
	// it is composed once here rather than inside CalcSneakScoreVsObserver on
	// every iteration of the player and mob loops.
	roomLight := messaging.FixedLight(room.LightLevel())
	cost := admitFullCost(actor, costs.ActionSneak, characters.PoolStamina,
		float64(cfg.SneakBaseStaminaCost))
	if cost.Status == characters.CostRefused {
		return SneakResult{Cost: cost}
	}

	// Transition to Concealing; the activity veto fires here if the actor
	// is busy crafting/casting. On veto error we return without a result
	// since the caller's skill-gate messaging handles the veto path.
	if err := char.Awareness.TransitionToConcealing(
		awareness.ConcealingData{},
		state.TransitionReason{Trigger: awareness.TriggerSneakCommand},
	); err != nil {
		// Activity veto or invalid transition (e.g. already Concealing).
		return SneakResult{Cost: cost}
	}

	// A player actor's side (alliesOf: its party's players and its own
	// charmed mobs and companions) never observes it. A mob actor skips only
	// itself, as before.
	allies := moverAllies{}
	if uid := actor.GetUserId(); uid > 0 {
		allies = alliesOf(actor)
		allies.users[uid] = true
	}

	// Exclude the mob actor itself from the observer lists.
	selfMobId := actor.GetMobInstanceId()

	rollHappened := false

	// Check each player in the room. Score is computed per-observer so
	// that NightVision observers (effectiveLit=true) apply the appropriate
	// light modifier even in a dark room.
	for _, observerId := range room.GetPlayers() {
		if allies.users[observerId] {
			continue
		}
		observer := users.GetByUserId(observerId)
		if observer == nil {
			continue
		}
		sneakScore := CalcSneakScoreVsObserver(char, observer.Character, roomLight)
		observerScore, sight := sneakObserverScore(observer.Character, room)
		rollHappened = true
		success := combat.RunContest(sneakScore, []contest.Entry{{Score: observerScore}}).Success
		if !success {
			// Notify the observing player, as far as its sight allows.
			observer.SendText(messaging.CategorySystem, sneakNoticeLine(
				moverName(actor)+` tries to hide but you notice them.`, actor.GetName(), sight))
			char.Awareness.ResolveConcealment(false, state.TransitionReason{
				Trigger: awareness.TriggerSneakFailed,
			})
			return SneakResult{Cost: cost, SpottedBy: observer.Character, RollHappened: true}
		}
	}

	// Check each mob in the room.
	for _, mobInstanceId := range room.GetMobs() {
		if mobInstanceId == selfMobId || allies.mobs[mobInstanceId] {
			continue // yourself, or your own pet or companion
		}
		m := mobs.GetInstance(mobInstanceId)
		if m == nil {
			continue
		}
		sneakScore := CalcSneakScoreVsObserver(char, &m.Character, roomLight)
		observerScore, _ := sneakObserverScore(&m.Character, room)
		rollHappened = true
		success := combat.RunContest(sneakScore, []contest.Entry{{Score: observerScore}}).Success
		if !success {
			char.Awareness.ResolveConcealment(false, state.TransitionReason{
				Trigger: awareness.TriggerSneakFailed,
			})
			return SneakResult{Cost: cost, SpottedBy: &m.Character, RollHappened: true}
		}
	}

	// All observers failed to spot the actor — transition to Hidden.
	// The condition-#9 mirror cascade in Awareness_Cascades.go handles AddCondition automatically.
	char.Awareness.ResolveConcealment(true, state.TransitionReason{
		Trigger: awareness.TriggerSneakSuccess,
	})
	char.SetMiscData(`sneaking`, true)

	return SneakResult{Cost: cost, Success: true, RollHappened: rollHappened}
}
