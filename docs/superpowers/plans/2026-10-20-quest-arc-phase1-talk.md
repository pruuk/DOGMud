# Quest Arc Phase 1: One `talk` Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `talk <npc>` is the only way to converse: every one of the 302 dialogue files is converted once into keyed, labelled `info` / `offer` / `handin` topics served through the phase 0 menu, with a saved heard set, topic-only item hand-ins, behavior-tree topics, per-viewer markers and proposed-world validators.

**Spec:** `docs/superpowers/specs/2026-10-20-quest-arc-phase1-talk-design.md` (rulings R1 to R13). Read it first; this plan does not restate the reasons.

**Architecture:** `internal/dialogue` stays pure (no `users`/`characters` import) and gains the topic model, eligibility, markers, opener selection, menu building, effect application and the `World` validator. A new package `internal/conversation` is the glue the commands call: it builds `PlayerState`, runs the talk flow through `users.OpenMenu`, resolves `give`, answers the companion asker and supplies the roster marker through a hook in `internal/rooms`. Stage A builds all of it against fixtures while the shipped files still run on the old engine; stage B converts the content on the branch; stage C switches the commands, deletes the old engine and lands everything in one merge.

**Tech Stack:** Go 1.x, `gopkg.in/yaml.v2`, the repo's `testify` (`assert`, `require`), existing test seams (`users.NewTestUser`, `users.SeedUsersForTest`, `mobs.SeedMobsForTest`, `items.SeedItemsForTest`, `events.DrainQueuedMessagesForTest`).

---

## Rules for every task (subagents: read this first)

- **Edit files with the Edit and Write tools only.** No `sed -i`, no `>`/`>>` into tracked files, no Python read-modify-write. Generated output goes to the session scratchpad and is copied into place with `cp`. The global hook `guard.py` blocks the rest.
- **`git add <named paths>` only.** Never `-A`, `.`, `--all`, `commit -a`. Never `--no-verify`.
- **Commit messages through a quoted heredoc** (`git commit -F - <<'EOF'`), ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- **No em or en dashes** in any text you write (code comments, player text, docs).
- **Balance numbers come from `config.yaml`;** this plan adds none.
- **Test binaries load Go config defaults,** not `_datafiles/config.yaml` (`dogmud-writing-tests`).
- **Run tests with** `go test ./internal/<pkg>/ -run <Name> -count=1`. The race detector does not run on this machine (no gcc).
- **Batch readers** (zone passes): print files in chunks of at most 400 lines and report "read N of N files" with the list.
- **The owner's server may be running.** Test servers use private ports and are killed by PID only.

## Facts verified against source (master `243b7fb39` plus spec commits, 2026-10-20)

| Fact | Value | Source |
|---|---|---|
| Old gate helper | `checkQuestGate(questRequired, questExcluded []string, requiresItem int, flagRequired, flagExcluded map[string]string, masterworkRequired, goldRequired int, ps *PlayerState) bool` | `internal/dialogue/engine.go:31` |
| Item-excluded helper | `itemExcludedBlocks(itemId int, ps *PlayerState) bool` | `engine.go:94` |
| Charge / effects | `tryCharge(amount int, ps) bool`; `applyPaidEffects(charged int, grantsQuest string, requiresItem, givesItem int, flagSet *QuestFlagSet, bumpsRep []RepBump, givesGold int, ps) bool` | `engine.go:108,119` |
| Old entry points (deleted in C) | `Match`, `MatchWithFallbackInfo`, `TreeAdvance`, `Greet` | `engine.go:170,182,253,320` |
| Memory API | `PlayerMemory{VisitCount, LastVisitRound, UnlockedNodes, CurrentRootSeen, Greeted, RecentTopics}`; `SweepMemories`, `ForgetMobInstance`, `GetMemory`, `UpdateMemory`, `IsExpired`, `ResetMemory` | `internal/dialogue/memory.go` |
| Greetings kept | `PickGreeting`, `HasGreeted`, `MarkGreeted`; arrival hook | `greetings.go`; `internal/usercommands/go.go:552-567` |
| Loader | `Load(mobId int, zone string) *DialogueFile`, cache key `"%d:%s"`, `yaml.Unmarshal` (lenient) | `internal/dialogue/loader.go:44` |
| Old validator | `ValidateDialogueFile(df DialogueFile, v DialogueValidators) (errs, warns []string)`; `RegistryValidators()` | `validate.go:36`, `registry_validators.go` |
| Old validator rules kept | grant token in `questExcluded`; quest end token in `questExcluded`; tokens, flags, items exist; no `;` in spoken text | `validate.go:118-160` |
| Flag reference collector | `CollectFlagReferences(df *DialogueFile) (refs, sets []FlagRef)`, used by `questengine.ValidateAllFlags` | `validation.go`, `internal/questengine/loader.go:106` |
| Outside callers of `dialogue` | `usercommands/{ask,talk,go}.go`, `questengine/loader.go`, `hooks/NewTurn_SweepDialogueMemory.go`, `llm/client.go` (comment), `llm/types.go` (comment), `modules/gmcp/gmcp.{Dialogue,Quest_refs,Build,Mob}.go` | grep `dialogue\.[A-Z]` |
| `PlayerState` builder | `buildPlayerState(user *users.UserRecord) *dialogue.PlayerState` | `internal/usercommands/talk.go:150` |
| LLM-only helpers | `buildPlayerCondition`, `buildTutorialContext`, `buildQuestContext` | `talk.go:268,293,343` |
| `ask` chain | `askNpcChain(user, mob, mobId, room, rest, allowQuest) bool` fires quest `dialogue` event, then btree `player_ask`, then LLM, then `deliverDialogue` | `internal/usercommands/ask.go:177` |
| Companion routes | `companionai.RouteAsk(userId, mobInstanceId, text) bool` (`ask.go:142`); `companionai.SetNpcAsker(AskNpcForOwner)` (`ask.go:289`); `companionai.AskNpc(owner, mobInst, text, authorized)` called at `modules/aicompanion/actions.go:475` | |
| Companion authority | `askAuthority{MobInstanceId int; Topic string; Expires int64}`, `valid(mobInstanceId, text, now)`, set in `cmdAskFor` | `modules/aicompanion/commands.go:573,581,648` |
| `give` mob branch | sleep refusal `give.go:161`, quest `item_give` notify `give.go:200`, transfer `give.go:230`, btree `player_give` `give.go:282` | `internal/usercommands/give.go` |
| Give lines | `You give the <ansi fg="item">%s</ansi> to <ansi fg="mobname">%s</ansi>.` via `messaging.HideNames(..., []string{name}, giverSight)`; room `SendTextVisual(CategoryLoot, "<ansi fg=\"username\">%s</ansi> gave their <ansi fg=\"item\">%s</ansi> to <ansi fg=\"mobname\">%s</ansi>.", userId)` | `give.go:186-195` |
| Say | `actions.Say(actor actions.Actor, text string) SayResult` (room line, sight-aware, synchronous); player self line `You say, "<ansi fg="saytext">%s</ansi>"` | `internal/actions/say.go:27`, `usercommands/say.go:43` |
| Actors | `actions.UserActor{User, Room}`, `actions.MobActor{Mob, Room}` | `actions/actor_user.go:12`, `actor_mob.go:12` |
| Menu session | `users.(*UserRecord).OpenMenu(menu.Menu, menu.Session) (menu.CloseReason, bool)`; `choose` closes then calls `OnChoose`, so `OnChoose` may reopen | `internal/users/userrecord.menu.go:93,277` |
| Menu types | `Entry{Id, Marker, Label, Detail, Dim, Aliases}`, `Menu{Heading, Entries, Leave, Default, Footer, Columns, Prefix, AcceptText, LeaveAliases}`, `Confirm(question, yesLabel, noLabel) Menu` (ids `yes`/`no`, Default `no`), `LintLabels(Menu) []string`, `MarkerQuest`, `MarkerHandIn`, `MarkerInfo` | `internal/menu/*.go` |
| Session | `menu.Session{Topic string; TopicNames []string; OnChoose func(Entry); OnClose func(CloseReason); StillValid func() bool}` | `internal/menu/session.go` |
| Sight | `messaging.ParticipantSight(observer *characters.Character, room) SightDecision`; `messaging.SightFull`; `user.Character.Perceives(&mob.Character)` | `messaging/predicates.go:56` |
| Roster marker slot | `FormattedName.QuestAlert bool` (`formattedname.go:46,97`), set at `rooms/roomdetails.go:294-299`; cleared at `actions/perception.go:46`, `formattedname.go:178`, `hooks/spell_resolution.go:474`; roster replaced by figures at Shapes/None (`roomdetails.go:336-347`) | |
| GMCP NPC | `GMCPRoomModule_Payload_Contents_Character{Id, Name, Adjectives, Aggro, QuestFlag bool "quest_flag"}`, set at `gmcp.Room.go:399-404` | `modules/gmcp/gmcp.Room.go:608` |
| Dead mob star source | `Mob.QuestFlags` (`mobs.go:122`), `SpawnInfo.QuestFlags` (`spawninfo.go:29`), copy `rooms.go:1139-1140`; builder `mobs.js:696,753`, `build.html:923,966`, `admin/mobs/index.html:17`, `admin/mobs/mob.data.html:70-72`, `gmcp.Mob.go:266,390` | |
| Behavior trees | `EventContext{EventType, UserId, MobId, Text, ItemId, ItemUUID, RoomId, Extra}`; `TryMobBehavior(mobInstanceId int, event EventContext) bool`; events registry `KnownBehaviorEvents` (`events.go`); conditions `conditionRegistry["keyword_match"] = condKeywordMatch` (`conditions.go:12`), params via `getStringParam(params, key)`; `NodeDef{Type?, Event, Children, Check, Params map[string]any yaml:",inline"}`; `TreeDef` wraps `tree:`; `GetBehaviorPath(mobId, zone, name)` | `internal/behaviortree/*.go` |
| Character save | `QuestProgress map[int]string yaml:"questprogress"` `character.go:319`; `QuestFlags map[string]string` `:320`; load migrations at `internal/users/users.go:557-566` | |
| Boot order | `questengine.LoadDataFiles()` at `main.go:1892` (quests loaded before, it calls `ValidateAllFlags`) | |
| Smoke sweep | `TestSmoke_ShippedDialoguePassesTheEditorValidator` `boot_smoke_test.go:231` (env `bootSmokeEnvVar`), baseline `knownDialogueValidatorFindings` | |
| Editor | GMCP `Build.Dialogue.{Get,Update,Create,Delete}` in `gmcp.Build.go:398-425`, handlers in `gmcp.Dialogue.go`; panel `window.Builder.DialoguePanel` in `static/js/dialogue.js:564`, opened from `mobs.js:258-261` | |
| Quest `item_give` hand-ins | 33 triggers in 24 quest files, all with `grant` | content sweep |
| Quest `dialogue` trigger | 1: `quests/28-waking_to_gaius.yaml:422`, mob 9491, `has: [28-said]`, `missing: [28-asked]`, `grant: 28-asked` | |
| `player_ask` branches | 26 in 12 behavior files (list in spec 4.1) | content sweep |
| Help | `_datafiles/world/dogmud/templates/help/{talk,ask,quests}.template` | |

## File map

**Create**
- `internal/dialogue/topic.go`: `Gates`, `Effects`, `Topic`, `Opener`, `OpenerVariant`, kind constants, `HeardKey`.
- `internal/dialogue/topics.go`: `Eligible`, `TopicMarker`, `Marker`, `OpenerFor`, `Breadcrumbs`, `Menu`, `FindTopic`, `ItemTopic`, `Apply`.
- `internal/dialogue/world.go`: `World`, `QuestInfo`, `Problem`, `ValidateWorld`.
- `internal/dialogue/world_registry.go`: `RegistryWorld()`, `LoadAllFiles()`.
- `internal/dialogue/manifest.go`: topic-key manifest parse and `HeardRenames()`.
- `internal/dialogue/testing.go`: `InstallForTest(df *DialogueFile) func()`.
- `internal/characters/heard.go`: `HasHeard`, `MarkHeard`, `RenameHeardTopics`.
- `internal/behaviortree/topics.go`: `topic_is` condition, `TopicKeysInFile`.
- `internal/conversation/` (`context.md`, `playerstate.go`, `talk.go`, `give.go`, `marker.go`, `companion.go`, tests).
- `internal/rooms/npcmarker.go`: `NpcMarker` hook.
- `cmd/dialogueconvert/` (one-time; deleted in Task 24).
- `_datafiles/world/dogmud/dialogue/topic-keys.txt`.

**Modify (main ones)**: `internal/dialogue/{types,loader,walk,validation,memory,save,engine,validate,registry_validators,context.md}.go`, `internal/characters/{character,formattedname}.go`, `internal/actions/perception.go`, `internal/hooks/spell_resolution.go`, `internal/rooms/{roomdetails,rooms,spawninfo}.go`, `internal/mobs/mobs.go`, `internal/usercommands/{talk,ask,give}.go`, `internal/questengine/loader.go`, `internal/behaviortree/{events,conditions}.go`, `internal/users/users.go`, `modules/gmcp/gmcp.{Room,Dialogue,Build,Quest_refs,Mob}.go`, `modules/aicompanion/{commands,actions}.go`, `main.go`, `boot_smoke_test.go`, JS/HTML builder files, help templates, every dialogue file, 24 quest files, 12 behavior files.

---

# Stage A: engine on fixtures

Shipped content keeps running on the old engine during stage A. New code is reached only by tests until Task 19 switches the commands.

### Task 1: Topic types

**Files:**
- Create: `internal/dialogue/topic.go`
- Modify: `internal/dialogue/types.go` (add two fields to `DialogueFile`, two to `PlayerState`)
- Test: `internal/dialogue/topic_test.go`

- [ ] **Step 1: Write the failing test**

```go
package dialogue

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

const topicFixtureYAML = `mobid: 94
zone: thornwall_city
defaultMood: neutral
opener:
  text: "Watch business only."
  hints: "talk to the warden"
  variants:
    - questRequired: [10-start]
      text: "Any word on the racket?"
topics:
  - key: protection_report
    kind: info
    label: "I have proof of the protection racket."
    text: "Show me."
    marked: true
    heardRequired: [where_to_look]
    questRequired: [10-start]
    questExcluded: [10-report, 10-end]
    requiresItem: 30
    grantsQuest: 10-report
    moodChange: friendly
  - key: drowning_debt
    kind: offer
    quest: 10
    label: "The Drowning Post's Debt"
    playerLine: "Do you need help with the tavern's trouble?"
    text: "The keeper is paying someone. Find out who."
    breadcrumb: "Come back when the watch trusts you."
    questExcluded: [10-start, 10-end]
    grantsQuest: 10-start
`

func TestTopicFile_YAMLRoundTripKeepsInlineGatesAndEffects(t *testing.T) {
	var df DialogueFile
	require.NoError(t, yaml.UnmarshalStrict([]byte(topicFixtureYAML), &df))

	require.NotNil(t, df.Opener)
	require.Equal(t, "Watch business only.", df.Opener.Text)
	require.Len(t, df.Opener.Variants, 1)
	require.Equal(t, []string{"10-start"}, df.Opener.Variants[0].QuestRequired)

	require.Len(t, df.Topics, 2)
	info := df.Topics[0]
	require.Equal(t, KindInfo, info.Kind)
	require.Equal(t, 30, info.RequiresItem)
	require.Equal(t, "10-report", info.GrantsQuest)
	require.Equal(t, "friendly", info.MoodChange)
	require.Equal(t, []string{"where_to_look"}, info.HeardRequired)
	require.True(t, info.Marked)
	offer := df.Topics[1]
	require.Equal(t, KindOffer, offer.Kind)
	require.Equal(t, 10, offer.Quest)
	require.Equal(t, "Come back when the watch trusts you.", offer.Breadcrumb)

	out, err := yaml.Marshal(&df)
	require.NoError(t, err)
	var again DialogueFile
	require.NoError(t, yaml.UnmarshalStrict(out, &again))
	require.Equal(t, df.Topics, again.Topics)
	require.Equal(t, df.Opener, again.Opener)
}

func TestHeardKey(t *testing.T) {
	require.Equal(t, "94.where_to_look", HeardKey(94, "where_to_look"))
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/dialogue/ -run 'TestTopicFile|TestHeardKey' -count=1`
Expected: FAIL to compile (`undefined: KindInfo`, `df.Opener undefined`).

- [ ] **Step 3: Write `internal/dialogue/topic.go`**

```go
package dialogue

import "fmt"

// Topic kinds (quest arc #375 phase 1). Every topic runs one code path; the
// kind sets its marker and what choosing it does.
const (
	KindInfo   = "info"
	KindOffer  = "offer"
	KindHandIn = "handin"
)

// Gates is the gating family every topic and opener variant shares. The YAML
// names and the semantics are the ones patterns and tree nodes used.
type Gates struct {
	QuestRequired      []string          `yaml:"questRequired,omitempty" json:"questRequired,omitempty"`
	QuestExcluded      []string          `yaml:"questExcluded,omitempty" json:"questExcluded,omitempty"`
	QuestFlagRequired  map[string]string `yaml:"questFlagRequired,omitempty" json:"questFlagRequired,omitempty"`
	QuestFlagExcluded  map[string]string `yaml:"questFlagExcluded,omitempty" json:"questFlagExcluded,omitempty"`
	RequiresItem       int               `yaml:"requiresItem,omitempty" json:"requiresItem,omitempty"`
	ItemExcluded       int               `yaml:"itemExcluded,omitempty" json:"itemExcluded,omitempty"`
	GoldRequired       int               `yaml:"goldRequired,omitempty" json:"goldRequired,omitempty"`
	MasterworkRequired int               `yaml:"masterworkRequired,omitempty" json:"masterworkRequired,omitempty"`
}

// pass reports whether the player clears every gate. A nil PlayerState skips
// the checks, as everywhere in this package.
func (g Gates) pass(ps *PlayerState) bool {
	return checkQuestGate(g.QuestRequired, g.QuestExcluded, g.RequiresItem,
		g.QuestFlagRequired, g.QuestFlagExcluded, g.MasterworkRequired, g.GoldRequired, ps) &&
		!itemExcludedBlocks(g.ItemExcluded, ps)
}

// Effects is what choosing a topic changes. requiresItem (a gate) is also
// taken when the effects apply.
type Effects struct {
	GrantsQuest   string        `yaml:"grantsQuest,omitempty" json:"grantsQuest,omitempty"`
	GivesItem     int           `yaml:"givesItem,omitempty" json:"givesItem,omitempty"`
	SetsQuestFlag *QuestFlagSet `yaml:"setsQuestFlag,omitempty" json:"setsQuestFlag,omitempty"`
	BumpsRep      []RepBump     `yaml:"bumpsRep,omitempty" json:"bumpsRep,omitempty"`
	GivesGold     int           `yaml:"givesGold,omitempty" json:"givesGold,omitempty"`
	ChargesGold   int           `yaml:"chargesGold,omitempty" json:"chargesGold,omitempty"`
	MoodChange    string        `yaml:"moodChange,omitempty" json:"moodChange,omitempty"`
}

// Topic is one menu entry on an NPC. See the spec, section 3.2.
type Topic struct {
	Key           string   `yaml:"key" json:"key"`
	Kind          string   `yaml:"kind" json:"kind"`
	Quest         int      `yaml:"quest,omitempty" json:"quest,omitempty"`
	Label         string   `yaml:"label" json:"label"`
	PlayerLine    string   `yaml:"playerLine,omitempty" json:"playerLine,omitempty"`
	Text          string   `yaml:"text" json:"text"`
	Hints         string   `yaml:"hints,omitempty" json:"hints,omitempty"`
	Marked        bool     `yaml:"marked,omitempty" json:"marked,omitempty"`
	HeardRequired []string `yaml:"heardRequired,omitempty" json:"heardRequired,omitempty"`
	Breadcrumb    string   `yaml:"breadcrumb,omitempty" json:"breadcrumb,omitempty"`
	// Behavior fires the mob's behavior tree with event player_topic and
	// this topic's key once the topic has played (R12).
	Behavior bool `yaml:"behavior,omitempty" json:"behavior,omitempty"`
	Gates    `yaml:",inline"`
	Effects  `yaml:",inline"`
}

// OpenerVariant is a gated alternative opener. It carries no effects.
type OpenerVariant struct {
	Gates `yaml:",inline"`
	Text  string `yaml:"text" json:"text"`
	Hints string `yaml:"hints,omitempty" json:"hints,omitempty"`
}

// Opener is what the NPC says when talk opens, before the menu.
type Opener struct {
	Text     string          `yaml:"text" json:"text"`
	Hints    string          `yaml:"hints,omitempty" json:"hints,omitempty"`
	Variants []OpenerVariant `yaml:"variants,omitempty" json:"variants,omitempty"`
}

// HeardKey is the saved form of a heard topic: "<mob template id>.<key>".
func HeardKey(mobId int, key string) string {
	return fmt.Sprintf("%d.%s", mobId, key)
}
```

