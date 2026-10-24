# Quest Arc Phase 2a: Offer Card, Hand-in, Repeats and Loop Validators (Implementation Plan)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every quest offer shows its objective and reward, a hand-in plays the next quest's offer, repeatables carry an optional lifetime limit and say that they repeat, and the quest validators run at boot, CI and editor save behind a shrinking backlog.

**Architecture:** Spec `docs/superpowers/specs/2026-10-24-quest-arc-phase2-loop-design.md` (rulings P1 to P6), sections 3 to 6 and 8. The talk flow in `internal/conversation` gains a card body and a deferred next-offer event; `internal/quests` owns reward wording, the repeat fields and the step-id manifest; `internal/characters` saves completion counts; a new pure `internal/questloop` package holds the loop rules and the backlog ratchet. Plan 2b (the completability walkthrough, spec section 7) is a separate plan.

**Tech Stack:** Go 1.x, `gopkg.in/yaml.v2`, testify `require`, the repo's events queue, Node (zero-dependency) for the web client test.

---

## Ground rules for every task

- **Edit files with the Edit and Write tools only.** No `sed -i`, no shell redirection into a
  tracked file, no Python read-modify-write. Generated files are written to the scratchpad and
  copied into place with `cp`.
- **Stage by name:** `git add <paths>`. Never `git add -A`, `.`, `--all`, or `git commit -a`.
- Commit messages go through a quoted heredoc (`git commit -F - <<'EOF'`) and end with the
  `Co-Authored-By` line from the session's system reminder.
- Never kill a process you did not start; never by name or port.
- Balance numbers come from `config.yaml`; this plan adds none.
- Player copy: no em or en dashes, no raw balance numbers, 80-column wrap.
- Run Go tests from the worktree root. Test binaries load Go config defaults, not
  `config.yaml` (`dogmud-writing-tests`).

## Facts verified against source (master `fd18743db`, 2026-10-24)

The spec's section 1 table holds the content counts. These are the code facts each task relies
on, read while writing this plan.

| Fact | Source |
|---|---|
| `menu.Menu` fields `Heading, Entries, Leave, Default, Footer, Columns, Prefix, AcceptText, LeaveAliases`; `Render` writes Heading, blank line, entries, blank line, footer; `wrap(text, width)` is unexported in `menu.go` | `internal/menu/menu.go:45,72,180` |
| `menu.Confirm(question, yesLabel, noLabel) Menu` sets Heading and two entries (`yes`, `no`, aliases `y`, `n`), Default `no` | `internal/menu/resolve.go:132` |
| `offerCard(user, mob, t, prefer)` builds `menu.Confirm(t.Label, "Accept", "Leave")` | `internal/conversation/talk.go:320` |
| `resolve` pitches an offer then calls `offerCard`; `play` speaks and applies info and hand-in topics and returns `(granted, applied)` | `internal/conversation/talk.go:252,273` |
| `heldAlso(ps, granted)` counts a just-granted token as held | `internal/conversation/talk.go:172` |
| `session(...)` sets `ShowLast: true`; ShowLast menus go out at event priority 50, ordinary lines at 0 | `internal/conversation/talk.go:202`, `internal/users/userrecord.menu.go:123,134` |
| Event queue: lower priority number runs first, FIFO within a priority, and events queued during a pass run in that pass | `internal/events/events.go:66,177` |
| `PlayerState.GiveQuest` queues `events.Quest` at priority 0; `HandleQuestUpdate` prints the completion banner and reward lines when it runs | `internal/conversation/playerstate.go:57`, `internal/hooks/Quest_HandleQuestUpdate.go:259` |
| Hook registration list | `internal/hooks/hooks.go:104` (`events.RegisterListener(events.Quest{}, HandleQuestUpdate)`) |
| Completion fires on step id `end`; start on `start` | `Quest_HandleQuestUpdate.go:250,259` |
| Cooldown refusal block | `Quest_HandleQuestUpdate.go:206-216` |
| Repeatable reset block | `Quest_HandleQuestUpdate.go:446-452` |
| Reward parsers `parseSkillGrants`, `parseStatGrants`, `parseRecipeGrants`, `parseItemGrants` and types `skillGrant{skill, level}`, `statGrant{stat, amount}`, `itemGrant{itemId, qty}` live unexported in hooks; tests `Quest_ItemGrants_test.go`, `Quest_RecipeGrants_test.go` | `Quest_HandleQuestUpdate.go:26-160`; `internal/hooks/` |
| Stockpile payout wording `You receive <name> (x3)!` | `Quest_HandleQuestUpdate.go:361` |
| `items.ArticlePrefix(name)`, `items.MidSentenceName(name)` | `internal/items/article.go:19,49` |
| Name lookups: `items.GetItemSpec(id).Name`, `crafting.GetRecipe(id).Name`, `spells.GetSpell(id).Name`, `factions.GetDefinition(id).Name` | `internal/items/itemspec.go:814,327`, `internal/crafting/crafting.go:126,34`, `internal/spells/spells.go:129,22`, `internal/factions/registry.go:103` |
| `conversation` already imports items, skills, crafting, spells, factions, questengine; `modules/gmcp` imports `conversation`; none of those reach `conversation` | `go list -deps`, 2026-10-24 |
| `characters` imports `quests`; `Character.QuestProgress map[int]string` `yaml:"questprogress,omitempty"` at line 319, `HeardTopics` at 321 | `internal/characters/character.go:319,321` |
| `ClearQuestToken`, `SetQuestCooldown`, `QuestCooldownActive`, `miscDataToUint64` | `internal/characters/quests.go:126,192,198,209` |
| `RenameHeardTopics(renames map[string]string)` | `internal/characters/heard.go:26` |
| `LoadUser` migration block; heard renames at line 568 | `internal/users/users.go:558-568` |
| `TestLoadUserRenamesHeardTopicsFromTheManifest` | `internal/users/loaduser_test.go:83` |
| `dialogue.PlayerState` callbacks (no `QuestOpen`); `Eligible` checks gates then `heardRequired` | `internal/dialogue/types.go`, `internal/dialogue/topics.go:29` |
| 32 `PlayerState{` literals outside `conversation/playerstate.go`, in `boot_smoke_test.go`, `veyra_secrets_dialogue_test.go`, 12 files under `internal/dialogue/`, 2 under `internal/questengine/` | grep 2026-10-24 |
| Topic manifest: `dialogue.Manifest{Keys, Retired, Renamed}`, `ParseManifest`, `Diff`, `HeardRenames`; error text names `topic-keys.txt` | `internal/dialogue/manifest.go` |
| Root guard `topic_manifest_test.go` (`TestTopicKeyManifest`, `_GuardCanFail`) | repo root |
| `quests.GetAllQuests() []Quest` (values), `GetQuestById(int) *Quest` | `internal/quests/quests.go:286,321` |
| `ValidateQuestRefs(q Quest, v QuestValidators) (errs, warns []string)`; production validators `realQuestValidators()` | `internal/quests/validate_refs.go:219`, `modules/gmcp/gmcp.Quest.go:58` |
| Editor save `buildQuestUpdate(d questDeps, q quests.Quest) BuildResult` | `modules/gmcp/gmcp.Quest.go:224` |
| `ActionDef.Grant`, `ActionDef.Sequence *SequenceDef{OnComplete []ActionDef}` | `internal/quests/triggers.go:45,151` |
| `Conditions{Has, Missing, InRoom, HasItem, MissingItem, HasFlag, MissingFlag, HasGold, HasMasterwork}` | `internal/quests/triggers.go:26` |
| Dialogue boot door: `validateShippedDialogue()`, `dialogueDoor(probs, isReload) error`, called at `main.go:1904` | `main_dialogue.go:25,39` |
| `dialogue.LoadAllFiles() ([]*DialogueFile, []Problem)` | `internal/dialogue/world_registry.go` |
| Smoke env var `DOGMUD_BOOT_SMOKE` (`bootSmokeEnvVar`); sweep pattern `TestSmoke_ShippedDialoguePassesTheValidators` calls `loadAllDataFiles(false)` | `boot_smoke_test.go:38,212` |
| `usercommands.Quests` builds `QuestRecord{Id, Name, Description, Completion, BarFull, BarEmpty}` from `GetQuestProgress()` | `internal/usercommands/quests.go:17` |
| Text list template | `_datafiles/world/dogmud/templates/character/quests.template` |
| `Char.Quests` builder skips `end` and secret quests; payload `GMCPCharModule_Payload_Quest` | `modules/gmcp/gmcp.Char.go:632-680,1143` |
| Web panel `renderQuests` prints `<span class="qname">` + name; test `tools/webclient-tests/quest-panel-refresh.js` | `_datafiles/html/public/webclient-pure.html:957-962` |
| Help files | `_datafiles/world/dogmud/templates/help/talk.template`, `quests.template` |

## File map

| File | Change |
|---|---|
| `internal/menu/menu.go` | `Body []string` on `Menu`, rendered and wrapped |
| `internal/quests/rewardparse.go` (new) | exported reward-string parsers moved from hooks |
| `internal/quests/quests.go` | `RepeatLimit`, `Rewards.Summary`, `Rewards.Surprise` |
| `internal/quests/rewardsummary.go` (new) | `RewardNames`, `RewardSummary`, `RewardPaidNames` |
| `internal/quests/manifest.go` (new), `internal/manifest/manifest.go` (new) | step-id manifest over a shared key-manifest core |
| `internal/dialogue/manifest.go` | delegates to `internal/manifest` |
| `internal/characters/quests.go`, `character.go` | `QuestCompletions`, `QuestOpen`, `RecordCompletion`, `RenameQuestSteps` |
| `internal/hooks/Quest_HandleQuestUpdate.go` | count completions, cap refusal, use moved parsers |
| `internal/dialogue/types.go`, `topics.go` | `PlayerState.QuestOpen`, offer gate |
| `internal/events/eventtypes.go` | `TalkNextOffer` |
| `internal/conversation/talk.go`, `card.go` (new), `nextoffer.go` (new), `playerstate.go` | card body, next offer, `QuestOpen` callback |
| `internal/questworld/` (new package) | live-registry reward names, quest ref validators (moved from gmcp), the registry world |
| `internal/questloop/*` (new package) | `World`, `Finding`, rules, backlog |
| `main_quests.go` (new), `main.go` | quest boot door |
| `modules/gmcp/gmcp.Quest.go` | editor door |
| `internal/usercommands/quests.go`, `templates/character/quests.template` | `(repeatable)` tag, finished repeatables |
| `modules/gmcp/gmcp.Char.go`, `webclient-pure.html`, `static/css/dashboard.css`, `tools/webclient-tests/quest-panel-refresh.js` | `repeatable` and finished-repeatable `status` rows in `Char.Quests`, the panel's "Repeatable work" group |
| `_datafiles/world/dogmud/quests/step-ids.txt`, `loop-backlog.txt` (new) | generated manifests |
| root tests `step_manifest_test.go`, `quest_loop_smoke_test.go` (new) | guards |
| docs | seven `context.md`, two help files, requirements, roadmap, `docs/README.md` |

---

### Task 0: Worktree

**Files:** none.

