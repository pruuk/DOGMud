package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// SleepOptions are the circumstances of a sleep.
type SleepOptions struct {
	// InBed is true when the sleeper lies down in a bed (a player-housing
	// bed, housing.RoomHasBed). It adds condition BedSleepConditionId, whose
	// recovery statmods double the sleeping regen.
	InBed bool
}

// BedSleepConditionId is Sleeping in a Bed, applied alongside Sleeping (15)
// to a sleeper in a bed.
const BedSleepConditionId = 131

// SleepResult is the structured outcome of a Sleep call.
type SleepResult struct {
	Success bool   // true if the condition was applied (or was already applied — idempotent)
	Reason  string // empty on success; populated on failure (e.g., "in combat")
}

// Sleep applies the Sleeping condition (id 15) to the actor's character. Used by
// the player sleep command, the mob sleep command, and the schedule executor
// (via mob.Command("sleep")).
//
// Fails when the actor is in combat or has Aggro. User actors receive a
// player-visible "You can't sleep right now." message; mob actors fail
// silently — the schedule executor retries on the next idle tick after
// combat ends.
//
// Idempotent: if the actor is already sleeping, returns Success without
// re-applying the condition or re-emitting the room emote.
func Sleep(actor Actor, opts SleepOptions) SleepResult {
	c := actor.GetCharacter()
	if c == nil {
		return SleepResult{Success: false, Reason: "no character"}
	}

	// Idempotent: already sleeping — nothing to do.
	if c.HasConditionFlag(conditions.Sleeping) {
		return SleepResult{Success: true}
	}

	// Combat gate.
	if c.IsInCombat() {
		if actor.IsPlayer() {
			actor.SendText(messaging.CategorySystem,
				"You can't sleep right now.")
		}
		return SleepResult{Success: false, Reason: "in combat"}
	}

	// Apply condition 15 (Sleeping) synchronously, on the character. It cannot go
	// through the user's event path because the idempotence check above reads
	// the Sleeping flag back, so a queued apply would let a second sleep in the
	// same tick emit the room emote twice. (The schedule executor is not the
	// reason: it reads the flag on a later tick, by which time an event would
	// have drained.) Condition 15 is therefore flagged silent-start, and the applier
	// owes the holder the start line: that is what the SendText below is for.
	// The condition YAML has no room text, so the third-person visual is also ours.
	if err := c.AddCondition(15, false); err != nil {
		// Never surface the raw internal error (it leaks the condition id). Log it
		// for ops and give the player clean flavor.
		mudlog.Error("Sleep", "msg", "AddCondition(15 Sleeping) failed", "actor", actor.GetName(), "error", err)
		if actor.IsPlayer() {
			actor.SendText(messaging.CategorySystem,
				"You can't seem to settle into sleep right now.")
		}
		return SleepResult{Success: false, Reason: err.Error()}
	}

	// The start line the silent-start flag makes ours to send. Read through
	// AuthoredStartLine, not StartUserNotice(), which is empty by design for a
	// silent-start condition. A mob holder has no client, so only a player gets it.
	if actor.IsPlayer() {
		if spec := conditions.GetConditionSpec(15); spec != nil {
			// Tagged for {actee}, plain for {actee_plain}: the holder is the
			// actee, and AuthoredStartLine puts it there. Condition 15's line
			// carries no token today, so this is for the day one is authored.
			line := spec.AuthoredStartLine(
				c.GetCharacterName(true),
				c.GetCharacterName(false))
			if line != "" {
				actor.SendText(messaging.CategoryConditionApply, line)
			}
		}
	}

	// A bed adds its own condition (sleepInBed). A failure there costs only
	// the bonus: the sleeper is asleep either way.
	inBed := opts.InBed && sleepInBed(actor, c)

	// Emit third-person room visual to other occupants.
	room := actor.GetRoom()
	if room != nil {
		where := ``
		if inBed {
			where = ` in the bed`
		}
		room.SendTextVisual(messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> lies down%s to sleep.`,
				actor.GetName(), where),
			actor.GetUserId(),
		)
	}

	return SleepResult{Success: true}
}

// sleepInBed adds Sleeping in a Bed to a sleeper lying down in a bed and
// sends its start line (silent-start, applied synchronously, for the same
// reasons as 15). It reports whether the bed took.
func sleepInBed(actor Actor, c *characters.Character) bool {
	if err := c.AddCondition(BedSleepConditionId, false); err != nil {
		mudlog.Error("Sleep", "msg", "AddCondition(Sleeping in a Bed) failed", "actor", actor.GetName(), "error", err)
		return false
	}
	if !actor.IsPlayer() {
		return true
	}
	spec := conditions.GetConditionSpec(BedSleepConditionId)
	if spec == nil {
		return true
	}
	if line := spec.AuthoredStartLine(c.GetCharacterName(true), c.GetCharacterName(false)); line != "" {
		actor.SendText(messaging.CategoryConditionApply, line)
	}
	return true
}
