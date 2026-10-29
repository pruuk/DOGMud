package usercommands

import (
	"fmt"
	"math"
	"sort"
	"strings"

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
	Following   bool // the quest the map and hint follow, marked "(following)" (#375 phase 5)
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
		// A shelved run has its set-aside row (shelvedRows).
		if c.IsShelved(questId) {
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
		Following:   questStep != `all+` && questStep != `end` && questInfo.QuestId == user.Character.GetFocusQuestId(),
		Description: description,
		Progress:    progress,
		Guide:       guide,
		Completion:  fmt.Sprintf(`%d%%`, int(math.Floor(completion*100))),
		BarFull:     barFull,
		BarEmpty:    barEmpty,
	}, completion
}

// questsSubcommand splits `quests abandon <name>` / `quests focus <name>`;
// anything else is the list's own argument ("", "all", "all+").
func questsSubcommand(rest string) [2]string {
	word, arg, _ := strings.Cut(strings.TrimSpace(rest), ` `)
	switch strings.ToLower(word) {
	case `abandon`, `focus`:
		return [2]string{strings.ToLower(word), strings.TrimSpace(arg)}
	}
	return [2]string{``, rest}
}

// shelvedRows are the `quests all` rows for quests set aside, in quest id
// order. They carry no step description, bar or percentage.
func shelvedRows(c *characters.Character) []questRecord {
	var rows []questRecord
	for questId := range c.ShelvedQuests {
		q := quests.GetQuestById(questId)
		if q == nil || q.Secret {
			continue
		}
		rows = append(rows, questRecord{Id: q.QuestId, Name: q.Name,
			Description: fmt.Sprintf(`Set aside. Talk to %s to take it up again.`, QuestGiver(questId))})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Id < rows[j].Id })
	return rows
}

// SendQuestLogLine sends a quest log action's reply (#375 phase 5): a system
// line on success, a refusal otherwise, wrapped to the reader's width because
// CategorySystem is never wrapped by the pipeline (it carries tables), and the
// abandon line runs past 80 columns.
func SendQuestLogLine(user *users.UserRecord, ok bool, line string) {
	cat := messaging.CategorySystem
	if !ok {
		cat = messaging.CategoryRefusal
	}
	user.SendText(cat, messaging.WrapAnsi(line, user.GetLineWidth()))
}

func Quests(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	switch sub := questsSubcommand(rest); sub[0] {
	case `abandon`, `focus`:
		id := findActiveQuest(user, sub[1])
		if id == 0 {
			user.SendText(messaging.CategoryRefusal, questNoSuchLine)
			return true, nil
		}
		if sub[0] == `focus` {
			if FocusQuest(user, id) {
				// Re-send Char.Quests so the panel and marker follow.
				events.AddToQueue(events.Quest{UserId: user.UserId, QuestToken: quests.PartsToToken(id, user.Character.GetQuestProgress()[id]), MarkerOnly: true})
				SendQuestLogLine(user, true, fmt.Sprintf(`You turn your attention to %s.`, quests.GetQuestById(id).Name))
			} else {
				user.SendText(messaging.CategoryRefusal, questNoSuchLine)
			}
			return true, nil
		}
		ok, line := AbandonQuest(user, id)
		SendQuestLogLine(user, ok, line)
		return true, nil
	}

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
			// A shelved quest gets its set-aside row below.
			if user.Character.IsShelved(quest.QuestId) {
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

	// Sort lexigraphically
	sort.Slice(allQuests, func(i, j int) bool {
		return allQuests[i].Id < allQuests[j].Id
	})

	// Quests set aside count as unshown in plain `quests`; `quests all` lists
	// them after the active ones.
	shelved := shelvedRows(user.Character)
	qInfo.QuestsTotal += len(shelved)
	if showComplete {
		allQuests = append(allQuests, shelved...)
	}

	qInfo.QuestsFound = len(allQuests)
	qInfo.Records = allQuests

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
