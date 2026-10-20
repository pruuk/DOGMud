package usercommands

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/prompt"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

/*
* Role Permissions:
* room	 				(All)
* room.edit				(All edit commands)
* room.edit.container	(Edit containers)
* room.edit.exits		(Edit exits)
* room.edit.mutators	(Edit mutators)
* room.edit.nouns		(Edit nouns)
* room.copy				(Copy room properties from one room to another)
* room.info				(See a room summary)
* room.set				(Set properties of the room)
 */
func Room(rest string, user *users.UserRecord, liveRoom *rooms.Room, flags events.EventFlag) (bool, error) {

	handled := true

	args := util.SplitButRespectQuotes(rest)

	if len(args) == 0 {
		infoOutput, _ := templates.Process("admincommands/help/command.room", nil, user.UserId)
		user.SendText(messaging.CategoryUsage, infoOutput)
		return handled, nil
	}

	var room *rooms.Room
	if liveRoom.IsEphemeral() {
		room = liveRoom
	} else {
		room = rooms.LoadRoomTemplate(liveRoom.RoomId)
	}

	if room == nil {
		err := fmt.Errorf("room %d did not load from its template", liveRoom.RoomId)
		user.SendText(messaging.CategoryAdmin, err.Error())
		return true, err
	}

	roomCmd := strings.ToLower(args[0])

	switch roomCmd {
	case `edit`:
		return adminRoom_Edit(rest, user, room, flags)
	case `noun`, `nouns`:
		return adminRoom_Noun(args, user, room)
	case `info`:
		return adminRoom_Info(args, user, room)
	case `secretexit`:
		if len(args) < 2 {
			user.SendText(messaging.CategoryAdmin, fmt.Sprintf(`Invalid room command: <ansi fg="command">%s</ansi>`, roomCmd))
			return handled, nil
		}
		return adminRoom_SecretExit(args, user, room)
	case `copy`:
		if len(args) < 3 {
			user.SendText(messaging.CategoryAdmin, fmt.Sprintf(`Invalid room command: <ansi fg="command">%s</ansi>`, roomCmd))
			return handled, nil
		}
		return adminRoom_Copy(args, user, room)
	case `exit`:
		if len(args) < 2 {
			user.SendText(messaging.CategoryAdmin, fmt.Sprintf(`Invalid room command: <ansi fg="command">%s</ansi>`, roomCmd))
			return handled, nil
		}
		return adminRoom_Exit(args, user, room)
	case `set`:
		if len(args) < 2 {
			user.SendText(messaging.CategoryAdmin, fmt.Sprintf(`Invalid room command: <ansi fg="command">%s</ansi>`, roomCmd))
			return handled, nil
		}
		return adminRoom_Set(args, user, room)
	default:
		user.SendText(messaging.CategoryAdmin, fmt.Sprintf(`Invalid room command: <ansi fg="command">%s</ansi>`, roomCmd))
	}

	return handled, nil
}

