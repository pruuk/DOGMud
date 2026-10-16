package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"
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
//   - Bonded (driven by the aicompanion module): refused while the module
//     drives it, since parting ways is companion-part's business. With the
//     module switched off it parts peacefully and despawns.
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
		refusal.Prompt(user, "dismiss", refusal.Whom, "dismiss <name>")
		return true, nil
	}

	if len(user.Character.Companions) == 0 {
		refusal.NoCompanion(user)
		return true, nil
	}

	comp := user.Character.GetCompanion(rest)
	if comp == nil {
		user.SendText(messaging.CategoryRefusal, fmt.Sprintf(
			`You have no companion named "%s".`, refusal.Echo(rest),
		))
		return true, nil
	}

	compName := comp.Name
	instanceId := comp.InstanceId
	sourceType := comp.SourceType

	// A bonded companion is a person, not a working: it cannot be dismissed
	// by command. Parting ways happens in conversation, on its own terms.
	// That holds only while something drives THIS companion. With the
	// aicompanion module switched off, its commands (companion-part among
	// them) are gone, the engine still fields the companion from its saved
	// record, and refusing here would leave the owner with no way to end
	// the bond; with the module on but no profile for this companion,
	// nothing will ever take it up, and it is the same.
	if sourceType == characters.CompanionBonded && companionai.DrivesBonded(comp.MobId) {
		user.SendText(messaging.CategoryRefusal, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> is not yours to dismiss. To part ways, tell them.`,
			compName,
		))
		return true, nil
	}

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

	// An undriven bonded companion goes its own way peacefully, as it would
	// by companion-part: it was never a creature bent to the owner's will,
	// so there is no betrayal for it to answer. Its mind, kept by the module,
	// is untouched.
	if sourceType == characters.CompanionBonded {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`You part ways with <ansi fg="mobname">%s</ansi>, who turns away and takes a different road.`,
			compName,
		))
		room.SendTextVisual(messaging.CategoryMobEmote,
			fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi> turns away and takes a different road.`,
				compName,
			),
			user.UserId,
		)
		mob.Command("despawn")
		return true, nil
	}

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
		targeting.Commit(&mob.Character, state.ActorRef{UserId: user.UserId}, targeting.ReasonAttack)
		user.SendText(messaging.CategoryMobEmote, fmt.Sprintf(
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
