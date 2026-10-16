# Messaging M7 slice 3: the remaining refusals Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Every subagent brief carries these rules:** edit files with the Edit and Write tools only (no `sed -i`, no Python `open(path, 'w')`, no `>` into a tracked file; `~/.claude/hooks/guard.py` blocks them); `git add` named paths only; never `--no-verify`; no em or en dashes in prose or player text; every `gh` command carries `--repo pruuk/DOGMud`. Line numbers below are as of master `1abb49f12`: once a file has been edited, find a site by its text, not its number.

**Goal:** Every refusal a player can read rides `messaging.CategoryRefusal` (failures ride `CategoryError`), repeated situations read one way from one helper, the lines that broke a copy rule are reworded, and a guard stops a refusal-shaped line from landing on `CategorySystem` again.

**Architecture:** The ledger `docs/superpowers/audits/messaging-m7-refusal-ledger.md` rules on all 1325 non-admin `CategorySystem` sends (five read-only agents read every site; the controller reconciled and amended). Slice 3 touches the 520 rows ruled FAM, RETAG, REWORD or FAIL. `internal/refusal` gains ten helpers and an exported `Echo`; the busy, cost and asleep texts move into it. Then five file groups are converted in turn, each first widening a new source guard (a literal opening like a refusal, sent on `CategorySystem`, fails) so the group's task starts red.

**Tech Stack:** Go, testify, `go/ast` root guards (`package main` at the repo root), Python 3 for the read-only classifier.

**Spec:** `docs/superpowers/specs/2026-10-14-messaging-m7-refusals-design.md`, "Suggested slicing" item 3, sections 1, 2, 4, 6 and 7. Slices 1 and 2 shipped as #547 and #549. The spec's Testing section puts a playtest at the end of this slice (Task 13).

---

## Facts verified against source (master `1abb49f12`, 2026-10-16)

| # | Fact | Source |
|---|---|---|
| 1 | 1325 non-comment `messaging.CategorySystem` references in non-test Go files under `internal/` and `modules/`, admin files and `internal/refusal/` excluded; the ledger has 1325 rows with identical file:line keys | scratch `refusal_classify.py --tsv` (Task 1 lands it), `diff` of the key lists |
| 2 | Ledger counts: NOT 695, RETAG 353, FAM 102, REWORD 51, FAIL 14, NARR 63, USAGE 47; slice 3 touches FAM + RETAG + REWORD + FAIL = 520 | ledger header table |
| 3 | Per group (FAM/RETAG/REWORD/FAIL): G1 non-usercommands 23/58/5/1; G2 `usercommands/[a-f]*` 13/59/6/4; G3 `[g-o]*` 19/109/16/2; G4 `[p-r]*` 13/31/12/1; G5 `[s-w]*` 34/96/12/6 (G5 FAM includes the `usercommands.go:430` amendment) | `awk` over the ledger |
| 4 | `busyRefusalText(verb)` returns `<ansi fg="red">You can't %s while focused on your work. Finish or be interrupted first.</ansi>`; `refuseWhileBusy` sends it on `CategorySystem` when `user.Character.IsActing()` | `internal/usercommands/busy_refuse.go:12,24` |
| 5 | `cancel` ends any activity in progress: casting, crafting, salvaging | `internal/usercommands/cancel.go` (doc comment and `switch a.State()`) |
| 6 | `costRefusalText(result characters.CostCommitResult)` returns "" unless `Status == CostRefused`, else "You are too spent to manage that right now." (stamina) or "You cannot muster the resolve for that right now." (conviction); `CostRefusalText` exports it; 17 non-test callers | `internal/actions/action_cost.go:25-44`, grep |
| 7 | `CostCommitResult{Status, Pool, Charged}` lives in `internal/characters/pools.go:72`; `internal/characters` does not depend on `internal/refusal`, `actions` or `usercommands`, so `refusal` may import `characters` | `go list -deps ./internal/characters` |
| 8 | `RefuseIfAsleep(c, name, user)` builds `<ansi fg="mobname">%s</ansi> is fast asleep. You could make some noise. Try <ansi fg="command">shout wake up</ansi>.`, hides the name with `messaging.HideNames` at `ParticipantSight`, sends on `CategorySystem` | `internal/actions/sleeping_target.go:44-62` |
| 9 | `refusal.hint(c)` returns `` ` Try <ansi fg="command">`+c+`</ansi>.` ``; `echo(typed)` strips tags, trims, truncates to 24 display columns plus `...`, escapes leftovers; neither is exported | `internal/refusal/refusal.go` |
| 10 | `ValidateActorName` returns lowercase fragments: "name must be between %d and %d characters long", `NameRejectReason` (config), `that name matched the prohibited pattern: %q` (leaks the moderation pattern), "that name is in use", "that name is already in use", "that name is in use by a companion" | `internal/users/validate_actor_name.go:24-71` |
| 11 | Its errors reach players at `pet.go:39` and `renameself.go:46` (prefixed "That name won't work: "), `companion.go:122` (via `validateCompanionName`, `companion.go:294`) and at signup (`inputhandlers/login_prompt_handler.go:100`, shown as `err.Error()` at `:301`) | grep `ValidateActorName(`, `ValidateName(` |
| 12 | `validate_actor_name_test.go:82` table-tests `ValidateActorName` with `errSubstr: "between"`; banned names are set in tests with `configs.AddOverlayOverrides(map[string]any{"Validation.BannedNames": []string{...}})` | `validate_actor_name_test.go`, `playtestprofiles/credentials_test.go:48` |
| 13 | The phrase guard table is `refusalPhrases` (rows `{family, re, probe}`) in root `refusal_phrase_guard_test.go:27-62`; `TestRefusalPhraseGuardCanFail` needs every probe to hit | file |
| 14 | Root guard `aim_refusal_category_guard_test.go` is `package main` and defines `calledName(call *ast.CallExpr) string` | file |
| 15 | `special_move_categories_test.go:56-60` `systemNoticeMarkers` lets taunt, throw, rally and warcry send `CostRefusalText`, "focused on your work", "need a moment", "no target", "already rallied", "still echoes", "to throw.", "isn't something you can throw", "crumbles apart", `ChannelDefenceShortageText`, "text)", "cannot make a sound" on `CategorySystem`; it scans only lines containing `messaging.CategorySystem` | file |
| 16 | `trio.go:12-14` and `messaging.go:113-116` comments say the remaining refusals (special move busy and shield lines, craft, cost notices) stay on `CategorySystem` until later slices | files |
| 17 | Tests pinning text this slice changes: "focused on your work" in `actions/melee_target_test.go`, `usercommands/busy_refuse_test.go`, `special_move_categories_test.go`; "too spent"/"muster the resolve" in `actions/action_cost_test.go`, `usercommands/u8_voluntary_refusal_output_test.go`; "CURSED" in `remove_curse_test.go`; "attached to a party" in `party_answer_test.go`; "is fast asleep" in `sight_aim_commands_test.go`; `CostRefusalText`/`busyRefusalText` in `action_cost_test.go`, `command_readiness_drift_test.go`, `sneak_test.go`, `special_move_categories_test.go` | grep of `_test.go` |
| 18 | One admin-file refusal opens like a refusal on `CategorySystem`: `admin.zap.go:60` "You are not in combat." (the ledger skips admin files; the Task 5 guard does not) | grep |
| 19 | A narrow opener `^(You (can't\|cannot\|don't have\|aren't\|are not\|need to\|must\|'re too\|are too\|'re already\|are already\|have no)\|Only (the\|officers) )` over the ledger's "today" column hits 192 converted rows and 3 NOT rows: `bank.go:60,82` (partial deposits, both hold ", but ") and `guild.go:671` ("Only you may now ...", which `Only (the\|officers) ` does not match) | scratch run over the ledger |
| 20 | `character.go:400` passes `charValue` for both gold amounts in "You only have %d gold and it would cost %d gold to hire %s." | file |
| 21 | `drop.go:171` sends `Todo. Grenades disabled for now.` to the player after a grenade drop | file |
| 22 | `help.go:79-82` sends `err.Error()` as the help text when `templates.Process("help/help", ...)` fails | file |
| 23 | `mutations.go:29` prints the internal mutation id with `[data missing]` when `mutations.GetMutation` is nil | file |
| 24 | `auctions.Bid` returns player sentences plus `errors.New("User not found.")` (a server fault); the caller sends `err.Error()` | `modules/auctions/auctions.go:344-345,1051-1078` |
| 25 | `GameBridge.TeachSpell` prints the spell id, not its name; `spells.GetSpell(spellId) *SpellData` exists | `internal/questengine/bridge.go:290`, `internal/spells/spells.go:129` |
| 26 | `usercommands.go:430` is the casting gate for every command but cancel and flee: "You are holding %d/%d folds. Type cancel to stop." | file |
| 27 | Command test fixture: `aimScene(t, band)` (room 1: Aliceia user 1, Bobrick user 2, Skeleton mob 100), `aimTold(1)`, bands `aimFull`, `aimShapes`, `aimDark`; `refusalEchoRun` drives one command per scene | `sight_aim_commands_test.go:28-70`, `refusal_echo_sight_test.go:23-36` |
| 28 | `party invite` resolves the invitee through `AimBySight`, then `ResolveTargetActor`; on that error it sends `%s not found.` with the raw typed `rest` | `party.go:194-204` |

