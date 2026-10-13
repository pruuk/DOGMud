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
// interrupt lines. hooks.go registers it with events.Last, so it runs after
// every other NewRound listener (UserRoundTick, MobRoundTick, DoCombat with its
// spell resolution, AutoHeal, module listeners), and the lines it queues land
// behind every line those listeners queued this round.
func FlushWorkInterruptNarration(e events.Event) events.ListenerReturn {
	flushWorkInterrupts()
	return events.Continue
}

// flushWorkInterrupts sends what the ended hooks sent before the deferral: the
// player's own line, then (craft only) the room's stop line resolved against
// the room the work was in. A crafter or room that no longer resolves is
// dropped silently, as is a crafter who has since started dying: a later blow
// that round killed them, and the death narration covers it.
func flushWorkInterrupts() {
	for _, p := range takeWorkInterrupts() {
		sendWorkInterrupt(p)
	}
}

func sendWorkInterrupt(p pendingWorkInterrupt) {
	room := rooms.LoadRoom(p.roomId)
	if room == nil {
		return
	}
	var (
		crafter actions.Actor
		char    *characters.Character
		user    *users.UserRecord
	)
	switch {
	case p.userId > 0:
		user = users.GetByUserId(p.userId)
		if user == nil {
			return
		}
		char = user.Character
		crafter = actions.NewUserActorInRoom(user, room)
	case p.mobInstanceId > 0:
		m := mobs.GetInstance(p.mobInstanceId)
		if m == nil {
			return
		}
		char = &m.Character
		crafter = actions.NewMobActorInRoom(m, room)
	default:
		return
	}
	if char == nil || char.Health < 1 || !char.IsAlive() {
		return
	}

	if p.salvage {
		// Salvage has no room lines yet; only a player salvager is told.
		if user != nil {
			user.SendText(messaging.CategorySystem, `<ansi fg="red">The pain interrupts your salvaging.</ansi>`)
		}
		return
	}

	// "Pain", not "blow": a bleed tick lands here too, with no blow (#515).
	// A mob has no connection to tell. The room line below is sent in addition.
	if user != nil {
		user.SendText(messaging.CategorySystem, `<ansi fg="red">The pain interrupts your crafting.</ansi>`)
	}
	recipe := crafting.GetRecipe(p.recipeId)
	if recipe == nil {
		return
	}
	actions.NarrateCraftPhase(crafter, recipe, p.phase, room)
}
