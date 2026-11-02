# #375 phase 7 tier 3, The Confluence: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to
> implement this plan task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Clear all 51 backlog findings on quests 73, 74 and 78 to 87: the Margin mystery moves to `search`
with Quist offering The Undercroft, and Veyra teaches her nine secrets through a new `sale` topic kind, gated
on skill, with quests 78 to 87 retired and #397 closed.

**Architecture:** Two PRs in order. **3a** is content only. **3b** adds `sale` to `internal/dialogue` and
`internal/conversation`, then moves Veyra's content onto it. `sale` reuses the offer's pitch and card path and
`Apply`'s all-or-nothing charge.

**Tech stack:** Go, YAML world data, testify. Spec (owner approved):
`docs/superpowers/specs/2026-11-02-quest-arc-phase7-tier3-design.md`. Its tables and wording are approved: use
them verbatim.

**Worktrees:** 3a: `C:/tmp/dogmud-375-p7-tier3a`, branch `feat/375-p7-tier3a`, cut from
`docs/375-p7-tier3-spec`. 3b: `C:/tmp/dogmud-375-p7-tier3b`, branch `feat/375-p7-tier3b`, cut from master once
3a has merged. Paths are relative to them; data paths are under `_datafiles/world/dogmud/`.

**Brief for every subagent:**
- Edit files with Edit and Write only. Never rewrite a repo file from the shell. Deleting a file with `git rm` is fine.
- `git add` named paths only. Never `--no-verify`. Commit messages through a quoted heredoc, ending with a
  blank line then `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- No em or en dashes, YAML comments included. Hints at most 200 characters. No `;` in spoken text. An unquoted
  `: ` in a YAML plain scalar breaks the loader.
- Do not start a server except the isolated boot, and do not push. Test logs go in the session scratchpad, never
  `C:/tmp` or `$TMP`.
- The sweep needs `DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke -count=1` from the worktree root; it silently
  skips without the variable. Do not set it on `./internal/rooms`.
- `loop-backlog.txt`: delete a line when its finding is fixed. Never add a line.
- `HasQuest` counts later steps (`internal/characters/quests.go:67-89`).
- Do not weaken a test to pass. A test pinning the old behaviour is updated to follow the change, and reported.
- Report any fact below that does not match the source, rather than working around it.

## Facts verified against source (`e8d4c29e1` plus the spec commit, 2026-11-02)

| Fact | Source |
|---|---|
| `event: command, command: search, room: N` is the shipped pattern; `search` raises it unless on cooldown | `quests/20-ullas_silence.yaml`; `internal/usercommands/skill.search.go:24-33,51` |
| Quest 73 granting triggers: 6226 `73-chart`, 6131 `73-scrivener`, 6143 `73-map`; each has a flavour twin that grants nothing | `quests/73-*.yaml` |
| Quest 74 granting triggers: 6189 `74-ledger`, 6188 `74-record`, 6198 `74-survey`, 6215 `74-reveal` plus `give_item: 40142`; `topic_heard` 9465 `q74_turnin` grants `74-descent` | `quests/74-*.yaml` |
| NPC lines to rewrite: `9431.yaml:40`, `9435.yaml:39`, `9441.yaml:71,81,91,107`, `9444.yaml:40`, `9465.yaml:54,71,80,89-90,115,135` | `dialogue/the_confluence/` |
| A topic key moving between mobs is a `# renamed` line, as `9117.vein_offer -> 9116.vein_offer` | `dialogue/topic-keys.txt` |
| `Topic`, `Effects`, `Gates` | `internal/dialogue/topic.go:14-80` |
| `Eligible`, `TopicMarker`, `Marker`, `Apply` | `internal/dialogue/topics.go` |
| `tryCharge`, `applyPaidEffects` refund on failure | `internal/dialogue/engine.go:99-120` |
| `validateTopicFile`'s kind switch, `chargesGold` rule at line 205 | `internal/dialogue/world.go:133-207` |
| `World` validation callbacks are built at `world_registry.go`, e.g. `FlagDeclared` | `internal/dialogue/world_registry.go:148` |
| `resolve`, `play`, `offerCard`, `accept`, `talkState` | `internal/conversation/talk.go` |
| `playerState` closures, e.g. `CarriesItem`, `ChargeGold` | `internal/conversation/playerstate.go:43,106` |
| `crafting.GetRecipe(id) *RecipeSpec`, with `Skill`, `SkillMinimum`, `Station` | `internal/crafting/crafting.go:32-40,126` |
| The craft test: `char.GetSkillLevel(skills.SkillTag(recipe.Skill)) < recipe.SkillMinimum` | `internal/actions/craft.go:190-191` |
| `Character.LearnRecipe(id) bool`, `HasRecipe(id) bool` | `internal/characters/spells.go:133`; `internal/actions/craft.go:183` |
| `GameBridge.LearnRecipe` tells the player what they learned | `internal/questengine/bridge.go:462` |
| `mobs.CheckPlayerHarm(m) HarmBlock`, `HarmBlockedNonCombatant` | `internal/mobs/harm_authorization.go:41-49` |
| Tests touching the retired quests: `internal/guards/veyra_secrets_dialogue_test.go`, `internal/guards/quest_charge_gold_gate_guard_test.go`, `internal/hooks/Quest_HandleQuestUpdate_bridge_test.go:91`, `internal/crafting/pinnacle_assembly_test.go`. Fixtures that only borrow a number (`internal/dialogue/charges_gold_test.go`, `gold_gate_test.go`, `internal/characters/questfocus_test.go`, `internal/behaviortree/validate_test.go`) need no change | grep |
| Backlog is 128 lines before this plan; 3a takes it to 118, 3b to 77 | `quests/loop-backlog.txt` |

