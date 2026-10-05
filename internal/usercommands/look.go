package usercommands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func Look(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	secretLook := flags.Has(events.CmdSecretly)

	isSneaking := user.Character.IsHidden()

	// trim off some fluff
	if len(rest) > 2 {
		if rest[0:3] == `at ` {
			rest = rest[3:]
		}
	}
	if len(rest) > 3 {
		if rest[0:4] == `the ` {
			rest = rest[4:]
		}
	}

	lookAt := rest

	// Every sight rule of look lives in actions.ResolveLook, shared with the
	// mob look (slice 5a): the no-sight refusal, the creature named only at
	// clear sight and only if perceived, the exit's through-sight and lock,
	// the pet at clear sight. This function only words the answer.
	res := actions.ResolveLook(&actions.UserActor{User: user, Room: room}, lookAt)
	if res.Kind == actions.LookDark {
		user.SendText(messaging.CategorySystem, `You can't see anything!`)
		return true, nil
	}
	sight := res.Sight

	events.AddToQueue(events.Looking{
		UserId: user.UserId,
		RoomId: room.RoomId,
		Target: lookAt,
		Hidden: isSneaking,
	})

	switch res.Kind {
	case actions.LookRoom:

		if !secretLook && !isSneaking {
			room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> is looking around.`, user.Character.Name),
				[]string{user.Character.Name},
				user.UserId,
			)

			// Make it a "secret looks" now because we don't want another look message sent out by the lookRoom() func
			secretLook = true
		}
		lookRoom(user, room.RoomId, secretLook || isSneaking)
		return true, nil

	case actions.LookCreature:

		target := res.Target

		if target.IsPlayer() {

			u := target.(*actions.UserActor).User

			if !isSneaking {
				u.SendText(messaging.CategoryMobEmote,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> is looking at you.`, user.Character.Name),
				)

				// The looker and the looked-at each have their own line.
				room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> is looking at <ansi fg="username">%s</ansi>.`, user.Character.Name, u.Character.Name),
					[]string{user.Character.Name, u.Character.Name},
					user.UserId, u.UserId)
			}

			descTxt, _ := templates.Process("character/description", u.Character, user.UserId)
			user.SendText(messaging.CategoryRoomDescription, descTxt)

			itemNames := []string{}
			for _, item := range u.Character.Items {
				itemNames = append(itemNames, item.DisplayName())
			}

			invData := map[string]any{
				`Equipment`: &u.Character.Equipment,
				`ItemNames`: itemNames,
			}

			inventoryTxt, _ := templates.Process("character/inventory-look", invData, user.UserId)
			user.SendText(messaging.CategoryRoomDescription, inventoryTxt)

		} else {

			m := target.(*actions.MobActor).Mob

			if !isSneaking {
				targetName := m.Character.GetMobName(0).String()
				room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> is looking at %s.`, user.Character.Name, targetName),
					[]string{user.Character.Name},
					user.UserId,
				)
			}

			descTxt, _ := templates.Process("character/description", &m.Character, user.UserId)
			user.SendText(messaging.CategoryRoomDescription, descTxt)

			itemNames := []string{}
			for _, item := range m.Character.Items {
				itemNames = append(itemNames, item.DisplayName())
			}

			invData := map[string]any{
				`Equipment`:     &m.Character.Equipment,
				`ItemNames`:     itemNames,
				`HideEquipment`: m.HideEquipmentSlots,
			}

			inventoryTxt, _ := templates.Process("character/inventory-look", invData, user.UserId)
			user.SendText(messaging.CategoryRoomDescription, inventoryTxt)
		}

		return true, nil

	case actions.LookExitTooDark:
		user.SendText(messaging.CategorySystem, `It's too dark to see anything in that direction.`)
		return true, nil

	case actions.LookExitLocked:
		user.SendText(messaging.CategorySystem, fmt.Sprintf("The %s exit is locked.", res.ExitName))
		return true, nil

	case actions.LookExit:
		user.SendText(messaging.CategorySystem, fmt.Sprintf("You peer toward the %s.", res.ExitName))
		if !isSneaking {
			room.SendTextVisualHidingNames(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> peers toward the %s.`, user.Character.Name, res.ExitName), []string{user.Character.Name}, user.UserId)
		}

		lookRoom(user, res.ExitRoomId, secretLook || isSneaking)

		return true, nil
	}
	// fall through to container / noun / pet lookup branches below (LookOther)

	if room.MatchesSealedCrate(strings.ToLower(lookAt)) {
		user.SendText(messaging.CategoryRoomDescription, `A heavy iron-banded shipping crate sits at the roadside, its lid`+
			` latched shut and its sides marked with the caravan's burned-in seal.`+
			` The wood is weather-stained and the latch is recently oiled.`)
		return true, nil
	}

	containerName := room.FindContainerByName(lookAt)
	if containerName != `` {
		if container, exists := room.Containers[containerName]; exists && container.Hidden {
			if user == nil || !user.Character.HasDiscovery(room.RoomId, containerName) {
				containerName = `` // Treat as not found
			}
		}
	}
	if containerName != `` {

		itemNames := []string{}
		itemNamesFormatted := []string{}

		container := room.Containers[containerName]

		if container.Lock.IsLocked() {
			user.SendText(messaging.CategorySystem, ``)
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`The <ansi fg="container">%s</ansi> is locked.`, containerName))
			user.SendText(messaging.CategorySystem, ``)
			return true, nil
		}

		if container.Gold > 0 {
			itemNames = append(itemNames, fmt.Sprintf(`%d gold`, container.Gold))
			itemNamesFormatted = append(itemNamesFormatted, fmt.Sprintf(`<ansi fg="gold">%d gold</ansi>`, container.Gold))
		}

		for _, item := range container.Items {
			if !item.IsValid() {
				room.RemoveItem(item, false)
				continue
			}

			itemNames = append(itemNames, item.Name())
			itemNamesFormatted = append(itemNamesFormatted, fmt.Sprintf(`<ansi fg="itemname">%s</ansi>`, item.DisplayName()))
		}

		if len(container.Recipes) > 0 {

			user.SendText(messaging.CategorySystem, ``)
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`You can <ansi fg="command">use</ansi> the <ansi fg="container">%s</ansi> if you put the following objects inside:`, containerName))

			for finalItemId, recipeList := range container.Recipes {

				neededItems := map[int]int{}

				for _, inputItemId := range recipeList {
					neededItems[inputItemId] += 1
				}

				user.SendText(messaging.CategorySystem, ``)

				finalItem := items.New(finalItemId)
				user.SendText(messaging.CategorySystem, fmt.Sprintf(`    <ansi fg="230">To receive 1 <ansi fg="itemname">%s</ansi>:</ansi> `, finalItem.DisplayName()))

				for inputItemId, qtyNeeded := range neededItems {
					tmpItem := items.New(inputItemId)
					totalContained := container.Count(inputItemId)
					colorClass := "8" // None fulfilled
					if totalContained == qtyNeeded {
						colorClass = "10"
					} else if totalContained > 0 {
						colorClass = "3"
					}
					user.SendText(messaging.CategorySystem, fmt.Sprintf(`        <ansi fg="%s">[%d/%d]</ansi> <ansi fg="itemname">%s</ansi>`, colorClass, totalContained, qtyNeeded, tmpItem.DisplayName()))
				}

			}

		}

		chestStuff := map[string]any{
			`ItemNames`:          itemNames,
			`ItemNamesFormatted`: itemNamesFormatted,
		}

		textOut, _ := templates.Process("descriptions/insidecontainer", chestStuff, user.UserId)

		user.SendText(messaging.CategoryRoomDescription, ``)
		user.SendText(messaging.CategoryRoomDescription, textOut)
		user.SendText(messaging.CategoryRoomDescription, ``)

		return true, nil
	}

	// If the input is a recognized direction alias but no exit exists,
	// stop here — never fall through to item/mob matching.
	if alias := keywords.TryDirectionAlias(lookAt); alias != lookAt {
		user.SendText(messaging.CategorySystem, "There is no exit in that direction.")
		return true, nil
	}

	//
	// A noun on something you wear or carry (lighting plan 5a). Exact match,
	// and before item matching, so "hood" reaches the lantern's hood rather
	// than the lantern.
	//
	if noun, desc, ok := user.Character.FindItemNoun(lookAt); ok {
		user.SendText(messaging.CategoryRoomDescription, ``)
		user.SendText(messaging.CategoryRoomDescription, fmt.Sprintf(`You look at the <ansi fg="noun">%s</ansi>:`, noun))
		user.SendText(messaging.CategoryRoomDescription, ``)
		user.SendText(messaging.CategoryRoomDescription, util.SplitStringNL(desc, 80))
		user.SendText(messaging.CategoryRoomDescription, ``)
		return true, nil
	}

	//
	// Check for anything in their backpack they might want to look at
	//
	lookItem, lookDestination, foundItem := user.Character.FindItem(lookAt)

	if foundItem {

		user.SendText(messaging.CategoryRoomDescription, ``)

		user.SendText(messaging.CategoryRoomDescription,
			fmt.Sprintf(`You look at the <ansi fg="item">%s</ansi> %s:`, lookItem.DisplayNameFor(user.UserId), lookDestination),
		)

		user.SendText(messaging.CategoryRoomDescription, ``)

		if !isSneaking {
			room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> is admiring their <ansi fg="item">%s</ansi>.`, user.Character.Name, lookItem.DisplayName()),
				[]string{user.Character.Name},
				user.UserId,
			)
		}

		// Highlight the item's nouns, then wrap, the same order the room-noun
		// branch below uses, so a multi-word noun is never split by a wrap
		// before it can be matched.
		itemDesc := lookItem.LongDescriptionFor(user.UserId)
		for noun := range lookItem.GetSpec().Nouns {
			itemDesc = strings.Replace(itemDesc, noun, `<ansi fg="noun">`+noun+`</ansi>`, 1)
		}
		user.SendText(messaging.CategoryRoomDescription, util.SplitStringNL(itemDesc, 80))

		// Show potion aging info based on alchemy skill
		lookSpec := lookItem.GetSpec()
		if lookSpec.Aging.HasAging() && lookItem.CraftedRound > 0 {
			elapsed := util.GetRoundCount() - lookItem.CraftedRound
			bottleMult := lookItem.BottleMultiplier
			if bottleMult <= 0 {
				bottleMult = lookSpec.BottleAgingMultiplier
			}
			effSpeed := items.CalcEffectiveAgingSpeed(bottleMult, lookItem.CraftSkill)
			phase, _ := items.GetAgingPhase(elapsed, lookSpec.Aging, effSpeed)
			alchSkill := user.Character.GetSkillLevel(skills.Alchemy)
			desc := items.GetPhaseDescription(
				phase, alchSkill, elapsed, lookSpec.Aging, effSpeed)
			if desc != "" {
				user.SendText(messaging.CategoryRoomDescription,
					fmt.Sprintf(` - <ansi fg="yellow-bold">%s</ansi>`, desc),
				)
			}
		}

		user.SendText(messaging.CategoryRoomDescription, ``)

		return true, nil
	}

	//
	// A floor item named in full, in more than one word ("arch lantern"),
	// beats a room noun one of those words matches ("arch").
	//
	if floorItem, found := floorItemNamedInFull(room, lookAt); found {
		lookAtFloorItem(user, room, floorItem, isSneaking)
		return true, nil
	}

	//
	// Look for any nouns in the room info
	//
	foundNoun, foundDesc := room.FindNoun(lookAt)
	if foundNoun == "" && user != nil {
		// Check discovered hidden nouns
		if key, hn, ok := room.FindHiddenNoun(lookAt); ok {
			if user.Character.HasDiscovery(room.RoomId, key) {
				foundNoun = key
				foundDesc = hn.Description
			}
		}
	}
	if len(foundNoun) > 0 {

		user.SendText(messaging.CategoryRoomDescription, ``)

		user.SendText(messaging.CategoryRoomDescription,
			fmt.Sprintf(`You look at the <ansi fg="noun">%s</ansi>:`, foundNoun),
		)

		user.SendText(messaging.CategoryRoomDescription, ``)

		if !isSneaking {

			// Noun highlighting is universal (2026-06-12). It was formerly
			// gated on the room.nouns role permission or a pet with the
			// SeeNouns condition flag — but the default user role can never hold
			// permissions and no dogmud-world condition carries see-nouns, so the
			// gate made the feature admin-only by accident. Discoverability
			// for everyone beats a vestigial perk.
			renderNouns := true

			if renderNouns && len(room.Nouns) > 0 {
				for noun, _ := range room.Nouns {
					foundDesc = strings.Replace(foundDesc, noun, `<ansi fg="noun">`+noun+`</ansi>`, 1)
				}
			}

			room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> is examining the <ansi fg="noun">%s</ansi>.`, user.Character.Name, foundNoun),
				[]string{user.Character.Name},
				user.UserId,
			)
		}

		user.SendText(messaging.CategoryRoomDescription, util.SplitStringNL(foundDesc, 80))

		user.SendText(messaging.CategoryRoomDescription, ``)

		return true, nil
	}

	//
	// Look for any pets in the room
	//
	// A pet is a creature too: by name only with faces (see above).
	if petUserId := res.PetUserId; petUserId > 0 {
		if petUser := users.GetByUserId(petUserId); petUser != nil {

			user.SendText(messaging.CategoryRoomDescription, fmt.Sprintf(`You look at %s`, petUser.Character.Pet.DisplayName()))

			room.SendTextVisualHidingNames(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> is looking at %s.`, user.Character.Name, petUser.Character.Pet.DisplayName()), []string{user.Character.Name}, user.UserId)

			textOut, _ := templates.Process("character/pet", petUser, user.UserId)
			user.SendText(messaging.CategoryRoomDescription, textOut)

			return true, nil
		}
	}

	if len(room.Corpses) > 0 {

		// Corpse name resolution lives in room.FindCorpse below. An earlier
		// deduplicating index was built here (two maps and two slices) whose
		// only consumers were each other, so it computed a lookup table and
		// then threw it away on every look. Removed; see review finding 31.
		if corpse, corpseFound := room.FindCorpse(rest); corpseFound {

			corpseColor := `mob-corpse`
			// A player corpse is named "<name> corpse" inside a user-corpse
			// tag, which Anonymize does not strip, so the dead player's name
			// is hidden from a shapes-only observer by name. The observer
			// line calls it "the corpse of <name>" so the hidden form reads
			// "the corpse of a figure" (not "the a figure corpse").
			hidden := []string{user.Character.Name}
			observedCorpse := corpse.DisplayName()
			if corpse.UserId > 0 {
				corpseColor = `user-corpse`
				if corpse.CorpseName == "" {
					observedCorpse = `corpse of ` + corpse.Character.Name
				}
				hidden = append(hidden, corpse.Character.Name)
			}

			user.SendText(messaging.CategoryRoomDescription, fmt.Sprintf(`You look at the <ansi fg="%s">%s</ansi>.`, corpseColor, corpse.DisplayName()))
			room.SendTextVisualHidingNames(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> is looking at the <ansi fg="%s">%s</ansi>.`, user.Character.Name, corpseColor, observedCorpse), hidden, user.UserId)

			if corpse.CorpseDescription != "" {
				user.SendText(messaging.CategoryRoomDescription, corpse.CorpseDescription)
			} else {
				descTxt, _ := templates.Process("character/description-corpse", &corpse.Character, user.UserId)
				user.SendText(messaging.CategoryRoomDescription, descTxt)
			}

			// Corpse-loot redesign (2026-07-07): show what can be looted,
			// mirroring the room-container listing.
			if corpse.HasLoot() {

				itemNames := []string{}
				itemNamesFormatted := []string{}

				if corpse.Loot.Gold > 0 {
					itemNames = append(itemNames, fmt.Sprintf(`%d gold`, corpse.Loot.Gold))
					itemNamesFormatted = append(itemNamesFormatted, fmt.Sprintf(`<ansi fg="gold">%d gold</ansi>`, corpse.Loot.Gold))
				}

				for _, item := range corpse.Loot.Items {
					if !item.IsValid() {
						continue
					}
					itemNames = append(itemNames, item.Name())
					itemNamesFormatted = append(itemNamesFormatted, fmt.Sprintf(`<ansi fg="itemname">%s</ansi>`, item.DisplayName()))
				}

				corpseStuff := map[string]any{
					`ItemNames`:          itemNames,
					`ItemNamesFormatted`: itemNamesFormatted,
				}

				textOut, _ := templates.Process("descriptions/insidecontainer", corpseStuff, user.UserId)
				user.SendText(messaging.CategoryRoomDescription, textOut)
			}

			return true, nil

		}

	}

	//
	// Items lying on the floor. Checked last, so an exit, a noun, a creature
	// or a corpse of the same name always wins. A bauble left lying (in a
	// household, or its finder could not carry it) is found here by any word
	// of its name.
	//
	// Not gated beyond the SightNone refusal at the top: at shapes the room
	// look still lists what lies on the ground (lookRoom), and only a
	// creature's name is withheld there (lighting plan 5c), so an item the
	// listing shows can be looked at.
	//
	if floorItem, found := room.FindOnFloor(lookAt, false); found {
		lookAtFloorItem(user, room, floorItem, isSneaking)
		return true, nil
	}

	// Nothing found. At shapes the name may have been a creature's, which
	// was deliberately not resolved above; say why rather than deny it.
	if sight == messaging.SightShapes {
		user.SendText(messaging.CategorySystem, `You can only make out shapes here.`)
		return true, nil
	}
	user.SendText(messaging.CategorySystem, "Look at what???")

	return true, nil

}

