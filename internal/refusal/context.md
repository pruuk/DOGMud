# refusal: Package Documentation

## Overview

One helper per refusal situation (messaging M7, #371): the line a player
reads when a command cannot do what they asked. Before this package the same
refusal was written a dozen ways ("You don't have a "rope" to drink.", "You
don't have that item.", "You don't seem to be carrying that."). Now each
situation has one helper and one line, sent on `messaging.CategoryRefusal`
(treated exactly as `CategorySystem`; alias `refusal: 8` in
`ansi-aliases.yaml`). `Failed` is the exception and sends on `CategoryError`.

Every helper has a `...Text` sibling, or a `...Line` constant, that returns
the line without sending it. Use those where the caller returns a line
instead of sending one (`actions.AimBySight` returns
`NothingToAimLine` / `NotHereCreatureLine`).

## Recipient

`Recipient` is `interface { SendText(cat messaging.Category, msg string) }`.
`*users.UserRecord` and every `actions.Actor` satisfy it (a mob actor's
`SendText` does nothing, so a helper called for a mob is silent).

## The echo rule

A refusal may repeat what the player typed only when the name is theirs or
one they already know. A name they can only find by looking is never echoed,
because an echo would tell a blind or shape-sighted player which names exist
in the room.

- Sight-gated helpers (`NotHereCreature`, `NotHereThing`, `NothingToAim`)
  take no name at all, so they cannot echo.
- `NotCarrying` echoes (the player's own belongings), through `echo`: ANSI
  tags stripped, trimmed, truncated by display width to 24 columns plus
  `...` (`runewidth.Truncate`, so wide characters count as two columns and
  the longest echo still fits 80 columns), then leftover markup escaped with
  `util.EscapeAnsiTags` so typed input cannot style the line.
- The normalizer runs on refusal lines exactly as it ran on `CategorySystem`,
  so an echoed "a apple" reads "an apple" and "rope rope" reads "rope". That
  is accepted, not a regression.

Hints use the command colour: `hint` appends ` Try <ansi fg="command">x</ansi>.`
(owner ruling 2026-10-15; not single quotes).

## Helpers

| Helper | Line |
|---|---|
| `NotHereCreature` / `NotHereCreatureLine` | You don't see them here. |
| `NotHereThing` / `NotHereThingLine` | You don't see that here. |
| `NothingToAim` / `NothingToAimLine` | You can't see anything to aim at. |
| `NotCarrying(u, "rope")` / `NotCarryingText` | You aren't carrying 'rope'. Try **inventory**. |
| `NotCarrying(u, "")` | You aren't carrying that. Try **inventory**. |
| `NoTarget(u, "kick")` / `NoTargetText` | You have no target. Try **kick &lt;name&gt;**. |
| `Cooldown(u, "kick")` / `CooldownText` | You need a moment to recover before you can kick. |
| `WhileFighting(u, "steal")` / `WhileFightingText` | You can't steal while fighting. |
| `NotInGuild` / `NotInGuildText` | You aren't in a guild. Try **guild create &lt;TAG&gt; &lt;name&gt;**. |
| `NoCompanion` / `NoCompanionLine` | You have no companion travelling with you. |
| `NotPartyLeader(u, "set the loot mode")` / `NotPartyLeaderText` | Only the party leader can set the loot mode. |
| `NoPartyInvite` / `NoPartyInviteLine` | You have no party invitation to answer. |
| `RecipientFull` / `RecipientFullLine` | They can't carry any more. |
| `Failed(u, "", err)` / `FailedText` | That didn't work. The error has been logged. |
| `Failed(u, "salvage", err)` | That salvage didn't work. The error has been logged. |

## Prompts

A prompt answers a command typed without what it needs (slice 2).
`Prompt(to, verb, ask, usage)` / `PromptText` writes `<Verb> <ask>?` with the
verb capitalized, then ` Try <usage>.` in the command colour when `usage` is
not empty. `verb` carries any preposition ("talk to", "steal from", "plant
on"); an empty verb asks the bare question. `ask` is one of a fixed set, so
no caller writes a question literal:

| `Ask` | Example |
|---|---|
| `What` | Pet what? Try **pet &lt;name&gt;**. |
| `Whom` | Steal from whom? Try **steal &lt;name&gt;**. |
| `HowMuch` | Deposit how much? Try **bank deposit &lt;amount&gt;**. |
| `HowMuchGold` | Drop how much gold? Try **drop &lt;amount&gt; gold**. |

The special moves get theirs from `actions.StageMeleeTarget`
(`MeleeTargetOpts.PromptVerb`, default `Verb`): "Kick whom? Try **kick
&lt;name&gt;**.". `TalkToPlayer` / `TalkToPlayerLine` answers `talk` aimed at
a player: "To speak with a player, try **say &lt;message&gt;**.".

A name that was typed and matched nothing is a miss, not a prompt: it gets
`NotHereCreature` or `NotHereThing`, never a question back. Interactive
questions that wait for an answer (`cmdPrompt.Ask`, e.g. mail's "Send mail
to whom?") are not prompts in this sense and stay where they are.

`Cooldown` takes what the player tried ("kick", "consider anyone"), never a
duration. `NotPartyLeader` with an empty action reads "do that".

## Failed

`Failed(to, action, err, logArgs...)` is for server faults only: the player
did nothing wrong and the server could not do the job. Never use it for a
player's mistake. The party invite case (no invitation to answer) and the
full-recipient case (`actions.ErrInventoryFull`) are refusals with their own
helpers (`NoPartyInvite`, `RecipientFull`), not failures. So is `give 0 gold
<name>`: `Give` answers a zero or negative amount with "Give how much gold?"
on `CategoryRefusal` before any path that could reach `Failed`.

It logs through `mudlog.Error` with the action, the error, the calling
`file:line` (`runtime.Caller(1)`) and the extra args (pass `"userId", id`),
then sends `FailedText(action)` on `CategoryError`. Because it records its
caller's line, do not wrap `Failed` in a local helper: every log line would
point at the wrapper. Call it directly at each site.

## Guards

Two root-level tests keep the package the only source of these lines.

- `refusal_phrase_guard_test.go` (`TestRefusalPhrasesLiveInThePackage`)
  walks every non-test Go file under `internal/` and `modules/` (skipping
  `internal/refusal/`) and fails on any string literal matching a row of
  `refusalPhrases`. `TestRefusalPhraseGuardCanFail` runs the scanner over a
  source holding every row's `probe` and requires one hit per row, so a
  broken pattern cannot pass silently.
- `aim_refusal_category_guard_test.go` (`TestAimRefusalsRideCategoryRefusal`)
  finds each `AimBySight` call, follows the identifier its refusal is
  assigned to, and requires every `SendText` of it in that function to use
  `CategoryRefusal`. `TestAimRefusalGuardCanFail` proves it can fail.

The rows cover these families: not here (including "Nothing by that name is
in this room" and "nobody like that here to ask"), not carrying, no target
(including "Your target is gone"), cooldown (including "You need a moment
before you can ..."), guild, companion (including "You have no companion
to ..."), party leader, in combat (including the generic "can't do that while
fighting" gate), failure, "Who???" and prompts. The prompt row matches a
literal opening with a question (`Pet what?`, `Talk to whom?`, `Bid how
much?`); interactive `cmdPrompt.Ask` questions do not match it. The "nobody
like that here" row is narrowed to
"to ask": the companion module's "to cast it at" text is a trace reason for
the model, not a line a player reads.

To add a phrase row: append `{family, regexp, probe}` to `refusalPhrases`,
where `probe` is a real old-style line the regexp must match. When a hit
fires on text that is not that refusal, narrow the regexp. Never allow-list
a file; a hit is either converted to the helper or the pattern is narrowed.

Behaviour tests: `internal/usercommands/refusal_echo_sight_test.go` (no
echo through sight) and `internal/usercommands/refusal_commands_test.go`
(command-level lines).

## Left for later slices

Slices 3 and 4 of #371 move the remaining refusal families into this
package (`WhileBusy`, `TargetAsleep`, `Cost`, `NotWearing`, then package by
package), add their phrase rows, and add `CategoryUsage`, `CategoryAdmin`
and the tag guard.
