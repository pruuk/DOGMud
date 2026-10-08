package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/lightnotice"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// sendReservationDisclosure tells the player, in bands, when the thing they
// just equipped set more of them aside. Silent otherwise.
//
// Equipping a reserving item used to say nothing at all: the pool maxima moved,
// the reachable pool shrank, and the first the player heard of it was a refusal
// several actions later with nothing to connect it to. The equip is the only
// moment where cause and effect are adjacent, so it is the only place the line
// is worth anything.
//
// The empty-string contract is what keeps it off ordinary equips. Every gate
// lives in Character.ReservationIncreaseNotice, which reports a larger SHARE
// rather than more points, so a plain +Vitality item that grows the pool (and
// with it the point cost of reserves already worn) stays quiet.
func sendReservationDisclosure(user *users.UserRecord, before characters.ReservationTotals) {
	if notice := user.Character.ReservationIncreaseNotice(before); notice != `` {
		user.SendText(messaging.CategorySystem, `<ansi fg="yellow">`+notice+`</ansi>`)
	}
}

// sendReservationReturnDisclosure is the `remove` half, kept next to its twin so
// neither can be changed without the other being read.
//
// U7b shipped the equip line alone, which told the player capacity could be
// taken and never that it came back. Same empty-string contract, same share
// test, so an ordinary remove is as silent as an ordinary equip.
func sendReservationReturnDisclosure(user *users.UserRecord, before characters.ReservationTotals) {
	if notice := user.Character.ReservationDecreaseNotice(before); notice != `` {
		user.SendText(messaging.CategorySystem, `<ansi fg="yellow">`+notice+`</ansi>`)
	}
}