## Wording (owner review: approve with the plan)

Only lines whose words change are listed. RETAG rows (353) keep their words; only the category changes, and `CategoryRefusal` looks and wraps exactly like `CategorySystem` (same alias colour 8, same stages, no wrap), so a player sees no difference. Every helper line fits 80 columns (pinned by tests).

### A. New helpers

| Helper | Line | Replaces |
|---|---|---|
| `WhileBusy(to, doing)` | You can't change equipment while you're busy. Try **cancel**. | "You can't X while focused on your work. Finish or be interrupted first." (in red), "You are already working on something. ...", "You're already busy working on something.", "You are too busy to cast right now.", "You're already busy with something else.", "You are busy with something else.", and the casting gate "You are holding 2/5 folds. Type cancel to stop." |
| `WhileUnderAttack(to, doing)` | You can't steal while under attack. | "You can't do that while you are under attack!" |
| `TargetAsleepText(name)` | **Mira** is fast asleep. You could make some noise. Try **shout wake up**. | unchanged words, moved into the package |
| `Cost(to, result)` | You are too spent to manage that right now. / You cannot muster the resolve for that right now. | unchanged words, moved from `actions` |
| `NotWearing(to, typed)` | You aren't wearing 'ring'. Try **inventory**. | You don't appear to be using a "ring". |
| `NotInStorage(to, typed)` | You don't have 'rope' in storage. Try **storage**. (empty: 'that') | You don't have a rope in storage. / You don't have that item in storage. |
| `NotInParty(to)` | You aren't in a party. Try **party create**. | You are not attached to a party. / You are not in a party. |
| `NotGuildLeader(to, action)` | Only the guild leader can disband the guild. | five "Only the (guild) leader can ..." lines |
| `NotGuildOfficer(to, action)` | Only guild officers and the leader can invite. | three "Only officers and the leader can ..." lines |
| `CompanionsFull(to)` | You are already sustaining as many companions as your will can hold. | that line, and "You cannot maintain any more companions." |
| `NotPartyMember(to, typed)` | No one in your party is called 'Bob'. | `%s not found.` (party kick, promote) |
| `Echo(typed)` (exported) | the slice 1 echo, for reworded lines that repeat what the player typed | raw `%s` of typed input |

### B. Sites moving to a helper (FAM, 102)

| Helper call | Today | Sites |
|---|---|---|
| `WhileBusy(_, verb)` | the busy lines in A | `busy_refuse.go:26` (all `refuseWhileBusy` callers), `actions/melee_target.go:164`, `actions/drink.go:180` ("drink"), `actions/mutation_helpers.go:73` ("use that ability"), `bash.go:45`, `grapple.go:40`, `kick.go:39`, `rally.go:27`, `taunt.go:37`, `trip.go:44` ("trip someone"), `warcry.go:27` ("muster a warcry"), `shoot.go:114`, `remove.go:32,55` ("change equipment"), `craft.go:145,321,416` ("craft"), `salvage.go:32,124,184`, `skill.cast.go:113,305` ("cast"), `skill.skullduggery.sneak.go:33`, `usercommands.go:430` ("do that") |
| `Cost(_, res.Cost)` | the cost lines | `bash.go:39`, `drain.go:33`, `gore.go:33`, `grapple.go:34`, `kick.go:33`, `maul.go:33`, `pounce.go:36`, `rake.go:33`, `rally.go:22`, `shoot.go:108`, `skill.skullduggery.sneak.go:51`, `taunt.go:31`, `throttle.go:45`, `throw.go:246`, `trip.go:38`, `warcry.go:22` |
| `Cooldown(_, doing)` | "You need to wait %s more rounds ..." (a raw count), "You can only create 1 map per round." | `actions/forage.go:66` ("forage again"), `actions/search.go:190` ("search again"), `actions/track.go:206,229` ("track again"), `assess.go:40` ("assess again"), `skill.map.go:117` ("draw another map"), `skill.skullduggery.shadow.go:88` ("shadow anyone"), `skill.skullduggery.sneak.go:43` ("sneak") |
| `WhileFighting(_, doing)` | "You can't sleep right now.", "You can't do that! You are in combat!", "You're too busy to quit right now!" | `actions/sleep.go:46` ("sleep"), `go.go:71` ("move"), `quit.go:13` ("quit") |
| `WhileUnderAttack(_, doing)` | "You can't do that while you are under attack!" | `actions/defuse.go:82` ("defuse a trap"), `actions/plant.go:66` ("plant anything"), `actions/steal.go:115` ("steal") |
| `NotHereCreature(_)` | "They seem to have vanished.", `%s not found.`, "Could not find your target." | `actions/plant.go:124,270`, `actions/shadow.go:104,150`, `actions/steal.go:221,440`, `party.go:203`, `shoot.go:143` |
| `NotHereThing(_)` | "There is no such exit or container.", `You don't see a %s in the %s.`, "No container found by that name" | `actions/defuse.go:245`, `unlock.go:149`, `get.go:542,801`, `put.go:55` |
| `NotCarrying(_, typed)` | "You can't seem to identify that.", "Could not find item: %s", `You don't see a "%s" in your %s.` | `hooks/spell_resolution.go:863`, `mail.go:175`, `get.go:356` |
| `NoTarget(_, verb)` | "There is nothing here to attack.", "You need a target to cast that spell." | `attack.go:124` ("attack"), `skill.cast.go:252` ("cast "+spellId) |
| `NoCompanion(_)` | "You have no companions.", "You have no companions to dismiss." | `companion.go:31`, `dismiss.go:61` |
| `Prompt(...)` | "You must ask someone something", "Oops!", "You wanna pock a lock? ...", "Specify what you want to sell. ...", "You can share gold by typing ...?", "You need to specify a message.", the bare usage lines of `salvage` and `disenchant` | `ask.go:89` ("ask", What, "ask <name> <topic>"), `drop.go:93` ("drop", HowMuchGold, "drop <amount> gold"), `picklock.go:46` ("pick", What, "picklock <exit or container>"), `sell.go:48,61,72` ("sell", What, "sell <item>"), `share.go:83` ("share", HowMuchGold, "share <amount> gold"), `whisper.go:26` ("whisper", What, "whisper <name> <message>"), `salvage.go:26` ("salvage", What, "salvage <item>"), `skill.disenchant.go:23` ("disenchant", What, "disenchant <item>") |
| `TargetAsleepText` | (in `RefuseIfAsleep`) | `actions/sleeping_target.go:60` |
| `NotWearing`, `NotInStorage`, `NotInParty`, `NotGuildLeader`, `NotGuildOfficer`, `CompanionsFull` | as in A | `remove.go:57`; `storage.go:255,269,284`; `party.go:61`, `report.go:38`; `guild.go:216,368,405,654,714`; `guild.go:268,334,748`; `hooks/charm_spell.go:74,121`, `hooks/companion_summon.go:130` |

### C. Rewords (REWORD, 51)

