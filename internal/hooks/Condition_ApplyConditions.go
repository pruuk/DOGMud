package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/life"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// ApplyConditions applies a queued condition to its holder and narrates the start.
//
// Every nothing-to-do exit (wrong type, unknown condition, missing holder, an add
// the primitive refused) returns Continue, not Cancel. Cancel stops every
// later listener on the same event, and a listener that merely has nothing to
// do must not veto the event for a second listener such as a client condition bar.
// This is the only listener on events.Condition today; the convention is what keeps
// that true in effect if another is ever added.
func ApplyConditions(e events.Event) events.ListenerReturn {

	evt, typeOk := e.(events.Condition)
	if !typeOk {
		mudlog.Error("Event", "Expected Type", "Condition", "Actual Type", e.Type())
		return events.Continue
	}

	//mudlog.Debug(`Event`, `type`, evt.Type(), `UserId`, evt.UserId, `MobInstanceId`, evt.MobInstanceId, `ConditionId`, evt.ConditionId)

	conditionInfo := conditions.GetConditionSpec(evt.ConditionId)
	if conditionInfo == nil {
		return events.Continue
	}

	var targetChar *characters.Character

	if evt.MobInstanceId > 0 {

		conditionMob := mobs.GetInstance(evt.MobInstanceId)
		if conditionMob == nil {
			return events.Continue
		}

		targetChar = &conditionMob.Character

	} else {

		conditionUser := users.GetByUserId(evt.UserId)
		if conditionUser == nil {
			return events.Continue
		}

		targetChar = conditionUser.Character
	}

	// A condition queued for a life that has since ended is stale: refuse it with no
	// add and no notice. The death cascade bumps LifeEpoch beside its condition
	// strip, so this is the queued half of that strip.
	//
	// The test is the epoch, not IsAlive or DeathQueued, because of the order
	// the queue flushes in. The killing swing queues its CharacterDied first
	// and its on-hit condition second; RouteAttributedDeath then cascades a player
	// Dead -> Respawning -> Alive and clears DeathQueued before this listener
	// runs, so by then the respawned player looks alive and unqueued. Nor may
	// it be DeathQueued alone: a ReviveOnDeath save never ends the life, and
	// the blow's condition still belongs on the revived character. The !IsAlive
	// half refuses a holder observed mid-death, which no legitimate condition
	// targets. Conditions queued after the respawn carry the new epoch and land.
	if evt.LifeEpoch != targetChar.LifeEpoch || !targetChar.IsAlive() {
		return events.Continue
	}

	if evt.ConditionId < 0 {
		targetChar.RemoveCondition(conditionInfo.ConditionId * -1)
		return events.Continue
	}

	// Snapshot whether the condition was already active BEFORE we add/refresh.
	// Used below to suppress start text on a pure refresh — refreshing an
	// already-active condition (e.g. ambusher's mob_idle → add_condition 9 tick)
	// shouldn't re-fire "{actee} disappears into the shadows." every round.
	wasAlreadyActive := targetChar.HasCondition(evt.ConditionId)

	// Apply the condition. A DurationMult of 0 or 1 means the authored duration, and
	// for 1.0 AddConditionScaled is equivalent to AddCondition(id, false): both set
	// TriggersLeft to the spec's TriggerCount with Permanent false, and both
	// refresh an already-held condition in place rather than appending a second copy.
	//
	// The error is NOT discardable. It once reported only an unknown condition id,
	// which conditionInfo above already ruled out, but the primitives now also refuse
	// a poison-flagged condition when the holder carries poison-immunity. Narrating a
	// condition that never landed told an immune player venom was seeping into their
	// bloodstream, so a refusal returns here and nothing below it runs: no start
	// notice, no start_remove_conditions cure, no TrackConditionStarted, no ConditionsTriggered.
	// The same refusal applies on the magnitude path, for a former condition
	// applied through this door instead of synchronously.
	var addErr error
	if evt.Magnitude != 0 || evt.Triggers > 0 {
		addErr = targetChar.AddConditionMagnitude(evt.ConditionId, evt.Triggers, evt.Magnitude, evt.Source)
	} else if evt.DurationMult > 0 && evt.DurationMult != 1.0 {
		addErr = targetChar.AddConditionScaled(evt.ConditionId, evt.DurationMult)
	} else {
		addErr = targetChar.AddCondition(evt.ConditionId, false)
	}
	if addErr != nil {
		return events.Continue
	}

	// A tick_pool condition's per-round amount is computed here, where the
	// record now exists, on a fresh application and a refresh alike, at the
	// applier's scale (a spell's caster scale; 0, meaning 1.0, for the rest).
	setTickAmountAtApply(targetChar, conditionInfo, evt.ConditionId, evt.TickScale)

	//
	// Send the start notice (authored, or the generic line; a secret condition is
	// silent) only on first application, not on refresh.
	//
	// A mob holder has no client, so only room text can reach anyone; without
	// it there is nothing to render and the name and room lookups are skipped.
	startText := conditionInfo.Narration(conditions.PhaseStart)
	holderCanRead := evt.UserId != 0 && len(startText.Actee) > 0
	if !wasAlreadyActive && (holderCanRead || len(startText.Observer) > 0) {
		var charName, charPlainName string
		var holder *users.UserRecord
		var roomId, excludeId int

		if evt.UserId != 0 {
			if u := users.GetByUserId(evt.UserId); u != nil {
				charName = u.Character.GetCharacterName(true)
				charPlainName = u.Character.GetCharacterName(false)
				roomId = u.Character.RoomId
				excludeId = u.UserId
				holder = u
			}
		} else if evt.MobInstanceId != 0 {
			if m := mobs.GetInstance(evt.MobInstanceId); m != nil {
				// The mob tag, not the player one. GetCharacterName(true) tags
				// every name `username`, so a mob holder rendered in the player
				// colour. mobDisplayName is what the spell code already uses.
				charName = m.Character.GetCharacterName(true)
				if r := rooms.LoadRoom(m.Character.RoomId); r != nil {
					charName = mobDisplayName(m, r, 0)
				}
				charPlainName = m.Character.GetCharacterName(false)
				roomId = m.Character.RoomId
			}
		}

		if charName != "" {
			roles := conditionInfo.Narrate(conditions.PhaseStart, charName, charPlainName)
			// The holder is the ACTEE: the condition happens to them. A mob holder
			// has no client, so its line is rendered and dropped.
			if roles.Actee != "" && holder != nil {
				holder.SendText(messaging.CategoryConditionApply, roles.Actee)
			}
			// Visual, not audio. Start text describes what the room SEES
			// ("A warm glow surrounds Alice"), and Room.SendText is never
			// sight-gated, so it reached blind and unsighted observers. M2
			// fixed the same defect for a spell's cast_observer line.
			if roles.Observer != "" {
				if r := rooms.LoadRoom(roomId); r != nil {
					// HidingNames, not plain SendTextVisual. Sight-gating alone
					// leaves the line leaning on tag-based Anonymize, which by
					// its own docstring only strips identity TAGS and cannot
					// see a bare name. M4d PR 1 filed that as a latent defect
					// waiting for the first bare name to be authored; condition
					// 115 authors `{actee_plain}` ("{actee_plain} is raked
					// open, blood welling from ragged claw-wounds."), so it was
					// live. Read in play 2026-09-21: "Cave Crawler is raked
					// open..." among lines that otherwise all said "A figure".
					sendConditionStartRoomText(r, conditionInfo,
						roles.Observer, []string{charPlainName}, excludeId)
				}
			}
		}
	}

	// Remove conditions listed in start_remove_conditions (cure effects)
	if conditionSpec := conditions.GetConditionSpec(evt.ConditionId); conditionSpec != nil && len(conditionSpec.StartRemoveConditions) > 0 {
		for _, removeId := range conditionSpec.StartRemoveConditions {
			targetChar.RemoveCondition(removeId)
		}
	}

	targetChar.TrackConditionStarted(evt.ConditionId)

	//
	// If the condition calls for an immediate triggering
	//
	if conditionInfo.TriggerNow {

		// U5c: BACKSTOP only. A DoT tick that routed through ApplyHarm has
		// already queued an ATTRIBUTED death, and shouldSweepReap skips those.
		// What still lands here is a condition that dropped health by some other
		// route, which has no killer to name.
		if evt.MobInstanceId > 0 && shouldSweepReap(targetChar) {
			mudlog.Debug("U5c backstop", "reason", "unattributed condition-tick death",
				"mob", targetChar.Name, "instanceId", evt.MobInstanceId)
			targetChar.Die(state.ActorRef{}, life.TriggerHealthZero)
		}

	}

	events.AddToQueue(events.ConditionsTriggered{UserId: evt.UserId, MobInstanceId: evt.MobInstanceId, ConditionIds: []int{evt.ConditionId}})

	return events.Continue
}

// sendConditionStartRoomText sends a condition's start room line on the
// visual channel. A darkness source's line is judged as if the room were lit
// (lighting plan 5d, ruling D6): the record is already held when the line
// goes out, so the room is judged with the new darkness in it, and the
// observers it has just blinded would miss "a pall gathers around X". It is
// the mirror of sendConditionEndRoomText's light end line. Every other start
// line is judged by the room as it is.
//
// names is the holder's PLAIN name, handed to HideNames explicitly because a
// start line may author a bare {actee_plain}.
func sendConditionStartRoomText(r *rooms.Room, spec *conditions.ConditionSpec, msg string, names []string, skip ...int) {
	if spec.IsDarknessSource() {
		r.SendTextVisualAsLitHidingNames(messaging.CategoryConditionApply, msg, names, skip...)
		return
	}
	r.SendTextVisualHidingNames(messaging.CategoryConditionApply, msg, names, skip...)
}
