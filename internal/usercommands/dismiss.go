package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Dismiss severs the player's bond with a companion.
// Behavior depends on how the companion came to be bound:
//   - Charmed (wild animal charmed via the charm spell): the bond-break
//     is thematically a betrayal — the mob turns hostile and remains in
//     the world as a natural mob.
//   - Summoned / Conjured / Raised (companions the player created):
//     these are mage-crafted beings, not independent creatures. Dismiss
//     dissolves them peacefully — no aggro, immediate despawn.
//
// publishReleasedReservation republishes the player's vitals after a companion
// record has been removed.
//
// A fielded companion holds a slice of its owner's Conviction, and
// Char.Vitals carries that figure to the web client as conviction_reserved.
// Nothing else on the dismiss path emits an event, and Char.Vitals is a
// push-only snapshot, so without this the client kept showing a phantom
// reservation for a companion that no longer exists. The prompt bar and
// `status` never had the bug because both read GetPoolReservation live.
//
// It could not be left to the regen tick to correct, either.
// NewRound_AutoHeal only republishes when health actually moved OR when the
// player still has at least one companion, so dismissing the LAST one at full
// health is exactly the case that never self-corrects, and the phantom
// survives until something else happens to the character.
func publishReleasedReservation(user *users.UserRecord) {
	events.AddToQueue(events.CharacterVitalsChanged{UserId: user.UserId})
}

// Syntax: dismiss <name>
func Dismiss(rest string, user *users.UserRecord,
	room *rooms.Room, flags events.EventFlag) (bool, error) {

	rest = strings.TrimSpace(rest)
	if rest == "" {
		user.SendText(messaging.CategorySystem, "Dismiss whom? (dismiss <companion name>)")
		return true, nil
	}

	if len(user.Character.Companions) == 0 {
		user.SendText(messaging.CategorySystem, "You have no companions to dismiss.")
		return true, nil
	}

	comp := user.Character.GetCompanion(rest)
	if comp == nil {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`You have no companion named "%s".`, rest,
		))
		return true, nil
	}

	compName := comp.Name
	instanceId := comp.InstanceId
	sourceType := comp.SourceType

	mob := mobs.GetInstance(instanceId)

	if mob == nil {
		// Companion is offline / already gone — just clean up the record.
		user.Character.RemoveCompanion(instanceId)
		publishReleasedReservation(user)
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`Your bond with <ansi fg="mobname">%s</ansi> fades away.`,
			compName,
		))
		return true, nil
	}

	// Break the charm link if one exists.
	if mob.Character.IsCharmed(user.UserId) {
		mob.Character.RemoveCharm()
	}

	// Remove from CharmedMobs tracking on the player.
	user.Character.TrackCharmed(instanceId, false)

	// Remove the companion record before doing anything that might trigger
	// room-wide combat logic.
	user.Character.RemoveCompanion(instanceId)
	publishReleasedReservation(user)

	isPlayerCrafted := sourceType == characters.CompanionSummoned ||
		sourceType == characters.CompanionConjured ||
		sourceType == characters.CompanionRaised

	if isPlayerCrafted {
		// Mage-crafted companion dissolves peacefully — no aggro, immediate despawn.
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`You release <ansi fg="mobname">%s</ansi>. It dissolves back into the energies that shaped it.`,
			compName,
		))
		room.SendTextVisual(messaging.CategoryMobEmote,
			fmt.Sprintf(
				`<ansi fg="username">%s</ansi> dismisses `+
					`<ansi fg="mobname">%s</ansi>; it dissolves away.`,
				user.Character.Name, compName,
			),
			user.UserId,
		)
		mob.Command("despawn")
		return true, nil
	}

	// Charmed wild creature — the bond-break is a betrayal; it turns hostile.
	user.SendText(messaging.CategorySystem, fmt.Sprintf(
		`You sever the bond with <ansi fg="mobname">%s</ansi>.`,
		compName,
	))

	// The betrayal only lands if the owner is THERE to receive it. A creature
	// dismissed from another room would otherwise acquire aggro it can carry
	// across zones via patrol and pathto -- the griefing shape the expiry path
	// rules out (U10c spec 3.10), violated by the command next door.
	//
	// This is very hard to reach in play: companions follow their owner
	// closely. It is a guard rather than a fix, and it exists so the two exits
	// from a charmed bond cannot disagree about the same anti-grief rule.
	present := mob.Character.RoomId == user.Character.RoomId
	if present {
		mob.Character.SetAggro(user.UserId, 0, characters.DefaultAttack)
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> turns on you with fury!`,
			compName,
		))
	}

	// Room message (exclude the dismissing player — they already saw it).
	room.SendTextVisual(messaging.CategoryMobEmote,
		fmt.Sprintf(
			`<ansi fg="username">%s</ansi> dismisses `+
				`<ansi fg="mobname">%s</ansi>!`,
			user.Character.Name, compName,
		),
		user.UserId,
	)
	// The dismissal itself is always announced; the hostility only when it
	// actually happened.
	if present {
		room.SendTextVisual(messaging.CategoryMobEmote,
			fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi> turns hostile!`,
				compName,
			),
			user.UserId,
		)
	}

	return true, nil
}