func room_Edit_Containers(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	// This basic struct will be used to keep track of what we're editing
	type ContainerEdit struct {
		Name      string
		NameNew   string
		Container rooms.Container
		Exists    bool
	}

	containerOptions := []templates.NameDescription{}

	for name, c := range room.Containers {

		// If it's ephemeral, don't bother.
		if c.DespawnRound != 0 {
			continue
		}

		containerOption := templates.NameDescription{Name: name}

		if c.Lock.Difficulty > 0 {
			containerOption.Description += fmt.Sprintf(`[Lvl %d Lock] `, c.Lock.Difficulty)
		}

		if len(c.Recipes) > 0 {
			containerOption.Description += fmt.Sprintf(`[%d Recipe(s)] `, len(c.Recipes))
		}

		containerOptions = append(containerOptions, containerOption)

	}

	// Must sort since maps will often change between iterations
	sort.SliceStable(containerOptions, func(i, j int) bool {
		return containerOptions[i].Name < containerOptions[j].Name
	})

	//
	// Create a holder for container editing data
	//
	currentlyEditing := ContainerEdit{}

	cmdPrompt, _ := user.StartPrompt(`room edit containers`, rest)
	cmdPrompt.RoomBound = true // edits this room; a move cancels it (#375)

	question := cmdPrompt.AskMenu(containerMenu(containerOptions))
	if !question.Done {
		user.SendText(messaging.CategoryAdmin, menu.Render(*question.Menu))
		return true, nil
	}

	currentlyEditing.Name = question.Response

	for _, o := range containerOptions {
		if strings.EqualFold(o.Name, currentlyEditing.Name) {
			currentlyEditing.Name = o.Name
			break
		}
	}

	// Load the (possible) existing container
	currentlyEditing.Container, currentlyEditing.Exists = room.Containers[currentlyEditing.Name]

	// If they entered a container name...
	if currentlyEditing.Name != `new` {

		// Does the container name they entered not exist? Failure!
		if !currentlyEditing.Exists {
			user.SendText(messaging.CategoryAdmin, "Invalid option selected.")
			user.SendText(messaging.CategoryAdmin, "Aborting...")
			user.ClearPrompt()
			return true, nil
		}

		// Since they picked a container that exists, lets get the question of delete out of the way immediately.
		question := cmdPrompt.Ask(`Delete this container?`, []string{`yes`, `no`}, `no`)
		if !question.Done {
			return true, nil
		}

		// Delete the container if that's what they want!
		if question.Response == `yes` {

			delete(room.Containers, currentlyEditing.Name)
			rooms.SaveRoomTemplate(*room)

			user.SendBlankLine(messaging.CategoryAdmin)
			user.SendText(messaging.CategoryAdmin, fmt.Sprintf(`<ansi fg="container">%s</ansi> deleted from the room.`, currentlyEditing.Name))
			user.SendBlankLine(messaging.CategoryAdmin)

			user.ClearPrompt()
			return true, nil
		}

	}

	//
	// Name Selection
	//
	{
		// If they are creating a new container, we don't want that to become a viable container name, lets empty it
		if currentlyEditing.Name == `new` {
			currentlyEditing.Name = ``
		}

		// allow them to name/rename the container.
		question := cmdPrompt.Ask(`Choose a name for this container:`, []string{currentlyEditing.Name}, currentlyEditing.Name)
		if !question.Done {
			return true, nil
		}
		currentlyEditing.NameNew = question.Response

		// Make sure they aren't using any reserved names.
		if currentlyEditing.NameNew == `quit` || currentlyEditing.NameNew == `new` {
			user.SendText(messaging.CategoryAdmin, "Invalid new name selected.")
			user.SendText(messaging.CategoryAdmin, "Aborting...")
			user.ClearPrompt()
			return true, nil
		}

		// Make sure the new name isn't a duplicate
		if currentlyEditing.Name != currentlyEditing.NameNew {
			if _, ok := room.Containers[currentlyEditing.NameNew]; ok {

				user.SendText(messaging.CategoryAdmin, `<ansi fg="red">A container with that name already exists!</ansi>`)
				question.RejectResponse()
				return true, nil

			}
		}

	}

	//
	// Lock Options
	//
	{
		var pending bool
		currentlyEditing.Container.Lock, pending = editLockAndTrap(cmdPrompt, user, currentlyEditing.Container.Lock, `container`)
		if pending {
			return true, nil
		}
	}

	//
	// Recipe Options
	//
	{
		var pending bool
		currentlyEditing.Container.Recipes, pending = editContainerRecipes(cmdPrompt, user, currentlyEditing.Container.Recipes)
		if pending {
			return true, nil
		}
	}

	//
	// Done editing. Save results
	//
	if currentlyEditing.Name != `` {
		delete(room.Containers, currentlyEditing.Name)
	}

	if room.Containers == nil {
		room.Containers = map[string]rooms.Container{}
	}

	room.Containers[currentlyEditing.NameNew] = currentlyEditing.Container
	rooms.SaveRoomTemplate(*room)

	user.SendBlankLine(messaging.CategoryAdmin)

	if currentlyEditing.Container.Lock.Difficulty > 0 {
		lockId := fmt.Sprintf(`%d-%s`, room.RoomId, currentlyEditing.NameNew)
		user.SendText(messaging.CategoryAdmin, fmt.Sprintf(`<ansi fg="red">To Create Key -  LockId: <ansi fg="231" bg="5">%s</ansi></ansi>`, lockId))

		seqString := ``
		for _, dir := range util.GetLockSequence(lockId, int(currentlyEditing.Container.Lock.Difficulty), string(configs.GetServerConfig().Seed), currentlyEditing.Container.Lock.RotationSeed) {
			seqString += string(dir) + " "
		}
		user.SendText(messaging.CategoryAdmin, fmt.Sprintf(`<ansi fg="red">To pick lock - Sequence: <ansi fg="green">%s</ansi></ansi>`, seqString))
	}

	user.SendBlankLine(messaging.CategoryAdmin)
	user.SendText(messaging.CategoryAdmin, `Changes saved.`)
	user.SendBlankLine(messaging.CategoryAdmin)

	user.ClearPrompt()

	return true, nil
}