- [ ] **Step 1: Create the worktree from the spec branch** (it carries the spec and this plan).

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch -q origin
git worktree add -b feat/375-phase2a-loop "C:/tmp/dogmud-375-phase2a" docs/375-phase2-spec
git -C "C:/tmp/dogmud-375-phase2a" log --oneline -3
```

Expected: the top commit is the plan commit on `docs/375-phase2-spec`.

- [ ] **Step 2: Record the worktree in memory** (the `C:/tmp` rule): add a line to the
  session handoff memory naming `C:/tmp/dogmud-375-phase2a`. Every later task runs inside it.

- [ ] **Step 3: Baseline**

```bash
cd "C:/tmp/dogmud-375-phase2a" && go build ./... && go test ./internal/menu/ ./internal/quests/ ./internal/dialogue/ ./internal/conversation/ ./internal/characters/ ./internal/hooks/ 2>&1 | tail -15
```

Expected: build OK, every package `ok`. If anything fails here, stop and report: it is not
this plan's breakage.

---

### Task 1: `menu.Menu.Body`

**Files:**
- Modify: `internal/menu/menu.go` (struct at :45, `Render` at :72)
- Test: `internal/menu/menu_test.go`

- [ ] **Step 1: Write the failing tests** (append to `internal/menu/menu_test.go`)

```go
func TestRender_BodyWrapsBetweenHeadingAndEntries(t *testing.T) {
	m := Confirm("Cull the Wash", "Accept", "Leave")
	m.Body = []string{
		"Garve has asked you to thin out the bandits in the wash again: a bandit scout first, then a bandit squatter.",
		"Reward: 6 gold, and training in weapon combat.",
	}
	got := plain(Render(m))
	want := strings.Join([]string{
		`Cull the Wash`,
		``,
		`Garve has asked you to thin out the bandits in the wash again: a bandit scout`,
		`first, then a bandit squatter.`,
		`Reward: 6 gold, and training in weapon combat.`,
		``,
		`  1. Accept`,
		`  2. Leave`,
		``,
		`Type a number, or press Enter for 2.`,
	}, "\n")
	if got != want {
		t.Fatalf("render mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
	for _, line := range strings.Split(got, "\n") {
		if n := utf8.RuneCountInString(line); n > lineWidth {
			t.Fatalf("line over %d columns (%d): %q", lineWidth, n, line)
		}
	}
}

func TestRender_NoBodyIsUnchanged(t *testing.T) {
	m := talkMenu()
	before := Render(m)
	m.Body = nil
	if Render(m) != before {
		t.Fatal("a nil Body changed the render")
	}
}
```

Add `"unicode/utf8"` to the test file's imports.

- [ ] **Step 2: Run them and see the first fail**

Run: `go test ./internal/menu/ -run 'TestRender_Body|TestRender_NoBody' -v`
Expected: FAIL to compile, `m.Body undefined`.

- [ ] **Step 3: Implement.** In `menu.go`, add to `Menu` after `Heading`:

```go
	// Body is optional paragraphs between the heading and the entries,
	// each wrapped at the 80-column width (an offer card's objective and
	// reward). The labels lint does not read it.
	Body []string
```

In `Render`, replace the heading block with:

```go
	if m.Heading != "" {
		b.WriteString(m.Heading)
		b.WriteString("\n\n")
	}
	if len(m.Body) > 0 {
		for _, para := range m.Body {
			for _, line := range wrap(para, lineWidth) {
				b.WriteString(line)
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
```

- [ ] **Step 4: Run the package**

Run: `go test ./internal/menu/`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/menu/menu.go internal/menu/menu_test.go
git commit -F - <<'EOF'
feat(menu): optional wrapped Body between heading and entries (#375 phase 2a)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: Move the reward parsers into `quests`

One parse of the reward strings, shared by the payout (hooks) and the offer card
(`RewardSummary`). Pure refactor: payout output must not change.

**Files:**
- Create: `internal/quests/rewardparse.go`, `internal/quests/rewardparse_test.go`
- Modify: `internal/hooks/Quest_HandleQuestUpdate.go` (delete `skillGrant`, `statGrant`,
  `itemGrant` and the four `parse*` funcs, lines 26-160 minus `statGainDescriptions`; update
  their call sites at about lines 343, 374, 395, 410)
- Delete: `internal/hooks/Quest_ItemGrants_test.go`, `internal/hooks/Quest_RecipeGrants_test.go`
  (their cases move to `rewardparse_test.go`)

- [ ] **Step 1: Read both hook test files in full** (`cat` them) and note every case; each
  one moves over unchanged in meaning.

- [ ] **Step 2: Create `internal/quests/rewardparse.go`**

```go
package quests

import (
	"strconv"
	"strings"
)

// SkillGrant is one parsed skill reward: a skill tag and the level it trains to.
type SkillGrant struct {
	Skill string
	Level int
}

// StatGrant is one parsed stat reward: a lowercase stat name and the amount added.
type StatGrant struct {
	Stat   string
	Amount int
}

// ItemGrant is one parsed item-stockpile reward: an item id and a quantity.
type ItemGrant struct {
	ItemId int
	Qty    int
}

// ParseSkillGrants parses a skillinfo reward, "skill:level[,skill:level]".
// Malformed entries (no colon, unparseable level, empty name) are skipped.
func ParseSkillGrants(skillInfo string) []SkillGrant {
	if skillInfo == `` {
		return nil
	}
	var grants []SkillGrant
	for _, entry := range strings.Split(skillInfo, `,`) {
		details := strings.Split(strings.TrimSpace(entry), `:`)
		if len(details) <= 1 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(details[0]))
		level, err := strconv.Atoi(strings.TrimSpace(details[1]))
		if err != nil || name == `` {
			continue
		}
		grants = append(grants, SkillGrant{Skill: name, Level: level})
	}
	return grants
}

// ParseStatGrants parses a stat_info reward, "stat:amount[,stat:amount]".
// Malformed entries (no colon, unparseable amount, empty name) are skipped.
func ParseStatGrants(statInfo string) []StatGrant {
	if statInfo == `` {
		return nil
	}
	var grants []StatGrant
	for _, entry := range strings.Split(statInfo, `,`) {
		details := strings.Split(strings.TrimSpace(entry), `:`)
		if len(details) <= 1 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(details[0]))
		amount, err := strconv.Atoi(strings.TrimSpace(details[1]))
		if err != nil || name == `` {
			continue
		}
		grants = append(grants, StatGrant{Stat: name, Amount: amount})
	}
	return grants
}

// ParseRecipeGrants parses a recipe_info reward, a comma-separated list of
// recipe ids. Empty entries are skipped.
func ParseRecipeGrants(recipeInfo string) []string {
	if recipeInfo == `` {
		return nil
	}
	var ids []string
	for _, entry := range strings.Split(recipeInfo, `,`) {
		if id := strings.TrimSpace(entry); id != `` {
			ids = append(ids, id)
		}
	}
	return ids
}

// ParseItemGrants parses an item_info reward, "itemid[:qty][,itemid[:qty]]".
// A bare id means one. Unparseable ids and non-positive quantities are skipped.
func ParseItemGrants(itemInfo string) []ItemGrant {
	if itemInfo == `` {
		return nil
	}
	var grants []ItemGrant
	for _, entry := range strings.Split(itemInfo, `,`) {
		entry = strings.TrimSpace(entry)
		if entry == `` {
			continue
		}
		details := strings.Split(entry, `:`)
		itemId, err := strconv.Atoi(strings.TrimSpace(details[0]))
		if err != nil || itemId <= 0 {
			continue
		}
		qty := 1
		if len(details) > 1 {
			q, qErr := strconv.Atoi(strings.TrimSpace(details[1]))
			if qErr != nil || q <= 0 {
				continue
			}
			qty = q
		}
		grants = append(grants, ItemGrant{ItemId: itemId, Qty: qty})
	}
	return grants
}
```

Check the moved bodies against the hook originals line by line before deleting them: the
behaviour (skip rules, trimming, lowercasing) must be identical.

- [ ] **Step 3: Create `internal/quests/rewardparse_test.go`** carrying every case from the
  two deleted hook tests, renamed to the exported names and fields (`g[0].ItemId`, `g[0].Qty`),
  plus:

```go
func TestParseSkillGrants(t *testing.T) {
	require.Nil(t, ParseSkillGrants(""))
	require.Equal(t, []SkillGrant{{Skill: "weapon-combat", Level: 1}}, ParseSkillGrants("Weapon-Combat:1"))
	require.Equal(t, []SkillGrant{{Skill: "a", Level: 2}, {Skill: "b", Level: 3}}, ParseSkillGrants(" a:2 , b:3 ,nocolon, c:x"))
}

func TestParseStatGrants(t *testing.T) {
	require.Nil(t, ParseStatGrants(""))
	require.Equal(t, []StatGrant{{Stat: "strength", Amount: 3}, {Stat: "vitality", Amount: 2}}, ParseStatGrants("Strength:3,vitality:2,bad"))
}
```

(imports: `testing`, `github.com/stretchr/testify/require`).

- [ ] **Step 4: Point hooks at the moved code.** In `Quest_HandleQuestUpdate.go` delete the
  three types and four funcs (keep `statGainDescriptions`). Replace
  `parseItemGrants(` with `quests.ParseItemGrants(`, and likewise for skill, stat, recipe.
  Then let the compiler list the field uses:

Run: `go build ./internal/hooks/`
Expected: errors naming `grant.itemId`, `grant.qty`, `grant.skill`, `grant.level`,
`grant.stat`, `grant.amount`. Rename each to `ItemId`, `Qty`, `Skill`, `Level`, `Stat`,
`Amount`. Rebuild until clean. Delete the two hook test files with `git rm`.

- [ ] **Step 5: Run**

Run: `go test ./internal/quests/ ./internal/hooks/`
Expected: `ok` for both.

- [ ] **Step 6: Commit**

```bash
git add internal/quests/rewardparse.go internal/quests/rewardparse_test.go internal/hooks/Quest_HandleQuestUpdate.go
git rm -q internal/hooks/Quest_ItemGrants_test.go internal/hooks/Quest_RecipeGrants_test.go
git commit -F - <<'EOF'
refactor(quests): one reward-string parser, moved from hooks (#375 phase 2a)

The payout and the coming offer card read the same parse.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: Quest fields and `RewardSummary`

**Files:**
- Modify: `internal/quests/quests.go` (`Quest` at :61, `QuestReward` at :36)
- Create: `internal/quests/rewardsummary.go`, `internal/quests/rewardsummary_test.go`

- [ ] **Step 1: Write the failing tests** in `internal/quests/rewardsummary_test.go`

```go
package quests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func testNames() RewardNames {
	items := map[int]string{30036: "Healing Salve", 9001: "Iron Lockpicks", 9002: "Oak Bow"}
	return RewardNames{
		Item: func(id int) string { return items[id] },
		Recipe: func(id string) string {
			return map[string]string{"iron-dagger": "Iron Dagger"}[id]
		},
		Spell:   func(id string) string { return map[string]string{"glow": "Glow"}[id] },
		Faction: func(id string) string { return map[string]string{"caravan": "the Caravan Guild"}[id] },
	}
}

func TestRewardSummary_Table(t *testing.T) {
	cases := []struct {
		name string
		r    QuestReward
		want string
	}{
		{"none", QuestReward{}, ""},
		{"gold", QuestReward{Gold: 6}, "6 gold"},
		{"gold and skill", QuestReward{Gold: 6, SkillInfo: "weapon-combat:1"}, "6 gold and training in weapon combat"},
		{"item article", QuestReward{ItemId: 9002}, "an Oak Bow"},
		{"plural item", QuestReward{ItemId: 9001}, "some Iron Lockpicks"},
		{"stockpile counts", QuestReward{ItemInfo: "30036:3,9002"}, "Healing Salve (x3) and an Oak Bow"},
		{"recipe", QuestReward{RecipeInfo: "iron-dagger"}, "the recipe for Iron Dagger"},
		{"spell", QuestReward{SpellId: "glow"}, "the spell Glow"},
		{"stat", QuestReward{StatInfo: "strength:1"}, "more Strength"},
		{"rep", QuestReward{RepFaction: "caravan", RepAmount: 5}, "the goodwill of the Caravan Guild"},
		{"rep loss hidden", QuestReward{RepFaction: "caravan", RepAmount: -5}, ""},
		{"three joins", QuestReward{Gold: 2, ItemId: 9002, SpellId: "glow"}, "2 gold, an Oak Bow, and the spell Glow"},
		{"silent kinds", QuestReward{ConditionId: 3, RoomId: 10, QuestId: "5-start"}, ""},
		{"unknown name skipped", QuestReward{ItemId: 4242, Gold: 1}, "1 gold"},
	}
	for _, c := range cases {
		q := &Quest{Rewards: c.r}
		require.Equal(t, c.want, RewardSummary(q, testNames()), c.name)
	}
}

func TestRewardSummary_OverrideWins(t *testing.T) {
	q := &Quest{Rewards: QuestReward{Gold: 6, Summary: "Garve will make it worth your while."}}
	require.Equal(t, "Garve will make it worth your while.", RewardSummary(q, testNames()))
}

func TestRewardPaidNames(t *testing.T) {
	q := &Quest{Rewards: QuestReward{ItemId: 9002, ItemInfo: "30036:2", RecipeInfo: "iron-dagger", SpellId: "glow", Gold: 4}}
	require.Equal(t, []string{"Oak Bow", "Healing Salve", "Iron Dagger", "Glow"}, RewardPaidNames(q, testNames()))
}
```

- [ ] **Step 2: Run to see it fail**

Run: `go test ./internal/quests/ -run 'TestRewardSummary|TestRewardPaidNames'`
Expected: FAIL to compile (`RewardNames` undefined).

- [ ] **Step 3: Add the fields.** In `QuestReward`, after `RepAmount`:

```go
	// Summary replaces the generated reward line on the offer card
	// (RewardSummary). The loop validator's reward rule requires it to name
	// every item, recipe and spell paid unless Surprise is set (#375 P2).
	Summary  string `yaml:"summary,omitempty" json:"summary,omitempty"`
	Surprise bool   `yaml:"surprise,omitempty" json:"surprise,omitempty"`
```

In `Quest`, after `CooldownRounds`:

```go
	RepeatLimit    int            `yaml:"repeat_limit,omitempty" json:"repeat_limit,omitempty"`       // lifetime completions of a repeatable; 0 = no cap (#375 P3)
```

- [ ] **Step 4: Create `internal/quests/rewardsummary.go`**

```go
package quests

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// RewardNames are the registry lookups RewardSummary needs, injected the way
// QuestValidators is so this package imports no registry. Each returns ""
// for an unknown id, and that part of the line is left out (the refs
// validator reports the bad id).
type RewardNames struct {
	Item    func(id int) string    // display name, "Healing Salve"
	Recipe  func(id string) string // recipe name
	Spell   func(id string) string // spell name
	Faction func(id string) string // faction display name
}

// RewardSummary is the offer card's reward text (#375 P2): the authored
// Rewards.Summary when set, else the completion payout in words. Gold and
// stockpile counts are the only numbers it shows. "" means nothing to show.
func RewardSummary(q *Quest, n RewardNames) string {
	if q == nil {
		return ""
	}
	if s := strings.TrimSpace(q.Rewards.Summary); s != "" {
		return s
	}
	r := q.Rewards
	var parts []string
	if r.Gold > 0 {
		parts = append(parts, fmt.Sprintf("%d gold", r.Gold))
	}
	if name := lookupInt(n.Item, r.ItemId); name != "" {
		parts = append(parts, articled(name))
	}
	for _, g := range ParseItemGrants(r.ItemInfo) {
		name := lookupInt(n.Item, g.ItemId)
		if name == "" {
			continue
		}
		if g.Qty > 1 {
			parts = append(parts, fmt.Sprintf("%s (x%d)", name, g.Qty))
		} else {
			parts = append(parts, articled(name))
		}
	}
	for _, id := range ParseRecipeGrants(r.RecipeInfo) {
		if name := lookupStr(n.Recipe, id); name != "" {
			parts = append(parts, "the recipe for "+name)
		}
	}
	if name := lookupStr(n.Spell, r.SpellId); name != "" {
		parts = append(parts, "the spell "+name)
	}
	for _, g := range ParseSkillGrants(r.SkillInfo) {
		parts = append(parts, "training in "+strings.ReplaceAll(g.Skill, "-", " "))
	}
	for _, g := range ParseStatGrants(r.StatInfo) {
		if g.Amount > 0 && g.Stat != "" {
			parts = append(parts, "more "+strings.ToUpper(g.Stat[:1])+g.Stat[1:])
		}
	}
	if r.RepAmount > 0 {
		if name := lookupStr(n.Faction, r.RepFaction); name != "" {
			parts = append(parts, "the goodwill of "+name)
		}
	}
	return joinParts(parts)
}

// RewardPaidNames are the names of every item, recipe and spell the quest
// pays on completion, in RewardSummary's order: what an authored Summary
// must mention unless the reward is a surprise.
func RewardPaidNames(q *Quest, n RewardNames) []string {
	var out []string
	if q == nil {
		return out
	}
	r := q.Rewards
	if name := lookupInt(n.Item, r.ItemId); name != "" {
		out = append(out, name)
	}
	for _, g := range ParseItemGrants(r.ItemInfo) {
		if name := lookupInt(n.Item, g.ItemId); name != "" {
			out = append(out, name)
		}
	}
	for _, id := range ParseRecipeGrants(r.RecipeInfo) {
		if name := lookupStr(n.Recipe, id); name != "" {
			out = append(out, name)
		}
	}
	if name := lookupStr(n.Spell, r.SpellId); name != "" {
		out = append(out, name)
	}
	return out
}

func lookupInt(f func(int) string, id int) string {
	if f == nil || id <= 0 {
		return ""
	}
	return f(id)
}

func lookupStr(f func(string) string, id string) string {
	if f == nil || id == "" {
		return ""
	}
	return f(id)
}

// articled puts "a", "an" or "some" before an item name, by the one rule
// the game uses (items.ArticlePrefix).
func articled(name string) string {
	return items.ArticlePrefix(name) + name
}

// joinParts joins "A", "A and B", "A, B, and C".
func joinParts(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + ", and " + parts[len(parts)-1]
}
```

The import block is `fmt`, `strings` and `github.com/GoMudEngine/GoMud/internal/items`.
`items` does not depend on `quests` (`go list -deps ./internal/items`, checked 2026-10-24),
so the import makes no cycle; `go build ./...` confirms.

- [ ] **Step 5: Run**

Run: `go test ./internal/quests/`
Expected: `ok` (the round-trip test still passes: the new fields are `omitempty`).

- [ ] **Step 6: Commit**

```bash
git add internal/quests/quests.go internal/quests/rewardsummary.go internal/quests/rewardsummary_test.go
git commit -F - <<'EOF'
feat(quests): repeat_limit, reward summary override, RewardSummary (#375 P2, P3)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: Completion count, `QuestOpen`, step renames on the character

**Files:**
- Modify: `internal/characters/character.go:319` (field), `internal/characters/quests.go`
- Test: `internal/characters/quests_repeat_test.go` (new)

- [ ] **Step 1: Write the failing tests**

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestQuestOpen_NonRepeatableAlwaysOpen(t *testing.T) {
	c := &Character{}
	require.True(t, c.QuestOpen(&quests.Quest{QuestId: 9}))
}

func TestQuestOpen_CooldownAndCap(t *testing.T) {
	util.SetRoundCountForTest(1000)
	q := &quests.Quest{QuestId: 53, Repeatable: true, CooldownRounds: 300, RepeatLimit: 2}
	c := &Character{}
	require.True(t, c.QuestOpen(q))

	c.RecordCompletion(q)
	require.Equal(t, 1, c.QuestCompletions[53])
	require.False(t, c.QuestOpen(q), "on cooldown")

	util.SetRoundCountForTest(1300)
	require.True(t, c.QuestOpen(q), "cooldown over, one run left")
	require.True(t, c.LastRepeat(q), "the next run is the last")

	c.RecordCompletion(q)
	util.SetRoundCountForTest(5000)
	require.False(t, c.QuestOpen(q), "at the cap")
	require.True(t, c.RepeatCapReached(q))
}

func TestQuestOpen_NoLimitNeverCaps(t *testing.T) {
	util.SetRoundCountForTest(1000)
	q := &quests.Quest{QuestId: 54, Repeatable: true, CooldownRounds: 1}
	c := &Character{}
	for i := 0; i < 50; i++ {
		c.RecordCompletion(q)
	}
	util.SetRoundCountForTest(2000)
	require.True(t, c.QuestOpen(q))
	require.False(t, c.LastRepeat(q))
}

func TestQuestCompletions_YAMLRoundTrip(t *testing.T) {
	c := &Character{QuestCompletions: map[int]int{53: 4}}
	out, err := yaml.Marshal(c)
	require.NoError(t, err)
	require.Contains(t, string(out), "questcompletions:")
	var back Character
	require.NoError(t, yaml.Unmarshal(out, &back))
	require.Equal(t, 4, back.QuestCompletions[53])
}

func TestRenameQuestSteps(t *testing.T) {
	c := &Character{QuestProgress: map[int]string{53: "firstkill", 7: "deliver"}}
	c.RenameQuestSteps(map[string]string{"53-firstkill": "53-scout", "53-scout": "53-scoutdown"})
	require.Equal(t, "scoutdown", c.QuestProgress[53], "chains are followed")
	require.Equal(t, "deliver", c.QuestProgress[7])
}
```

- [ ] **Step 2: Run to see it fail**

Run: `go test ./internal/characters/ -run 'TestQuestOpen|TestQuestCompletions|TestRenameQuestSteps'`
Expected: FAIL to compile.

- [ ] **Step 3: Add the field** in `character.go` after `QuestProgress` (line 319):

```go
	QuestCompletions        map[int]int                    `yaml:"questcompletions,omitempty"`    // completed runs of each repeatable quest (#375 P3)
```

Keep the column alignment gofmt gives.

- [ ] **Step 4: Add the methods** to `internal/characters/quests.go` (after
  `QuestCooldownActive`):

```go
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
func (c *Character) RenameQuestSteps(renames map[string]string) {
	if len(renames) == 0 || len(c.QuestProgress) == 0 {
		return
	}
	for qid, step := range c.QuestProgress {
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
			c.QuestProgress[qid] = nstep
		}
	}
}
```

Note: `RecordCompletion` stamps the cooldown itself, so Task 5 replaces the hook's separate
`SetQuestCooldown` call with it.

- [ ] **Step 5: Run**

Run: `go test ./internal/characters/`
Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/characters/character.go internal/characters/quests.go internal/characters/quests_repeat_test.go
git commit -F - <<'EOF'
feat(characters): saved completion counts, QuestOpen, step renames (#375 P3)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: Count completions and refuse at the cap in the quest hook

**Files:**
- Modify: `internal/hooks/Quest_HandleQuestUpdate.go:206-216` and `:446-452`
- Test: `internal/hooks/Quest_RepeatLimit_test.go` (new)

- [ ] **Step 1: Read the setup the new tests copy.** `Quest_MarkerRefresh_test.go` (35 lines)
  drives `HandleQuestUpdate` with `users.NewTestUser`, `users.SeedUsersForTest` and a fixture
  quest; `drainEventQueueOnCleanup(t)` lives in `Quest_HandleQuestUpdate_bridge_test.go:57`.
  `quests.SeedQuestsForTest(qs...)` (`internal/quests/test_helpers.go:9`) seeds a quest without
  files. `events.DrainQueuedMessagesForTest(userId) []string` (`internal/events/events.go:442`)
  returns the user's queued lines. Read all three before writing.

- [ ] **Step 2: Write the failing tests** in `Quest_RepeatLimit_test.go`

```go
package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

func repeatQuest(limit int) *quests.Quest {
	return &quests.Quest{QuestId: 999053, Name: "Cull Test", Repeatable: true,
		CooldownRounds: 10, RepeatLimit: limit,
		Steps: []quests.QuestStep{{Id: "start", Hint: "h"}, {Id: "end"}}}
}

func runOnce(t *testing.T, userId int) {
	t.Helper()
	HandleQuestUpdate(events.Quest{UserId: userId, QuestToken: "999053-start"})
	HandleQuestUpdate(events.Quest{UserId: userId, QuestToken: "999053-end"})
}

func TestRepeatLimit_CountsEachCompletion(t *testing.T) {
	drainEventQueueOnCleanup(t)
	defer quests.SeedQuestsForTest(repeatQuest(2))()
	u := users.NewTestUser(531, "repeatuser", "RepeatUser", 5031)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{531: u})()
	util.SetRoundCountForTest(1000)

	runOnce(t, 531)
	require.Equal(t, 1, u.Character.QuestCompletions[999053])
	require.False(t, u.Character.HasQuest("999053-start"), "progress cleared for the repeat")
	require.True(t, u.Character.QuestCooldownActive(999053))
}

func TestRepeatLimit_RefusesStartAtCap(t *testing.T) {
	drainEventQueueOnCleanup(t)
	defer quests.SeedQuestsForTest(repeatQuest(2))()
	u := users.NewTestUser(532, "capuser", "CapUser", 5032)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{532: u})()
	u.Character.QuestCompletions = map[int]int{999053: 2}
	util.SetRoundCountForTest(5000)
	events.DrainQueuedMessagesForTest(532)

	HandleQuestUpdate(events.Quest{UserId: 532, QuestToken: "999053-start"})
	require.False(t, u.Character.HasQuest("999053-start"))
	lines := strings.Join(events.DrainQueuedMessagesForTest(532), "\n")
	require.Contains(t, lines, "You have done all of this work there is to do.")
}

func TestRepeatLimit_NoLimitKeepsRepeating(t *testing.T) {
	drainEventQueueOnCleanup(t)
	defer quests.SeedQuestsForTest(repeatQuest(0))()
	u := users.NewTestUser(533, "againuser", "AgainUser", 5033)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{533: u})()
	for i, round := range []uint64{1000, 1100, 1200} {
		util.SetRoundCountForTest(round)
		runOnce(t, 533)
		require.Equal(t, i+1, u.Character.QuestCompletions[999053])
	}
}
```

Check the `users.NewTestUser` signature in `Quest_MarkerRefresh_test.go` matches
(user id, username, character name, character id) before running.

- [ ] **Step 3: Run to see them fail**

Run: `go test ./internal/hooks/ -run TestRepeatLimit -v`
Expected: the count test fails (`QuestCompletions` empty), the cap test fails (start lands).

- [ ] **Step 4: Implement.** Replace the cooldown block at :206 with:

```go
	// Repeatable-quest gate: refuse to (re)start a repeatable quest on its
	// post-completion cooldown or at its lifetime repeat_limit. The talk
	// menu already hides such an offer (PlayerState.QuestOpen); this is the
	// backstop for any other grant path.
	{
		_, incomingStep := quests.TokenToParts(evt.QuestToken)
		if incomingStep == `start` && questInfo.Repeatable {
			if questUser.Character.RepeatCapReached(questInfo) {
				questUser.SendText(messaging.CategoryRefusal,
					`You have done all of this work there is to do.`)
				return events.Continue
			}
			if questUser.Character.QuestCooldownActive(questInfo.QuestId) {
				questUser.SendText(messaging.CategoryRefusal,
					`You have put in the work recently. Rest a while before taking this on again.`)
				return events.Continue
			}
		}
	}
```

Replace the reset block at :446 with:

```go
		// Repeatable quest: reset progress so it can be taken again, count
		// the run and stamp its cooldown (RecordCompletion). Non-repeatable
		// quests are unaffected.
		if questInfo.Repeatable {
			questUser.Character.ClearQuestToken(evt.QuestToken)
			questUser.Character.RecordCompletion(questInfo)
		}
```

`questInfo` is the `*quests.Quest` from `quests.GetQuest` at :193; check the type with
`go build` and adjust nothing else.

- [ ] **Step 5: Run**

Run: `go test ./internal/hooks/`
Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/hooks/Quest_HandleQuestUpdate.go internal/hooks/Quest_RepeatLimit_test.go
git commit -F - <<'EOF'
feat(hooks): count repeatable completions, refuse a start at the cap (#375 P3)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 6: `PlayerState.QuestOpen` gates offers

**Files:**
- Modify: `internal/dialogue/types.go` (`PlayerState`), `internal/dialogue/topics.go:29`
  (`Eligible`)
- Modify: every `PlayerState{` literal listed below
- Test: `internal/dialogue/questopen_test.go` (new)

- [ ] **Step 1: Write the failing test**

```go
package dialogue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func offerFile() *DialogueFile {
	return &DialogueFile{MobId: 9112, Topics: []Topic{
		{Key: "cull_offer", Kind: KindOffer, Quest: 53, Label: "Cull the Wash", Text: "t"},
		{Key: "chat", Kind: KindInfo, Label: "Chat", Text: "t"},
	}}
}

func TestEligible_OfferHiddenWhenQuestNotOpen(t *testing.T) {
	df := offerFile()
	open := false
	ps := &PlayerState{
		HasQuest:  func(string) bool { return false },
		HasHeard:  func(string) bool { return false },
		QuestOpen: func(id int) bool { require.Equal(t, 53, id); return open },
	}
	require.False(t, df.Eligible(&df.Topics[0], ps))
	require.True(t, df.Eligible(&df.Topics[1], ps), "info topics ignore QuestOpen")
	require.Equal(t, "", df.Marker(ps), "no [!] while the quest is not open")
	open = true
	require.True(t, df.Eligible(&df.Topics[0], ps))
}

func TestEligible_NilQuestOpenFailsOffersClosed(t *testing.T) {
	df := offerFile()
	ps := &PlayerState{HasQuest: func(string) bool { return false }, HasHeard: func(string) bool { return false }}
	require.False(t, df.Eligible(&df.Topics[0], ps))
}
```

- [ ] **Step 2: Run to see it fail**

Run: `go test ./internal/dialogue/ -run TestEligible_ -v`
Expected: FAIL to compile (`QuestOpen` unknown field).

- [ ] **Step 3: Implement.** Add to `PlayerState` in `types.go`, after `MarkHeard`:

```go
	// QuestOpen reports whether a quest can be offered now: false for a
	// repeatable on its cooldown or at its repeat_limit (#375 P3). Eligible
	// asks it for every offer topic with a quest; nil fails those closed,
	// like every interrogative callback here.
	QuestOpen func(questId int) bool
```

In `Eligible` (`topics.go`), after the `if ps == nil { return true }` line, add:

```go
	if t.Kind == KindOffer && t.Quest > 0 && (ps.QuestOpen == nil || !ps.QuestOpen(t.Quest)) {
		return false
	}
```

- [ ] **Step 4: Run the new tests**

Run: `go test ./internal/dialogue/ -run TestEligible_ -v`
Expected: PASS.

- [ ] **Step 5: Give every existing PlayerState literal the callback.** List them:

Run: `grep -rn "PlayerState{" --include=*.go . | grep -v "internal/conversation/playerstate.go"`
Expected: 32 lines in `boot_smoke_test.go`, `veyra_secrets_dialogue_test.go`,
`internal/dialogue/{bumpsrep,charges_gold,delivery_askpath,giveitem_failure,gold_gate,integration_dialogue,item_excluded,masterwork_gate,playerstate_guards,toke_ways,topics,velk_opener}_test.go`,
`internal/questengine/{newbie_topic_steps,quest28_pacing}_test.go`. Some are `&dialogue.PlayerState{`
in other packages and `&PlayerState{` inside `dialogue`.

To each literal that sets any callback, add the line
`QuestOpen: func(int) bool { return true },` (with the `dialogue.` prefix not needed: it is a
field). Leave an empty literal `PlayerState{}` alone unless its test then fails. A literal in
a test that asserts an offer is hidden for another reason keeps the callback too: the reason
under test must not change.

- [ ] **Step 6: Run every package that holds a literal**

Run: `go test ./internal/dialogue/ ./internal/questengine/ . -run . 2>&1 | tail -20`
Expected: `ok` for all three. The root package runs without `DOGMUD_BOOT_SMOKE`, so the
smoke sweeps skip; Task 13 runs them.

- [ ] **Step 7: Commit**

```bash
git add internal/dialogue/types.go internal/dialogue/topics.go internal/dialogue/questopen_test.go
git add $(git diff --name-only -- '*_test.go')
git status --short
git commit -F - <<'EOF'
feat(dialogue): PlayerState.QuestOpen hides offers on cooldown or at the cap (#375 P3)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

Check `git status --short` before committing: only the files this task touched are staged.

---

### Task 7: The offer card, the `QuestOpen` callback and production reward names

**Files:**
- Create: `internal/questworld/names.go`, `internal/conversation/card.go`,
  `internal/conversation/card_test.go`
- Modify: `internal/conversation/playerstate.go` (add `QuestOpen`), `internal/conversation/talk.go:320` (`offerCard`)

- [ ] **Step 1: Read the existing conversation tests' setup**

Run: `ls internal/conversation/` and `grep -n "func Test" internal/conversation/*_test.go | head -30`

Read `internal/conversation/playerstate_test.go` in full; its user construction is the
pattern for anything here that needs a `*users.UserRecord`. The card text itself is built by
a pure function, so most of this task's tests need no user.

- [ ] **Step 2: Write the failing tests** in `card_test.go`

```go
package conversation

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

func cullQuest() *quests.Quest {
	return &quests.Quest{
		QuestId: 53, Name: "Cull the Wash", Repeatable: true, CooldownRounds: 300,
		Steps: []quests.QuestStep{
			{Id: "start", Description: "Garve has asked you to thin out the bandits in the wash again: a bandit scout first, then a bandit squatter."},
			{Id: "end"},
		},
		Rewards: quests.QuestReward{Gold: 6, SkillInfo: "weapon-combat:1"},
	}
}

func TestCardBody_ObjectiveRewardAndRepeat(t *testing.T) {
	body := cardBody(cullQuest(), quests.RewardNames{}, false)
	require.Equal(t, []string{
		"Garve has asked you to thin out the bandits in the wash again: a bandit scout first, then a bandit squatter.",
		"Reward: 6 gold and training in weapon combat.",
		"You can take this on again after a while.",
	}, body)
}

func TestCardBody_LastRun(t *testing.T) {
	body := cardBody(cullQuest(), quests.RewardNames{}, true)
	require.Equal(t, "This is the last time this work will be offered to you.", body[len(body)-1])
}

func TestCardBody_NotRepeatableNoRewardNoObjective(t *testing.T) {
	q := &quests.Quest{QuestId: 9, Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}}
	require.Empty(t, cardBody(q, quests.RewardNames{}, false))
}

func TestCardBody_NilQuest(t *testing.T) {
	require.Empty(t, cardBody(nil, quests.RewardNames{}, false))
}
```

- [ ] **Step 3: Run to see it fail**

Run: `go test ./internal/conversation/ -run 'TestCardBody' -v`
Expected: FAIL to compile (`cardBody` undefined).

- [ ] **Step 4: Create `internal/questworld/names.go`** (a new package: the live-registry
  side of the quest validators. Task 14 adds its validators and world; it ships a
  `context.md` in Task 18). `questworld` imports registries that none of them can import back:
  `go list -deps` of mobs, rooms, conditions, items, spells, skills, crafting, factions,
  dialogue and quests reaches neither `conversation` nor `questworld` (checked 2026-10-24).

```go
// Package questworld builds the quest validators' inputs from the live
// registries: reward names, quest reference checks and the proposed world.
// It is the one place the offer card, the boot door and the quest editor
// get them (#375 phase 2).
package questworld

import (
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// Names is quests.RewardNames over the live registries.
func Names() quests.RewardNames {
	return quests.RewardNames{
		Item: func(id int) string {
			if s := items.GetItemSpec(id); s != nil {
				return s.Name
			}
			return ""
		},
		Recipe: func(id string) string {
			if r := crafting.GetRecipe(id); r != nil {
				return r.Name
			}
			return ""
		},
		Spell: func(id string) string {
			if s := spells.GetSpell(id); s != nil {
				return s.Name
			}
			return ""
		},
		Faction: func(id string) string {
			if f := factions.GetDefinition(id); f != nil {
				return f.Name
			}
			return ""
		},
	}
}
```

Check the four `Name` fields exist with `go build ./internal/questworld/` (facts table).

- [ ] **Step 5: Create `internal/conversation/card.go`**

```go
package conversation

import "github.com/GoMudEngine/GoMud/internal/quests"

// The repeat lines on an offer card (#375 P6). No round count: the
// player-copy rule.
const (
	cardRepeatLine = "You can take this on again after a while."
	cardLastLine   = "This is the last time this work will be offered to you."
)

// cardBody is the offer card's body (spec 3, 4.3): the quest's first-step
// description as the objective, the reward line, and for a repeatable the
// repeat line. lastRun picks the last-time line.
func cardBody(q *quests.Quest, names quests.RewardNames, lastRun bool) []string {
	if q == nil {
		return nil
	}
	var body []string
	if len(q.Steps) > 0 && q.Steps[0].Description != "" {
		body = append(body, q.Steps[0].Description)
	}
	if r := quests.RewardSummary(q, names); r != "" {
		body = append(body, "Reward: "+r+".")
	}
	if q.Repeatable {
		if lastRun {
			body = append(body, cardLastLine)
		} else {
			body = append(body, cardRepeatLine)
		}
	}
	return body
}
```

A Summary ending in its own full stop must not get a second one. Change the reward line to:

```go
	if r := quests.RewardSummary(q, names); r != "" {
		if last := r[len(r)-1]; last != '.' && last != '!' && last != '?' {
			r += "."
		}
		body = append(body, "Reward: "+r)
	}
```

and add a case to `card_test.go`:

```go
func TestCardBody_SummaryKeepsItsOwnStop(t *testing.T) {
	q := cullQuest()
	q.Repeatable = false
	q.Rewards.Summary = "Garve will make it worth your while."
	require.Equal(t, "Reward: Garve will make it worth your while.", cardBody(q, quests.RewardNames{}, false)[1])
}
```

- [ ] **Step 6: Wire the card.** In `talk.go`, change `offerCard` so the card carries the body:

```go
func offerCard(user *users.UserRecord, mob *mobs.Mob, t *dialogue.Topic, prefer uuid.UUID) {
	inst, key := mob.InstanceId, t.Key
	card := menu.Confirm(t.Label, "Accept", "Leave")
	if q := quests.GetQuestById(t.Quest); q != nil {
		card.Body = cardBody(q, questworld.Names(), user.Character.LastRepeat(q))
	}
	user.OpenMenu(card, session(user, mob, func(e menu.Entry) {
```

(the rest of the function is unchanged). Add the `quests` and `questworld` imports.

- [ ] **Step 7: The production callback.** In `playerstate.go`, add to the literal after
  `MarkHeard`:

```go
		QuestOpen: func(questId int) bool {
			return user.Character.QuestOpen(quests.GetQuestById(questId))
		},
```

and the `quests` import. `Character.QuestOpen(nil)` returns true, so an offer naming a quest
that does not exist is left to the validators, as today.

- [ ] **Step 8: A wiring test** in `card_test.go` that the production `PlayerState` sets the
  callback (copy the user setup from `playerstate_test.go`):

```go
func TestPlayerState_SetsQuestOpen(t *testing.T) {
	user := newTestUser(t) // the helper playerstate_test.go uses; use its real name
	require.NotNil(t, PlayerState(user).QuestOpen)
}
```

- [ ] **Step 9: Run**

Run: `go test ./internal/conversation/ ./internal/dialogue/`
Expected: `ok`.

- [ ] **Step 10: Commit**

```bash
git add internal/questworld/names.go internal/conversation/card.go internal/conversation/card_test.go internal/conversation/talk.go internal/conversation/playerstate.go
git commit -F - <<'EOF'
feat(conversation): offer card shows objective, reward and repeat line (#375 P2, P6)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 8: Hand-in plays the next offer

The pitch must land after the hand-in's completion banner and reward lines, which print when
the queued `events.Quest` is handled. So the hand-in queues a `TalkNextOffer` event at a
priority after ordinary lines (0) and before ShowLast menus (50), and its listener pitches.

**Files:**
- Modify: `internal/events/eventtypes.go` (new type), `internal/hooks/hooks.go:104` (register)
- Create: `internal/conversation/nextoffer.go`, `internal/conversation/nextoffer_test.go`
- Modify: `internal/conversation/talk.go` (`resolve`, after `play` for a hand-in)

- [ ] **Step 1: Write the failing pure test** for choosing the next offer, in
  `nextoffer_test.go`:

```go
package conversation

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
)

func toke() *dialogue.DialogueFile {
	return &dialogue.DialogueFile{MobId: 9116, Topics: []dialogue.Topic{
		{Key: "footing_in", Kind: dialogue.KindHandIn, Quest: 31, Label: "Find Your Footing", Text: "t", Effects: dialogue.Effects{GrantsQuest: "31-end"}},
		{Key: "blood", Kind: dialogue.KindOffer, Quest: 32, Label: "First Blood", Text: "t", Gates: dialogue.Gates{QuestRequired: []string{"31-end"}, QuestExcluded: []string{"32-start"}}},
		{Key: "heat", Kind: dialogue.KindOffer, Quest: 35, Label: "First Heat", Text: "t", Gates: dialogue.Gates{QuestRequired: []string{"31-end"}, QuestExcluded: []string{"35-start"}}},
		{Key: "other", Kind: dialogue.KindOffer, Quest: 60, Label: "Other", Text: "t"},
	}}
}

func held(tokens ...string) *dialogue.PlayerState {
	set := map[string]bool{}
	for _, t := range tokens {
		set[t] = true
	}
	return &dialogue.PlayerState{
		HasQuest:  func(t string) bool { return set[t] },
		HasHeard:  func(string) bool { return false },
		QuestOpen: func(int) bool { return true },
	}
}

func TestNextOffers_DerivedFromGatesOnTheFinalToken(t *testing.T) {
	got := nextOffers(toke(), "31-end", held("31-end"))
	require.Equal(t, []string{"blood", "heat"}, got, "an ungated offer is not a next offer")
}

func TestNextOffers_OneLeftIsPitched(t *testing.T) {
	got := nextOffers(toke(), "31-end", held("31-end", "32-start"))
	require.Equal(t, []string{"heat"}, got)
}

func TestNextOffers_NoneWhenNotEligible(t *testing.T) {
	require.Empty(t, nextOffers(toke(), "31-end", held()))
}
```

- [ ] **Step 2: Run to see it fail**

Run: `go test ./internal/conversation/ -run TestNextOffers -v`
Expected: FAIL to compile.

- [ ] **Step 3: Add the event** to `internal/events/eventtypes.go`, beside `Quest`:

```go
// TalkNextOffer asks the talk flow to pitch an NPC's next quest offer after
// a hand-in (#375 phase 2). It is queued at TalkNextOfferPriority, so it runs
// after the hand-in's completion banner and reward lines (priority 0) and
// before a menu shown last (priority 50).
type TalkNextOffer struct {
	UserId        int
	MobInstanceId int
	TopicKey      string
}

func (t TalkNextOffer) Type() string { return `TalkNextOffer` }

// TalkNextOfferPriority is the queue priority of TalkNextOffer.
const TalkNextOfferPriority = 45
```

If `eventtypes.go` declares event types in a different style (for example, a `Type()` on a
pointer receiver), match the file.

- [ ] **Step 4: Create `internal/conversation/nextoffer.go`**

```go
package conversation

import (
	"slices"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// nextOffers are the keys of df's offers unlocked by finalToken (P1: a
// chain is an offer whose questRequired names the finished quest's final
// token), eligible for ps, in authored order.
func nextOffers(df *dialogue.DialogueFile, finalToken string, ps *dialogue.PlayerState) []string {
	if df == nil || finalToken == "" {
		return nil
	}
	var out []string
	for i := range df.Topics {
		t := &df.Topics[i]
		if t.Kind != dialogue.KindOffer || !slices.Contains(t.QuestRequired, finalToken) {
			continue
		}
		if df.Eligible(t, ps) {
			out = append(out, t.Key)
		}
	}
	return out
}

// queueNextOffer runs after a hand-in that granted finalToken: when exactly
// one next offer opened on this NPC, it queues that offer's pitch to run
// after the completion lines. It reports whether it queued one; when it did
// not, the caller re-shows the menu as before.
func queueNextOffer(user *users.UserRecord, mobInstanceId int, df *dialogue.DialogueFile, finalToken string) bool {
	keys := nextOffers(df, finalToken, heldAlso(PlayerState(user), finalToken))
	if len(keys) != 1 {
		return false
	}
	events.AddToQueue(events.TalkNextOffer{
		UserId:        user.UserId,
		MobInstanceId: mobInstanceId,
		TopicKey:      keys[0],
	}, events.TalkNextOfferPriority)
	return true
}

// HandleTalkNextOffer pitches the queued next offer as if the player had
// chosen it: their line, the pitch, then the card. If the NPC is gone or the
// offer no longer applies, the talk menu is re-shown (or nothing, as
// openMenu decides).
func HandleTalkNextOffer(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.TalkNextOffer)
	if !ok {
		return events.Continue
	}
	user := users.GetByUserId(evt.UserId)
	if user == nil {
		return events.Continue
	}
	choose(user, evt.MobInstanceId, evt.TopicKey)
	return events.Continue
}
```

`choose` already re-checks presence and eligibility, then calls `resolve`, which pitches an
offer and shows its card. That is the one code path the spec asks for.

- [ ] **Step 5: Call it from the hand-in.** In `resolve` (`talk.go`), replace the tail:

```go
	granted, applied := play(user, room, mob, df, t, ps, buf)
	if applied && t.Behavior {
		return // a timed scene is under way: the menu closes (R18)
	}
	if applied && t.Kind == dialogue.KindHandIn && queueNextOffer(user, mob.InstanceId, df, granted) {
		return // the next offer's pitch and card follow the completion lines
	}
	openMenu(user, mob.InstanceId, granted)
```

A hand-in that grants a mid-quest token (not the final one) finds no offer gated on it and
falls through to `openMenu`, as before.

- [ ] **Step 6: Register the listener** in `internal/hooks/hooks.go`, next to the
  `events.Quest{}` registration:

```go
	events.RegisterListener(events.TalkNextOffer{}, conversation.HandleTalkNextOffer)
```

Check `hooks` already imports `conversation` (`Quest_HandleQuestUpdate.go` calls
`conversation.RefreshMarkers`); `go build ./internal/hooks/` confirms.

- [ ] **Step 7: Event-order test** in `nextoffer_test.go`: prove the event sorts after a
  priority-0 line queued later in the same pass.

```go
func TestTalkNextOffer_RunsAfterCompletionLines(t *testing.T) {
	require.Less(t, 0, events.TalkNextOfferPriority)
	require.Less(t, events.TalkNextOfferPriority, 50, "before a ShowLast menu (users.menuShowLastPriority)")
}
```

Add the `events` import. If `internal/events` has a test helper that drains the queue in order
(`grep -n "ForTest" internal/events/*.go`), also queue a `TalkNextOffer` then a priority-0
`events.Message` and assert the drained order is Message first; otherwise the priority
assertion above stands.

- [ ] **Step 8: Run**

Run: `go test ./internal/conversation/ ./internal/events/ ./internal/hooks/`
Expected: `ok`.

- [ ] **Step 9: Commit**

```bash
git add internal/events/eventtypes.go internal/hooks/hooks.go internal/conversation/nextoffer.go internal/conversation/nextoffer_test.go internal/conversation/talk.go
git commit -F - <<'EOF'
feat(conversation): a hand-in pitches the one next offer it unlocked (#375 P1)

Queued after the completion lines, so the pitch follows the reward.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 9: One key-manifest core for topics and steps

`dialogue.ParseManifest` and `Diff` are generic apart from the file name in their errors. The
step-id manifest reuses them through a small shared package instead of copying them.

**Files:**
- Create: `internal/manifest/manifest.go`, `internal/manifest/manifest_test.go`,
  `internal/manifest/context.md`
- Modify: `internal/dialogue/manifest.go`

- [ ] **Step 1: Write the failing test** `internal/manifest/manifest_test.go`

```go
package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse_SectionsAndErrorsNameTheFile(t *testing.T) {
	m, err := Parse("step-ids.txt", []byte("# header\n53-start\n53-end\n# retired\n53-old\n# renamed\n53-a -> 53-b\n"))
	require.NoError(t, err)
	require.Equal(t, []string{"53-end", "53-start"}, m.Keys)
	require.Equal(t, []string{"53-old"}, m.Retired)
	require.Equal(t, map[string]string{"53-a": "53-b"}, m.Renamed)

	_, err = Parse("step-ids.txt", []byte("53-start\n53-start\n"))
	require.ErrorContains(t, err, "step-ids.txt line 2")
}

func TestDiff_MissingAndUnlisted(t *testing.T) {
	m := Manifest{Keys: []string{"a", "b", "gone"}, Retired: []string{"old"}, Renamed: map[string]string{"r": "b"}}
	missing, unlisted := m.Diff([]string{"a", "b", "new"})
	require.Equal(t, []string{"gone"}, missing)
	require.Equal(t, []string{"new"}, unlisted)
}
```

- [ ] **Step 2: Run to see it fail**

Run: `go test ./internal/manifest/`
Expected: FAIL (no Go files / undefined).

- [ ] **Step 3: Create `internal/manifest/manifest.go`** by moving `Manifest`, `ParseManifest`
  (renamed `Parse`, with a `name string` first parameter used in every error in place of the
  literal `topic-keys.txt`), `validKey` and `Diff` out of `internal/dialogue/manifest.go`.
  Package doc:

```go
// Package manifest is the committed stable-key list format shared by the
// talk topic keys (dialogue/topic-keys.txt) and the quest step ids
// (quests/step-ids.txt): keys, then optional "# retired" and "# renamed"
// ("old -> new") sections. Saved characters point at these keys, so a key
// leaves content only through a retired or renamed line.
package manifest
```

The body is the existing code with `fmt.Errorf("topic-keys.txt line %d: ...", n+1, ...)`
becoming `fmt.Errorf("%s line %d: ...", name, n+1, ...)`. Imports: `bytes`, `fmt`, `sort`,
`strings`.

- [ ] **Step 4: Make `dialogue` delegate.** `internal/dialogue/manifest.go` keeps its public
  API so callers and `topic_manifest_test.go` are untouched:

```go
// Manifest is the committed list of shipped topic keys
// (dialogue/topic-keys.txt); the format lives in internal/manifest.
type Manifest = manifest.Manifest

// ParseManifest reads topic-keys.txt.
func ParseManifest(data []byte) (Manifest, error) {
	return manifest.Parse("topic-keys.txt", data)
}
```

`HeardRenames` stays as it is. Remove the now-unused imports.

- [ ] **Step 5: `internal/manifest/context.md`**

```markdown
# Manifest Context

## Purpose

`internal/manifest` is the stable-key list format two committed files share:
`_datafiles/world/dogmud/dialogue/topic-keys.txt` (talk topic keys) and
`_datafiles/world/dogmud/quests/step-ids.txt` (quest step ids). Saved characters
hold these keys, so a key may leave content only through a `# retired` line or a
`# renamed` line (`old -> new`).

## Files

- **manifest.go**: `Manifest{Keys, Retired, Renamed}`, `Parse(name, data)` (name
  is the file name errors quote), `Manifest.Diff(content) (missing, unlisted)`.

## Callers

- `dialogue.ParseManifest`, `dialogue.HeardRenames` (topic keys).
- `quests.ParseStepManifest`, `quests.StepRenames` (step ids).
- Root guards `topic_manifest_test.go`, `step_manifest_test.go`.
```

- [ ] **Step 6: Run**

Run: `go test ./internal/manifest/ ./internal/dialogue/ . -run 'Manifest|TopicKey'`
Expected: `ok` for all three.

- [ ] **Step 7: Commit**

```bash
git add internal/manifest/manifest.go internal/manifest/manifest_test.go internal/manifest/context.md internal/dialogue/manifest.go
git commit -F - <<'EOF'
refactor(manifest): one stable-key manifest format for topics and steps (#375 phase 2a)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 10: The step-id manifest

**Files:**
- Create: `internal/quests/manifest.go`, `internal/quests/manifest_test.go`,
  `_datafiles/world/dogmud/quests/step-ids.txt`, `step_manifest_test.go` (repo root)
- Modify: `internal/users/users.go:568`, `internal/users/loaduser_test.go`

- [ ] **Step 1: Write `internal/quests/manifest.go`**

```go
package quests

import (
	"fmt"
	"os"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/manifest"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// ParseStepManifest reads quests/step-ids.txt: one "<questId>-<stepId>" per
// shipped step. Saved characters hold their current step id, so a step id
// leaves content only through a retired or renamed line (#375 phase 2).
func ParseStepManifest(data []byte) (manifest.Manifest, error) {
	return manifest.Parse("step-ids.txt", data)
}

// StepTokens lists every step of every quest as "<questId>-<stepId>".
func StepTokens(qs []Quest) []string {
	var out []string
	for _, q := range qs {
		for _, s := range q.Steps {
			out = append(out, fmt.Sprintf("%d-%s", q.QuestId, s.Id))
		}
	}
	return out
}

// StepRenames reads the shipped manifest's renames for the load migration.
// A missing or unreadable manifest renames nothing.
func StepRenames() map[string]string {
	path := util.FilePath(string(configs.GetFilePathsConfig().DataFiles) + `/quests/step-ids.txt`)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	m, err := ParseStepManifest(data)
	if err != nil {
		mudlog.Warn("quests.StepRenames()", "path", path, "error", err)
		return nil
	}
	return m.Renamed
}
```

Check `go list -deps ./internal/manifest` stays tiny (it imports only the standard library),
so `quests` importing it adds no edge.

- [ ] **Step 2: `internal/quests/manifest_test.go`**

```go
package quests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStepTokens(t *testing.T) {
	qs := []Quest{{QuestId: 53, Steps: []QuestStep{{Id: "start"}, {Id: "end"}}}, {QuestId: 7, Steps: []QuestStep{{Id: "start"}}}}
	require.Equal(t, []string{"53-start", "53-end", "7-start"}, StepTokens(qs))
}

func TestParseStepManifest_NamesItsFile(t *testing.T) {
	_, err := ParseStepManifest([]byte("1-a\n1-a\n"))
	require.ErrorContains(t, err, "step-ids.txt")
}
```

- [ ] **Step 3: Generate the manifest** from shipped content into the scratchpad, then copy it
  into place. Write `<scratchpad>/gen_step_ids.py` with the Write tool:

```python
import glob, os, sys, yaml
root = sys.argv[1]
toks = []
for p in sorted(glob.glob(os.path.join(root, "quests", "*.yaml"))):
    q = yaml.safe_load(open(p, encoding="utf-8"))
    for s in q["steps"]:
        toks.append(f'{q["questid"]}-{s["id"]}')
head = """# Step-id manifest (#375 phase 2). One line per shipped quest step, as saved
# in a character's questprogress: "<quest id>-<step id>".
#
# Saved characters hold their current step id, so a step id may leave quest
# content only by moving to the "# retired" section (the step is gone) or the
# "# renamed" section as "old -> new" (LoadUser rewrites saved progress through
# it). A new step id must be added to the list. step_manifest_test.go enforces
# both.
"""
sys.stdout.write(head + "\n".join(sorted(toks)) + "\n")
```

Run (scratchpad path from the session's system prompt):

```bash
python -I "<scratchpad>/gen_step_ids.py" "_datafiles/world/dogmud" > "<scratchpad>/step-ids.txt"
wc -l "<scratchpad>/step-ids.txt"
cp "<scratchpad>/step-ids.txt" _datafiles/world/dogmud/quests/step-ids.txt
```

Expected: about 308 step lines (81 quests; the spec counts 227 non-start steps plus 81 first
steps) plus 8 header lines. Write down the exact count for the guard test below.

Confirm the quest loader ignores a `.txt` file in the quests folder:
`grep -n "yaml\|Glob\|Walk" internal/quests/quests.go | sed -n 1,20p` around `LoadDataFiles`
(line 361). It loads `*.yaml` only; if not, stop and report.

- [ ] **Step 4: Root guard** `step_manifest_test.go`

```go
package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"gopkg.in/yaml.v2"
)

func shippedStepTokens(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "quests", "*.yaml"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("found no quest files (err %v): the guard cannot run", err)
	}
	var qs []quests.Quest
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		var q quests.Quest
		if err := yaml.Unmarshal(data, &q); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		qs = append(qs, q)
	}
	toks := quests.StepTokens(qs)
	sort.Strings(toks)
	return toks
}

// TestStepIdManifest: saved characters hold their current step id, so a
// shipped step id may leave content only through a "# retired" or
// "# renamed" line, and a new step id must be added to the manifest.
func TestStepIdManifest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("_datafiles", "world", "dogmud", "quests", "step-ids.txt"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	m, err := quests.ParseStepManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	missing, unlisted := m.Diff(shippedStepTokens(t))
	for _, k := range missing {
		t.Errorf("step %s is gone from quest content: add it under \"# retired\" or \"# renamed\" (old -> new)", k)
	}
	for _, k := range unlisted {
		t.Errorf("step %s is not in step-ids.txt: append it", k)
	}
}

// TestStepIdManifest_GuardCanFail proves the guard sees a removed step.
func TestStepIdManifest_GuardCanFail(t *testing.T) {
	toks := shippedStepTokens(t)
	m, _ := quests.ParseStepManifest([]byte("0-removed_on_purpose\n"))
	m.Keys = append(m.Keys, toks...)
	missing, _ := m.Diff(toks)
	if len(missing) != 1 || missing[0] != "0-removed_on_purpose" {
		t.Fatalf("guard did not see the removed step: %v", missing)
	}
}
```

Run: `go test . -run StepIdManifest -v`
Expected: both PASS.

- [ ] **Step 5: Rename saved progress on load.** In `users.go`, after the
  `RenameHeardTopics` line (568):

```go
	// A renamed quest step keeps the player on it (#375 phase 2): renames
	// come from the step-id manifest, quests/step-ids.txt.
	loadedUser.Character.RenameQuestSteps(quests.StepRenames())
```

Add the `quests` import if `users.go` lacks it.

- [ ] **Step 6: Load test.** Read `TestLoadUserRenamesHeardTopicsFromTheManifest`
  (`internal/users/loaduser_test.go:83`) in full and add a twin,
  `TestLoadUserRenamesQuestStepsFromTheManifest`, built the same way: a temp DataFiles with
  `quests/step-ids.txt` containing `# renamed\n53-firstkill -> 53-scout\n`, a saved user with
  `questprogress: {53: firstkill}`, load it, and require `QuestProgress[53] == "scout"`.

Run: `go test ./internal/users/ -run TestLoadUserRenames -v`
Expected: both PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/quests/manifest.go internal/quests/manifest_test.go _datafiles/world/dogmud/quests/step-ids.txt step_manifest_test.go internal/users/users.go internal/users/loaduser_test.go
git commit -F - <<'EOF'
feat(quests): step-id manifest, its guard, and step renames on load (#375 phase 2)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 11: `internal/questloop`: world, findings, grant index, start/end/hint

**Files:**
- Create: `internal/questloop/questloop.go`, `internal/questloop/grants.go`,
  `internal/questloop/rules_loop.go`, `internal/questloop/questloop_test.go`

- [ ] **Step 1: Write the failing tests** `questloop_test.go` (a fixture world; every rule
  test in Tasks 11 to 13 builds on it)

```go
package questloop

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// goodWorld is one closed loop: Garve (mob 9112) offers quest 53, a kill
// trigger moves it to firstkill, and Garve takes the hand-in. It must
// produce no findings; each rule test breaks one thing.
func goodWorld() World {
	q := &quests.Quest{
		QuestId: 53, Name: "Cull the Wash",
		Steps: []quests.QuestStep{
			{Id: "start", Description: "d", Hint: "Type attack scout."},
			{Id: "firstkill", Description: "d", Hint: "Type talk garve."},
			{Id: "end", Description: "d"},
		},
		Triggers: []quests.TriggerDef{{
			Event: "mob_death", Mob: 9110,
			Conditions: quests.Conditions{Has: []string{"53-start"}, Missing: []string{"53-firstkill"}},
			Actions:    []quests.ActionDef{{Grant: "53-firstkill"}},
		}},
	}
	df := &dialogue.DialogueFile{MobId: 9112, Zone: "pothole_coulee", Topics: []dialogue.Topic{
		{Key: "cull_offer", Kind: dialogue.KindOffer, Quest: 53, Label: "Cull the Wash", Text: "t",
			Effects: dialogue.Effects{GrantsQuest: "53-start"}},
		{Key: "cull_in", Kind: dialogue.KindHandIn, Quest: 53, Label: "Cull the Wash", Text: "t",
			Gates:   dialogue.Gates{QuestRequired: []string{"53-firstkill"}},
			Effects: dialogue.Effects{GrantsQuest: "53-end"}},
	}}
	return World{Quests: []*quests.Quest{q}, Dialogue: []*dialogue.DialogueFile{df}}
}

func rules(fs []Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Rule+" "+f.Where)
	}
	return out
}

func TestValidate_GoodWorldIsClean(t *testing.T) {
	require.Empty(t, Validate(goodWorld()))
}

func TestStart_TriggerGrantIsRefused(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers = append(w.Quests[0].Triggers, quests.TriggerDef{
		Event: "room_enter", Room: 5, Actions: []quests.ActionDef{{Grant: "53-start"}}})
	require.Contains(t, rules(Validate(w)), "start quest 53 trigger 1")
}

func TestStart_RewardGrantIsRefused(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Rewards.QuestId = "53-start"
	require.Contains(t, rules(Validate(w)), "start quest 53 rewards questid")
}

func TestStart_NoOfferIsRefused(t *testing.T) {
	w := goodWorld()
	w.Dialogue[0].Topics = w.Dialogue[0].Topics[1:]
	require.Contains(t, rules(Validate(w)), "start step 0 (start)")
}

func TestEnd_TriggerFinishIsRefused(t *testing.T) {
	w := goodWorld()
	w.Dialogue[0].Topics = w.Dialogue[0].Topics[:1]
	w.Quests[0].Triggers = append(w.Quests[0].Triggers, quests.TriggerDef{
		Event: "mob_death", Mob: 9111,
		Conditions: quests.Conditions{Has: []string{"53-firstkill"}},
		Actions:    []quests.ActionDef{{Grant: "53-end"}}})
	got := rules(Validate(w))
	require.Contains(t, got, "end quest 53 trigger 1")
	require.Contains(t, got, "end step 2 (end)")
}

func TestEnd_OneStepQuestFailsByDesign(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Steps = []quests.QuestStep{{Id: "end"}}
	w.Quests[0].Triggers = nil
	w.Dialogue[0].Topics = w.Dialogue[0].Topics[:1]
	w.Dialogue[0].Topics[0].GrantsQuest = "53-end"
	got := rules(Validate(w))
	require.Contains(t, got, "end mob 9112 topic cull_offer", "an offer is not a hand-in")
}

func TestHint_MissingHintIsRefused(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Steps[1].Hint = "  "
	require.Contains(t, rules(Validate(w)), "hint step 1 (firstkill)")
}

func TestHint_FinalStepNeedsNone(t *testing.T) {
	w := goodWorld()
	require.Empty(t, w.Quests[0].Steps[2].Hint)
	require.Empty(t, Validate(w))
}

func TestFinding_KeyIsRuleQuestWhere(t *testing.T) {
	f := Finding{Rule: "hint", QuestId: 53, Where: "step 1 (firstkill)", Msg: "no hint"}
	require.Equal(t, "hint 53 step 1 (firstkill)", f.Key())
	require.False(t, strings.Contains(f.Key(), "no hint"), "the message is not part of the key")
}
```

- [ ] **Step 2: Run to see it fail**

Run: `go test ./internal/questloop/`
Expected: FAIL (package has no non-test files).

- [ ] **Step 3: Create `questloop.go`**

```go
// Package questloop holds the quest arc's loop validators (#375 phase 2):
// every quest starts at an NPC offer, ends at an NPC hand-in, hints each
// step, moves one action at a time, chains at its hand-in NPC, and declares
// repeats and reward overrides honestly. It is pure: it reads a proposed
// world and reports findings, so boot, CI and the quest editor run the same
// rules. internal/questworld builds the world from the live registries.
package questloop

import (
	"fmt"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// World is everything the loop rules read. Refs and Names may be zero in
// tests: a nil callback inside them skips that check.
type World struct {
	Quests   []*quests.Quest
	Dialogue []*dialogue.DialogueFile
	Refs     *quests.QuestValidators // nil skips the refs rule
	Names    quests.RewardNames
}

// Finding is one rule failure. Key (rule, quest, where) is its backlog
// line; Msg explains it and may be reworded without touching the backlog.
type Finding struct {
	Rule    string
	QuestId int
	Where   string
	Msg     string
}

// Key is the finding's backlog line: "rule questId where".
func (f Finding) Key() string { return fmt.Sprintf("%s %d %s", f.Rule, f.QuestId, f.Where) }

func (f Finding) String() string { return f.Key() + ": " + f.Msg }

// Validate runs every rule over w, sorted by key.
func Validate(w World) []Finding {
	g := indexGrants(w)
	var out []Finding
	for _, q := range w.Quests {
		if q == nil || len(q.Steps) == 0 {
			continue // Quest.Validate refuses these at load
		}
		out = append(out, ruleStart(q, g)...)
		out = append(out, ruleEnd(q, g)...)
		out = append(out, ruleHint(q)...)
		out = append(out, ruleOneAction(q, g)...)
		out = append(out, ruleRepeat(q)...)
		out = append(out, ruleReward(q, w.Names)...)
		out = append(out, ruleRefs(q, w.Refs)...)
	}
	out = append(out, ruleChain(w)...)
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}

func token(q *quests.Quest, i int) string { return fmt.Sprintf("%d-%s", q.QuestId, q.Steps[i].Id) }

func stepWhere(q *quests.Quest, i int) string { return fmt.Sprintf("step %d (%s)", i, q.Steps[i].Id) }
```

`ruleOneAction`, `ruleRepeat`, `ruleReward`, `ruleRefs` and `ruleChain` arrive in Tasks 12
and 13. For this task add stubs at the bottom of `rules_loop.go` so the package builds:

```go
func ruleOneAction(*quests.Quest, grants) []Finding  { return nil }
func ruleRepeat(*quests.Quest) []Finding              { return nil }
func ruleReward(*quests.Quest, quests.RewardNames) []Finding { return nil }
func ruleRefs(*quests.Quest, *quests.QuestValidators) []Finding { return nil }
func ruleChain(World) []Finding                       { return nil }
```

Each later task deletes its stub when it adds the real rule.

- [ ] **Step 4: Create `grants.go`**, the index every rule reads

```go
package questloop

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// way is one thing in the world that grants a quest token.
type way struct {
	where   string             // "quest 53 trigger 1", "quest 53 rewards questid", "mob 9112 topic cull_in"
	trigger *quests.TriggerDef // set for a trigger
	owner   int                // the quest whose file holds the trigger
	nested  bool               // granted inside a sequence's on_complete
	topic   *dialogue.Topic    // set for a topic
	mobId   int                // the topic's NPC
	reward  bool               // rewards.questid
}

// grants maps a token ("53-firstkill") to every way that grants it.
type grants map[string][]way

func indexGrants(w World) grants {
	g := grants{}
	for _, q := range w.Quests {
		if q == nil {
			continue
		}
		for i := range q.Triggers {
			t := &q.Triggers[i]
			where := fmt.Sprintf("quest %d trigger %d", q.QuestId, i)
			walkGrants(t.Actions, false, func(tok string, nested bool) {
				g[tok] = append(g[tok], way{where: where, trigger: t, owner: q.QuestId, nested: nested})
			})
		}
		if tok := q.Rewards.QuestId; tok != "" && !strings.HasPrefix(tok, "-") {
			g[tok] = append(g[tok], way{where: fmt.Sprintf("quest %d rewards questid", q.QuestId), owner: q.QuestId, reward: true})
		}
	}
	for _, df := range w.Dialogue {
		if df == nil {
			continue
		}
		for i := range df.Topics {
			t := &df.Topics[i]
			if t.GrantsQuest == "" || strings.HasPrefix(t.GrantsQuest, "-") {
				continue
			}
			g[t.GrantsQuest] = append(g[t.GrantsQuest], way{
				where: fmt.Sprintf("mob %d topic %s", df.MobId, t.Key), topic: t, mobId: df.MobId})
		}
	}
	return g
}

// walkGrants visits every grant action, sequences included at any depth.
// A "-" grant removes a token and is not a grant.
func walkGrants(actions []quests.ActionDef, nested bool, fn func(tok string, nested bool)) {
	for _, a := range actions {
		if a.Grant != "" && !strings.HasPrefix(a.Grant, "-") {
			fn(a.Grant, nested)
		}
		if a.Sequence != nil {
			walkGrants(a.Sequence.OnComplete, true, fn)
		}
	}
}

// ownGrants counts the grants a trigger makes of its own quest's tokens.
func ownGrants(t *quests.TriggerDef, questId int) int {
	n := 0
	prefix := fmt.Sprintf("%d-", questId)
	walkGrants(t.Actions, false, func(tok string, _ bool) {
		if strings.HasPrefix(tok, prefix) {
			n++
		}
	})
	return n
}
```

- [ ] **Step 5: Create `rules_loop.go`** with start, end and hint

```go
package questloop

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// ruleStart: the first step's token is granted by at least one offer topic
// and by nothing else.
func ruleStart(q *quests.Quest, g grants) []Finding {
	tok := token(q, 0)
	var out []Finding
	offers := 0
	for _, w := range g[tok] {
		if w.topic != nil && w.topic.Kind == dialogue.KindOffer {
			offers++
			continue
		}
		out = append(out, Finding{Rule: "start", QuestId: q.QuestId, Where: w.where,
			Msg: tok + " is granted here; only an NPC offer may start a quest"})
	}
	if offers == 0 {
		out = append(out, Finding{Rule: "start", QuestId: q.QuestId, Where: stepWhere(q, 0),
			Msg: "no offer topic grants " + tok})
	}
	return out
}

// ruleEnd: the final step's token is granted by at least one hand-in topic
// and by nothing else. A one-step quest's only token is also its start, so
// it cannot pass both start and end: intended (spec 5.2).
func ruleEnd(q *quests.Quest, g grants) []Finding {
	last := len(q.Steps) - 1
	tok := token(q, last)
	var out []Finding
	handins := 0
	for _, w := range g[tok] {
		if w.topic != nil && w.topic.Kind == dialogue.KindHandIn {
			handins++
			continue
		}
		out = append(out, Finding{Rule: "end", QuestId: q.QuestId, Where: w.where,
			Msg: tok + " is granted here; only an NPC hand-in may finish a quest"})
	}
	if handins == 0 {
		out = append(out, Finding{Rule: "end", QuestId: q.QuestId, Where: stepWhere(q, last),
			Msg: "no hand-in topic grants " + tok})
	}
	return out
}

// ruleHint: every step a player can stand on (all but the final one) has a hint.
func ruleHint(q *quests.Quest) []Finding {
	var out []Finding
	for i := 0; i < len(q.Steps)-1; i++ {
		if strings.TrimSpace(q.Steps[i].Hint) == "" {
			out = append(out, Finding{Rule: "hint", QuestId: q.QuestId, Where: stepWhere(q, i), Msg: "this step has no hint"})
		}
	}
	return out
}
```

Note `TestStart_TriggerGrantIsRefused` expects the where `quest 53 trigger 1`, which is the
trigger's own `where` string, and `TestStart_NoOfferIsRefused` expects `step 0 (start)`.
The one-step test expects the offer topic flagged under `end`: the offer grants `53-end`,
is not a hand-in, so `ruleEnd` reports it at `mob 9112 topic cull_offer`.

- [ ] **Step 6: Run**

Run: `go test ./internal/questloop/ -v`
Expected: PASS for every test in this task.

- [ ] **Step 7: Commit**

```bash
git add internal/questloop/questloop.go internal/questloop/grants.go internal/questloop/rules_loop.go internal/questloop/questloop_test.go
git commit -F - <<'EOF'
feat(questloop): proposed-world loop validator with start, end, hint rules (#375 phase 2)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 12: The `one-action` rule

Spec 5.2 (P4): for every non-start step, at least one way in; each way requires the previous
step's token; no way carries an item, flag or gold condition; no trigger grants more than one
of its own quest's steps; no grant sits inside a sequence. Several ways are alternatives and
pass.

**Files:**
- Create: `internal/questloop/rule_oneaction.go`, `internal/questloop/rule_oneaction_test.go`
- Modify: `internal/questloop/rules_loop.go` (delete the `ruleOneAction` stub)

- [ ] **Step 1: Write the failing tests**

```go
package questloop

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

func TestOneAction_AlternativesPass(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers = append(w.Quests[0].Triggers, quests.TriggerDef{
		Event: "skill_use", Skill: "brawling",
		Conditions: quests.Conditions{Has: []string{"53-start"}},
		Actions:    []quests.ActionDef{{Grant: "53-firstkill"}}})
	require.Empty(t, Validate(w))
}

func TestOneAction_NoWayIn(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers = nil
	require.Contains(t, rules(Validate(w)), "one-action step 1 (firstkill)")
}

func TestOneAction_MustRequirePreviousStep(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers[0].Conditions.Has = nil
	require.Contains(t, rules(Validate(w)), "one-action quest 53 trigger 0")
}

func TestOneAction_TopicMustRequirePreviousStep(t *testing.T) {
	w := goodWorld()
	w.Dialogue[0].Topics[1].QuestRequired = nil
	require.Contains(t, rules(Validate(w)), "one-action mob 9112 topic cull_in")
}

func TestOneAction_ExtraConditionsAreRefused(t *testing.T) {
	for name, c := range map[string]quests.Conditions{
		"has_item":     {HasItem: 7},
		"missing_item": {MissingItem: 7},
		"has_flag":     {HasFlag: map[string]string{"53-x": "y"}},
		"missing_flag": {MissingFlag: map[string]string{"53-x": "y"}},
		"has_gold":     {HasGold: 5},
	} {
		w := goodWorld()
		c.Has = []string{"53-start"}
		w.Quests[0].Triggers[0].Conditions = c
		require.Contains(t, rules(Validate(w)), "one-action quest 53 trigger 0", name)
	}
}

func TestOneAction_DoubleGrantIsRefused(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers[0].Actions = append(w.Quests[0].Triggers[0].Actions, quests.ActionDef{Grant: "53-end"})
	require.Contains(t, rules(Validate(w)), "one-action quest 53 trigger 0")
}

func TestOneAction_SequenceGrantIsRefused(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers[0].Actions = []quests.ActionDef{{Sequence: &quests.SequenceDef{
		OnComplete: []quests.ActionDef{{Grant: "53-firstkill"}}}}}
	require.Contains(t, rules(Validate(w)), "one-action quest 53 trigger 0")
}
```

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/questloop/ -run TestOneAction -v`
Expected: the five refusal tests FAIL (the stub returns nothing); `AlternativesPass` passes.

- [ ] **Step 3: Implement** `rule_oneaction.go` and delete the stub

```go
package questloop

import (
	"slices"

	"github.com/GoMudEngine/GoMud/internal/quests"
)

// ruleOneAction is spec 5.2's one-action rule (P4). One finding per broken
// way, so the backlog names each.
func ruleOneAction(q *quests.Quest, g grants) []Finding {
	var out []Finding
	add := func(where, msg string) {
		out = append(out, Finding{Rule: "one-action", QuestId: q.QuestId, Where: where, Msg: msg})
	}
	for i := 1; i < len(q.Steps); i++ {
		tok, prev := token(q, i), token(q, i-1)
		ways := g[tok]
		if len(ways) == 0 {
			add(stepWhere(q, i), "nothing moves a player onto "+tok)
			continue
		}
		for _, w := range ways {
			switch {
			case w.topic != nil:
				if !slices.Contains(w.topic.QuestRequired, prev) {
					add(w.where, "grants "+tok+" without requiring "+prev)
				}
			case w.trigger != nil:
				c := w.trigger.Conditions
				if !slices.Contains(c.Has, prev) {
					add(w.where, "grants "+tok+" without requiring "+prev)
				}
				if c.HasItem > 0 || c.MissingItem > 0 || len(c.HasFlag) > 0 || len(c.MissingFlag) > 0 || c.HasGold > 0 {
					add(w.where, "an item, flag or gold condition hides a second action; make it its own step")
				}
				if w.nested {
					add(w.where, "grants "+tok+" inside a sequence; a step moves on one action")
				}
				if ownGrants(w.trigger, w.owner) > 1 {
					add(w.where, "grants more than one step of its quest at once")
				}
			}
		}
	}
	return dedupe(out)
}

// dedupe drops repeated keys (one trigger can break the rule two ways; the
// backlog needs it once), keeping the first message.
func dedupe(fs []Finding) []Finding {
	seen := map[string]bool{}
	var out []Finding
	for _, f := range fs {
		if seen[f.Key()] {
			continue
		}
		seen[f.Key()] = true
		out = append(out, f)
	}
	return out
}
```

A reward `questid` way (a quest whose rewards grant one of its own later steps) has neither
topic nor trigger; it is already a `start` or `end` finding when it names those steps, and
for a middle step add one more case before the `switch`:

```go
			if w.reward {
				add(w.where, "a completion reward grants "+tok)
				continue
			}
```

- [ ] **Step 4: Run**

Run: `go test ./internal/questloop/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/questloop/rule_oneaction.go internal/questloop/rule_oneaction_test.go internal/questloop/rules_loop.go
git commit -F - <<'EOF'
feat(questloop): one-action rule, alternatives allowed (#375 P4)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 13: `chain`, `repeat`, `reward`, `refs` rules

**Files:**
- Create: `internal/questloop/rules_more.go`, `internal/questloop/rules_more_test.go`
- Modify: `internal/questloop/rules_loop.go` (delete the four remaining stubs)

- [ ] **Step 1: Write the failing tests**

```go
package questloop

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// chainWorld adds quest 54, offered after 53 by a second NPC.
func chainWorld(offerMob int) World {
	w := goodWorld()
	q54 := &quests.Quest{QuestId: 54, Name: "Next", Steps: []quests.QuestStep{
		{Id: "start", Description: "d", Hint: "h"}, {Id: "end", Description: "d"}}}
	w.Quests = append(w.Quests, q54)
	offer := dialogue.Topic{Key: "next_offer", Kind: dialogue.KindOffer, Quest: 54, Label: "Next", Text: "t",
		Gates: dialogue.Gates{QuestRequired: []string{"53-end"}}, Effects: dialogue.Effects{GrantsQuest: "54-start"}}
	handin := dialogue.Topic{Key: "next_in", Kind: dialogue.KindHandIn, Quest: 54, Label: "Next", Text: "t",
		Gates: dialogue.Gates{QuestRequired: []string{"54-start"}}, Effects: dialogue.Effects{GrantsQuest: "54-end"}}
	if offerMob == 9112 {
		w.Dialogue[0].Topics = append(w.Dialogue[0].Topics, offer, handin)
	} else {
		w.Dialogue = append(w.Dialogue, &dialogue.DialogueFile{MobId: offerMob, Topics: []dialogue.Topic{offer, handin}})
	}
	return w
}

func TestChain_NextOfferOnTheHandInNPCPasses(t *testing.T) {
	require.Empty(t, Validate(chainWorld(9112)))
}

func TestChain_NextOfferElsewhereIsRefused(t *testing.T) {
	require.Contains(t, rules(Validate(chainWorld(9200))), "chain mob 9200 topic next_offer")
}

func TestRepeat_Rules(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Repeatable = true
	require.Contains(t, rules(Validate(w)), "repeat quest 53", "a repeatable needs a cooldown")
	w.Quests[0].CooldownRounds = 300
	require.Empty(t, Validate(w))
	w.Quests[0].RepeatLimit = -1
	require.Contains(t, rules(Validate(w)), "repeat quest 53")
	w = goodWorld()
	w.Quests[0].RepeatLimit = 3
	require.Contains(t, rules(Validate(w)), "repeat quest 53", "a limit on a quest that does not repeat")
}

func TestReward_OverrideMustNameWhatIsPaid(t *testing.T) {
	w := goodWorld()
	w.Names = quests.RewardNames{Item: func(int) string { return "Oak Bow" }}
	w.Quests[0].Rewards = quests.QuestReward{ItemId: 9002, Summary: "Garve will make it worth your while."}
	require.Contains(t, rules(Validate(w)), "reward quest 53 rewards summary")
	w.Quests[0].Rewards.Summary = "An oak bow, and Garve's thanks."
	require.Empty(t, Validate(w), "names are matched without case")
	w.Quests[0].Rewards.Summary = "Garve will make it worth your while."
	w.Quests[0].Rewards.Surprise = true
	require.Empty(t, Validate(w))
}

func TestReward_SurpriseNeedsSummary(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Rewards.Surprise = true
	require.Contains(t, rules(Validate(w)), "reward quest 53 rewards summary")
}

func TestRefs_RefusalsBecomeFindings(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers[0].Mob = 424242
	v := quests.QuestValidators{MobExists: func(id int) bool { return id != 424242 }}
	fillValidators(&v)
	w.Refs = &v
	require.Contains(t, rules(Validate(w)), "refs quest 53 refs")
}
```

Add to the test file a helper that fills every unset `QuestValidators` func with an
always-true one (`StepExists`, `MobExists`, `ItemExists`, `RoomExists`, `ConditionExists`,
`SpellExists`, `SkillExists`, `StatExists`, `RecipeExists`, `FactionExists`, `FlagDeclared`,
`DialogueGrants`, `MobHasDialogue`), so `ValidateQuestRefs` can run:

```go
func fillValidators(v *quests.QuestValidators) {
	yesI := func(int) bool { return true }
	yesS := func(string) bool { return true }
	if v.StepExists == nil { v.StepExists = yesS }
	if v.MobExists == nil { v.MobExists = yesI }
	if v.ItemExists == nil { v.ItemExists = yesI }
	if v.RoomExists == nil { v.RoomExists = yesI }
	if v.ConditionExists == nil { v.ConditionExists = yesI }
	if v.SpellExists == nil { v.SpellExists = yesS }
	if v.SkillExists == nil { v.SkillExists = yesS }
	if v.StatExists == nil { v.StatExists = yesS }
	if v.RecipeExists == nil { v.RecipeExists = yesS }
	if v.FactionExists == nil { v.FactionExists = yesS }
	if v.FlagDeclared == nil { v.FlagDeclared = func(string, string) bool { return true } }
	if v.DialogueGrants == nil { v.DialogueGrants = yesS }
	if v.MobHasDialogue == nil { v.MobHasDialogue = yesI }
}
```

(gofmt will expand the one-line ifs.)

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/questloop/ -run 'TestChain|TestRepeat|TestReward|TestRefs' -v`
Expected: the refusal assertions FAIL against the stubs.

- [ ] **Step 3: Implement** `rules_more.go` and delete the four stubs

```go
package questloop

import (
	"fmt"
	"slices"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// ruleChain: an offer gated on quest N's final token sits on an NPC that
// takes N's hand-in (P1), so the hand-in can pitch it.
func ruleChain(w World) []Finding {
	finals := map[string]int{} // final token -> quest id
	for _, q := range w.Quests {
		if q != nil && len(q.Steps) > 0 {
			finals[token(q, len(q.Steps)-1)] = q.QuestId
		}
	}
	handinMobs := map[int][]int{} // quest id -> mobs with a hand-in for it
	for _, df := range w.Dialogue {
		if df == nil {
			continue
		}
		for _, t := range df.Topics {
			if t.Kind == dialogue.KindHandIn && t.Quest > 0 && !slices.Contains(handinMobs[t.Quest], df.MobId) {
				handinMobs[t.Quest] = append(handinMobs[t.Quest], df.MobId)
			}
		}
	}
	var out []Finding
	for _, df := range w.Dialogue {
		if df == nil {
			continue
		}
		for _, t := range df.Topics {
			if t.Kind != dialogue.KindOffer {
				continue
			}
			for _, req := range t.QuestRequired {
				prior, ok := finals[req]
				if !ok || prior == t.Quest || slices.Contains(handinMobs[prior], df.MobId) {
					continue
				}
				out = append(out, Finding{Rule: "chain", QuestId: prior,
					Where: fmt.Sprintf("mob %d topic %s", df.MobId, t.Key),
					Msg: fmt.Sprintf("offers quest %d after quest %d, but this NPC does not take quest %d's hand-in; add a bridge quest to here", t.Quest, prior, prior)})
			}
		}
	}
	return out
}

// ruleRepeat: a repeatable has a cooldown; repeat_limit is not negative and
// only appears on a repeatable (P3).
func ruleRepeat(q *quests.Quest) []Finding {
	var out []Finding
	add := func(msg string) {
		out = append(out, Finding{Rule: "repeat", QuestId: q.QuestId, Where: fmt.Sprintf("quest %d", q.QuestId), Msg: msg})
	}
	if q.Repeatable && q.CooldownRounds <= 0 {
		add("a repeatable quest needs cooldown_rounds above zero")
	}
	if q.RepeatLimit < 0 {
		add("repeat_limit cannot be negative")
	}
	if q.RepeatLimit > 0 && !q.Repeatable {
		add("repeat_limit is set on a quest that does not repeat")
	}
	return dedupe(out)
}

// ruleReward: an authored summary names every item, recipe and spell the
// quest pays, unless the reward is a surprise; a surprise needs a summary (P2).
func ruleReward(q *quests.Quest, n quests.RewardNames) []Finding {
	r := q.Rewards
	where := "rewards summary"
	summary := strings.TrimSpace(r.Summary)
	if r.Surprise && summary == "" {
		return []Finding{{Rule: "reward", QuestId: q.QuestId, Where: where, Msg: "surprise is set but there is no summary to show"}}
	}
	if summary == "" || r.Surprise {
		return nil
	}
	low := strings.ToLower(summary)
	var missing []string
	for _, name := range quests.RewardPaidNames(q, n) {
		if !strings.Contains(low, strings.ToLower(name)) {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return []Finding{{Rule: "reward", QuestId: q.QuestId, Where: where,
		Msg: "the summary does not name " + strings.Join(missing, ", ") + "; name them or set surprise"}}
}

// ruleRefs: ValidateQuestRefs' refusals, so its checks run at every door.
// Its warnings stay with the editor.
func ruleRefs(q *quests.Quest, v *quests.QuestValidators) []Finding {
	if v == nil {
		return nil
	}
	errs, _ := quests.ValidateQuestRefs(*q, *v)
	var out []Finding
	for _, e := range errs {
		out = append(out, Finding{Rule: "refs", QuestId: q.QuestId, Where: "refs", Msg: e})
	}
	return out
}
```

Two refs errors on one quest share the key `refs 53 refs`; that is intended (the backlog holds
"quest 53 has ref problems", the messages list them). Keep `Validate`'s output unique by key
only for `one-action` and `repeat` (their `dedupe`); refs findings may repeat a key.

- [ ] **Step 4: Run**

Run: `go test ./internal/questloop/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/questloop/rules_more.go internal/questloop/rules_more_test.go internal/questloop/rules_loop.go
git commit -F - <<'EOF'
feat(questloop): chain, repeat, reward and refs rules (#375 P1 to P3)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 14: Backlog ratchet, the registry world, and the boot door

**Files:**
- Create: `internal/questloop/backlog.go`, `internal/questloop/backlog_test.go`,
  `internal/questworld/questworld.go`, `main_quests.go`, `quest_loop_smoke_test.go` (root),
  `_datafiles/world/dogmud/quests/loop-backlog.txt`
- Modify: `modules/gmcp/gmcp.Quest.go` (`realQuestValidators` moves out), `modules/gmcp/gmcp.Quest_refs.go`
  (`collectDialogueGrantTokens`, `mobHasDialogueFile` move out), `main.go:1904`

- [ ] **Step 1: Write the failing backlog tests** `backlog_test.go`

```go
package questloop

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBacklog(t *testing.T) {
	b, err := ParseBacklog([]byte("# header\n\nhint 53 step 1 (firstkill)\nend 7 step 3 (end)\n"))
	require.NoError(t, err)
	require.True(t, b["hint 53 step 1 (firstkill)"])
	require.Len(t, b, 2)
	_, err = ParseBacklog([]byte("hint 53 step 1\nhint 53 step 1\n"))
	require.Error(t, err, "a repeated line is a mistake")
}

func TestRatchet(t *testing.T) {
	fs := []Finding{
		{Rule: "hint", QuestId: 53, Where: "step 1 (firstkill)"},
		{Rule: "end", QuestId: 9, Where: "step 2 (end)"},
	}
	b := Backlog{"hint 53 step 1 (firstkill)": true, "start 4 step 0 (start)": true}
	fresh, known, stale := Ratchet(fs, b)
	require.Equal(t, []string{"end 9 step 2 (end)"}, keys(fresh))
	require.Equal(t, []string{"hint 53 step 1 (firstkill)"}, keys(known))
	require.Equal(t, []string{"start 4 step 0 (start)"}, stale)
}

func TestFormatBacklog_RoundTrips(t *testing.T) {
	fs := []Finding{{Rule: "hint", QuestId: 53, Where: "step 1 (firstkill)"}, {Rule: "hint", QuestId: 53, Where: "step 1 (firstkill)"}}
	b, err := ParseBacklog(FormatBacklog(fs))
	require.NoError(t, err)
	require.Equal(t, Backlog{"hint 53 step 1 (firstkill)": true}, b)
}

func keys(fs []Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Key())
	}
	return out
}
```

- [ ] **Step 2: Implement `backlog.go`**

```go
package questloop

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// Backlog is the committed list of known findings, quests/loop-backlog.txt
// (spec 5.3). A listed finding is a warning; an unlisted one refuses; a
// listed line with no finding is stale and fails CI, so the list only
// shrinks. Phase 7 deletes lines as each zone pass clears them.
type Backlog map[string]bool

const backlogHeader = `# Quest loop backlog (#375 phase 2, spec 5.3). One finding key per line,
# "rule questId where", for a loop-rule failure in shipped content that the
# phase 7 zone passes have not fixed yet. A finding listed here is a boot
# warning; any finding not listed refuses boot, CI and the quest editor's
# save. A line with no matching finding fails CI: delete it when its finding
# is fixed. Never add a line by hand to make new content pass.
`

// ParseBacklog reads the backlog text; comment and blank lines are skipped.
func ParseBacklog(data []byte) (Backlog, error) {
	b := Backlog{}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	for n, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if b[line] {
			return b, fmt.Errorf("loop-backlog.txt line %d: repeated line %q", n+1, line)
		}
		b[line] = true
	}
	return b, nil
}

// FormatBacklog writes the findings' keys as backlog text, sorted, once each.
func FormatBacklog(fs []Finding) []byte {
	seen := map[string]bool{}
	var ks []string
	for _, f := range fs {
		if k := f.Key(); !seen[k] {
			seen[k] = true
			ks = append(ks, k)
		}
	}
	sort.Strings(ks)
	return []byte(backlogHeader + strings.Join(ks, "\n") + "\n")
}

// Ratchet splits findings into fresh (not in the backlog: they refuse) and
// known (listed: warnings), and returns the backlog lines no finding matched.
func Ratchet(fs []Finding, b Backlog) (fresh, known []Finding, stale []string) {
	matched := map[string]bool{}
	for _, f := range fs {
		k := f.Key()
		if b[k] {
			known = append(known, f)
			matched[k] = true
		} else {
			fresh = append(fresh, f)
		}
	}
	for k := range b {
		if !matched[k] {
			stale = append(stale, k)
		}
	}
	sort.Strings(stale)
	return fresh, known, stale
}
```

Run: `go test ./internal/questloop/ -run 'Backlog|Ratchet' -v`
Expected: PASS.

- [ ] **Step 3: Move the registry validators into `questworld`.** Create
  `internal/questworld/questworld.go` holding, moved verbatim from gmcp and exported:
  `realQuestValidators()` as `Validators() quests.QuestValidators` (from
  `gmcp.Quest.go:58-82`), `collectDialogueGrantTokens()` as an unexported helper (from
  `gmcp.Quest_refs.go:182`), and `mobHasDialogueFile` (from `gmcp.Quest_refs.go:197`).
  `collectDialogueGrantTokens` uses gmcp's `walkDialogueGates`; in `questworld`, replace that
  walk with a direct loop, since only topics carry grants (`walkDialogueGates` visits topics
  for `grantsQuest`, and opener variants with no grant):

```go
func dialogueGrantTokens() map[string]bool {
	granted := map[string]bool{}
	dialogue.WalkAllFiles(func(mobId int, zone string, df *dialogue.DialogueFile) {
		for _, t := range df.Topics {
			if t.GrantsQuest != "" {
				granted[t.GrantsQuest] = true
			}
		}
	})
	return granted
}
```

Then add the world builders:

```go
// Registry is the loop world over the loaded registries: every quest, every
// dialogue file (strictly decoded; their own problems are the dialogue
// door's), the live reference checks and reward names.
func Registry() questloop.World {
	all := quests.GetAllQuests()
	qs := make([]*quests.Quest, 0, len(all))
	for i := range all {
		qs = append(qs, &all[i])
	}
	files, _ := dialogue.LoadAllFiles()
	v := Validators()
	return questloop.World{Quests: qs, Dialogue: files, Refs: &v, Names: Names()}
}

// WithQuest is w with q in place of the quest that has its id (or added),
// the proposed world an editor save is checked against.
func WithQuest(w questloop.World, q quests.Quest) questloop.World {
	out := w
	out.Quests = nil
	replaced := false
	for _, existing := range w.Quests {
		if existing != nil && existing.QuestId == q.QuestId {
			out.Quests = append(out.Quests, &q)
			replaced = true
			continue
		}
		out.Quests = append(out.Quests, existing)
	}
	if !replaced {
		out.Quests = append(out.Quests, &q)
	}
	return out
}

// BacklogPath is the shipped backlog's path.
func BacklogPath() string {
	return util.FilePath(string(configs.GetFilePathsConfig().DataFiles) + `/quests/loop-backlog.txt`)
}

// LoadBacklog reads the shipped backlog; a missing file is an empty backlog.
func LoadBacklog() (questloop.Backlog, error) {
	data, err := os.ReadFile(BacklogPath())
	if errors.Is(err, os.ErrNotExist) {
		return questloop.Backlog{}, nil
	}
	if err != nil {
		return nil, err
	}
	return questloop.ParseBacklog(data)
}
```

In gmcp, replace `validators: realQuestValidators()` with `validators: questworld.Validators()`
and delete the moved functions; keep `walkDialogueGates` (the reference scan still uses it).
Run `go build ./...` and remove imports gmcp no longer needs. Run
`go test ./modules/gmcp/` and expect `ok`.

- [ ] **Step 4: Boot door** `main_quests.go`

```go
package main

import (
	"errors"
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/questworld"
)

// validateShippedQuests is the boot door of the quest loop validators
// (#375 phase 2): every rule over the loaded quests and dialogue, split by
// the shipped backlog. Load quests, dialogue, mobs, items, spells, recipes
// and factions first.
func validateShippedQuests() (fresh, known []questloop.Finding, stale []string, err error) {
	b, err := questworld.LoadBacklog()
	if err != nil {
		return nil, nil, nil, err
	}
	fresh, known, stale = questloop.Ratchet(questloop.Validate(questworld.Registry()), b)
	return fresh, known, stale, nil
}

// questDoor logs the backlog's known findings as warnings and refuses on a
// fresh one: a panic at boot, an error on a reload (the dialogueDoor rule).
// Stale backlog lines are CI's job (quest_loop_smoke_test.go), not boot's.
func questDoor(isReload bool) error {
	fresh, known, _, err := validateShippedQuests()
	if err != nil {
		mudlog.Error("quests", "backlog", err.Error())
		if !isReload {
			panic(err.Error())
		}
		return err
	}
	if len(known) > 0 {
		mudlog.Warn("quests", "loop backlog findings", len(known), "file", questworld.BacklogPath())
	}
	if len(fresh) == 0 {
		return nil
	}
	for _, f := range fresh {
		mudlog.Error("quests", "loop", f.String())
	}
	msg := fmt.Sprintf("%d quest loop problems not in the backlog; see the log above", len(fresh))
	if !isReload {
		panic(msg)
	}
	return errors.New(msg)
}
```

In `main.go` at line 1904, keep the dialogue door's error and add the quest door after it:

```go
	reloadErr = dialogueDoor(validateShippedDialogue(), isReload)
	if err := questDoor(isReload); err != nil && reloadErr == nil {
		reloadErr = err
	}
```

- [ ] **Step 5: CI sweep and backlog writer** `quest_loop_smoke_test.go`

```go
package main

import (
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/questworld"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// loopBacklogWriteEnv, when set to a path, makes the sweep write the
// current findings there as backlog text instead of failing. It is how the
// backlog is generated (into a scratch path, then copied into place by
// hand); it never writes the shipped file.
const loopBacklogWriteEnv = `DOGMUD_LOOP_BACKLOG_WRITE`

// TestSmoke_ShippedQuestsPassTheLoop runs the boot door's quest loop rules
// over shipped content: a finding not in loop-backlog.txt fails, and so does
// a backlog line no finding matches (the list only shrinks).
func TestSmoke_ShippedQuestsPassTheLoop(t *testing.T) {
	if os.Getenv(bootSmokeEnvVar) == `` {
		t.Skipf("set %s=1 to run the quest loop sweep", bootSmokeEnvVar)
	}
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig failed: %v", err)
	}
	func() {
		defer func() { _ = recover() }() // boot failures are the boot test's job
		loadAllDataFiles(false)
	}()
	if len(quests.GetAllQuests()) < 70 {
		t.Fatalf("only %d quests loaded; the sweep would pass vacuously", len(quests.GetAllQuests()))
	}

	if path := os.Getenv(loopBacklogWriteEnv); path != `` {
		fs := questloop.Validate(questworld.Registry())
		if err := os.WriteFile(path, questloop.FormatBacklog(fs), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %d findings to %s", len(fs), path)
		return
	}

	fresh, known, stale, err := validateShippedQuests()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fresh {
		t.Errorf("not in loop-backlog.txt: %s", f.String())
	}
	for _, k := range stale {
		t.Errorf("loop-backlog.txt line %q matches no finding: delete it", k)
	}
	t.Logf("quest loop: %d known backlog findings, %d new, %d stale", len(known), len(fresh), len(stale))
}

// TestQuestLoop_GuardCanFail proves the ratchet refuses an unlisted finding
// and flags a stale line, without the shipped world.
func TestQuestLoop_GuardCanFail(t *testing.T) {
	fs := []questloop.Finding{{Rule: "hint", QuestId: 1, Where: "step 0 (start)"}}
	fresh, _, stale := questloop.Ratchet(fs, questloop.Backlog{"end 2 step 1 (end)": true})
	if len(fresh) != 1 || len(stale) != 1 {
		t.Fatalf("ratchet did not refuse: fresh %v stale %v", fresh, stale)
	}
}
```

- [ ] **Step 6: Generate the backlog** into the scratchpad and copy it in:

```bash
DOGMUD_BOOT_SMOKE=1 DOGMUD_LOOP_BACKLOG_WRITE="<scratchpad>/loop-backlog.txt" go test . -run TestSmoke_ShippedQuestsPassTheLoop -v 2>&1 | tail -5
grep -vc "^#" "<scratchpad>/loop-backlog.txt"
cut -d' ' -f1 "<scratchpad>/loop-backlog.txt" | grep -v "^#" | sort | uniq -c
cp "<scratchpad>/loop-backlog.txt" _datafiles/world/dogmud/quests/loop-backlog.txt
```

Expected, cross-checked against the spec's census (record the real numbers in the commit
message and the PR body; a big gap from these means a rule is wrong, so stop and report):
`end` about 30 (27 trigger finishes + 3 with no finish, plus extra lines for trigger finishes
that also have a hand-in), `start` covering 28, 29, 77, the 1000000 test quest and the trigger
starts, `hint` for the 12 hintless quests' steps, `one-action` in the range 40 to 70, `chain`
a handful, `repeat` 0 (all seven repeatables have cooldowns), `reward` 0, `refs` 0 if the
editor validator is clean on shipped content.

- [ ] **Step 7: Run the sweep for real**

Run: `DOGMUD_BOOT_SMOKE=1 go test . -run 'TestSmoke_ShippedQuestsPassTheLoop|TestQuestLoop_GuardCanFail|TestSmoke_ShippedDialoguePassesTheValidators' -v 2>&1 | tail -8`
Expected: PASS, `0 new, 0 stale`.

- [ ] **Step 8: Boot check** (`dogmud-shipping`, detached-worktree boot). Build and boot the
  server on a private port from this worktree with the gitignored config override, wait for
  the "listening" log line, and confirm the log shows the `loop backlog findings` warning and
  no panic. Stop the server by the PID you started. The `dogmud-shipping` skill has the exact
  steps; follow it.

- [ ] **Step 9: Commit**

```bash
git add internal/questloop/backlog.go internal/questloop/backlog_test.go internal/questworld/questworld.go main_quests.go main.go quest_loop_smoke_test.go _datafiles/world/dogmud/quests/loop-backlog.txt modules/gmcp/gmcp.Quest.go modules/gmcp/gmcp.Quest_refs.go
git commit -F - <<'EOF'
feat(quests): loop validators at boot and CI behind a shrinking backlog (#375 phase 2)

Backlog generated from shipped content: <N> findings (<per-rule counts>).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

Fill in the counts from Step 6 before committing.

---

### Task 15: The editor door

**Files:**
- Modify: `modules/gmcp/gmcp.Quest.go` (`questDeps`, `realQuestDeps`, `buildQuestUpdate`)
- Test: the gmcp quest editor test file (`grep -ln "buildQuestUpdate" modules/gmcp/*_test.go`)

- [ ] **Step 1: Read the existing `buildQuestUpdate` tests** in full (the file the grep names)
  to see how `questDeps` fakes are built.

- [ ] **Step 2: Write the failing tests** in that file:
  - `TestBuildQuestUpdate_RefusesAFreshLoopFinding`: deps whose `loop` returns one finding
    not in the backlog; the result is `Ok: false` and the error names the finding's key.
  - `TestBuildQuestUpdate_AllowsABacklogFinding`: the same finding listed in the fake
    backlog; `Ok: true`, and the finding is in `Warnings`.
  - `TestBuildQuestUpdate_ChecksTheProposedQuest`: the fake `loop` records the quest it was
    given; require it is the edited quest (a changed `Name`), not the cached one.

- [ ] **Step 3: Implement.** Add to `questDeps`:

```go
	// loop runs the quest loop rules over the world with q in place, and
	// backlog returns the shipped loop backlog (#375 phase 2 editor door).
	loop    func(q quests.Quest) []questloop.Finding
	backlog func() (questloop.Backlog, error)
```

In `realQuestDeps`:

```go
		loop: func(q quests.Quest) []questloop.Finding {
			return questloop.Validate(questworld.WithQuest(questworld.Registry(), q))
		},
		backlog: questworld.LoadBacklog,
```

In `buildQuestUpdate`, after the `ValidateQuestRefs` refusal and before `d.save(q)`:

```go
	if d.loop != nil && d.backlog != nil {
		b, err := d.backlog()
		if err != nil {
			return buildErr("quest %d refused: cannot read the loop backlog: %s", q.QuestId, err.Error())
		}
		fresh, known, _ := questloop.Ratchet(d.loop(q), b)
		if len(fresh) > 0 {
			lines := make([]string, 0, len(fresh))
			for _, f := range fresh {
				lines = append(lines, f.String())
			}
			return buildErr("quest %d refused:\n%s", q.QuestId, strings.Join(lines, "\n"))
		}
		for _, f := range known {
			warns = append(warns, "known (loop backlog): "+f.String())
		}
	}
```

Existing test fakes leave `loop` nil and keep passing.

- [ ] **Step 4: Run**

Run: `go test ./modules/gmcp/`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add modules/gmcp/gmcp.Quest.go modules/gmcp/<the test file>
git commit -F - <<'EOF'
feat(gmcp): quest editor save runs the loop rules on the proposed world (#375 phase 2)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 16: `(repeatable)` and finished repeatables in the text quest list

**Files:**
- Modify: `internal/usercommands/quests.go`, `_datafiles/world/dogmud/templates/character/quests.template`
- Test: `internal/usercommands/quests_repeat_test.go` (new)

- [ ] **Step 1: Read the existing quest command tests' setup.**
  `grep -ln "func Test.*Quests\|Quests(\"" internal/usercommands/*_test.go` and read the one
  that calls `Quests(` (chunks of at most 400 lines). Copy its user, quest registration and
  output capture.

- [ ] **Step 2: Extract the row builder** so it can be tested without templates. In
  `quests.go`, move `QuestRecord` to package level as `questRecord` and add:

```go
// questRecord is one row of the quests list.
type questRecord struct {
	Id          int
	Name        string
	Repeatable  bool
	Description string
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
```

In `Quests`, use `questRecord` for every row, set `Repeatable: questInfo.Repeatable` on the
in-progress rows, and when `showComplete`, append `finishedRepeatRows(user.Character,
allQuestProgress, showHidden)` to `allQuests` (and add their count to `qInfo.QuestsTotal`)
before the sort. Add the `characters` import.

- [ ] **Step 3: Template.** In `quests.template`, change the name cell to print the tag:

```
  <ansi fg="questname">{{ padRight 41 (print $qInfo.Name (and $qInfo.Repeatable " (repeatable)")) }}</ansi>
```

If `and` with a string does not render as intended in this template engine (check the result
in Step 5), use `{{ if $qInfo.Repeatable }}` around a second `padRight` variant instead. The
name cell must stay 41 wide so the bars line up.

- [ ] **Step 4: Write the tests** in `quests_repeat_test.go`, with the setup from Step 1:
  - `TestFinishedRepeatRows_Status`: a character with `QuestCompletions{53: 1}` and no
    progress on 53. With the cooldown active, the row's description is `Not offered again yet.`;
    after advancing the round past the cooldown, `Ready to take on again.`; with
    `RepeatLimit: 1`, `Done for good.`
  - `TestFinishedRepeatRows_SkipsInProgressAndSecret`: a repeatable in progress gives no
    finished row; a secret repeatable gives none unless `showHidden`.
  - `TestQuestsCommand_TagsRepeatable`: the rendered `quests` output for a player on step
    `start` of a repeatable contains `(repeatable)`; plain `quests` does not list a finished
    repeatable, and `quests all` does, with its status line.

- [ ] **Step 5: Run**

Run: `go test ./internal/usercommands/ -run 'Quests|FinishedRepeat' -v`
Expected: PASS. Print one rendered `quests all` from the test (`t.Log`) and check by eye
that the bars still line up at 80 columns.

- [ ] **Step 6: Commit**

```bash
git add internal/usercommands/quests.go internal/usercommands/quests_repeat_test.go _datafiles/world/dogmud/templates/character/quests.template
git commit -F - <<'EOF'
feat(quests): tag repeatables and list finished ones in quests all (#375 P6)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 17: Repeatables in `Char.Quests` and the web panel

Spec 4.3: active repeatables carry the `(repeatable)` tag; finished repeatables that are not
at their cap are sent with `status` `ready` or `waiting` and drawn below the active quests
under "Repeatable work", dimmed, unfocusable, with no map marker. Done-for-good ones are not
sent. The panel already re-reads `Char.Quests` on every room change
(`roomChangeHandler`, `gmcp.Char.go:145`), which is how "waiting" turns to "ready".

**Files:**
- Modify: `modules/gmcp/gmcp.Char.go` (builder at :632-700, struct at :1143)
- Create: `modules/gmcp/gmcp.Char_repeat_test.go`
- Modify: `_datafiles/html/public/webclient-pure.html` (`renderQuests`, :938-997)
- Modify: `_datafiles/html/public/static/css/dashboard.css` (beside `.qname`)
- Modify: `tools/webclient-tests/quest-panel-refresh.js`

- [ ] **Step 1: Payload fields.** Add to `GMCPCharModule_Payload_Quest` after `Name`:

```go
	Repeatable bool `json:"repeatable,omitempty"` // the panel tags it "(repeatable)" (#375 P6)
	// Status is set only on a finished repeatable: "ready" (can be taken
	// again) or "waiting" (on cooldown). Such a row has no description,
	// hint or target and cannot be focused (#375 P6).
	Status string `json:"status,omitempty"`
```

- [ ] **Step 2: The finished rows, as a pure helper** so they test without a GMCP session.
  Add to `gmcp.Char.go`:

```go
// Status values for a finished repeatable in Char.Quests (#375 P6).
const (
	questStatusReady   = "ready"
	questStatusWaiting = "waiting"
)

// finishedRepeatQuests are the Char.Quests rows for repeatables c has
// finished and holds no progress on, not secret and not at their cap,
// sorted by id. Done-for-good ones are left to "quests all".
func finishedRepeatQuests(c *characters.Character, lookup func(int) *quests.Quest) []GMCPCharModule_Payload_Quest {
	progress := c.GetQuestProgress()
	var ids []int
	for id, n := range c.QuestCompletions {
		if _, inProgress := progress[id]; n > 0 && !inProgress {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	var rows []GMCPCharModule_Payload_Quest
	for _, id := range ids {
		q := lookup(id)
		if q == nil || !q.Repeatable || q.Secret || c.RepeatCapReached(q) {
			continue
		}
		status := questStatusReady
		if c.QuestCooldownActive(id) {
			status = questStatusWaiting
		}
		rows = append(rows, GMCPCharModule_Payload_Quest{Id: id, Name: q.Name, Repeatable: true, Status: status})
	}
	return rows
}
```

In the builder, set `Repeatable: qDef.Repeatable,` in the in-progress `questPayload` literal
(:660), and after the progress loop, before `if !all {` (:697):

```go
		// Finished repeatables, below the active list (#375 P6).
		payload.Quests = append(payload.Quests, finishedRepeatQuests(user.Character, quests.GetQuestById)...)
```

Add the `sort`, `characters` and `quests` imports if the file lacks them (`go build ./modules/gmcp/`).

- [ ] **Step 3: Go test** `gmcp.Char_repeat_test.go`

```go
package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

func TestFinishedRepeatQuests(t *testing.T) {
	qs := map[int]*quests.Quest{
		53: {QuestId: 53, Name: "Cull the Wash", Repeatable: true, CooldownRounds: 300},
		54: {QuestId: 54, Name: "Back to the Forge", Repeatable: true, CooldownRounds: 300, RepeatLimit: 1},
		55: {QuestId: 55, Name: "Another Brew", Repeatable: true, CooldownRounds: 300},
		56: {QuestId: 56, Name: "Hidden", Repeatable: true, Secret: true, CooldownRounds: 300},
	}
	lookup := func(id int) *quests.Quest { return qs[id] }
	util.SetRoundCountForTest(1000)
	c := &characters.Character{
		QuestCompletions: map[int]int{53: 1, 54: 1, 55: 2, 56: 1},
		QuestProgress:    map[int]string{55: "start"}, // in progress again: an ordinary row
	}
	c.SetQuestCooldown(53, 300)

	rows := finishedRepeatQuests(c, lookup)
	require.Equal(t, []GMCPCharModule_Payload_Quest{
		{Id: 53, Name: "Cull the Wash", Repeatable: true, Status: questStatusWaiting},
	}, rows, "54 is done for good, 55 is in progress, 56 is secret")

	util.SetRoundCountForTest(1300)
	require.Equal(t, questStatusReady, finishedRepeatQuests(c, lookup)[0].Status)
}
```

Run: `go test ./modules/gmcp/ -run TestFinishedRepeatQuests -v`
Expected: PASS (write this test before Step 2's helper and see it fail to compile first).

- [ ] **Step 4: Web test first.** In `quest-panel-refresh.js`, before the summary line, add:

```js
console.log('SCENARIO 6: repeatables are tagged and finished ones are listed in words');
reset();
handleGMCP('Char.Quests', [
  { id: 53, name: 'Cull the Wash', completion: 33, focused: true, repeatable: true, hint: 'Type attack scout.' },
  { id: 7, name: 'The Last Delivery', completion: 50, focused: false },
  { id: 54, name: 'Back to the Forge', repeatable: true, status: 'waiting' },
  { id: 58, name: 'Reading the Stones', repeatable: true, status: 'ready' },
]);
var p6 = panelText();
check('active repeatable shows "(repeatable)"', /Cull the Wash<\/span><span class="qrepeat">\(repeatable\)/.test(p6), p6.slice(0, 300));
check('the one-off quest has no tag', !/Last Delivery<\/span><span class="qrepeat">/.test(p6), p6.slice(0, 300));
check('a "Repeatable work" group follows the active quests', p6.indexOf('Repeatable work') > p6.indexOf('Last Delivery'), p6);
check('waiting status in words', p6.includes('Back to the Forge') && p6.includes('Not offered again yet.'), p6);
check('ready status in words', p6.includes('Reading the Stones') && p6.includes('Ready to take on again.'), p6);
check('finished rows are not focusable rows', !/class="qrow[^"]*" data-quest-id="54"/.test(p6), p6);

console.log('SCENARIO 7: only finished repeatables, nothing active');
reset();
handleGMCP('Char.Quests', [{ id: 54, name: 'Back to the Forge', repeatable: true, status: 'ready' }]);
var p7 = panelText();
check('empty state still says no active quests', p7.includes('No active quests'), p7);
check('and the repeatable work is listed', p7.includes('Back to the Forge') && p7.includes('Ready to take on again.'), p7);
```

Run: `node tools/webclient-tests/quest-panel-refresh.js`
Expected: scenarios 6 and 7 FAIL; 1 to 5 pass.

- [ ] **Step 5: Render.** In `renderQuests`, split the list after the copy line
  (`var quests = ...`):

```js
            // Finished repeatables (status set) are listed under their own
            // label, dimmed and unfocusable (#375 P6); the rest are active.
            var repeats = quests.filter(function (q) { return !!q.status; });
            quests = quests.filter(function (q) { return !q.status; });
            var repeatsHTML = "";
            if (repeats.length) {
                repeats.sort(function (a, b) { return (a.name || "").localeCompare(b.name || ""); });
                repeatsHTML = '<div class="qgroup">Repeatable work</div>';
                repeats.forEach(function (q) {
                    var line = q.status === "ready" ? "Ready to take on again." : "Not offered again yet.";
                    repeatsHTML += '<div class="qrepeatrow" data-quest-id="' + Number(q.id) + '">'
                        + '<span class="qname">' + escapeHTML(q.name || "") + '</span>'
                        + '<span class="qrepeat">(repeatable)</span>'
                        + '<div class="qhint">' + line + '</div>'
                        + '</div>';
                });
            }
```

Change the empty branch to keep the repeatable group:

```js
            if (!quests.length) {
                writeQuestsHTML(host, '<div class="quests-empty">No active quests.</div>' + repeatsHTML);
                return;
            }
```

In the active row, after the `qname` span:

```js
                      + '<span class="qname">' + escapeHTML(q.name || "") + '</span>'
                      + (q.repeatable ? '<span class="qrepeat">(repeatable)</span>' : '')
```

and append the group after the `forEach` over active quests, before the
`if (writeQuestsHTML(host, html))` line:

```js
            html += repeatsHTML;
```

The click binding selects `.qrow` only, so `.qrepeatrow` rows send no focus request. Every
GMCP string still goes through `escapeHTML` (the escape test in Step 7 checks it).

- [ ] **Step 6: Styles** in `_datafiles/html/public/static/css/dashboard.css`, beside the
  `.qname` rule (`grep -n "\.qname" _datafiles/html/public/static/css/dashboard.css`):

```css
.qrepeat { margin-left: 0.4em; opacity: 0.75; font-size: 0.9em; }
.qgroup { margin-top: 0.6em; opacity: 0.75; font-size: 0.85em; text-transform: uppercase; letter-spacing: 0.05em; }
.qrepeatrow { opacity: 0.6; padding: 2px 0; cursor: default; }
```

Dimming uses opacity, never a colour change, and the status is in words (colourblind rule).

- [ ] **Step 7: Run**

```bash
node tools/webclient-tests/quest-panel-refresh.js
node tools/jstest/webclient-html-escape.test.js
go test ./modules/gmcp/
```

Expected: `ALL CHECKS PASSED`, the escape test passes, `ok`.

- [ ] **Step 8: Commit**

```bash
git add modules/gmcp/gmcp.Char.go modules/gmcp/gmcp.Char_repeat_test.go _datafiles/html/public/webclient-pure.html _datafiles/html/public/static/css/dashboard.css tools/webclient-tests/quest-panel-refresh.js
git commit -F - <<'EOF'
feat(webclient): quest panel tags repeatables and lists finished ones (#375 P6)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 18: Docs

**Files:** the `context.md` of `internal/questloop` (new), `internal/questworld` (new),
`internal/quests`, `internal/questengine` (if it names the moved validators; check with
grep), `internal/dialogue`, `internal/conversation`, `internal/menu`, `internal/characters`;
the two help files; the requirements; the roadmap; `docs/README.md`.

**Verify before you document** (CLAUDE.md): every symbol named must exist. After writing,
run `python tools/context_md_audit.py` and expect no phantom symbols in the files you
touched.

- [ ] **Step 1: `internal/questloop/context.md`**: purpose (the loop rules, pure, proposed
  world), files (`questloop.go`, `grants.go`, `rules_loop.go`, `rule_oneaction.go`,
  `rules_more.go`, `backlog.go`), the rule table from spec 5.2 with rule ids, `Finding.Key`
  as the backlog line, the ratchet (fresh refuses, known warns, stale fails CI), the three
  doors and where each lives, and the plan 2b walkthrough rules still to come.
- [ ] **Step 2: `internal/questworld/context.md`**: purpose (live-registry inputs),
  `Names`, `Validators` (moved from gmcp), `Registry`, `WithQuest`, `BacklogPath`,
  `LoadBacklog`; callers (conversation card, `main_quests.go`, gmcp editor, the sweep).
- [ ] **Step 3: Updates.** `quests/context.md`: `repeat_limit`, `rewards.summary`,
  `rewards.surprise`, `RewardSummary`, `RewardPaidNames`, `RewardNames`, the moved
  `Parse*Grants`, the step-id manifest. `dialogue/context.md`: `PlayerState.QuestOpen` and the
  offer gate; `Manifest` is now `manifest.Manifest`. `conversation/context.md`: the card body,
  `queueNextOffer`, `HandleTalkNextOffer`, `TalkNextOffer` ordering. `menu/context.md`:
  `Body`. `characters/context.md`: `QuestCompletions`, `RecordCompletion`, `QuestOpen`,
  `LastRepeat`, `RepeatCapReached`, `RenameQuestSteps`.
- [ ] **Step 4: Help** (player copy: plain words, 80 columns, no dashes, no numbers):
  - `talk.template`, Quests section: after the Accept / Leave paragraph, add that the quest
    line shows what to do first and the reward, and that finishing a quest with someone
    can lead straight into their next one.
  - `quests.template`: a "Quests you can repeat" paragraph: some work can be taken again
    after a while; the list marks it `(repeatable)`; `quests all` shows each finished one with
    "Ready to take on again", "Not offered again yet" or "Done for good".
  Run the help-file guards (`go test ./... -run Help 2>&1 | tail -5`) and expect `ok`.
- [ ] **Step 5: Requirements** (`docs/roadmaps/QUEST_DIALOGUE_REQUIREMENTS.md`): rule 13
  becomes "Repeatable quests have a frequency, and a repeat limit where wanted." The done-when
  line becomes "Repeatable quests enforce their frequency, and their limit where set." Note
  under the rules: "(Owner 2026-10-24, phase 2 ruling P3.)"
- [ ] **Step 6: Roadmap** (`QUEST_DIALOGUE_ROADMAP.md`) phase 2: add **Spec** and **Plans**
  lines (this plan and the 2b plan, the latter "not written yet"), mark the two spec questions
  answered by P3 and P4, and set **Status:** "2a implemented on `feat/375-phase2a-loop`".
- [ ] **Step 7: `docs/README.md`**: rows for this plan, `internal/questloop/context.md`,
  `internal/questworld/context.md`, `internal/manifest/context.md`,
  `_datafiles/world/dogmud/quests/step-ids.txt`, `_datafiles/world/dogmud/quests/loop-backlog.txt`.
  Follow the table's existing row style.
- [ ] **Step 8: Commit** (name every file):

```bash
git add internal/questloop/context.md internal/questworld/context.md internal/quests/context.md internal/dialogue/context.md internal/conversation/context.md internal/menu/context.md internal/characters/context.md _datafiles/world/dogmud/templates/help/talk.template _datafiles/world/dogmud/templates/help/quests.template docs/roadmaps/QUEST_DIALOGUE_REQUIREMENTS.md docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md docs/README.md
git status --short
git commit -F - <<'EOF'
docs(375): phase 2a context.md, help, requirements and roadmap (#375 phase 2a)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 19: Gate, playtest, PR

- [ ] **Step 1: Full suite and the smoke sweeps**

```bash
go build ./... && go vet ./... && go test ./... 2>&1 | grep -v "^ok\|no test files" | head -40
DOGMUD_BOOT_SMOKE=1 go test . -run 'TestSmoke_' 2>&1 | tail -10
node tools/webclient-tests/quest-panel-refresh.js
```

Expected: no failing package; smoke sweeps PASS; `ALL CHECKS PASSED`. A failure is reported
with its output, never re-run until it passes.

- [ ] **Step 2: Pre-push gate** in the order `dogmud-shipping` gives (lint with
  `--new-from-merge-base`, race run of the touched packages, YAML parse of every changed
  YAML file). Follow the skill.

- [ ] **Step 3: Code review.** Dispatch a review subagent (Opus) over
  `git diff master...HEAD` against the spec, with this plan's facts table; fix what it
  confirms, with tests.

- [ ] **Step 4: Adversarial playtest** (`dogmud-playtesting`; this phase changes what
  players see). Goals for the seats:
  - take Cull the Wash at Garve (it needs 34-end: give the fixture character the
    prerequisite tokens) and read the card: objective, reward line, repeat line;
  - finish it, check `quests all` shows "Not offered again yet", that Garve shows no `[!]`
    and no offer line while on cooldown, then advance time past the cooldown and see
    "Ready to take on again" and the `[!]` back;
  - finish Find Your Footing at Toke and see the menu come back with the trail offers
    marked (several next offers); finish a quest whose giver has exactly one next offer and
    see its pitch and card follow the reward lines, in that order;
  - the web client quest panel shows `(repeatable)` on an active repeatable, and after
    finishing it lists the quest under "Repeatable work" as "Not offered again yet.", which
    turns to "Ready to take on again." on the first move after the cooldown ends;
  - try `give` of an offer's wanted item and see the same card.
  Brief the seats to return findings in their reply (seat subagents cannot write report
  files). File every finding as a GitHub issue (`--repo pruuk/DOGMud`) before the PR.

- [ ] **Step 5: PR.** Push the branch and open the PR with `gh pr create --repo pruuk/DOGMud`
  (never a bare `gh`), base `master`. The body: what changed (P1 to P6), the backlog counts,
  the playtest result, deploy notes ("new save field `questcompletions`, starts empty; no
  wipe; phase 1's deploy notes still apply"). No closing keyword for #375 (it is an epic;
  "closes #N" auto-closes). End with the session's PR attribution line. Do not merge: the
  owner approves the merge.

