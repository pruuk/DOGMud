# #375 phase 5: quest log actions (abandon and focus), implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Players can abandon a quest (shelved: it stops counting, its giver's `[!]` returns, and
re-accepting resumes the saved step with no handout of resale value) and can focus a quest by
name, from the `quests` command and from a right-click menu on the web quest panel.

**Architecture:** A new `Character.ShelvedQuests` map holds an abandoned quest's step outside
`QuestProgress`, so every reader of "held" sees it as not held with no change. Any grant of
`N-start` resumes from the shelf (one method, called from `GiveQuestToken`). A boot-built offer
index in `dialogue` says which quests an NPC offers (only those can be abandoned). During a
resume, the dialogue offer and `GameBridge` hand over only items worth nothing that the player
lacks, and no gold. `usercommands.AbandonQuest` / `FocusQuest` back the command, GMCP and the
web menu.

**Tech stack:** Go (`internal/characters`, `dialogue`, `conversation`, `questengine`, `hooks`,
`usercommands`, `modules/gmcp`), templates, plain JavaScript in the web client, node checks.

**Spec:** `docs/superpowers/specs/2026-10-28-quest-arc-phase5-quest-log-actions-design.md`
(owner approved 2026-10-28; rulings 1 to 5). Its facts table is the source for every line number
below; re-read each file before editing, as lines move.

---

## Ground rules

- Edit files with the Edit and Write tools only (no `sed -i`, no `>` into tracked files, no Python
  rewrites). `git add` named paths only; never `-A`, `.`, `--all`, `commit -a`, `--no-verify`.
- Commits via quoted heredoc ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Every new test is seen failing before it passes (write it, run it, see the failure, then code).
- No em or en dashes in code comments, copy or docs. Player copy follows `dogmud-player-copy`.
- Colour never carries meaning alone (owner is green/blue colourblind).
- Every `gh` command carries `--repo pruuk/DOGMud`.
- Servers: private ports only, kill by PID only. Edge leftovers are checked by command line
  (`Win32_Process`), not by a printed PID.
- Work in worktree `C:/tmp/dogmud-375-p5`, branch `feat/375-p5-quest-log`; record it in memory.

## Facts added for the plan (master `4342f3be8`)

| Fact | Where |
|---|---|
| `Character.QuestStepCounts` is cleared by `GiveQuestToken` on advance (`clearQuestStepCounts`) and by `ClearQuestToken`; shelving must use neither | `internal/characters/quests.go:104-131, 151-183` |
| Test seams: `quests.SeedQuestsForTest(q...) func()`; `questIsSecret` var in package `characters`; `users.NewTestUser(id, username, char, connId)`, `users.SeedUsersForTest(map) func()` | `internal/quests`; `internal/characters/questfocus_test.go:64`; `internal/users/test_helpers.go:29, 56` |
| `items.GetItemSpec(itemId) *ItemSpec`, `.Value int` | `internal/items/itemspec.go:814, 269` |
| `dialogue.PlayerState` callbacks (`HasQuest`, `HasItem`, `CarriesItem`, `GiveItem`, `GiveQuest`, ...) built for live players in `conversation.playerState` | `internal/dialogue/types.go:6`; `internal/conversation/playerstate.go` |
| Dialogue `Apply` order: charge, then `applyPaidEffects` -> `applyQuestEffects` (item, requiresItem, quest, flag, rep, gold) | `internal/dialogue/topics.go:174-190`; `engine.go:112-160` |
| `Char.Quests` payload built per active quest at `gmcp.Char.go:665-735`; `Char.Quests.Focus` handler `gmcp.CharOp.go:135-165` | `modules/gmcp` |
| Web: quest rows `.qrow[data-quest-id]` rendered at `webclient-pure.html:1018`, click handlers bound at `:1076-1082`; the automation context menu pattern (`buildMenu`, `closeMenu`, `makeCtxRow`, `openMenu`, delegated `contextmenu`) at `:3737-3844`; CSS `.auto-ctx` and `.ctx-row` in `static/css/dashboard.css:781-820` | web client |

## File map

| File | Change |
|---|---|
| `internal/characters/character.go` | `ShelvedQuests` field |
| `internal/characters/quests.go` | `ShelveQuest`, `IsShelved`, `ResumeShelvedQuest`, `TakeResumed`; `GiveQuestToken` resumes |
| `internal/characters/quest_shelf_test.go` (new) | shelf tests |
| `internal/dialogue/offers.go` (new), `offers_test.go` (new) | offer index |
| `internal/dialogue/save.go` | index update on save |
| `main_quests.go`, `main.go` | build the index at boot |
| `internal/dialogue/types.go`, `topics.go`, `engine.go`, `resume_test.go` (new) | `Shelved` callback; resume effects |
| `internal/conversation/playerstate.go` | wire `Shelved` |
| `internal/questengine/bridge.go`, `bridge_resume_test.go` (new) | resumed bridge, banner |
| `internal/hooks/Quest_HandleQuestUpdate.go`, `Quest_Resume_test.go` (new) | resume banner, resumed bridge |
| `internal/usercommands/questlog.go` (new), `questlog_test.go` (new), `quests.go` | abandon, focus, `quests all` rows |
| `_datafiles/world/dogmud/templates/help/quests.template` | two new lines |
| `modules/gmcp/gmcp.Char.go`, `gmcp.CharOp.go`, tests | `abandonable`; `Char.Quests.Abandon`; Focus via `FocusQuest` |
| `_datafiles/html/public/webclient-pure.html`, `static/css/dashboard.css` | quest menu |
| `tools/webclient-tests/quest-panel-menu.js` (new) | stub-DOM check |
| docs (Task 11) | context.md files, roadmap, skill, help, PATCH_NOTES, README |

---

### Task 0: Worktree and spec commit

- [ ] **Step 1:** From the main checkout (spec and this plan are uncommitted there):

```bash
cd "/c/Users/Calabe Davis/workspace/DOGMud"
git fetch -q origin && git worktree add -b feat/375-p5-quest-log C:/tmp/dogmud-375-p5 origin/master
cp docs/superpowers/specs/2026-10-28-quest-arc-phase5-quest-log-actions-design.md C:/tmp/dogmud-375-p5/docs/superpowers/specs/
cp docs/superpowers/plans/2026-10-28-quest-arc-phase5-quest-log-actions.md C:/tmp/dogmud-375-p5/docs/superpowers/plans/
```

