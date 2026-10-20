package usercommands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// zoneMutatorMenu toggles zone mutators; Done, Enter or a typed 0 answers
// "0".
func zoneMutatorMenu(mutatorOptions []templates.NameDescription) menu.Menu {
	m := optionsMenu(`Select a mutator to add/remove, or nothing to continue:`, mutatorOptions, 2)
	m.Leave = `Done`
	m.LeaveAliases = []string{`0`}
	return m
}

/*
* Role Permissions:
* zone 				(All)
 */
func Zone(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	handled := true

	// args should look like one of the following:
	// info <optional room id>
	// <move to room id>
	args := util.SplitButRespectQuotes(rest)

	if len(args) == 0 {
		// send some sort of help info?
		infoOutput, _ := templates.Process("admincommands/help/command.zone", nil, user.UserId)
		user.SendText(messaging.CategoryUsage, infoOutput)

		return handled, nil
	}

	zoneCmd := strings.ToLower(args[0])
	args = args[1:]

	// Interactive Editing
	if zoneCmd == `edit` {
		return zone_Edit(``, user, room, flags)
	}

	zoneConfig := rooms.GetZoneConfig(room.Zone)
	if zoneConfig == nil {
		user.SendText(messaging.CategoryAdmin, fmt.Sprintf(`Couldn't find zone info for <ansi fg="red">%s</ansi>`, room.Zone))
		return true, nil
	}

	if zoneCmd == `info` {

		user.SendBlankLine(messaging.CategoryAdmin)
		user.SendText(messaging.CategoryAdmin, fmt.Sprintf(`<ansi fg="yellow-bold">Zone Config for:    <ansi fg="red">%s</ansi></ansi>`, room.Zone))
		user.SendText(messaging.CategoryAdmin, fmt.Sprintf(`   <ansi fg="yellow-bold">Root Room Id:</ansi>    <ansi fg="red">%d</ansi>`, zoneConfig.RoomId))

		user.SendBlankLine(messaging.CategoryAdmin)

		return true, nil
	}

	// Everthing after this point requires additional args
	if len(args) < 1 {
		user.SendText(messaging.CategoryAdmin, `Not enough arguments provided.`)
		return true, nil
	}

	if zoneCmd == `set` {

		setWhat := args[0]

		args = args[1:]

		if setWhat == `autoscale` {
			user.SendText(messaging.CategoryAdmin, `Autoscaling has been removed. Use per-mob statpool values instead.`)
			return true, nil
		}

	}

	return true, nil
}

func zone_Edit(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	originalZoneConfig := rooms.GetZoneConfig(room.Zone)
	if originalZoneConfig == nil {
		user.SendText(messaging.CategoryAdmin, `Could not find zone config.`)
		return true, nil
	}

	// Make a copy that we'll edit
	editZoneConfig := *originalZoneConfig

	allZoneMutators := []string{}
	for _, roomMut := range editZoneConfig.Mutators {
		allZoneMutators = append(allZoneMutators, roomMut.MutatorId)
	}

	cmdPrompt, _ := user.StartPrompt(`zone edit`, rest)
	cmdPrompt.RoomBound = true // edits this room's zone; a move cancels it (#375)

	selectedMutatorList := []string{}
	if muts, ok := cmdPrompt.Recall(`mutators`); ok {
		selectedMutatorList = muts.([]string)
	} else {
		if len(selectedMutatorList) == 0 {
			selectedMutatorList = append(selectedMutatorList, allZoneMutators...)
		}
	}

	// Keep track of the state
	cmdPrompt.Store(`mutators`, selectedMutatorList)

	selectedMutatorLookup := map[string]bool{}
	for _, mutId := range selectedMutatorList {
		selectedMutatorLookup[mutId] = true
	}

	mutatorOptions := []templates.NameDescription{}

	for _, mutId := range mutators.GetAllMutatorIds() {
		marked := false
		if _, ok := selectedMutatorLookup[mutId]; ok {
			marked = true
		}

		mutatorOptions = append(mutatorOptions, templates.NameDescription{Id: mutId, Marked: marked, Name: mutId})

	}

	sort.SliceStable(mutatorOptions, func(i, j int) bool {
		return mutatorOptions[i].Name < mutatorOptions[j].Name
	})

	question := cmdPrompt.AskMenu(zoneMutatorMenu(mutatorOptions), `0`)
	if !question.Done {
		user.SendText(messaging.CategoryAdmin, menu.Render(*question.Menu))
		return true, nil
	}

	if question.Response != `0` {

		mutatorSelected := ``

		if b, ok := optionByResponse(mutatorOptions, question.Response); ok {
			if id, ok := b.Id.(string); ok {
				mutatorSelected = id
			}
		}

		if mutatorSelected == `` {

			user.SendText(messaging.CategoryAdmin, "Invalid selection.")
			question.RejectResponse()

			user.SendText(messaging.CategoryAdmin, menu.Render(*question.Menu))
			return true, nil
		}

		if _, ok := selectedMutatorLookup[mutatorSelected]; ok {

			delete(selectedMutatorLookup, mutatorSelected)
			for idx, mutId := range selectedMutatorList {
				if mutId == mutatorSelected {
					selectedMutatorList = append(selectedMutatorList[0:idx], selectedMutatorList[idx+1:]...)
					break
				}
			}

		} else {

			selectedMutatorList = append(selectedMutatorList, mutatorSelected)
			selectedMutatorLookup[mutatorSelected] = true

		}

		cmdPrompt.Store(`mutators`, selectedMutatorList)

		question.RejectResponse()

		for idx, data := range mutatorOptions {
			_, data.Marked = selectedMutatorLookup[data.Id.(string)]
			mutatorOptions[idx] = data
		}

		// Re-ask so the stored menu carries the updated [x] marks.
		question = cmdPrompt.AskMenu(zoneMutatorMenu(mutatorOptions), `0`)
		user.SendText(messaging.CategoryAdmin, menu.Render(*question.Menu))
		return true, nil

	}

	//
	// Music Options
	//
	{

		musicMenu := menu.Confirm(`Should the zone have music?`, `Yes`, `No`)
		if editZoneConfig.MusicFile != `` {
			musicMenu.Default = `yes`
		}
		question := cmdPrompt.AskMenu(musicMenu)
		if !question.Done {
			user.SendText(messaging.CategoryAdmin, menu.Render(*question.Menu))
			return true, nil
		}

		if question.Response == `yes` {

			relativeString := configs.GetFilePathsConfig().WebCDNLocation.String()
			if len(relativeString) > 0 {
				user.SendText(messaging.CategoryAdmin, `   <ansi fg="red">Note:</ansi> Music file path must be relative to: <ansi fg="red">`+relativeString+`</ansi>`)
			}

			question := cmdPrompt.Ask(`Zone music file path?`, []string{editZoneConfig.MusicFile}, editZoneConfig.MusicFile)
			if !question.Done {
				return true, nil
			}
			editZoneConfig.MusicFile = question.Response

		} else {
			editZoneConfig.MusicFile = ``
		}

	}

	//
	// Done editing. Save results
	//
	editZoneConfig.Mutators = mutators.MutatorList{}
	for _, mutId := range selectedMutatorList {
		editZoneConfig.Mutators = append(editZoneConfig.Mutators, mutators.Mutator{MutatorId: mutId})
	}

	rooms.SaveZoneConfig(&editZoneConfig)

	user.SendBlankLine(messaging.CategoryAdmin)
	user.SendText(messaging.CategoryAdmin, `Changes saved.`)
	user.SendBlankLine(messaging.CategoryAdmin)

	user.ClearPrompt()

	return true, nil
}
