package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func Pet(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	args := util.SplitButRespectQuotes(rest)

	if len(args) == 0 {
		user.SendText(messaging.CategorySystem, `Pet what?`)
		return true, nil
	}

	if args[0] == `name` {

		if !user.Character.Pet.Exists() {
			user.SendText(messaging.CategorySystem, `You have no pet to name.`)
			return true, nil
		}

		if user.Character.Pet.Name != `` && user.Character.Pet.Name != user.Character.Pet.Type {
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`%s already has a name.`, user.Character.Pet.DisplayName()))
			return true, nil
		}

		newName := strings.Join(args[1:], ` `)

		if err := users.ValidateActorName(newName, users.ValidateActorOpts{}); err != nil {
			user.SendText(messaging.CategorySystem, `That name won't work: `+err.Error())
			return true, nil
		}

		user.Character.Pet.Name = newName

		user.EventLog.Add(`pet`, `Named your pet: `+user.Character.Pet.DisplayName())

		user.SendText(messaging.CategorySystem, fmt.Sprintf(`You name your pet: %s.`, user.Character.Pet.DisplayName()))
		room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> names their pet %s`, user.Character.Name, user.Character.Pet.DisplayName()), user.UserId)

		// rename their pet?
		return true, nil
	}

	petUserId := 0
	if ownPetNamed(rest, user) {
		petUserId = user.UserId
	} else if messaging.ParticipantSight(user.Character, room) != messaging.SightFull {
		// Another player's pet is named, never a shape: below full sight
		// the name is refused the same whether the pet is there or not.
		user.SendText(messaging.CategorySystem, actions.AimNotHereLine)
		return true, nil
	} else {
		petUserId = petOwnerInSight(rest, user, room)
	}
	if petUserId == 0 {
		user.SendText(messaging.CategorySystem, `Can't find that to pet.`)
		return true, nil
	}

	petUser := users.GetByUserId(petUserId)
	if petUser == nil {
		user.SendText(messaging.CategorySystem, `Can't find that to pet.`)
		return true, nil
	}

	user.SendText(messaging.CategorySystem, fmt.Sprintf(`You pet %s`, petUser.Character.Pet.DisplayName()))

	room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> pets %s`, user.Character.Name, petUser.Character.Pet.DisplayName()), user.UserId)

	roll := util.RollDice(1, 4)

	if roll == 1 {
		room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`%s twirls a bit.`, petUser.Character.Pet.DisplayName()))
	} else if roll == 2 {
		room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`%s stiffens.`, petUser.Character.Pet.DisplayName()))
	}

	return true, nil
}

// ownPetNamed reports whether who names user's own pet: the word "pet", or
// the pet's own name in full. The own pet is always at hand, so naming it
// needs no sight and confirms nothing about the room (#454).
func ownPetNamed(who string, user *users.UserRecord) bool {
	pet := user.Character.Pet
	if !pet.Exists() {
		return false
	}
	if who == `pet` {
		return true
	}
	match, _ := util.FindMatchIn(who, pet.PlainName())
	return match != ``
}

// petOwnerInSight is the user id of the player in room whose pet who names,
// or 0. Naming another player's pet is a typed name, so it resolves only at
// full sight (#454), and only when the viewer perceives the pet's owner, as
// a named creature must be perceived.
func petOwnerInSight(who string, user *users.UserRecord, room *rooms.Room) int {
	if messaging.ParticipantSight(user.Character, room) != messaging.SightFull {
		return 0
	}
	ownerId := room.FindByPetName(who)
	if ownerId == 0 || ownerId == user.UserId {
		return ownerId
	}
	owner := users.GetByUserId(ownerId)
	if owner == nil || !user.Character.Perceives(owner.Character) {
		return 0
	}
	return ownerId
}