---

# PR 3a: the Margin

### Task 1: Quests 73 and 74 by `search`

**Files:** `quests/73-the_margin_notation.yaml`, `quests/74-the_undercroft.yaml`, the NPC files in the facts table,
`quests/loop-backlog.txt`. Test: create `internal/questengine/tier3_margin_test.go`.

- [ ] **Step 1: Failing test.**

```go
package questengine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func loadTier3Quest(t *testing.T, id string) *quests.Quest {
	t.Helper()
	paths, err := filepath.Glob("../../_datafiles/world/dogmud/quests/" + id + "-*.yaml")
	require.NoError(t, err)
	require.Len(t, paths, 1)
	data, err := os.ReadFile(paths[0])
	require.NoError(t, err)
	var q quests.Quest
	require.NoError(t, yaml.Unmarshal(data, &q))
	return &q
}

// Rule 14: a step completes on something the player does, never a typed noun.
func TestTheMarginsRecordsAreFoundBySearch(t *testing.T) {
	for id, sites := range map[string]map[int]string{
		"73": {6226: "73-chart", 6131: "73-scrivener", 6143: "73-map"},
		"74": {6189: "74-ledger", 6188: "74-record", 6198: "74-survey", 6215: "74-reveal"},
	} {
		q := loadTier3Quest(t, id)
		found := map[int]bool{}
		for _, tr := range q.Triggers {
			require.NotEqual(t, "room_interact", tr.Event, "rule 14: no typed noun in quest %s", id)
			if tr.Event != "command" || tr.Command != "search" {
				continue
			}
			for _, a := range tr.Actions {
				if a.Grant != "" {
					require.Equal(t, sites[tr.Room], a.Grant, "quest %s room %d", id, tr.Room)
					found[tr.Room] = true
				}
			}
		}
		require.Len(t, found, len(sites), "quest %s", id)
	}
}

func TestTheOldSkyGivesTheRubbing(t *testing.T) {
	q := loadTier3Quest(t, "74")
	for _, tr := range q.Triggers {
		if tr.Event == "command" && tr.Room == 6215 {
			for _, a := range tr.Actions {
				if a.GiveItem == 40142 {
					return
				}
			}
		}
	}
	t.Fatal("the search at 6215 gives no rubbing")
}
```

Run `go test ./internal/questengine/ -run 'TheMargins|TheOldSky' -count=1`. See it fail on `room_interact`.

- [ ] **Step 2: Triggers.** Each granting `room_interact` becomes the block below, keeping its `conditions`,
`grant`, `actor`, `observer` and any `give_item`. Delete each flavour twin (the trigger with `missing` of the same
or an earlier step and no grant). Keep the `topic_heard` and every other trigger.

```yaml
  - event: command
    command: search
    room: 6226
```

- [ ] **Step 3: Hints.** Replace from the spec's table, verbatim: 73 `start`, `chart`, `scrivener`; 74 `start`,
`ledger`, `record`, `survey`, `descent`, `reveal`. Walk each direction against the room files, and fix a wrong one
in both the quest and the spec's table, reporting it.

- [ ] **Step 4: NPC lines.** Each "Type look damaged chart" and similar in the facts table's list becomes "Type
search." Read each line. Check the seven rooms' noun text: it may describe the thing, and it must not promise a
find a `look` no longer gives. Report anything changed.
`grep -rnE 'look (damaged|old|marginal|building|consecration|mason|orbital)' _datafiles/world/dogmud/dialogue/the_confluence _datafiles/world/dogmud/quests/7[34]-*.yaml _datafiles/world/dogmud/rooms/the_confluence` should find no instruction to type it (exit 1 is the pass).