- [ ] **Step 2:** In the worktree add both to `docs/README.md` (after the phase 4 slice 3 plan
  row): the spec row ("#375 phase 5: abandon shelves a quest (not held, its `[!]` returns,
  re-accept resumes), no handout of resale value on a resume, abandon needs an NPC offer,
  `quests focus` / `quests abandon` and a right-click menu on the web quest panel") and the plan
  row ("Plan for phase 5 in 12 tasks"). Commit spec, plan, README. Then remove the two
  uncommitted copies from the main checkout (`rm` the two files there; they are now committed on
  the branch).
- [ ] **Step 3:** Memory `project-375-phase5-worktree.md` naming the worktree and branch, plus its
  `MEMORY.md` line.

### Task 1: The shelf on `Character`

**Files:** `internal/characters/character.go`, `internal/characters/quests.go`, create
`internal/characters/quest_shelf_test.go`

- [ ] **Step 1: Write the failing tests.**

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func shelfQuest(t *testing.T) {
	t.Helper()
	t.Cleanup(quests.SeedQuestsForTest(&quests.Quest{
		QuestId: 42, Name: "The Hunt",
		Steps: []quests.QuestStep{{Id: "start"}, {Id: "track"}, {Id: "hunt"}, {Id: "end"}},
	}))
}

// Abandoning moves the step to the shelf: the quest is not held, its flags
// and counts stay, and focus leaves it (spec section 1, owner ruling 1).
func TestShelveQuest_MovesTheStep(t *testing.T) {
	shelfQuest(t)
	c := &Character{QuestProgress: map[int]string{42: "track"}, LastQuestId: 42,
		QuestFlags: map[string]string{"42-x": "y"}}
	c.BumpQuestStepCount(quests.CountKey(42, "track", 0))

	require.True(t, c.ShelveQuest(42))
	require.False(t, c.HasQuest("42-start"))
	_, held := c.GetQuestProgress()[42]
	require.False(t, held)
	require.True(t, c.IsShelved(42))
	require.Equal(t, "y", c.GetQuestFlag("42-x"))
	require.Equal(t, 1, c.QuestStepCount(quests.CountKey(42, "track", 0)))
	require.Equal(t, 0, c.LastQuestId)

	require.False(t, c.ShelveQuest(42), "not held any more")
	require.False(t, c.ShelveQuest(7), "never held")
}

func TestShelveQuest_RefusesAFinishedQuest(t *testing.T) {
	shelfQuest(t)
	c := &Character{QuestProgress: map[int]string{42: "end"}}
	require.False(t, c.ShelveQuest(42))
	require.False(t, c.IsShelved(42))
}

// Any grant of N-start resumes the shelved step and keeps the counts.
func TestGiveQuestToken_ResumesTheShelf(t *testing.T) {
	shelfQuest(t)
	c := &Character{QuestProgress: map[int]string{42: "track"}}
	c.BumpQuestStepCount(quests.CountKey(42, "track", 0))
	require.True(t, c.ShelveQuest(42))

	require.True(t, c.GiveQuestToken("42-start"))
	require.Equal(t, "track", c.GetQuestProgress()[42])
	require.False(t, c.IsShelved(42))
	require.Equal(t, 42, c.LastQuestId)
	require.Equal(t, 1, c.QuestStepCount(quests.CountKey(42, "track", 0)), "a resume keeps counts")
	require.True(t, c.TakeResumed(42))
	require.False(t, c.TakeResumed(42), "taken once")
}

// A grant of a later token while shelved is not a resume: the quest is not
// held, and only an accept (N-start) takes it back up.
func TestGiveQuestToken_LaterTokenWhileShelvedDoesNothing(t *testing.T) {
	shelfQuest(t)
	c := &Character{QuestProgress: map[int]string{42: "track"}}
	require.True(t, c.ShelveQuest(42))
	require.False(t, c.GiveQuestToken("42-hunt"))
	require.True(t, c.IsShelved(42))
	_, held := c.GetQuestProgress()[42]
	require.False(t, held)
}

func TestShelvedQuests_SurviveYAMLRoundTrip(t *testing.T) {
	c := &Character{Name: "Mira", ShelvedQuests: map[int]string{42: "track"}}
	out, err := yaml.Marshal(c)
	require.NoError(t, err)
	require.Contains(t, string(out), "shelvedquests:")
	var back Character
	require.NoError(t, yaml.Unmarshal(out, &back))
	require.True(t, back.IsShelved(42))

	out, err = yaml.Marshal(&Character{Name: "Mira"})
	require.NoError(t, err)
	require.NotContains(t, string(out), "shelvedquests")
}
```

- [ ] **Step 2:** `go test ./internal/characters/ -run 'Shelve|ShelvedQuests|ResumesTheShelf|LaterTokenWhileShelved'` : FAIL (undefined `ShelveQuest`, `ShelvedQuests`).
- [ ] **Step 3: Implement.** In `character.go`, after `QuestFlags` (line 322):

```go
	ShelvedQuests           map[int]string                 `yaml:"shelvedquests,omitempty"`       // abandoned quests: id -> step reached (#375 phase 5)
```

  and an unexported, unsaved field beside the other unexported ones in the struct:

```go
	resumedQuests map[int]bool // quests resumed from the shelf, until the grant banner takes them (#375 phase 5)
```

  In `quests.go`, after `ClearQuestToken`:

```go
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
```

  In `GiveQuestToken`, right after `questId, newStep := quests.TokenToParts(questToken)`:

```go
	// An accept of a shelved quest takes it back up where it was (#375 phase 5).
	if newStep == `start` {
		if _, resumed := c.ResumeShelvedQuest(questId); resumed {
			return true
		}
	}
