package characters

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// questIsSecret reports whether questId is a secret quest (a hidden progress
// marker the player is never shown). A package variable so tests can stand in
// for the quest registry, which a characters test binary never loads.
var questIsSecret = func(questId int) bool {
	q := quests.GetQuestById(questId)
	return q != nil && q.Secret
}

// IsSecretQuest reports whether questId is a secret quest. Player-facing
// quest surfaces (hint, the Quests panel) use it to keep hidden markers out.
func IsSecretQuest(questId int) bool {
	return questIsSecret(questId)
}

func (c *Character) GetMemoryCapacity() int {
	// Map is now a free command; memory capacity based on Perception
	memCap := (c.Stats.Perception.ValueAdj >> 1)
	if memCap < 0 {
		memCap = 0
	}
	return memCap + 5
}

func (c *Character) GetMapSprawlCapacity() int {
	// Map is now a free command; sprawl capacity based on Perception
	sprawlCap := (c.Stats.Perception.ValueAdj >> 2)
	if sprawlCap < 0 {
		sprawlCap = 0
	}
	return sprawlCap
}

// Remember visiting a room. This may cause to forget an older room if the memory is full.
func (c *Character) RememberRoom(roomId int) {
	mapHistory := c.GetMemoryCapacity()
	if len(c.roomHistory) >= mapHistory*2 {
		// Prune out everything except {mapHistory}-1 items at the end
		c.roomHistory = c.roomHistory[len(c.roomHistory)-(mapHistory-1):]
	}
	c.roomHistory = append(c.roomHistory, roomId)
}

func (c *Character) IsQuestDone(questToken string) bool {
	testQuestId, _ := quests.TokenToParts(questToken)
	if c.QuestProgress == nil {
		c.QuestProgress = make(map[int]string)
	}

	stage := c.QuestProgress[testQuestId]

	return stage == `end`
}

func (c *Character) HasQuest(questToken string) bool {

	if c.QuestProgress == nil {
		c.QuestProgress = make(map[int]string)
	}

	testQuestId, testQuestStep := quests.TokenToParts(questToken)

	currentStep, ok := c.QuestProgress[testQuestId]
	if !ok {
		return false
	}

	// If on that step currently, then true
	if currentStep == testQuestStep {
		return true
	}

	currentToken := quests.PartsToToken(testQuestId, currentStep)

	// If the current token comes after the test token then they've already done that quest
	return quests.IsTokenAfter(questToken, currentToken)
}

func (c *Character) GetQuestProgress() map[int]string {

	if c.QuestProgress == nil {
		c.QuestProgress = make(map[int]string)
	}

	retMap := make(map[int]string)
	for questId, stepName := range c.QuestProgress {
		retMap[questId] = stepName
	}
	return retMap
}

func (c *Character) GiveQuestToken(questToken string) bool {

	if c.QuestProgress == nil {
		c.QuestProgress = make(map[int]string)
	}

	questId, newStep := quests.TokenToParts(questToken)

	// An accept of a shelved quest takes it back up where it was (#375 phase 5).
	if newStep == `start` {
		if _, resumed := c.ResumeShelvedQuest(questId); resumed {
			return true
		}
	}

	currentProgress := c.QuestProgress[questId]

	currentToken := quests.PartsToToken(questId, currentProgress)

	if quests.IsTokenAfter(currentToken, questToken) {
		c.QuestProgress[questId] = newStep
		c.clearQuestStepCounts(questId)
		// A secret quest is a hidden marker the player never sees, so it must
		// not steal the focus (hint, Quests panel, minimap marker) from the
		// quest the player is actually working on.
		if !questIsSecret(questId) {
			c.LastQuestId = questId
		}
		return true
	}

	return false
}

// awakeningQuestId is The Awakening, whose end step is the "Opened" state every
// gate in Pothole Coulee reads (30-end).
const awakeningQuestId = 30

// MarkOpened puts the character at The Awakening's end step without the rite,
// for a character who is Opened by skipping it (the veteran start route, and
// pre-Pothole characters through MigrateNewbieAwakening). It writes the step
// directly because GiveQuestToken refuses `end` on empty progress for a quest of
// several steps, and because the grant pipeline would pay the rite's banner and
// reward line to someone who never stood at the pool. The quest focus stays
// where it is: there is nothing left to do on this quest.
func (c *Character) MarkOpened() {
	if c.QuestProgress == nil {
		c.QuestProgress = make(map[int]string)
	}
	c.QuestProgress[awakeningQuestId] = `end`
}