- [ ] **Step 5: Run.** `go test ./internal/questengine/ ./internal/dialogue/ ./internal/quests/ ./internal/questloop/ ./internal/narration/ ./internal/guards/ -count=1` and the sweep. A narration golden diff that is only the removed flavour lines is regenerated with `go test ./internal/narration/ -update -count=1`. Delete the `hint-length` lines for 73 and 74 from the backlog.

- [ ] **Step 6: Commit** with named paths, message `fix(quests): the Margin's records are found by search`.

### Task 2: Quist offers The Undercroft

**Files:** `dialogue/the_confluence/9441.yaml`, `9465.yaml`, `dialogue/topic-keys.txt`, `quests/loop-backlog.txt`.
Test: `internal/dialogue/shipped_quest_bugs_test.go`.

- [ ] **Step 1: Failing test,** appended:

```go
// Tier 3: Quist offers The Undercroft at his hand-in, through the chain.
func TestQuistOffersTheUndercroft(t *testing.T) {
	quist := loadShippedDialogue(t, "the_confluence/9441.yaml")
	require.Contains(t, menuKeys(quist, heldState("73-end")), "undercroft_offer")
	require.NotContains(t, menuKeys(quist, heldState("73-map")), "undercroft_offer")
	require.NotContains(t, menuKeys(quist, heldState("73-end", "74-start")), "undercroft_offer")

	cael := loadShippedDialogue(t, "the_confluence/9465.yaml")
	require.Nil(t, cael.FindTopic("q74_grant"), "the offer moved to Quist")
	require.Contains(t, menuKeys(cael, heldState("74-start")), "q74_work")
	require.NotContains(t, menuKeys(cael, heldState("74-start", "74-ledger")), "q74_work")
}
```

- [ ] **Step 2: The offer.** Add to `9441.yaml` after `report`, and add `9465.q74_grant -> 9441.undercroft_offer` under `# renamed`:

```yaml
- key: undercroft_offer
  kind: offer
  quest: 74
  label: The Undercroft
  playerLine: Where does this lead?
  text: Under the temple, if you have the stomach for it. Brother Cael keeps the
    archive on the island, and he has been reading the same records alone for years.
    Tell him the Margin sent you. There are three of the temple's own that disagree
    with its founding story, and he will show you where.
  hints: Accept, then go to Brother Cael in the Archive on the temple island.
  questRequired:
  - 73-end
  questExcluded:
  - 74-start
  - 74-end
  grantsQuest: 74-start
```

- [ ] **Step 3: Cael.** Replace `q74_grant` in `9465.yaml`, and add `9465.q74_work` to `topic-keys.txt`:

```yaml
- key: q74_work
  kind: info
  label: What should I read?
  text: 'So the Margin''s question has reached the archive, and you with it. Good.
    I am tired of being the only one who reads these. Here is the work, and it is
    reading, not digging: three of the temple''s own records disagree with its
    founding story. The building ledger here, open on the stand. The consecration
    record in the Chapter House. And a mason''s survey in the Older East Corridor,
    by the stair. See all three yourself. Aldric has allowed that much. Then come
    back to me.'
  hints: Read the building ledger open on the stand beside Brother Cael. Type search.
  marked: true
  questRequired:
  - 74-start
  questExcluded:
  - 74-ledger
  - 74-end
```

Cael's `73-end`, excluding `74-start`, opener variant (lines 92-100) hints that he would welcome help: reword the hint
to "Quist the Elder at the Margin Hall has the next step, if you have not taken it from him." Read the other
variants for anything that assumes he offers it.

- [ ] **Step 4:** Delete `chain 73 mob 9465 topic q74_grant`. Run the Task 1 command and the sweep. The hand-in
`report` must queue the offer: `queueNextOffer` needs exactly one next offer on his file, and the `73-end` gated
topics that are `info` do not count.

- [ ] **Step 5: Commit** with named paths, message `fix(quests): Quist offers The Undercroft at his hand-in`.

### Task 3: Hand-ins, guard, docs, gates

**Files:** `dialogue/the_confluence/9464.yaml`, `9454.yaml`, `quests/74-the_undercroft.yaml`, `quests/loop-backlog.txt`,
`internal/guards/tier3_backlog_guard_test.go`, `docs/PATCH_NOTES.md`, `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`, `docs/README.md`.

- [ ] **Step 1: Failing test,** appended to `shipped_quest_bugs_test.go`:

```go
func TestTheUndercroftsEndingsFollowTheReveal(t *testing.T) {
	for _, c := range []struct{ file, topic string }{
		{"the_confluence/9464.yaml", "aldrichandoff"},
		{"the_confluence/9454.yaml", "marginhandoff"},
	} {
		df := loadShippedDialogue(t, c.file)
		state := func(tok string) *PlayerState {
			ps := heldState(tok)
			ps.HasItem = func(id int) bool { return id == 40142 }
			ps.CarriesItem = ps.HasItem
			return ps
		}
		require.NotContains(t, menuKeys(df, state("74-descent")), c.topic, c.file)
		require.Contains(t, menuKeys(df, state("74-reveal")), c.topic, c.file)
	}
}
```

Adapt the fixtures to each topic's real gates without loosening the check. See it fail.

- [ ] **Step 2:** `aldrichandoff`, `marginhandoff`: `questRequired` `74-reveal`. Step `reveal` gains
`destination: {mob: [9464, 9454]}`. Delete the remaining 73 and 74 backlog lines. Check 74's and 73's dashes in comments.

- [ ] **Step 3: The guard.** Create `internal/guards/tier3_backlog_guard_test.go`, the same shape as
`tier2_backlog_guard_test.go`, with `tier3Quests` = 73, 74 and, once 3b lands, the retired ones. For this PR: 73 and 74.
Its `_GuardCanFail` fixture uses `hint-length 74 step record`, `chain 73 mob 9465 topic q74_grant`, and a line for another quest.

- [ ] **Step 4: Docs.** Patch note under a new dated heading:

```markdown
## 2026-11-02: The Margin's question

- The Margin Notation and The Undercroft are now followed by searching: at each
  record, type search to find what it shows.
- Quist the Elder sends you on to Brother Cael as soon as you bring him the three
  records.
```

Add the guard and this plan to `docs/README.md`. Roadmap: tier 3's line says the Margin is built and Veyra is next.

- [ ] **Step 5: Gates.** `gofmt -l internal/ modules/`, `go build ./...`, `go test ./... -count=1`, `DOGMUD_BOOT_SMOKE=1 go test . -count=1`, `golangci-lint run --new-from-merge-base=origin/master`, and the isolated boot per the `dogmud-shipping` skill. The backlog must print 118: `grep -v '^#' _datafiles/world/dogmud/quests/loop-backlog.txt | grep -c .`

- [ ] **Step 6: Commit** with named paths, message `fix(quests): The Undercroft's endings follow the reveal`.

### Task 4: 3a playtest and PR (controller)

- [ ] Run a `Margin` seat from the spec's table (`mid`, Margin Hall, a torch), in a committed `tools/playtest/scenarios/375-p7-tier3a.yaml`. Check its claims against raw `events.jsonl`. Fix what belongs to this pass; file the rest.
- [ ] Clean up by run id and PID. Push, open the PR on `pruuk/DOGMud`, wait for all four CI jobs, merge with `--merge`. Remove the worktree.

---

# PR 3b: Veyra

### Task 5: The `sale` kind in `internal/dialogue`

**Files:** `internal/dialogue/topic.go`, `topics.go`, `types.go`, `world.go`, `world_registry.go`. Test: create `internal/dialogue/sale_test.go`, and add rows to `world_test.go`.

- [ ] **Step 1: Failing tests.**