```

- [ ] **Step 4:** Rerun the tests: PASS. Then `go test ./internal/characters/`: PASS.
- [ ] **Step 5:** Commit (`feat(characters): shelve and resume quests (#375 p5)`).

### Task 2: The offer index

**Files:** create `internal/dialogue/offers.go`, `internal/dialogue/offers_test.go`; modify
`internal/dialogue/save.go`, `main_quests.go`, `main.go`

- [ ] **Step 1: Failing test.**

```go
package dialogue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOfferIndex(t *testing.T) {
	t.Cleanup(SetOfferIndexForTest(nil))
	BuildOfferIndex([]*DialogueFile{
		{MobId: 9137, Topics: []Topic{
			{Key: "hunt_offer", Kind: KindOffer, Effects: Effects{GrantsQuest: "42-start"}},
			{Key: "hunt_turnin", Kind: KindHandIn, Effects: Effects{GrantsQuest: "42-end"}},
		}},
		{MobId: 9106, Topics: []Topic{{Key: "a", Effects: Effects{GrantsQuest: "32-start"}}}},
		{MobId: 9108, Topics: []Topic{{Key: "b", Effects: Effects{GrantsQuest: "32-start"}}}},
	})
	require.Equal(t, []int{9137}, Offerers(42))
	require.Equal(t, []int{9106, 9108}, Offerers(32))
	require.Empty(t, Offerers(28))

	// A save replaces that file's offers only.
	UpdateOfferIndex(&DialogueFile{MobId: 9106})
	require.Equal(t, []int{9108}, Offerers(32))
	require.Equal(t, []int{9137}, Offerers(42))
}
```

- [ ] **Step 2:** `go test ./internal/dialogue/ -run TestOfferIndex` : FAIL (undefined).
- [ ] **Step 3: Implement `offers.go`.**

```go
package dialogue

import (
	"slices"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/quests"
)

// The offer index (#375 phase 5): which NPCs offer each quest, that is, have a
// topic granting its -start. A quest may be abandoned only when some NPC
// offers it (owner ruling 3). Dialogue loads lazily per NPC, so the index is
// built once at boot from every file and kept current by SaveDialogueFile.
var (
	offerMu    sync.RWMutex
	offersByMob = map[int][]int{} // mob id -> quest ids it offers
	offerers    = map[int][]int{} // quest id -> mob ids, sorted
)

func offeredQuests(df *DialogueFile) []int {
	var out []int
	for _, t := range df.Topics {
		qid, step := quests.TokenToParts(t.GrantsQuest)
		if qid > 0 && step == `start` && !slices.Contains(out, qid) {
			out = append(out, qid)
		}
	}
	return out
}

func rebuildOfferersLocked() {
	offerers = map[int][]int{}
	for mob, qs := range offersByMob {
		for _, q := range qs {
			offerers[q] = append(offerers[q], mob)
		}
	}
	for q := range offerers {
		slices.Sort(offerers[q])
	}
}

// BuildOfferIndex replaces the index from files.
func BuildOfferIndex(files []*DialogueFile) {
	offerMu.Lock()
	defer offerMu.Unlock()
	offersByMob = map[int][]int{}
	for _, df := range files {
		if df != nil {
			offersByMob[df.MobId] = offeredQuests(df)
		}
	}
	rebuildOfferersLocked()
}

// UpdateOfferIndex replaces one file's offers.
func UpdateOfferIndex(df *DialogueFile) {
	if df == nil {
		return
	}
	offerMu.Lock()
	defer offerMu.Unlock()
	offersByMob[df.MobId] = offeredQuests(df)
	rebuildOfferersLocked()
}

// Offerers is the sorted mob ids that offer questId (shared; do not modify).
func Offerers(questId int) []int {
	offerMu.RLock()
	defer offerMu.RUnlock()
	return offerers[questId]
}

// SetOfferIndexForTest replaces the index and returns a restore func.
func SetOfferIndexForTest(byQuest map[int][]int) func() {
	offerMu.Lock()
	prevMob, prevQ := offersByMob, offerers
	offersByMob = map[int][]int{}
	offerers = map[int][]int{}
	for q, mobs := range byQuest {
		for _, m := range mobs {
			offersByMob[m] = append(offersByMob[m], q)
		}
	}
	rebuildOfferersLocked()
	offerMu.Unlock()
	return func() {
		offerMu.Lock()
		offersByMob, offerers = prevMob, prevQ
		offerMu.Unlock()
	}
}
```

  Check `quests.TokenToParts` on an empty string returns `(0, "")` before relying on it (read
  `internal/quests`); if not, guard `t.GrantsQuest == ""` first.

- [ ] **Step 4:** In `save.go` `SaveDialogueFile`, after the cache write (`dialogueCache[key] = &cp`), add
  `UpdateOfferIndex(&cp)`.
- [ ] **Step 5:** In `main_quests.go` add:

```go
// offerIndexDoor builds the dialogue offer index (#375 phase 5): which NPCs
// offer each quest, for `quests abandon`. Dialogue loads lazily, so this reads
// every file once.
func offerIndexDoor() {
	files, _ := dialogue.LoadAllFiles()
	dialogue.BuildOfferIndex(files)
}
```

  and call `offerIndexDoor()` in `main.go` right after `destinationDoor()` (line 1983). Check
  `main_quests.go` imports `dialogue`; add it if not.
- [ ] **Step 6:** `go test ./internal/dialogue/ -run TestOfferIndex` PASS; `go build ./...`;
  `DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke_ServerBootsCleanWithRealData` PASS. Add a smoke
  assertion in the same test file is not needed; Task 6 tests the shipped index through quest 28.
- [ ] **Step 7:** Commit (`feat(dialogue): index of which NPCs offer each quest (#375 p5)`).

### Task 3: Offer effects on a resume

**Files:** `internal/dialogue/types.go`, `internal/dialogue/topics.go`, create
`internal/dialogue/resume_test.go`; modify `internal/conversation/playerstate.go`

- [ ] **Step 1: Failing test** (`resume_test.go`). Build a `PlayerState` with counters, the way
  existing `Apply` tests do (read one in `internal/dialogue/*_test.go` for the shape first):

```go
package dialogue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type resumeProbe struct {
	charged, given, granted int
	flags                   map[string]string
	carries                 bool
}

func (p *resumeProbe) state(shelved bool) *PlayerState {
	return &PlayerState{
		Shelved:     func(int) bool { return shelved },
		CarriesItem: func(int) bool { return p.carries },
		ChargeGold:  func(n int) bool { p.charged += n; return true },
		GiveGold:    func(int) {},
		GiveItem:    func(int) bool { p.given++; return true },
		GiveQuest:   func(string) { p.granted++ },
		SetQuestFlag: func(k, v string) {
			if p.flags == nil {
				p.flags = map[string]string{}
			}
			p.flags[k] = v
		},
	}
}

// Owner ruling 2: a resume hands over only an item worth nothing that the
// player lacks, and charges nothing; the grant and the flag still apply.
func TestApply_ResumeEffects(t *testing.T) {
	t.Cleanup(SetItemValueForTest(map[int]int{30: 0, 10038: 40}))
	df := &DialogueFile{MobId: 1}
	notice := &Topic{Key: "o", Kind: KindOffer, Effects: Effects{GrantsQuest: "10-start", GivesItem: 30,
		SetsQuestFlag: &QuestFlagSet{Key: "10-x", Value: "y"}, ChargesGold: 5}, Gates: Gates{GoldRequired: 5}}
	sling := &Topic{Key: "s", Kind: KindOffer, Effects: Effects{GrantsQuest: "50-start", GivesItem: 10038}}

	p := &resumeProbe{}
	require.True(t, df.Apply(notice, p.state(true)))
	require.Equal(t, 0, p.charged)
	require.Equal(t, 1, p.given, "a worthless item the player lacks comes back")
	require.Equal(t, 1, p.granted)
	require.Equal(t, "y", p.flags["10-x"])

	p = &resumeProbe{carries: true}
	require.True(t, df.Apply(notice, p.state(true)))
	require.Equal(t, 0, p.given, "already carried")

	p = &resumeProbe{}
	require.True(t, df.Apply(sling, p.state(true)))
	require.Equal(t, 0, p.given, "starter gear does not come back")
	require.Equal(t, 1, p.granted)

	p = &resumeProbe{}
	require.True(t, df.Apply(sling, p.state(false)))
	require.Equal(t, 1, p.given, "a first accept is unchanged")
}
```

  Adjust the `PlayerState` field names (`ChargeGold`, `GiveGold`, ...) to the real ones in
  `types.go` and the charge path in `tryCharge`; the assertions stay.

- [ ] **Step 2:** `go test ./internal/dialogue/ -run TestApply_ResumeEffects` : FAIL (undefined `Shelved`, `SetItemValueForTest`).
- [ ] **Step 3: Implement.** In `types.go` `PlayerState`, add:

```go
	// Shelved reports whether the player set this quest aside (#375 phase 5).
	// Nil means never.
	Shelved func(questId int) bool
```

  In `topics.go`, a seam and the resume rule:

```go
// itemValue is the item's sale value, a seam for tests.
var itemValue = func(itemId int) int {
	if s := items.GetItemSpec(itemId); s != nil {
		return s.Value
	}
	return 0
}

// SetItemValueForTest replaces itemValue and returns a restore func.
func SetItemValueForTest(values map[int]int) func() {
	prev := itemValue
	itemValue = func(id int) int { return values[id] }
	return func() { itemValue = prev }
}

// resuming reports whether choosing t takes a shelved quest back up.
func resuming(t *Topic, ps *PlayerState) bool {
	if ps == nil || ps.Shelved == nil {
		return false
	}
	qid, step := quests.TokenToParts(t.GrantsQuest)
	return step == `start` && ps.Shelved(qid)
}

// resumeGivesItem is owner ruling 2: on a resume, only an item worth nothing
// that the player does not carry is handed over again.
func resumeGivesItem(itemId int, ps *PlayerState) bool {
	return itemId > 0 && itemValue(itemId) == 0 && !carriesItemPasses(itemId, ps)
}
```

  and in `Apply`:

```go
	charge, give := t.ChargesGold, t.GivesItem
	if resuming(t, ps) {
		charge = 0
		if !resumeGivesItem(give, ps) {
			give = 0
		}
	}
	if !tryCharge(charge, ps) {
		return false
	}
	if !applyPaidEffects(charge, t.GrantsQuest, t.RequiresItem, give,
		t.SetsQuestFlag, t.BumpsRep, t.GivesGold, ps) {
		return false
	}
```

  Check `topics.go` imports `items` and `quests` (dialogue already imports both packages).
  `carriesItemPasses(id, ps)` returns true when the item is carried; read it (`topic.go:38`) to
  confirm the polarity, since a nil callback returns false there.

- [ ] **Step 4:** In `conversation/playerstate.go`, add to the built `PlayerState`:

```go
		Shelved: func(questId int) bool { return user.Character.IsShelved(questId) },
```

- [ ] **Step 5:** Tests PASS; `go test ./internal/dialogue/ ./internal/conversation/` PASS.
- [ ] **Step 6:** Commit (`feat(dialogue): a resumed offer hands over nothing of value (#375 p5)`).

### Task 4: Resumed grants in `GameBridge` and the hook

**Files:** `internal/questengine/bridge.go`, create `internal/questengine/bridge_resume_test.go`;
`internal/hooks/Quest_HandleQuestUpdate.go`, create `internal/hooks/Quest_Resume_test.go`

- [ ] **Step 1: Failing bridge test** (`bridge_resume_test.go`), on the fixture pattern of
  `bridge_giveitem_test.go`:

```go
package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const (
	resumeShotId   = 91201 // valued, like the Pouch of Shot
	resumeNoticeId = 91202 // worth nothing, like the Protection Notice
	resumeUserId   = 9121
)

func resumeFixture(t *testing.T) *users.UserRecord {
	t.Helper()
	t.Cleanup(quests.SeedQuestsForTest(&quests.Quest{QuestId: 950, Name: "First Shot",
		Steps: []quests.QuestStep{{Id: "start"}, {Id: "shoot"}, {Id: "end"}}}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		resumeShotId:   {ItemId: resumeShotId, Name: "pouch of shot", Type: items.Object, Subtype: items.Mundane, Value: 40},
		resumeNoticeId: {ItemId: resumeNoticeId, Name: "notice", Type: items.Object, Subtype: items.Mundane},
	}))
	u := users.NewTestUser(resumeUserId, "resume", "Resume", uint64(resumeUserId))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{resumeUserId: u}))
	events.DrainQueuedMessagesForTest(resumeUserId)
	events.DrainQueuedItemOwnershipForTest(resumeUserId)
	t.Cleanup(func() {
		events.DrainQueuedMessagesForTest(resumeUserId)
		events.DrainQueuedItemOwnershipForTest(resumeUserId)
	})
	return u
}

func holds(u *users.UserRecord, itemId int) bool {
	for _, itm := range u.Character.GetAllCarriedItems() {
		if itm.ItemId == itemId {
			return true
		}
	}
	return false
}

// A grant chain for a resumed quest hands over no item of value and no gold
// (owner ruling 2, spec section 1); a worthless missing item still comes.
func TestGameBridge_ResumedHandsOverNothingOfValue(t *testing.T) {
	u := resumeFixture(t)
	u.Character.QuestProgress = map[int]string{950: "shoot"}
	require.True(t, u.Character.ShelveQuest(950))
	gold := u.Character.Gold

	b := NewGameBridge(u, 1)
	b.GrantQuest("950-start")
	require.Equal(t, "shoot", u.Character.GetQuestProgress()[950])
	require.True(t, b.Resumed())

	require.NoError(t, b.GiveItem(resumeShotId))
	require.False(t, holds(u, resumeShotId), "a valued item does not come back")
	require.NoError(t, b.GiveItem(resumeNoticeId))
	require.True(t, holds(u, resumeNoticeId), "a worthless missing item does")
	b.GiveGold(10)
	require.Equal(t, gold, u.Character.Gold, "no gold on a resume")

	first := NewGameBridge(u, 1)
	require.False(t, first.Resumed())
	require.NoError(t, first.GiveItem(resumeShotId))
	require.True(t, holds(u, resumeShotId), "outside a resume, unchanged")
}
```

  If `items.ItemSpec` has no `Value` field set this way, or `Gold` is not a plain int, read the
  structs and adjust the literal; keep the assertions.

- [ ] **Step 2:** Run: FAIL.
- [ ] **Step 3: Implement in `bridge.go`.**

```go
type GameBridge struct {
	user    *users.UserRecord
	roomId  int
	resumed bool // this chain takes a shelved quest back up (#375 phase 5)
}

// SetResumed marks the bridge's chain as a resume: GiveItem and GiveGold then
// follow owner ruling 2 (nothing of value is handed over).
func (b *GameBridge) SetResumed(v bool) { b.resumed = v }

// Resumed reports whether the bridge's chain is a resume.
func (b *GameBridge) Resumed() bool { return b.resumed }
```

  In `GiveItem`, first lines:

```go
	if b.resumed && !resumeMayGive(b.user, itemId) {
		return nil // a resume hands over nothing of value; the first accept did
	}
```

  with

```go
// resumeMayGive is owner ruling 2 for trigger handouts: an item worth nothing
// that the player does not carry or wear (the same "anywhere on the player"
// the dialogue CarriesItem callback checks, conversation/playerstate.go).
func resumeMayGive(user *users.UserRecord, itemId int) bool {
	spec := items.GetItemSpec(itemId)
	if spec == nil || spec.Value > 0 {
		return false
	}
	for _, itm := range user.Character.GetAllCarriedItems() {
		if itm.ItemId == itemId {
			return false
		}
	}
	for _, itm := range user.Character.GetAllWornItems() {
		if itm.ItemId == itemId {
			return false
		}
	}
	return true
}
```

  In `GiveGold`, first line: `if b.resumed { return }`.

  In `GrantQuest`, after the successful `GiveQuestToken`:

```go
	_, stepName := quests.TokenToParts(token)
	questId, _ := quests.TokenToParts(token)
	resumed := stepName == "start" && b.user.Character.TakeResumed(questId)
	if resumed {
		b.resumed = true
	}
```

  and the start banner becomes:

```go
	if stepName == "start" && resumed {
		bannerMsg = fmt.Sprintf("You take up <ansi fg=\"questname\">%s</ansi> again.", questName)
	} else if stepName == "start" {
		bannerMsg = fmt.Sprintf("You have been given a new quest: <ansi fg=\"questname\">%s</ansi>!", questName)
	} else {
```

  (Merge with the existing `stepName` declaration; do not declare it twice.)

- [ ] **Step 4: Hook.** In `Quest_HandleQuestUpdate.go`, after `freshlyGranted := ...GiveQuestToken(...)`:

```go
	questId, stepName := quests.TokenToParts(evt.QuestToken)
	resumed := stepName == `start` && questUser.Character.TakeResumed(questId)
```

  Where it builds `qbridge` for the `quest_granted` notify, add `qbridge.SetResumed(resumed)`
  before `Notify`. In the `start` banner block, print
  `You take up <ansi fg="questname">%s</ansi> again.` (event log: `Took up a quest again: ...`)
  when `resumed`, else the existing new-quest lines. Reuse the existing `stepName` variable
  further down rather than redeclaring it.
- [ ] **Step 5: Hook test** (`Quest_Resume_test.go`), modelled on
  `Quest_HandleQuestUpdate_bridge_test.go` (read it first): a user with quest 50 shelved at
  `shoot`; `HandleQuestUpdate(events.Quest{UserId, QuestToken: "50-start"})`; assert progress is
  `shoot`, the shelf is empty, and a `quest_granted` trigger on `50-start` with
  `give_item: 30064` did not give the pouch; with the quest never taken, the same event gives it.
- [ ] **Step 6:** Run hook and questengine tests: PASS. Commit
  (`feat(quests): a resumed grant says so and hands over nothing of value (#375 p5)`).

### Task 5: `AbandonQuest` and `FocusQuest`

**Files:** create `internal/usercommands/questlog.go`, `internal/usercommands/questlog_test.go`

- [ ] **Step 1: Failing tests.**

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func questlogFixture(t *testing.T) *users.UserRecord {
	t.Helper()
	t.Cleanup(quests.SeedQuestsForTest(
		&quests.Quest{QuestId: 42, Name: "The Hunt", Steps: []quests.QuestStep{{Id: "start"}, {Id: "track"}, {Id: "end"}}},
		&quests.Quest{QuestId: 28, Name: "Waking to Gaius", Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
		&quests.Quest{QuestId: 85, Name: "Secret", Secret: true, Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
	))
	t.Cleanup(dialogue.SetOfferIndexForTest(map[int][]int{42: {9137}, 85: {9584}}))
	t.Cleanup(setGiverNameForTest(map[int]string{9137: "Hunter Delk"}))
	u := users.NewTestUser(97101, "ql", "Questlog", 97101)
	u.Character.QuestProgress = map[int]string{42: "track", 28: "start", 85: "start"}
	return u
}

func TestAbandonQuest(t *testing.T) {
	u := questlogFixture(t)

	ok, line := AbandonQuest(u, 28)
	require.False(t, ok)
	require.Equal(t, "Nobody would take this back from you. It has to be finished.", line)

	ok, _ = AbandonQuest(u, 85)
	require.False(t, ok, "a secret quest cannot be abandoned")

	ok, line = AbandonQuest(u, 42)
	require.True(t, ok)
	require.Equal(t, "You set The Hunt aside. Talk to Hunter Delk to take it up again. You will pick up where you left off.", line)
	require.True(t, u.Character.IsShelved(42))

	ok, line = AbandonQuest(u, 42)
	require.False(t, ok)
	require.Equal(t, "You have no quest called that.", line)
}

func TestFindActiveQuest(t *testing.T) {
	u := questlogFixture(t)
	require.Equal(t, 42, findActiveQuest(u, "hunt"))
	require.Equal(t, 42, findActiveQuest(u, "The Hunt"))
	require.Equal(t, 0, findActiveQuest(u, "secret"), "secret quests are never matched")
	require.Equal(t, 0, findActiveQuest(u, "dragon"))
}

func TestFocusQuest(t *testing.T) {
	u := questlogFixture(t)
	require.True(t, FocusQuest(u, 42))
	require.Equal(t, 42, u.Character.LastQuestId)
	require.False(t, FocusQuest(u, 85))
	require.False(t, FocusQuest(u, 7))
}
```

- [ ] **Step 2:** Run: FAIL (undefined).
- [ ] **Step 3: Implement `questlog.go`.**

```go
package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Quest log actions (#375 phase 5, spec sections 2 and 4).
const (
	questNoSuchLine     = `You have no quest called that.`
	questNoWayBackLine  = `Nobody would take this back from you. It has to be finished.`
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

// QuestGiver is the first NPC that offers questId, or "".
func QuestGiver(questId int) string {
	if ids := dialogue.Offerers(questId); len(ids) > 0 {
		return giverName(ids[0])
	}
	return ``
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
// unfinished quests: exact name first, then a close match. 0 when none.
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
	match, close := util.FindMatchIn(strings.TrimSpace(name), names...)
	if match != `` {
		return ids[match]
	}
	return ids[close]
}

// AbandonQuest sets questId aside and returns the line to show the player.
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
	conversation.RefreshMarkers(user.UserId)
	events.AddToQueue(events.Quest{UserId: user.UserId, QuestToken: quests.PartsToToken(questId, step), MarkerOnly: true})
	return true, fmt.Sprintf(`You set %s aside. Talk to %s to take it up again. You will pick up where you left off.`,
		q.Name, QuestGiver(questId))
}

// FocusQuest makes questId the quest the map, hint and panel follow.
func FocusQuest(user *users.UserRecord, questId int) bool {
	step, ok := user.Character.GetQuestProgress()[questId]
	if !ok || step == `end` || characters.IsSecretQuest(questId) {
		return false
	}
	user.Character.LastQuestId = questId
	return true
}
```

  Check against source before keeping: `util.FindMatchIn` semantics (read `util.go:370`); how
  the existing Focus handler re-sends `Char.Quests` (it queues `GMCPCharUpdate`, a `modules/gmcp`
  type usercommands cannot import). Use the same thing the MarkerOnly `events.Quest` path does to
  refresh the panel (read `gmcp.Char.go:140` handler for `events.Quest`), or leave the refresh to
  the callers (the GMCP handler re-sends itself; the command path can rely on the MarkerOnly
  event). Remove the `events.AddToQueue` line if the MarkerOnly event does not re-send
  `Char.Quests` for a quest no longer held, and test the refresh in Task 7 instead.
  If `GiverName` is empty (an offer whose NPC has no spec), the line reads "Talk to  to"; guard:
  fall back to `the one who gave it to you`.

- [ ] **Step 4:** Run: PASS. Commit (`feat(usercommands): abandon and focus a quest (#375 p5)`).

### Task 6: The `quests` command

**Files:** `internal/usercommands/quests.go`, `_datafiles/world/dogmud/templates/help/quests.template`,
`internal/usercommands/questlog_test.go`

- [ ] **Step 1: Failing tests** added to `questlog_test.go`:

```go
func TestQuestsCommand_AbandonAndFocusWords(t *testing.T) {
	require.Equal(t, [2]string{"abandon", "the hunt"}, questsSubcommand("abandon the hunt"))
	require.Equal(t, [2]string{"focus", "hunt"}, questsSubcommand("focus hunt"))
	require.Equal(t, [2]string{"", "all"}, questsSubcommand("all"))
}

func TestShelvedRows(t *testing.T) {
	u := questlogFixture(t)
	_, _ = AbandonQuest(u, 42)
	rows := shelvedRows(u.Character)
	require.Len(t, rows, 1)
	require.Equal(t, "The Hunt", rows[0].Name)
	require.Equal(t, "Set aside. Talk to Hunter Delk to take it up again.", rows[0].Description)
}

// On shipped data, quest 28 (started by walking into a room) has no offer.
func TestShippedOfferIndex_Quest28HasNoOffer(t *testing.T) {
	chdirRepoRootForTest(t) // use the package's existing helper if it has one; else copy the quests one
	files, _ := dialogue.LoadAllFiles()
	t.Cleanup(dialogue.SetOfferIndexForTest(nil))
	dialogue.BuildOfferIndex(files)
	require.Empty(t, dialogue.Offerers(28))
	require.NotEmpty(t, dialogue.Offerers(42))
}
```

- [ ] **Step 2:** Run: FAIL.
- [ ] **Step 3: Implement** in `quests.go`:

```go
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

// shelvedRows are the `quests all` rows for quests set aside.
func shelvedRows(c *characters.Character) []questRecord {
	var rows []questRecord
	for questId := range c.ShelvedQuests {
		q := quests.GetQuestById(questId)
		if q == nil || q.Secret {
			continue
		}
		giver := QuestGiver(questId)
		if giver == `` {
			giver = `the one who gave it to you`
		}
		rows = append(rows, questRecord{Id: q.QuestId, Name: q.Name,
			Description: fmt.Sprintf(`Set aside. Talk to %s to take it up again.`, giver)})
	}
	return rows
}
```

  At the top of `Quests(...)`:

```go
	switch sub := questsSubcommand(rest); sub[0] {
	case `abandon`, `focus`:
		id := findActiveQuest(user, sub[1])
		if id == 0 {
			user.SendText(messaging.CategoryRefusal, questNoSuchLine)
			return true, nil
		}
		if sub[0] == `focus` {
			FocusQuest(user, id)
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`You turn your attention to %s.`, quests.GetQuestById(id).Name))
			return true, nil
		}
		ok, line := AbandonQuest(user, id)
		cat := messaging.CategorySystem
		if !ok {
			cat = messaging.CategoryRefusal
		}
		user.SendText(cat, line)
		return true, nil
	}
```

  and where `finished` rows are appended under `showComplete`, also append `shelvedRows(...)`
  and add their count to `qInfo.QuestsTotal`. Check `character/quests.template` renders a row
  with an empty `BarFull` and `Completion` without a stray `%`; if it does not, add a template
  branch on `.Completion` being empty.

  The focus line goes through `dogmud-player-copy` (no raw numbers, 80 columns).

- [ ] **Step 4: Help.** In `templates/help/quests.template`, after the `quests all` line, add two
  lines in the template's own markup (read it first):
  `quests focus <name>`: "Follow that quest: the map and the hint point to it." and
  `quests abandon <name>`: "Set the quest aside. Talk to whoever gave it to you to take it up again, right where you left off."
- [ ] **Step 5:** Tests PASS; `go test ./internal/usercommands/` PASS. Commit
  (`feat(usercommands): quests focus, quests abandon, set-aside rows (#375 p5)`).

### Task 7: GMCP

**Files:** `modules/gmcp/gmcp.Char.go`, `modules/gmcp/gmcp.CharOp.go`, their tests

- [ ] **Step 1: Failing tests.** Read the existing `Char.Quests` payload tests in `modules/gmcp`
  (grep `Payload_Quest` in `*_test.go`) and add: a quest with an offer has `Abandonable: true`;
  quest 28 (no offer) and a finished one have false. Read the Focus handler test (grep
  `Char.Quests.Focus` in tests) and add the same for `Char.Quests.Abandon`: an abandonable id is
  shelved; quest 28's id changes nothing.
- [ ] **Step 2:** Run: FAIL.
- [ ] **Step 3: Implement.** Payload field after `Focused`:

```go
	// Abandonable: the player may set this quest aside (#375 phase 5): active,
	// not secret, offered by some NPC.
	Abandonable bool `json:"abandonable,omitempty"`
```

  set in the builder with `Abandonable: usercommands.Abandonable(user.Character, questId)`
  (check that `modules/gmcp` may import `usercommands` without a cycle: `go list -deps`; if it
  cycles, move `Abandonable`, `QuestGiver` and the offer check into `internal/characters` or a
  small `internal/questlog` package and call it from both).

  In `gmcp.CharOp.go`, replace the Focus handler's checks with `usercommands.FocusQuest(u,
  req.Id)` (same re-send of `Char.Quests` on success), and add next to it:

```go
	case `Char.Quests.Abandon`:
		var req struct {
			Id int `json:"id"`
		}
		if err := json.Unmarshal(evt.Payload, &req); err != nil {
			break
		}
		uid := userIdForConnection(evt.ConnectionId)
		if uid <= 0 {
			break
		}
		u := users.GetByUserId(uid)
		if u == nil {
			break
		}
		ok, line := usercommands.AbandonQuest(u, req.Id)
		cat := messaging.CategorySystem
		if !ok {
			cat = messaging.CategoryRefusal
		}
		u.SendText(cat, line)
		events.AddToQueue(GMCPCharUpdate{UserId: uid, Identifier: `Char.Quests`})
```

- [ ] **Step 4:** Tests PASS; `go test ./modules/gmcp/` PASS. Commit
  (`feat(gmcp): abandonable flag and Char.Quests.Abandon (#375 p5)`).

### Task 8: Web quest panel menu

**Files:** `_datafiles/html/public/webclient-pure.html`, `_datafiles/html/public/static/css/dashboard.css`,
create `tools/webclient-tests/quest-panel-menu.js`

- [ ] **Step 1: Failing node check.** Copy the harness of
  `tools/webclient-tests/quest-panel-refresh.js` (its DOM stub and how it loads the page's
  script). The check: render quests `[{id:42,name:"The Hunt",abandonable:true,focused:true},
  {id:28,name:"Waking to Gaius"}]`; dispatch a `contextmenu` on the 42 row; the menu has rows
  "Focus" and "Abandon", neither disabled; clicking "Abandon" sends `SendGMCP("Char.Quests.Abandon",
  {id:42})` (record calls); on the 28 row, "Abandon" carries class `ctx-disabled` and a `title`
  equal to "Nobody would take this back from you. It has to be finished.", and clicking it sends
  nothing; "Focus" sends `Char.Quests.Focus`. A `.qrepeatrow` gets no menu.
- [ ] **Step 2:** `node tools/webclient-tests/quest-panel-menu.js` : FAIL.
- [ ] **Step 3: Implement.** In the quest row markup (`:1018`) add
  `data-abandonable="' + (q.abandonable ? 1 : 0) + '"`. After the automation context menu block,
  add a quest context menu IIFE copying its structure (`buildMenu` with class `quest-ctx`,
  `closeMenu`, `closeOnOutside`, `closeOnEscape`, `makeCtxRow`), with:

```js
            function openMenu(e, row) {
                var id = parseInt(row.getAttribute("data-quest-id"), 10);
                if (isNaN(id)) return;
                if (!menu) menu = buildMenu();
                while (menu.firstChild) menu.removeChild(menu.firstChild);
                var hdr = document.createElement("div");
                hdr.className = "ctx-header";
                hdr.textContent = row.querySelector(".qname") ? row.querySelector(".qname").textContent : "";
                menu.appendChild(hdr);
                menu.appendChild(makeCtxRow("Focus", true, function () {
                    SendGMCP("Char.Quests.Focus", { id: id });
                }));
                var sep = document.createElement("div");
                sep.className = "ctx-sep";
                menu.appendChild(sep);
                var canDrop = row.getAttribute("data-abandonable") === "1";
                var drop = makeCtxRow("Abandon", false, function () {
                    if (canDrop) SendGMCP("Char.Quests.Abandon", { id: id });
                });
                if (!canDrop) {
                    drop.className += " ctx-disabled";
                    drop.title = "Nobody would take this back from you. It has to be finished.";
                }
                menu.appendChild(drop);
                // position and close listeners exactly as the automation menu does
            }
            document.addEventListener("contextmenu", function (e) {
                var row = e.target.closest(".qrow[data-quest-id]");
                if (!row) return;
                e.preventDefault();
                openMenu(e, row);
            });
```

  CSS in `dashboard.css`, after the `.auto-ctx` block: a `.quest-ctx` block mirroring `.auto-ctx`
  (same rules for `.ctx-header`, `.ctx-row`, `.ctx-sep`), plus

```css
.quest-ctx .ctx-row.ctx-disabled,
.quest-ctx .ctx-row.ctx-disabled:hover {
  color: var(--text-secondary, #9a8a6a);
  background: none;
  font-style: italic;
  cursor: default;
}
```

  (dimmed and italic: shape and brightness, no hue carries the meaning).

- [ ] **Step 4:** Node check PASS; every `node tools/webclient-tests/*.js` PASS.
- [ ] **Step 5:** Commit (`feat(webclient): right-click Focus and Abandon on quest rows (#375 p5)`).

### Task 9: Screenshot from the real web client

- [ ] Boot a local server from a detached worktree at the branch head on private ports with a
  `Playtest.ProfilesManifest` (two `early` characters: one with `set_quest_tokens:
  ["42-start","42-track"]` at Hunter's Hollow 5290, one with `["28-start"]`), per
  `tools/webshot/README.md`. A `shots.json` session per character that right-clicks its quest row
  (an `eval` step dispatching a `contextmenu` MouseEvent at the row's centre), shoots
  `p4s5-menu-hunt.png` and `p4s5-menu-tutorial.png` into the main checkout root, then clicks
  Abandon on The Hunt (`eval` clicking the row), `tail 6`, and shoots `p4s5-abandoned.png`.
  Read each PNG; the disabled Abandon must be visibly dimmed. Stop the server by PID; check
  no Edge is left by command line; remove the boot worktree.

### Task 10: Playtest

- [ ] Scenario `tools/playtest/scenarios/375-p5-quest-log.yaml` with two seats (personality
  `bug-finder`), goals under `tools/playtest/goals/scenarios/375-p5-quest-log/`:
  - **hunt** (`early`, start 5290 Hunter's Hollow, `set_quest_tokens: ["42-start","42-track"]`,
    `grant_items: [40017]`): `quests`; `quests abandon hunt` (quote); `quests` and `quests all`
    (quote the set-aside row); `look` and say whether Hunter Delk shows `[!]`; walk north to the
    Thornbrake and kill a pronghorn (`attack pronghorn`), then `quests` (it must not reappear);
    walk back, `talk delk`, take the hunt again (quote "You take up The Hunt again."); check the
    inventory holds one Salt Pouch, not two; kill another pronghorn and quote the progress line.
  - **tutor** (`early`, start 5234 Squatter's Hollow, `set_quest_tokens: ["28-start","33-start"]`):
    `quests abandon waking` (quote the refusal); `quests focus wash` and `quests` (the focused
    quest's guide line); `quests focus waking`.
- [ ] Run per `dogmud-playtesting` (each seat's helper script named after its actor id; the
  container removed with `docker rm -f` after `stop`). File findings (`--repo pruuk/DOGMud`,
  labels `bug`, `from-playtest`, an `area:*`, and one of `ready` / `needs-design` /
  `owner-decision`). Fix defects in this branch's own code in the branch.

### Task 11: Docs

- [ ] `context.md` for `internal/characters` (the shelf methods), `internal/dialogue` (offer
  index, `Shelved`, resume effects), `internal/conversation` (`Shelved` callback),
  `internal/questengine` (`GameBridge.SetResumed`, resume banner), `internal/hooks` (resume
  banner), `internal/usercommands` (`questlog.go`), `modules/gmcp` (`abandonable`,
  `Char.Quests.Abandon`). Every symbol named must exist (`tools/context_md_audit.py`).
- [ ] Roadmap phase 5 **Status**: in PR, spec and plan paths.
- [ ] `dogmud-authoring-quests` skill: a section "Abandon and resume" with the four rules (shelved
  quests are not held; a resume hands over nothing of value from any source, so an item only an
  NPC hands out for a quest is authored worth nothing; abandon needs an offer; a pick made in the
  offer can change through abandon, so a pick that must stick belongs mid-quest).
- [ ] `docs/PATCH_NOTES.md` dated entry: abandon and focus, from the quest log and the web panel,
  in player copy.
- [ ] Commit the docs.

### Task 12: Gate and PR

- [ ] Gate per `dogmud-shipping`: `go build ./...`, `go vet ./...`, `go test ./...`,
  `golangci-lint run --new-from-merge-base=origin/master`, `DOGMUD_BOOT_SMOKE=1 go test . -run
  TestSmoke`, every `node tools/webclient-tests/*.js`, and the boot check. Paste each tail.
- [ ] `gh pr create --repo pruuk/DOGMud`: summary; the rulings; tests; screenshots by file name;
  playtest; gate; deploy notes (a new saved field `shelvedquests` on characters, omitted when
  empty, so old saves load unchanged; no config change). End with the generated-with line. Do not
  write "closes #375".
- [ ] Watch CI; merge only on the owner's go.

---

## Self-review

- Spec section 1 (shelf, resume through every grant path, banner, triggers still fire, handouts):
  Tasks 1 and 4. Section 2 (abandon, refusals, lines, offer index): Tasks 2 and 5. Section 3
  (offer effects): Task 3. Section 4 (focus): Tasks 5 and 7. Section 5 (`quests all`, help):
  Task 6. Section 6 (GMCP, web): Tasks 7 and 8. Section 7 (testing): each task plus Tasks 9 and
  10. Section 8 (docs): Task 11.
- Behaviour-tree and overlay grants resume through `GiveQuestToken` (Task 1) with no further
  change; the overlay path is exercised by the playtest seats' `set_quest_tokens`.
- Points the implementer must read before coding are named in place (the `PlayerState` field
  names, the items test seam, the carry helper, `FindMatchIn`, the import cycle check, the
  template's empty-bar rendering); each says what to do with either answer.