// floorItemNamedInFull finds a floor item the words name in full, as more
// than one word ("arch lantern"). A multi-word full name is specific: it
// outranks a room noun or a carried item that only one of its words matches.
func floorItemNamedInFull(room *rooms.Room, lookAt string) (items.Item, bool) {
	if !strings.Contains(strings.TrimSpace(lookAt), ` `) {
		return items.Item{}, false
	}
	for i := range room.Items {
		if _, full := room.Items[i].NameMatch(lookAt, false); full {
			return room.Items[i], true
		}
	}
	return items.Item{}, false
}

// lookAtFloorItem shows an item lying on (or fixed in) the room's floor.
func lookAtFloorItem(user *users.UserRecord, room *rooms.Room, floorItem items.Item, isSneaking bool) {
	// A found bauble lies somewhere in particular ("on the bookshelf").
	where := `on the ground`
	if floorItem.IsBauble() && floorItem.BaubleSpot != `` {
		where = floorItem.BaubleSpot
	}
	// A fixture is part of the room, not lying on its floor (R9).
	if floorItem.IsFixture() {
		where = `here`
	}

	user.SendText(messaging.CategoryRoomDescription, ``)
	user.SendText(messaging.CategoryRoomDescription,
		fmt.Sprintf(`You look at the <ansi fg="item">%s</ansi> %s:`, floorItem.DisplayNameFor(user.UserId), where),
	)
	user.SendText(messaging.CategoryRoomDescription, ``)

	if !isSneaking {
		room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="username">%s</ansi> is looking at the <ansi fg="item">%s</ansi> %s.`, user.Character.Name, floorItem.DisplayName(), where),
			[]string{user.Character.Name},
			user.UserId,
		)
	}

	user.SendText(messaging.CategoryRoomDescription,
		util.SplitStringNL(floorItem.LongDescriptionFor(user.UserId), 80),
	)
	if floorItem.BaubleBelongsTo(room.RoomId) {
		user.SendText(messaging.CategoryRoomDescription,
			`It belongs to this household. Taking it would be theft.`)
	}
	user.SendText(messaging.CategoryRoomDescription, ``)
}

func lookRoom(user *users.UserRecord, roomId int, secretLook bool) {

	room := rooms.LoadRoom(roomId)

	if room == nil {
		return
	}

	// Make sure to prepare the room before anyone looks in if this is the first time someone has dealt with it in a while.
	if room.PlayerCt() < 1 {
		room.Prepare(true)
	}

	if !secretLook {
		// Find the exit back
		lookFromName := room.FindExitTo(user.Character.RoomId)
		if lookFromName == "" {
			room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> is looking into the room from somewhere...`, user.Character.Name),
				[]string{user.Character.Name},
				user.UserId,
			)
		} else {
			room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> is looking into the room from the <ansi fg="exit">%s</ansi> exit`, user.Character.Name, lookFromName),
				[]string{user.Character.Name},
				user.UserId,
			)
		}
	}

	var details rooms.RoomTemplateDetails

	tinyMapOn := user.GetConfigOption(`tinymap`)
	if tinyMapOn == nil {
		tinyMapOn = true
	}

	if user.ScreenReader {
		tinyMapOn = false
	}

	// Web-client sessions already have the graphical Map panel, so the inline
	// ASCII minimap beside room descriptions is redundant and only steals
	// horizontal space from the room text. Suppress it automatically for them.
	if connections.IsWebsocket(user.ConnectionId()) {
		tinyMapOn = false
	}

	if tinyMapOn.(bool) && roomId > 0 {

		zMapper := mapper.GetMapper(room.RoomId)
		if zMapper == nil {

			mudlog.Error("Map", "error", "Could not find mapper for zone:"+room.Zone)

			details = rooms.GetDetails(room, user)

		} else {

			c := mapper.Config{
				ZoomLevel: 1,
				Width:     5,
				Height:    5,
				UserId:    user.UserId,
			}

			c.OverrideSymbol(roomId, '@', `You`)

			output := zMapper.GetLimitedMap(room.RoomId, c)
			tinyMap := []string{}
			tinyMap = append(tinyMap, `╔═════╗`)
			for _, mapLine := range output.Render {
				tinyMap = append(tinyMap, `║`+string(mapLine)+`║`)
			}
			tinyMap = append(tinyMap, `╚═════╝`)
			// This additional check is for ephemeral room copies,
			// which can slightly mess with the map render of the @
			if tinyMap[3][3] != '@' {
				youLine := []rune(tinyMap[3])
				youLine[3] = '@'
				tinyMap[3] = string(youLine)
			}

			legend := output.GetLegend(keywords.GetAllLegendAliases(room.Zone))

			for i := 1; i <= c.Height; i++ {
				for sym, txtLegend := range legend {
					txtLc := strings.ToLower(txtLegend)
					tinyMap[i] = strings.Replace(tinyMap[i], string(sym), fmt.Sprintf(`<ansi fg="map-room"><ansi fg="map-%s" bg="mapbg-%s">%c</ansi></ansi>`, txtLc, txtLc, sym), -1)
				}
			}

			details = rooms.GetDetails(room, user, tinyMap)

		}

	} else {
		details = rooms.GetDetails(room, user)
	}

	textOut, _ := templates.Process("descriptions/room-title", details, user.UserId)
	user.SendText(messaging.CategoryRoomDescription, textOut)

	textOut, _ = templates.Process("descriptions/room", details, user.UserId)
	user.SendText(messaging.CategoryRoomDescription, textOut)

	// Fixtures are part of the room, not things lying on its floor (lighting
	// 5e, ruling R9): a line each, lit or unlit, right after the description
	// and under its sight rule.
	if lines := fixtureLines(room); len(lines) > 0 {
		textOut, _ = templates.Process("descriptions/fixtures", map[string]any{
			`Fixtures`: lines,
			`IsDark`:   details.IsDark,
			`IsNight`:  details.IsNight,
		}, user.UserId)
		user.SendText(messaging.CategoryRoomDescription, textOut)
	}

	// Append discovered hidden noun descriptions.
	// No user nil check: command dispatch always supplies a live user, and
	// user is already dereferenced above (mapper config, template calls).
	// The old check here was unreachable and made the earlier dereferences
	// look like bugs to static analysis.
	if room.HiddenNouns != nil {
		hiddenKeys := make([]string, 0, len(room.HiddenNouns))
		for k := range room.HiddenNouns {
			hiddenKeys = append(hiddenKeys, k)
		}
		sort.Strings(hiddenKeys)
		for _, key := range hiddenKeys {
			if user.Character.HasDiscovery(room.RoomId, key) {
				hn := room.HiddenNouns[key]
				if hn.HiddenDescription != "" {
					user.SendText(messaging.CategoryRoomDescription, hn.HiddenDescription)
				}
			}
		}
	}

	signCt := 0
	privateSigns := room.GetPrivateSigns()
	for _, sign := range privateSigns {
		if sign.VisibleUserId == user.UserId {
			signCt++
			textOut, _ = templates.Process("descriptions/rune", sign, user.UserId)
			user.SendText(messaging.CategoryRoomDescription, textOut)
		}
	}

	publicSigns := room.GetPublicSigns()
	for _, sign := range publicSigns {
		signCt++
		textOut, _ = templates.Process("descriptions/sign", sign, user.UserId)
		user.SendText(messaging.CategoryRoomDescription, textOut)
	}

	if signCt > 0 {
		user.SendText(messaging.CategoryRoomDescription, "")
	}

	textOut, _ = templates.Process("descriptions/who", details, user.UserId)
	if len(textOut) > 0 {
		user.SendText(messaging.CategoryRoomDescription, textOut)
	}

	groundStuff := []string{}
	for containerName, container := range room.Containers {

		// Skip hidden containers the user hasn't discovered
		if container.Hidden {
			if user == nil || !user.Character.HasDiscovery(room.RoomId, containerName) {
				continue
			}
		}

		chestName := fmt.Sprintf(`<ansi fg="container">%s</ansi>`, containerName)

		if container.HasLock() {
			if container.Lock.IsLocked() {
				chestName += ` <ansi fg="white">(locked)</ansi>`
			} else {
				chestName += ` <ansi fg="white">(unlocked)</ansi>`
			}
		}

		groundStuff = append(groundStuff, chestName)

	}

	if room.Gold > 0 {
		groundStuff = append(groundStuff, fmt.Sprintf(`<ansi fg="gold">%d gold</ansi>`, room.Gold))
	}

	// Stack identical floor items for display
	type groundStack struct {
		name  string
		count int
	}
	groundStackOrder := []string{}
	groundStacks := map[string]*groundStack{}

	for _, item := range room.Items {
		if !item.IsValid() {
			room.RemoveItem(item, false)
			continue
		}
		// A fixture shows as part of the room, above (R9).
		if item.IsFixture() {
			continue
		}
		// Baubles share one ItemId; each is its own object with its own name.
		key := fmt.Sprintf("%d|%s|%d|%s", item.ItemId, item.EnchantType, item.EnchantTier, item.Bauble)
		if entry, exists := groundStacks[key]; exists {
			entry.count++
		} else {
			// A found bauble left lying shows where: "(on the bookshelf)".
			groundStacks[key] = &groundStack{name: item.DisplayNameFor(user.UserId) + item.BaubleSpotSuffix(), count: 1}
			groundStackOrder = append(groundStackOrder, key)
		}
	}
	for _, key := range groundStackOrder {
		entry := groundStacks[key]
		if entry.count > 1 {
			groundStuff = append(groundStuff, fmt.Sprintf(`%s <ansi fg="uses-left">(x%d)</ansi>`, entry.name, entry.count))
		} else {
			groundStuff = append(groundStuff, entry.name)
		}
	}

	// Find stashed items
	for _, item := range room.Stash {
		if !item.IsValid() {
			room.RemoveItem(item, true)
		}
		if item.StashedBy != user.UserId {
			continue
		}
		name := item.DisplayNameFor(user.UserId) + ` <ansi fg="item-stashed">(stashed)</ansi>`
		groundStuff = append(groundStuff, name)
	}

	groundStuff = append(groundStuff, details.VisibleCorpses...)

	groundDetails := map[string]any{
		`GroundStuff`: groundStuff,
		`IsDark`:      !room.IsLit(),
		`IsNight`:     gametime.IsNight(),
	}
	textOut, _ = templates.Process("descriptions/ontheground", groundDetails, user.UserId)
	if len(textOut) > 0 {
		user.SendText(messaging.CategoryRoomDescription, textOut)
	}

	textOut, _ = templates.Process("descriptions/exits", details, user.UserId)
	user.SendText(messaging.CategoryRoomDescription, textOut)

}

// fixtureLine is one fixture in the room look's descriptions/fixtures
// template (lighting 5e, R9).
type fixtureLine struct {
	Name     string
	Lit      bool // gives light, or darkens, right now
	Darkness bool // a darkness fixture: the template words it differently
}

// fixtureLines lists the room's fixtures in floor order, each lit or unlit
// from its current output (internal/itemlight).
func fixtureLines(room *rooms.Room) []fixtureLine {
	var out []fixtureLine
	for _, it := range room.Items {
		if it.IsFixture() {
			out = append(out, fixtureLine{
				Name:     it.DisplayName(),
				Lit:      itemlight.Lit(room.RoomId, it.UUID),
				Darkness: it.GetSpec().Fixture == items.FixtureDarkness,
			})
		}
	}
	return out
}
