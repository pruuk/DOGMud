# Messaging M7 slice 2: prompts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Every subagent brief carries these rules:** edit files with the Edit and Write tools only (no `sed -i`, no Python `open(path, 'w')`, no `>` into a tracked file; `~/.claude/hooks/guard.py` blocks them); `git add` named paths only; never `--no-verify`; no em or en dashes in prose or player text; every `gh` command carries `--repo pruuk/DOGMud`.

**Goal:** Every command typed without what it needs asks one way ("Pet what? Try **pet &lt;name&gt;**."), from one helper, on `CategoryRefusal`, and a guard stops a new hand-written prompt.

**Architecture:** `internal/refusal` gains `Prompt(to, verb, ask, usage)` and `PromptText`. The question word comes from a fixed `Ask` set, so no call site writes a question literal and the slice 1 phrase guard can police them with one new row. The 11 special moves get their prompt from `actions.StageMeleeTarget`, which switches to the helper. Seven "prompts" that are really misses (a name was typed and matched nothing) move to the slice 1 not-here lines instead.

**Tech Stack:** Go, testify, the slice 1 `go/ast` phrase guard.

**Spec:** `docs/superpowers/specs/completed/2026-10-14-messaging-m7-refusals-design.md`, "Suggested slicing" item 2 ("Prompts (`Prompt`), with usage hints"). Slice 1 shipped as #547 (`8362b2bfd`). Per the spec's Testing section, the playtests fall at the end of slices 1 and 3, so this slice has no playtest.

---

## Facts verified against source (`origin/master` `8362b2bfd`, 2026-10-15)

| # | Fact | Source |
|---|---|---|
| 1 | `internal/refusal` has `send`, `echo`, `hint` (`refusal.go:30,37,44`); `hint(c)` returns `` ` Try <ansi fg="command">`+c+`</ansi>.` `` | `internal/refusal/refusal.go` |
| 2 | `NotHereCreature(to)`, `NotHereThing(to)` and their `...Line` constants exist | `internal/refusal/nothere.go:9-19` |
| 3 | Test helpers `fakeRecipient`, `one(t, f, cat)`, `fits80(t, line)` exist in package `refusal` | `internal/refusal/refusal_test.go:19-56` |
| 4 | The phrase guard table is `refusalPhrases` in the root `refusal_phrase_guard_test.go`; each row has `family`, `re`, `probe`; `TestRefusalPhraseGuardCanFail` needs every probe to hit | `refusal_phrase_guard_test.go:23-50,121-137` |
| 5 | 44 Go string literals outside `internal/refusal` and tests match `(?i)^\S+( (on\|from\|to\|at\|with))? (what\|whom\|who\|how much)\?`, listed in the site tables below; none is an interactive `cmdPrompt.Ask` question (`mail.go:104` "Send mail to whom?" and the two "Attach how much gold?" do not match) | dry run of the pattern over `internal/` and `modules/`, comment lines skipped |
| 6 | `StageMeleeTarget` sends `opts.promptMsg()` on `CategorySystem` when `rest == ""`; `promptMsg` returns `PromptMsg` or title-cased `Verb + " whom?"`, `"Attack whom?"` for an empty verb | `internal/actions/melee_target.go:53-61,168-170` |
| 7 | `MeleeTargetOpts.PromptMsg` is set by one caller: `pounce.go:25` (`"Pounce on whom?"`) | grep `PromptMsg` |
| 8 | 11 callers build `MeleeTargetOpts` through `stageSpecialMoveTarget`: bash, drain, gore, grapple, kick, maul, pounce, rake, taunt, throttle, trip | grep `MeleeTargetOpts{` |
| 9 | `melee_target_test.go` pins each move's prompt (`prompt: "Kick whom?"` etc., lines 26-113) and checks `tt.opts.promptMsg()` at `:122` | file |
| 10 | Tests that pin prompt text: `say_empty_test.go:20` (`[]string{"Say what?"}`), `shout_empty_test.go:20` (`"Shout what?"`), `storage_remove_slot_test.go:34` (`Contains(lower(got), "add what?")`), `shapes_roster_test.go:64` (`NotContains(out, "Look at what???")`) | grep of `_test.go` |
| 11 | `special_move_categories_test.go:56-60` `systemNoticeMarkers` allows `"Throw what?"` on `CategorySystem` in `throw.go`; the scan only reads lines containing `messaging.CategorySystem` | file |
| 12 | `lock.go:20` (the `lock` command) says "Unlock what?" | file |
| 13 | `equip` is the command, with keywords `wear`, `wield`, `hold`; its bare prompt is "Wear WHAT?" | `usercommands.go:109`, `keywords.yaml:289,330`, `equip.go:61-63` |
| 14 | Help syntax: `put stick in chest`, `bank deposit [amount/all]`, `storage add <item>`, `pet <petname>`, `dismiss <name>`, `reply <message>`, `talk saris`, `plant <item> <target>`, `shadow [target]`, `defuse [direction]`/`[container name]`, `lock [exit name/container name]`, `cast identify sword`; auction bids go through the `auction` command (`auctions.go:66`, `args[0] == "bid"` at `:299`) | `_datafiles/world/dogmud/templates/help/*.template` |
| 15 | Slice 1 left two hand-written how-much prompts: `drop.go:80` and `give.go:424` `giveHowMuchGoldLine` | grep `how much gold` |
| 16 | The miss paths: `ask.go:103` after `ResolveTargetActor` fails; `look.go:543` after every look target failed; `skill.skullduggery.shadow.go:72`, `follow.go:554` after the target resolves to nothing; `plant.go:124` and `steal.go:129` after creature, container (and for steal, household and fixture) lookups all failed, so the name may be a thing; `talk.go:51` when `err != nil` OR the target is a player | files |
| 17 | `internal/actions/{defuse,plant,shadow,steal}.go` prompts go through `actor.SendText`; a mob actor's `SendText` is a no-op | `actor_mob.go:43` |
| 18 | Four files this slice edits name a local `refusal`, which shadows the package: `ask.go:94`, `skill.skullduggery.shadow.go:54`, `talk.go:43`, `follow.go:432` (`searchName, refusal := actions.AimBySight(...)` or `name, refusal := ...`) | grep `refusal :=` |
| 19 | `speechWrapperHeard` returns each line tag-stripped and trimmed | `speech_sight_wrapper_test.go:22-32` |
| 20 | Command test fixture: `aimScene(t, aimFull)` returns `(*users.UserRecord, *rooms.Room)` for room 1 (Aliceia user 1, Bobrick user 2, Skeleton mob) with its own cleanup; `aimTold(1)` returns user 1's messages joined, tags stripped; `UserCommand` is the command func type | `sight_aim_commands_test.go:44-70`, `usercommands.go:42` |

