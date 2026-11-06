package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Sort(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	hasBag := user.Character.Equipment.ComponentBag.ItemId > 0
	hasBandolier := false
	if user.Character.Equipment.Belt.ItemId > 0 {
		beltSpec := user.Character.Equipment.Belt.GetSpec()
		hasBandolier = beltSpec.IsBandolier
	}

	if !hasBag && !hasBandolier {
		user.SendText(messaging.CategoryRefusal, `You don't have a component bag or bandolier equipped.`)
		return true, nil
	}

	materialsMoved := 0
	potionsMoved := 0
	var refused []characters.BandolierRelease

	if hasBag {
		materialsMoved = user.Character.SortComponentItems()
	}
	if hasBandolier {
		potionsMoved, refused = user.Character.SortPotionItems()
	}

	// An ambient bandolier (the Vitalis) names what it will not take (#794).
	told := map[string]bool{}
	for _, rel := range refused {
		name := rel.Item.DisplayName()
		if told[name] {
			continue
		}
		told[name] = true
		user.SendText(messaging.CategoryRefusal, characters.BandolierRefusalLine(user.Character.Equipment.Belt.DisplayName(), rel.Item, rel.Why))
	}

	if materialsMoved == 0 && potionsMoved == 0 {
		if len(refused) > 0 {
			return true, nil
		}
		user.SendText(messaging.CategoryRefusal, `No items found to sort.`)
		return true, nil
	}

	parts := make([]string, 0, 2)
	if materialsMoved > 0 {
		parts = append(parts, fmt.Sprintf(
			`%d material(s) into your %s`,
			materialsMoved, user.Character.Equipment.ComponentBag.DisplayName()))
	}
	if potionsMoved > 0 {
		parts = append(parts, fmt.Sprintf(
			`%d potion(s) into your %s`,
			potionsMoved, user.Character.Equipment.Belt.DisplayName()))
	}

	user.SendText(messaging.CategorySystem, fmt.Sprintf(
		`<ansi fg="green">Sorted %s.</ansi>`,
		strings.Join(parts, " and ")))

	return true, nil
}

// bandolierRotLine tells the player that a potion rotted as it left a
// preserving bandolier (#794; characters.BandolierRotLine).
func bandolierRotLine(beltName string, itm items.Item) string {
	return characters.BandolierRotLine(beltName, itm.DisplayName())
}