```go
package dialogue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func saleTopic() *Topic {
	return &Topic{
		Key: "teach_x", Kind: KindSale, Label: "Secret of X", PlayerLine: "Teach me?", Text: "pitch",
		NotReadyText: "Come back.", PaidText: "Paid.",
		Effects: Effects{ChargesGold: 100, TeachesRecipes: []string{"a", "b"}},
	}
}

func saleState(known map[string]bool, skilled map[string]bool) *PlayerState {
	return &PlayerState{
		HasQuest:       func(string) bool { return false },
		KnowsRecipe:    func(id string) bool { return known[id] },
		RecipeSkillMet: func(id string) bool { return skilled[id] },
	}
}

func saleFile(t *Topic) *DialogueFile { return &DialogueFile{MobId: 1, Topics: []Topic{*t}} }

func TestSale_HiddenOnlyWhenEveryRecipeIsKnown(t *testing.T) {
	tp := saleTopic()
	df := saleFile(tp)
	require.True(t, df.Eligible(tp, saleState(nil, nil)), "no skill: still listed")
	require.True(t, df.Eligible(tp, saleState(map[string]bool{"a": true}, nil)), "knows part of it")
	require.False(t, df.Eligible(tp, saleState(map[string]bool{"a": true, "b": true}, nil)))
}

func TestSaleReady_NeedsEverySkill(t *testing.T) {
	tp := saleTopic()
	require.False(t, SaleReady(tp, saleState(nil, nil)))
	require.False(t, SaleReady(tp, saleState(nil, map[string]bool{"a": true})), "one of two")
	require.True(t, SaleReady(tp, saleState(nil, map[string]bool{"a": true, "b": true})))
	require.False(t, SaleReady(tp, &PlayerState{}), "a nil callback fails closed")
}

func TestSale_MarkerOnlyWhenReady(t *testing.T) {
	tp := saleTopic()
	df := saleFile(tp)
	require.Empty(t, df.TopicMarker(tp, saleState(nil, nil)))
	require.Equal(t, "[*]", df.TopicMarker(tp, saleState(nil, map[string]bool{"a": true, "b": true})))
}

func TestSale_ApplyChargesThenTeaches(t *testing.T) {
	tp := saleTopic()
	df := saleFile(tp)
	var learned []string
	paid := 0
	ps := saleState(nil, map[string]bool{"a": true, "b": true})
	ps.LearnRecipe = func(id string) { learned = append(learned, id) }
	ps.ChargeGold = func(n int) bool { paid = n; return true }
	require.True(t, df.Apply(tp, ps))
	require.Equal(t, 100, paid)
	require.Equal(t, []string{"a", "b"}, learned)
}

func TestSale_RefusedChargeTeachesNothing(t *testing.T) {
	tp := saleTopic()
	df := saleFile(tp)
	var learned []string
	ps := saleState(nil, map[string]bool{"a": true, "b": true})
	ps.LearnRecipe = func(id string) { learned = append(learned, id) }
	ps.ChargeGold = func(int) bool { return false }
	require.False(t, df.Apply(tp, ps))
	require.Empty(t, learned)
}
```

Use the real constant for `[*]`, `menu.MarkerInfo`. Add `breakFixture` rows to `world_test.go`, reading how the table builds a fixture: a `sale` with no `teachesRecipes`, with an unknown recipe, with no `chargesGold`, no `notReadyText`, no `paidText`, no `playerLine`, with `grantsQuest`; and `teachesRecipes`, `notReadyText`, `paidText` on an `info` topic; a dash in `notReadyText`. Run and see them fail on undefined names.

- [ ] **Step 2: The fields.**

`topic.go`, below `KindHandIn`: `KindSale = "sale"`. In `Effects`:

```go
	// TeachesRecipes is what a sale teaches once its price is paid.
	TeachesRecipes []string `yaml:"teachesRecipes,omitempty" json:"teachesRecipes,omitempty"`
```

In `Topic`, below `Objective`:

```go
	// NotReadyText is what the NPC says when the player chooses a sale without
	// the skill for every recipe. PaidText is what the NPC says once it is paid.
	// Sales only.
	NotReadyText string `yaml:"notReadyText,omitempty" json:"notReadyText,omitempty"`
	PaidText     string `yaml:"paidText,omitempty" json:"paidText,omitempty"`
```

`types.go`, in `PlayerState`:

```go
	// KnowsRecipe reports whether the player knows a recipe. It hides a sale whose
	// recipes are all known. Nil counts as not known.
	KnowsRecipe func(id string) bool
	// RecipeSkillMet reports whether the player has the skill the recipe needs, the
	// same test the craft command uses. Nil fails closed.
	RecipeSkillMet func(id string) bool
	// LearnRecipe teaches a recipe. It backs a sale's teachesRecipes.
	LearnRecipe func(id string)
```

- [ ] **Step 3: The logic.** In `topics.go`:

```go
// SaleReady reports whether the player has the skill for every recipe a sale
// teaches, components included. These are pinnacle items, and the bar is all of them.
func SaleReady(t *Topic, ps *PlayerState) bool {
	if t == nil || ps == nil || ps.RecipeSkillMet == nil || len(t.TeachesRecipes) == 0 {
		return false
	}
	for _, id := range t.TeachesRecipes {
		if !ps.RecipeSkillMet(id) {
			return false
		}
	}
	return true
}

// saleKnown reports whether the player already knows every recipe a sale teaches.
func saleKnown(t *Topic, ps *PlayerState) bool {
	if ps == nil || ps.KnowsRecipe == nil || len(t.TeachesRecipes) == 0 {
		return false
	}
	for _, id := range t.TeachesRecipes {
		if !ps.KnowsRecipe(id) {
			return false
		}
	}
	return true
}
```

In `Eligible`, after the `ps == nil` return: `if t.Kind == KindSale && saleKnown(t, ps) { return false }`.

In `TopicMarker`'s switch: `case KindSale: if SaleReady(t, ps) { return menu.MarkerInfo }; return ""`.