## Wording (owner review: approve with the plan)

`Prompt(to, verb, ask, usage)` writes `<Verb> <ask>?` with the verb capitalized, then the hint. **Bold** is the command colour. Every line fits 80 columns (`TestPromptLinesFit80`).

### Prompts

| Call | Line | Replaces | Sites |
|---|---|---|---|
| `("get", What, "get <item>")` | Get what? Try **get &lt;item&gt;**. | Get what? | `get.go:115` |
| `("drop", What, "drop <item>")` | Drop what? Try **drop &lt;item&gt;**. | Drop what? | `drop.go:24` |
| `("give", What, "give <item> <name>")` | Give what? Try **give &lt;item&gt; &lt;name&gt;**. | Give what? To whom? (give {object-name} {receiver-name}) | `give.go:29` |
| `("show", What, "show <item> <name>")` | Show what? Try **show &lt;item&gt; &lt;name&gt;**. | Show what? To whom? | `show.go:26` |
| `("put", What, "put <item> in <container>")` | Put what? Try **put &lt;item&gt; in &lt;container&gt;**. | Place what where? | `put.go:23,69` |
| `("loot", What, "loot <corpse>")` | Loot what? Try **loot &lt;corpse&gt;**. | Loot what? | `loot.go:33` |
| `("assess", What, "assess <corpse>")` | Assess what? Try **assess &lt;corpse&gt;**. | Assess what? | `assess.go:24` |
| `("equip", What, "equip <item>")` | Equip what? Try **equip &lt;item&gt;**. | Wear WHAT? | `equip.go:62` |
| `("throw", What, "throw <item>")` | Throw what? Try **throw &lt;item&gt;**. | Throw what? Specify a throwable item. | `throw.go:188` |
| `("storage add", What, "storage add <item>")` | Storage add what? Try **storage add &lt;item&gt;**. | add what? | `storage.go:60,173` |
| `("lock", What, "lock <exit or container>")` | Lock what? Try **lock &lt;exit or container&gt;**. | Unlock what? (on `lock`) | `lock.go:20` |
| `("unlock", What, "unlock <exit or container>")` | Unlock what? Try **unlock &lt;exit or container&gt;**. | Unlock what? | `unlock.go:20` |
| `("defuse", What, "defuse <exit or container>")` | Defuse what? Try **defuse &lt;exit or container&gt;**. | Defuse what? Specify an exit or container. | `actions/defuse.go:88` |
| `("identify", What, "cast identify <item>")` | Identify what? Try **cast identify &lt;item&gt;**. | Identify what? (Usage: cast identify &lt;item&gt;) | `hooks/spell_resolution.go:854` |
| `("cast", What, "cast <spell> <name>")` | Cast what? Try **cast &lt;spell&gt; &lt;name&gt;**. | Cast what? (Usage: cast &lt;spell&gt; [target]), in red | `skill.cast.go:49` |
| `("consider", Whom, "consider <name>")` | Consider whom? Try **consider &lt;name&gt;**. | Consider whom? (try consider &lt;target&gt;) | `consider.go:21` |
| `("dismiss", Whom, "dismiss <name>")` | Dismiss whom? Try **dismiss &lt;name&gt;**. | Dismiss whom? (dismiss &lt;companion name&gt;) | `dismiss.go:55` |
| `("pet", What, "pet <name>")` | Pet what? Try **pet &lt;name&gt;**. | Pet what? | `pet.go:20` |
| `("talk to", Whom, "talk <name>")` | Talk to whom? Try **talk &lt;name&gt;**. | Talk to whom? | `talk.go:28,35` |
| `("invite", Whom, "party invite <name>")` | Invite whom? Try **party invite &lt;name&gt;**. | Invite who? | `party.go:179` |
| `("whisper to", Whom, "whisper <name> <message>")` | Whisper to whom? Try **whisper &lt;name&gt; &lt;message&gt;**. | Whisper to who? | `whisper.go:19` |
| `("reply with", What, "reply <message>")` | Reply with what? Try **reply &lt;message&gt;**. | Reply with what? | `reply.go:22` |
| `("say", What, "say <message>")` | Say what? Try **say &lt;message&gt;**. | Say what? | `say.go:26` |
| `("shout", What, "shout <message>")` | Shout what? Try **shout &lt;message&gt;**. | Shout what? | `shout.go:30` |
| `("follow", Whom, "follow <name>")` | Follow whom? Try **follow &lt;name&gt;**. | Follow whom? Try help command | `follow.go:394` |
| `("shadow", Whom, "shadow <name>")` | Shadow whom? Try **shadow &lt;name&gt;**. | Shadow whom? | `skill.skullduggery.shadow.go:49`, `actions/shadow.go:76` |
| `("steal from", Whom, "steal <name>")` | Steal from whom? Try **steal &lt;name&gt;**. | Steal from whom? | `skill.skullduggery.steal.go:44`, `actions/steal.go:122` |
| `("plant", What, "plant <item> <name>")` | Plant what? Try **plant &lt;item&gt; &lt;name&gt;**. | Plant what? | `skill.skullduggery.plant.go:32`, `actions/plant.go:79` |
| `("plant on", Whom, "plant <item> <name>")` | Plant on whom? Try **plant &lt;item&gt; &lt;name&gt;**. | Plant on whom? | `skill.skullduggery.plant.go:37,89`, `actions/plant.go:73` |
| `(rest, HowMuch, "bank "+rest+" <amount>")` | Deposit how much? Try **bank deposit &lt;amount&gt;**. (and Withdraw) | deposit how much? Make sure to include the amount of gold or "all". | `bank.go:32` |
| `("bid", HowMuch, "auction bid <amount>")` | Bid how much? Try **auction bid &lt;amount&gt;**. | Bid how much? | `auctions.go:318` |
| `("drop", HowMuchGold, "drop <amount> gold")` | Drop how much gold? Try **drop &lt;amount&gt; gold**. | unchanged text, now from the helper | `drop.go:80` |
| `("give", HowMuchGold, "give <amount> gold <name>")` | Give how much gold? Try **give &lt;amount&gt; gold &lt;name&gt;**. | unchanged text, now from the helper | `give.go:424` |
| melee: `(promptVerb, Whom, Verb+" <name>")` | Kick whom? Try **kick &lt;name&gt;**. (all 11 moves; pounce reads "Pounce on whom? Try **pounce &lt;name&gt;**.") | Kick whom? and 10 siblings | `actions/melee_target.go:169` |

