package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/life"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// ApplyBuffs applies a queued buff to its holder and narrates the start.
//
// Every nothing-to-do exit (wrong type, unknown buff, missing holder, an add
// the primitive refused) returns Continue, not Cancel. Cancel stops every
// later listener on the same event, and a listener that merely has nothing to
// do must not veto the event for a second listener such as a client buff bar.
// This is the only listener on events.Buff today; the convention is what keeps
// that true in effect if another is ever added.
func ApplyBuffs(e events.Event) events.ListenerReturn {

	evt, typeOk := e.(events.Buff)
	if !typeOk {
		mudlog.Error("Event", "Expected Type", "Buff", "Actual Type", e.Type())
		return events.Continue
	}

	//mudlog.Debug(`Event`, `type`, evt.Type(), `UserId`, evt.UserId, `MobInstanceId`, evt.MobInstanceId, `BuffId`, evt.BuffId)

	buffInfo := buffs.GetBuffSpec(evt.BuffId)
	if buffInfo == nil {
		return events.Continue
	}

	var targetChar *characters.Character

	if evt.MobInstanceId > 0 {

		buffMob := mobs.GetInstance(evt.MobInstanceId)
		if buffMob == nil {
			return events.Continue
		}

		targetChar = &buffMob.Character

	} else {

		buffUser := users.GetByUserId(evt.UserId)
		if buffUser == nil {
			return events.Continue
		}

		targetChar = buffUser.Character
	}

	if evt.BuffId < 0 {
		targetChar.RemoveBuff(buffInfo.BuffId * -1)
		return events.Continue
	}

	// Snapshot whether the buff was already active BEFORE we add/refresh.
	// Used below to suppress start text on a pure refresh — refreshing an
	// already-active buff (e.g. ambusher's mob_idle → add_buff 9 tick)
	// shouldn't re-fire "{source} disappears into the shadows." every round.
	wasAlreadyActive := targetChar.HasBuff(evt.BuffId)

	// Apply the buff. A DurationMult of 0 or 1 means the authored duration, and
	// for 1.0 AddBuffScaled is equivalent to AddBuff(id, false): both set
	// TriggersLeft to the spec's TriggerCount with PermaBuff false, and both
	// refresh an already-held buff in place rather than appending a second copy.
	//
	// The error is NOT discardable. It once reported only an unknown buff id,
	// which buffInfo above already ruled out, but the primitives now also refuse
	// a poison-flagged buff when the holder carries poison-immunity. Narrating a
	// buff that never landed told an immune player venom was seeping into their
	// bloodstream, so a refusal returns here and nothing below it runs: no start
	// notice, no start_remove_buffs cure, no TrackBuffStarted, no BuffsTriggered.
	var addErr error
	if evt.DurationMult > 0 && evt.DurationMult != 1.0 {
		addErr = targetChar.AddBuffScaled(evt.BuffId, evt.DurationMult)
	} else {
		addErr = targetChar.AddBuff(evt.BuffId, false)
	}
	if addErr != nil {
		return events.Continue
	}

	//
	// Send the start notice (authored, or the generic line; a secret buff is
	// silent) only on first application, not on refresh.
	//
	// A mob holder has no client, so only room text can reach anyone; without
	// it there is nothing to render and the name and room lookups are skipped.
	startUser := buffInfo.StartUserNotice()
	holderCanRead := evt.UserId != 0 && startUser != ""
	if !wasAlreadyActive && (holderCanRead || buffInfo.StartRoomText != "") {
		var charName, charPlainName string
		var sendFunc func(string)
		var roomId, excludeId int

		if evt.UserId != 0 {
			if u := users.GetByUserId(evt.UserId); u != nil {
				charName = u.Character.GetCharacterName(true)
				charPlainName = u.Character.GetCharacterName(false)
				roomId = u.Character.RoomId
				excludeId = u.UserId
				sendFunc = func(msg string) { u.SendText(messaging.CategoryBuffApply, msg) }
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
			tCtx := textutil.TokenContext{
				SourceName:      charName,
				SourcePlainName: charPlainName,
			}
			cfg := textutil.SendTextConfig{
				UserSendFunc: sendFunc,
				// Visual, not audio. Start text describes what the room SEES
				// ("A warm glow surrounds Alice"), and Room.SendText is never
				// sight-gated, so it reached blind and unsighted observers. M2
				// fixed the same defect for cast_room_text.
				RoomSendFunc: func(msg string, skip ...int) {
					if r := rooms.LoadRoom(roomId); r != nil {
						r.SendTextVisual(messaging.CategoryBuffApply, msg, skip...)
					}
				},
				ExcludeId: excludeId,
			}
			textutil.SendPhaseText(startUser, buffInfo.StartRoomText, tCtx, "cyan", cfg)
		}
	}

	// Remove buffs listed in start_remove_buffs (cure effects)
	if buffSpec := buffs.GetBuffSpec(evt.BuffId); buffSpec != nil && len(buffSpec.StartRemoveBuffs) > 0 {
		for _, removeId := range buffSpec.StartRemoveBuffs {
			targetChar.RemoveBuff(removeId)
		}
	}

	targetChar.TrackBuffStarted(evt.BuffId)

	//
	// If the buff calls for an immediate triggering
	//
	if buffInfo.TriggerNow {

		// U5c: BACKSTOP only. A DoT tick that routed through ApplyHarm has
		// already queued an ATTRIBUTED death, and shouldSweepReap skips those.
		// What still lands here is a buff that dropped health by some other
		// route, which has no killer to name.
		if evt.MobInstanceId > 0 && shouldSweepReap(targetChar) {
			mudlog.Debug("U5c backstop", "reason", "unattributed buff-tick death",
				"mob", targetChar.Name, "instanceId", evt.MobInstanceId)
			targetChar.Die(state.ActorRef{}, life.TriggerHealthZero)
		}

	}

	events.AddToQueue(events.BuffsTriggered{UserId: evt.UserId, MobInstanceId: evt.MobInstanceId, BuffIds: []int{evt.BuffId}})

	return events.Continue
}