`Behavior` is a flag: the tree always tests the topic's own key (spec 4.1).

- [ ] **Step 4: Add fields to `internal/dialogue/types.go`**

In `DialogueFile`, after `Greetings`:

```go
	Opener      *Opener      `yaml:"opener,omitempty" json:"opener,omitempty"`
	Topics      []Topic      `yaml:"topics,omitempty" json:"topics,omitempty"`
```

In `PlayerState`, after `ChargeGold`:

```go
	// HasHeard reports whether the player has heard a topic, by HeardKey.
	// It backs heardRequired and the Dim / [*] state. Nil fails
	// heardRequired closed and treats every topic as unheard.
	HasHeard func(key string) bool
	// MarkHeard records a heard info topic on the character (saved).
	MarkHeard func(key string)
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/dialogue/ -count=1`
Expected: PASS (all old tests still pass; the new fields are additive).

- [ ] **Step 6: Commit**

```bash
git add internal/dialogue/topic.go internal/dialogue/topic_test.go internal/dialogue/types.go
git commit -F - <<'EOF'
feat(dialogue): topic, opener, gates and effects types (#375 phase 1)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 2: Eligibility, markers, opener, breadcrumbs, menu

**Files:**
- Create: `internal/dialogue/topics.go`
- Test: `internal/dialogue/topics_test.go`

`internal/dialogue` may import `internal/menu` (pure, no GoMud imports; verify with `go list -f '{{.Imports}}' ./internal/menu`).

- [ ] **Step 1: Write the failing tests**

```go
package dialogue

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// fakePlayer is a PlayerState over plain sets, for topic tests.
type fakePlayer struct {
	tokens map[string]bool
	items  map[int]bool
	heard  map[string]bool
	gold   int
}

func (f *fakePlayer) ps() *PlayerState {
	return &PlayerState{
		HasQuest:  func(t string) bool { return f.tokens[t] },
		HasItem:   func(id int) bool { return f.items[id] },
		HasHeard:  func(k string) bool { return f.heard[k] },
		MarkHeard: func(k string) { f.heard[k] = true },
		HasGold:   func(n int) bool { return f.gold >= n },
		GetQuestFlag: func(string) string { return "" },
		GiveQuest: func(t string) { f.tokens[t] = true },
		RemoveItem: func(id int) bool { delete(f.items, id); return true },
		ChargeGold: func(n int) bool {
			if f.gold < n {
				return false
			}
			f.gold -= n
			return true
		},
		GiveGold: func(n int) { f.gold += n },
	}
}

func newFake() *fakePlayer {
	return &fakePlayer{tokens: map[string]bool{}, items: map[int]bool{}, heard: map[string]bool{}}
}

func loadFixture(t *testing.T, src string) *DialogueFile {
	t.Helper()
	var df DialogueFile
	require.NoError(t, yaml.UnmarshalStrict([]byte(src), &df))
	return &df
}

const menuFixtureYAML = `mobid: 50
zone: testzone
opener:
  text: "Hello."
  variants:
    - questRequired: [7-start]
      text: "Back again?"
topics:
  - key: weather
    kind: info
    label: "How is the weather?"
    text: "Wet."
  - key: rumor
    kind: info
    label: "Heard any rumors?"
    text: "Wolves in the hills."
    marked: true
  - key: wolves
    kind: info
    label: "Where are the wolves?"
    text: "North ridge."
    heardRequired: [rumor]
  - key: hunt
    kind: offer
    quest: 7
    label: "The Wolf Hunt"
    playerLine: "Do you want the wolves dealt with?"
    text: "Bring me three pelts."
    breadcrumb: "Come back when you can hold a spear."
    goldRequired: 5
    questExcluded: [7-start, 7-end]
    grantsQuest: 7-start
  - key: hunt_done
    kind: handin
    quest: 7
    label: "The Wolf Hunt"
    playerLine: "I have the pelts."
    text: "Good work."
    questRequired: [7-pelts]
    questExcluded: [7-end]
    grantsQuest: 7-end
`

func TestEligible_GatesAndHeardRequired(t *testing.T) {
	df, f := loadFixture(t, menuFixtureYAML), newFake()
	wolves := df.FindTopic("wolves")
	require.False(t, df.Eligible(wolves, f.ps()), "wolves needs rumor heard")
	f.heard["50.rumor"] = true
	require.True(t, df.Eligible(wolves, f.ps()))

	hunt := df.FindTopic("hunt")
	require.False(t, df.Eligible(hunt, f.ps()), "needs 5 gold")
	f.gold = 5
	require.True(t, df.Eligible(hunt, f.ps()))
	f.tokens["7-start"] = true
	require.False(t, df.Eligible(hunt, f.ps()), "questExcluded once started")
}

func TestEligible_NilHasHeardFailsHeardRequiredClosed(t *testing.T) {
	df := loadFixture(t, menuFixtureYAML)
	ps := newFake().ps()
	ps.HasHeard = nil
	require.False(t, df.Eligible(df.FindTopic("wolves"), ps))
}

func TestMarker_PriorityHandInOverOfferOverInfo(t *testing.T) {
	df, f := loadFixture(t, menuFixtureYAML), newFake()
	require.Equal(t, menu.MarkerInfo, df.Marker(f.ps()), "only the marked rumor is lit")
	f.gold = 5
	require.Equal(t, menu.MarkerQuest, df.Marker(f.ps()))
	f.tokens["7-start"], f.tokens["7-pelts"] = true, true
	require.Equal(t, menu.MarkerHandIn, df.Marker(f.ps()))
	f.tokens["7-end"] = true
	f.heard["50.rumor"] = true
	require.Equal(t, "", df.Marker(f.ps()), "everything heard or done")
}

func TestOpenerFor_FirstPassingVariantElseRoot(t *testing.T) {
	df, f := loadFixture(t, menuFixtureYAML), newFake()
	text, _ := df.OpenerFor(f.ps())
	require.Equal(t, "Hello.", text)
	f.tokens["7-start"] = true
	text, _ = df.OpenerFor(f.ps())
	require.Equal(t, "Back again?", text)
}

func TestBreadcrumbs_OnlyForHiddenOffersNotYetTaken(t *testing.T) {
	df, f := loadFixture(t, menuFixtureYAML), newFake()
	require.Equal(t, []string{"Come back when you can hold a spear."}, df.Breadcrumbs(f.ps()))
	f.gold = 5
	require.Empty(t, df.Breadcrumbs(f.ps()), "eligible offers have no breadcrumb")
	f.gold = 0
	f.tokens["7-start"] = true
	require.Empty(t, df.Breadcrumbs(f.ps()), "a taken quest never breadcrumbs")
}

func TestMenu_QuestsFirstThenInfoInAuthoredOrderHeardDim(t *testing.T) {
	df, f := loadFixture(t, menuFixtureYAML), newFake()
	f.gold = 5
	f.heard["50.weather"] = true
	m := df.Menu("Hunter Bram", f.ps())
	require.Equal(t, "Talking with Hunter Bram", m.Heading)
	require.Equal(t, "Leave", m.Leave)
	ids := []string{}
	for _, e := range m.Entries {
		ids = append(ids, e.Id)
	}
	require.Equal(t, []string{"hunt", "weather", "rumor"}, ids)
	require.Equal(t, menu.MarkerQuest, m.Entries[0].Marker)
	require.Equal(t, "Do you want the wolves dealt with?", m.Entries[0].Detail)
	require.True(t, m.Entries[1].Dim, "heard weather is dim")
	require.Equal(t, menu.MarkerInfo, m.Entries[2].Marker)
}

func TestItemTopic_EligibleFirstElseWanted(t *testing.T) {
	df := loadFixture(t, `mobid: 51
zone: testzone
topics:
  - key: early
    kind: info
    label: "Here is the purse."
    text: "Not yet."
    questRequired: [9-start]
    requiresItem: 77
    grantsQuest: 9-found
    questExcluded: [9-found, 9-end]
`)
	f := newFake()
	f.items[77] = true
	tp, wanted := df.ItemTopic(77, f.ps())
	require.Nil(t, tp)
	require.True(t, wanted, "a topic wants it, none eligible: hand it back")
	f.tokens["9-start"] = true
	tp, wanted = df.ItemTopic(77, f.ps())
	require.NotNil(t, tp)
	require.True(t, wanted)
	tp, wanted = df.ItemTopic(78, f.ps())
	require.Nil(t, tp)
	require.False(t, wanted)
}