In `Apply`, after `applyPaidEffects` succeeds:

```go
	if ps != nil && ps.LearnRecipe != nil {
		for _, id := range t.TeachesRecipes {
			ps.LearnRecipe(id)
		}
	}
```

Read how `Menu` builds a line, and how the menu's "heard" dimming reads a kind: a `sale` line must read as its label, not dimmed.

- [ ] **Step 4: The validator.** Add `RecipeExists func(id string) bool` to `World`, wired at `world_registry.go` to `crafting.GetRecipe(id) != nil` (a nil callback skips the check, as its siblings do). In `validateTopicFile`, the first loop treats `KindSale` as `KindInfo`'s label uniqueness does not apply to it, and the switch gains:

```go
		case KindSale:
			if t.PlayerLine == "" || t.NotReadyText == "" || t.PaidText == "" {
				add(where, "sale topics need playerLine, notReadyText and paidText")
			}
			if t.ChargesGold <= 0 {
				add(where, "sale topics need chargesGold")
			}
			if len(t.TeachesRecipes) == 0 {
				add(where, "sale topics need teachesRecipes")
			}
			if t.GrantsQuest != "" || t.Quest != 0 {
				add(where, "a sale is not a quest: no grantsQuest and no quest")
			}
			for _, id := range t.TeachesRecipes {
				if w.RecipeExists != nil && !w.RecipeExists(id) {
					add(where, "teachesRecipes %q is not a recipe", id)
				}
			}
```

After the switch, refuse the three fields on any other kind with "teachesRecipes, notReadyText and paidText belong on sale topics only". Exempt `KindSale` from the `chargesGold needs goldRequired` rule: Accept is its gate. Give `notReadyText` and `paidText` the same dash and `semi` checks as `breadcrumb` (around line 189). If an existing unknown-kind check rejects it, add the kind there.

- [ ] **Step 5: Run.** `go test ./internal/dialogue/ -count=1`, then `go test ./... -count=1`: other packages read kinds. Expected: PASS.

- [ ] **Step 6: Commit** with named paths, message `feat(dialogue): a sale topic teaches recipes for a price`.

### Task 6: `sale` in `internal/conversation`

**Files:** `internal/conversation/talk.go`, `card.go`, `playerstate.go`. Test: create `internal/conversation/sale_test.go`.

- [ ] **Step 1: Failing test.** Find tier 1's menu tests with `grep -rln TalkMenuAgain internal --include=*_test.go`, and read them for how a real user, mob and room are fixtured, and how a menu is driven. Using it, with a sale teaching two recipes seeded in `crafting`:
  - no skill: choosing it says `notReadyText`, charges nothing, opens no card, teaches nothing, and the talk menu is open again;
  - skill for one recipe of two: the same;
  - skill for both, gold enough: pitch, card whose body holds "Price:", Accept charges, teaches both, says `paidText`, and the menu reopens without the line;
  - skill for both, too little gold: Accept prints "You don't have enough gold for that.", teaches nothing, and the line stays.

Also `playerstate` tests: `RecipeSkillMet` matches `actions`' test on a seeded recipe (below and at the minimum), and returns false for an unknown recipe.

- [ ] **Step 2: PlayerState.** In `playerState` (`playerstate.go`), beside the others:

```go
		KnowsRecipe: user.Character.HasRecipe,
		RecipeSkillMet: func(id string) bool {
			r := crafting.GetRecipe(id)
			if r == nil {
				return false
			}
			return user.Character.GetSkillLevel(skills.SkillTag(r.Skill)) >= r.SkillMinimum
		},
		LearnRecipe: func(id string) {
			if user.Character.LearnRecipe(id) {
				// the same line GameBridge.LearnRecipe prints; share it, do not copy it
			}
		},
```

Read `GameBridge.LearnRecipe` (`bridge.go:462`) and move its notice into one helper both call, so both print the same line. Check the import does not make a cycle (`go list -deps ./internal/crafting` names neither package today).

- [ ] **Step 3: The path.** In `resolve`, before the `KindOffer` branch:

```go
	if t.Kind == dialogue.KindSale {
		playerSay(user, room, t.PlayerLine)
		if !dialogue.SaleReady(t, ps) {
			npcSay(mob, room, t.NotReadyText)
			openMenu(user, mob.InstanceId, "")
			return
		}
		npcSay(mob, room, t.Text)
		sendHints(user, t.Hints)
		saleCard(user, mob, t)
		return
	}
```

Add beside `offerCard`:

```go
// saleCard is the private Accept / Leave after a sale's pitch, with its price.
func saleCard(user *users.UserRecord, mob *mobs.Mob, t *dialogue.Topic) {
	inst, key := mob.InstanceId, t.Key
	card := menu.Confirm(t.Label, "Accept", "Leave")
	card.Body = []string{fmt.Sprintf("Price: %s gold.", util.FormatNumber(t.ChargesGold))}
	user.OpenMenu(card, session(user, mob, func(e menu.Entry) {
		if e.Id == "yes" {
			acceptSale(user, inst, key)
		}
		openMenu(user, inst, "")
	}))
}

// acceptSale takes the price and teaches, after re-checking the sale: the player
// may have dropped gold or changed gear since the pitch.
func acceptSale(user *users.UserRecord, mobInstanceId int, key string) {
	mob, room, ok := present(user, mobInstanceId)
	if !ok {
		return
	}
	df := dialogue.Load(int(mob.MobId), mob.Zone)
	buf := &lineBuffer{user: user}
	ps := talkState(user, buf)
	t := df.FindTopic(key)
	if t == nil || !df.Eligible(t, ps) || !dialogue.SaleReady(t, ps) {
		user.SendText(messaging.CategoryRefusal, `That offer is no longer open.`)
		return
	}
	if !applyWithHandover(user, room, mob, df, t, ps, buf) {
		return
	}
	npcSay(mob, room, t.PaidText)
	after(user, room, mob, df, t)
}
```

Use the helper `ChargeGold`'s line in `playerstate.go` uses for thousands, in place of `util.FormatNumber` if it differs.

- [ ] **Step 4: Run.** `go test ./internal/conversation/ ./internal/questengine/ ./internal/hooks/ -count=1`. Expected: PASS.

- [ ] **Step 5: Commit** with named paths, message `feat(talk): a sale pitches, shows its price, and teaches on Accept`.

### Task 7: Veyra's content

**Files:** `dialogue/the_confluence/9584.yaml`, `dialogue/topic-keys.txt`, `quests/step-ids.txt`, `quests/loop-backlog.txt`; `git rm` `quests/78-*.yaml` to `quests/87-*.yaml`; `internal/guards/veyra_secrets_dialogue_test.go`, plus the tests in the facts table if they pin these quests. Test: create `internal/guards/veyra_cannot_be_harmed_test.go`.

- [ ] **Step 1: Failing tests.** Rewrite `veyra_secrets_dialogue_test.go`. It must prove, reading the shipped files:
  - nine `sale` topics, each with the price, recipes and `notReadyText` from the spec's table;
  - each `notReadyText` names exactly the stations of its recipes, read through `crafting` and the recipe files with a station-to-words map (`forge`: "the forge", `loom`: "the loom", `alchemy_bench`: "the alchemy bench", `jeweler_bench`: "the jeweler's bench", `enchanting_circle`: "the enchanting circle", `cooking_fire`: "the cooking fire"), and holds no digit;
  - a player with `80-start`'s recipes sees no `teach_blackrazor`;
  - `secrets_list` is gone, and no topic gates on `78-` to `87-`.

Add to `internal/guards/tier3_backlog_guard_test.go`'s `tier3Quests`: 78 to 87. Add a test that `quests.RetiredQuests()` holds 78 to 87, that no file in `quests/`, `dialogue/` or `behaviors/` names their tokens. `veyra_cannot_be_harmed_test.go`:

```go
package guards

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/stretchr/testify/require"
)

// Owner ruling 2026-11-02: Veyra teaches the pinnacle recipes, so nobody may grief
// by keeping her dead or baiting her into a fight.
func TestVeyraCannotBeHarmed(t *testing.T) {
	configs.SetConfigForTest(t, configs.GetConfig())
	require.NoError(t, configs.ReloadConfig())
	templates, err := fileloader.LoadAllFlatFiles[int, *mobs.Mob](configs.GetFilePathsConfig().DataFiles.String() + `/mobs`)
	require.NoError(t, err)
	veyra := templates[9584]
	require.NotNil(t, veyra)
	require.True(t, veyra.IsNonCombatant())
	require.False(t, veyra.LegacyHostile || veyra.AutoAggro)
	require.Empty(t, veyra.CombatCommands)
	require.Equal(t, mobs.HarmBlockedNonCombatant, mobs.CheckPlayerHarm(veyra))
}
```

Match the field names to `internal/mobs/mobs.go`, and add the `charm_immune` field. See them fail.

- [ ] **Step 2: The sales.** In `9584.yaml`, each `teach_*` keeps its key, label, `playerLine`, `text` and `hints`, and becomes:

```yaml
  kind: sale
  notReadyText: Come back when you've practiced at the jeweler's bench, the alchemy bench and the forge.
  paidText: Paid. The recipes are in your hands. Forge it at my stations when you are ready.
  chargesGold: 25000
  teachesRecipes: [hungering-guard, obsidian-edge-resin, assemble-the-blackrazor]
```

