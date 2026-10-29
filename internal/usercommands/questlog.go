package usercommands

import (
	"fmt"
	"slices"
	"strings"
	"sync"

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
	questAmbiguousLine = `More than one quest matches that. Type more of its name.`

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

// QuestGiver is the name of the first NPC whose offer can take questId back
// off the shelf (else the first that offers it), or a plain stand-in phrase
// when none is known.
func QuestGiver(questId int) string {
	ids := dialogue.Resumers(questId)
	if len(ids) == 0 {
		ids = dialogue.Offerers(questId)
	}
	if len(ids) > 0 {
		if name := giverName(ids[0]); name != `` {
			return name
		}
	}
	return questGiverFallback
}

// autoStarted is the quests whose -start a quest trigger or a behaviour tree
// grants by itself (questloop.AutoStarts), set at boot by the offer index
// door in main_quests.go.
var (
	autoStartedMu sync.RWMutex
	autoStarted   = map[int]bool{}
)

// SetAutoStarted replaces the quests whose -start is granted automatically
// (owner ruling 6): none of them can be abandoned.
func SetAutoStarted(questIds []int) {
	m := make(map[int]bool, len(questIds))
	for _, id := range questIds {
		m[id] = true
	}
	autoStartedMu.Lock()
	autoStarted = m
	autoStartedMu.Unlock()
}

// SetAutoStartedForTest is SetAutoStarted with a restore func.
func SetAutoStartedForTest(questIds []int) func() {
	autoStartedMu.RLock()
	prev := autoStarted
	autoStartedMu.RUnlock()
	SetAutoStarted(questIds)
	return func() {
		autoStartedMu.Lock()
		autoStarted = prev
		autoStartedMu.Unlock()
	}
}

func isAutoStarted(questId int) bool {
	autoStartedMu.RLock()
	defer autoStartedMu.RUnlock()
	return autoStarted[questId]
}

// Abandonable reports whether the player may set questId aside: active, not
// finished, not secret, and offered by some NPC (owner ruling 3) through a
// topic that can show again while it is shelved (dialogue.Resumable: an offer
// hidden by another quest's token, like quest 88's, would lose it), and its
// -start is not granted automatically by a quest trigger or a behaviour tree
// (owner ruling 6: that grant would take it back up without the player
// choosing to, like quest 30's room_enter 5200).
func Abandonable(c *characters.Character, questId int) bool {
	step, ok := c.GetQuestProgress()[questId]
	if !ok || step == `end` || characters.IsSecretQuest(questId) {
		return false
	}
	return dialogue.Resumable(questId) && !isAutoStarted(questId)
}

// findActiveQuest matches name against the player's active, non-secret,
// unfinished quests, sorted by name so the answer never depends on map
// order: a full match first, then a close match (util.FindMatchIn). When no
// name matches in full and more than one matches in part, it picks none and
// reports ambiguous, so the player is asked for more of the name. A numbered
// search ("2.the") is the player choosing among them and goes to FindMatchIn
// as typed. 0 and false when nothing matches.
func findActiveQuest(user *users.UserRecord, name string) (id int, ambiguous bool) {
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
	slices.Sort(names)
	search := strings.TrimSpace(name)
	match, closeMatch := util.FindMatchIn(search, names...)
	if match != `` {
		return ids[match], false
	}
	if closeMatch == `` {
		return 0, false
	}
	if _, n := util.GetMatchNumber(search); n > 1 {
		return ids[closeMatch], false
	}
	partial := 0
	for _, n := range names {
		if _, c := util.FindMatchIn(search, n); c != `` {
			partial++
		}
	}
	if partial > 1 {
		return 0, true
	}
	return ids[closeMatch], false
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