func room_Edit_Containers_SendRecipes(user *users.UserRecord, recipeResultItemId int, recipeItems map[int]int) {

	itm := items.New(recipeResultItemId)

	user.SendBlankLine(messaging.CategoryAdmin)
	user.SendText(messaging.CategoryAdmin, fmt.Sprintf(`    Current Recipe for %d (<ansi fg="itemname">%s</ansi>):`, recipeResultItemId, itm.DisplayName()))

	itemsList := []string{}
	for itemId, qty := range recipeItems {
		itm := items.New(itemId)
		itemsList = append(itemsList, fmt.Sprintf(`        <ansi fg="red">[x%d]</ansi> %d (<ansi fg="itemname">%s</ansi>)`, qty, itemId, itm.DisplayName()))
	}

	// Must sort since maps will often change between iterations
	sort.SliceStable(itemsList, func(i, j int) bool {
		return itemsList[i] < itemsList[j]
	})

	for _, txt := range itemsList {
		user.SendText(messaging.CategoryAdmin, txt)
	}

	user.SendBlankLine(messaging.CategoryAdmin)
}

// editLockAndTrap handles the lock/trap questionnaire shared by containers and exits.
// Returns the updated lock and whether the caller should early-return (pending=true means a question is still in progress).
func editLockAndTrap(cmdPrompt *prompt.Prompt, user *users.UserRecord, lock gamelock.Lock, itemType string) (gamelock.Lock, bool) {

	question := cmdPrompt.Ask(`Will this `+itemType+` be locked?`, []string{`yes`, `no`}, util.BoolYN(lock.Difficulty > 0))
	if !question.Done {
		return lock, true
	}

	if question.Response == `yes` {

		defaultDifficultyAnswer := ``
		if lock.Difficulty > 0 {
			defaultDifficultyAnswer = strconv.Itoa(int(lock.Difficulty))
		}

		question := cmdPrompt.Ask(`What difficulty will the lock be (2-32)?`, []string{defaultDifficultyAnswer}, defaultDifficultyAnswer)
		if !question.Done {
			return lock, true
		}

		difficultyInt, _ := strconv.Atoi(question.Response)

		// Make sure the provided difficulty is within acceptable range.
		if difficultyInt < 2 || difficultyInt > 32 {
			user.SendText(messaging.CategoryAdmin, "Difficulty must between 2 and 32, inclusive.")
			question.RejectResponse()
			return lock, true
		}

		lock.Difficulty = uint8(difficultyInt)

	} else {
		// reset the lock state if there is no lock.
		lock = gamelock.Lock{}
	}

	if lock.Difficulty > 0 {
		//
		// Lock Trap Options
		//
		question = cmdPrompt.Ask(`Will this lock have a trap?`, []string{`yes`, `no`}, util.BoolYN(len(lock.TrapConditionIds) > 0))
		if !question.Done {
			return lock, true
		}

		if question.Response == `yes` {

			selectedConditionList := []int{}
			if cb, ok := cmdPrompt.Recall(`trapConditions`); ok {
				selectedConditionList = cb.([]int)
			}

			if len(selectedConditionList) == 0 {
				selectedConditionList = append(selectedConditionList, lock.TrapConditionIds...)
			}

			// Keep track of the state
			cmdPrompt.Store(`trapConditions`, selectedConditionList)

			selectedConditionLookup := map[int]bool{}
			for _, bId := range selectedConditionList {
				selectedConditionLookup[bId] = true
			}

			conditionOptions := []templates.NameDescription{}

			for _, conditionId := range conditions.GetAllConditionIds() {
				// Condition 0 is the quit timer, not a trap effect, and its
				// id "0" would collide with this menu's leave answer.
				if conditionId == 0 {
					continue
				}
				if b := conditions.GetConditionSpec(conditionId); b != nil {

					if b.Name == `empty` {
						continue
					}

					marked := false
					if _, ok := selectedConditionLookup[conditionId]; ok {
						marked = true
					}

					conditionOptions = append(conditionOptions, templates.NameDescription{Id: conditionId, Marked: marked, Name: b.Name})
				}
			}

			// Ties break on id: names repeat ("Stunned" twice), and the ids
			// arrive in map order, so without it the numbers swap between
			// redraws.
			sort.SliceStable(conditionOptions, func(i, j int) bool {
				if conditionOptions[i].Name != conditionOptions[j].Name {
					return conditionOptions[i].Name < conditionOptions[j].Name
				}
				return conditionOptions[i].Id.(int) < conditionOptions[j].Id.(int)
			})

			question := cmdPrompt.AskMenu(trapConditionMenu(conditionOptions), `0`)
			if !question.Done {
				user.SendText(messaging.CategoryAdmin, menu.Render(*question.Menu))
				return lock, true
			}

			conditionSelected := question.Response

			if conditionSelected != `0` {

				conditionSelectedInt := 0

				if b, ok := optionByResponse(conditionOptions, conditionSelected); ok {
					conditionSelectedInt = b.Id.(int)
				}

				if conditionSelectedInt == 0 {

					user.SendText(messaging.CategoryAdmin, "Invalid selection.")
					question.RejectResponse()

					user.SendText(messaging.CategoryAdmin, menu.Render(*question.Menu))
					return lock, true
				}

				if _, ok := selectedConditionLookup[conditionSelectedInt]; ok {

					delete(selectedConditionLookup, conditionSelectedInt)
					for idx, conditionId := range selectedConditionList {
						if conditionId == conditionSelectedInt {
							selectedConditionList = append(selectedConditionList[0:idx], selectedConditionList[idx+1:]...)
							break
						}
					}

				} else {

					selectedConditionList = append(selectedConditionList, conditionSelectedInt)
					selectedConditionLookup[conditionSelectedInt] = true

				}

				cmdPrompt.Store(`trapConditions`, selectedConditionList)

				question.RejectResponse()

				for idx, data := range conditionOptions {
					_, data.Marked = selectedConditionLookup[data.Id.(int)]
					conditionOptions[idx] = data
				}

				// Re-ask so the stored menu carries the updated [x] marks.
				question = cmdPrompt.AskMenu(trapConditionMenu(conditionOptions), `0`)
				user.SendText(messaging.CategoryAdmin, menu.Render(*question.Menu))
				return lock, true

			}

		}

		if cb, ok := cmdPrompt.Recall(`trapConditions`); ok {
			lock.TrapConditionIds = cb.([]int)
		}

		if lock.RelockInterval == `` {
			lock.RelockInterval = gamelock.DefaultRelockTime
		}

		question = cmdPrompt.Ask(`How long until it automatically relocks?`, []string{lock.RelockInterval}, lock.RelockInterval)
		if !question.Done {
			return lock, true
		}

		lock.RelockInterval = question.Response

		// If the default time is chosen, can just leave it blank.
		if lock.RelockInterval == gamelock.DefaultRelockTime {
			lock.RelockInterval = ``
		}

	}

	return lock, false
}

