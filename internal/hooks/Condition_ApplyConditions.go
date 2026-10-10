package hooks

import (
	"slices"

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
	// A hide landing on a holder already hidden (Empathic Shroud taking over
	// a sneak, #444) is seen by nobody: no one watched them vanish, so its
	// start room line ("seems to shimmer and fade from view") would only
	// give the hidden holder away.
	hideOnHidden := targetChar.IsHidden() && slices.Contains(conditionInfo.Flags, conditions.Hidden)
	// Who could not make the holder out, taken before the condition lands
	// (#458): the start line is judged by the room as it was, so a hide
	// landing on a visible holder reaches everyone who watched them vanish.
	startUnseenBy := conditionLineUnseenBy(rooms.LoadRoom(targetChar.RoomId), targetChar)

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
	//
	// A darkness's start line is judged against the room as it was before the
	// darkness lands (owner rule, 2026-10-05), so the snapshot comes first.
	startSnap := darknessStartSnapshot(conditionInfo, targetChar, wasAlreadyActive)
	// A ward or a heal replaces its family's other record (owner rulings R4,
	// R6); the add discards it, so its name is read first for the
	// replacement line.
	replaced := familyRivalNames(&targetChar.Conditions, evt.ConditionId)
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

	// The record remembers who applied it and from what, on every door
	// (messaging M6 slice 1, section 1): the newest application owns it.
	targetChar.Conditions.Stamp(evt.ConditionId, evt.Source, evt.Caster)

	// A tick_pool condition's per-round amount is computed here, where the
	// record now exists, on a fresh application and a refresh alike, at the
	// applier's scale (a spell's caster scale; 0, meaning 1.0, for the rest).
	setTickAmountAtApply(targetChar, conditionInfo, evt.ConditionId, evt.TickScale)

	holder, holderKnown := conditionPartyOf(state.ActorRef{UserId: evt.UserId, MobInstanceId: evt.MobInstanceId})
	if holderKnown && len(replaced) > 0 {
		narrateFamilyReplacement(conditionInfo, replaced, holder, startUnseenBy)
	}

	//
	// Send the start lines (authored, or the generic holder line; a secret
	// condition is silent) only on first application, not on refresh. One line
	// per audience: the caster, the holder and the room (narrateConditionStart).
	//
	// A mob holder has no client, so with no caster line and no room text
	// there is nothing to render.
	startText := conditionInfo.Narration(conditions.PhaseStart)
	holderCanRead := evt.UserId != 0 && len(startText.Actee) > 0
	casterCanRead := evt.Caster.UserId != 0 && len(startText.Actor) > 0
	if !wasAlreadyActive && holderKnown && (holderCanRead || casterCanRead || len(startText.Observer) > 0) {
		// The room line is visual and hides names (sendConditionStartRoomText):
		// condition 115 authors a bare {actee_plain}, which tag-based Anonymize
		// cannot see; read in play 2026-09-21 as "Cave Crawler is raked
		// open..." among lines that otherwise all said "A figure". It goes
		// only to the players who perceived the holder before it landed
		// (startUnseenBy, #458).
		narrateConditionStart(conditionInfo, evt, holder, startSnap, hideOnHidden, startUnseenBy)
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

// darknessStartSnapshot is the holder's room as everyone in it could see it
// BEFORE a darkness source lands, for that darkness's start line (owner rule,
// 2026-10-05; lighting plan 5d, ruling D6 as amended). It is nil for any other
// condition, for a refresh (which narrates nothing), and when the holder's
// room is not loaded, so only a darkness pays the room walk.
func darknessStartSnapshot(spec *conditions.ConditionSpec, holder *characters.Character, refresh bool) rooms.VisualSnapshot {
	if refresh || !spec.IsDarknessSource() {
		return nil
	}
	r := rooms.LoadRoom(holder.RoomId)
	if r == nil {
		return nil
	}
	return r.VisualSnapshot()
}

// sendConditionStartRoomText sends a condition's start room line on the
// visual channel. With a snapshot (a darkness source's, from
// darknessStartSnapshot) the line is judged against the room as it was
// before the darkness landed: the record is already held when the line goes
// out, so judged by the room as it now is, the observers it has just blinded
// would miss "a pall gathers around X", while an observer already blind
// before it learns nothing (owner rule, 2026-10-05). Every other start line
// is judged by the room as it is.
//
// names is the holder's PLAIN name, handed to HideNames explicitly because a
// start line may author a bare {actee_plain}.
func sendConditionStartRoomText(r *rooms.Room, snap rooms.VisualSnapshot, msg string, names []string, skip ...int) {
	if snap != nil {
		r.SendTextVisualToSnapshot(snap, messaging.CategoryConditionApply, msg, names, skip...)
		return
	}
	r.SendTextVisualHidingNames(messaging.CategoryConditionApply, msg, names, skip...)
}

// conditionLineUnseenBy lists the players in r who must not read a room line
// about holder: those who do not perceive them, by the rule the quit line
// uses (playersNotPerceiving, characters.Character.Perceives: the holder is
// hidden and the reader has no see-hidden). Every condition room line skips
// them: start (judged before the condition lands), trigger and end (#458).
// The one exception is a hide's own end line, conditionEndLineUnseenBy. A
// hidden mob's own acts skip the same readers (mobActUnseenBy). nil
// for a visible holder, which costs no room walk, and for a nil room.
func conditionLineUnseenBy(r *rooms.Room, holder *characters.Character) []int {
	if r == nil || !holder.IsHidden() {
		return nil
	}
	return playersNotPerceiving(r, holder)
}

// conditionEndLineUnseenBy is conditionLineUnseenBy for an end line. A
// hidden-flagged condition's end line ("emerges from the shadows", "shimmers
// back into view") announces the holder coming back into view, so it reaches
// every watcher, the ones who could not see them most of all. It must not
// wait on the holder's Awareness: a shroud that runs out tells its end line
// in the prune pass before the Validate that reveals the holder.
func conditionEndLineUnseenBy(r *rooms.Room, holder *characters.Character, spec *conditions.ConditionSpec) []int {
	if slices.Contains(spec.Flags, conditions.Hidden) {
		return nil
	}
	return conditionLineUnseenBy(r, holder)
}

// conditionMobNames is a mob holder's names for a condition line: the
// narration name for {actee} (the mob tag, not GetCharacterName(true)'s
// player one, and StripNameAdjectives because look's form carries the
// adjective span, a state tag, #453) and the bare name for {actee_plain}.
// A hidden mob is named too: the line goes only to the readers who perceive
// it (conditionLineUnseenBy), or it is the reveal that brings the mob back
// into view, so the name gives the mob away to no one (#458).
func conditionMobNames(m *mobs.Mob) (name, plainName string) {
	name = m.Character.GetCharacterName(true)
	if r := rooms.LoadRoom(m.Character.RoomId); r != nil {
		name = messaging.StripNameAdjectives(mobSeenName(m, r, 0))
	}
	return name, m.Character.GetCharacterName(false)
}