// QuestStepCount is the saved count: N progress of one count trigger on its
// own step, keyed "<questId>-<stepId>-t<triggerIndex>" (quests.CountKey,
// #375 phase 3b slice 4).
func (c *Character) QuestStepCount(key string) int {
	return c.QuestStepCounts[key]
}

// BumpQuestStepCount adds one to a step's count: N progress and returns the
// new count.
func (c *Character) BumpQuestStepCount(key string) int {
	if c.QuestStepCounts == nil {
		c.QuestStepCounts = map[string]int{}
	}
	c.QuestStepCounts[key]++
	return c.QuestStepCounts[key]
}

// clearQuestStepCounts drops every count of questId: a count belongs to the
// step the player was on, so it goes when that quest's progress moves or is
// cleared (a removal, a repeatable quest's reset). An abandon (planned, #375
// phase 5) must NOT call this: an abandoned step keeps its counts.
func (c *Character) clearQuestStepCounts(questId int) {
	prefix := strconv.Itoa(questId) + "-"
	for key := range c.QuestStepCounts {
		if strings.HasPrefix(key, prefix) {
			delete(c.QuestStepCounts, key)
		}
	}
	if len(c.QuestStepCounts) == 0 {
		c.QuestStepCounts = nil
	}
}

func (c *Character) ClearQuestToken(questToken string) {

	if c.QuestProgress == nil {
		c.QuestProgress = make(map[int]string)
	}

	questId, _ := quests.TokenToParts(questToken)

	delete(c.QuestProgress, questId)
	c.clearQuestStepCounts(questId)
	// A removed quest must not come back off the shelf on the next accept.
	delete(c.ShelvedQuests, questId)
	if len(c.ShelvedQuests) == 0 {
		c.ShelvedQuests = nil
	}

	// Don't leave the focus quest dangling at a quest we just removed. This
	// fires on the common repeatable-quest-completion reset (and explicit quest
	// removal): without it, LastQuestId keeps pointing at a quest no longer in
	// progress, which broke hint (no arg) and leaves the web Quests panel /
	// minimap marker with no focus. Clearing to 0 lets GetFocusQuestId / hint
	// fall back deterministically to an active quest.
	if c.LastQuestId == questId {
		c.LastQuestId = 0
	}
}

// ShelveQuest sets an active quest aside (#375 phase 5, owner ruling 1): its
// step moves from QuestProgress to ShelvedQuests, so every reader of "held"
// sees it as not held. Flags and step counts stay for the resume. A finished
// quest, or one not held, is refused.
func (c *Character) ShelveQuest(questId int) bool {
	step, ok := c.QuestProgress[questId]
	if !ok || step == `end` {
		return false
	}
	if c.ShelvedQuests == nil {
		c.ShelvedQuests = make(map[int]string)
	}
	c.ShelvedQuests[questId] = step
	delete(c.QuestProgress, questId)
	if c.LastQuestId == questId {
		c.LastQuestId = 0
	}
	return true
}

// IsShelved reports whether questId is set aside.
func (c *Character) IsShelved(questId int) bool {
	_, ok := c.ShelvedQuests[questId]
	return ok
}

// ResumeShelvedQuest takes a shelved quest back up at the step it was set
// aside on, and marks it resumed until TakeResumed reads it.
func (c *Character) ResumeShelvedQuest(questId int) (string, bool) {
	step, ok := c.ShelvedQuests[questId]
	if !ok {
		return ``, false
	}
	delete(c.ShelvedQuests, questId)
	if len(c.ShelvedQuests) == 0 {
		c.ShelvedQuests = nil
	}
	if c.QuestProgress == nil {
		c.QuestProgress = make(map[int]string)
	}
	c.QuestProgress[questId] = step
	if !questIsSecret(questId) {
		c.LastQuestId = questId
	}
	if c.resumedQuests == nil {
		c.resumedQuests = make(map[int]bool)
	}
	c.resumedQuests[questId] = true
	return step, true
}

// TakeResumed reports, once, whether questId was just resumed from the shelf:
// the grant banner reads it to say "You take up ... again".
func (c *Character) TakeResumed(questId int) bool {
	if !c.resumedQuests[questId] {
		return false
	}
	delete(c.resumedQuests, questId)
	return true
}