### Misses that read as prompts

A name was typed and matched nothing, so these get the slice 1 not-here lines (sight-safe, no echo).

| Site | Today | Becomes |
|---|---|---|
| `ask.go:103` | ask who what? | `refusal.NotHereCreature`: You don't see them here. |
| `skill.skullduggery.shadow.go:72` | Shadow whom? | `refusal.NotHereCreature` |
| `follow.go:554` | Follow whom? | `refusal.NotHereCreature` |
| `look.go:543` | Look at what??? | `refusal.NotHereThing`: You don't see that here. (a look name may be anything) |
| `skill.skullduggery.plant.go:124` | Plant on whom? | `refusal.NotHereThing` (the name may be a container) |
| `skill.skullduggery.steal.go:129` | Steal from whom? | `refusal.NotHereThing` (the name may be a container or a household find) |
| `talk.go:51`, `err != nil` | Talk to whom? | `refusal.NotHereCreature` |
| `talk.go:51`, target is a player | Talk to whom? | new `refusal.TalkToPlayer`: To speak with a player, try **say &lt;message&gt;**. |

## File structure

| File | Responsibility |
|---|---|
| Create `internal/refusal/prompt.go` | `Ask`, `What`, `Whom`, `HowMuch`, `HowMuchGold`, `PromptText`, `Prompt`, `TalkToPlayerLine`, `TalkToPlayer` |
| Create `internal/refusal/prompt_test.go` | wording, category, width |
| Modify `internal/actions/melee_target.go`, `melee_target_test.go`, `internal/usercommands/pounce.go` | melee prompt through the helper |
| Modify the sites in the two Wording tables | call the helpers |
| Modify `refusal_phrase_guard_test.go` | the prompt row |
| Modify tests in fact 10 and 11 | pinned text |
| Modify `internal/refusal/context.md`, `internal/actions/context.md` (if it names `PromptMsg`), `docs/PATCH_NOTES.md` | docs |

