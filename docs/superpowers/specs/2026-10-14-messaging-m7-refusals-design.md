# Messaging M7: refusals and system output

Issue #371 (under the messaging arc #381). Arc spec section:
`docs/superpowers/specs/2026-08-31-messaging-unification-design.md`, "M7:
Refusals and system output". This spec answers that section's open questions
and designs the whole stage; the implementation plan splits it into slices.

## Facts verified against source (master `9c0ca1a57`, 2026-10-14)

| Fact | Source |
|---|---|
| 2110 non-comment, non-test references to `messaging.CategorySystem` in `internal/` and `modules/` | `grep -rn CategorySystem --include=*.go internal modules`, minus `_test.go` and comment lines |
| 587 of them are in `internal/usercommands/admin.*.go` | same grep on `admin.*` |
| Nearly all are one-player sends: `user.SendText` 1834, `actor.SendText` 125, `u.SendText` 42, `targetUser.SendText` 14, `craftDeliver` 20, other receivers 29 | inventory by send path, 2026-10-14 |
| `CategoryError` 16, `CategoryWarning` 28, `CategoryTip` 10 production references | same grep per category |
| Categories are an `iota` enum; `CategorySystem`, `CategoryError`, `CategoryWarning` at `internal/messaging/messaging.go:82-84`; new values are appended so none shifts (`CategoryLight`, `CategoryCraft` were) | `messaging.go:23-123` |
| System gets every normalize stage (`skipStages` returns 0 for it) and never wraps (absent from `shouldWrap`); the pipeline comment calls System and Broadcast "mixed buckets" and says "Error and Warning are Group A system output, which M7 owns" | `internal/messaging/normalize.go:25`, `internal/messaging/pipeline.go:100-121` |
| Each category paints its text with the ANSI alias named by `Category.String()`; `system: 8` | `pipeline.go:170-174` (`applyCategoryColor`), `_datafiles/world/dogmud/ansi-aliases.yaml:260` |
| Sight refusals deliberately do not echo the typed name, so a creature that is there but unseen and a name that matches nothing read the same (#454): `AimNothingLine`, `AimNotHereLine` | `internal/actions/sight_aim.go:19-27`, `AimBySight` at `:57` |
| Existing refusal helpers: `actions.CostRefusalText` (`action_cost.go:42`), `actions.ShopSightRefusalText` (`shop_sight.go:28`), `usercommands.busyRefusalText` / `refuseWhileBusy` (`busy_refuse.go:12,24`), `usercommands.fleeRefusalText` (`flee.go:15`), `(*Character).ReservationRefusal` (`reservation.go:672`), `(*Character).CursedRefusal` (`wear_slot.go:36`), `actions.RefuseIfAsleep` / `RefuseMobIfAsleep` (`sleeping_target.go:44,67`), `usercommands.noSightRefusal` (`look.go:553`), `usercommands.craftSkillTooLowText` (`craft.go:299`), `combat.ChannelDefenceShortageText` (`combat_helpers.go:173`) | grep for each `func`/`const`/`var` |
| No general "not here" helper exists; "it isn't here" is written about 22 ways ("You don't see that here.", "...them here.", "...'%s' here.", "...a %s around.", "Can't find that to pet.", "You don't seem to be carrying that.") | inventory, `-A1` grep of send lines |
| Most repeated refusal literals: "You need a moment to recover before attempting another special move." 14, "You are not in a guild." 14, "You have no target!" 10, "You have no companion travelling with you." 6, "You aren't advanced enough at skullduggery for that." 6, "You can't do that while in combat!" 5; plus 37 lines with "You don't have" | literal-argument sends, normalized |
| "Something went wrong" appears on 17 non-comment lines in 7 files (`give.go` 6, `party.go` 5, `actions_mob.go` 2, one each in `salvage.go`, `admin.room.go`, `character.go`, `show.go`) | grep |
| 35 lines carry a bare "what?" / "whom?" prompt ("Pet what?", "Show what? To whom?") | grep |
| 75 lines contain "Usage:"; ten admin commands have a `*Usage(user)` function | grep; `admin.*.go` |
| Help output: 466 templates in `_datafiles/world/dogmud/templates/help`, rendered by `usercommands/help.go` and sent on `CategorySystem` (`help.go:89,97`); `modules/webhelp` reuses `GetHelpContents` | `help.go` |
| Localization is `internal/language` (go-i18n): 14 `language.T` calls (login, inbox, online); 16 keys in `_datafiles/localize/en.yaml`; `dogmud/localize` is not in `LanguagePaths` (`config.yaml:2362`) | grep |
| No `*_guard_test.go` mentions `CategorySystem`; two package tests police it for named files (`mobcommands/throttle_categories_test.go:17`, `usercommands/special_move_categories_test.go:56-80`); `messaging/pipeline_test.go:249` `TestSystemNeverWraps` | grep |
| Issues #254 and #284 (named in #371 as M7 bugs) are closed; #281 (commands dropped inside TurnMs) is input handling, not output | `gh issue view` |

## Problem

Every player-facing string now has an owner except Group A: refusals, prompts,
usage lines, admin output, reports and confirmations, all sent on one mixed
bucket, `CategorySystem`. Nothing gives a refusal one place to live, so the
same situation reads differently from command to command (about 22 ways to
say "it isn't here"), some refusals leak internal failures ("Something went
wrong."), and nothing stops the next author from adding a 23rd wording. Nor
can any code or client tell a refusal from a score sheet.

## Owner rulings

- 2026-09-07: refusals are "standardizable with a few extra keywords passed
  in": Go plus a shared vocabulary, not a store.
- 2026-10-14: the first thing players should notice is consistent refusals
  (over retagging first, translation, or only the known bugs).
- 2026-10-14: a refusal echoes what the player asked for and adds a short hint
  when a next step exists.
- 2026-10-14: approach A: one Go package of refusal helpers with keywords, a
  refusal category, and a phrase guard; the stage covers every Group A string,
  sliced at the plan level.

## Design

### 1. `internal/refusal`

A new package (with `context.md`) holding one helper per refusal situation.
Each helper takes the player (the `users.UserRecord` or the actor the command
already has) plus keywords, and sends exactly one line on `CategoryRefusal`.
Each has a sibling `...Text(...) string` returning the line, for tests and for
the few callers that combine lines. Helpers never print raw numbers (the
player-copy rule: no seconds, no damage, no skill values).

Families (each a file):

| Family | Helper shape | Replaces |
|---|---|---|
| Not here | `NotHereCreature(u)`, `NotHereThing(u)`, `NothingToAim(u)` | `AimNotHereLine`, `AimNothingLine`, the "You don't see..." variants |
| Not carrying | `NotCarrying(u, typed)`, `NotWearing(u, typed)` | "You don't have", "You aren't carrying", "You don't seem to be carrying that." |
| No target | `NoTarget(u, verb)` | "You have no target!" |
| Busy or unable | `WhileFighting(u, verb)`, `WhileBusy(u, verb)`, `TargetAsleep(u, name)` | "You can't do that while in combat!", `busyRefusalText`, the sleep refusals |
| Cooldown | `Cooldown(u, move)` | "You need a moment to recover before attempting another special move." |
| Membership | `NotInGuild(u)`, `NoCompanion(u)`, `NotPartyLeader(u)` | the repeated guild, companion and party lines |
| Cost and shortage | `Cost(u, result)` | `CostRefusalText`, the shortage texts that read as refusals |
| Prompt | `Prompt(u, verb, usage)` | the 35 "X what?" / "X whom?" prompts |
| Failure | `Failed(u, action, err)` | "Something went wrong." |

Helpers that already live on a type and need its state (`ReservationRefusal`,
`CursedRefusal`, `ChannelDefenceShortageText`, `craftSkillTooLowText`,
`fleeRefusalText`, `ShopSightRefusalText`, `noSightRefusal`) keep their logic
where it is; their callers send the result on `CategoryRefusal`, and where the
wording overlaps a family above, they call the family's `...Text` for it.

### 2. Echo rule (sight-safe)

- A name the player can only find by looking (a creature; an item on the
  floor or in a room container) never echoes. Per #454 a name that is there
  but unseen must read the same as a name that matches nothing. One fixed
  line per kind: "You don't see them here." (creatures), "You don't see that
  here." (things).
- A name the player holds or knows echoes it: their inventory and equipment,
  exits, commands, skills and spells. "You aren't carrying 'rope'." The echo
  is the player's own input, trimmed and with ANSI tags stripped, so typed
  markup cannot reach the line.

### 3. Hints

A helper adds a hint only where a next step exists, from its keywords:
"You have no target. Try 'attack <name>'." / "Pet what? Try 'pet <name>'."
Hints name a command the player can type, in single quotes, and fit the line
in 80 visible columns. Cooldowns name the move and never a duration.

### 4. Failures

`Failed(u, action, err)` logs the error with its cause and the player, and
tells the player plainly: "That didn't work. The error has been logged."
(`action` names what failed when it helps: "That trade didn't work..."). It
sends on `CategoryError`. The two `behaviortree/actions_mob.go` sites are
checked first: if they reach no player, they become logs only.

### 5. Categories

Appended to the enum, each with a `String()` case and an alias at the same
colour as System (`8`), so nothing looks different until the owner recolours
one. All three take every normalize stage and never wrap, exactly as System
does today (`skipStages` default, absent from `shouldWrap`):

| Category | Alias | Carries |
|---|---|---|
| `CategoryRefusal` | `refusal` | every refusal and prompt (sections 1 to 3) |
| `CategoryUsage` | `usage` | "Usage:" lines and the `*Usage(user)` blocks |
| `CategoryAdmin` | `admin` | output of admin-only commands (`admin.*.go`) |

`CategorySystem` keeps reports, tables, help and confirmations of the
player's own action ("You pick up the sword."). It stays a mixed bucket by
design: those strings are one audience, not refusals, and are already owned
by their commands. `CategoryError` carries internal failures (section 4);
`CategoryWarning` and `CategoryTip` keep their current meaning, recorded in
`internal/messaging/context.md`.

Narration found tagged `CategorySystem` (a line about a world event, or one
other players would see; the inventory found two in a 61-site sample, for
example `usercommands/throw.go:275`) moves to the right narration category,
or is filed if moving it is not mechanical. `special_move_categories_test.go`
already allows some of these on purpose; each is checked against its
allow-list before it moves.

### 6. Sweep method

The remaining refusals and the retag are found per package by a read-only
classifier (Python under `tools/`, reads Go source, never writes) that lists
every `CategorySystem` send with a proposed class (refusal, prompt, usage,
admin, report, confirmation, narration, failure). Conversion is mechanical
for usage and admin (by file and by the "Usage:" marker). Refusal rewording
goes to the owner on a review page per slice, the same way M6's copy did;
retagging that does not change text needs no page.

### 7. Guards

- **Refusal phrases**: a root guard fails if a refusal phrase from the
  families above ("You don't see", "You have no target", "You aren't
  carrying", "You don't have", "Something went wrong", "what?" as a whole
  prompt, the cooldown and membership lines) appears in a Go string literal
  outside `internal/refusal` and `_test.go` files. Phrases are listed in one
  table; each needs a probe proving the guard can fail.
- **No echo through sight**: a test drives the sight-gated helpers with a
  typed name and proves the line never contains it, and that an unseen
  creature and a missing name produce the same line.
- **Tags** (final slice): `admin.*.go` sends no `CategorySystem`; a line
  containing "Usage:" is sent on `CategoryUsage`; the two existing
  per-file category tests are folded into the new guard.

## Out of scope

- Localization: no translations are planned; the helpers are the natural seam
  if they ever are. Recorded, not done.
- Help text: belongs to the help discoverability audit; help stays on
  `CategorySystem`.
- #281 (commands dropped inside TurnMs): input handling.
- Reports, tables and confirmations get no rewording here.

## Suggested slicing (the plan decides)

1. Package, `CategoryRefusal`, echo and hint rules, the families with the most
   repeats (not here, not carrying, no target, cooldown, membership, while
   fighting), `Failed`, and the phrase and echo guards.
2. Prompts (`Prompt`), with usage hints.
3. The remaining refusals, package by package, with review pages.
4. `CategoryUsage`, `CategoryAdmin`, narration moved out of System, the tag
   guard.

## Testing

- Unit tests per helper (text, echo, hint, no numbers).
- The sight test above, in the dark and with shapes-only sight.
- The narration snapshot golden for any line it covers.
- Boot check per slice.
- A playtest at the end of slices 1 and 3: common refusals in light and dark,
  wrong names, cooldowns, a failed trade, prompts.

## Risks

| Risk | Answer |
|---|---|
| A new category changes how a line looks or wraps | Same alias colour, same stages, no wrap; `TestSystemNeverWraps` gets siblings for the three new categories |
| An echo leaks who is in the room | Section 2: sight-gated helpers never echo; the echo test |
| The phrase guard blocks a legitimate non-refusal string | Phrases are whole refusal phrases, not single words; a hit outside the package is either converted or its phrase narrowed in the table, never allow-listed by file |
| 2110 sites is too many to review | Only reworded refusals go to the owner; retagging is mechanical and proven by tests |