func Equip(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if refuseWhileBusy(user, `change equipment`) {
		return true, nil
	}

	if rest == "all" {
		return Gearup(``, user, room, flags)
	}

	if rest == "" {
		user.SendText(messaging.CategorySystem, `Wear WHAT?`)
		return true, nil
	}

	// Check for arm#N / N.arm suffix for arm slot targeting (arms 1-6).
	// Also supports legacy "arm1" through "arm6" plain suffix.
	targetArmSlot := 0
	restLower := strings.ToLower(rest)
	words := strings.Fields(restLower)
	if len(words) >= 2 {
		lastWord := words[len(words)-1]
		armBase, armNum := util.GetMatchNumber(lastWord)
		if armBase == "arm" && armNum >= 1 && armNum <= 6 {
			targetArmSlot = armNum
		} else if len(lastWord) == 4 && strings.HasPrefix(lastWord, "arm") {
			// Legacy: "arm1" through "arm6" (no separator)
			if n := lastWord[3] - '0'; n >= 1 && n <= 6 {
				targetArmSlot = int(n)
			}
		}
		if targetArmSlot > 0 {
			// Strip the last word from rest
			lastSpaceIdx := strings.LastIndex(rest, " ")
			rest = strings.TrimSpace(rest[:lastSpaceIdx])
		}
	}

	// Check whether the user has an item in their inventory that matches.
	// Equippable-first: prefer something that can actually be worn/wielded
	// (`wear stillwater` → the pendant, not the raw pearl), falling back to
	// the unfiltered match so the fashionable-flavor rejection still fires
	// when the only match truly isn't equipment.
	matchItem, found := user.Character.FindInBackpackWhere(rest, func(it items.Item) bool {
		spec := it.GetSpec()
		return spec.Type == items.Weapon || spec.Subtype == items.Wearable
	})
	if !found {
		matchItem, found = user.Character.FindInBackpack(rest)
	}

	if !found {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`You don't have a "%s" to wear.`, rest))
	} else {

		iSpec := matchItem.GetSpec()
		if iSpec.Type != items.Weapon && iSpec.Subtype != items.Wearable {
			user.SendText(messaging.CategorySystem,
				fmt.Sprintf(`Your <ansi fg="item">%s</ansi> doesn't look very fashionable.`, matchItem.DisplayName()),
			)
			return true, nil
		}

		// Snapshot reservation BEFORE anything touches the equipment set, so the
		// disclosure below can tell the player when the thing they just put on
		// set more of them aside.
		beforeReservation := user.Character.ReservationTotals()

		// A darkness's "puts on" line is judged against the room as it was
		// before the item goes on (owner rule, 2026-10-05), so the snapshot is
		// taken here, before the equip. Only a darkness pays the room walk.
		var beforeDark rooms.VisualSnapshot
		if conditions.AnyDarknessSource(iSpec.WornConditionIds) {
			beforeDark = room.VisualSnapshot()
		}

		// One shared body for both spellings; a named arm only confines where
		// the item goes (spec ruling 11), so the arm path meets Wear's
		// MinStrength, reservation and curse gates like any other equip.
		actor := &actions.UserActor{User: user, Room: room}
		var result actions.EquipItemResult
		if targetArmSlot > 0 {
			result = actions.EquipItemInArm(actor, rest, targetArmSlot)
		} else {
			result = actions.EquipItem(actor, rest)
		}

		if result.Equipped {

			for _, oldItem := range result.DisplacedItems {
				if oldItem.ItemId != 0 {
					user.SendText(messaging.CategorySystem,
						fmt.Sprintf(`You remove your <ansi fg="item">%s</ansi> and return it to your backpack.`, oldItem.DisplayName()),
					)
					room.SendTextVisual(messaging.CategoryEquipment,
						fmt.Sprintf(`<ansi fg="username">%s</ansi> removes their <ansi fg="item">%s</ansi> and stores it away.`, user.Character.Name, oldItem.DisplayName()),
						user.UserId,
					)
				}
			}

			if result.ArmLabel != `` {
				if result.Item.GetSpec().Type == items.Offhand {
					user.SendText(messaging.CategorySystem, fmt.Sprintf(`You equip your <ansi fg="item">%s</ansi> in your %s.`, result.Item.DisplayName(), result.ArmLabel))
				} else {
					user.SendText(messaging.CategorySystem, fmt.Sprintf(`You wield your <ansi fg="item">%s</ansi> in your %s.`, result.Item.DisplayName(), result.ArmLabel))
				}
				room.SendTextVisual(messaging.CategoryEquipment,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> equips their <ansi fg="item">%s</ansi>.`, user.Character.Name, result.Item.DisplayName()),
					user.UserId,
				)
			} else if result.Item.GetSpec().Subtype == items.Wearable {
				// A light says it is lit as it goes on (#260); the light
				// notice that may follow speaks of the room, not the item.
				lightNote := ``
				if conditions.AnyLightSource(result.Item.GetSpec().WornConditionIds) {
					lightNote = ` It casts light around you.`
				}
				user.SendText(messaging.CategorySystem,
					fmt.Sprintf(`You wear your <ansi fg="item">%s</ansi>.%s`, result.Item.DisplayName(), lightNote),
				)
				putsOn := fmt.Sprintf(`<ansi fg="username">%s</ansi> puts on their <ansi fg="item">%s</ansi>.`, user.Character.Name, result.Item.DisplayName())
				// A darkness is judged against the room before it went on (owner
				// rule, 2026-10-05; lighting plan 5d, ruling D6 as amended): it
				// is already worn, so the room as it is now would silence its
				// arrival for everyone it has just blinded, and judging it as
				// lit would name the wearer to someone already blind. Two
				// else-less ifs rather than an if/else:
				// messaging_surface_guard_test.go's walk splits an if/else into
				// separate events and would lose this line's observer, which it
				// tracks.
				snapped := beforeDark != nil && conditions.AnyDarknessSource(result.Item.GetSpec().WornConditionIds)
				if snapped {
					room.SendTextVisualToSnapshot(beforeDark, messaging.CategoryEquipment, putsOn, nil, user.UserId)
				}
				if !snapped {
					room.SendTextVisual(messaging.CategoryEquipment, putsOn, user.UserId)
				}
			} else {
				user.SendText(messaging.CategorySystem,
					fmt.Sprintf(`You wield your <ansi fg="item">%s</ansi>. You're feeling dangerous.`, result.Item.DisplayName()),
				)
				room.SendTextVisual(messaging.CategoryEquipment,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> wields their <ansi fg="item">%s</ansi>.`, user.Character.Name, result.Item.DisplayName()),
					user.UserId,
				)
			}

			sendReservationDisclosure(user, beforeReservation)

			// A light just put on changes the band at once; the notice rides
			// the "You wear" line rather than waiting for the next command.
			if result.Item.GetSpec().Type == items.Light {
				lightnotice.Check(user, lightnotice.TriggerCommand)
			}

			// Trigger any outstanding condition onStart events
			if len(result.Item.GetSpec().WornConditionIds) > 0 {
				for _, condition := range user.Character.Conditions.List {
					if condition.OnStartWaiting {
						user.Character.TrackConditionStarted(condition.ConditionId)
					}
				}
			}

			// Quest engine: command notification
			bridge := questengine.NewGameBridge(user, room.RoomId)
			questengine.GetEngine().Notify("command", questengine.EventDetails{
				UserId:  user.UserId,
				RoomId:  room.RoomId,
				Command: "equip",
			}, bridge, bridge)

		} else if result.Found {
			failureReason := result.FailureReason
			if len(failureReason) <= 1 {
				failureReason = fmt.Sprintf(`You can't figure out how to equip the <ansi fg="item">%s</ansi>.`, matchItem.DisplayName())
			}
			user.SendText(messaging.CategorySystem, failureReason)
		}

	}

	return true, nil
}