| Site | Today | Becomes | Why |
|---|---|---|---|
| `actions/steal.go:231` | **Mob** is someone's companion. You can't steal from them. | same, the name through `messaging.HideNames` at the thief's sight | a shape-aimed steal named the mob |
| `actions/steal.go:242` | You can't steal from **Mob**. | same, name hidden at the thief's sight | as above |
| `actions/steal.go:877,917` | You can't carry the **bauble** - you're already overloaded! | You can't carry that. You're already overloaded! | the household match has no sight check |
| `auctions.go:237` | ... See help set for learn how to change this. | Auctions are disabled. See **help set** to learn how to change this. | broken English |
| `character.go:45` | Alt character are disabled on this server. | Alt characters are disabled on this server. | grammar |
| `character.go:50` | You must gain at least 20 total skill ranks to access character alts. | You need more skill training before you can have alt characters. | raw number |
| `character.go:400` | You only have **X gold** and it would cost **X gold** to hire **Name**. | same words; the first amount is the player's own gold | bug |
| `companion.go:122`, `pet.go:39`, `renameself.go:46`, signup | fragments; the banned case prints the moderation pattern | Task 3 sentences: "Names must be between 2 and 32 characters long.", "That name is not allowed.", "That name is already in use." (`NameRejectReason` is config, unchanged) | leak, fragments |
| `craft.go:135` | No recipe found for "**typed**". Type **craft list** to see available recipes. | No recipe matches '**typed**'. Try **craft list**. (typed through `Echo`) | over 80, raw echo |
| `dismiss.go:86` | **Name** is not yours to dismiss. If you want to part ways, you will have to tell them. | **Name** is not yours to dismiss. To part ways, tell them. | over 80 |
| `get.go:413` | You can't do that! | You can only take things from your own pet. | unclear |
| `get.go:636` | The **item** belongs to this household. To take it anyway, **steal item**. | The **item** belongs to this household. Try **steal item**. | over 80 |
| `get.go:904` | You leave the **item**: it belongs to this household. To take it anyway, **steal item**. | You leave the **item**; it belongs to this household. Try **steal item**. | over 80 |
| `go.go:91` | You can't do that! | You can't move right now. | unclear |
| `go.go:179` | There's a lock preventing you from going that way. You'll need a Key or to pick the lock with lockpicks. | That way is locked. You need a **key**, or **pick** the lock with **lockpicks**. | 104 columns |
| `grapple.go:73` | You get both hands on **X** and heave. It might as well be a boulder. Nothing shifts. | You heave at **X** with both hands. Nothing shifts. | over 80, idiom |
| `guild.go:145` | Cannot found the guild: *err* | You can't found that guild: *err*. | raw error shape (the error is the player's, a tag or name rule) |
| `guild.go:309` | Cannot join: *err* | That guild no longer exists. | the only reachable error |
| `guild.go:558` | Only the leader can withdraw (unless the leader has delegated treasury access to officers). | Only the leader can withdraw, unless officers have treasury access. | 91 columns |
| `guild.go:572` | Cannot withdraw: *err* | The treasury doesn't hold that much gold. | the reachable error |
| `guild.go:601` | Cannot donate: *err* | The guild vault is full. | the reachable error |
| `guild.go:617` | Only the leader can take from the vault (unless ...). | Only the leader can take from the vault, unless officers have treasury access. | 102 columns |
| `guild.go:734` | Invalid title: *err* | You can't use that title: *err*. | raw error shape |
| `inbox.go:55` | Your pack is too full to receive the **item**. Free some space and check your mail again. | Your pack is too full for the **item**. Make room and check your mail again. | over 80 |
| `jail.go:66` | You can't cover the **N gold** fine, even counting your bank. You'll have to sit it out. | You can't pay the **N gold** fine, even with your bank. You must serve your time. | over 80, idiom |
| `list.go:113` | **Keeper** has closed up for the night and gone to bed. Come back in the morning, or make some noise nearby. | **Keeper** has gone to bed. Come back in the morning, or make some noise. | 100 columns |
| `party.go:157` | You already own a party Type **party list** for more info. | You already lead a party. Try **party list**. | run-on |
| `party.go:159` | You are already party of a party. | You are already in a party. | grammar |
| `party.go:624,651` | `%s not found.` (blank name prints " not found.") | `NotPartyMember(user, rest)`; an empty name gets `Prompt(user, "kick"/"promote", Whom, "party kick <name>"/"party promote <name>")` | unclear |
| `password.go:49` | *err* | Your new password must be between N and M characters long. (the bcrypt error goes to `Failed`) | raw error |
| `picklock.go:51` | The shipping crate has no lock to pick. It's sealed by the caravan's binding, not by mechanism. | The shipping crate has no lock. The caravan's binding seals it. | 95 columns |
| `put.go:36` | The shipping crate is sealed; the caravan only lets through what they put in themselves. | The shipping crate is sealed. Only the caravan can put things in it. | 88 columns |
| `remove.go:36,59` | You can't seem to remove your **item**... It's **CURSED!** | `user.Character.CursedRefusal(it)`: Your **item** is cursed and prevents you from removing it. | caps; reuse the existing line |
| `renameself.go:77` | Rename failed: name was just claimed | Someone else just took that name. | Go error text |
| `salvage.go:42` | You don't have "**typed**" and there's no corpse of that name here. | You aren't carrying that, and you don't see a corpse like that here. | sight echo |
| `set.go:594` | There was a problem setting your macro. | Your macro has no commands in it. | reads as a fault; the cause is the input |
| `shoot.go:131` | Your **bow** is empty. Check that you still have ammunition, and the breath to ready it. | Your **bow** is empty. You may be out of ammunition, or too winded. | over 80 |
| `skill.cast.go:65` | No spell found for "**typed**". Use the spell name or an alias (e.g. mm, heal). Type spells to list what you know. | There is no spell called '**typed**'. Try **spells**. (typed through `Echo`) | about 100 columns |
| `skill.cast.go:163` | **Spell** requires a **item #123** in your inventory. | the fallback name is "component", never the id | internal id |
| `skill.cast.go:337` | You reach for the Veil, but there is no anchor to pull you. Set one first with **cast fold-anchor**. | You have no anchor to return to. Try **cast fold-anchor**. | 96 columns |
| `stand.go:82` | You're too winded to get up. Catch your breath first. Your gear is holding *band* of your stamina in reserve, so you have less to draw on than you might expect. | two lines: You're too winded to get up. Catch your breath first. / Your gear holds *band* of your stamina in reserve. | over 140 |
| `storage.go:33` | Maybe you meant to use the **put** command to **put** something into the **chest**? | Try **put &lt;item&gt; &lt;container&gt;**. | names a container never typed, hidden ones too |
| `storage.go:366` | The keepers will not take the **item**: it was reported stolen, and not long ago. | The keepers won't take the **item**: it was reported stolen. | over 80 |
| `talk.go:195` | You are carrying too much to take the **item**. Put something down, then ask again. | You can't carry the **item**. Drop something, then ask again. | over 80 |
| `use.go:36` | The **chest** is locked. | That is locked. | names a room container with no sight check |
| `use.go:45` | The **chest** seems to be missing something. | That seems to be missing something. | as above |

### D. Failures (FAIL, 14)

Each becomes `refusal.Failed(user, "<noun>", err, "userId", user.UserId)` (logs the cause and the calling line; the player reads "That <noun> didn't work. The error has been logged." on `CategoryError`). Where no `err` exists, the call builds one with `fmt.Errorf` naming the fault.

| Site | Today | Noun |
|---|---|---|
| `bug.go:31,38,53` | Could not save bug report. Please notify an admin. (error never logged) | bug report |
| `suggest.go:29,36,51` | Could not save suggestion. Please notify an admin. (never logged) | suggestion |
| `drop.go:105` | Oops! | drop |
| `go.go:252` | Oops, couldn't move there! | move |
| `guild.go:542` | Cannot deposit: *err* | deposit |
| `petition.go:62` | Your petition could not be filed. ... (never logged) | petition |
| `salvage.go:148` | Something is wrong with that corpse. | salvage (`fmt.Errorf("corpse mob %d has no spec", ...)`) |
| `skill.map.go:153,225` | No map found (or an error occured)" | map |
| `actions/sleep.go:65` | You can't seem to settle into sleep right now. | sleep |

### E. Found along the way (Task 11)

| Site | Today | Becomes |
|---|---|---|
| `drop.go:171` | the player reads "Todo. Grenades disabled for now." | the line is deleted |
| `help.go:79-82` | a template failure shows the Go error as the help page | `refusal.Failed(user, "help", err, ...)` and return |
| `mutations.go:29` | `<id> (depth) [data missing]` | `mudlog.Error` with the id; the row is skipped |
| `auctions.go:345` | `Bid`'s "User not found." reaches the player | `Bid` returns a sentinel `errBidderNotFound`; the caller sends `Failed(user, "bid", err, ...)` for it and the player sentences on `CategoryRefusal` |
| `bridge.go:290` | You have learned the **fold-anchor** spell. (the id) | the spell's `Name`, the id only when `GetSpell` is nil |

## File structure

| File | Responsibility |
|---|---|
| Create `tools/refusal_classify.py` | the read-only classifier (spec section 6) |
| Create `internal/refusal/busy.go`, `busy_test.go` | `WhileBusy`, `WhileUnderAttack`, `TargetAsleepText` |
| Create `internal/refusal/cost.go`, `cost_test.go` | `Cost`, `CostText` |
| Modify `internal/refusal/carrying.go`, `membership.go`, `refusal.go`, their tests | `NotWearing`, `NotInStorage`, `NotInParty`, `NotGuildLeader`, `NotGuildOfficer`, `CompanionsFull`, `NotPartyMember`, `Echo` |
| Modify `internal/users/validate_actor_name.go`, its test | sentences, no banned pattern |
| Create `refusal_system_opener_guard_test.go` (root) | the opener guard, widened per group |
| Modify `refusal_phrase_guard_test.go` | rows for the new families |
| Modify the ledger's FAM, RETAG, REWORD and FAIL sites | the conversion |
| Modify `internal/refusal/context.md`, `internal/messaging/context.md`, `internal/usercommands/context.md`, `internal/actions/context.md`, `docs/README.md`, the ledger | docs |

---

### Task 1: The classifier tool

**Files:** Create `tools/refusal_classify.py`; Modify `docs/README.md`.

- [ ] **Step 1: Write the tool.** Copy this file exactly:

```python
#!/usr/bin/env python3
"""List every CategorySystem send with a proposed class (messaging M7, #371).

Spec section 6: a read-only classifier that reads Go source and never writes
it. It PROPOSES; a human or agent rules. The slice 3 ledger
(docs/superpowers/audits/messaging-m7-refusal-ledger.md) is its output ruled
row by row. Classes:

  usage      the text holds "Usage:" or opens "Syntax:"
  prompt     a bare question back ("X what?")
  refusal    opens with a refusal phrase ("You can't", "There is no", "Only")
  narration  names a third party seeing something ("enters", "notices you")
  refusal?   holds a refusal word mid-line ("can't", "already", "failed")
  other      reports, tables, confirmations
  dynamic    no literal reaches the send; the expression column shows what does

Usage:
    python tools/refusal_classify.py                    # summary by file
    python tools/refusal_classify.py --tsv              # one row per send
    python tools/refusal_classify.py internal/actions   # one tree

Admin files (internal/usercommands/admin.*.go) and internal/refusal are
skipped: slice 4 retags admin output by file.
"""
import argparse
import os
import re
import sys
from collections import Counter, defaultdict

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

LIT = re.compile(r'`([^`]*)`|"((?:[^"\\]|\\.)*)"')
TAG = re.compile(r'<[^>]+>')

REFUSAL_OPEN = re.compile(
    r"^(?:You (?:can'?t|cannot|can not|don'?t|do not|aren'?t|are not|have no|"
    r"haven'?t|need|must|lack|'?re (?:not|too|already)|are (?:too|already)|"
    r"already|have nothing|may not|won'?t|'?d need|fail)|"
    r"There (?:is|are|'s) no|There isn'?t|Nothing |No such|No one|Nobody|No |None |"
    r"(?:That|This|It|They) (?:is|isn'?t|doesn'?t|can'?t|won'?t|aren'?t|are not|has no)|"
    r"Only |Not |Invalid|Unknown|Can'?t|Cannot|Sorry|"
    r"%s (?:is|isn'?t|can'?t|doesn'?t|won'?t|has no|does not|are not))",
    re.I)
REFUSAL_WORD = re.compile(
    r"\b(can'?t|cannot|isn'?t|aren'?t|doesn'?t|don'?t|not enough|too far|"
    r"already|must be|need to|unable|vanished|missing|failed)\b", re.I)
USAGE = re.compile(r"Usage:|^\s*Syntax:", re.I)
PROMPT = re.compile(r"^\S+( (on|from|to|at|with))? (what|whom|who|which|how much)\?", re.I)
NARRATION = re.compile(r"\b(enters|leaves|notices you|arrives|the room)\b", re.I)


def call_span(text, start):
    """Text from start to the paren that closes the call holding start."""
    depth, i, n = 0, start, len(text)
    while i < n:
        c = text[i]
        if c in '`"':
            q = c
            i += 1
            while i < n and text[i] != q:
                if q == '"' and text[i] == '\\':
                    i += 1
                i += 1
        elif c == '(':
            depth += 1
        elif c == ')':
            if depth == 0:
                return text[start:i]
            depth -= 1
        elif c == '\n' and depth == 0 and text[i - 1] not in ',(':
            return text[start:i]
        i += 1
    return text[start:]


def classify(lit):
    if not lit:
        return 'dynamic'
    plain = TAG.sub('', lit).strip()
    if USAGE.search(plain):
        return 'usage'
    if PROMPT.search(plain):
        return 'prompt'
    if REFUSAL_OPEN.search(plain):
        return 'refusal'
    if NARRATION.search(plain):
        return 'narration'
    if REFUSAL_WORD.search(plain):
        return 'refusal?'
    return 'other'


def scan(roots):
    rows = []
    for root in roots:
        for dp, _, fns in os.walk(os.path.join(REPO, root)):
            for fn in sorted(fns):
                if not fn.endswith('.go') or fn.endswith('_test.go'):
                    continue
                path = os.path.join(dp, fn)
                rel = os.path.relpath(path, REPO).replace('\\', '/')
                if rel.startswith('internal/usercommands/admin.') or rel.startswith('internal/refusal/'):
                    continue
                with open(path, encoding='utf-8') as f:
                    text = f.read()
                for m in re.finditer(r'messaging\.CategorySystem', text):
                    line_start = text.rfind('\n', 0, m.start()) + 1
                    if text[line_start:m.start()].lstrip().startswith('//'):
                        continue
                    lineno = text.count('\n', 0, m.start()) + 1
                    span = call_span(text, m.end())
                    lits = [a or b for a, b in LIT.findall(span)]
                    lits = [l for l in lits if TAG.sub('', l).strip()]
                    lit = max(lits, key=lambda l: len(TAG.sub('', l))) if lits else ''
                    expr = ' '.join(span.lstrip(', ').split())[:100]
                    rows.append((rel, lineno, classify(lit), lit, expr))
    return rows


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('roots', nargs='*', default=['internal', 'modules'])
    ap.add_argument('--tsv', action='store_true')
    a = ap.parse_args()
    sys.stdout.reconfigure(encoding='utf-8')
    rows = scan(a.roots)
    if a.tsv:
        clean = lambda s: s.replace('\t', ' ').replace('\n', '\\n')
        for rel, ln, cls, lit, expr in rows:
            print(f'{rel}\t{ln}\t{cls}\t{clean(lit)}\t{clean(expr)}')
        return 0
    print(f'{len(rows)} CategorySystem sends')
    for cls, n in Counter(r[2] for r in rows).most_common():
        print(f'  {cls:10s} {n}')
    byfile = defaultdict(Counter)
    for r in rows:
        byfile[r[0]][r[2]] += 1
    print()
    for fn, c in sorted(byfile.items(), key=lambda kv: (-sum(kv[1].values()), kv[0])):
        print(f'{sum(c.values()):4d}  {fn}  ' + ' '.join(f'{k}={v}' for k, v in sorted(c.items())))
    return 0


if __name__ == '__main__':
    sys.exit(main())
```

- [ ] **Step 2: Prove it reproduces the ledger's inventory.** Task 1 runs before any conversion, so run `python tools/refusal_classify.py | head -3` now. Expected: `1325 CategorySystem sends`, and `python tools/refusal_classify.py --tsv | wc -l` prints 1325.

- [ ] **Step 3: Register it.** In `docs/README.md`, next to the other `tools/` and ledger rows, add a row for `tools/refusal_classify.py` ("Read-only classifier for M7 (#371): lists every `CategorySystem` send outside admin files with a proposed class; the slice 3 ledger is its output ruled row by row") and confirm the ledger row the plan PR added is present.

- [ ] **Step 4: Commit.** `git add tools/refusal_classify.py docs/README.md` and commit `feat(tools): refusal classifier for M7 slice 3 (#371)`.

### Task 2: New helpers

**Files:** Create `internal/refusal/busy.go`, `internal/refusal/busy_test.go`, `internal/refusal/cost.go`, `internal/refusal/cost_test.go`; Modify `internal/refusal/carrying.go`, `internal/refusal/membership.go`, `internal/refusal/refusal.go`, `internal/refusal/refusal_test.go`.

- [ ] **Step 1: Write the failing tests.** `internal/refusal/busy_test.go`:

```go
package refusal

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/assert"
)

func TestWhileBusy(t *testing.T) {
	f := &fakeRecipient{}
	WhileBusy(f, "change equipment")
	assert.Equal(t, `You can't change equipment while you're busy. Try <ansi fg="command">cancel</ansi>.`,
		one(t, f, messaging.CategoryRefusal))
	for _, doing := range []string{"change equipment", "use that ability", "muster a warcry", "throw anything", "pick a lock", "do that"} {
		fits80(t, WhileBusyText(doing))
	}
}

func TestWhileUnderAttack(t *testing.T) {
	f := &fakeRecipient{}
	WhileUnderAttack(f, "steal")
	assert.Equal(t, "You can't steal while under attack.", one(t, f, messaging.CategoryRefusal))
}

func TestTargetAsleepText(t *testing.T) {
	assert.Equal(t, `<ansi fg="mobname">Mira</ansi> is fast asleep. You could make some noise. Try <ansi fg="command">shout wake up</ansi>.`,
		TargetAsleepText("Mira"))
	fits80(t, TargetAsleepText("Bartholomew the Innkeeper"))
}
```

`internal/refusal/cost_test.go`:

```go
package refusal

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/assert"
)

func TestCost(t *testing.T) {
	f := &fakeRecipient{}
	Cost(f, characters.CostCommitResult{Status: characters.CostRefused, Pool: characters.PoolStamina})
	assert.Equal(t, "You are too spent to manage that right now.", one(t, f, messaging.CategoryRefusal))
	assert.Equal(t, "You cannot muster the resolve for that right now.",
		CostText(characters.CostCommitResult{Status: characters.CostRefused, Pool: characters.PoolConviction}))

	f = &fakeRecipient{}
	Cost(f, characters.CostCommitResult{Status: characters.CostPaid, Pool: characters.PoolStamina})
	assert.Empty(t, f.got, "a cost that was paid sends nothing")
}
```

(`CostStatus` values are `CostNoCharge`, `CostPaid`, `CostPartiallyPaid`, `CostRefused`, `pools.go:55-58`.)

Append to `internal/refusal/refusal_test.go`:

```go
func TestNewMembershipAndCarrying(t *testing.T) {
	cases := []struct {
		send func(Recipient)
		want string
	}{
		{func(r Recipient) { NotWearing(r, " ring ") }, `You aren't wearing 'ring'. Try <ansi fg="command">inventory</ansi>.`},
		{func(r Recipient) { NotInStorage(r, "rope") }, `You don't have 'rope' in storage. Try <ansi fg="command">storage</ansi>.`},
		{func(r Recipient) { NotInStorage(r, "") }, `You don't have that in storage. Try <ansi fg="command">storage</ansi>.`},
		{func(r Recipient) { NotInParty(r) }, `You aren't in a party. Try <ansi fg="command">party create</ansi>.`},
		{func(r Recipient) { NotGuildLeader(r, "disband the guild") }, "Only the guild leader can disband the guild."},
		{func(r Recipient) { NotGuildOfficer(r, "invite") }, "Only guild officers and the leader can invite."},
		{func(r Recipient) { CompanionsFull(r) }, "You are already sustaining as many companions as your will can hold."},
		{func(r Recipient) { NotPartyMember(r, "Bob") }, "No one in your party is called 'Bob'."},
	}
	for _, c := range cases {
		f := &fakeRecipient{}
		c.send(f)
		assert.Equal(t, c.want, one(t, f, messaging.CategoryRefusal))
		fits80(t, c.want)
	}
	assert.Equal(t, "Only the guild leader can do that.", NotGuildLeaderText(""))
	assert.Equal(t, "Only guild officers and the leader can do that.", NotGuildOfficerText(""))
	fits80(t, NotWearingText(strings.Repeat("w", 60)))
	fits80(t, NotInStorageText(strings.Repeat("w", 60)))
	fits80(t, NotPartyMemberText(strings.Repeat("w", 60)))
}

