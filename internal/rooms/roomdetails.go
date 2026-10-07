package rooms

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/colorpatterns"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/guilds"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

type RoomTemplateDetails struct {
	VisiblePlayers []string
	VisibleMobs    []string
	VisibleCorpses []string
	VisibleExits   map[string]exit.RoomExit
	TemporaryExits map[string]exit.TemporaryRoomExit
	UserId         int
	Character      *characters.Character
	RoomSymbol     string
	RoomLegend     string
	Nouns          []string
	Zone           string
	Title          string
	Description    string
	IsDark         bool
	IsNight        bool
	TrackingString string
	RoomAlerts     []string // Messages to show below room description as a special alert
	ShowPvp        bool     // Whether to display that the room is PVP
}

func GetDetails(r *Room, user *users.UserRecord, tinymap ...[]string) RoomTemplateDetails {

	c := configs.GetGamePlayConfig()

	// Per-room mapsymbol/maplegend take priority over the biome; see
	// Room.MapSymbolAndLegend (shared with the zone mapper's priority).
	roomSymbol, roomLegend := r.MapSymbolAndLegend()

	showPvp := false
	// Don't need to show the PVP flag if Pvp is globally enabled or globally disabled
	if c.PVP == configs.PVPLimited {
		showPvp = r.IsPvp()
	}

	details := RoomTemplateDetails{
		VisiblePlayers: []string{},
		VisibleMobs:    []string{},
		VisibleCorpses: []string{},
		VisibleExits:   make(map[string]exit.RoomExit),
		TemporaryExits: make(map[string]exit.TemporaryRoomExit),
		Zone:           r.Zone,
		Title:          r.Title,
		Description:    r.GetDescription(),
		UserId:         user.UserId,    // Who is viewing the room
		Character:      user.Character, // The character of the user viewing the room
		RoomSymbol:     roomSymbol,
		RoomLegend:     roomLegend,
		IsDark:         !r.IsLit(),
		IsNight:        gametime.IsNight(),
		TrackingString: ``,
		ShowPvp:        showPvp,
	}

	//
	// Start Room Alerts
	//

	if r.IsBank {
		details.RoomAlerts = append(details.RoomAlerts, `          <ansi fg="yellow-bold">This is a bank!</ansi> Type <ansi fg="command">bank</ansi> to deposit/withdraw.`)
	}

	if r.IsStorage {
		details.RoomAlerts = append(details.RoomAlerts, ` <ansi fg="yellow-bold">This is an item storage location!</ansi> Type <ansi fg="command">storage</ansi> to store/unstore.`)
	}

	if r.IsCharacterRoom {
		details.RoomAlerts = append(details.RoomAlerts, `      <ansi fg="yellow-bold">This is a character room!</ansi> Type <ansi fg="command">character</ansi> to interact.`)
	}

	if r.RoomId == -1 {
		details.RoomAlerts = append(details.RoomAlerts, `      <ansi fg="yellow-bold">Type <ansi fg="command">start</ansi> to begin playing.</ansi>`)
	}

	//
	// End Room Alerts
	//
	// Noun highlighting is universal (2026-06-12). It was formerly gated
	// on the room.nouns role permission or a pet with the SeeNouns condition
	// flag — but the default user role can never hold permissions and no
	// dogmud-world condition carries see-nouns, so the gate made the feature
	// admin-only by accident. Discoverability for everyone beats a
	// vestigial perk.
	renderNouns := true

	if len(tinymap) > 0 {
		desclineWidth := 80 - 7 // 7 is the width of the tinymap
		padding := 1
		description := util.SplitString(details.Description, desclineWidth-padding)

		for i := 0; i < len(tinymap[0]); i++ {
			if i > len(description)-1 {
				description = append(description, strings.Repeat(` `, desclineWidth))
			}

			padWidth := desclineWidth - util.VisibleWidth(description[i])
			if padWidth < 0 {
				padWidth = 0
			}
			description[i] += strings.Repeat(` `, padWidth) + tinymap[0][i]
		}

		if renderNouns && len(r.Nouns) > 0 {
			for i := range description {
				for noun, _ := range r.Nouns {
					// Skip if noun is already inside an ANSI tag to avoid nested tags
					if strings.Contains(description[i], `>`+noun+`</ansi>`) {
						continue
					}
					description[i] = strings.Replace(description[i], noun, `<ansi fg="noun">`+noun+`</ansi>`, 1)
				}
			}
		}

		details.Description = strings.Join(description, "\n")
	} else {

		roomDesc := util.SplitString(details.Description, 80)

		if renderNouns && len(r.Nouns) > 0 {
			for i := range roomDesc {
				for noun, _ := range r.Nouns {
					// Skip if noun is already inside an ANSI tag to avoid nested tags
					if strings.Contains(roomDesc[i], `>`+noun+`</ansi>`) {
						continue
					}
					roomDesc[i] = strings.Replace(roomDesc[i], noun, `<ansi fg="noun">`+noun+`</ansi>`, 1)
				}
			}
		}

		details.Description = strings.Join(roomDesc, "\n")
	}

	for mut := range r.ActiveMutators {
		mutSpec := mut.GetSpec()

		if mutSpec.NameModifier != nil {

			if mutSpec.NameModifier.Behavior == mutators.TextPrepend {

				if mutSpec.NameModifier.Text != `` {
					details.Title = colorpatterns.ApplyColorPattern(mutSpec.NameModifier.Text, mutSpec.NameModifier.ColorPattern) + ` ` + details.Title
				}

			} else if mutSpec.NameModifier.Behavior == mutators.TextReplace {

				if mutSpec.NameModifier.Text != `` {
					details.Title = colorpatterns.ApplyColorPattern(mutSpec.NameModifier.Text, mutSpec.NameModifier.ColorPattern)
				} else {
					details.Title = colorpatterns.ApplyColorPattern(details.Title, mutSpec.NameModifier.ColorPattern)
				}

			} else if mutSpec.NameModifier.Behavior == mutators.TextAppend {

				if mutSpec.NameModifier.Text != `` {
					details.Title = details.Title + ` ` + colorpatterns.ApplyColorPattern(mutSpec.NameModifier.Text, mutSpec.NameModifier.ColorPattern)
				}

			}

		}

		if mutSpec.DescriptionModifier != nil {

			// Handle any text changes
			if mutSpec.DescriptionModifier.Behavior == mutators.TextPrepend {

				if mutSpec.DescriptionModifier.Text != `` {

					details.Description = colorpatterns.ApplyColorPattern(mutSpec.DescriptionModifier.Text, mutSpec.DescriptionModifier.ColorPattern) +
						term.CRLFStr +
						details.Description

				}

			} else if mutSpec.DescriptionModifier.Behavior == mutators.TextReplace {

				if mutSpec.DescriptionModifier.Text != `` {
					details.Description = colorpatterns.ApplyColorPattern(mutSpec.DescriptionModifier.Text, mutSpec.DescriptionModifier.ColorPattern)
				} else {
					details.Description = colorpatterns.ApplyColorPattern(details.Description, mutSpec.DescriptionModifier.ColorPattern)
				}

			} else if mutSpec.DescriptionModifier.Behavior == mutators.TextAppend {

				if mutSpec.DescriptionModifier.Text != `` {

					details.Description = details.Description +
						term.CRLFStr +
						colorpatterns.ApplyColorPattern(mutSpec.DescriptionModifier.Text, mutSpec.DescriptionModifier.ColorPattern)

				}
			}

		}

		// Alert modifiers can only add to the list.
		// No current plans to allow them to overwrite existing alerts.
		if mutSpec.AlertModifier != nil {

			alertText := mutSpec.AlertModifier.Text

			// center the text
			if len(mutSpec.AlertModifier.Text) < 65 {
				padding := (65 - len(mutSpec.AlertModifier.Text)) >> 1
				alertText = strings.Repeat(` `, padding) + alertText
			}

			details.RoomAlerts = append(details.RoomAlerts, colorpatterns.ApplyColorPattern(alertText, mutSpec.AlertModifier.ColorPattern))

		}
	}

	nameFlags := []characters.NameRenderFlag{}
	// Health display now available to all players (Peep skill removed)
	nameFlags = append(nameFlags, characters.RenderHealth)

	if useShortAdjectives := user.GetConfigOption(`shortadjectives`); useShortAdjectives != nil && useShortAdjectives.(bool) {
		nameFlags = append(nameFlags, characters.RenderShortAdjectives)
	}

	for _, playerId := range r.players {
		if playerId != user.UserId {

			renderFlags := append([]characters.NameRenderFlag{}, nameFlags...)

			player := users.GetByUserId(playerId)
			if player != nil {

				// Sneaking or camo, and the viewer has no see-hidden. One rule,
				// shared with naming a creature (Room.FindByNameSeenBy).
				if !user.Character.Perceives(player.Character) {
					continue
				}

				pName := player.Character.GetPlayerName(user.UserId, renderFlags...)
				playerEntry := pName.String()
				if tag := guilds.TagForUser(playerId); tag != "" {
					playerEntry = fmt.Sprintf(`<ansi fg="cyan">[%s]</ansi> %s`, tag, playerEntry)
				}
				// Chunk 5 (Presence): read AFK status from canonical Presence machine.
				if player.Character != nil && player.Character.Presence != nil {
					if d, ok := player.Character.Presence.AFKData(); ok && d.Manual {
						if d.Message != "" {
							playerEntry += ` <ansi fg="8">(AFK: ` + d.Message + `)</ansi>`
						} else {
							playerEntry += ` <ansi fg="8">(AFK)</ansi>`
						}
					}
				}
				// Chunk 3.3: sleeping suffix
				if player.Character != nil && player.Character.HasConditionFlag(conditions.Sleeping) {
					playerEntry += ` <ansi fg="8">(asleep)</ansi>`
				}
				details.VisiblePlayers = append(details.VisiblePlayers, playerEntry)
			}
		}
	}

	if user.Character.Pet.Exists() && r.RoomId == user.Character.RoomId {
		details.VisiblePlayers = append(details.VisiblePlayers, fmt.Sprintf(`%s (your pet)`, user.Character.Pet.DisplayName()))
	}

	visibleFriendlyMobs := []string{}

	// Count visible mob names for duplicate detection
	mobNameCount := map[string]int{}
	for _, mobInstanceId := range r.mobs {
		if mob := mobs.GetInstance(mobInstanceId); mob != nil {
			// Defense-in-depth: a stale listing (mob moved away via schedule/
			// patrol but still listed here) must never render as a ghost.
			if mob.Character.RoomId != r.RoomId {
				continue
			}
			if !user.Character.Perceives(&mob.Character) {
				continue
			}
			mobNameCount[mob.Character.Name]++
		}
	}
	mobNameIndex := map[string]int{}

	for idx, mobInstanceId := range r.mobs {
		if mob := mobs.GetInstance(mobInstanceId); mob != nil {

			// Defense-in-depth: skip stale listings (see count loop above).
			if mob.Character.RoomId != r.RoomId {
				continue
			}

			// Sneaking or camo, and the viewer has no see-hidden.
			if !user.Character.Perceives(&mob.Character) {
				continue
			}

			tmpNameFlags := nameFlags

			mobName := mob.Character.GetMobName(user.UserId, tmpNameFlags...)

			// Check if this mob is targeting one of the player's companions.
			if tgt := mob.Character.CurrentCombatTarget(); mobName.Suffix == `` && tgt.MobInstanceId > 0 {
				if targetMob := mobs.GetInstance(tgt.MobInstanceId); targetMob != nil {
					if targetMob.Character.Charmed != nil && targetMob.Character.Charmed.UserId == user.UserId {
						mobName.Suffix = `aggro`
					}
				}
			}

			// Assign duplicate index when multiple mobs share the same name
			if mobNameCount[mob.Character.Name] > 1 {
				mobNameIndex[mob.Character.Name]++
				mobName.DuplicateIndex = mobNameIndex[mob.Character.Name]
			}

			for _, qFlag := range mob.QuestFlags {
				if user.Character.HasQuest(qFlag) || (len(qFlag) >= 5 && qFlag[len(qFlag)-5:] == `start`) {
					mobName.QuestAlert = true
					break
				}
			}

			// Build the mob name string once, then optionally decorate.
			mobNameStr := mobName.String()

			// Chunk 3.3: sleeping suffix
			if mob.Character.HasConditionFlag(conditions.Sleeping) {
				mobNameStr += ` <ansi fg="8">(asleep)</ansi>`
			}

			if mob.Character.IsCharmed() {
				visibleFriendlyMobs = append(visibleFriendlyMobs, mobNameStr)
			} else {
				details.VisibleMobs = append(details.VisibleMobs, mobNameStr)
			}
		} else {
			r.mobs = append(r.mobs[:idx], r.mobs[idx+1:]...)
		}
	}

	// Add the friendly mobs to the end
	details.VisibleMobs = append(details.VisibleMobs, visibleFriendlyMobs...)

	// The roster follows the viewer's sight in THIS room (lighting plan 5c).
	// Perceives above only answers hiding; light was never consulted, so a
	// heat-sight caster in a pitch-dark cave read "Also here: Midroad Scout
	// (100%)" while every narrated line about the same scout said "A figure".
	//
	// Shapes: each creature is the same anonymous figure HideNames writes,
	// with no health, shop, guild, AFK or sleep adjective, and all of them in
	// one list, because the template colors mobs and players differently and
	// that color alone would sort them. The count stays. None: nobody is
	// listed. look refuses before it gets here, but who and peering into
	// the next room do not.
	sight := messaging.ParticipantSight(user.Character, r)
	switch sight {
	case messaging.SightShapes:
		figures := make([]string, 0, len(details.VisiblePlayers)+len(details.VisibleMobs))
		for range len(details.VisiblePlayers) + len(details.VisibleMobs) {
			figures = append(figures, messaging.UnseenFigure(messaging.SightShapes))
		}
		details.VisiblePlayers = figures
		details.VisibleMobs = []string{}
	case messaging.SightNone:
		details.VisiblePlayers = []string{}
		details.VisibleMobs = []string{}
	}

	for exitStr, exitInfo := range r.ExitsTemp {
		details.TemporaryExits[exitStr] = exitInfo
	}

	// Do this twice to ensure secrets are last

	for exitStr, exitInfo := range r.Exits {

		// If it's a secret room we need to make sure the player has recently been there before including it in the exits
		if exitInfo.Secret {
			if targetRm := LoadRoom(exitInfo.RoomId); targetRm != nil {
				if targetRm.HasVisited(user.UserId, VisitorUser) {
					details.VisibleExits[exitStr] = exitInfo
				}
			}
		} else {
			details.VisibleExits[exitStr] = exitInfo
		}
	}

	// add any corpses present
	mobCorpses := map[string]int{}
	playerCorpses := map[string]int{}

	for _, c := range r.Corpses {
		if c.Prunable {
			continue
		}

		// Corpses follow the viewer's sight as the roster does (#428, and
		// the ground-listing half of #276). Shapes: one entry per corpse in
		// the hidden form look's corpse observer line writes ("corpse of a
		// figure", messaging.HideNames on the dead one's name), uncolored
		// by kind and never counted by name, so a mob's corpse and a
		// player's read alike. None: no corpses listed.
		switch sight {
		case messaging.SightShapes:
			if c.MobId > 0 || c.UserId > 0 {
				details.VisibleCorpses = append(details.VisibleCorpses,
					messaging.HideNames(`corpse of `+c.Character.Name, []string{c.Character.Name}, messaging.SightShapes))
			}
			continue
		case messaging.SightNone:
			continue
		}

		if c.MobId > 0 {
			mobCorpses[c.Character.Name] = mobCorpses[c.Character.Name] + 1
		}

		if c.UserId > 0 {
			playerCorpses[c.Character.Name] = playerCorpses[c.Character.Name] + 1
		}

	}

	for name, qty := range playerCorpses {
		if qty == 1 {
			details.VisibleCorpses = append(details.VisibleCorpses, fmt.Sprintf(`<ansi fg="user-corpse">%s corpse</ansi>`, name))
		} else {
			details.VisibleCorpses = append(details.VisibleCorpses, fmt.Sprintf(`<ansi fg="user-corpse">%d %s corpses</ansi>`, qty, name))
		}
	}

	for name, qty := range mobCorpses {
		if qty == 1 {
			details.VisibleCorpses = append(details.VisibleCorpses, fmt.Sprintf(`<ansi fg="mob-corpse">%s corpse</ansi>`, name))
		} else {
			details.VisibleCorpses = append(details.VisibleCorpses, fmt.Sprintf(`<ansi fg="mob-corpse">%d %s corpses</ansi>`, qty, name))
		}
	}

	// assign mutator exits last so that they can overwrite normal exits
	for mut := range r.ActiveMutators {
		spec := mut.GetSpec()
		for exitName, exitInfo := range spec.Exits {
			details.VisibleExits[exitName] = exitInfo
		}
	}

	if searchMobName := user.Character.GetMiscData(`tracking-mob`); searchMobName != nil {

		// Condition-absent cleanup: if tracking misc data was set but the
		// active-tracking condition expired or was removed, drop the misc
		// data so the render doesn't fire forever.
		if !user.Character.HasCondition(86) {
			user.Character.SetMiscData("tracking-mob", nil)
			user.Character.SetMiscData("tracking-user", nil)
			user.Character.SetMiscData("tracking-display-count", nil)
		} else if searchMobNameStr, ok := searchMobName.(string); ok {

			if r.isInRoom(searchMobNameStr, ``) {

				// Always show when target is found in the room
				details.TrackingString = `Tracking <ansi fg="mobname">` + searchMobNameStr + `</ansi>... They are here!`
				user.Character.RemoveCondition(86)
				user.Character.SetMiscData("tracking-display-count", nil)
				user.Character.SetMiscData("tracking-mob", nil)
				user.Character.SetMiscData("tracking-user", nil)

			} else {

				allNames := []string{}

				for mobInstId, _ := range r.Visitors(VisitorMob) {
					if mob := mobs.GetInstance(mobInstId); mob != nil {
						allNames = append(allNames, mob.Character.Name)
					}
				}

				match, closeMatch := util.FindMatchIn(searchMobNameStr, allNames...)
				if match == `` && closeMatch == `` {

					// Always show when trail is lost
					details.TrackingString = `You lost the trail of <ansi fg="mobname">` + searchMobNameStr + `</ansi>`
					user.Character.RemoveCondition(86)
					user.Character.SetMiscData("tracking-display-count", nil)
					user.Character.SetMiscData("tracking-mob", nil)
					user.Character.SetMiscData("tracking-user", nil)

				} else {

					exitName := r.findMobExit(0, searchMobNameStr)
					if exitName == `` {

						// Always show when trail is lost
						details.TrackingString = `You lost the trail of <ansi fg="username">` + searchMobNameStr + `</ansi>`
						user.Character.RemoveCondition(86)
						user.Character.SetMiscData("tracking-display-count", nil)
						user.Character.SetMiscData("tracking-mob", nil)
						user.Character.SetMiscData("tracking-user", nil)

					} else {

						// Throttle ongoing direction hints to every 3rd room
						trackCount := 0
						if tc := user.Character.GetMiscData("tracking-display-count"); tc != nil {
							if tcInt, ok := tc.(int); ok {
								trackCount = tcInt
							}
						}
						trackCount++
						user.Character.SetMiscData("tracking-display-count", trackCount)
						if trackCount%3 == 1 {
							details.TrackingString = `Tracking <ansi fg="mobname">` + searchMobNameStr + `</ansi>... They went <ansi fg="exit">` + exitName + `</ansi>`
						}
					}

				}
			}
		}

	}

	if searchUserName := user.Character.GetMiscData(`tracking-user`); searchUserName != nil {

		// Condition-absent cleanup: if tracking misc data was set but the
		// active-tracking condition expired or was removed, drop the misc
		// data so the render doesn't fire forever.
		if !user.Character.HasCondition(86) {
			user.Character.SetMiscData("tracking-mob", nil)
			user.Character.SetMiscData("tracking-user", nil)
			user.Character.SetMiscData("tracking-display-count", nil)
		} else if searchUserNameStr, ok := searchUserName.(string); ok {

			if r.isInRoom(``, searchUserNameStr) {

				// Always show when target is found in the room
				details.TrackingString = `Tracking <ansi fg="username">` + searchUserNameStr + `</ansi>... They are here!`
				user.Character.RemoveCondition(86)
				user.Character.SetMiscData("tracking-display-count", nil)
				user.Character.SetMiscData("tracking-mob", nil)
				user.Character.SetMiscData("tracking-user", nil)

			} else {

				allNames := []string{}

				for userId, _ := range r.Visitors(VisitorUser) {
					if u := users.GetByUserId(userId); u != nil {
						allNames = append(allNames, u.Character.Name)
					}
				}

				match, closeMatch := util.FindMatchIn(searchUserNameStr, allNames...)
				if match == `` && closeMatch == `` {

					// Always show when trail is lost
					details.TrackingString = `You lost the trail of <ansi fg="username">` + searchUserNameStr + `</ansi>`
					user.Character.RemoveCondition(86)
					user.Character.SetMiscData("tracking-display-count", nil)
					user.Character.SetMiscData("tracking-mob", nil)
					user.Character.SetMiscData("tracking-user", nil)

				} else {

					exitName := r.findUserExit(0, searchUserNameStr)
					if exitName == `` {

						// Always show when trail is lost
						details.TrackingString = `You lost the trail of <ansi fg="username">` + searchUserNameStr + `</ansi>`
						user.Character.RemoveCondition(86)
						user.Character.SetMiscData("tracking-display-count", nil)
						user.Character.SetMiscData("tracking-mob", nil)
						user.Character.SetMiscData("tracking-user", nil)

					} else {

						// Throttle ongoing direction hints to every 3rd room
						trackCount := 0
						if tc := user.Character.GetMiscData("tracking-display-count"); tc != nil {
							if tcInt, ok := tc.(int); ok {
								trackCount = tcInt
							}
						}
						trackCount++
						user.Character.SetMiscData("tracking-display-count", trackCount)
						if trackCount%3 == 1 {
							details.TrackingString = `Tracking <ansi fg="username">` + searchUserNameStr + `</ansi>... They went <ansi fg="exit">` + exitName + `</ansi>`
						}
					}

				}
			}

		}
	}

	return details

}