---

### Task 0: Worktree

- [ ] **Step 1:** After the PR carrying this plan merges:

```bash
cd "/c/Users/Calabe Davis/workspace/DOGMud" && git fetch -q origin
git worktree add -b feature/m7-slice2-prompts C:/tmp/dogmud-m7s2 origin/master
```

Record `C:/tmp/dogmud-m7s2` in the session handoff. All later paths are relative to it.

### Task 1: The `Prompt` helper

**Files:** Create `internal/refusal/prompt.go`, `internal/refusal/prompt_test.go`.

- [ ] **Step 1: Write the failing test** `internal/refusal/prompt_test.go`:

```go
package refusal

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/assert"
)

func TestPromptWording(t *testing.T) {
	f := &fakeRecipient{}
	Prompt(f, "pet", What, "pet <name>")
	assert.Equal(t, `Pet what? Try <ansi fg="command">pet <name></ansi>.`, one(t, f, messaging.CategoryRefusal))

	assert.Equal(t, `Steal from whom? Try <ansi fg="command">steal <name></ansi>.`,
		PromptText("steal from", Whom, "steal <name>"))
	assert.Equal(t, `Deposit how much? Try <ansi fg="command">bank deposit <amount></ansi>.`,
		PromptText("deposit", HowMuch, "bank deposit <amount>"))
	assert.Equal(t, `Drop how much gold? Try <ansi fg="command">drop <amount> gold</ansi>.`,
		PromptText("drop", HowMuchGold, "drop <amount> gold"))
	assert.Equal(t, "Say what?", PromptText("say", What, ""), "no usage, no hint")
	assert.Equal(t, "Talk to whom?", PromptText("  talk to ", Whom, ""), "the verb is trimmed")
}

func TestTalkToPlayerWording(t *testing.T) {
	f := &fakeRecipient{}
	TalkToPlayer(f)
	assert.Equal(t, `To speak with a player, try <ansi fg="command">say <message></ansi>.`, one(t, f, messaging.CategoryRefusal))
}

// promptRows is every Prompt call this slice makes (the plan's Wording
// table), so a long verb or usage cannot push a line past 80 columns.
var promptRows = []struct {
	verb  string
	ask   Ask
	usage string
}{
	{"get", What, "get <item>"}, {"drop", What, "drop <item>"},
	{"give", What, "give <item> <name>"}, {"show", What, "show <item> <name>"},
	{"put", What, "put <item> in <container>"}, {"loot", What, "loot <corpse>"},
	{"assess", What, "assess <corpse>"}, {"equip", What, "equip <item>"},
	{"throw", What, "throw <item>"}, {"storage add", What, "storage add <item>"},
	{"lock", What, "lock <exit or container>"}, {"unlock", What, "unlock <exit or container>"},
	{"defuse", What, "defuse <exit or container>"}, {"identify", What, "cast identify <item>"},
	{"cast", What, "cast <spell> <name>"}, {"consider", Whom, "consider <name>"},
	{"dismiss", Whom, "dismiss <name>"}, {"pet", What, "pet <name>"},
	{"talk to", Whom, "talk <name>"}, {"invite", Whom, "party invite <name>"},
	{"whisper to", Whom, "whisper <name> <message>"}, {"reply with", What, "reply <message>"},
	{"say", What, "say <message>"}, {"shout", What, "shout <message>"},
	{"follow", Whom, "follow <name>"}, {"shadow", Whom, "shadow <name>"},
	{"steal from", Whom, "steal <name>"}, {"plant", What, "plant <item> <name>"},
	{"plant on", Whom, "plant <item> <name>"}, {"withdraw", HowMuch, "bank withdraw <amount>"},
	{"bid", HowMuch, "auction bid <amount>"}, {"drop", HowMuchGold, "drop <amount> gold"},
	{"give", HowMuchGold, "give <amount> gold <name>"}, {"throttle", Whom, "throttle <name>"},
	{"pounce on", Whom, "pounce <name>"}, {"grapple", Whom, "grapple <name>"},
}

func TestPromptLinesFit80(t *testing.T) {
	for _, r := range promptRows {
		fits80(t, PromptText(r.verb, r.ask, r.usage))
	}
	fits80(t, TalkToPlayerLine)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/refusal/ -run 'Prompt|TalkToPlayer'`