func TestEchoIsExported(t *testing.T) {
	assert.Equal(t, echo(`<ansi fg="red">rope</ansi> `), Echo(`<ansi fg="red">rope</ansi> `))
}
```

- [ ] **Step 2: Run them to see them fail.** `go test ./internal/refusal/` Expected: build failure, `undefined: WhileBusy` and siblings.

- [ ] **Step 3: Implement.** `internal/refusal/busy.go`:

```go
package refusal

// WhileBusyText answers a command refused while the player is casting,
// crafting or salvaging. cancel ends any of them, so the hint names it.
func WhileBusyText(doing string) string {
	return `You can't ` + doing + ` while you're busy.` + hint(`cancel`)
}

func WhileBusy(to Recipient, doing string) { send(to, WhileBusyText(doing)) }

// WhileUnderAttackText answers an action refused while something attacks the
// player, before they are in combat with it (the in-combat gate is
// WhileFighting).
func WhileUnderAttackText(doing string) string {
	return `You can't ` + doing + ` while under attack.`
}

func WhileUnderAttack(to Recipient, doing string) { send(to, WhileUnderAttackText(doing)) }

// TargetAsleepText answers a trade or talk with a sleeper. It names the
// sleeper; actions.RefuseIfAsleep hides the name at the player's sight
// before sending, so this helper has no send of its own.
func TargetAsleepText(name string) string {
	return `<ansi fg="mobname">` + name + `</ansi> is fast asleep. You could make some noise.` + hint(`shout wake up`)
}
```

`internal/refusal/cost.go`:

```go
package refusal

import "github.com/GoMudEngine/GoMud/internal/characters"

// CostText is the refusal for a voluntary action whose pool could not pay;
// empty when the cost was not refused.
func CostText(result characters.CostCommitResult) string {
	if result.Status != characters.CostRefused {
		return ``
	}
	switch result.Pool {
	case characters.PoolStamina:
		return `You are too spent to manage that right now.`
	case characters.PoolConviction:
		return `You cannot muster the resolve for that right now.`
	}
	return ``
}

// Cost sends CostText when the cost was refused, and nothing otherwise. A mob
// actor's SendText is a no-op, so mob wrappers stay silent as before.
func Cost(to Recipient, result characters.CostCommitResult) {
	if text := CostText(result); text != `` {
		send(to, text)
	}
}
```

Append to `internal/refusal/carrying.go`:

```go
// NotWearingText answers a name the player looked for in what they wear.
func NotWearingText(typed string) string {
	return `You aren't wearing ` + quoted(typed) + `.` + hint(`inventory`)
}

func NotWearing(to Recipient, typed string) { send(to, NotWearingText(typed)) }

// NotInStorageText answers a name the player looked for in their storage.
func NotInStorageText(typed string) string {
	return `You don't have ` + quoted(typed) + ` in storage.` + hint(`storage`)
}

func NotInStorage(to Recipient, typed string) { send(to, NotInStorageText(typed)) }
```

and change `NotCarryingText` to use the same helper, adding `quoted` to `refusal.go`:

```go
// quoted is typed echoed in single quotes, or "that" when nothing is left.
func quoted(typed string) string {
	if e := echo(typed); e != `` {
		return `'` + e + `'`
	}
	return `that`
}

// Echo is the slice 1 echo for lines outside the package that repeat what the
// player typed about their own things (spec section 2). Never pass it a name
// the player could only find by looking.
func Echo(typed string) string { return echo(typed) }
```

```go
func NotCarryingText(typed string) string {
	return `You aren't carrying ` + quoted(typed) + `.` + hint(`inventory`)
}
```

Append to `internal/refusal/membership.go`:

```go
func NotInPartyText() string { return `You aren't in a party.` + hint(`party create`) }

func NotInParty(to Recipient) { send(to, NotInPartyText()) }

// NotGuildLeaderText answers a leader-only guild command; action completes
// "Only the guild leader can ...", empty reads "do that".
func NotGuildLeaderText(action string) string {
	return `Only the guild leader can ` + orDoThat(action) + `.`
}

// NotGuildOfficerText answers an officer-or-leader guild command.
func NotGuildOfficerText(action string) string {
	return `Only guild officers and the leader can ` + orDoThat(action) + `.`
}

func NotGuildLeader(to Recipient, action string)  { send(to, NotGuildLeaderText(action)) }
func NotGuildOfficer(to Recipient, action string) { send(to, NotGuildOfficerText(action)) }

// CompanionsFullLine answers a summon or charm past the companion limit.
const CompanionsFullLine = `You are already sustaining as many companions as your will can hold.`

func CompanionsFull(to Recipient) { send(to, CompanionsFullLine) }

// NotPartyMemberText answers a party command naming someone not in the
// player's party. It echoes: the player's own party is a name they know.
func NotPartyMemberText(typed string) string {
	return `No one in your party is called ` + quoted(typed) + `.`
}

func NotPartyMember(to Recipient, typed string) { send(to, NotPartyMemberText(typed)) }