func (c *Character) SetQuestFlag(key, value string) {
	if c.QuestFlags == nil {
		c.QuestFlags = make(map[string]string)
	}
	// Runtime defense-in-depth: log if flag isn't in the registry.
	// Skip if registry is empty (test environment / data not loaded yet).
	if len(quests.GetFlagRegistry()) > 0 {
		if err := quests.ValidateFlag(key, value); err != nil {
			mudlog.Error("SetQuestFlag", "warning", err.Error())
		}
	}
	c.QuestFlags[key] = value
}

func (c *Character) GetQuestFlag(key string) string {
	if c.QuestFlags == nil {
		return ""
	}
	return c.QuestFlags[key]
}

func (c *Character) HasQuestFlag(key string) bool {
	if c.QuestFlags == nil {
		return false
	}
	_, ok := c.QuestFlags[key]
	return ok
}

func (c *Character) ClearQuestFlag(key string) {
	if c.QuestFlags == nil {
		return
	}
	delete(c.QuestFlags, key)
}

// questCooldownKey is the MiscData key under which a repeatable quest's
// "available again at round N" timestamp is stored.
func questCooldownKey(questId int) string {
	return fmt.Sprintf("questcd-%d", questId)
}

// SetQuestCooldown records that quest questId may not be re-taken until
// cooldownRounds rounds from now. Stored in persisted MiscData so the
// cooldown survives logout.
func (c *Character) SetQuestCooldown(questId int, cooldownRounds uint64) {
	c.SetMiscData(questCooldownKey(questId), util.GetRoundCount()+cooldownRounds)
}

// QuestCooldownActive reports whether quest questId is still inside its
// post-completion cooldown window (current round < stored "available at").
func (c *Character) QuestCooldownActive(questId int) bool {
	v := c.GetMiscData(questCooldownKey(questId))
	if v == nil {
		return false
	}
	return util.GetRoundCount() < miscDataToUint64(v)
}

// RecordCompletion counts one completed run of a repeatable quest. The
// completion pass calls it beside the progress reset and the cooldown stamp.
func (c *Character) RecordCompletion(q *quests.Quest) {
	if q == nil || !q.Repeatable {
		return
	}
	if c.QuestCompletions == nil {
		c.QuestCompletions = make(map[int]int)
	}
	c.QuestCompletions[q.QuestId]++
	c.SetQuestCooldown(q.QuestId, uint64(q.CooldownRounds))
}

// RepeatCapReached reports whether a repeatable quest with a repeat_limit has
// been completed that many times.
func (c *Character) RepeatCapReached(q *quests.Quest) bool {
	return q != nil && q.Repeatable && q.RepeatLimit > 0 && c.QuestCompletions[q.QuestId] >= q.RepeatLimit
}

// LastRepeat reports whether the next run of a repeatable quest is the last
// its repeat_limit allows.
func (c *Character) LastRepeat(q *quests.Quest) bool {
	return q != nil && q.Repeatable && q.RepeatLimit > 0 && c.QuestCompletions[q.QuestId] == q.RepeatLimit-1
}

// QuestOpen reports whether q can be offered to this character now: always
// for a non-repeatable (its gates decide), and for a repeatable only off
// cooldown and under its cap (#375 P3). It backs PlayerState.QuestOpen.
func (c *Character) QuestOpen(q *quests.Quest) bool {
	if q == nil || !q.Repeatable {
		return true
	}
	return !c.QuestCooldownActive(q.QuestId) && !c.RepeatCapReached(q)
}

// RenameQuestSteps moves saved progress through the step-id manifest's
// renames ("<qid>-<old>" -> "<qid>-<new>"), following chains. A rename that
// changes the quest id is ignored: a step cannot move between quests.
// The shelf holds steps the same way, so it is renamed too (#375 phase 5).
func (c *Character) RenameQuestSteps(renames map[string]string) {
	renameQuestSteps(c.QuestProgress, renames)
	renameQuestSteps(c.ShelvedQuests, renames)
}

func renameQuestSteps(progress map[int]string, renames map[string]string) {
	if len(renames) == 0 || len(progress) == 0 {
		return
	}
	for qid, step := range progress {
		tok := quests.PartsToToken(qid, step)
		for hops := 0; hops < len(renames); hops++ {
			n, ok := renames[tok]
			if !ok {
				break
			}
			tok = n
		}
		nid, nstep := quests.TokenToParts(tok)
		if nid == qid {
			progress[qid] = nstep
		}
	}
}