Expected: FAIL, `undefined: Prompt` (and `What`, `PromptText`, `TalkToPlayer`).

- [ ] **Step 3: Write `internal/refusal/prompt.go`**

```go
package refusal

import "strings"

// Ask is the question a prompt ends on. The set is fixed so no caller writes
// a question literal (the phrase guard's "prompt" row holds that).
type Ask string

const (
	What        Ask = `what`
	Whom        Ask = `whom`
	HowMuch     Ask = `how much`
	HowMuchGold Ask = `how much gold`
)

// PromptText answers a command typed without what it needs:
// "<Verb> <ask>?" with the verb capitalized, then a hint naming usage when
// usage is not empty. verb carries any preposition ("talk to", "steal
// from") and must not be empty.
func PromptText(verb string, ask Ask, usage string) string {
	v := strings.TrimSpace(verb)
	line := strings.ToUpper(v[:1]) + v[1:] + ` ` + string(ask) + `?`
	if usage != `` {
		line += hint(usage)
	}
	return line
}

func Prompt(to Recipient, verb string, ask Ask, usage string) {
	send(to, PromptText(verb, ask, usage))
}

// TalkToPlayerLine answers `talk` aimed at a player: talk is for creatures
// with something to say, and players hear say.
const TalkToPlayerLine = `To speak with a player, try <ansi fg="command">say <message></ansi>.`

func TalkToPlayer(to Recipient) { send(to, TalkToPlayerLine) }
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/refusal/`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/refusal/prompt.go internal/refusal/prompt_test.go
git commit -m "feat(refusal): Prompt helper with a fixed question set (#371)"
```

(Commit messages go through a quoted heredoc ending with the `Co-Authored-By` line.)

### Task 2: Special-move prompts

**Files:** Modify `internal/actions/melee_target.go:30-33,53-61,169`, `internal/actions/melee_target_test.go`, `internal/usercommands/pounce.go:25`.

- [ ] **Step 1: Update the test first.** In `melee_target_test.go`, the pounce case's `PromptMsg: "Pounce on whom?"` (`:75`) becomes `PromptVerb: "pounce on"`. Every `prompt:` value becomes the helper's line, e.g. `prompt: "Kick whom? Try <ansi fg=\"command\">kick <name></ansi>."` and for pounce `prompt: "Pounce on whom? Try <ansi fg=\"command\">pounce <name></ansi>."` (all 11 cases, lines 26-113). The assertion at `:122` becomes `assert.Equal(t, tt.prompt, tt.opts.promptText(), "empty-target prompt changed")`.
- [ ] **Step 2:** Run `go test ./internal/actions/ -run MeleeTarget`. Expected: FAIL to compile (`PromptVerb`, `promptText` undefined).
- [ ] **Step 3: Implement.** Replace the `PromptMsg` field and its comment with:

```go
	// PromptVerb is the verb the empty-target prompt asks with, preposition
	// included. Defaults to Verb. `pounce` uses "pounce on".
	PromptVerb string
