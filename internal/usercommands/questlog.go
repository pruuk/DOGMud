package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Quest log actions (#375 phase 5, spec sections 2 and 4).
const (
	questNoSuchLine    = `You have no quest called that.`
	questNoWayBackLine = `Nobody would take this back from you. It has to be finished.`

	// questGiverFallback stands in when the offering NPC has no name.
	questGiverFallback = `the one who gave it to you`
)

// giverName is the NPC name for a mob id, a seam for tests.
var giverName = func(mobId int) string {
	if m := mobs.GetMobSpec(mobs.MobId(mobId)); m != nil {
		return m.Character.Name
	}
	return ``
}

func setGiverNameForTest(names map[int]string) func() {
	prev := giverName
	giverName = func(id int) string { return names[id] }
	return func() { giverName = prev }
}

// QuestGiver is the name of the first NPC that offers questId, or a plain
// stand-in phrase when none is known.
func QuestGiver(questId int) string {
	if ids := dialogue.Offerers(questId); len(ids) > 0 {
		if name := giverName(ids[0]); name != `` {
			return name
		}
	}
	return questGiverFallback
}

// Abandonable reports whether the player may set questId aside: active, not
// finished, not secret, and offered by some NPC (owner ruling 3).
func Abandonable(c *characters.Character, questId int) bool {
	step, ok := c.GetQuestProgress()[questId]
	if !ok || step == `end` || characters.IsSecretQuest(questId) {
		return false
	}
	return len(dialogue.Offerers(questId)) > 0
}

// findActiveQuest matches name against the player's active, non-secret,
// unfinished quests: a full match first, then a close match. 0 when none.
func findActiveQuest(user *users.UserRecord, name string) int {
	ids := map[string]int{}
	var names []string
	for id, step := range user.Character.GetQuestProgress() {
		q := quests.GetQuestById(id)
		if q == nil || q.Secret || step == `end` {
			continue
		}
		ids[q.Name] = id
		names = append(names, q.Name)
	}
	match, closeMatch := util.FindMatchIn(strings.TrimSpace(name), names...)
	if match != `` {
		return ids[match]
	}
	return ids[closeMatch]
}

// AbandonQuest sets questId aside and returns the line to show the player.
// On success it queues a MarkerOnly events.Quest: the gmcp listener for that
// event re-sends Char.Quests and the quest hook refreshes the NPC markers, so
// callers need not.
func AbandonQuest(user *users.UserRecord, questId int) (bool, string) {
	step, ok := user.Character.GetQuestProgress()[questId]
	if !ok || step == `end` || characters.IsSecretQuest(questId) {
		return false, questNoSuchLine
	}
	if !Abandonable(user.Character, questId) {
		return false, questNoWayBackLine
	}
	q := quests.GetQuestById(questId)
	if q == nil || !user.Character.ShelveQuest(questId) {
		return false, questNoSuchLine
	}
	events.AddToQueue(events.Quest{UserId: user.UserId, QuestToken: quests.PartsToToken(questId, step), MarkerOnly: true})
	return true, fmt.Sprintf(`You set %s aside. Talk to %s to take it up again. You will pick up where you left off.`,
		q.Name, QuestGiver(questId))
}

// FocusQuest makes questId the quest the map, hint and panel follow. It holds
// the checks the Char.Quests.Focus handler always had (active, not secret).
// The caller re-sends Char.Quests (the GMCP handler queues GMCPCharUpdate; the
// command queues a MarkerOnly events.Quest).
func FocusQuest(user *users.UserRecord, questId int) bool {
	if _, ok := user.Character.GetQuestProgress()[questId]; !ok {
		return false
	}
	if characters.IsSecretQuest(questId) {
		return false
	}
	user.Character.LastQuestId = questId
	return true
}
