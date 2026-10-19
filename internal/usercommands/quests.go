package usercommands

import (
	"fmt"
	"math"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func Quests(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	type QuestRecord struct {
		Id          int
		Name        string
		Description string
		Completion  string
		BarFull     string
		BarEmpty    string
	}

	type QuestInfo struct {
		QuestsTotal int
		QuestsFound int
		Records     []QuestRecord
	}

	showHidden := questsShowHidden(rest, user)
	showComplete := rest == `all` || rest == `all+`

	qInfo := QuestInfo{}
	allQuests := []QuestRecord{}
	var completion float64

	qInfo.QuestsTotal = 0

	allQuestProgress := user.Character.GetQuestProgress()

	if showHidden {
		for _, quest := range quests.GetAllQuests() {
			if _, ok := allQuestProgress[quest.QuestId]; ok {
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

			totalSteps := len(questInfo.Steps)
			completedSteps := 0
			description := questInfo.Description

			if questStep != `all+` {
				for _, step := range questInfo.Steps {
					completedSteps++
					if step.Id == questStep {
						description = step.Description
						break
					}
				}
			}

			completion = float64(completedSteps) / float64(totalSteps)

			if !showComplete && completion >= 1 {
				continue
			}

			barFull, barEmpty := util.ProgressBar(completion, 25)

			qDisplay := QuestRecord{
				Id:          questInfo.QuestId,
				Name:        questInfo.Name,
				Description: description,
				Completion:  fmt.Sprintf(`%d%%`, int(math.Floor(completion*100))),
				BarFull:     barFull,
				BarEmpty:    barEmpty,
			}

			allQuests = append(allQuests, qDisplay)
		}
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
