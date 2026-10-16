package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/refusal"
)

// SleepOptions is reserved for future authoring knobs (bed-item
// bonus, custom emote prose, etc.). Empty for chunk 3.3.
type SleepOptions struct{}

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
// player-visible refusal.WhileFighting line; mob actors fail
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
			refusal.WhileFighting(actor, "sleep")
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

	// Emit third-person room visual to other occupants.
	room := actor.GetRoom()
	if room != nil {
		room.SendTextVisual(messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> lies down to sleep.`,
				actor.GetName()),
			actor.GetUserId(),
		)
	}

	return SleepResult{Success: true}
}
