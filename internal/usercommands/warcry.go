package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Warcry(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	result := actions.ExecuteWarcry(&actions.UserActor{User: user, Room: room})
	if result.Cost.Status == characters.CostRefused {
		refusal.Cost(user, result.Cost)
		return true, nil
	}

	if result.Crafting {
		refusal.WhileBusy(user, "muster a warcry")
		return true, nil
	}

	if result.AlreadyActive {
		user.SendText(messaging.CategoryRefusal, "Your warcry still echoes. You can't shout it louder.")
		// Refused, but the warcry is riding on the player, which is what a
		// quest step on warcry is checking for.
		notifyWarcry(user, room)
		return true, nil
	}

	if result.OnCooldown {
		refusal.CooldownAfterCost(user, "warcry", result.Cost)
		return true, nil
	}
	if !result.Executed {
		return true, nil
	}

	// The shout always lifts the shouter (ExecuteWarcry applied the record);
	// it only names allies when one is here to hear it.
	if shoutReachesAllies(user) {
		user.SendText(messaging.CategoryWarcry, `<ansi fg="red-bold">You let out a thunderous warcry that ignites the fighting spirit of your allies!</ansi>`)
	} else {
		user.SendText(messaging.CategoryWarcry, `<ansi fg="red-bold">You let out a thunderous warcry that ignites your own fighting spirit!</ansi>`)
	}
	// Heard, not seen: the name follows each listener's sight (sight gates 5b).
	actions.SendHeard(&actions.UserActor{User: user, Room: room}, messaging.CategoryWarcry,
		fmt.Sprintf(`<ansi fg="red-bold"><ansi fg="username">%s</ansi> lets out a thunderous warcry!</ansi>`, user.Character.Name),
	)

	// Apply to party members in the room
	if party := parties.Get(user.UserId); party != nil {
		for _, memberId := range party.GetMembers() {
			if memberId == user.UserId {
				continue
			}
			if memberUser := users.GetByUserId(memberId); memberUser != nil {
				if memberUser.Character.RoomId == user.Character.RoomId {
					_ = memberUser.Character.AddConditionMagnitude(conditions.ConditionIdWarcry, result.Duration, 1.0+result.Bonus, "warcry")
					memberUser.SendText(messaging.CategoryWarcry,
						fmt.Sprintf(`<ansi fg="red-bold"><ansi fg="username">%s</ansi>'s warcry stirs your blood!</ansi>`, user.Character.Name))

					// Apply to this party member's companions in the room
					applyWarcryToCompanions(memberUser, room, result.Bonus, result.Duration)
				}
			}
		}
	}

	// Apply to caster's own companions in the room
	applyWarcryToCompanions(user, room, result.Bonus, result.Duration)

	// Resonant Larynx (shout-stacking): the same breath also looses a rally,
	// under the war-cry cooldown already paid. ApplyRallyEffect applies the
	// rally to the caster; fan it to the same allies the war cry reached.
	if mutations.HasMutationFlag(user.Character.Mutations, "shout-stacking") {
		rb, rd := actions.ApplyRallyEffect(user.Character)
		user.SendText(messaging.CategoryRally, `<ansi fg="cyan-bold">Your layered voice weaves a rallying cry into the same breath!</ansi>`)
		// Heard, not seen, like the warcry line above.
		actions.SendHeard(&actions.UserActor{User: user, Room: room}, messaging.CategoryRally,
			fmt.Sprintf(`<ansi fg="cyan-bold"><ansi fg="username">%s</ansi>'s shout gathers into a rally in the same breath!</ansi>`, user.Character.Name),
		)
		if party := parties.Get(user.UserId); party != nil {
			for _, memberId := range party.GetMembers() {
				if memberId == user.UserId {
					continue
				}
				memberUser := users.GetByUserId(memberId)
				if memberUser == nil || memberUser.Character.RoomId != user.Character.RoomId {
					continue
				}
				_ = memberUser.Character.AddConditionMagnitude(conditions.ConditionIdRally, rd, 1.0+rb, "rally")
				// M1 audit defect: this fold loop is a copy of the primary
				// party loop above that kept the condition apply and dropped the
				// line telling the member. Wording matches rally.go's own
				// member line, because what is being applied here IS a rally.
				memberUser.SendText(messaging.CategoryRally,
					fmt.Sprintf(`<ansi fg="cyan-bold"><ansi fg="username">%s</ansi>'s rallying cry steadies your nerves!</ansi>`, user.Character.Name))
				applyRallyToCompanions(memberUser, room, rb, rd)
			}
		}
		applyRallyToCompanions(user, room, rb, rd)
	}

	// Rhetoric progression is awarded inside actions.ExecuteWarcry,
	// so the mob wrapper gets it too.

	notifyWarcry(user, room)

	return true, nil
}

// notifyWarcry is the quest engine's warcry command notification. It fires
// only when the player ends the command under a warcry, never on a cost,
// busy or cooldown refusal (command_issued fires on those too).
func notifyWarcry(user *users.UserRecord, room *rooms.Room) {
	questengine.NotifyCommand(user, room.RoomId, "warcry")
}

// shoutReachesAllies reports whether a warcry or rally from user would reach
// anyone but the shouter: a party member in the same room, or one of the
// shouter's own companions there. It reads the same membership the fan-out
// loops in Warcry and Rally apply to, so a shout's opening line names allies
// exactly when the fan-out has someone to lift. (A party member's companions
// are reached only through that member, who already counts.)
func shoutReachesAllies(user *users.UserRecord) bool {
	if party := parties.Get(user.UserId); party != nil {
		for _, memberId := range party.GetMembers() {
			if memberId == user.UserId {
				continue
			}
			if memberUser := users.GetByUserId(memberId); memberUser != nil &&
				memberUser.Character.RoomId == user.Character.RoomId {
				return true
			}
		}
	}
	for _, mobInstId := range user.Character.GetCharmIds() {
		if mob := mobs.GetInstance(mobInstId); mob != nil && mob.Character.RoomId == user.Character.RoomId {
			return true
		}
	}
	return false
}

func applyWarcryToCompanions(owner *users.UserRecord, room *rooms.Room, bonus float64, duration int) {
	for _, mobInstId := range owner.Character.GetCharmIds() {
		mob := mobs.GetInstance(mobInstId)
		if mob == nil {
			continue
		}
		if mob.Character.RoomId != owner.Character.RoomId {
			continue
		}
		_ = mob.Character.AddConditionMagnitude(conditions.ConditionIdWarcry, duration, 1.0+bonus, "warcry")
	}
}
