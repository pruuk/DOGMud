package usercommands

import (
	"fmt"
	"math"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// questRecord is one row of the quests list.
type questRecord struct {
	Id          int
	Name        string
	Repeatable  bool
	Description string
	Progress    string // "Roots gathered: 2 of 5" for a count or explore step, else ""
	Guide       string // the destination line (questengine.GuideLine), else ""
	Completion  string
	BarFull     string
	BarEmpty    string
}

// The finished-repeatable status lines in "quests all" (#375 P6).
const (
	repeatReadyLine = "Ready to take on again."
	repeatWaitLine  = "Not offered again yet."
	repeatDoneLine  = "Done for good."
)

// finishedRepeatRows are the "quests all" rows for repeatables the player
// has finished and holds no progress on (completion clears it), with their
// status line in place of a step description.
func finishedRepeatRows(c *characters.Character, progress map[int]string, showHidden bool) []questRecord {
	var rows []questRecord
	for questId, n := range c.QuestCompletions {
		if n < 1 {
			continue
		}
		if _, inProgress := progress[questId]; inProgress {
			continue
		}
		q := quests.GetQuestById(questId)
		if q == nil || !q.Repeatable || (q.Secret && !showHidden) {
			continue
		}
		line := repeatReadyLine
		switch {
		case c.RepeatCapReached(q):
			line = repeatDoneLine
		case c.QuestCooldownActive(q.QuestId):
			line = repeatWaitLine
		}
		full, empty := util.ProgressBar(1, 25)
		rows = append(rows, questRecord{Id: q.QuestId, Name: q.Name, Repeatable: true,
			Description: line, Completion: "100%", BarFull: full, BarEmpty: empty})
	}
	return rows
}

// activeQuestRecord is the quests row of a quest the player holds at
// questStep (`all+` for an admin's unstarted row), with its completion. A
// step reached by a count or distinct trigger carries its progress line
// (#375 phase 3b slice 4).
func activeQuestRecord(user *users.UserRecord, questInfo *quests.Quest, questStep string) (questRecord, float64) {
	totalSteps := len(questInfo.Steps)
	completedSteps := 0
	description := questInfo.Description
	progress := ``
	guide := ``

	if questStep != `all+` {
		for _, step := range questInfo.Steps {
			completedSteps++
			if step.Id == questStep {
				description = step.Description
				break
			}
		}
		progress = questengine.GetEngine().UserStepProgress(user, questInfo.QuestId, questStep)
		guide = questengine.GuideLine(questengine.GetEngine().ResolveDestination(user, questInfo.QuestId, questStep))
	}

	completion := float64(completedSteps) / float64(totalSteps)
	barFull, barEmpty := util.ProgressBar(completion, 25)

	return questRecord{
		Id:          questInfo.QuestId,
		Name:        questInfo.Name,
		Repeatable:  questInfo.Repeatable,
		Description: description,
		Progress:    progress,
		Guide:       guide,
		Completion:  fmt.Sprintf(`%d%%`, int(math.Floor(completion*100))),
		BarFull:     barFull,
		BarEmpty:    barEmpty,
	}, completion
}

func Quests(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	type QuestInfo struct {
		QuestsTotal int
		QuestsFound int
		Records     []questRecord
	}

	showHidden := questsShowHidden(rest, user)
	showComplete := rest == `all` || rest == `all+`

	qInfo := QuestInfo{}
	allQuests := []questRecord{}
	var completion float64

	qInfo.QuestsTotal = 0

	allQuestProgress := user.Character.GetQuestProgress()

	if showHidden {
		for _, quest := range quests.GetAllQuests() {
			if _, ok := allQuestProgress[quest.QuestId]; ok {
				continue
			}
			// A finished repeatable gets its status row below, not a 0%
			// placeholder row.
			if quest.Repeatable && user.Character.QuestCompletions[quest.QuestId] > 0 {
				continue
			}
			allQuestProgress[quest.QuestId] = `all+`
		}
	}

	for questId, questStep := range allQuestProgress {
		questToken := quests.PartsToToken(questId, questStep)
		if questInfo := quests.GetQuest(questToken); questInfo != nil {

			// Secret quests are not rendered
			if !showHidden && questInfo.Secret {
				continue
			}

			qInfo.QuestsTotal++

			var qDisplay questRecord
			qDisplay, completion = activeQuestRecord(user, questInfo, questStep)

			if !showComplete && completion >= 1 {
				continue
			}

			allQuests = append(allQuests, qDisplay)
		}
	}

	// Finished repeatables count as finished quests either way, so plain
	// `quests` points at `quests all` rather than saying there are none.
	finished := finishedRepeatRows(user.Character, allQuestProgress, showHidden)
	qInfo.QuestsTotal += len(finished)
	if showComplete {
		allQuests = append(allQuests, finished...)
	}

	qInfo.QuestsFound = len(allQuests)
	qInfo.Records = allQuests

	// Sort lexigraphically
	sort.Slice(allQuests, func(i, j int) bool {
		return allQuests[i].Id < allQuests[j].Id
	})

	if line := questsNoneShownLine(qInfo.QuestsFound, qInfo.QuestsTotal); line != `` {
		user.SendText(messaging.CategorySystem, line)
		return true, nil
	}

	questTxt, _ := templates.Process("character/quests", qInfo, user.UserId)
	user.SendText(messaging.CategoryScreen, questTxt)

	return true, nil
}

// questsNoneShownLine is the plain line `quests` prints in place of an empty
// box when nothing is shown (#620: an empty "( 0 out of 2 shown )" box after
// both finished tutorial quests), or "" when there is a list to show.
func questsNoneShownLine(found, total int) string {
	if found > 0 {
		return ``
	}
	if total > 0 {
		return `You have no quests in progress. To see the ones you have finished, type <ansi fg="command">quests all</ansi>.`
	}
	return `You have no quests yet.`
}

// questsShowHidden reports whether `quests all+`, which lists every quest
// including unstarted and secret ones, applies. Admins only (#289); anyone
// else typing it gets the plain `all` view.
func questsShowHidden(rest string, user *users.UserRecord) bool {
	return rest == `all+` && user.Role == users.RoleAdmin
}