func orDoThat(action string) string {
	if action == `` {
		return `do that`
	}
	return action
}
```

and make `NotPartyLeaderText` use `orDoThat`.

- [ ] **Step 4: Run them to see them pass.** `go test ./internal/refusal/` Expected: PASS (including the slice 1 and 2 tests).

- [ ] **Step 5: Commit.** `git add internal/refusal/` (named files) and commit `feat(refusal): busy, cost, storage, guild and party helpers (#371)`.

### Task 3: Name validation speaks in sentences

**Files:** Modify `internal/users/validate_actor_name.go`, `internal/users/validate_actor_name_test.go`, `internal/usercommands/pet.go`, `internal/usercommands/renameself.go`.

- [ ] **Step 1: Write the failing test.** Append to `validate_actor_name_test.go`:

```go
// M7 slice 3 (#371): every error is a sentence a player can read, and the
// banned case never prints the moderation pattern.
func TestValidateActorNameSpeaksInSentences(t *testing.T) {
	setupValidationConfig(t)
	_ = configs.AddOverlayOverrides(map[string]any{"Validation.BannedNames": []string{"*grob*"}})
	t.Cleanup(func() { _ = configs.AddOverlayOverrides(map[string]any{"Validation.BannedNames": []string{}}) })

	err := ValidateActorName("", ValidateActorOpts{})
	if err == nil || err.Error() != "Names must be between 1 and 20 characters long." {
		t.Fatalf("length: %v", err)
	}
	err = ValidateActorName("Grobnar", ValidateActorOpts{})
	if err == nil || err.Error() != "That name is not allowed." {
		t.Fatalf("banned: %v", err)
	}
	if strings.Contains(err.Error(), "grob") {
		t.Fatalf("the banned pattern leaked: %v", err)
	}
	cleanup := seedMobName(t, "Skeletor")
	defer cleanup()
	err = ValidateActorName("Skeletor", ValidateActorOpts{})
	if err == nil || err.Error() != "That name is already in use." {
		t.Fatalf("mob collision: %v", err)
	}
}
```

If the zero-length case trips `NameRejectRegex` first in the test config, use a 25-letter name for the length case instead (the bound is 20).

- [ ] **Step 2: Run it to see it fail.** `go test ./internal/users/ -run TestValidateActorNameSpeaksInSentences` Expected: FAIL on the length message.

- [ ] **Step 3: Implement.** In `validate_actor_name.go`: the length error becomes `fmt.Errorf("Names must be between %d and %d characters long.", ...)`; the banned case becomes `errors.New("That name is not allowed.")` (drop `bannedPattern` from the message; keep `_, ok :=`); the four collision errors all become `errors.New("That name is already in use.")`. Update the existing table row `errSubstr: "between"` (still matches). In `pet.go:39` and `renameself.go:46` send `err.Error()` without the `That name won't work: ` prefix, on `messaging.CategoryRefusal`. `companion.go:122` already sends `err.Error()`; change its category to `CategoryRefusal`.

- [ ] **Step 4: Run.** `go test ./internal/users/ ./internal/usercommands/ ./internal/inputhandlers/ ./internal/playtestprofiles/` Expected: PASS. A test pinning an old fragment ("that name is in use") is updated to the sentence.

- [ ] **Step 5: Commit.** `git add` the four files plus any updated test, commit `fix(names): validation errors are sentences, no banned pattern (#371)`.

### Task 4: Every FAM site calls its helper

**Files:** the FAM sites in Wording table B; `internal/usercommands/busy_refuse.go`, `internal/actions/action_cost.go`, `internal/actions/sleeping_target.go`; root `refusal_phrase_guard_test.go`; the tests in Fact 17.

- [ ] **Step 1: Add the phrase rows (red).** Append to `refusalPhrases`:

```go
	{"busy", regexp.MustCompile(`while focused on your work`), "You can't bash while focused on your work. Finish or be interrupted first."},
	{"busy, working", regexp.MustCompile(`already (busy )?working on something|busy with something else|too busy to cast`), "You're already busy working on something."},
	{"cost", regexp.MustCompile(`too spent to manage|cannot muster the resolve`), "You are too spent to manage that right now."},
	{"asleep", regexp.MustCompile(`is fast asleep\. You could make some noise`), "%s is fast asleep. You could make some noise."},
	{"under attack", regexp.MustCompile(`while you are under attack`), "You can't do that while you are under attack!"},
	{"not here, vanished", regexp.MustCompile(`seem to have vanished`), "They seem to have vanished."},
	{"cooldown, rounds", regexp.MustCompile(`more rounds (before|to)`), "You need to wait %d more rounds before you can try that again."},
	{"party", regexp.MustCompile(`not attached to a party|You are not in a party`), "You are not attached to a party."},
	{"guild leader", regexp.MustCompile(`Only the guild leader can|Only officers and the leader can`), "Only officers and the leader can invite."},
	{"storage", regexp.MustCompile(`You don't have .* in storage`), "You don't have a %s in storage."},
	{"companions full", regexp.MustCompile(`maintain any more companions|as many companions as your will`), "You cannot maintain any more companions."},
	{"no companions", regexp.MustCompile(`You have no companions`), "You have no companions to dismiss."},
	{"prompt, specify", regexp.MustCompile(`^Specify what you want|You need to specify a message`), "Specify what you want to sell."},
```

Run `go test . -run 'TestRefusalPhrase'` Expected: `TestRefusalPhraseGuardCanFail` PASS; `TestRefusalPhrasesLiveInThePackage` FAIL listing the FAM sites. If a hit is not one of table B's sites, it is either a sibling of the same situation (convert it too, and add it to the ledger as a controller row) or a different text (narrow the row's regexp). Never allow-list a file.

- [ ] **Step 2: Delete the old texts first and let the compiler list consumers.** Delete `busyRefusalText` (keep `refuseWhileBusy`, whose body becomes `refusal.WhileBusy(user, verb)` on the `IsActing()` branch); delete `actions.CostRefusalText` and `costRefusalText`; in `RefuseIfAsleep` build the line with `refusal.TargetAsleepText(name)` and send on `CategoryRefusal`. Run `go build ./... && go vet ./internal/... ./modules/...` and fix every caller: `actions.CostRefusalText(x)` sends become `refusal.Cost(user, x)` (a caller that tested the text for emptiness tests `refusal.CostText(x) != ""`); a caller that prefixed or joined it (`throw.go:246`, `throttle.go:45`, `trip.go:38`, `taunt.go:31`, `shoot.go:108`, `sneak.go:51`) keeps its composition around `refusal.CostText`.

- [ ] **Step 3: Convert the rest of table B.** Each row's call replaces the old send, on the line the table names. A FAM site that sat between `user.SendText(..., "")` blank-line sends keeps the blanks only if its siblings in the same function do. Imports: add `github.com/GoMudEngine/GoMud/internal/refusal`; in a function with a local named `refusal` (from `AimBySight`), rename the local to `aimRefusal` first (slice 2 trap).

- [ ] **Step 4: Update the pinned tests (Fact 17).** Each assertion on old text asserts the helper's `...Text`/`...Line` instead, and on `CategoryRefusal`. In `special_move_categories_test.go` remove the markers whose sends left `CategorySystem` ("focused on your work", `CostRefusalText`, "need a moment", "no target"); a marker still needed is one whose line is still on System in those four files (`grep -n CategorySystem internal/usercommands/{taunt,throw,rally,warcry}.go`).

- [ ] **Step 5: Run.** `go test . ./internal/... ./modules/...` Expected: PASS, the phrase guard green.

- [ ] **Step 6: Commit.** `git add` each changed file by name, commit `refactor(refusal): FAM sites call the helpers (#371)`.

### Task 5: The opener guard, and group G1 (outside `usercommands`)

**Files:** Create root `refusal_system_opener_guard_test.go`; the G1 sites (ledger sections for every file not under `internal/usercommands/`).

- [ ] **Step 1: Write the guard.** `refusal_system_opener_guard_test.go`:

```go
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// Messaging M7 slice 3 (#371): a line that opens like a refusal rides
// messaging.CategoryRefusal, never CategorySystem. The guard looks at every
// call that passes messaging.CategorySystem and reads the argument after it
// (a string literal, the format of fmt.Sprintf, or the left end of a +
// concatenation), strips tags and tests it against refusalOpener. The
// slice 3 ledger (docs/superpowers/audits/messaging-m7-refusal-ledger.md)
// rules on the rest, which no pattern can tell apart from a report.
//
// A hit is converted, or refusalOpener is narrowed; never allow-list a file.
// A line holding ", but " reports a partial success ("You don't have that
// much gold on hand, but what you do have you deposit."), not a refusal.
var refusalOpener = regexp.MustCompile(`^(You (can't|cannot|don't have|aren't|are not|need to|must|'re too|are too|'re already|are already|have no)|Only (the|officers) )`)

var openerTag = regexp.MustCompile(`<[^>]*>`)

// openerGuardScope is widened by each group task of slice 3; Task 12 sets it
// to every Go file under internal/ and modules/.
var openerGuardScope = []string{
	"internal/actions/", "internal/behaviortree/", "internal/bountyhunter/",
	"internal/combat/", "internal/hooks/", "internal/justice/",
	"internal/messaging/", "internal/mobcommands/", "internal/questengine/",
	"internal/rooms/", "modules/",
}

func inOpenerScope(rel string) bool {
	for _, p := range openerGuardScope {
		if strings.HasPrefix(rel, p) {
			return true
		}
	}
	return false
}

// leadLiteral is the text a send argument opens with, when a literal decides it.
func leadLiteral(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return ``, false
		}
		s, err := strconv.Unquote(v.Value)
		if err != nil {
			return v.Value, true
		}
		return s, true
	case *ast.CallExpr:
		if calledName(v) == `Sprintf` && len(v.Args) > 0 {
			return leadLiteral(v.Args[0])
		}
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			return leadLiteral(v.X)
		}
	case *ast.ParenExpr:
		return leadLiteral(v.X)
	}
	return ``, false
}

func systemOpenerHits(t *testing.T, rel string, src []byte) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, 0)
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		for i, a := range call.Args {
			sel, ok := a.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != `CategorySystem` || i+1 >= len(call.Args) {
				continue
			}
			s, ok := leadLiteral(call.Args[i+1])
			if !ok {
				continue
			}
			plain := strings.TrimSpace(openerTag.ReplaceAllString(s, ``))
			if refusalOpener.MatchString(plain) && !strings.Contains(plain, `, but `) {
				hits = append(hits, rel+`:`+strconv.Itoa(fset.Position(call.Pos()).Line)+`: `+plain)
			}
		}
		return true
	})
	return hits
}