```

Replace `promptMsg` with:

```go
// promptText is the empty-target prompt: "<PromptVerb> whom?" with a hint
// naming Verb, or "Attack whom?" for an empty Verb.
func (o MeleeTargetOpts) promptText() string {
	verb, usage := o.PromptVerb, o.Verb
	if usage == `` {
		usage = `attack`
	}
	if verb == `` {
		verb = usage
	}
	return refusal.PromptText(verb, refusal.Whom, usage+` <name>`)
}
```

At `:169`, `user.SendText(messaging.CategorySystem, opts.promptMsg())` becomes `user.SendText(messaging.CategoryRefusal, opts.promptText())`. `actions` already imports `internal/refusal` (`sight_aim.go:10`). In `pounce.go:25`, `PromptMsg: "Pounce on whom?",` becomes `PromptVerb: "pounce on",`.

- [ ] **Step 4:** `go build ./... && go test ./internal/actions/ ./internal/usercommands/`. Expected: `ok` for both.
- [ ] **Step 5: Commit** `internal/actions/melee_target.go internal/actions/melee_target_test.go internal/usercommands/pounce.go`: `refactor(refusal): special-move prompts through Prompt (#371)`.

### Task 3: Item and object prompts

**Files:** the sites in the Prompts table from `get` through `cast`, plus `drop.go:80` and `give.go:424`.

- [ ] **Step 1: Update pinned tests first.** `storage_remove_slot_test.go:34`: `assert.Contains(t, strings.ToLower(got), "add what?")` becomes `assert.Contains(t, got, refusal.PromptText("storage add", refusal.What, "storage add <item>"))` (import `internal/refusal`). In `special_move_categories_test.go:58` delete the `"Throw what?",` marker (the line leaves `CategorySystem`).
- [ ] **Step 2:** Run `go test ./internal/usercommands/ -run 'Storage|SpecialMove'`. Expected: the storage test FAILs on the new text.
- [ ] **Step 3: Convert.** At each site the `user.SendText(messaging.CategorySystem, "<old>")` (or `actor.SendText(...)`) becomes `refusal.Prompt(user, <args>)` (or `refusal.Prompt(actor, ...)`) with the args from the Wording table row. Exact forms:
  - `get.go:115` `refusal.Prompt(user, "get", refusal.What, "get <item>")`
  - `drop.go:24` `refusal.Prompt(user, "drop", refusal.What, "drop <item>")`
  - `drop.go:80` `refusal.Prompt(user, "drop", refusal.HowMuchGold, "drop <amount> gold")`
  - `give.go:29` `refusal.Prompt(user, "give", refusal.What, "give <item> <name>")`
  - `give.go:424` the const becomes `var giveHowMuchGoldLine = refusal.PromptText("give", refusal.HowMuchGold, "give <amount> gold <name>")`
  - `show.go:26` `refusal.Prompt(user, "show", refusal.What, "show <item> <name>")`
  - `put.go:23` and `put.go:69` `refusal.Prompt(user, "put", refusal.What, "put <item> in <container>")`
  - `loot.go:33` `refusal.Prompt(user, "loot", refusal.What, "loot <corpse>")`
  - `assess.go:24` `refusal.Prompt(user, "assess", refusal.What, "assess <corpse>")`
  - `equip.go:62` `refusal.Prompt(user, "equip", refusal.What, "equip <item>")`
  - `throw.go:188` `refusal.Prompt(user, "throw", refusal.What, "throw <item>")`
  - `storage.go:60` and `storage.go:173`: `refusal.Prompt(user, "storage add", refusal.What, "storage add <item>")` (the `term.CRLFStr` tail goes; `send` ends the line)
  - `lock.go:20` `refusal.Prompt(user, "lock", refusal.What, "lock <exit or container>")`
  - `unlock.go:20` `refusal.Prompt(user, "unlock", refusal.What, "unlock <exit or container>")`
  - `internal/actions/defuse.go:88` `refusal.Prompt(actor, "defuse", refusal.What, "defuse <exit or container>")`
  - `internal/hooks/spell_resolution.go:854` `refusal.Prompt(user, "identify", refusal.What, "cast identify <item>")`
  - `skill.cast.go:49` `refusal.Prompt(user, "cast", refusal.What, "cast <spell> <name>")`

  Add the `internal/refusal` import where a file lacks it; drop `term` or `messaging` imports the edit leaves unused (`go build` names them).
- [ ] **Step 4:** `gofmt -l internal/ modules/` prints nothing; `go build ./... && go test ./internal/usercommands/ ./internal/actions/ ./internal/hooks/`. Expected: `ok`.
- [ ] **Step 5: Commit** the touched files by name: `refactor(refusal): item and object prompts through Prompt (#371)`.

### Task 4: Creature, speech and amount prompts

**Files:** the remaining Prompts table rows.

- [ ] **Step 1: Update pinned tests first.** `speechWrapperHeard` strips tags (fact 19), so the tests pin the stripped line. `say_empty_test.go:20`: `[]string{"Say what?"}` becomes `[]string{"Say what? Try say <message>."}`; `shout_empty_test.go:20`: `[]string{"Shout what?"}` becomes `[]string{"Shout what? Try shout <message>."}`.
- [ ] **Step 1b: Rename the shadowing locals (fact 18).** In `skill.skullduggery.shadow.go:54-56` and `modules/follow/follow.go:432-434`, rename the local `refusal` to `aimRefusal` (both the `:=` and the `SendText` use). `go build ./...` must pass before Step 3.
- [ ] **Step 2:** Run `go test ./internal/usercommands/ -run 'Say_Empty|Shout_Empty'`. Expected: FAIL on the new text.
- [ ] **Step 3: Convert.**
  - `consider.go:21` `refusal.Prompt(user, "consider", refusal.Whom, "consider <name>")`
  - `dismiss.go:55` `refusal.Prompt(user, "dismiss", refusal.Whom, "dismiss <name>")`
  - `pet.go:20` `refusal.Prompt(user, "pet", refusal.What, "pet <name>")`
  - `talk.go:28` and `talk.go:35` `refusal.Prompt(user, "talk to", refusal.Whom, "talk <name>")`
  - `party.go:179` `refusal.Prompt(user, "invite", refusal.Whom, "party invite <name>")`
  - `whisper.go:19` `refusal.Prompt(user, "whisper to", refusal.Whom, "whisper <name> <message>")`
  - `reply.go:22` `refusal.Prompt(user, "reply with", refusal.What, "reply <message>")`
  - `say.go:26` `refusal.Prompt(user, "say", refusal.What, "say <message>")`
  - `shout.go:30` `refusal.Prompt(user, "shout", refusal.What, "shout <message>")`
  - `modules/follow/follow.go:394` `refusal.Prompt(user, "follow", refusal.Whom, "follow <name>")`
  - `skill.skullduggery.shadow.go:49` and `internal/actions/shadow.go:76` (`actor`) `refusal.Prompt(.., "shadow", refusal.Whom, "shadow <name>")`
  - `skill.skullduggery.steal.go:44` and `internal/actions/steal.go:122` (`actor`) `refusal.Prompt(.., "steal from", refusal.Whom, "steal <name>")`
  - `skill.skullduggery.plant.go:32` and `internal/actions/plant.go:79` (`actor`) `refusal.Prompt(.., "plant", refusal.What, "plant <item> <name>")`
  - `skill.skullduggery.plant.go:37`, `:89` and `internal/actions/plant.go:73` (`actor`) `refusal.Prompt(.., "plant on", refusal.Whom, "plant <item> <name>")`
  - `bank.go:32` `refusal.Prompt(user, rest, refusal.HowMuch, "bank "+rest+" <amount>")` (`rest` is exactly `deposit` or `withdraw` here)
  - `modules/auctions/auctions.go:318` `refusal.Prompt(user, "bid", refusal.HowMuch, "auction bid <amount>")`
- [ ] **Step 4:** `gofmt -l internal/ modules/` prints nothing; `go build ./... && go test ./internal/usercommands/ ./internal/actions/ ./modules/...`. Expected: `ok`.
- [ ] **Step 5: Commit** by name: `refactor(refusal): creature, speech and amount prompts through Prompt (#371)`.

### Task 5: Misses

**Files:** the "Misses that read as prompts" table; `shapes_roster_test.go:64`; a new `internal/usercommands/prompt_miss_test.go`.

- [ ] **Step 1: Write the failing test** `internal/usercommands/prompt_miss_test.go`, on the slice 1 fixture (`aimScene(t, aimFull)`, `aimTold(1)`; `sight_aim_commands_test.go:36-70`):

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// M7 slice 2: a typed name that matches nothing reads the slice 1
// not-here line, never a question back at the player.
func TestPromptMisses_ReadNotHere(t *testing.T) {
	cases := []struct {
		name string
		cmd  UserCommand
		rest string
		want string
	}{
		{"ask", Ask, "zzyzx to sit", refusal.NotHereCreatureLine},
		{"talk", Talk, "zzyzx", refusal.NotHereCreatureLine},
		{"look", Look, "zzyzx", refusal.NotHereThingLine},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			user, room := aimScene(t, aimFull)
			handled, err := c.cmd(c.rest, user, room, 0)
			require.NoError(t, err)
			require.True(t, handled)
			out := aimTold(1)
			assert.Contains(t, out, c.want)
			assert.NotContains(t, out, "zzyzx", "no echo")
			assert.NotContains(t, out, "?", "a miss is not a question")
		})
	}
}

