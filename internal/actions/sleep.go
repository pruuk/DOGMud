package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

// SleepOptions is reserved for future authoring knobs (bed-item
// bonus, custom emote prose, etc.). Empty for chunk 3.3.
type SleepOptions struct{}

// SleepResult is the structured outcome of a Sleep call.
type SleepResult struct {
	Success bool   // true if the buff was applied (or was already applied — idempotent)
	Reason  string // empty on success; populated on failure (e.g., "in combat")
}

// Sleep applies the Sleeping buff (id 15) to the actor's character. Used by
// the player sleep command, the mob sleep command, and the schedule executor
// (via mob.Command("sleep")).
//
// Fails when the actor is in combat or has Aggro. User actors receive a
// player-visible "You can't sleep right now." message; mob actors fail
// silently — the schedule executor retries on the next idle tick after
// combat ends.
//
// Idempotent: if the actor is already sleeping, returns Success without
// re-applying the buff or re-emitting the room emote.
func Sleep(actor Actor, opts SleepOptions) SleepResult {
	c := actor.GetCharacter()
	if c == nil {
		return SleepResult{Success: false, Reason: "no character"}
	}

	// Idempotent: already sleeping — nothing to do.
	if c.HasBuffFlag(buffs.Sleeping) {
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

	// Apply buff 15 (Sleeping) synchronously, on the character. It cannot go
	// through the user's event path because the idempotence check above reads
	// the Sleeping flag back, so a queued apply would let a second sleep in the
	// same tick emit the room emote twice. (The schedule executor is not the
	// reason: it reads the flag on a later tick, by which time an event would
	// have drained.) Buff 15 is therefore flagged silent-start, and the applier
	// owes the holder the start line: that is what the SendText below is for.
	// The buff YAML has no room text, so the third-person visual is also ours.
	if err := c.AddBuff(15, false); err != nil {
		// Never surface the raw internal error (it leaks the buff id). Log it
		// for ops and give the player clean flavor.
		mudlog.Error("Sleep", "msg", "AddBuff(15 Sleeping) failed", "actor", actor.GetName(), "error", err)
		if actor.IsPlayer() {
			actor.SendText(messaging.CategorySystem,
				"You can't seem to settle into sleep right now.")
		}
		return SleepResult{Success: false, Reason: err.Error()}
	}

	// The start line the silent-start flag makes ours to send. Read through
	// AuthoredStartLine, not StartUserNotice(), which is empty by design for a
	// silent-start buff. A mob holder has no client, so only a player gets it.
	if actor.IsPlayer() {
		if spec := buffs.GetBuffSpec(15); spec != nil {
			// Tagged for {source}, plain for {source_plain}, the textutil
			// contract every narration site follows. Buff 15's line carries no
			// token today, so this is for the day one is authored.
			line := spec.AuthoredStartLine(textutil.TokenContext{
				SourceName:      c.GetCharacterName(true),
				SourcePlainName: c.GetCharacterName(false),
			})
			if line != "" {
				actor.SendText(messaging.CategoryBuffApply, line)
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