func TestRefusalOpenersNeverRideSystem(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal(`runtime.Caller failed`)
	}
	root := filepath.Dir(here)
	scanned := 0
	for _, top := range []string{`internal`, `modules`} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, `.go`) || strings.HasSuffix(path, `_test.go`) {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if !inOpenerScope(rel) {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			for _, h := range systemOpenerHits(t, rel, src) {
				t.Errorf(`%s (send it on messaging.CategoryRefusal, or use internal/refusal)`, h)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 50 {
		t.Fatalf(`scanned only %d Go files; the walk or the scope is broken`, scanned)
	}
}

// TestRefusalOpenerGuardCanFail feeds the finder each send shape.
func TestRefusalOpenerGuardCanFail(t *testing.T) {
	src := "package p\n" +
		"func a(u U) { u.SendText(messaging.CategorySystem, `You can't do that.`) }\n" +
		"func b(u U) { u.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg=\"red\">You aren't carrying %s.</ansi>`, x)) }\n" +
		"func c(u U) { u.SendText(messaging.CategorySystem, `Only the guild leader can `+x) }\n" +
		"func d(u U) { craftDeliver(u, messaging.CategorySystem, `You need to be at a forge.`) }\n" +
		"func e(u U) { u.SendText(messaging.CategoryRefusal, `You can't do that.`) }\n" +
		"func f(u U) { u.SendText(messaging.CategorySystem, `You don't have that much gold on hand, but you deposit it.`) }\n" +
		"func g(u U) { u.SendText(messaging.CategorySystem, `Only you may now withdraw gold.`) }\n" +
		"func h(u U) { u.SendText(messaging.CategorySystem, `You pick up the sword.`) }\n"
	hits := systemOpenerHits(t, `probe.go`, []byte(src))
	want := []string{`probe.go:2:`, `probe.go:3:`, `probe.go:4:`, `probe.go:5:`}
	if len(hits) != len(want) {
		t.Fatalf(`want %d hits, got %v`, len(want), hits)
	}
	for i, w := range want {
		if !strings.HasPrefix(hits[i], w) {
			t.Fatalf(`hit %d: want %s, got %s`, i, w, hits[i])
		}
	}
}
```

- [ ] **Step 2: Run it to see it fail.** `go test . -run 'TestRefusalOpener'` Expected: `CanFail` PASS; `NeverRideSystem` FAIL listing G1 refusals still on System.

- [ ] **Step 3: Convert G1.** For every G1 row in the ledger: RETAG changes `messaging.CategorySystem` to `messaging.CategoryRefusal` and nothing else; REWORD and FAIL apply tables C and D. The G1 rewords: `steal.go:231,242` wrap the formatted line in `messaging.HideNames(line, []string{m.Character.Name}, sight)` where `sight` is the thief's `messaging.ParticipantSight(char, room)` (the `melee_target.go:207` pattern; look up how `steal.go` already reaches the room and character); `steal.go:877,917` become `` `You can't carry that. You're already overloaded!` ``; `auctions.go:237` as table C. The G1 failure: `actions/sleep.go:65` `refusal.Failed(actor, "sleep", err, "userId", actor.GetUserId())`.

- [ ] **Step 4: Sight test for the steal rewords (write it before Step 3's steal edit, run it red).** The steal stub actor discards text (`economy_test.go:52`), so the test records its own. Append to `internal/actions/steal_companion_test.go`:

```go
// recordingStealActor keeps what the thief is told.
type recordingStealActor struct {
	*stubActorWithId
	told []string
}

func (r *recordingStealActor) SendText(_ messaging.Category, msg string) { r.told = append(r.told, msg) }

// M7 slice 3 (#371): the companion refusal names the mob only for a thief
// who can see it.
func TestSteal_CompanionRefusalHidesTheNameInTheDark(t *testing.T) {
	for _, lit := range []bool{true, false} {
		actor := &recordingStealActor{stubActorWithId: newStealPlayerActor(200, 8)}
		if lit {
			lamp := 60
			actor.room.Lamp = &lamp
		} else {
			actor.room.Biome = "cave"
		}
		target := newStealTestMob(testMobInstId, 50, 1)
		target.Character.Charmed = characters.NewCharm(4242, 10, ``)
		mobs.SetInstanceForTest(testMobInstId, target)
		delete(actor.char.Cooldowns, skills.Skullduggery.String("steal"))

		Steal(actor, StealOptions{TargetMobInstanceId: testMobInstId})

		mobs.SetInstanceForTest(testMobInstId, nil)
		told := strings.Join(actor.told, "\n")
		if lit != strings.Contains(told, "Bandit") {
			t.Errorf("lit=%v: the name shows only in light; told %q", lit, told)
		}
	}
}
```

Add `strings` and `messaging` to the imports. If `Steal` reads the room through `actor.GetRoom()` and the stub returns a copy, set the light on the room the stub returns instead (`stubActor.room` is a `*rooms.Room`, so the pointer above is the one it returns). If the dark scene never reaches the companion check (a sight gate earlier in `Steal` refuses first), the test asserts on that first refusal instead and still requires the name absent.

- [ ] **Step 5: Run.** `go test . ./internal/... ./modules/...` Expected: PASS, the opener guard green for its scope.

- [ ] **Step 6: Commit.** `git add` named files, commit `refactor(refusal): G1 refusals ride CategoryRefusal; opener guard (#371)`.

### Task 6: Group G2 (`usercommands/[a-f]*`)

**Files:** root `refusal_system_opener_guard_test.go`; the G2 ledger sections (`achievements.go` to `flee.go`) and `admin.zap.go:60`.

- [ ] **Step 1: Widen the guard (red).** Add `"internal/usercommands/a", "internal/usercommands/b", "internal/usercommands/c", "internal/usercommands/d", "internal/usercommands/e", "internal/usercommands/f"` to `openerGuardScope`. Run `go test . -run TestRefusalOpenersNeverRideSystem` Expected: FAIL listing G2 sites and `admin.zap.go:60`.

- [ ] **Step 2: Convert G2.** RETAG rows change the category only (`admin.zap.go:60` too: an admin's refusal is a refusal). Rewords from table C: `character.go:45,50,400`, `companion.go:122` (done in Task 3; category only), `craft.go:135` (`fmt.Sprintf(`<ansi fg="red">No recipe matches '%s'.</ansi> Try <ansi fg="command">craft list</ansi>.`, refusal.Echo(rest))` through `craftDeliver(user, messaging.CategoryRefusal, ...)`), `dismiss.go:86`. Failures from table D: `bug.go:31,38,53`, `drop.go:105`. The `character.go` refusals that print the typed `rawResponse` (`:232,311,357,458`) pass it through `refusal.Echo`.

- [ ] **Step 3: The hire amount.** The hire refusal sits inside a multi-step `cmdPrompt` flow with no test fixture (the `usercommands_test.go:7387` hire tests cover hired-alt listing, not this refusal), so the one-argument fix (`charValue, charValue` becomes `user.Character.Gold, charValue`) is checked by the task review and by the Task 13 playtest step that hires an alt with too little gold. Say so in the commit message.

- [ ] **Step 4: Run.** `go test . ./internal/usercommands/` Expected: PASS.

- [ ] **Step 5: Commit.** `refactor(refusal): G2 refusals ride CategoryRefusal (#371)`.

### Task 7: Group G3 (`usercommands/[g-o]*`)

**Files:** the guard; the G3 ledger sections (`gearup.go` to `online.go`).

- [ ] **Step 1: Widen the guard (red)** with prefixes `g` to `o` (`"internal/usercommands/g"` ... `"internal/usercommands/o"`). Run; expect FAIL on G3 sites.

- [ ] **Step 2: Convert G3.** RETAG by category. Rewords from table C: `get.go:413,636,904`, `go.go:91,179`, `grapple.go:73`, `guild.go:145,309,558,572,601,617,734`, `inbox.go:55`, `jail.go:66`, `list.go:113`. Failures: `go.go:252`, `guild.go:542`. For `guild.go:572,601`, check the error first: when it is not the player-facing one (the guild vanished mid-command), call `refusal.Failed` instead of the plain line.

- [ ] **Step 3: Run.** `go test . ./internal/usercommands/` Expected: PASS.

- [ ] **Step 4: Commit.** `refactor(refusal): G3 refusals ride CategoryRefusal (#371)`.

### Task 8: Group G4 (`usercommands/[p-r]*`)

**Files:** the guard; the G4 ledger sections (`party.go` to `report.go`); `internal/usercommands/refusal_echo_sight_test.go`.

- [ ] **Step 1: Write the sight test (red).** Add `{"party invite", partyInvite, false}` to both command tables in `refusal_echo_sight_test.go`, where `partyInvite` is

```go
func partyInvite(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	return Party("invite "+rest, user, room, flags)
}
```

Expected today: FAIL at full light with "zzyzx" (`%s not found.` echoes the typed name).

- [ ] **Step 2: Widen the guard (red)** with prefixes `p`, `q`, `r`.

- [ ] **Step 3: Convert G4.** RETAG by category. `party.go:203` is FAM `refusal.NotHereCreature(user)` (Task 4 may have done it). Rewords: `party.go:157,159,624,651`, `password.go:49` (the length error: the sentence; the bcrypt error: `refusal.Failed(user, "password change", err, "userId", user.UserId)`; tell them apart with the error's text from `SetPassword`), `pet.go:39` (Task 3), `picklock.go:51`, `put.go:36`, `remove.go:36,59` (`user.Character.CursedRefusal(it)` on `CategoryRefusal`), `renameself.go:46` (Task 3), `renameself.go:77`. Failure: `petition.go:62`.

- [ ] **Step 4: Run.** `go test . ./internal/usercommands/` Expected: PASS, the party invite rows included; `remove_curse_test.go` updated to `CursedRefusal`'s text.

- [ ] **Step 5: Commit.** `refactor(refusal): G4 refusals ride CategoryRefusal (#371)`.

### Task 9: Group G5 (`usercommands/[s-w]*`)

**Files:** the guard; the G5 ledger sections (`salvage.go` to `whisper.go`, including `usercommands.go`); a new `internal/usercommands/refusal_room_container_test.go`.

- [ ] **Step 1: Write the container sight test (red).** `internal/usercommands/refusal_room_container_test.go`:

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/assert"
)

// M7 slice 3 (#371): a refusal about a room container never names it, since
// the player found it by looking (spec section 2).
func TestRoomContainerRefusalsNeverNameIt(t *testing.T) {
	for _, band := range []aimBand{aimFull, aimDark} {
		user, room := aimScene(t, band)
		room.IsStorage = false
		room.Containers = map[string]rooms.Container{
			"chest": {Lock: gamelock.Lock{Difficulty: 5}, Recipes: map[int][]int{1: {2}}},
		}
		events.DrainQueuedMessagesForTest(1)
		_, _ = Use("chest", user, room, events.EventFlag(0))
		_, _ = Storage("", user, room, events.EventFlag(0))
		told := strings.ToLower(aimTold(1))
		assert.NotContains(t, told, "chest", "band %d", band)
		room.Containers = nil
	}
}
```

A locked `Lock{Difficulty: 5}` reads `IsLocked()` true while `UnlockedRound` is 0 (`gamelock.go:24-31`). Run: expect FAIL (both commands name the chest today).

- [ ] **Step 2: Widen the guard (red)** with prefixes `s` to `w`, which takes in `usercommands.go`.

- [ ] **Step 3: Convert G5.** RETAG by category. FAM `usercommands.go:430` is `refusal.WhileBusy(user, "do that")` (the fold counts go). Rewords: `salvage.go:42`, `set.go:594`, `shoot.go:131`, `skill.cast.go:65` (`refusal.Echo(rest)`), `skill.cast.go:163`, `skill.cast.go:337`, `stand.go:82` (two sends), `storage.go:33,366`, `talk.go:195`, `use.go:36,45`. Failures: `salvage.go:148`, `skill.map.go:153,225` (drop the adjacent `mudlog.Error`; `Failed` logs), `suggest.go:29,36,51`.

- [ ] **Step 4: Run.** `go test . ./internal/usercommands/` Expected: PASS.

- [ ] **Step 5: Commit.** `refactor(refusal): G5 refusals ride CategoryRefusal (#371)`.

### Task 10: The re-inventory

- [ ] **Step 1:** Run `python tools/refusal_classify.py --tsv > "$TMP/after.tsv"` and count: the rows left must be the ledger's NOT, NARR and USAGE rows (695 + 63 + 47 = 805), give or take lines a Task 11 fix removes. Any row the classifier marks `refusal` that the ledger ruled NOT is read once more; a real refusal is converted and added to the ledger as a controller row.

- [ ] **Step 2:** In the ledger, add a "Done" column note at the top: "Slice 3 converted the FAM, RETAG, REWORD and FAIL rows in PR #<n>." (Rows are never deleted.)

### Task 11: Found along the way

**Files:** `drop.go`, `help.go`, `mutations.go`, `modules/auctions/auctions.go`, `internal/questengine/bridge.go`, and their tests.

- [ ] **Step 1:** `drop.go:171`: delete the grenade debug send (keep any other grenade handling in that block).
- [ ] **Step 2:** `help.go:79-82`: on `err != nil` call `refusal.Failed(user, "help", err, "userId", user.UserId)` and return `true, nil`.
- [ ] **Step 3:** `mutations.go:29`: replace the `[data missing]` send with `mudlog.Error("mutations: missing spec", "mutationId", mutId, "userId", user.UserId)` and `continue`.
- [ ] **Step 4:** `auctions.go`: `var errBidderNotFound = errors.New("bidder not found")`, returned by `Bid` in place of `errors.New("User not found.")`; at `:344` `if errors.Is(err, errBidderNotFound) { refusal.Failed(user, "bid", err, "userId", user.UserId) } else { user.SendText(messaging.CategoryRefusal, err.Error()) }`. A test in `modules/auctions` asserting "User not found." is updated.
- [ ] **Step 5:** `bridge.go:290`: `name := spellId; if sp := spells.GetSpell(spellId); sp != nil { name = sp.Name }` and print `name`. Add a test beside the existing bridge tests (`grep -ln TeachSpell internal/questengine/*_test.go`) asserting the name, not the id.
- [ ] **Step 6:** Run `go test ./internal/... ./modules/...`; commit `fix: debug and internal text reaching players (#371)`.

### Task 12: Widen to everything, siblings, docs

- [ ] **Step 1:** Set `openerGuardScope = []string{"internal/", "modules/"}` and update its comment. Run `go test . -run TestRefusalOpener` Expected: PASS. A hit in a file no group covered (`internal/usercommands/admin.*` beyond `zap.go:60`, a `[x-z]*` file) is converted the same way.
- [ ] **Step 2:** `special_move_categories_test.go`: prune `systemNoticeMarkers` to the markers still matched by a `CategorySystem` line in the four files; if none remain, the test asserts those files send nothing on `CategorySystem` (slice 4 folds it into the tag guard).
- [ ] **Step 3:** Comments: `trio.go:12-14` and `messaging.go:113-116` now say every refusal rides `CategoryRefusal` (slice 3, #371) and System keeps reports, confirmations, help, usage and admin output until slice 4.
- [ ] **Step 4:** `internal/refusal/context.md`: add every new helper to the Helpers table with its line; document `Echo` (own things only), `TargetAsleepText` (caller hides the name), `Cost` (silent unless refused); add the opener guard to Guards; replace "Left for later slices" with slice 4's list (`CategoryUsage`, `CategoryAdmin`, narration out of System, the tag guard). `internal/messaging/context.md`: `CategoryRefusal` now carries every refusal. `internal/actions/context.md` and `internal/usercommands/context.md`: drop `CostRefusalText` and `busyRefusalText`, point to `refusal.Cost` and `refusal.WhileBusy`. Verify every symbol named exists (`python tools/context_md_audit.py internal/refusal internal/actions internal/usercommands internal/messaging`).
- [ ] **Step 5:** `docs/README.md`: the plan row.
- [ ] **Step 6:** File issues (search first, `gh issue list --repo pruuk/DOGMud --search "..."`), each under the right epic: household steal matches floor items by name with no sight check (`skill.skullduggery.steal.go:135` `householdBaubleNamed`, under #382); `follow.go:659` names a mob without `HideNames` (#382); `gmcp.Mudlet.go:587,690` read `gmcpData` without a cache-miss check (#394); the ledger's 63 NARR rows as slice 4 input (comment on #371, not a new issue).
- [ ] **Step 7:** Commit `docs(refusal): slice 3 helpers, guard and context (#371)`.

### Task 13: Gate, boot, playtest, PR

- [ ] **Step 1:** Pre-push gate per `dogmud-shipping` (full `go test ./...`, `-race` on the touched packages, `go vet`, the boot check from a detached worktree).
- [ ] **Step 2:** Playtest per `dogmud-playtesting` (spec Testing): in light and dark, `kick` while crafting (WhileBusy), `kick` while spent (Cost), `remove` a cursed item, `use` and `storage` beside a container in the dark, `party invite` a missing name and an unseen player, `party kick` with no name, `guild disband` as a member, `steal` from a companion at shapes, `salvage` a missing corpse, `cast` a misspelled spell, a long `craft` name, `jail` with too little gold, hire an alt with too little gold (the first amount is the player's gold), `help`. Every line reads as the Wording tables, none echoes a looked-for name, none passes 80 columns. File findings as issues under #371.
- [ ] **Step 3:** Push and open the PR with `--repo pruuk/DOGMud`, body naming slice 3 of #371 (no closing keyword: #371 stays open for slice 4), the Wording tables' counts, the ledger link, and the test and playtest evidence.

## Self-review

- Spec coverage for slice 3: remaining refusals moved package by package (Tasks 4 to 9, by the ledger); the families the spec named for later (`WhileBusy`, `TargetAsleep`, `Cost`, `NotWearing`) plus the ones the ledger found (Task 2); owned helpers keep their logic (`CursedRefusal` reused at `remove.go`; `ReservationRefusal`, `ShopSightRefusalText`, `ChannelDefenceShortageText`, `craftSkillTooLowText` sends are RETAG rows); review page (the Wording tables, approved with the plan, as slices 1 and 2 did); classifier (Task 1, spec section 6); phrase rows with probes (Task 4); playtest (Task 13). Deferred to slice 4 by the spec: `CategoryUsage`, `CategoryAdmin`, narration out of System, the tag guard.
- The opener guard sees only literal-led sends (Fact 19 sized it: 192 of the converted rows). The other RETAG rows (dynamic text, "There is no ...", "That is locked.") have the ledger, not a guard, until slice 4's tag guard; Task 10 re-runs the classifier to catch drift.