func TestNilFileIsSafe(t *testing.T) {
	var df *DialogueFile
	require.Equal(t, "", df.Marker(newFake().ps()))
	require.Nil(t, df.FindTopic("x"))
	text, hints := df.OpenerFor(nil)
	require.Equal(t, "", text+hints)
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/dialogue/ -run 'TestEligible|TestMarker|TestOpenerFor|TestBreadcrumbs|TestMenu_|TestItemTopic|TestNilFile' -count=1`
Expected: FAIL to compile (`df.FindTopic undefined`).

- [ ] **Step 3: Write `internal/dialogue/topics.go`**

```go
package dialogue

import "github.com/GoMudEngine/GoMud/internal/menu"

// markerRank orders name markers: [?] over [!] over [*] (requirements rule 9).
var markerRank = map[string]int{menu.MarkerInfo: 1, menu.MarkerQuest: 2, menu.MarkerHandIn: 3}

// FindTopic returns the topic with key, or nil.
func (df *DialogueFile) FindTopic(key string) *Topic {
	if df == nil {
		return nil
	}
	for i := range df.Topics {
		if df.Topics[i].Key == key {
			return &df.Topics[i]
		}
	}
	return nil
}

func (df *DialogueFile) heard(t *Topic, ps *PlayerState) bool {
	return ps != nil && ps.HasHeard != nil && ps.HasHeard(HeardKey(df.MobId, t.Key))
}

// Eligible reports whether t is open to the player now: every gate passes
// and every heardRequired key has been heard. Mood never gates a topic. A
// nil PlayerState skips the checks; a nil HasHeard fails heardRequired
// closed, the way every interrogative callback in this package does.
func (df *DialogueFile) Eligible(t *Topic, ps *PlayerState) bool {
	if df == nil || t == nil || !t.Gates.pass(ps) {
		return false
	}
	if ps == nil {
		return true
	}
	for _, k := range t.HeardRequired {
		if ps.HasHeard == nil || !ps.HasHeard(HeardKey(df.MobId, k)) {
			return false
		}
	}
	return true
}

// TopicMarker is the marker on an eligible topic's menu line.
func (df *DialogueFile) TopicMarker(t *Topic, ps *PlayerState) string {
	switch t.Kind {
	case KindHandIn:
		return menu.MarkerHandIn
	case KindOffer:
		return menu.MarkerQuest
	}
	if t.Marked && !df.heard(t, ps) {
		return menu.MarkerInfo
	}
	return ""
}

// Marker is the marker before the NPC's name for this player, or "".
func (df *DialogueFile) Marker(ps *PlayerState) string {
	if df == nil {
		return ""
	}
	out := ""
	for i := range df.Topics {
		t := &df.Topics[i]
		if !df.Eligible(t, ps) {
			continue
		}
		if m := df.TopicMarker(t, ps); markerRank[m] > markerRank[out] {
			out = m
		}
	}
	return out
}

// OpenerFor is the opener for this player: the first variant whose gates
// pass, else the root text.
func (df *DialogueFile) OpenerFor(ps *PlayerState) (text, hints string) {
	if df == nil || df.Opener == nil {
		return "", ""
	}
	for _, v := range df.Opener.Variants {
		if v.Gates.pass(ps) {
			return v.Text, v.Hints
		}
	}
	return df.Opener.Text, df.Opener.Hints
}

// Breadcrumbs are the lines the NPC says after the opener for offers hidden
// by their gates. An offer whose quest the player already holds never
// breadcrumbs: "come back later" is for quests not yet open.
func (df *DialogueFile) Breadcrumbs(ps *PlayerState) []string {
	if df == nil {
		return nil
	}
	var out []string
	for i := range df.Topics {
		t := &df.Topics[i]
		if t.Kind != KindOffer || t.Breadcrumb == "" || df.Eligible(t, ps) {
			continue
		}
		if ps != nil && ps.HasQuest != nil && t.GrantsQuest != "" && ps.HasQuest(t.GrantsQuest) {
			continue
		}
		out = append(out, t.Breadcrumb)
	}
	return out
}

// Menu builds the talk menu: eligible offers and hand-ins first, then
// eligible info topics, each group in authored order. Heard info is Dim.
func (df *DialogueFile) Menu(npcName string, ps *PlayerState) menu.Menu {
	m := menu.Menu{Heading: "Talking with " + npcName, Leave: "Leave"}
	if df == nil {
		return m
	}
	var quests, info []menu.Entry
	for i := range df.Topics {
		t := &df.Topics[i]
		if !df.Eligible(t, ps) {
			continue
		}
		e := menu.Entry{Id: t.Key, Marker: df.TopicMarker(t, ps), Label: t.Label}
		if t.Kind == KindInfo {
			e.Dim = df.heard(t, ps)
			info = append(info, e)
			continue
		}
		e.Detail = t.PlayerLine
		quests = append(quests, e)
	}
	m.Entries = append(quests, info...)
	return m
}

// ItemTopic is the first eligible topic wanting itemId (its requiresItem),
// and whether any topic on this NPC wants that item at all. A wanted item
// with no eligible topic is handed back by give (R8).
func (df *DialogueFile) ItemTopic(itemId int, ps *PlayerState) (t *Topic, wanted bool) {
	if df == nil || itemId <= 0 {
		return nil, false
	}
	for i := range df.Topics {
		tp := &df.Topics[i]
		if tp.RequiresItem != itemId {
			continue
		}
		wanted = true
		if df.Eligible(tp, ps) {
			return tp, true
		}
	}
	return nil, wanted
}
```

Note: `Eligible` uses `HasItem` (the backpack) for `requiresItem`, so `ItemTopic` asks the same question `give` will act on.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/dialogue/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/dialogue/topics.go internal/dialogue/topics_test.go
git commit -F - <<'EOF'
feat(dialogue): topic eligibility, markers, opener, breadcrumbs, menu (#375 phase 1)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 3: Applying a chosen topic

**Files:**
- Modify: `internal/dialogue/topics.go`
- Test: `internal/dialogue/topics_apply_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package dialogue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApply_InfoRecordsHeardAndGrants(t *testing.T) {
	df, f := loadFixture(t, menuFixtureYAML), newFake()
	require.True(t, df.Apply(df.FindTopic("rumor"), f.ps()))
	require.True(t, f.heard["50.rumor"])
}

func TestApply_OfferIsNotRecordedAsHeard(t *testing.T) {
	df, f := loadFixture(t, menuFixtureYAML), newFake()
	f.gold = 5
	require.True(t, df.Apply(df.FindTopic("hunt"), f.ps()))
	require.True(t, f.tokens["7-start"])
	require.False(t, f.heard["50.hunt"], "quest state lives in tokens")
}

func TestApply_TakesRequiredItem(t *testing.T) {
	df := loadFixture(t, `mobid: 52
zone: testzone
topics:
  - key: purse
    kind: handin
    quest: 9
    label: "The Lost Purse"
    playerLine: "I found your purse."
    text: "Thank you."
    requiresItem: 77
    questExcluded: [9-end]
    grantsQuest: 9-end
`)
	f := newFake()
	f.items[77] = true
	require.True(t, df.Apply(df.FindTopic("purse"), f.ps()))
	require.False(t, f.items[77])
	require.True(t, f.tokens["9-end"])
}

func TestApply_FailedChargeChangesNothing(t *testing.T) {
	df := loadFixture(t, `mobid: 53
zone: testzone
topics:
  - key: paid
    kind: info
    label: "Tell me the secret."
    text: "For a price."
    chargesGold: 10
    goldRequired: 10
    grantsQuest: 3-told
    questExcluded: [3-told, 3-end]
`)
	f := newFake()
	f.gold = 4
	require.False(t, df.Apply(df.FindTopic("paid"), f.ps()))
	require.Equal(t, 4, f.gold)
	require.False(t, f.tokens["3-told"])
	require.False(t, f.heard["53.paid"])
}

func TestApply_FailedDeliveryRefundsAndIsNotHeard(t *testing.T) {
	df := loadFixture(t, `mobid: 54
zone: testzone
topics:
  - key: gift
    kind: info
    label: "Can you spare a lantern?"
    text: "Take this."
    chargesGold: 2
    goldRequired: 2
    givesItem: 900
`)
	f := newFake()
	f.gold = 2
	ps := f.ps()
	ps.GiveItem = func(int) bool { return false }
	require.False(t, df.Apply(df.FindTopic("gift"), ps))
	require.Equal(t, 2, f.gold, "charge refunded")
	require.False(t, f.heard["54.gift"])
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/dialogue/ -run TestApply_ -count=1`
Expected: FAIL to compile (`df.Apply undefined`).

- [ ] **Step 3: Add `Apply` to `internal/dialogue/topics.go`**

```go
// Apply takes a chosen topic's price and runs its effects in the engine's
// fixed order (delivery first, then the rest; see applyQuestEffects), and
// records an info topic as heard once its effects landed. It reports false,
// with nothing changed and any charge refunded, when the price could not be
// taken or an item could not be delivered. It does not check eligibility:
// the caller re-checks right before.
func (df *DialogueFile) Apply(t *Topic, ps *PlayerState) bool {
	if df == nil || t == nil {
		return false
	}
	if !tryCharge(t.ChargesGold, ps) {
		return false
	}
	if !applyPaidEffects(t.ChargesGold, t.GrantsQuest, t.RequiresItem, t.GivesItem,
		t.SetsQuestFlag, t.BumpsRep, t.GivesGold, ps) {
		return false
	}
	if t.Kind == KindInfo && ps != nil && ps.MarkHeard != nil {
		ps.MarkHeard(HeardKey(df.MobId, t.Key))
	}
	return true
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/dialogue/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/dialogue/topics.go internal/dialogue/topics_apply_test.go
git commit -F - <<'EOF'
feat(dialogue): Apply runs a chosen topic's price and effects (#375 phase 1)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 4: Heard set on the character

**Files:**
- Create: `internal/characters/heard.go`
- Modify: `internal/characters/character.go:320` (add the field after `QuestFlags`)
- Test: `internal/characters/heard_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package characters

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestHeard_MarkKeepsSortedUnique(t *testing.T) {
	c := &Character{}
	require.False(t, c.HasHeard("94.b"))
	c.MarkHeard("94.b")
	c.MarkHeard("12.a")
	c.MarkHeard("94.b")
	require.Equal(t, []string{"12.a", "94.b"}, c.HeardTopics)
	require.True(t, c.HasHeard("12.a"))
}

func TestHeard_RenameAppliesAndDedupes(t *testing.T) {
	c := &Character{HeardTopics: []string{"12.a", "12.old", "94.b"}}
	c.RenameHeardTopics(map[string]string{"12.old": "12.a", "94.b": "94.c"})
	require.Equal(t, []string{"12.a", "94.c"}, c.HeardTopics)
}

func TestHeard_SurvivesYAMLRoundTrip(t *testing.T) {
	c := &Character{Name: "Mira"}
	c.MarkHeard("94.where_to_look")
	out, err := yaml.Marshal(c)
	require.NoError(t, err)
	require.Contains(t, string(out), "heardtopics:")
	var back Character
	require.NoError(t, yaml.Unmarshal(out, &back))
	require.True(t, back.HasHeard("94.where_to_look"))
}

func TestHeard_EmptyIsOmitted(t *testing.T) {
	out, err := yaml.Marshal(&Character{Name: "Mira"})
	require.NoError(t, err)
	require.NotContains(t, string(out), "heardtopics")
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/characters/ -run TestHeard_ -count=1`
Expected: FAIL to compile (`c.HasHeard undefined`).

- [ ] **Step 3: Add the field** after `QuestFlags` in `internal/characters/character.go`:

```go
	HeardTopics             []string                       `yaml:"heardtopics,omitempty"`         // heard talk topics, "<mobId>.<key>", sorted (#375)
```

(Match the column alignment of the neighbouring lines with `gofmt`.)

- [ ] **Step 4: Write `internal/characters/heard.go`**

```go
package characters

import "sort"

// HasHeard reports whether the character has heard a talk topic, by its
// saved key "<mob template id>.<key>" (dialogue.HeardKey).
func (c *Character) HasHeard(key string) bool {
	i := sort.SearchStrings(c.HeardTopics, key)
	return i < len(c.HeardTopics) && c.HeardTopics[i] == key
}

// MarkHeard records a heard info topic, keeping the list sorted and unique.
func (c *Character) MarkHeard(key string) {
	i := sort.SearchStrings(c.HeardTopics, key)
	if i < len(c.HeardTopics) && c.HeardTopics[i] == key {
		return
	}
	c.HeardTopics = append(c.HeardTopics, "")
	copy(c.HeardTopics[i+1:], c.HeardTopics[i:])
	c.HeardTopics[i] = key
}

// RenameHeardTopics applies the topic-key manifest's renames (old -> new)
// so a renamed topic stays heard. Run at load.
func (c *Character) RenameHeardTopics(renames map[string]string) {
	if len(renames) == 0 || len(c.HeardTopics) == 0 {
		return
	}
	keys := c.HeardTopics
	c.HeardTopics = nil
	for _, k := range keys {
		if n, ok := renames[k]; ok {
			k = n
		}
		c.MarkHeard(k)
	}
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/characters/ -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/characters/heard.go internal/characters/heard_test.go internal/characters/character.go
git commit -F - <<'EOF'
feat(characters): saved heard-topic set (#375 phase 1)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 5: Topic-key manifest

**Files:**
- Create: `internal/dialogue/manifest.go`
- Test: `internal/dialogue/manifest_test.go`

Format of `_datafiles/world/dogmud/dialogue/topic-keys.txt`:

```
# Topic-key manifest (#375). One <mobId>.<key> per line, sorted.
94.protection_report
94.where_to_look
# retired
12.old_rumor
# renamed
12.gossip -> 12.rumor
```

- [ ] **Step 1: Write the failing tests**

```go
package dialogue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const manifestFixture = `# Topic-key manifest (#375).
94.a
94.b
# retired
12.gone
# renamed
12.old -> 12.new
`

func TestParseManifest(t *testing.T) {
	m, err := ParseManifest([]byte(manifestFixture))
	require.NoError(t, err)
	require.Equal(t, []string{"94.a", "94.b"}, m.Keys)
	require.Equal(t, []string{"12.gone"}, m.Retired)
	require.Equal(t, map[string]string{"12.old": "12.new"}, m.Renamed)
}

func TestParseManifest_RefusesMalformedRename(t *testing.T) {
	_, err := ParseManifest([]byte("# renamed\n12.old 12.new\n"))
	require.Error(t, err)
}

func TestManifestDiff(t *testing.T) {
	m, err := ParseManifest([]byte(manifestFixture))
	require.NoError(t, err)
	missing, unlisted := m.Diff([]string{"94.a", "94.c"})
	require.Equal(t, []string{"94.b"}, missing, "shipped key gone with no retired/renamed line")
	require.Equal(t, []string{"94.c"}, unlisted)
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/dialogue/ -run 'TestParseManifest|TestManifestDiff' -count=1`
Expected: FAIL to compile.

- [ ] **Step 3: Write `internal/dialogue/manifest.go`**

```go
package dialogue

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Manifest is the committed list of shipped topic keys. Saved characters
// point at these keys (HeardTopics), so a key may leave content only through
// a retired or renamed line.
type Manifest struct {
	Keys    []string
	Retired []string
	Renamed map[string]string // old -> new
}

// ParseManifest reads the manifest text: keys, then optional "# retired"
// and "# renamed" sections ("old -> new"). Other comment lines are ignored.
func ParseManifest(data []byte) (Manifest, error) {
	m := Manifest{Renamed: map[string]string{}}
	section := "keys"
	for n, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			continue
		case line == "# retired":
			section = "retired"
			continue
		case line == "# renamed":
			section = "renamed"
			continue
		case strings.HasPrefix(line, "#"):
			continue
		}
		switch section {
		case "keys":
			m.Keys = append(m.Keys, line)
		case "retired":
			m.Retired = append(m.Retired, line)
		case "renamed":
			old, nu, ok := strings.Cut(line, "->")
			if !ok || strings.TrimSpace(old) == "" || strings.TrimSpace(nu) == "" {
				return m, fmt.Errorf("topic-keys.txt line %d: a rename is \"old -> new\": %q", n+1, line)
			}
			m.Renamed[strings.TrimSpace(old)] = strings.TrimSpace(nu)
		}
	}
	sort.Strings(m.Keys)
	return m, nil
}

// Diff compares content keys with the manifest: missing are listed keys gone
// from content with no retired or renamed line; unlisted are content keys
// the manifest does not list yet.
func (m Manifest) Diff(content []string) (missing, unlisted []string) {
	have := map[string]bool{}
	for _, k := range content {
		have[k] = true
	}
	listed := map[string]bool{}
	for _, k := range m.Keys {
		listed[k] = true
	}
	excused := map[string]bool{}
	for _, k := range m.Retired {
		excused[k] = true
	}
	for k := range m.Renamed {
		excused[k] = true
	}
	for _, k := range m.Keys {
		if !have[k] && !excused[k] {
			missing = append(missing, k)
		}
	}
	for _, k := range content {
		if !listed[k] {
			unlisted = append(unlisted, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(unlisted)
	return missing, unlisted
}

// HeardRenames reads the shipped manifest's renames for the load migration.
// A missing or unreadable manifest renames nothing.
func HeardRenames() map[string]string {
	path := util.FilePath(string(configs.GetFilePathsConfig().DataFiles) + `/dialogue/topic-keys.txt`)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	m, err := ParseManifest(data)
	if err != nil {
		return nil
	}
	return m.Renamed
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/dialogue/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/dialogue/manifest.go internal/dialogue/manifest_test.go
git commit -F - <<'EOF'
feat(dialogue): topic-key manifest parse and diff (#375 phase 1)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 6: Proposed-world validators

**Files:**
- Create: `internal/dialogue/world.go`, `internal/dialogue/world_registry.go`
- Test: `internal/dialogue/world_test.go`, fixtures under `internal/dialogue/testdata/world/`

Every rule in spec 5.2 gets one malformed fixture and a test that sees `ValidateWorld` refuse it, plus one clean fixture that passes. That proves each check can fail.

- [ ] **Step 1: Write the test harness and the clean fixture test**

```go
package dialogue

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// testWorld is a World over two quests: 7 (The Wolf Hunt, steps start,
// pelts, end) and 9 (The Lost Purse, steps start, end), flag 7-mood=calm,
// items 77 and 900.
func testWorld(files ...*DialogueFile) World {
	quests := map[int]QuestInfo{
		7: {Title: "The Wolf Hunt", StartToken: "7-start", EndToken: "7-end"},
		9: {Title: "The Lost Purse", StartToken: "9-start", EndToken: "9-end"},
	}
	tokens := map[string]int{"7-start": 7, "7-pelts": 7, "7-end": 7, "9-start": 9, "9-found": 9, "9-end": 9}
	return World{
		Files:        files,
		QuestById:    func(id int) (QuestInfo, bool) { q, ok := quests[id]; return q, ok },
		QuestOfToken: func(tok string) (QuestInfo, bool) { id, ok := tokens[tok]; return quests[id], ok },
		FlagDeclared: func(k, v string) bool { return k == "7-mood" && v == "calm" },
		ItemExists:   func(id int) bool { return id == 77 || id == 900 },
	}
}

func problems(t *testing.T, src string) []string {
	t.Helper()
	ps := ValidateWorld(testWorld(loadFixture(t, src)))
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.String())
	}
	return out
}

func requireProblem(t *testing.T, src, want string) {
	t.Helper()
	got := problems(t, src)
	for _, g := range got {
		if strings.Contains(g, want) {
			return
		}
	}
	t.Fatalf("expected a problem containing %q, got:\n%s", want, strings.Join(got, "\n"))
}

func TestValidateWorld_CleanFixturePasses(t *testing.T) {
	require.Empty(t, problems(t, menuFixtureYAML))
}
```

- [ ] **Step 2: Write one failing test per rule**

Each test takes `menuFixtureYAML` (from Task 2), breaks exactly one thing with `strings.Replace(menuFixtureYAML, old, new, 1)` and asserts the problem text. Write them all:

```go
func breakFixture(old, nu string) string { return strings.Replace(menuFixtureYAML, old, nu, 1) }

// emDash is built from its code point so no dash character sits in source.
var emDash = string(rune(0x2014))

func TestValidateWorld_Rules(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"bad key", breakFixture("key: weather", "key: Weather Talk"), `key must match`},
		{"duplicate key", breakFixture("key: rumor", "key: weather"), `duplicate key`},
		{"bad kind", breakFixture("kind: info\n    label: \"How is", "kind: chat\n    label: \"How is"), `kind must be info, offer or handin`},
		{"offer without playerLine", breakFixture(`    playerLine: "Do you want the wolves dealt with?"`+"\n", ""), `offer topics need quest and playerLine`},
		{"marked offer", breakFixture("    breadcrumb: \"Come back", "    marked: true\n    breadcrumb: \"Come back"), `marked belongs on info topics only`},
		{"breadcrumb on info", breakFixture("    text: \"Wet.\"", "    text: \"Wet.\"\n    breadcrumb: \"Later.\""), `breadcrumb belongs on offer topics only`},
		{"offer grants wrong token", breakFixture("grantsQuest: 7-start", "grantsQuest: 7-pelts"), `offer must grant "7-start"`},
		{"handin grants wrong token", breakFixture("grantsQuest: 7-end", "grantsQuest: 7-pelts"), `handin must grant "7-end"`},
		{"missing quest", breakFixture("quest: 7\n    label: \"The Wolf Hunt\"\n    playerLine: \"Do", "quest: 70\n    label: \"The Wolf Hunt\"\n    playerLine: \"Do"), `quest 70 does not exist`},
		{"label not title", breakFixture(`label: "The Wolf Hunt"`, `label: "Wolves"`), `label must be the quest title`},
		{"long label", breakFixture(`label: "How is the weather?"`, `label: "`+strings.Repeat("a", 51)+`"`), `50`},
		{"dash in label", breakFixture(`label: "How is the weather?"`, "label: \"How is it "+emDash+" wet?\""), `dash`},
		{"dash in breadcrumb", breakFixture(`breadcrumb: "Come back when you can hold a spear."`, "breadcrumb: \"Come back "+emDash+" later.\""), `dash`},
		{"duplicate info label", breakFixture(`label: "Heard any rumors?"`, `label: "How is the weather?"`), `label repeats topic`},
		{"heardRequired not info", breakFixture("heardRequired: [rumor]", "heardRequired: [hunt]"), `heardRequired "hunt" is not an info topic`},
		{"heardRequired cycle", breakFixture("    text: \"Wolves in the hills.\"", "    text: \"Wolves in the hills.\"\n    heardRequired: [wolves]"), `heardRequired cycle`},
		{"charge without gold gate", breakFixture("    goldRequired: 5\n", "    chargesGold: 5\n"), `chargesGold 5 needs goldRequired of at least 5`},
		{"semicolon", breakFixture(`text: "Wet."`, `text: "Wet; very."`), `semicolons`},
		{"grant not excluded", breakFixture("questExcluded: [7-start, 7-end]", "questExcluded: [7-end]"), `must also appear in questExcluded`},
		{"end token not excluded", breakFixture("questExcluded: [7-start, 7-end]", "questExcluded: [7-start]"), `must include the end token "7-end"`},
		{"unknown token", breakFixture("questRequired: [7-pelts]", "questRequired: [7-nope]"), `quest token "7-nope" does not exist`},
		{"unknown item", breakFixture("    goldRequired: 5\n", "    goldRequired: 5\n    requiresItem: 4242\n"), `item 4242 does not exist`},
		{"undeclared flag", breakFixture("    text: \"Wet.\"", "    text: \"Wet.\"\n    questFlagRequired: {7-mood: angry}"), `quest flag "7-mood"="angry" is not declared`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.NotEqual(t, menuFixtureYAML, c.src, "the fixture edit did not apply")
			requireProblem(t, c.src, c.want)
		})
	}
}

func TestValidateWorld_BehaviorTopicsMatchTheTree(t *testing.T) {
	df := loadFixture(t, breakFixture("    text: \"Wet.\"", "    text: \"Wet.\"\n    behavior: true"))
	w := testWorld(df)
	w.BehaviorTopics = func(mobId int) []string { return []string{"ferry"} }
	var got []string
	for _, p := range ValidateWorld(w) {
		got = append(got, p.String())
	}
	joined := strings.Join(got, "\n")
	require.Contains(t, joined, `behavior: true but the mob's tree has no topic_is "weather"`)
	require.Contains(t, joined, `topic_is "ferry" names no topic on this mob`)
}

func TestValidateWorld_ItemGiveGrantsAreRefused(t *testing.T) {
	w := testWorld(loadFixture(t, menuFixtureYAML))
	w.ItemGiveGrants = []string{"quest 10 trigger 2"}
	ps := ValidateWorld(w)
	require.Len(t, ps, 1)
	require.Contains(t, ps[0].String(), "item_give triggers may not grant")
	require.Contains(t, ps[0].String(), fmt.Sprint("quest 10 trigger 2"))
}
```

- [ ] **Step 3: Run to verify failure**

Run: `go test ./internal/dialogue/ -run TestValidateWorld -count=1`
Expected: FAIL to compile (`undefined: World`).

- [ ] **Step 4: Write `internal/dialogue/world.go`**

```go
package dialogue

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/menu"
)

// QuestInfo is what the dialogue validators need about one quest.
type QuestInfo struct {
	Title      string
	StartToken string
	EndToken   string
}

// Problem is one validator finding. Every finding refuses: the phase 1
// conversion clears the backlog in one pass, so there is no warning tier.
type Problem struct {
	MobId int
	Where string
	Msg   string
}

func (p Problem) String() string { return fmt.Sprintf("mob %d %s: %s", p.MobId, p.Where, p.Msg) }

// World is everything the dialogue validators read. Boot and CI build it
// from the loaded registries (RegistryWorld); the phase 6 builder overlays
// pending edits on it, so a save is checked against the world as it would
// be. Nil funcs report "does not exist".
type World struct {
	Files        []*DialogueFile
	QuestById    func(questId int) (QuestInfo, bool)
	QuestOfToken func(token string) (QuestInfo, bool)
	FlagDeclared func(key, value string) bool
	ItemExists   func(id int) bool
	// BehaviorTopics lists the topic_is keys in a mob's behavior tree.
	// Nil skips the behavior checks (no trees in this world).
	BehaviorTopics func(mobId int) []string
	// ItemGiveGrants names quest item_give triggers that carry a grant
	// action ("quest 10 trigger 2"). Hand-ins are NPC topics (R7).
	ItemGiveGrants []string
}

var topicKeyPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

// dashRunes are the em and en dash, built from code points so no dash
// character sits in source.
var dashRunes = string([]rune{0x2014, 0x2013})

// ValidateWorld runs every dialogue rule over every file in w.
func ValidateWorld(w World) []Problem {
	var out []Problem
	for _, df := range w.Files {
		out = append(out, validateTopicFile(w, df)...)
	}
	for _, where := range w.ItemGiveGrants {
		out = append(out, Problem{Where: where, Msg: "item_give triggers may not grant a quest token: hand-ins are NPC topics"})
	}
	return out
}

func validateTopicFile(w World, df *DialogueFile) []Problem {
	var out []Problem
	add := func(where, format string, a ...any) {
		out = append(out, Problem{MobId: df.MobId, Where: where, Msg: fmt.Sprintf(format, a...)})
	}
	semi := func(where, text string) {
		if strings.Contains(text, ";") {
			add(where, "semicolons are the command separator and truncate spoken text, rewrite without ';'")
		}
	}

	for i, g := range df.Greetings {
		semi(fmt.Sprintf("greeting %d", i+1), g.Text)
	}
	if df.Opener != nil {
		semi("opener", df.Opener.Text)
		for i, v := range df.Opener.Variants {
			where := fmt.Sprintf("opener variant %d", i+1)
			semi(where, v.Text)
			out = append(out, gateRefProblems(w, df.MobId, where, v.Gates, Effects{})...)
		}
	}

	keys, infoKeys, infoLabels := map[string]bool{}, map[string]bool{}, map[string]string{}
	for _, t := range df.Topics {
		if t.Kind == KindInfo {
			infoKeys[t.Key] = true
		}
	}
	for i := range df.Topics {
		t := &df.Topics[i]
		where := fmt.Sprintf("topic %q", t.Key)
		if !topicKeyPattern.MatchString(t.Key) {
			add(where, "key must match ^[a-z0-9_]+$")
		}
		if keys[t.Key] {
			add(where, "duplicate key")
		}
		keys[t.Key] = true

		switch t.Kind {
		case KindInfo:
			if t.Quest != 0 || t.PlayerLine != "" {
				add(where, "quest and playerLine belong on offer and handin topics only")
			}
			if t.Breadcrumb != "" {
				add(where, "breadcrumb belongs on offer topics only")
			}
			lower := strings.ToLower(t.Label)
			if prev, dup := infoLabels[lower]; dup {
				add(where, "label repeats topic %q", prev)
			} else {
				infoLabels[lower] = t.Key
			}
		case KindOffer, KindHandIn:
			if t.Quest == 0 || t.PlayerLine == "" {
				add(where, "%s topics need quest and playerLine", t.Kind)
			}
			if t.Marked {
				add(where, "marked belongs on info topics only")
			}
			if t.Kind == KindHandIn && t.Breadcrumb != "" {
				add(where, "breadcrumb belongs on offer topics only")
			}
			if q, ok := lookupQuest(w, t.Quest); !ok {
				add(where, "quest %d does not exist", t.Quest)
			} else {
				want := q.StartToken
				if t.Kind == KindHandIn {
					want = q.EndToken
				}
				if t.GrantsQuest != want {
					add(where, "%s must grant %q, not %q", t.Kind, want, t.GrantsQuest)
				}
				if t.Label != q.Title {
					add(where, "label must be the quest title %q", q.Title)
				}
			}
		default:
			add(where, "kind must be info, offer or handin, not %q", t.Kind)
		}

		lint := menu.Menu{Entries: []menu.Entry{{Id: t.Key, Label: t.Label, Detail: t.PlayerLine}}}
		for _, p := range menu.LintLabels(lint) {
			add(where, "%s", p)
		}
		if strings.ContainsAny(t.Breadcrumb, dashRunes) {
			add(where+" breadcrumb", "no em or en dash in new text")
		}
		semi(where+" text", t.Text)
		semi(where+" breadcrumb", t.Breadcrumb)
		for _, k := range t.HeardRequired {
			if !infoKeys[k] {
				add(where, "heardRequired %q is not an info topic in this file", k)
			}
		}
		if t.ChargesGold > 0 && t.GoldRequired < t.ChargesGold {
			add(where, "chargesGold %d needs goldRequired of at least %d", t.ChargesGold, t.ChargesGold)
		}
		out = append(out, gateRefProblems(w, df.MobId, where, t.Gates, t.Effects)...)
	}

	if cyc := heardCycle(df); cyc != "" {
		add("topics", "heardRequired cycle through %q", cyc)
	}

	if w.BehaviorTopics != nil {
		tree := map[string]bool{}
		for _, k := range w.BehaviorTopics(df.MobId) {
			tree[k] = true
		}
		for _, t := range df.Topics {
			if t.Behavior && !tree[t.Key] {
				add(fmt.Sprintf("topic %q", t.Key), "behavior: true but the mob's tree has no topic_is %q", t.Key)
			}
		}
		treeKeys := make([]string, 0, len(tree))
		for k := range tree {
			treeKeys = append(treeKeys, k)
		}
		sort.Strings(treeKeys)
		for _, k := range treeKeys {
			if !keys[k] {
				add("behavior tree", "topic_is %q names no topic on this mob", k)
			}
		}
	}
	return out
}

func lookupQuest(w World, id int) (QuestInfo, bool) {
	if w.QuestById == nil {
		return QuestInfo{}, false
	}
	return w.QuestById(id)
}

// gateRefProblems carries the old validator's reference rules over to
// topics: a grant must be excluded (or the offer repeats), the quest's end
// token must be excluded (or a finished quest is re-offered), and every
// token, flag and item must exist.
func gateRefProblems(w World, mobId int, where string, g Gates, e Effects) []Problem {
	var out []Problem
	add := func(format string, a ...any) {
		out = append(out, Problem{MobId: mobId, Where: where, Msg: fmt.Sprintf(format, a...)})
	}
	excluded := func(tok string) bool {
		for _, x := range g.QuestExcluded {
			if x == tok {
				return true
			}
		}
		return false
	}
	quest := func(tok string) (QuestInfo, bool) {
		if w.QuestOfToken == nil {
			return QuestInfo{}, false
		}
		return w.QuestOfToken(tok)
	}
	if e.GrantsQuest != "" {
		if !excluded(e.GrantsQuest) {
			add("grantsQuest %q must also appear in questExcluded, or the offer repeats forever", e.GrantsQuest)
		}
		if q, ok := quest(e.GrantsQuest); ok && !excluded(q.EndToken) {
			add("questExcluded must include the end token %q, or a player who finished the quest gets it again", q.EndToken)
		}
	}
	for _, tok := range append(append([]string{e.GrantsQuest}, g.QuestRequired...), g.QuestExcluded...) {
		if tok == "" {
			continue
		}
		if _, ok := quest(tok); !ok {
			add("quest token %q does not exist", tok)
		}
	}
	flag := func(k, v string) bool { return w.FlagDeclared != nil && w.FlagDeclared(k, v) }
	for _, m := range []map[string]string{g.QuestFlagRequired, g.QuestFlagExcluded} {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if !flag(k, m[k]) {
				add("quest flag %q=%q is not declared by any quest", k, m[k])
			}
		}
	}
	if e.SetsQuestFlag != nil && !flag(e.SetsQuestFlag.Key, e.SetsQuestFlag.Value) {
		add("setsQuestFlag %q=%q is not declared by any quest", e.SetsQuestFlag.Key, e.SetsQuestFlag.Value)
	}
	for _, id := range []int{g.RequiresItem, g.ItemExcluded, e.GivesItem} {
		if id != 0 && (w.ItemExists == nil || !w.ItemExists(id)) {
			add("item %d does not exist", id)
		}
	}
	return out
}

// heardCycle returns a key on a heardRequired cycle, or "".
func heardCycle(df *DialogueFile) string {
	deps := map[string][]string{}
	for _, t := range df.Topics {
		deps[t.Key] = t.HeardRequired
	}
	const (
		unseen = iota
		active
		done
	)
	state := map[string]int{}
	var visit func(k string) string
	visit = func(k string) string {
		switch state[k] {
		case active:
			return k
		case done:
			return ""
		}
		state[k] = active
		for _, d := range deps[k] {
			if c := visit(d); c != "" {
				return c
			}
		}
		state[k] = done
		return ""
	}
	for _, t := range df.Topics {
		if c := visit(t.Key); c != "" {
			return c
		}
	}
	return ""
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/dialogue/ -run TestValidateWorld -count=1 -v`
Expected: PASS, with every subtest of `TestValidateWorld_Rules` listed. If a case fails on "the fixture edit did not apply", fix the case's `old` string, never the rule.

- [ ] **Step 6: Write `internal/dialogue/world_registry.go`**

```go
package dialogue

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"gopkg.in/yaml.v2"
)

// LoadAllFiles reads every dialogue file under {DataFiles}/dialogue
// strictly: an unknown key is a failure, so a stray old-format field or a
// typo cannot be silently ignored. It bypasses the live cache.
func LoadAllFiles() (files []*DialogueFile, failures []string) {
	base := filepath.Join(string(configs.GetFilePathsConfig().DataFiles), "dialogue")
	paths, _ := filepath.Glob(filepath.Join(base, "*", "*.yaml"))
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: unreadable: %v", filepath.ToSlash(p), err))
			continue
		}
		var df DialogueFile
		if err := yaml.UnmarshalStrict(data, &df); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", filepath.ToSlash(p), err))
			continue
		}
		files = append(files, &df)
	}
	return files, failures
}

// RegistryWorld is the loaded world: the shipped dialogue files plus the
// quest and item registries. Load quests and items first. Unreadable files
// come back as problems. behaviorTopics may be nil.
func RegistryWorld(behaviorTopics func(mobId int) []string) (World, []Problem) {
	files, failures := LoadAllFiles()
	var probs []Problem
	for _, f := range failures {
		probs = append(probs, Problem{Where: "file", Msg: f})
	}
	info := func(q *quests.Quest) (QuestInfo, bool) {
		if q == nil || len(q.Steps) == 0 {
			return QuestInfo{}, false
		}
		return QuestInfo{
			Title:      q.Name,
			StartToken: quests.PartsToToken(q.QuestId, q.Steps[0].Id),
			EndToken:   fmt.Sprintf("%d-end", q.QuestId),
		}, true
	}
	var giveGrants []string
	for _, q := range quests.GetAllQuests() {
		for i, t := range q.Triggers {
			if t.Event != "item_give" {
				continue
			}
			for _, a := range t.Actions {
				if a.Grant != "" {
					giveGrants = append(giveGrants, fmt.Sprintf("quest %d trigger %d", q.QuestId, i))
					break
				}
			}
		}
	}
	return World{
		Files:     files,
		QuestById: func(id int) (QuestInfo, bool) { return info(quests.GetQuestById(id)) },
		QuestOfToken: func(tok string) (QuestInfo, bool) {
			if !strings.Contains(tok, "-") {
				return QuestInfo{}, false
			}
			return info(quests.GetQuest(tok))
		},
		FlagDeclared:   func(k, v string) bool { return quests.ValidateFlag(k, v) == nil },
		ItemExists:     func(id int) bool { return items.GetItemSpec(id) != nil },
		BehaviorTopics: behaviorTopics,
		ItemGiveGrants: giveGrants,
	}, probs
}
```

Before relying on it, confirm by reading `quests.GetQuest` (`internal/quests/quests.go:290`) that it returns nil for a step id the quest does not have. If it returns the quest for any step, add a step check in `QuestOfToken` (walk `q.Steps` for the step part of `quests.TokenToParts(tok)`; accept `end` too) and add a test case for it.

- [ ] **Step 7: Run all dialogue tests**

Run: `go test ./internal/dialogue/ -count=1`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/dialogue/world.go internal/dialogue/world_registry.go internal/dialogue/world_test.go
git commit -F - <<'EOF'
feat(dialogue): proposed-world topic validators, one fixture per rule (#375 phase 1)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 7: Behavior-tree topics (`player_topic`, `topic_is`)

**Files:**
- Create: `internal/behaviortree/topics.go`, `internal/behaviortree/topics_test.go`
- Modify: `internal/behaviortree/events.go` (add `player_topic`; `player_ask` stays until Task 21), `internal/behaviortree/conditions.go:12` (register)

- [ ] **Step 1: Write the failing tests**

```go
package behaviortree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTopicIs_MatchesTheEventKeyOnly(t *testing.T) {
	ctx := &EvalContext{Event: EventContext{EventType: "player_topic", Text: "ferry"}}
	require.Equal(t, Success, condTopicIs(map[string]any{"topic": "ferry"}, ctx))
	require.Equal(t, Failure, condTopicIs(map[string]any{"topic": "portal"}, ctx))
	ctx.Event.EventType = "player_give"
	require.Equal(t, Failure, condTopicIs(map[string]any{"topic": "ferry"}, ctx), "only player_topic carries a key")
}

func TestPlayerTopicIsAKnownEvent(t *testing.T) {
	require.True(t, KnownBehaviorEvents["player_topic"])
}

func TestTopicKeysInFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "9574-a_weathered_deckhand.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`tree:
  type: selector
  children:
    - type: sequence
      event: player_topic
      children:
        - type: condition
          check: topic_is
          topic: ferry
        - type: action
          do: board_ferry
    - type: sequence
      event: player_topic
      children:
        - type: condition
          check: topic_is
          topic: fare
        - type: action
          do: say
          text: "Two coppers."
`), 0644))
	keys, err := TopicKeysInFile(path)
	require.NoError(t, err)
	require.Equal(t, []string{"fare", "ferry"}, keys)
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/behaviortree/ -run 'TestTopicIs|TestPlayerTopic|TestTopicKeysInFile' -count=1`
Expected: FAIL to compile.

- [ ] **Step 3: Write `internal/behaviortree/topics.go`**

```go
package behaviortree

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v2"
)

// condTopicIs succeeds when a player_topic event carries this branch's
// topic key (quest arc #375 R12: talk topics replace typed ask words).
func condTopicIs(params map[string]any, ctx *EvalContext) Result {
	if ctx.Event.EventType != "player_topic" {
		return Failure
	}
	if key := getStringParam(params, "topic"); key != "" && key == ctx.Event.Text {
		return Success
	}
	return Failure
}

// TopicKeysInFile lists the topic_is keys in a behavior file, sorted and
// unique, for the dialogue validator.
func TopicKeysInFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var def TreeDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	seen := map[string]bool{}
	var walk func(n NodeDef)
	walk = func(n NodeDef) {
		if n.Check == "topic_is" {
			if k := getStringParam(n.Params, "topic"); k != "" {
				seen[k] = true
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
		if n.Child != nil {
			walk(*n.Child)
		}
	}
	walk(def.Tree)
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}
```

`TreeDef.Tree` is the root (`types.go:137`); decorators nest one node under `NodeDef.Child` (`types.go:119`), composites under `Children`. Add a decorator case to the test (a `type: cooldown` node whose `child:` holds the `topic_is` sequence).

- [ ] **Step 4: Register.** In `conditions.go` beside `keyword_match`:

```go
	conditionRegistry["topic_is"] = condTopicIs
```

In `events.go` `KnownBehaviorEvents`, after `player_give`:

```go
	"player_topic":           true, // a player chose a behavior talk topic on this mob (Text = topic key)
```

- [ ] **Step 5: Run the package tests**

Run: `go test ./internal/behaviortree/ -count=1`
Expected: PASS. If an editor-enum test lists `EventNames()` exactly, add `player_topic` to its expectation.

- [ ] **Step 6: Commit**

```bash
git add internal/behaviortree/topics.go internal/behaviortree/topics_test.go internal/behaviortree/conditions.go internal/behaviortree/events.go
git commit -F - <<'EOF'
feat(behaviortree): player_topic event and topic_is condition (#375 R12)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 8: `internal/conversation` package and `PlayerState`

**Files:**
- Create: `internal/conversation/context.md`, `internal/conversation/playerstate.go`, `internal/dialogue/testing.go`
- Modify: `internal/usercommands/talk.go` (`buildPlayerState` becomes a one-line call), `docs/README.md`
- Test: `internal/conversation/playerstate_test.go`

- [ ] **Step 1: Write `internal/dialogue/testing.go`** (an exported seam other packages' tests use; production never calls it)

```go
package dialogue

import "fmt"

// InstallForTest puts df in the load cache as if read from disk, so tests in
// other packages can give a mob a dialogue file. It returns the undo.
func InstallForTest(df *DialogueFile) func() {
	key := fmt.Sprintf("%d:%s", df.MobId, df.Zone)
	prev, had := dialogueCache[key]
	prevNil := nilSentinel[key]
	dialogueCache[key] = df
	delete(nilSentinel, key)
	return func() {
		if had {
			dialogueCache[key] = prev
		} else {
			delete(dialogueCache, key)
		}
		if prevNil {
			nilSentinel[key] = true
		}
	}
}
```

Check: `Load` builds the key from `zone` as given (`fmt.Sprintf("%d:%s", mobId, zone)`, `loader.go:45`), so a test must install with the mob's own `Zone` string.

- [ ] **Step 2: Write the failing test**

```go
package conversation

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func TestPlayerState_HeardCallbacksUseTheCharacter(t *testing.T) {
	u := users.NewTestUser(7101, "heard", "Heardia", 7101)
	ps := PlayerState(u)
	require.False(t, ps.HasHeard("94.a"))
	ps.MarkHeard("94.a")
	require.True(t, u.Character.HasHeard("94.a"))
	require.True(t, ps.HasHeard("94.a"))
}
```

- [ ] **Step 3: Run to verify failure**

Run: `go test ./internal/conversation/ -count=1`
Expected: FAIL (`no Go files` or `undefined: PlayerState`).

- [ ] **Step 4: Move the builder.** Create `internal/conversation/playerstate.go` with package `conversation`. Move the whole body of `buildPlayerState` from `internal/usercommands/talk.go:148-266` into it as `func PlayerState(user *users.UserRecord) *dialogue.PlayerState` (copy every callback and its comments unchanged; fix imports), and add the two new callbacks:

```go
		HasHeard:  user.Character.HasHeard,
		MarkHeard: user.Character.MarkHeard,
```

Rename the two log tags `talk.GiveItem` to `conversation.GiveItem`. Then in `talk.go` replace the old function with:

```go
// buildPlayerState is conversation.PlayerState; kept so the existing
// callers and tests in this package read unchanged until Task 19.
func buildPlayerState(user *users.UserRecord) *dialogue.PlayerState {
	return conversation.PlayerState(user)
}
```

Remove imports `talk.go` no longer uses (`go build ./...` lists them).

- [ ] **Step 5: Write `internal/conversation/context.md`**

```markdown
# conversation: Package Documentation

## Purpose

`internal/conversation` is the game side of talking to NPCs (quest arc #375
phase 1). It turns a player's `talk`, a topic choice, a `give` and a bonded
companion's question into `internal/dialogue` calls, speaks the lines, and
opens the menus through `internal/users`. `internal/dialogue` stays pure; this
package owns everything that needs a user, a room or a mob.

## Files

| File | What it holds |
|---|---|
| `playerstate.go` | `PlayerState(user)`: the `dialogue.PlayerState` callbacks over a character |

## Rules

- Observers see a conversation, never a menu: the player's line and the NPC's
  answer go to the room through `actions.Say`; menus, cards and hints go to the
  player only.
- Every choice re-checks eligibility right before it acts.
```

(Tasks 9 to 12 add their files to the table.) Add a `docs/README.md` row for `internal/conversation/context.md` next to the other package rows.

- [ ] **Step 6: Run the tests**

Run: `go build ./... && go test ./internal/conversation/ ./internal/usercommands/ -count=1`
Expected: PASS (the old `talk_giveitem_test.go` and `talk_gold_test.go` still go through `buildPlayerState`).

- [ ] **Step 7: Commit**

```bash
git add internal/conversation/context.md internal/conversation/playerstate.go internal/conversation/playerstate_test.go internal/dialogue/testing.go internal/usercommands/talk.go docs/README.md
git commit -F - <<'EOF'
feat(conversation): new package, PlayerState with heard callbacks (#375 phase 1)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 9: The talk flow

**Files:**
- Create: `internal/conversation/talk.go`
- Test: `internal/usercommands/conversation_talk_test.go` (the `usercommands` test package already seeds a room, users and a mob through `seedAllRegistries`; tests there may call `conversation` directly)

**Also in this task (moved from Task 7):** add `"player_topic": true, // a player chose a behavior talk topic on this mob (Text = topic key)` to `KnownBehaviorEvents` in `internal/behaviortree/events.go` and the test `TestPlayerTopicIsAKnownEvent` (`require.True(t, KnownBehaviorEvents["player_topic"])`) to `internal/behaviortree/topics_test.go`, in the same commit as `talk.go`'s `EventType: "player_topic"` dispatch. `TestEventVocabulary_MatchesDispatchSitesBothWays` fails while an event is registered with no dispatch site, so they must land together.

Output order for a topic that takes an item (owner's own example, "Here is the evidence you were looking for, Warden [hand them the purse]"): the player's line, then the handover line, then the NPC's reply. Update the spec's section 4 items 4 and 5 to this order in this task's commit.

- [ ] **Step 1: Write the fixture helper and failing tests**

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// The seeded Skeleton (MobId 1, instance 100, zone "TestZone") gets this
// dialogue file. Quest tokens are not registered; PlayerState.HasQuest
// reads the character's progress, so tests grant tokens by hand.
const talkFixtureYAML = `mobid: 1
zone: TestZone
opener:
  text: "Rattle."
topics:
  - key: bones
    kind: info
    label: "Why do your bones rattle?"
    text: "Old age."
  - key: secret
    kind: info
    label: "Tell me a secret."
    text: "The crypt key is under the altar."
    marked: true
  - key: deeper
    kind: info
    label: "What lies under the altar?"
    text: "Stairs."
    heardRequired: [secret]
`

func talkFixture(t *testing.T) (*users.UserRecord, *mobs.Mob, func()) {
	t.Helper()
	cleanup := seedAllRegistries()
	var df dialogue.DialogueFile
	require.NoError(t, yaml.UnmarshalStrict([]byte(talkFixtureYAML), &df))
	undo := dialogue.InstallForTest(&df)
	user, _ := getTestUserAndRoom(t)
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)
	return user, mob, func() { undo(); cleanup() }
}

func drained(uid int) string { return strings.Join(events.DrainQueuedMessagesForTest(uid), "\n") }

func TestTalk_OpenerThenPrivateMenu(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	room := mustRoom(t, user)
	conversation.Talk(user, room, mob)

	mine, theirs := drained(1), drained(2)
	require.Contains(t, mine, "Rattle.")
	require.Contains(t, mine, "Talking with Skeleton")
	require.Contains(t, mine, "Why do your bones rattle?")
	require.NotContains(t, mine, "What lies under the altar?", "gated on heardRequired")
	require.Contains(t, theirs, "approaches")
	require.Contains(t, theirs, "Rattle.")
	require.NotContains(t, theirs, "Talking with", "observers never see the menu")
	require.True(t, user.HasOpenMenu())
}

func TestTalk_InfoChoiceSpeaksBothLinesMarksHeardAndReshows(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	conversation.Talk(user, mustRoom(t, user), mob)
	drained(1)
	drained(2)

	// Menu order: bones, secret. "2" is the marked secret.
	require.True(t, user.RouteMenuInput("2", true))
	mine, theirs := drained(1), drained(2)
	require.Contains(t, theirs, "Tell me a secret.")
	require.Contains(t, theirs, "The crypt key is under the altar.")
	require.Contains(t, mine, `You say, "`)
	require.True(t, user.Character.HasHeard("1.secret"))
	require.True(t, user.HasOpenMenu(), "the menu is re-shown after an answer")
	require.Contains(t, mine, "What lies under the altar?", "heardRequired now met")
}

func TestTalk_StillValidFailsWhenTheNpcLeaves(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	conversation.Talk(user, mustRoom(t, user), mob)
	mob.Character.RoomId = 2
	require.True(t, user.RouteMenuInput("1", true))
	require.False(t, user.HasOpenMenu())
	require.False(t, user.Character.HasHeard("1.bones"))
}
```

Add `mustRoom` to the same file:

```go
func mustRoom(t *testing.T, u *users.UserRecord) *rooms.Room {
	t.Helper()
	r := rooms.LoadRoom(u.Character.RoomId)
	require.NotNil(t, r)
	return r
}
```

(import `github.com/GoMudEngine/GoMud/internal/rooms`). Before writing assertions about the room line, confirm that the seeded room 1 gives user 2 `messaging.SightFull` of user 1 (`messaging.ParticipantSight(users.GetByUserId(2).Character, room)`); if it does not, the lamp pin in `seedAllRegistries` is too low for names and the test must raise it locally, not weaken the assertion.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/usercommands/ -run TestTalk_ -count=1`
Expected: FAIL to compile (`undefined: conversation.Talk`).

- [ ] **Step 3: Write `internal/conversation/talk.go`**

```go
package conversation

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Talk opens a conversation with mob: the approach line and the NPC's
// opener and breadcrumbs as speech the room hears, then the private menu.
// The caller has already resolved mob by sight and refused a sleeping one.
func Talk(user *users.UserRecord, room *rooms.Room, mob *mobs.Mob) {
	room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(
		`<ansi fg="username">%s</ansi> approaches <ansi fg="mobname">%s</ansi> for a conversation.`,
		user.Character.Name, mob.Character.Name), user.UserId)

	df := dialogue.Load(int(mob.MobId), mob.Zone)
	if df == nil {
		mob.Command(`emote nods.`)
		return
	}
	ps := PlayerState(user)
	text, hints := df.OpenerFor(ps)
	if text != "" {
		npcSay(mob, room, text)
	}
	for _, b := range df.Breadcrumbs(ps) {
		npcSay(mob, room, b)
	}
	sendHints(user, hints)
	openMenu(user, mob.InstanceId)
}

// present returns the mob and room while the player can still talk to it:
// same room, full sight, and the mob perceived. It backs StillValid.
func present(user *users.UserRecord, mobInstanceId int) (*mobs.Mob, *rooms.Room, bool) {
	mob := mobs.GetInstance(mobInstanceId)
	if mob == nil || mob.Character.RoomId != user.Character.RoomId {
		return nil, nil, false
	}
	room := rooms.LoadRoom(user.Character.RoomId)
	if room == nil || messaging.ParticipantSight(user.Character, room) != messaging.SightFull ||
		!user.Character.Perceives(&mob.Character) {
		return nil, nil, false
	}
	return mob, room, true
}

func session(user *users.UserRecord, mob *mobs.Mob, onChoose func(menu.Entry)) menu.Session {
	inst := mob.InstanceId
	return menu.Session{
		Topic:      "talk with " + mob.Character.Name,
		TopicNames: []string{mob.Character.Name},
		StillValid: func() bool { _, _, ok := present(user, inst); return ok },
		OnChoose:   onChoose,
	}
}

// openMenu shows (or re-shows) the talk menu, recomputed for this moment.
// Nothing opens when the NPC is gone or has nothing to talk about.
func openMenu(user *users.UserRecord, mobInstanceId int) {
	mob, _, ok := present(user, mobInstanceId)
	if !ok {
		return
	}
	df := dialogue.Load(int(mob.MobId), mob.Zone)
	m := df.Menu(mob.Character.Name, PlayerState(user))
	if len(m.Entries) == 0 {
		return
	}
	user.OpenMenu(m, session(user, mob, func(e menu.Entry) { choose(user, mobInstanceId, e.Id) }))
}

func choose(user *users.UserRecord, mobInstanceId int, key string) {
	mob, room, ok := present(user, mobInstanceId)
	if !ok {
		return
	}
	df := dialogue.Load(int(mob.MobId), mob.Zone)
	ps := PlayerState(user)
	t := df.FindTopic(key)
	if t == nil || !df.Eligible(t, ps) {
		openMenu(user, mobInstanceId)
		return
	}
	if t.Kind == dialogue.KindOffer {
		playerSay(user, room, t.PlayerLine)
		npcSay(mob, room, t.Text)
		sendHints(user, t.Hints)
		offerCard(user, mob, t)
		return
	}
	play(user, room, mob, df, t, ps)
	openMenu(user, mobInstanceId)
}

// play speaks and applies an info or hand-in topic: the player's line, the
// handover when it takes an item, the NPC's reply, then mood and any
// behavior. When Apply refuses (a price or a delivery failed) the NPC says
// nothing more; the refusal line came from the PlayerState callback.
func play(user *users.UserRecord, room *rooms.Room, mob *mobs.Mob, df *dialogue.DialogueFile, t *dialogue.Topic, ps *dialogue.PlayerState) {
	line := t.Label
	if t.Kind == dialogue.KindHandIn {
		line = t.PlayerLine
	}
	playerSay(user, room, line)
	itemName := backpackItemName(user, t.RequiresItem)
	if !df.Apply(t, ps) {
		return
	}
	if itemName != "" {
		handoverLines(user, room, mob, itemName)
	}
	npcSay(mob, room, t.Text)
	sendHints(user, t.Hints)
	after(user, room, mob, df, t)
}

// after shifts the NPC's mood and fires a behavior topic's tree.
func after(user *users.UserRecord, room *rooms.Room, mob *mobs.Mob, df *dialogue.DialogueFile, t *dialogue.Topic) {
	dialogue.ShiftMood(mob.InstanceId, t.MoodChange, df.DefaultMood)
	if t.Behavior {
		behaviortree.TryMobBehavior(mob.InstanceId, behaviortree.EventContext{
			EventType: "player_topic",
			UserId:    user.UserId,
			Text:      t.Key,
			RoomId:    room.RoomId,
		})
	}
}

// offerCard is the private Accept / Leave after an offer's pitch (R6).
// Enter is Leave. Accept re-checks eligibility before granting; either way
// the talk menu comes back.
func offerCard(user *users.UserRecord, mob *mobs.Mob, t *dialogue.Topic) {
	inst, key := mob.InstanceId, t.Key
	card := menu.Confirm(t.Label, "Accept", "Leave")
	user.OpenMenu(card, session(user, mob, func(e menu.Entry) {
		if e.Id == "yes" {
			accept(user, inst, key)
		}
		openMenu(user, inst)
	}))
}

func accept(user *users.UserRecord, mobInstanceId int, key string) {
	mob, room, ok := present(user, mobInstanceId)
	if !ok {
		return
	}
	df := dialogue.Load(int(mob.MobId), mob.Zone)
	ps := PlayerState(user)
	t := df.FindTopic(key)
	if t == nil || !df.Eligible(t, ps) {
		user.SendText(messaging.CategoryRefusal, `That offer is no longer open.`)
		return
	}
	if df.Apply(t, ps) {
		after(user, room, mob, df, t)
	}
}

// playerSay speaks a topic line as the player: the room hears it through
// actions.Say (sight-aware names, deafen filter), the player reads their
// own line. A muted player's line stays private.
func playerSay(user *users.UserRecord, room *rooms.Room, text string) {
	if text == "" {
		return
	}
	if !user.Muted {
		actions.Say(&actions.UserActor{User: user, Room: room}, text)
	}
	user.SendText(messaging.CategorySpeech, util.SplitStringNL(
		fmt.Sprintf(`You say, "<ansi fg="saytext">%s</ansi>"`, text), 80))
}

func npcSay(mob *mobs.Mob, room *rooms.Room, text string) {
	if text != "" {
		actions.Say(&actions.MobActor{Mob: mob, Room: room}, text)
	}
}

func sendHints(user *users.UserRecord, hints string) {
	if hints != "" {
		user.SendText(messaging.CategoryDialogueHint, fmt.Sprintf(`<ansi fg="181">  [%s]</ansi>`, hints))
	}
}

func backpackItemName(user *users.UserRecord, itemId int) string {
	if itemId <= 0 {
		return ""
	}
	for _, itm := range user.Character.GetAllBackpackItems() {
		if itm.ItemId == itemId {
			return itm.DisplayName()
		}
	}
	return ""
}

// handoverLines are give.go's own lines for an item handed to a mob.
func handoverLines(user *users.UserRecord, room *rooms.Room, mob *mobs.Mob, itemName string) {
	sight := messaging.ParticipantSight(user.Character, room)
	user.SendText(messaging.CategorySystem, messaging.HideNames(
		fmt.Sprintf(`You give the <ansi fg="item">%s</ansi> to <ansi fg="mobname">%s</ansi>.`, itemName, mob.Character.Name),
		[]string{mob.Character.Name}, sight))
	room.SendTextVisual(messaging.CategoryLoot, fmt.Sprintf(
		`<ansi fg="username">%s</ansi> gave their <ansi fg="item">%s</ansi> to <ansi fg="mobname">%s</ansi>.`,
		user.Character.Name, itemName, mob.Character.Name), user.UserId)
}
```

Notes for the implementer:
- `backpackItemName` runs before `Apply`, because `Apply` removes the item.
- `PlayerState(user).GiveItem` already prints "You can't carry the ..." on a failed delivery, and `ChargeGold` prints the payment.
- If `user.Character.Perceives` or `GetAllBackpackItems` has a different name, use the one `give.go` and `talk.go` already call (`grep -n "Perceives\|GetAllBackpackItems" internal/usercommands/*.go`).

- [ ] **Step 4: Add the offer and hand-in tests** to `conversation_talk_test.go`:

```go
const offerFixtureYAML = `mobid: 1
zone: TestZone
topics:
  - key: hunt
    kind: offer
    quest: 7
    label: "The Wolf Hunt"
    playerLine: "Do you want the wolves dealt with?"
    text: "Bring me three pelts."
    questExcluded: [7-start, 7-end]
    grantsQuest: 7-start
  - key: purse
    kind: handin
    quest: 9
    label: "The Lost Purse"
    playerLine: "Here is the purse you were looking for."
    text: "My thanks."
    requiresItem: 10001
    questExcluded: [9-end]
    grantsQuest: 9-end
`
```

Write a `offerFixture(t)` like `talkFixture` that installs this YAML, and these tests:

Quest grants are queued as `events.Quest{UserId, QuestToken}` (the `GiveQuest` callback); read them with `events.DrainQueuedQuestsForTest()` (`internal/events/events.go:349`). A test that needs the player to hold a token sets the progress directly: `user.Character.QuestProgress = map[int]string{7: "start"}`. `Character.HasQuest` (`internal/characters/quests.go:64`) is true for the current step without any quest registered; `GiveQuestToken` is not usable here because it orders steps through the quest registry (`IsTokenAfter`), which these tests do not seed and which has no exported test seeder. Drain the quest queue at the start of each test.

- `TestOffer_PitchThenCardLeaveGrantsNothing`: `conversation.Talk`, `RouteMenuInput("1", true)`; user 2's text contains the player line and "Bring me three pelts."; user 1's text contains `Accept` and `Leave`; `RouteMenuInput("", true)` (Enter = Leave); `events.DrainQueuedQuestsForTest()` holds no `7-start`; `user.HasOpenMenu()` is true (the talk menu came back).
- `TestOffer_AcceptGrantsOnce`: choose `1`, then `1` (Accept); the drained quests hold exactly one `7-start` for user 1; then set `QuestProgress[7] = "start"`, re-open with `conversation.Talk`, and the menu text no longer contains `The Wolf Hunt`.
- `TestOffer_AcceptAfterEligibilityLostGrantsNothing`: choose `1`, then set `QuestProgress[7] = "start"`, then Accept; user 1's text contains "That offer is no longer open." and no `7-start` is queued.
- `TestTalk_SecondNpcReplacesTheMenu`: seed a second mob instance in room 1 (copy the Skeleton seed as instance 101, MobId 2 "Merchant", with its own installed file); talk to the Skeleton, then to the Merchant; one menu is open, headed `Talking with Merchant`, and choosing `1` acts on the Merchant's topic.
- `TestHandIn_LineThenHandoverThenReply`: give user 1 item 10001 (`items.New(10001)` + `user.Character.StoreItem`), choose the hand-in; user 1's text shows, in this order, `You say, "Here is the purse` then `You give the` then `My thanks.`; the item is gone from the backpack.

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/usercommands/ -run 'TestTalk_|TestOffer_|TestHandIn_' -count=1 -v`
Expected: PASS.

- [ ] **Step 6: Commit** (also edit spec section 4 items 4 and 5 to "player line, handover line, NPC reply")

```bash
git add internal/conversation/talk.go internal/usercommands/conversation_talk_test.go docs/superpowers/specs/2026-10-20-quest-arc-phase1-talk-design.md internal/conversation/context.md
git commit -F - <<'EOF'
feat(conversation): talk flow, offer card, hand-ins, menu re-show (#375 phase 1)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

**Follow-ups from the Task 11 review, to apply once the Task 9 fixes land (they touch `talk.go`):** charmed NPCs offer no topics (controller ruling, matching the marker's charm exclusion): `conversation.Talk` on a charmed mob opens nothing beyond today's approach line and nod, and `conversation.Give` returns false for a charmed mob; test both. After `MarkHeard` and after a grant in the talk flow, call `conversation.RefreshMarkers(user.UserId)` so the GMCP NPC marker is not stale.

### Task 10: `give` resolves as the topic

**Files:**
- Create: `internal/conversation/give.go`
- Test: `internal/usercommands/conversation_give_test.go`

- [ ] **Step 1: Write the failing tests** (reuse `offerFixtureYAML` from Task 9)

- `TestGiveResolve_EligibleHandInPlaysTheTopic`: give user 1 item 10001; `handled := conversation.Give(user, room, mob, itm)` returns true; the item left the backpack; user 1's text has the player line, the `You give the` line and `My thanks.`, in that order; the mob's inventory does NOT hold the item (`mob.Character.Items` has no 10001).
- `TestGiveResolve_WantedButIneligibleIsHandedBack`: `user.Character.QuestProgress = map[int]string{9: "end"}` (topic hidden), give item 10001: returns true, the item is still in user 1's backpack, user 1's text contains `hands it back`, the mob holds nothing.
- `TestGiveResolve_UnwantedItemIsNotHandled`: an item no topic wants returns false and changes nothing (the caller's ordinary give runs).
- `TestGiveResolve_TwiceCannotCompleteTwice`: two copies of item 10001; first give completes and queues `9-end`; set `QuestProgress[9] = "end"` (the queued event is what would do it in the game); the second give is handed back and the second copy stays in the backpack.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/usercommands/ -run TestGiveResolve_ -count=1`
Expected: FAIL to compile.

- [ ] **Step 3: Write `internal/conversation/give.go`**

```go
package conversation

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Give resolves `give <item> <npc>` against the NPC's topics before any
// transfer (R8). An eligible topic wanting the item plays exactly as if
// chosen from the menu. A wanted item with no eligible topic is handed back
// and never leaves the player. It returns false when no topic on the NPC
// wants the item, and the caller runs the ordinary give.
func Give(user *users.UserRecord, room *rooms.Room, mob *mobs.Mob, itm items.Item) bool {
	df := dialogue.Load(int(mob.MobId), mob.Zone)
	ps := PlayerState(user)
	t, wanted := df.ItemTopic(itm.ItemId, ps)
	if !wanted {
		return false
	}
	if t == nil {
		sight := messaging.ParticipantSight(user.Character, room)
		user.SendText(messaging.CategorySystem, messaging.HideNames(fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> looks at the <ansi fg="item">%s</ansi> and hands it back.`,
			mob.Character.Name, itm.DisplayName()), []string{mob.Character.Name}, sight))
		return true
	}
	play(user, room, mob, df, t, ps)
	openMenu(user, mob.InstanceId)
	return true
}
```

`play` removes one backpack copy of `t.RequiresItem` through `PlayerState.RemoveItem`. That is the item the player named unless they carry several copies, which are identical by item id, so the result is the same. If `give.go` lets a player give a worn or wielded item (check how `giveItem` is found in `give.go`), only a backpack copy satisfies `requiresItem` (`HasItem` is backpack-only), so `ItemTopic` reports it ineligible and the item is handed back: state that in the test name and keep it.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/usercommands/ -run TestGiveResolve_ -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/conversation/give.go internal/usercommands/conversation_give_test.go internal/conversation/context.md
git commit -F - <<'EOF'
feat(conversation): give resolves as the topic, early items handed back (#375 R8)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 11: Markers replace the dead quest star

**Files:**
- Create: `internal/rooms/npcmarker.go`, `internal/conversation/marker.go`
- Modify: `internal/characters/formattedname.go:46,97-99,178`, `internal/actions/perception.go:46`, `internal/hooks/spell_resolution.go:474`, `internal/rooms/roomdetails.go:294-299`, `modules/gmcp/gmcp.Room.go:399-404,613`, delete `internal/mobs/mobs.go:122` `QuestFlags`, `internal/rooms/spawninfo.go:29` `QuestFlags`, `internal/rooms/rooms.go:1139-1141`, `modules/gmcp/gmcp.Mob.go:266,390` (the `QuestFlags` fields), `_datafiles/html/public/static/js/mobs.js:323,696,753`, `_datafiles/html/public/build.html:727,923,966`, `_datafiles/html/admin/mobs/index.html:17`, `_datafiles/html/admin/mobs/mob.data.html:70-72`
- Test: `internal/characters/formattedname_marker_test.go`, `internal/usercommands/conversation_marker_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package characters

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormattedName_MarkerLeadsTheName(t *testing.T) {
	f := FormattedName{Name: "warden velk", Type: "mobname", Marker: "[?]"}
	out := f.String()
	require.True(t, strings.HasPrefix(out, `<ansi fg="questflag">[?]</ansi> `), out)
	f.Marker = ""
	require.NotContains(t, f.String(), "questflag")
}
```

In `internal/usercommands/conversation_marker_test.go`, with `talkFixtureYAML` installed (Task 9):
- `TestMarker_RosterShowsInfoMarkerUntilHeard`: `rooms.LoadRoom(1).GetRoomDetails(user)` (find the real roster call with `grep -n "func (r \*Room) GetRoomDetails" internal/rooms/roomdetails.go`) has an entry containing `[*]` and `Skeleton`; after `user.Character.MarkHeard("1.secret")` it has none.
- `TestMarker_NoMarkerWithoutFullSight`: move user 1 and the Skeleton to the seeded dark room 2 (biome `cave`, sky light 0); the roster for user 1 contains no `[*]` (figures replace names at Shapes or None sight).
- `TestMarker_GMCPNpcCarriesTheMarker`: if GMCP payload building is reachable from a test (`grep -n "func.*Room.Info\|GetRoomInfo\|buildRoomPayload" modules/gmcp/gmcp.Room.go`), assert `Marker == "[*]"`; otherwise test the shared `rooms.NpcMarker(1, mob)` hook value, which GMCP reads.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/characters/ -run TestFormattedName_Marker -count=1`
Expected: FAIL to compile (`unknown field Marker`).

- [ ] **Step 3: FormattedName.** Replace the `QuestAlert bool` field with:

```go
	Marker             string // the viewer's talk marker, "[!]", "[?]", "[*]" or "" (#375)
```

Replace the render block at lines 97-99 with:

```go
	if f.Marker != `` {
		output = `<ansi fg="questflag">` + f.Marker + `</ansi> ` + output
	}
```

Replace `f.QuestAlert = false` with `f.Marker = ``` at `formattedname.go:178`, `actions/perception.go:46` and `hooks/spell_resolution.go:474`. Run `go build ./...`; fix any other `QuestAlert` the compiler names the same way.

- [ ] **Step 4: The hook.** `internal/rooms/npcmarker.go`:

```go
package rooms

import "github.com/GoMudEngine/GoMud/internal/mobs"

// NpcMarker returns the talk marker a viewer sees before mob's name. It is
// installed by internal/conversation (which can build the viewer's dialogue
// state); nil means no markers.
var NpcMarker func(viewerUserId int, mob *mobs.Mob) string
```

In `roomdetails.go` replace the `for _, qFlag := range mob.QuestFlags {...}` block with:

```go
			if NpcMarker != nil {
				mobName.Marker = NpcMarker(user.UserId, mob)
			}
```

`internal/conversation/marker.go`:

```go
package conversation

import (
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func init() {
	rooms.NpcMarker = Marker
}

// Marker is the talk marker viewerUserId sees on mob: [?], [!], [*] or "".
// The roster and GMCP call it only for viewers who see the mob by name.
func Marker(viewerUserId int, mob *mobs.Mob) string {
	user := users.GetByUserId(viewerUserId)
	if user == nil || mob == nil || mob.Character.IsCharmed() {
		return ""
	}
	return dialogue.Load(int(mob.MobId), mob.Zone).Marker(PlayerState(user))
}
```

The `init` only runs when something imports `conversation`. `usercommands` does from Task 8; confirm `main.go` imports `usercommands` (it registers commands), so the hook is installed in the server.

In `gmcp.Room.go`, replace the `QuestFlag bool` field of `GMCPRoomModule_Payload_Contents_Character` with `Marker string \`json:"marker"\`` and the `if len(mob.QuestFlags) > 0 {...}` block with:

```go
			if rooms.NpcMarker != nil {
				c.Marker = rooms.NpcMarker(user.UserId, mob)
			}
```

Leave the item payload's `QuestFlag` (items' quest token star) alone.

- [ ] **Step 5: Delete the dead source.** Remove `Mob.QuestFlags`, `SpawnInfo.QuestFlags`, the copy at `rooms.go:1139-1141`, the two `QuestFlags` uses in `gmcp.Mob.go` and their JSON field in the mob edit payload. In the builder: remove the "Quest flags" chips field and its gather in `mobs.js` (lines 696, 753, and the word in the comment at 323), the `gatherQuestFlags` field, its use at 966 and the comment word at 727 in `build.html`, the star span in `admin/mobs/index.html:17`, and the quest-flags textarea group in `admin/mobs/mob.data.html:70-72`. Then `grep -rn "questflags\|questFlags\|QuestFlags" --include=*.go --include=*.js --include=*.html internal modules _datafiles/html` must show only `Character.QuestFlags` (the character's quest flags, a different thing), dialogue/quest flag code, and nothing about mobs or spawns.

- [ ] **Step 6: Run**

Run: `go build ./... && go test ./internal/characters/ ./internal/rooms/ ./internal/usercommands/ ./modules/gmcp/ -count=1`
Expected: PASS. The unknown-YAML-key drift gate (`boot_smoke_test.go`) loads the `dogmud` world, which sets no `questflags` on mobs or spawns.

- [ ] **Step 7: Commit**

```bash
git add internal/characters/formattedname.go internal/characters/formattedname_marker_test.go internal/actions/perception.go internal/hooks/spell_resolution.go internal/rooms/npcmarker.go internal/rooms/roomdetails.go internal/rooms/rooms.go internal/rooms/spawninfo.go internal/mobs/mobs.go internal/conversation/marker.go internal/conversation/context.md internal/usercommands/conversation_marker_test.go modules/gmcp/gmcp.Room.go modules/gmcp/gmcp.Mob.go _datafiles/html/public/static/js/mobs.js _datafiles/html/public/build.html _datafiles/html/admin/mobs/index.html _datafiles/html/admin/mobs/mob.data.html
git commit -F - <<'EOF'
feat(markers): per-viewer talk markers replace the unused quest star (#375 R9)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 12: Companion asks through info topics

**Files:**
- Create: `internal/conversation/companion.go`
- Modify: `modules/aicompanion/commands.go` (`askAuthority`, `cmdAskFor`), `modules/aicompanion/actions.go:474-476`
- Test: `internal/usercommands/conversation_companion_test.go`, the module's existing ask-authority tests (`grep -ln "askAuth" modules/aicompanion/*_test.go`)

- [ ] **Step 1: Write the failing tests**

In `conversation_companion_test.go` with `talkFixtureYAML` installed:
- `TestCompanionTopic_MatchesKeyOrAllLabelWords`: `conversation.CompanionTopic(user, mob, "bones")` returns `"bones"`; `"secret tell"` returns `"secret"` (every word in the label); `"crypt"` returns `""` with ok=false; `"the"` (matches several) returns ok=false.
- `TestCompanionTopic_NeverOffersOrHandIns`: with `offerFixtureYAML`, `"wolf hunt"` returns ok=false.
- `TestAskForOwner_PlaysTheInfoTopic`: `conversation.AskForOwner(1, 100, "secret", true)` returns true, room user 2 hears `The crypt key is under the altar.`, and user 1 has heard `1.secret`.
- `TestAskForOwner_RefusesNonInfoKeys`: with `offerFixtureYAML`, `AskForOwner(1, 100, "hunt", true)` returns true, `events.DrainQueuedInputsForTest(100)` holds `emote shrugs.`, and `events.DrainQueuedQuestsForTest()` holds nothing.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/usercommands/ -run 'TestCompanionTopic_|TestAskForOwner_' -count=1`
Expected: FAIL to compile.

- [ ] **Step 3: Write `internal/conversation/companion.go`**

```go
package conversation

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// CompanionTopic resolves an owner's `companion-ask <who> about <what>`
// words to one of the NPC's eligible info topics (R13): the key itself, or
// the one label containing every word typed. Offers and hand-ins are never
// candidates: accepting a quest or handing over an item is the owner's own
// act. labels lists the eligible info labels for the refusal line.
func CompanionTopic(user *users.UserRecord, mob *mobs.Mob, what string) (key string, labels []string, ok bool) {
	df := dialogue.Load(int(mob.MobId), mob.Zone)
	ps := PlayerState(user)
	words := strings.Fields(strings.ToLower(what))
	var hits []string
	for i := range df.Topics {
		t := &df.Topics[i]
		if t.Kind != dialogue.KindInfo || !df.Eligible(t, ps) {
			continue
		}
		labels = append(labels, t.Label)
		if strings.EqualFold(t.Key, strings.TrimSpace(what)) {
			return t.Key, labels, true
		}
		label := strings.ToLower(t.Label)
		all := len(words) > 0
		for _, w := range words {
			if !strings.Contains(label, w) {
				all = false
				break
			}
		}
		if all {
			hits = append(hits, t.Key)
		}
	}
	if len(hits) == 1 {
		return hits[0], labels, true
	}
	return "", labels, false
}

// AskForOwner plays an info topic a companion was authorized to ask about,
// exactly as the owner choosing it. key comes from CompanionTopic at
// command time, never from the model's words. Installed as the companion
// NPC asker in Task 19.
func AskForOwner(ownerUserId int, mobInstanceId int, key string, authorized bool) bool {
	user := users.GetByUserId(ownerUserId)
	mob := mobs.GetInstance(mobInstanceId)
	if user == nil || user.Character == nil || mob == nil || !authorized {
		return false
	}
	if mob.Character.IsCharmed() || mob.Character.RoomId != user.Character.RoomId {
		return false
	}
	room := rooms.LoadRoom(mob.Character.RoomId)
	if room == nil {
		return false
	}
	df := dialogue.Load(int(mob.MobId), mob.Zone)
	ps := PlayerState(user)
	t := df.FindTopic(key)
	if t == nil || t.Kind != dialogue.KindInfo || !df.Eligible(t, ps) {
		mob.Command(`emote shrugs.`)
		return true
	}
	if df.Apply(t, ps) {
		npcSay(mob, room, t.Text)
		after(user, room, mob, df, t)
	}
	return true
}
```

The companion already spoke its own words aloud through `sayto` before `AskNpc` runs (`actions.go:466`), so no player line is spoken here. An unauthorized ask does nothing: today's unauthorized path only let an NPC answer without touching quests, and with topics there is no free text to answer.

- [ ] **Step 4: The module.** In `modules/aicompanion/commands.go` add a `TopicKey string` field to `askAuthority`. In `cmdAskFor`, after `target` is resolved and before `c.askAuth = ...`:

```go
	key, labels, ok := conversation.CompanionTopic(user, target, topic)
	if !ok {
		if len(labels) == 0 {
			user.SendText(messaging.CategoryRefusal, `They have nothing to tell your companion right now.`)
		} else {
			user.SendText(messaging.CategoryRefusal, `Your companion can ask about: `+strings.Join(labels, ` / `))
		}
		return true, nil
	}
```

and store `TopicKey: key` in the new `askAuthority`. In `actions.go:475` pass `c.askAuth.TopicKey` instead of `a.Query`:

```go
			companionai.AskNpc(owner.UserId, t.MobInstanceId, c.askAuth.TopicKey, true)
```

Keep `valid(...)`, which still checks the model's words against the owner's topic text. Read `valid` (`commands.go:581`) before editing; do not loosen it.

- [ ] **Step 5: Run**

Run: `go build ./... && go test ./internal/usercommands/ -run 'TestCompanionTopic_|TestAskForOwner_' -count=1 && go test ./modules/aicompanion/ -count=1`
Expected: PASS. Update any aicompanion test that asserted the query text reached `AskNpc`; it must now assert the topic key.

- [ ] **Step 6: Commit**

```bash
git add internal/conversation/companion.go internal/conversation/context.md internal/usercommands/conversation_companion_test.go modules/aicompanion/commands.go modules/aicompanion/actions.go
git commit -F - <<'EOF'
feat(conversation): companion-ask picks an info topic by key (#375 R13)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

(The module change only takes effect once Task 19 installs `AskForOwner` as the asker; until then the old asker receives a key, which is harmless because the module is off by default. If a module test drives the old asker end to end, move Step 4's `actions.go` edit into Task 19.)

### Task 12b: Ferry and instance-zone actions take topic parameters (R14)

**Why:** `actBoardFerry` (`internal/behaviortree/actions_ferry.go`) finds the destination by substring in `Event.Text`, and `actOpenInstancePortal` (`actions_mob.go`, around line 440) parses `"<zone> <gold>"` from it. Under `player_topic`, `Text` is a bare topic key (`^[a-z0-9_]+$`), so both would silently stop working after Task 17 (Task 7 review finding).

**Files:** `internal/behaviortree/actions_ferry.go`, `internal/behaviortree/actions_mob.go`, a new player-facing amount prompt (see below), tests beside each.

- **Ferry:** `board_ferry` gains an optional `destination: <keyword>` param. When set it is used instead of scanning `Event.Text`; it must name a key of `routes` (else Failure with an error log). Task 17 gives each ferry hand one topic per destination ("Take me to Stillwater.", key `to_stillwater`) and one branch per topic with `destination:`. Test: a `player_topic` event with `destination: stillwater` boards the stillwater route; an unknown destination fails; the old Text path still works for any remaining caller.
- **Instance zones:** `open_instance_portal` gains an optional `zone: <shortname>` param. With `zone` set and no amount in `Event.Text`, it does not open anything: it starts a private amount prompt for the player through the existing prompt flow (`users.(*UserRecord).StartPrompt(command, rest)` re-runs a command with the answer: read `internal/users/userrecord.go:791` and `internal/prompt/prompt.go` first, and follow how an existing player command asks a free-text question). The question shows the zone's `guidance` param (authored text, several lines) and asks "How much gold will you stake?". A whole number at or above `min_gold` opens the portal exactly as today (same charge, clone, exit, lines, gold scaling); anything else gets one refusal line and closes the prompt. Factor the existing open-portal body into one function both paths call, so the typed path and the prompt path cannot drift. Check open issue #641 (free-text prompt steps swallow typed commands): the amount prompt must not swallow a movement command; a non-numeric answer closes the prompt and, if it is a known command, runs as a command (match whatever #641's fix direction is if it has landed; otherwise close and run, and test it).
- Tests: a `player_topic` with `zone: arena` and no amount starts the prompt and shows the guidance; answering `250` opens the arena with the same charge and lines as the typed path; answering `north` closes the prompt and runs `north`; an amount below `min_gold` refuses.
- Task 17 then writes, per zone, the topic ("Open a way to the arena.") and its `guidance:` text: arena (spec R26: 100 to 200 midgame solo, 200 to 400 endgame solo, 400 and up for the bravest or groups, plus the no-second-chance warning; drafted in `behaviors/thornwall_city/315-sable.yaml` comments), Planar Oasis (owner's ranges: 100 to 300 midgame solo, 300 to 600 endgame solo, 600 and up for only the bravest solo players or for groups) and the crash site. Guidance for the arena and the crash site is drafted in the owner's style and shown to the owner as samples.
- Commit: `feat(behaviortree): ferry destination and instance-zone amount prompt for talk topics (#375 R14)`.

# Stage B: convert the content on the branch

Stage B never runs the converted files in the game: the commands still use the old engine until Task 19. The converted files are written into the repo in Task 15 only after the zone passes, and land with Stage C.

### Task 13: The one-time converter

**Files:**
- Create: `cmd/dialogueconvert/main.go`, `cmd/dialogueconvert/legacy.go`, `cmd/dialogueconvert/convert.go`, `cmd/dialogueconvert/convert_test.go`, `cmd/dialogueconvert/testdata/` fixtures

The converter keeps its own copy of the old types (`legacy.go`), so deleting them from `internal/dialogue` in Task 21 does not break it before Task 24 deletes the converter. It writes to an output folder outside the repo and never edits repo files.

- [ ] **Step 1: `legacy.go`.** Copy `Pattern`, `TreeNode`, `TreeRoot`, `QuestGreeting`, `Tree`, `MemoryConfig` from `internal/dialogue/types.go` verbatim, renamed with a `legacy` prefix (`legacyPattern`, ...), plus:

```go
type legacyFile struct {
	MobId       int                 `yaml:"mobid"`
	Zone        string              `yaml:"zone"`
	DefaultMood string              `yaml:"defaultMood"`
	Greetings   []dialogue.Greeting `yaml:"greetings,omitempty"`
	Patterns    []legacyPattern     `yaml:"patterns"`
	Tree        *legacyTree         `yaml:"tree,omitempty"`
	Memory      legacyMemoryConfig  `yaml:"memory"`
}
```

- [ ] **Step 2: Write the failing tests** in `convert_test.go`, each over a small fixture in `testdata/`:

| Test | Fixture shape | Asserts |
|---|---|---|
| `TestConvert_NodeKinds` | nodes granting `7-start`, `7-end`, `7-pelts`, and a plain node | kinds offer, handin, info+`marked`, info; `quest: 7`; gates and effects copied; offer/handin `label` = the quest title from the quest fixture |
| `TestConvert_RequiresToHeardRequired` | node B `requires: [A]` (A a node id); node D `requires: [X]` where node C `unlocks: [X]` | B `heardRequired: [a]`; D `heardRequired: [c]`; report lists D's mapping |
| `TestConvert_RequiresOnAnOfferIsReported` | `requires` naming a grant node | report line `requires names an offer`, no `heardRequired` written |
| `TestConvert_RootAndVariants` | root text, a plain variant, a variant with `grantsQuest` | opener text and one variant; the granting variant became an offer topic |
| `TestConvert_PatternClasses` | one each: `keywords: [""]`, greeting-only, overlapping a tree trigger, other with two responses | catch-all and overlap dropped, greeting dropped (tree file), other became an unmarked info topic whose `text` holds both responses joined by `\n---\n` for the zone pass to fold; report counts 1/1/1/1 |
| `TestConvert_TreelessOpener` | no tree; a greeting pattern and a catch-all | opener = the greeting's first response; catch-all dropped |
| `TestConvert_ChargesGetGoldRequired` | `chargesGold: 10`, `goldRequired: 0` | `goldRequired: 10` |
| `TestConvert_ItemGiveMergesIntoTopic` | topic wanting item 30 on mob 94; a quest `item_give` (mob 94, item 30, grant `10-report`, `npc_say`, `bump_rep`) | topic unchanged except `bumpsRep` appended; report holds the `npc_say` line for the zone pass; trigger listed for deletion |
| `TestConvert_ItemGiveWithNoTopicCreatesOne` | `item_give` on a mob with no topic wanting the item | new topic `give_<item>`, kind from the grant token, `questRequired` from `has`, `questExcluded` from `missing` plus the grant and end token, `requiresItem`, effects mapped, text from `npc_say` |
| `TestConvert_UnmappableActionsAreReported` | `item_give` with an `actor` action | report line naming quest, trigger index and action |
| `TestConvert_DialogueTrigger` | quest `event: dialogue` on mob 9491 | new marked info topic `asked_28` with the trigger's gates and grant; trigger listed for deletion |
| `TestConvert_BehaviorAskBranches` | a behavior file with one speech-only `player_ask` branch and one `board_ferry` branch | two info topics: the speech one with `text` from its `say`, the ferry one with `behavior: true`; report lists both branches for Task 17 |
| `TestConvert_LabelsLeftEmptyForTheZonePass` | any info topic | `label: ""` |
| `TestConvert_ReconciliationCounts` | the fixtures together | `report.Grants` totals by source equal the topics carrying those grants |

- [ ] **Step 3: Run to verify failure**

Run: `go test ./cmd/dialogueconvert/ -count=1`
Expected: FAIL to compile.

- [ ] **Step 4: Write `convert.go`.** The rules, in order (spec section 6.1):

```go
package main

// greetingWords are pattern keywords that only greet.
var greetingWords = map[string]bool{
	"hello": true, "hi": true, "greet": true, "greeting": true, "greetings": true,
	"hey": true, "hail": true, "howdy": true, "welcome": true,
	"good day": true, "good morning": true, "good evening": true,
}

// patternClass is "catchall", "greeting", "overlap" or "other".
func patternClass(p legacyPattern, triggers map[string]bool) string {
	var kws []string
	for _, k := range p.Keywords {
		if k = strings.ToLower(strings.TrimSpace(k)); k != "" {
			kws = append(kws, k)
		}
	}
	if len(kws) == 0 {
		return "catchall"
	}
	allGreet := true
	for _, k := range kws {
		if !greetingWords[k] {
			allGreet = false
		}
	}
	if allGreet {
		return "greeting"
	}
	for _, k := range kws {
		if triggers[k] {
			return "overlap"
		}
	}
	return "other"
}

// kindOf classifies a node by its grant token: "-start" offer, "-end"
// hand-in, any other grant a marked info topic (a mid-quest step), none an
// unmarked info topic.
func kindOf(grant string) (kind string, questId int, marked bool) {
	if grant == "" {
		return dialogue.KindInfo, 0, false
	}
	id, step := quests.TokenToParts(grant)
	switch step {
	case "start":
		return dialogue.KindOffer, id, false
	case "end":
		return dialogue.KindHandIn, id, false
	}
	return dialogue.KindInfo, 0, true
}
```

Write the rest of `convert.go` to these rules (each one is a test in Step 2):

1. **Nodes** become topics: `key` = slug of the node id (lowercase, non `[a-z0-9_]` to `_`); `kind`, `quest`, `marked` from `kindOf`; every gate and effect field copied; `text`, `hints`, `moodChange` copied; offer and hand-in `label` = the quest's `name`, info `label` empty, `playerLine` empty.
2. **`requires`**: for each id, if it is a node id that became an info topic, add that topic's key to `heardRequired`. If it is not a node id, find the nodes whose `unlocks` contain it and use the first (report the mapping, and every case with more than one). If it names an offer or hand-in node, write nothing and report `requires names an offer` (the zone pass replaces it with the right `questRequired`).
3. **Root**: `opener.text`/`hints` from the root; each variant without effects becomes an opener variant (gates copied); a variant with any effect becomes a topic through rule 1 (by its grant) and is reported.
4. **Patterns** by `patternClass` with the file's tree triggers: `catchall` dropped (except: in a file with no tree and no greeting pattern, its first response becomes `opener.text`); `greeting` dropped in a file with a tree, and in a file without one the first greeting pattern's first response becomes `opener.text`; `overlap` dropped; `other` becomes an info topic through rule 1 with `key` = slug of its first keyword (suffix `_2`, `_3` on a clash) and `text` = its responses joined by `"\n---\n"`.
5. **`chargesGold`**: raise `goldRequired` to at least the price.
6. **Quest triggers** (read every `quests/*.yaml` into `quests.Quest`):
   - `item_give` (mob M, item I): the converted file of M gets the merge. If a topic has `requiresItem == I`, append mapped effects to it: `bump_rep` to `bumpsRep`, `set_flag` to `setsQuestFlag` (report a clash), `give_item` to `givesItem` (report a clash), `give_gold` to `givesGold` (sum), `consume_item` dropped (the topic takes `requiresItem`), `grant` must equal the topic's `grantsQuest` (report a mismatch), `npc_say` text recorded in the report for the zone pass. Any other action (`actor` and the rest) is reported as unmappable. If no topic wants I, create `give_<I>` by rule 1 from the grant, with `questRequired` = `has`, `questExcluded` = `missing` plus the grant token and the quest's end token, `requiresItem: I`, mapped effects, `text` = the `npc_say` line.
   - `dialogue` (mob M): create a marked info topic `asked_<questId>` with the same gate mapping and the grant.
   - Every converted trigger is listed in the report as `delete quest <file> trigger <index>` for Task 16.
7. **Behavior files** (`behaviors/**/<mobId>-*.yaml`): for each node with `event: player_ask`, create an info topic on that mob: `key` = slug of its `keyword_match` first keyword (or `ask_<n>`), `text` = the first `say`/`respond`/`send_user_text` text in the branch, `questRequired` from any `player_has_quest` condition, `behavior: true` when the branch has any action other than `say`, `respond`, `send_user_text`, `emote`. A branch with `grant_quest` becomes an offer by rule 1 from its quest token instead. Every branch is listed in the report for Task 17 with its file, index, chosen key and planned edit.
8. **Files with no dialogue file** (behavior-only mobs): create one. The zone folder is `zoneNameSanitize(<the mob spec's zone:>)` read from `mobs/**/<mobId>-*.yaml`; report each new file.
9. Every `label` and `breadcrumb` the zone pass must write is left empty; `defaultMood` and `greetings` are copied unchanged; `memory:` is dropped.

`main.go` flags: `-data _datafiles/world/dogmud` (input root) and `-out <dir>` (required; refuse a path inside the repo). It writes `<out>/dialogue/<zone>/<mobId>.yaml` with `yaml.Marshal` of `dialogue.DialogueFile` and `<out>/report.md` with sections: Counts (patterns by class, nodes by kind, grants by source with component and total), Requires mappings, Unmappable actions, Merge lines (`npc_say` text to reconcile, per topic), Triggers to delete, Behavior branches, New files.

- [ ] **Step 5: Run the tests**

Run: `go test ./cmd/dialogueconvert/ -count=1 -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/dialogueconvert
git commit -F - <<'EOF'
feat(cmd): one-time dialogue-to-topics converter (#375 phase 1)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 14: Run the converter, reconcile, owner decisions

- [ ] **Step 1: Run into the scratchpad**

```bash
go run ./cmd/dialogueconvert -data _datafiles/world/dogmud -out "<scratchpad>/convert1"
```

- [ ] **Step 2: Check the counts against the spec's facts table, by component and by total.** Expected from source (2026-10-20): 302 input files; 1,086 nodes; patterns 299 catch-all / 314 greeting / 326 overlap / 253 other; grants 77 start + 62 end + 28 mid + 3 variant + 33 `item_give` + 1 `dialogue` + 2 behavior `grant_quest`. Every grant maps to exactly one topic. Any mismatch: stop, find which rule miscounted, fix it with a test, re-run. Do not hand-edit the output to make a count agree.

- [ ] **Step 3: Bring the owner one short page of decisions** (a published page or the chat, per `dogmud-player-copy` and the global "bring decisions" rule): every unmappable action (the 5 `actor` actions at least), every `requires names an offer`, every multi-unlock mapping, every clash. Each item: player-facing effect, options, a recommendation. Record each ruling in the report and apply it by changing the converter (with a test) or, for one-offs, in the Task 15 zone brief.

- [ ] **Step 4: Re-run** after converter fixes and keep the final output folder path for Task 15.

**Owner rulings for Task 14 (2026-10-20), applied in Tasks 15 to 17 and recorded as spec R15 to R21:**
- Malk 15 / Delia 16: offer, Accept, hand in; give before accepting hands back; delete the chain triggers that complete them on `-start` (live bug) and the item_give triggers.
- Drop every hand-in narration (`actor`) line; delete those item_give triggers.
- Hadwen: one `begin` behavior topic (questRequired [30-start], questExcluded [30-end]); the rite tree keeps granting 30-end and stops granting 29-start; replace quest 29 with two new bridge quests offered by Hadwen after 30-end (Toke's road: ends at Toke; Esk's road: ends at Esk or the arch), each a normal offer topic; retire quest 29 (keep its file only if saves hold 29 tokens: check `_datafiles/world/dogmud/users/*.yaml` for `29:` progress and keep a no-op end for them).
- Behavior topics close the menu (code change in `internal/conversation/talk.go`: after a `behavior: true` topic, do not call openMenu; test it). Do it in Task 19 if not before.
- Bandit leader 254: delete the ask branch and its topic; his dialogue stays as info only.
- Tutorials: quest 28's room 6261 lesson becomes `talk dewey` and choosing the world topic; grep `-i "ask "` across `_datafiles/world/dogmud` (dialogue, quests, rooms, help, templates) and rewrite every instruction to type ask, or drop it (98 lines in 36 files at last count).
- Toke: remove the seven grants and three give_items from quest 31's `quest_granted 31-end` trigger; Toke gets seven offer topics (32, 35, 38, 41, 44, 47, 50), each gated on 31-end and excluded on its own start and end; each starter item moves onto the offer of the quest it belongs to (`givesItem`). Each trail's existing trailhead offer stays.
- Quests 14, 63, 65, 67: delete the item_give trigger, keep the topic's `-end` grant, fold npc_say text. Quest 66: one hand-in topic granting 66-end with setsQuestFlag 66-addict-fate saved; delete the trigger and its chain. Mobs 95, 96: replace the offer `requires` with questRequired [9-start] / [10-start].

### Task 15: Zone passes (labels, lines, text)

One subagent per zone batch (34 zone folders; batch the small ones: about 8 batches, the largest being `the_confluence` 67, `greenford` 38, `pothole_coulee` 29, `stillwater` 22). Model: Opus (judgement: voice and copy).

**The new-player start gets extra validation (owner 2026-10-21).** Batch 1 is `pothole_coulee` plus `newcomer_antechamber` (Dewey, quest 28). Where players start out, a mistake costs the most, so this batch adds, on top of the steps below:
- The owner reviews every topic in the batch, not five samples: a published page per NPC listing label, player line, reply and gates in plain words.
- A scripted gate walk over the converted files, from a fresh character's quest state: every quest in the batch can be offered and completed, nothing is re-offered after its end, no topic is a dead end. It is a program, not a reading, and it runs for every later batch too.
- An adversarial review by a separate Opus subagent (not the writer): copy rules, voice, gates, and that every tutorial instruction names a command that exists after Task 19.
- Task 26 carries a dedicated new-player scenario (goal 7).

**Batch 2 (Thornwall: thornwall_city, thornwall_outskirts, marches_spur_road) APPROVED by the owner 2026-10-21** (page https://claude.ai/artifact/PBtUK4GZsQHH9dTgNBvdWN; rulings R26; reviews in `C:/tmp/dogmud-375-convert/pages/`).

**Batch 1 APPROVED by the owner 2026-10-21** after three review rounds (page https://claude.ai/artifact/MJ2dj1f7Fahi119VcUXG33, version 3). Its rulings became spec R22 to R25. Working copy: `C:/tmp/dogmud-375-convert/dialogue/{pothole_coulee,newcomer_antechamber}`; gate walk `go run ./cmd/dialoguecheck -data _datafiles/world/dogmud -dialogue C:/tmp/dogmud-375-convert/dialogue -zones <zones> -retired 29`; page generator `C:/tmp/dogmud-375-convert/review_page.py`.

- [ ] **Step 1: Brief each subagent with:**
  - The global Edit/Write rule; files live in the scratchpad output folder, edited with Edit.
  - Read in chunks of at most 400 lines; report "read N of N files" with the list.
  - The nine menu language conventions (`internal/menu/context.md`) and `dogmud-player-copy` (80-column wrap, no raw numbers, ESL-clear, no em or en dashes in new text).
  - Spec R24: write for players new to the genre wherever a newcomer can plausibly arrive. Labels ask about what the player sees, not game vocabulary; teach a concept before a line relies on it (order with `heardRequired`).
  - One command per hint (owner style ruling 2026-10-17, quest 28 header): say what the thing is, then the one command, then the help topic. A step hint names only that step's command; a quest step that needs two commands is split. Teach a spell by its short alias (`cast glow`), never only by its full id.
  - A hand-in that takes an item gates on the quest's `-start` (or the step before) plus `requiresItem`, never on a step granted when the item is picked up: an item can be picked up before the quest is accepted, and that player would hold it with no way to hand it in (batch 2 review B1). A step granted on pickup may drive hints and opener variants only.
  - Grep `behaviors/` too for player-facing `ask` instructions (batch 1 found the first line of the game there), and teach `whisper`, never `tell` (no such command).
  - For each topic: write `label` (info: the player's line in the player's voice, at most 50 characters; offer and hand-in: already the quest title, keep it); write `playerLine` for offers ("Do you want help ...?") and hand-ins (reporting the result), at most 70 characters; write `breadcrumb` where the converter flagged a gated offer and the NPC would plausibly say "come back later".
  - Fold any `text` holding `\n---\n` into one natural reply.
  - Rewrite every offer's `text` so it reads as a pitch **before** acceptance (no "thank you for agreeing").
  - Reconcile each merged hand-in: one line combining the topic text and the quest `npc_say` line from the report.
  - Decide `marked` for info topics: writer-flagged lore or directions that matter mark; small talk never marks; mid-quest step topics stay marked.
  - Apply the owner rulings from Task 14 that touch this zone.
  - Return 5 sample topics (one offer, one hand-in, three info) per zone, verbatim.
  - Walk every quest step's hint against the real route and the step before it: a direction must be true from where the player stands after the previous step (check room exits), and a step's trigger (a `look <noun>`, `get`, a room) must be something the earlier text has already named so the player can guess it. Rewrite hints that say "continue" in a direction the player came from, or that expect a noun nobody mentioned.
- **Owner notes for specific zones (apply when that batch runs):**
  - Stillwater, quest 20 "Ulla's Silence" (owner 2026-10-21): the quest reads oddly and several hints do not line up with the actions. "Continue west to the wardstone circle. The spiral connects the sites in a line." is wrong: the previous step came FROM the west. The step that finishes on `look slab` is unclear, because nothing earlier tells the player there is a slab to look at. Walk the whole quest step by step on the map, fix every hint and description, and make each trigger's noun one the text has named. Singular and plural room nouns (wardstone and wardstones) are fixed in the engine (`rooms.FindNoun`, fix PR from 2026-10-21); confirm quest triggers with `noun:` match both spellings too.
- [ ] **Step 2: After each batch, show the owner the samples** and wait for approval before the next batch. Corrections become brief changes for the remaining batches.
- [ ] **Step 3: Validate the whole output** with a small test-only program or `go test` helper that loads the output folder through `dialogue.LoadAllFiles` logic pointed at it and runs `ValidateWorld` with the real registries (load quests and items as `boot_smoke_test.go` does). Zero problems before copying.
- [ ] **Step 4: Copy into the repo.** Replace each shipped dialogue file with its converted version (`cp` from the output folder; new behavior-only files included). Commit together with Tasks 16 and 17 (end of Task 17). The branch is never merged or deployed between here and Task 25, so the old engine reading converted files on the branch does no harm.

### Task 16: Remove the converted quest triggers

**Renumbering from master (2026-10-21):** PR #648 merged quest 20's `pillars`/`pillar` trigger pair (0-based index 2 removed, old 3 became 2, every later quest 20 trigger shifts down by one). After this branch merges master, the converter report's `delete quest 20-ullas_silence.yaml trigger 17` and `trigger 18` name triggers 16 and 17. Delete by matching the trigger's content (event, item, grant), never by stale index; re-run the converter after the merge to regenerate the list.

- [ ] For every `delete quest <file> trigger <index>` line in the final report (33 `item_give`, 1 `dialogue`), delete that trigger block from the quest file with Edit, keeping comments that still describe remaining triggers and removing comments that described the deleted one. Then `grep -n "event: item_give" -r _datafiles/world/dogmud/quests` must list only triggers without a `grant` action (expect none) and `grep -n "event: dialogue" -r _datafiles/world/dogmud/quests` must be empty.

### Task 17: Rewrite the behavior-tree ask branches

- [ ] For each of the 26 branches in the report, edit its behavior file with Edit:
  - speech-only branch: delete it (its text now lives in the topic);
  - `grant_quest` branch: delete it (now an offer topic);
  - action branch: `event: player_ask` becomes `event: player_topic`, the `keyword_match` condition becomes `check: topic_is` with `topic: <key>`, and the branch's `say`/`respond` action that the topic's `text` now speaks is removed so the NPC does not speak twice. Keep every other condition and action.
  - ferry branches: one topic and one branch per destination, with `destination:` on `board_ferry` (Task 12b).
  - instance-zone branches (Sable 315, Threshold Keeper 9553): one topic per zone, with `zone:` and `guidance:` on `open_instance_portal` (Task 12b, R14); guidance drafts go to the owner as samples.
- [ ] Update the comment in `behaviors/archetypes/noncombat_questgiver.yaml` that mentions `player_ask`.
- [ ] Then `grep -rn "player_ask" _datafiles/world/dogmud` must be empty, and every `topic_is` key must name a topic on that mob (Task 18's validator checks this at boot).
- [ ] **Commit the content** (Tasks 15 to 17) by named paths: `git add _datafiles/world/dogmud/dialogue _datafiles/world/dogmud/quests/<each edited file> _datafiles/world/dogmud/behaviors/<each edited file>`, then `git diff --cached --stat` and check the file count against the report (302 dialogue files plus any new ones, 24 quest files, 1 quest file for the `dialogue` trigger if separate, 12 behavior files plus the archetype comment).

```bash
git commit -F - <<'EOF'
content(dialogue): convert every NPC to keyed, labelled topics (#375 phase 1)

Converted by cmd/dialogueconvert, labels and lines written per zone with
owner samples. Quest item_give hand-ins and the quest 28 dialogue trigger
fold into topics; behavior-tree ask branches become topics.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

# Stage C: switch and land

### Task 18: Strict loading, the boot door and the CI door

**Files:**
- Modify: `internal/dialogue/loader.go` (`yaml.UnmarshalStrict`), `internal/dialogue/walk.go` (same), `main.go` (after `questengine.LoadDataFiles()` at line 1892), `boot_smoke_test.go:231-300`
- Create: `main_dialogue.go` (the behavior-key index lives in package `main`, which keeps `internal/dialogue` free of `behaviortree`)

- [ ] **Step 1: Strict decode** in `Load` and `WalkAllFiles`: replace `yaml.Unmarshal(bytes, &df)` with `yaml.UnmarshalStrict(bytes, &df)`. The converted files carry no old keys.
- [ ] **Step 2: The boot door.** In `main.go`, right after `questengine.LoadDataFiles()`:

```go
	if probs := validateShippedDialogue(); len(probs) > 0 {
		for _, p := range probs {
			mudlog.Error("dialogue", "problem", p.String())
		}
		panic(fmt.Sprintf("%d dialogue problems; see the log above", len(probs)))
	}
```

with, in a new `main_dialogue.go` beside `main.go`:

```go
package main

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
)

// validateShippedDialogue is the boot door of the dialogue validators: the
// shipped files against the loaded quests, items and behavior trees.
func validateShippedDialogue() []dialogue.Problem {
	w, probs := dialogue.RegistryWorld(behaviorTopicKeys())
	return append(probs, dialogue.ValidateWorld(w)...)
}

// behaviorTopicKeys indexes topic_is keys by mob id from behaviors/**/<id>-*.yaml.
// (Review of Task 6 changed World.BehaviorTopics to map[int][]string so the
// validator can also report a topic_is on a mob with no dialogue file:
// return byMob directly instead of the closure below.)
func behaviorTopicKeys() func(int) []string {
	byMob := map[int][]string{}
	root := filepath.Join(string(configs.GetFilePathsConfig().DataFiles), "behaviors")
	paths, _ := filepath.Glob(filepath.Join(root, "*", "*.yaml"))
	for _, p := range paths {
		idText, _, _ := strings.Cut(filepath.Base(p), "-")
		id, err := strconv.Atoi(idText)
		if err != nil {
			continue
		}
		if keys, err := behaviortree.TopicKeysInFile(p); err == nil && len(keys) > 0 {
			byMob[id] = append(byMob[id], keys...)
		}
	}
	return func(id int) []string { return byMob[id] }
}
```

Check: items must be loaded before this call (they are loaded earlier in `main.go`; confirm by reading the load order above line 1892). Confirm the behavior folder layout with `ls _datafiles/world/dogmud/behaviors` (zone folders plus `archetypes`; archetype file names do not start with a number and are skipped).

- [ ] **Step 3: The CI door.** Rewrite `TestSmoke_ShippedDialoguePassesTheEditorValidator` as `TestSmoke_ShippedDialoguePassesTheValidators`: after `loadAllDataFiles(false)`, call `validateShippedDialogue()` and `t.Errorf` each problem; delete `knownDialogueValidatorFindings`, `dialogueFindingKey` and `knownIgnoredDialogueKeys` and every test that only fed them (`grep -n "knownDialogueValidatorFindings\|knownIgnoredDialogueKeys\|dialogueFindingKey" boot_smoke_test.go`). Keep `TestSmoke_AllDialogueFilesParse` if it exists, now parsing strictly.
- [ ] **Step 4: Run**

Run: `go build ./... && BOOT_SMOKE=1 go test . -run 'TestSmoke_ShippedDialogue|TestSmoke_AllDialogue' -count=1` (use the real env var name from `bootSmokeEnvVar` in `boot_smoke_test.go`).
Expected: PASS with the converted files from Task 15 in place.

- [ ] **Step 5: Commit** (local only: nothing in Stage C is pushed until Task 25 passes)

```bash
git add internal/dialogue/loader.go internal/dialogue/walk.go main.go main_dialogue.go boot_smoke_test.go
git commit -F - <<'EOF'
feat(dialogue): strict loading, validators at boot and in the smoke sweep (#375 phase 1)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 19: Switch the commands

**Files:**
- Modify: `internal/usercommands/talk.go`, `internal/usercommands/ask.go`, `internal/usercommands/give.go`, `internal/conversation/companion.go` (install the asker)
- Test: rewrite `internal/usercommands/ask_test.go`; delete or port the tests that drove the old paths (list below)

- [ ] **Step 1: Write the failing tests**

```go
func TestAsk_RedirectsToTalk(t *testing.T) {
	user, _, done := talkFixture(t)
	defer done()
	_, err := Ask("skeleton about bones", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	mine, theirs := drained(1), drained(2)
	require.Contains(t, mine, "talk skeleton")
	require.NotContains(t, mine, "Old age.", "ask never answers a topic")
	require.Empty(t, theirs, "nothing happens in the room")
	require.False(t, user.Character.HasHeard("1.bones"))
}

func TestTalkCommand_OpensTheMenu(t *testing.T) {
	user, _, done := talkFixture(t)
	defer done()
	_, err := Talk("skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	require.True(t, user.HasOpenMenu())
	require.Contains(t, drained(1), "Talking with Skeleton")
}

func TestGiveCommand_HandInGoesThroughTheTopic(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	require.True(t, user.Character.StoreItem(items.New(10001))) // seeded "Iron Sword"
	_, err := Give("sword skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	require.Contains(t, drained(2), "My thanks.")
	for _, itm := range user.Character.GetAllBackpackItems() {
		require.NotEqual(t, 10001, itm.ItemId, "the sword was handed in")
	}
	for _, itm := range mob.Character.Items {
		require.NotEqual(t, 10001, itm.ItemId, "a topic hand-in never transfers to the mob")
	}
}
```

(import `github.com/GoMudEngine/GoMud/internal/items`). If `give` parses the item by a different word, use the one `give_test.go` uses for item 10001.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/usercommands/ -run 'TestAsk_RedirectsToTalk|TestTalkCommand_|TestGiveCommand_' -count=1`
Expected: FAIL (ask still answers; talk still greets the old way).

- [ ] **Step 3: `talk.go`.** Keep the argument parsing and target resolution (lines 23-60 today). After `mob := target.(*actions.MobActor).Mob`, replace everything to the end of `Talk` with:

```go
	// A sleeping NPC cannot hold a conversation (moved from ask, #375).
	if actions.RefuseMobIfAsleep(mob, user) {
		return true, nil
	}
	conversation.Talk(user, room, mob)
	return true, nil
```

Delete `buildPlayerCondition`, `buildTutorialContext` and `buildQuestContext` (LLM-only) and the `buildPlayerState` shim; point the remaining callers and tests at `conversation.PlayerState` (`grep -rn "buildPlayerState" internal/usercommands`). Remove dead imports (`llm`, `configs`, `quests`, ...).

- [ ] **Step 4: `ask.go`.** Keep: the `len(args) < 2` branch (charmed blank stare and usage prompt), `AimBySight`, target resolution, the player refusal, the `to`/`about` stripping and `companionai.RouteAsk` (R13: chat with your own bonded companion), and the charmed blank stare. Replace the rest of `Ask` (the room "asks ... about" line, `askNpcChain`, the exits line) with:

```go
	user.SendText(messaging.CategoryRefusal, fmt.Sprintf(
		`Conversations happen through <ansi fg="command">talk</ansi> now. Try <ansi fg="command">talk %s</ansi>.`,
		strings.ToLower(searchName)))
	return true, nil
```

Delete `deliverDialogue`, `isQuestInquiry`, `askNpcChain`, `AskNpcForOwner` and the `init` that installed it. In `internal/conversation/companion.go` add:

```go
func init() {
	companionai.SetNpcAsker(AskForOwner)
}
```

(import `github.com/GoMudEngine/GoMud/internal/companionai`). If Task 12 deferred its `actions.go` edit, apply it now.

- [ ] **Step 5: `give.go`.** In the mob branch, inside `} else {` (the item case) and before the quest engine `Notify("item_give", ...)`, add:

```go
				// Topics first (#375 R8): a hand-in plays as the topic, and a
				// wanted item given too early is handed back before it moves.
				if conversation.Give(user, room, m, giveItem) {
					return true, nil
				}
```

Keep the quest engine notify (no shipped trigger grants on `item_give` any more; the validator refuses one that does) and everything after it.

- [ ] **Step 6: Old-path tests.** Run `go test ./internal/usercommands/ -count=1` and read every failure. For each failing test: if it tests behavior that still exists (gold gates, item delivery failure, sleeping NPC refusal, behaviour-tree broadcast on a give), port it to `talk`/topics/`give`; if it tests typed-topic matching, `ask` answers, the LLM path or tree memory, delete it and list it in the commit message. Expected to be touched: `ask_test.go`, `ask_behaviour_broadcast_test.go`, `talk_gold_test.go`, `talk_giveitem_test.go`, `give_test.go` (the quest-engine-intercept test keeps working: its fixture mob has no dialogue file, so `conversation.Give` returns false).
- [ ] **Step 7: Run**

Run: `go build ./... && go test ./internal/usercommands/ ./internal/conversation/ ./modules/... -count=1`
Expected: PASS.

- [ ] **Step 8: Commit** (local)

```bash
git add internal/usercommands internal/conversation
git commit -F - <<'EOF'
feat(commands): talk opens the topic menu, ask redirects, give resolves topics (#375 phase 1)

LLM calls leave talk and ask (R4). Deleted old-path tests: <list>.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

(`git add internal/usercommands internal/conversation` names two directories, which the guard allows; check `git diff --cached --stat` lists only files you changed.)

### Task 20: Switch the dialogue editor off

**Files:**
- Modify: `modules/gmcp/gmcp.Build.go:398-425`, `_datafiles/html/public/static/js/mobs.js:258-261`, `_datafiles/html/public/build.html:263`, `_datafiles/html/public/build-help.html` (its `#dialogue` section, if present)
- Delete: `modules/gmcp/gmcp.Dialogue.go`, its tests (`grep -ln "buildDialogue" modules/gmcp/*_test.go`), `_datafiles/html/public/static/js/dialogue.js`

The old panel edits patterns and trees, which no longer exist; phase 6 builds the topic builder new. Until then every `Build.Dialogue.*` message is refused.

- [ ] **Step 1: Failing test** in `modules/gmcp` (beside the other Build tests): sending `Build.Dialogue.Update` (and `Get`, `Create`, `Delete`) produces a `BuildResult` with `Ok == false` and a message containing `phase 6`. Use the same harness the existing Build tests use (`grep -n "func Test.*Build" modules/gmcp/*_test.go | head`).
- [ ] **Step 2: Implement.** Replace the four `case` blocks with one:

```go
	case `Build.Dialogue.Get`, `Build.Dialogue.Update`, `Build.Dialogue.Create`, `Build.Dialogue.Delete`:
		sendBuildResult(uid, buildErr("the dialogue editor is off until the topic builder ships (quest arc phase 6); edit dialogue files in the repo"))
```

Delete `gmcp.Dialogue.go` and its tests. In `mobs.js` remove the `Dialogue…` button (lines 258-261) and in `build.html` the `dialogue.js` script tag; delete `dialogue.js`. Keep `hasDialogue` in the mob payload (it is still true or false).
- [ ] **Step 3: Run** `go build ./... && go test ./modules/gmcp/ -count=1`. Expected: PASS.
- [ ] **Step 4: Commit** (local), naming every changed and deleted path.

### Task 21: Delete the old engine

**Files:** `internal/dialogue/{types,engine,memory,loader,validate,registry_validators,validation,save}.go`, `internal/dialogue/*_test.go`, `modules/gmcp/gmcp.Quest_refs.go`, `internal/behaviortree/events.go`, `internal/llm/{client,types}.go` (comments only), `internal/hooks/NewTurn_SweepDialogueMemory.go` (comment)

Use the refactoring rule (`dogmud-refactoring`): delete the field or function first and let the compiler list its consumers.

- [ ] **Step 1: Delete** from `types.go`: `Pattern`, `TreeNode`, `QuestGreeting`, `TreeRoot`, `Tree`, `MemoryConfig`, and `DialogueFile.Patterns`, `.Tree`, `.Memory`. From `engine.go`: `Match`, `MatchWithFallbackInfo`, `TreeAdvance`, `Greet` (keep `checkQuestGate`, `itemExcludedBlocks`, `tryCharge`, `applyPaidEffects`, `applyQuestEffects`). From `loader.go`: `validateQuestExclusions` and its call. Delete `validate.go` and `registry_validators.go` (`ValidateDialogueFile`, `DialogueValidators`, `gateFields`, `RegistryValidators`); `ValidateWorld` replaces them.
- [ ] **Step 2: `memory.go`.** `PlayerMemory` keeps `LastVisitRound` and `Greeted`. Delete `UpdateMemory`, `IsExpired`, `ResetMemory`. `MarkGreeted` sets `LastVisitRound = util.GetRoundCount()` so `SweepMemories` still ages entries out. Keep `GetMemory`, `SweepMemories`, `ForgetMobInstance`.
- [ ] **Step 3: `validation.go`.** Rewrite `CollectFlagReferences` over `df.Topics` (sources `"topic <key>"`) and `df.Opener.Variants` (`"opener variant <n>"`); same return shape, so `questengine.ValidateAllFlags` is unchanged.
- [ ] **Step 4: `save.go`.** `CreateNewDialogueFile` writes `Opener: &Opener{Text: "..."}` instead of a catch-all pattern.
- [ ] **Step 5: `gmcp.Quest_refs.go`.** `walkDialogueGates` visits topics (`"topic <key>"`, with `t.Gates` and `t.GrantsQuest`/`t.SetsQuestFlag`) and opener variants (gates only).
- [ ] **Step 6: Behavior tree.** Delete `player_ask` from `KnownBehaviorEvents` (no content uses it after Task 17; nothing fires it after Task 19). Keep `keyword_match` (`grep -rn "keyword_match" _datafiles` to confirm what still uses it; if nothing, leave it registered anyway, it is a generic condition).
- [ ] **Step 7: Tests.** `go test ./internal/dialogue/` fails to compile on the deleted functions. For each old test file decide, in the commit message, port or delete:
  - port to `Eligible`/`Apply`/`ValidateWorld` (behavior that survives): `gold_gate_test.go`, `charges_gold_test.go`, `item_excluded_test.go`, `masterwork_gate_test.go`, `giveitem_failure_test.go`, `bumpsrep_test.go`, `playerstate_guards_test.go` (nil-callback contracts), `cache_poisoning_test.go`, `save_test.go`, `walk_test.go`, `greetings_test.go`, `memory_sweep_test.go`;
  - delete (behavior gone): `match_fallback_test.go`, `delivery_askpath_test.go`, `integration_dialogue_test.go` and `toke_ways_test.go` if they only drive `Match`/`TreeAdvance`, `validate_test.go`, `registry_validators_test.go`.
  Read each file before deciding; a file that mixes both gets its surviving cases ported.
- [ ] **Step 8: Comments.** `llm/client.go:23` and `llm/types.go:20` mention `dialogue.Load/ShiftMood` and `dialogue.GetMemory()`; reword them to say the fields are no longer filled by `talk`/`ask` (R4). Remove the `RecentTopics` field from `llm.ConversationContext` only if nothing else sets it (`grep -rn "RecentTopics" --include=*.go`).
- [ ] **Step 9: Run** `go build ./... && go vet ./... && go test ./internal/... ./modules/... -count=1`. Expected: PASS.
- [ ] **Step 10: Commit** (local), listing ported and deleted tests in the body.

### Task 22: Manifest, guard and load rename

**Files:**
- Create: `_datafiles/world/dogmud/dialogue/topic-keys.txt`, `topic_manifest_test.go` (repo root, package `main`, runs at the repo root like `boot_smoke_test.go`)
- Modify: `internal/users/users.go:564` (after `MigrateQuestFlags`), `docs/README.md`

- [ ] **Step 1: Generate the manifest.** Write the guard test first (Step 2); run it once and it fails listing every content key as unlisted, in manifest form. Build the file from that list: the header comment, then the sorted keys. Write it with Write.
- [ ] **Step 2: The guard test**

```go
package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"gopkg.in/yaml.v2"
)

func shippedTopicKeys(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "dialogue", "*", "*.yaml"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("found no dialogue files (err %v): the guard cannot run", err)
	}
	var keys []string
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		var df dialogue.DialogueFile
		if err := yaml.UnmarshalStrict(data, &df); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		for _, tp := range df.Topics {
			keys = append(keys, dialogue.HeardKey(df.MobId, tp.Key))
		}
	}
	sort.Strings(keys)
	return keys
}

// TestTopicKeyManifest: saved characters hold heard keys, so a shipped key
// may leave content only through a "# retired" or "# renamed" line, and a
// new key must be added to the manifest.
func TestTopicKeyManifest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("_datafiles", "world", "dogmud", "dialogue", "topic-keys.txt"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	m, err := dialogue.ParseManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	missing, unlisted := m.Diff(shippedTopicKeys(t))
	for _, k := range missing {
		t.Errorf("topic key %s is gone from content: add it under \"# retired\" or \"# renamed\" (old -> new)", k)
	}
	for _, k := range unlisted {
		t.Errorf("topic key %s is not in topic-keys.txt: append it", k)
	}
}

// TestTopicKeyManifest_GuardCanFail proves the guard sees a removed key.
func TestTopicKeyManifest_GuardCanFail(t *testing.T) {
	keys := shippedTopicKeys(t)
	m := dialogue.Manifest{Keys: append([]string{"0.removed_on_purpose"}, keys...)}
	missing, _ := m.Diff(keys)
	if len(missing) != 1 || missing[0] != "0.removed_on_purpose" {
		t.Fatalf("guard did not see the removed key: %v", missing)
	}
}
```

- [ ] **Step 3: Load rename.** In `users.go`, after `loadedUser.Character.MigrateQuestFlags()`:

```go
	loadedUser.Character.RenameHeardTopics(dialogue.HeardRenames())
```

Add a test in `internal/users` (beside the other load tests; `grep -n "func Test.*Load" internal/users/*_test.go`) that writes a manifest with `12.old -> 12.new` into the test data folder, loads a user whose save has `heardtopics: [12.old]`, and sees `12.new`. If `users` cannot reach a manifest in its tests, test `RenameHeardTopics` (done in Task 4) and `HeardRenames` (in `internal/dialogue`, with `configs.AddOverlayOverrides` pointing `FilePaths.DataFiles` at a temp dir) separately, and state that in the commit.
- [ ] **Step 4: Run** `go test . -run TestTopicKeyManifest -count=1 && go test ./internal/users/ ./internal/dialogue/ -count=1`. Expected: PASS.
- [ ] **Step 5: Commit** (local): the manifest, the test, `users.go`, `docs/README.md` (row for `topic-keys.txt`).

### Task 23: Code guards

**Files:** Create `internal/usercommands/talk_guard_test.go`

- [ ] **Step 1: Write the guards, each with a self-test that proves it can fail**

```go
package usercommands

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var askTopicHandling = regexp.MustCompile(`dialogue\.|TreeAdvance|\.Match\(|player_ask`)

func askHasTopicHandling(src string) bool { return askTopicHandling.MatchString(stripGoComments(src)) }

func usesLLM(src string) bool { return strings.Contains(src, `"github.com/GoMudEngine/GoMud/internal/llm"`) }

// stripGoComments drops // line comments so a comment naming a forbidden
// symbol does not trip the guard.
func stripGoComments(src string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func TestGuard_AskHasNoTopicHandling(t *testing.T) {
	src, err := os.ReadFile("ask.go")
	if err != nil {
		t.Fatal(err)
	}
	if askHasTopicHandling(string(src)) {
		t.Fatal("ask.go handles topics again; ask only redirects to talk (#375 requirements rule 7)")
	}
}

func TestGuard_AskGuardCanFail(t *testing.T) {
	if !askHasTopicHandling("df := dialogue.Load(1, z)") {
		t.Fatal("the ask guard cannot see topic handling")
	}
	if askHasTopicHandling("// dialogue.Load is gone") {
		t.Fatal("the ask guard trips on a comment")
	}
}

func TestGuard_TalkAndAskMakeNoLLMCalls(t *testing.T) {
	files := []string{"talk.go", "ask.go"}
	more, _ := filepath.Glob(filepath.Join("..", "conversation", "*.go"))
	files = append(files, more...)
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if usesLLM(string(src)) {
			t.Errorf("%s imports internal/llm; LLM dialogue is out of this arc (R4, #643)", f)
		}
	}
}

func TestGuard_LLMGuardCanFail(t *testing.T) {
	if !usesLLM("import \"github.com/GoMudEngine/GoMud/internal/llm\"") {
		t.Fatal("the LLM guard cannot see the import")
	}
}
```

- [ ] **Step 2: Run** `go test ./internal/usercommands/ -run TestGuard_ -count=1`. Expected: PASS.
- [ ] **Step 3: Commit** (local).

### Task 24: Docs, help, skill, converter removal

- [ ] **Help:** rewrite `templates/help/talk.template` (the menu, numbers, Enter to leave, markers `[!]` `[?]` `[*]` with what each means, `give` to hand things in), `ask.template` (now: ask your bonded companion; NPCs use `talk`), and the talk/marker part of `quests.template`. Follow `dogmud-player-copy`: 80 columns, no dashes, ESL-clear. Keep the existing ansi tag style of the templates.
- [ ] **`context.md`:** rewrite `internal/dialogue/context.md` for topics (data model, public API from `topic.go`, `topics.go`, `world.go`, `world_registry.go`, `manifest.go`, greetings and mood, what was deleted); add the `conversation` files table (Tasks 9 to 12); update `internal/menu/context.md` "Who uses it" (`internal/conversation`, `internal/dialogue` for `LintLabels`), `internal/usercommands/context.md` (talk, ask, give), `internal/characters/context.md` (`HeardTopics`), `internal/rooms/context.md` (`NpcMarker`), `internal/mobs/context.md` (no `QuestFlags`), `internal/behaviortree/context.md` (`player_topic`, `topic_is`, `player_ask` gone), `modules/gmcp/context.md` (`marker`, dialogue editor off), `modules/aicompanion/context.md` (`companion-ask` topic resolution). Verify every symbol named with `codegraph_search` or `Select-String`, then run `python tools/context_md_audit.py` and fix what it reports.
- [ ] **Skill:** rewrite the dialogue sections of `.claude/skills/dogmud-authoring-quests/SKILL.md`: the two ways to mute an NPC (re-check both against the new loader: a strict-decode failure now refuses at boot), the questExcluded rule (now in `ValidateWorld`), discoverability (menus replace trigger words), the give.go section (topics handle hand-ins before the transfer; `item_give` may not grant), and a short topic-file example. Keep the parts about quest YAML that did not change.
- [ ] **Roadmap and README:** set phase 1 status in `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md` to "implemented, PR #<n>"; add `docs/README.md` rows for this plan and every new file not yet listed.
- [ ] **Delete the converter:** `git rm -r cmd/dialogueconvert` (its job is done; history keeps it).
- [ ] **Commit** (local).

### Task 25: Full gate

- [ ] Follow `dogmud-shipping` for the pre-push gate order: `gofmt -l .` empty, `go vet ./...`, `go test ./... -count=1`, the boot smoke tests with their env var, and the detached-worktree boot check (boot the server on a private port from a clean worktree of the branch, confirm it reaches "ready" with no dialogue problem panic, kill it by its PID).
- [ ] Wipe instance saves before any local smoke test, per `dogmud-shipping`.
- [ ] Every red result is investigated and fixed; nothing is skipped or marked flaky without a cause (`dogmud-writing-tests` lists the known flaky fumble rate).

### Task 26: Adversarial playtest

Follow `dogmud-playtesting` (harness location, an ephemeral goals file, `--checkout` the branch, private port, kill by PID, never touch the owner's server). One subagent per seat.

- [ ] Goals:
  1. Thornwall, quest 10 (Velk, mob 94): talk, see `[!]`, take the offer, Leave once then Accept; follow the step to the notice; `give` the notice to Velk at the right step (topic plays: line, handover, reply) and once at a wrong step (handed back, item kept); hand in.
  2. Pothole Coulee: the opening NPCs' menus, markers in the room list, `[*]` clearing after hearing, Dim on re-show.
  3. A ferry (9571 or 9574 to 9576): the ferry topic boards the ferry (behavior topic); the menu closes on the move.
  4. Relog and server restart: heard topics stay Dim and unmarked.
  5. Interrupts: combat starting, the NPC walking away, darkness falling mid-menu.
  6. `ask <npc> anything` redirects; `ask` on a hired companion behaves as before.
  7. New-player start, end to end, on a fresh character (owner 2026-10-21, required): wake in the newcomer antechamber, Dewey's talk lesson in room 6261 (quest 28), into Pothole Coulee; every opening NPC's menu; Find Your Footing (31); Crier Toke's seven trail offers accepted one at a time with each starter item arriving on its own offer; Hadwen's rite (quest 30: the menu closes on the scene, the mid-rite talk does not restart it); both roads after the rite, taken and handed in; First Heat turned in three ways on three characters (show the dagger, sell it to Rusk, report after selling it to Onna), each paying the quest reward (R22); no instruction anywhere tells the player to type `ask`. A second fresh character takes neither road and must not be stuck.
- [ ] File every finding as a GitHub issue (`gh issue create --repo pruuk/DOGMud`; search first), exploit-class bugs excepted per CLAUDE.md. Fix findings that break the phase's exit criteria before the PR; file the rest.

### Task 27: Push and open the PR

- [ ] `git push -u origin <branch>`; `gh pr create --repo pruuk/DOGMud --base master` with a body summarising the phase, the counts from the reconciliation report, the playtest results and issues filed, and the owner rulings. Do not write "closes #375" or any closing keyword for the epic (it stays open for phases 2 to 7). End the body with the Claude Code attribution line.
- [ ] Give the owner the PR link. The owner deploys; do not deploy.

## Self-review record

- Spec coverage: data model (Tasks 1, 4), eligibility/markers/opener/menu (2, 11), Apply (3), manifest (5, 22), validators (6, 18), behavior topics (7, 13, 17), conversation glue (8 to 12), conversion (13 to 17), command switch (19), editor off (20), deletion (21), guards (23), docs and help (24), gates (25), playtest (26).
- Known deviations from the spec text, each recorded in the spec: `behavior` is a flag (4.1); the hand-in output order is line, handover, reply (Task 9); the companion's topic is resolved from the owner's words at command time (4.2); the GMCP marker has no web client consumer yet (4, item 6).