// RetiredQuestSteps lists, sorted, the saved current steps that the step-id
// manifest retires ("<qid>-<step>"). It changes nothing: a retired current
// step needs a content decision, so LoadUser only logs it (spec 5.5).
func (c *Character) RetiredQuestSteps(retired map[string]bool) []string {
	return retiredQuestSteps(c.QuestProgress, retired)
}

// RetiredShelvedSteps is RetiredQuestSteps over the shelf (#375 phase 5):
// a shelved step resumes as is, so a retired one needs the same decision.
func (c *Character) RetiredShelvedSteps(retired map[string]bool) []string {
	return retiredQuestSteps(c.ShelvedQuests, retired)
}

func retiredQuestSteps(progress map[int]string, retired map[string]bool) []string {
	if len(retired) == 0 {
		return nil
	}
	var out []string
	for qid, step := range progress {
		if tok := quests.PartsToToken(qid, step); retired[tok] {
			out = append(out, tok)
		}
	}
	sort.Strings(out)
	return out
}

// DropRetiredQuests clears every quest in retired (quests.RetiredQuests:
// retired whole in the step-id manifest) from progress and the shelf, through
// ClearQuestToken, which also moves the focus off it. It returns the ids it
// dropped, sorted. Idempotent, so it runs on every load with no marker.
func (c *Character) DropRetiredQuests(retired map[int]bool) []int {
	var out []int
	for qid := range retired {
		_, held := c.QuestProgress[qid]
		_, shelved := c.ShelvedQuests[qid]
		if !held && !shelved {
			continue
		}
		c.ClearQuestToken(quests.PartsToToken(qid, `start`))
		out = append(out, qid)
	}
	sort.Ints(out)
	return out
}

// miscDataToUint64 coerces a MiscData value to uint64. MiscData stores any,
// and an integer written as uint64 may return from YAML reload as int,
// int64, or float64 — handle all of them; anything else yields 0.
func miscDataToUint64(v any) uint64 {
	switch n := v.(type) {
	case uint64:
		return n
	case int:
		if n < 0 {
			return 0
		}
		return uint64(n)
	case int64:
		if n < 0 {
			return 0
		}
		return uint64(n)
	case float64:
		if n < 0 {
			return 0
		}
		return uint64(n)
	default:
		return 0
	}
}

// GetFocusQuestId returns the player's "focused" quest — the one hint (no arg),
// the web Quests panel, and the minimap marker all resolve against. It is
// LastQuestId when that quest is still in progress; otherwise (LastQuestId is
// zero, or points at a quest that has since been removed from progress) it
// falls back deterministically to the lowest-id quest still in progress that
// is not yet complete, then to the lowest-id quest of any kind, then 0. The
// determinism matters: the marker re-resolves on every emit, so a random
// fallback would make the focus jitter between quests. Pure — never mutates.
//
// Secret quests are never the focus, on any path: they are hidden markers
// (quest 21, Newcomer's Path, is one) that `quests` does not list, so naming
// one in hint or the panel would show the player a quest they do not have.
// When every quest in progress is secret the result is 0.
func (c *Character) GetFocusQuestId() int {
	prog := c.GetQuestProgress()
	for questId := range prog {
		if questIsSecret(questId) {
			delete(prog, questId)
		}
	}
	if len(prog) == 0 {
		return 0
	}
	// A still-valid, still-ACTIVE explicit focus wins. We skip a LastQuestId
	// that sits at "end": completing a quest sets LastQuestId to it, so without
	// this a just-finished quest would stay the focus (hint/panel/marker) even
	// when the player has moved on to another active quest — e.g. after the
	// Toke's Road hand-in leads into Find Your Footing.
	if c.LastQuestId != 0 {
		if step, ok := prog[c.LastQuestId]; ok && step != "end" {
			return c.LastQuestId
		}
	}
	// Fallback: lowest-id quest still in progress and not at "end".
	best := 0
	for questId, step := range prog {
		if step == "end" {
			continue
		}
		if best == 0 || questId < best {
			best = questId
		}
	}
	if best != 0 {
		return best
	}
	// Everything is complete: lowest-id quest, so hint can still report it.
	for questId := range prog {
		if best == 0 || questId < best {
			best = questId
		}
	}
	return best
}