// editContainerRecipes handles the recipe questionnaire for containers.
// Returns the updated recipes and whether the caller should early-return.
func editContainerRecipes(cmdPrompt *prompt.Prompt, user *users.UserRecord, recipes map[int][]int) (map[int][]int, bool) {

	question := cmdPrompt.Ask(`Will this container have recipes?`, []string{`yes`, `no`}, util.BoolYN(len(recipes) > 0))
	if !question.Done {
		return recipes, true
	}

	if question.Response == `yes` {

		currentRecipes := map[int][]int{}
		if cr, ok := cmdPrompt.Recall(`recipes`); ok {
			currentRecipes = cr.(map[int][]int)
		}

		if len(currentRecipes) == 0 {
			for k, v := range recipes {
				currentRecipes[k] = append([]int{}, v...)
			}
		}

		recipeNow := 0
		if rNow, ok := cmdPrompt.Recall(`recipeNow`); ok {
			recipeNow = rNow.(int)
		}

		if recipeNow != 0 && items.GetItemSpec(recipeNow) == nil {
			user.SendText(messaging.CategoryAdmin, `<ansi fg="red">Invalid selection.</ansi>`)
			question.RejectResponse()
			return recipes, true
		}

		// Keep track of the state
		cmdPrompt.Store(`recipes`, currentRecipes)
		cmdPrompt.Store(`recipeNow`, recipeNow)

		// Select recipe to modify
		if _, ok := currentRecipes[recipeNow]; !ok {
			recipeOptions := []templates.NameDescription{}
			for productItemId, recipeItemList := range currentRecipes {

				itm := items.New(productItemId)
				productName := fmt.Sprintf(`%d (%s)`, productItemId, itm.DisplayName())

				allRequiredItems := []string{}
				for _, iId := range recipeItemList {
					itm := items.New(iId)
					allRequiredItems = append(allRequiredItems, fmt.Sprintf(`%d (%s)`, iId, itm.DisplayName()))
				}

				recipeOptions = append(recipeOptions,
					templates.NameDescription{
						Id:          productItemId,
						Marked:      recipeNow == productItemId,
						Name:        productName,
						Description: strings.Join(allRequiredItems, `, `),
					})

			}

			recipeOptions = append(recipeOptions,
				templates.NameDescription{
					Id:          0,
					Marked:      false,
					Name:        `new`,
					Description: `create a new recipe`,
				})

			recipeOptions = append(recipeOptions,
				templates.NameDescription{
					Id:          -1,
					Marked:      false,
					Name:        `skip`,
					Description: `skip this step`,
				})

			question := cmdPrompt.AskMenu(recipeMenu(recipeOptions))
			if !question.Done {
				user.SendText(messaging.CategoryAdmin, menu.Render(*question.Menu))
				return recipes, true
			}

			recipeSelected, ok := optionByResponse(recipeOptions, question.Response)
			if !ok {
				question.RejectResponse()
				user.SendText(messaging.CategoryAdmin, menu.Render(*question.Menu))
				return recipes, true
			}
			recipeNow = recipeSelected.Id.(int)

			if recipeSelected.Name == `new` {

				question := cmdPrompt.Ask(`What itemId will be created?`, []string{})
				if !question.Done {
					return recipes, true
				}

				itemIdInt, _ := strconv.Atoi(question.Response)
				if items.GetItemSpec(itemIdInt) == nil {

					user.SendText(messaging.CategoryAdmin, "Invalid itemId.")
					question.RejectResponse()

					return recipes, true
				}

				if _, ok := currentRecipes[itemIdInt]; !ok {
					currentRecipes[itemIdInt] = []int{}
				}

				recipeNow = itemIdInt

				// Keep track of the state
				cmdPrompt.Store(`recipes`, currentRecipes)
				cmdPrompt.Store(`recipeNow`, recipeNow)
			}
		}

		// If they're editing a recipe, lets add ingredients
		if recipeNow != -1 {

			neededItems := map[int]int{}
			for _, inputItemId := range currentRecipes[recipeNow] {
				neededItems[inputItemId] = neededItems[inputItemId] + 1
			}

			question = cmdPrompt.Ask(`Enter an itemId to add to the recipe, or nothing to continue:`, []string{``}, `skip`)
			if !question.Done {
				// They have a recipe to modify, ask for item id's
				user.SendBlankLine(messaging.CategoryAdmin)
				user.SendText(messaging.CategoryAdmin, `<ansi fg="cyan">Positive numbers add items, negative numbers remove items.</ansi>`)

				room_Edit_Containers_SendRecipes(user, recipeNow, neededItems)

				return recipes, true
			}

			if question.Response != `skip` {

				removeItem := false
				if question.Response[0] == '-' {
					removeItem = true
					question.Response = question.Response[1:]
				}

				recipeAdjustment := items.FindItem(question.Response)

				if itemSpec := items.GetItemSpec(recipeAdjustment); itemSpec == nil {
					user.SendText(messaging.CategoryAdmin, `<ansi fg="red">Invalid ItemId provided.</ansi>`)

					room_Edit_Containers_SendRecipes(user, recipeNow, neededItems)

					question.RejectResponse()
					return recipes, true
				}

				if removeItem {

					for idx, itemId := range currentRecipes[recipeNow] {

						if itemId == recipeAdjustment {
							currentRecipes[recipeNow] = append(currentRecipes[recipeNow][0:idx], currentRecipes[recipeNow][idx+1:]...)

							neededItems[recipeAdjustment] -= 1

							if neededItems[recipeAdjustment] == 0 {
								delete(neededItems, recipeAdjustment)
							}

							break
						}

					}

				} else {
					currentRecipes[recipeNow] = append(currentRecipes[recipeNow], recipeAdjustment)
					neededItems[recipeAdjustment] += 1
				}

				// Keep track of the state
				cmdPrompt.Store(`recipes`, currentRecipes)
				cmdPrompt.Store(`recipeNow`, recipeNow)

				room_Edit_Containers_SendRecipes(user, recipeNow, neededItems)

				question.RejectResponse()
				return recipes, true

			}

		}

		if allRecipes, ok := cmdPrompt.Recall(`recipes`); ok {
			recipes = allRecipes.(map[int][]int)

			for i, itms := range recipes {
				if len(itms) == 0 {
					delete(recipes, i)
				}
			}
		}

	} else {
		clear(recipes)
	}

	return recipes, false
}

