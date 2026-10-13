package hooks

import (
	"sync"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Deferred narration for work ended by damage (#514, owner ruling 2026-10-13,
// option A).
//
// Character.ApplyHarm ends a craft or salvage the instant damage lands, and
// damage lands BEFORE the hit line that caused it is sent (melee resolves
// inside rollCombatAttack ahead of its messaging; spells and special moves the
// same). Sending the interrupt lines from the ended hooks would put the effect
// before its cause. So the state change stays at the seam, and the craft-ended
// and salvage-ended hooks record a pending interruption here instead of
// sending. FlushWorkInterruptNarration sends them once the round's combat and
// spell lines are queued. Damage outside round processing waits for the next
// round's flush.
//
// The queue lives here, not on the Character, so characters carries no
// narration state.

type pendingWorkInterrupt struct {
	userId        int
	mobInstanceId int
	roomId        int
	// recipeId names the craft; empty with salvage set.
	recipeId string
	salvage  bool
	phase    crafting.Phase
}

var (
	pendingWorkInterruptsMu sync.Mutex
	pendingWorkInterrupts   []pendingWorkInterrupt
)

// queueWorkInterrupt records a damage-ended craft or salvage for the round-end
// flush. Callers have already applied the dying skip.
func queueWorkInterrupt(p pendingWorkInterrupt) {
	pendingWorkInterruptsMu.Lock()
	pendingWorkInterrupts = append(pendingWorkInterrupts, p)
	pendingWorkInterruptsMu.Unlock()
}

// takeWorkInterrupts drains the queue.
func takeWorkInterrupts() []pendingWorkInterrupt {
	pendingWorkInterruptsMu.Lock()
	defer pendingWorkInterruptsMu.Unlock()
	out := pendingWorkInterrupts
	pendingWorkInterrupts = nil
	return out
}

// FlushWorkInterruptNarration is the NewRound listener that sends the queued
// interrupt lines. hooks.go registers it with events.Last, and a Last listener
// runs after every non-Last listener whenever either was registered, so it
// follows UserRoundTick, MobRoundTick, DoCombat with its spell resolution,
// AutoHeal and every module listener. The lines it queues land behind every
// line those listeners queued this round.
func FlushWorkInterruptNarration(e events.Event) events.ListenerReturn {
	flushWorkInterrupts()
	return events.Continue
}

// flushWorkInterrupts sends what the ended hooks sent before the deferral: the
// player's own line, then (craft only) the room's stop line resolved against
// the room the work was in.
//
// Dropped silently: a crafter who no longer resolves; a crafter who has since
// started dying (a later blow that round killed them, and the death narration
// covers it); and a crafter whose Activity is no longer Free, because damage
// outside the round waited for this flush and they have begun new work since,
// so a stale "pain" line and a stop line for the old recipe would follow
// their "You begin..." line. A room that no longer resolves drops only the
// room line, as before the deferral.
func flushWorkInterrupts() {
	for _, p := range takeWorkInterrupts() {
		sendWorkInterrupt(p)
	}
}

func sendWorkInterrupt(p pendingWorkInterrupt) {
	var (
		char *characters.Character
		user *users.UserRecord
		mob  *mobs.Mob
	)
	switch {
	case p.userId > 0:
		if user = users.GetByUserId(p.userId); user != nil {
			char = user.Character
		}
	case p.mobInstanceId > 0:
		if mob = mobs.GetInstance(p.mobInstanceId); mob != nil {
			char = &mob.Character
		}
	}
	if char == nil || char.Health < 1 || !char.IsAlive() {
		return
	}
	if char.Activity != nil && char.Activity.State() != activity.Free {
		return
	}

	// The player's own line first. "Pain", not "blow": a bleed tick lands here
	// too, with no blow (#515). A mob has no connection to tell.
	if user != nil {
		line := `<ansi fg="red">The pain interrupts your crafting.</ansi>`
		if p.salvage {
			line = `<ansi fg="red">The pain interrupts your salvaging.</ansi>`
		}
		user.SendText(messaging.CategoryCraft, line)
	}
	// Salvage has no room lines yet.
	if p.salvage {
		return
	}
	room := rooms.LoadRoom(p.roomId)
	recipe := crafting.GetRecipe(p.recipeId)
	if room == nil || recipe == nil {
		return
	}
	var crafter actions.Actor
	if user != nil {
		crafter = actions.NewUserActorInRoom(user, room)
	} else {
		crafter = actions.NewMobActorInRoom(mob, room)
	}
	actions.NarrateCraftPhase(crafter, recipe, p.phase, room)
}