// Bobrick (user 2) is a player in the fixture room.
func TestTalkToAPlayerSaysUseSay(t *testing.T) {
	user, room := aimScene(t, aimFull)
	handled, err := Talk("bobrick", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)
	assert.Contains(t, aimTold(1), "To speak with a player, try say <message>.")
}
```

`aimScene(t, band)` returns `(*users.UserRecord, *rooms.Room)` and registers its own cleanup; `aimTold(1)` returns everything user 1 was sent, tags stripped (`sight_aim_commands_test.go:44-70`). `Ask`, `Talk`, `Look` are the registered functions (`usercommands.go:59,146,227`).

- [ ] **Step 2:** Run `go test ./internal/usercommands/ -run 'PromptMisses|TalkToAPlayer'`. Expected: FAIL (`ask who what?`, `Talk to whom?`, `Look at what???`).
- [ ] **Step 3: Convert.** First rename the shadowing local `refusal` to `aimRefusal` in `ask.go:94-96` and `talk.go:43-45` (fact 18; Task 4 did shadow and follow).
  - `ask.go:103`, `skill.skullduggery.shadow.go:72`, `modules/follow/follow.go:554`: `refusal.NotHereCreature(user)`
  - `look.go:543`, `skill.skullduggery.plant.go:124`, `skill.skullduggery.steal.go:129`: `refusal.NotHereThing(user)`
  - `talk.go:50-53` becomes:

```go
	target, err := actions.ResolveTargetActor(room, searchName, actions.ResolveTargetOptions{Viewer: user.Character})
	if err != nil {
		refusal.NotHereCreature(user)
		return true, nil
	}
	if target.IsPlayer() {
		refusal.TalkToPlayer(user)
		return true, nil
	}