func room_Edit_Mutators(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	allRoomMutators := []string{}
	for _, roomMut := range room.Mutators {
		allRoomMutators = append(allRoomMutators, roomMut.MutatorId)
	}

	cmdPrompt, _ := user.StartPrompt(`room edit mutators`, rest)
	cmdPrompt.RoomBound = true // edits this room; a move cancels it (#375)

	selectedMutatorList := []string{}
	if muts, ok := cmdPrompt.Recall(`mutators`); ok {
		selectedMutatorList = muts.([]string)
	} else {
		if len(selectedMutatorList) == 0 {
			selectedMutatorList = append(selectedMutatorList, allRoomMutators...)
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

	question := cmdPrompt.AskMenu(roomMutatorMenu(mutatorOptions), `0`)
	if !question.Done {
		user.SendText(messaging.CategoryAdmin, menu.Render(*question.Menu))
		return true, nil
	}

	if question.Response != `0` {

		mutatorSelected := ``

		if b, ok := optionByResponse(mutatorOptions, question.Response); ok {
			mutatorSelected = b.Id.(string)
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
		question = cmdPrompt.AskMenu(roomMutatorMenu(mutatorOptions), `0`)
		user.SendText(messaging.CategoryAdmin, menu.Render(*question.Menu))
		return true, nil

	}

	//
	// Done editing. Save results
	//
	room.Mutators = mutators.MutatorList{}
	for _, mutId := range selectedMutatorList {
		room.Mutators = append(room.Mutators, mutators.Mutator{MutatorId: mutId})
	}
	rooms.SaveRoomTemplate(*room)

	user.SendBlankLine(messaging.CategoryAdmin)
	user.SendText(messaging.CategoryAdmin, `Changes saved.`)
	user.SendBlankLine(messaging.CategoryAdmin)

	user.ClearPrompt()

	return true, nil
}

// containerMenu is the "Choose one:" container picker: a "new" entry first,
// then the room's containers. Enter picks new.
func containerMenu(containerOptions []templates.NameDescription) menu.Menu {
	return newOrExistingMenu(containerOptions, `create a new container`)
}

// newOrExistingMenu prepends a "new" entry (unless the list already has one)
// and makes it the Enter default. Containers and exits share it.
func newOrExistingMenu(opts []templates.NameDescription, newDescription string) menu.Menu {
	hasNew := false
	for _, o := range opts {
		if o.Name == `new` {
			hasNew = true
			break
		}
	}
	if !hasNew {
		opts = append([]templates.NameDescription{{Name: `new`, Description: newDescription}}, opts...)
	}
	m := optionsMenu(`Choose one:`, opts, 1)
	m.Default = `new`
	return m
}

// trapConditionMenu toggles conditions on a trap; Done, Enter or a typed 0
// answers "0".
func trapConditionMenu(conditionOptions []templates.NameDescription) menu.Menu {
	m := optionsMenu(`Select a condition to add to the trap, or nothing to continue:`, conditionOptions, 2)
	m.Leave = `Done`
	m.LeaveAliases = []string{`0`}
	return m
}

// recipeMenu picks a container recipe to modify; Enter picks skip.
func recipeMenu(recipeOptions []templates.NameDescription) menu.Menu {
	m := optionsMenu(`Modify which (or new)?`, recipeOptions, 1)
	for _, o := range recipeOptions {
		if o.Name == `skip` {
			m.Default = optionId(o)
		}
	}
	return m
}

// roomMutatorMenu toggles room mutators; Done, Enter or a typed 0 answers
// "0".
func roomMutatorMenu(mutatorOptions []templates.NameDescription) menu.Menu {
	m := optionsMenu(`Select a mutator to add to the room, or nothing to continue:`, mutatorOptions, 2)
	m.Leave = `Done`
	m.LeaveAliases = []string{`0`}
	return m
}