Remove `quest`, `questRequired`, `questExcluded`, `goldRequired`, `grantsQuest`. Each `paidText` is that quest's `npc_say` "Paid." line, read from `quests/<id>-*.yaml` before deleting it; each `notReadyText` and recipe list is from the spec's table. Each `hints` that says it needs "the coin" or to accept keeps true; reword only what is false.

- [ ] **Step 3: Her file.** Per the spec's "Her gate and the rest of her file": `convergence_intro` becomes `kind: info` with no `quest`, `playerLine`, `breadcrumb`, gates, or grant, and text losing "Say the word, and you're known to me. Then we can talk about what my secrets cost." and its hint rewritten. `secrets_list` goes. Opener: base line stays, the `77-end` variant gates on `77-end` alone, and its text and the base say nothing about being known; the masterwork variant keeps `masterworkRequired: 50` and loses its `78` gates, and its hint changes to "Choose a line below to hear what she teaches."; the two `78-end` variants go. `gold` and `confluence` lose their `78-end` gates, and `gold`'s last sentence becomes "I teach hands that have earned it." Update the file's header comment to describe sales.

- [ ] **Step 4: Retire.** `git rm` the ten quest files. In `step-ids.txt`, move `78-end` to `87-start` under `# retired` with a comment: `# Quests 78 to 87 (#375 phase 7 tier 3): Veyra sells recipes through talk, no quest.` In `topic-keys.txt`, move `9584.secrets_list` under `# retired`. Delete the 41 backlog lines.

- [ ] **Step 5: Run.** `go test ./internal/guards/ ./internal/dialogue/ ./internal/quests/ ./internal/questloop/ ./internal/questengine/ ./internal/crafting/ ./internal/hooks/ ./internal/narration/ -count=1` and the sweep. Read each failure: a test pinning a shipped quest follows the change. `learn_only` recipes must still have a source: if a rule or test requires a quest to teach them, make it read a sale's `teachesRecipes`, and report it. The narration golden is regenerated after reading the diff, and only loses these quests' lines.

- [ ] **Step 6: Commit** with named paths, message `feat(veyra): she teaches her secrets, gated on skill, with no quests`.

### Task 8: Docs and gates

**Files:** `docs/PATCH_NOTES.md`, `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`, `docs/roadmaps/QUEST_DIALOGUE_REQUIREMENTS.md`, `docs/README.md`, `internal/dialogue/context.md`, `internal/conversation/context.md`, `.claude/skills/dogmud-authoring-quests/SKILL.md`.

- [ ] **Step 1: Patch note.**

```markdown
## 2026-11-02: Veyra's secrets

- Veyra Coil-Tongue now teaches each of her nine secrets herself. Talk to her and
  choose the one you want. If you have the skill in every trade it takes, she names
  her price. If you do not, she tells you where to go and practice.
- She teaches every recipe a secret needs in one sitting, and what you have already
  bought stays yours.
```

- [ ] **Step 2:** Context files describe `KindSale`, `TeachesRecipes`, `NotReadyText`, `PaidText`, `SaleReady`, the three `PlayerState` callbacks, `saleCard`. The skill gains a short "Selling recipes" section, with a sale's trimmed YAML and the rule that a sale is never a quest. The requirements page's checkbox "Pinnacle recipes 79 to 87 are sold" is ticked, with its Coulee half checked for truth first.

- [ ] **Step 3: Gates.** `gofmt -l internal/ modules/`, `go build ./...`, `go test ./... -count=1`, `DOGMUD_BOOT_SMOKE=1 go test . -count=1`, `golangci-lint run --new-from-merge-base=origin/master`, the isolated boot. The backlog must print 77.

- [ ] **Step 4: Commit** with named paths, message `docs: Veyra's sales in context, skill, roadmap and patch notes`.

### Task 9: 3b playtest and PR (controller)

- [ ] Run the `Master crafter`, `Half skilled` and `Griefer` seats from the spec's table, in a committed `tools/playtest/scenarios/375-p7-tier3b.yaml`. Check that `grant_skills` reaches `GetSkillLevel`. Check claims against raw `events.jsonl`, and the server log for Veyra's health. Fix what belongs to this pass, with a test; file the rest.
- [ ] Clean up by run id and PID. Finish the roadmap lines. Push, open the PR on `pruuk/DOGMud` with `Closes #397.`, wait for all four CI jobs, merge with `--merge`. Remove the worktree, delete `docs/375-p7-tier3-spec`, and put the main checkout on master.