```

  - `shapes_roster_test.go:64`: `require.NotContains(t, out, "Look at what???")` becomes `require.NotContains(t, out, refusal.NotHereThingLine)` (the test's point is that a shape resolves; the miss line is what it must not see now).
- [ ] **Step 4:** `go build ./... && go test ./internal/usercommands/ ./modules/...`. Expected: `ok`.
- [ ] **Step 5: Commit** by name: `fix(refusal): misses that asked a question read the not-here lines (#371)`.

### Task 6: The prompt guard row

**Files:** Modify `refusal_phrase_guard_test.go`.

- [ ] **Step 1:** Add the row after `"not here, who"`:

```go
	{"prompt", regexp.MustCompile(`(?i)^\S+( (on|from|to|at|with))? (what|whom|who|how much)\?`), "Pet what?"},
```

and change the table comment's last line "Later slices add rows." to "Later slices add rows. The prompt row matches a literal that opens with a question; interactive `cmdPrompt.Ask` questions (\"Send mail to whom?\") do not, and must stay that way."

- [ ] **Step 2: Probe.** Temporarily put `user.SendText(messaging.CategorySystem, "Pet what?")` back at `pet.go:20`; run `go test -run TestRefusalPhrase .`; expect FAIL naming `internal/usercommands/pet.go`; revert that line.
- [ ] **Step 3:** `go test -count=1 -run TestRefusalPhrase .` Expected: `ok` (zero hits after Tasks 2 to 5; a hit names a missed site: convert it and add it to the Wording table).
- [ ] **Step 4: Commit** `refusal_phrase_guard_test.go`: `test(refusal): guard row for hand-written prompts (#371)`.

### Task 7: Docs

- [ ] **Step 1:** `internal/refusal/context.md`: add a Prompts section (the `Ask` set, `PromptText`/`Prompt`, verb carries the preposition, usage hint optional, `TalkToPlayer`), add the Prompts rows to the helper table, add the guard's prompt row, and move "Prompt" out of the "left for later slices" list. Check `internal/actions/context.md` for `PromptMsg` and rename it to `PromptVerb` there. Verify every symbol with `Select-String -Path internal\refusal\*.go -Pattern '^(func|type|const|var)\s'`, then `python tools/context_md_audit.py` reports nothing for `internal/refusal` and `internal/actions`.
- [ ] **Step 2:** `docs/PATCH_NOTES.md`: a dated entry, player-facing, no numbers, 80 columns: "A command typed without what it needs now asks the same way everywhere and shows how to finish it, for example **pet &lt;name&gt;**. `lock` no longer asks what to unlock. Asking or talking to someone who isn't there now says you don't see them; `talk` aimed at a player points you to `say`."
- [ ] **Step 3: Commit** by name: `docs: M7 slice 2 prompts (#371)`.

### Task 8: Gate, boot, PR

- [ ] **Step 1:** The `dogmud-shipping` gate in order: `gofmt -l internal/ modules/` prints nothing; `go build ./...`; `go test ./internal/refusal/ ./internal/actions/ ./internal/usercommands/ ./internal/hooks/ ./modules/... -count=1`; `go test -count=1 .`; `golangci-lint run --new-from-merge-base=origin/master` reports `0 issues`.
- [ ] **Step 2:** Boot check per `dogmud-shipping` (detached worktree, private ports, `Server Ready` 1, panics 0).
- [ ] **Step 3:** Push and open the PR with `--repo pruuk/DOGMud`, body naming slice 2 of #371 (no closing keyword), the Wording tables and the test evidence. Final whole-branch review before merge.

## Self-review notes

- Spec coverage for slice 2: `Prompt` with usage hints (Tasks 1 to 4), the guard's "what? as a whole prompt" row (Task 6). The spec's 35-prompt count was on `9c0ca1a57`; current master has 44 matching literals plus the melee default (fact 5, fact 6), all covered.
- Departure from the spec's signature `Prompt(u, verb, usage)`: an `Ask` argument is added, because the questions differ ("whom", "how much gold") and a free-text question would put a literal at every call site where the guard cannot tell it from a hand-written prompt.
- Seven sites the inventory found are misses, not prompts (fact 16); they take the slice 1 not-here lines, which keeps the #454 rule (no echo, unseen equals missing).
- Two copy bugs fixed on the way: `lock` asked "Unlock what?" (fact 12), and `equip` shouted "Wear WHAT?" (fact 13).
- Left for slices 3 and 4: `WhileBusy`, `TargetAsleep`, `Cost`, `NotWearing`, the remaining refusals package by package, `CategoryUsage`, `CategoryAdmin`, the tag guard.
