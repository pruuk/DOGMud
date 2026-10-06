package usercommands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/lightnotice"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Cancel aborts any in-progress activity (casting, crafting, salvaging).
// Casting: refunds 50% of unspent conviction.
// Crafting: no refund (materials not consumed until completion).
// Salvaging: no refund (item not consumed until completion).
//
// An activity in progress always wins, whatever the argument: players type
// `cancel cast` or `cancel spell` to stop a cast. Only when the user is free
// does `cancel <spell>` end a cancellable condition they hold (see
// cancelCondition).
func Cancel(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	a := user.Character.Activity
	if a == nil || a.IsFree() {
		if rest = strings.TrimSpace(rest); rest != `` {
			return cancelCondition(rest, user)
		}
		user.SendText(messaging.CategorySystem, `You aren't doing anything to cancel.`)
		return true, nil
	}

	switch a.State() {
	case activity.Casting:
		d, _ := a.CastingData()
		// Refund 50% of unspent conviction (existing behavior, preserved).
		unspent := d.TotalConvictionCost - d.ConvictionSpent
		if unspent > 0 {
			refund := unspent / 2
			user.Character.ApplyRestore(characters.PoolConviction, refund)
		}
		_ = a.TransitionToFree(state.TransitionReason{
			Trigger: activity.TriggerCastCancel,
			Actor:   state.ActorRef{UserId: user.UserId},
		})
		user.SendText(messaging.CategorySystem, `You stop casting.`)

	case activity.Crafting:
		_ = a.TransitionToFree(state.TransitionReason{
			Trigger: activity.TriggerCraftCancel,
			Actor:   state.ActorRef{UserId: user.UserId},
		})
		user.SendText(messaging.CategorySystem, `You stop crafting.`)

	case activity.Salvaging:
		_ = a.TransitionToFree(state.TransitionReason{
			Trigger: activity.TriggerSalvageCancel,
			Actor:   state.ActorRef{UserId: user.UserId},
		})
		user.SendText(messaging.CategorySystem, `You stop salvaging.`)
	}
	return true, nil
}

// cancelMinPrefix is the shortest partial name cancel accepts; anything
// shorter must match a name exactly, so `cancel i` cannot end Illumination.
const cancelMinPrefix = 3

// cancelNameMatches reports whether the lowercased token names candidate:
// equal to it, or a prefix of it at least cancelMinPrefix characters long.
func cancelNameMatches(token, candidate string) bool {
	candidate = strings.ToLower(candidate)
	if candidate == `` {
		return false
	}
	return token == candidate || (len(token) >= cancelMinPrefix && strings.HasPrefix(candidate, token))
}

// cancelCondition ends a cancellable condition the user holds. It walks only
// the held, unexpired records, in held order, and ends the first one whose
// spec carries conditions.Cancellable and whose name the token matches (see
// cancelNameMatches): the condition's own name, or the id, any alias or the
// display name of a spell whose ConditionIds grants it. A spell the user does
// not hold a condition from can never be reached.
//
// RemoveCondition only marks the record expired; the light (or whatever the
// condition does) stops at once, and the next NewTurn prune removes the record
// and sends its end narration. The immediate actor line says the cancel
// landed, since the end line arrives a round later.
func cancelCondition(name string, user *users.UserRecord) (bool, error) {
	token := strings.ToLower(name)
	var allSpells map[string]*spells.SpellData

	for _, rec := range user.Character.Conditions.GetConditions() {
		spec := conditions.GetConditionSpec(rec.ConditionId)
		if spec == nil || !slices.Contains(spec.Flags, conditions.Cancellable) {
			continue
		}
		matched := cancelNameMatches(token, spec.Name)
		if !matched {
			if allSpells == nil {
				allSpells = spells.GetAllSpells()
			}
			for _, sd := range allSpells {
				if !slices.Contains(sd.ConditionIds, rec.ConditionId) {
					continue
				}
				if cancelNameMatches(token, sd.SpellId) || cancelNameMatches(token, sd.Name) ||
					slices.ContainsFunc(sd.Aliases, func(a string) bool { return cancelNameMatches(token, a) }) {
					matched = true
					break
				}
			}
		}
		if !matched {
			continue
		}
		// The end line waits for the prune, but it announces a change to the
		// room's light, so it is judged against the room as it is now,
		// before the record stops counting (#220).
		rooms.KeepEndLineSnapshotsBeforeRemoval(user.Character, rec.ConditionId)
		user.Character.RemoveCondition(rec.ConditionId)
		user.SendText(messaging.CategorySystem, `You let the spell go.`)
		// A cancelled glow darkens the room at once; say so with this
		// command, not before the next one.
		lightnotice.Check(user, lightnotice.TriggerCommand)
		return true, nil
	}
	user.SendText(messaging.CategorySystem, fmt.Sprintf(`You have no %s you can let go of.`, name))
	return true, nil
}
