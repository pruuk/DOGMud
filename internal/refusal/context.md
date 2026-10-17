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
- `NotCarrying`, `NotWearing`, `NotInStorage` (the player's own belongings)
  and `NotPartyMember` (their own party) echo, through `echo`: ANSI
  tags stripped, trimmed, truncated by display width to 24 columns plus
  `...` (`runewidth.Truncate`, so wide characters count as two columns and
  the longest echo still fits 80 columns), then leftover markup escaped with
  `util.EscapeAnsiTags` so typed input cannot style the line. An empty
  echo reads "that".
- `Echo(typed)` exports the same echo for lines outside the package that
  repeat what the player typed about their own things: a companion name
  (`companion.go`, `dismiss.go`), an alt (`character.go`), a recipe (`craft.go`), a spell
  (`skill.cast.go`, `cancel.go`), the pet's pack (`get.go`), a setting
  (`set.go`, `sethome.go`). Never pass it a name the player could only find
  by looking (a creature, a floor item): those lines take no name at all.
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
| `NoTargetAfterCost(u, "taunt", cost)` / `NoTargetSpentText` | Your target is gone before you can taunt, and the effort is spent. (NoTarget when nothing was charged) |
| `CooldownAfterCost(u, "kick", cost)` / `CooldownSpentText` | You need a moment to recover before you can kick, and the effort is spent. (Cooldown when nothing was charged) |
| `WhileFighting(u, "steal")` / `WhileFightingText` | You can't steal while fighting. |
| `NotInGuild` / `NotInGuildText` | You aren't in a guild. Try **guild create &lt;TAG&gt; &lt;name&gt;**. |
| `NoCompanion` / `NoCompanionLine` | You have no companion travelling with you. |
| `NotPartyLeader(u, "set the loot mode")` / `NotPartyLeaderText` | Only the party leader can set the loot mode. |
| `NoPartyInvite` / `NoPartyInviteLine` | You have no party invitation to answer. |
| `RecipientFull` / `RecipientFullLine` | They can't carry any more. |
| `WhileBusy(u, "change equipment")` / `WhileBusyText` | You can't change equipment while you're busy. Try **cancel**. |
| `WhileUnderAttack(u, "steal")` / `WhileUnderAttackText` | You can't steal while under attack. |
| `TargetAsleepText("Mira")` (no send) | **Mira** is fast asleep. You could make some noise. Try **shout wake up**. |
| `Cost(u, res)` / `CostText` (stamina refused) | You are too spent to manage that right now. |
| `Cost(u, res)` / `CostText` (conviction refused) | You cannot muster the resolve for that right now. |
| `NotWearing(u, "ring")` / `NotWearingText` | You aren't wearing 'ring'. Try **inventory**. |
| `NotInStorage(u, "rope")` / `NotInStorageText` | You don't have 'rope' in storage. Try **storage**. |
| `NotInStorage(u, "")` | You don't have that in storage. Try **storage**. |
| `NotInParty` / `NotInPartyText` | You aren't in a party. Try **party create**. |
| `NotGuildLeader(u, "disband the guild")` / `NotGuildLeaderText` | Only the guild leader can disband the guild. |
| `NotGuildOfficer(u, "invite")` / `NotGuildOfficerText` | Only guild officers and the leader can invite. |
| `CompanionsFull` / `CompanionsFullLine` | You are already sustaining as many companions as your will can hold. |
| `NotPartyMember(u, "Bob")` / `NotPartyMemberText` | No one in your party is called 'Bob'. |
| `Failed(u, "", err)` / `FailedText` | That didn't work. The error has been logged. |
| `Failed(u, "salvage", err)` | That salvage didn't work. The error has been logged. |

`WhileBusy` covers casting, crafting and salvaging alike (`cancel` ends any
of them); `usercommands.refuseWhileBusy` and the special moves send it.
`NotGuildLeader` and `NotGuildOfficer` with an empty action read "do that".

`TargetAsleepText` has no send of its own: it names the sleeper, and its
caller (`actions.RefuseIfAsleep`) hides the name at the player's sight
before sending, so a sleeper the player cannot see is not named.

`Cost` sends nothing unless the cost was refused (`CostText` is empty for
any other status, and for a pool with no refusal line), so a caller passes
every `CostCommitResult` through it without checking first. A mob actor's
`SendText` does nothing, so mob wrappers stay silent.

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

Three root-level tests keep the package the only source of these lines and
keep every refusal on `CategoryRefusal`; four more, below, cover slice 4.

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
- `refusal_system_opener_guard_test.go`
  (`TestRefusalOpenersNeverRideSystem`, slice 3) walks every non-test Go
  file under `internal/` and `modules/` (`openerGuardScope`), finds each
  call passing `messaging.CategorySystem`, reads the argument after it (a
  string literal, the format of `fmt.Sprintf`, or the left end of a `+`
  concatenation), strips tags and fails when it matches `refusalOpener`
  ("You can't", "You don't have", "You aren't", "You need to", "Only the",
  "Only officers" and the like). A line holding `, but ` is a partial
  success ("You don't have that much gold on hand, but what you do have you
  deposit."), not a refusal, and passes. `TestRefusalOpenerGuardCanFail`
  feeds the finder each send shape and requires the hits it should find.
  A hit is sent on `CategoryRefusal` or `refusalOpener` is narrowed; never
  allow-list a file. Lines no opener can tell from a report were ruled one
  by one in the slice 3 ledger
  (`docs/superpowers/audits/messaging-m7-refusal-ledger.md`).

The rows cover these families: not here (including "Nothing by that name is
in this room" and "nobody like that here to ask"), not carrying, no target
(including "Your target is gone"), cooldown (including "You need a moment
before you can ..."), guild, companion (including "You have no companion
to ..."), party leader, in combat (including the generic "can't do that while
fighting" gate), failure, "Who???" and prompts. The prompt row matches a
literal opening with a question (`Pet what?`, `Talk to whom?`, `Bid how
much gold?`). Two more rows catch the "which" form (`Switch to which
target?`) and the would-you-like form (`What would you like to sell?`).
`refusalPhraseHits` skips any literal passed directly to a `.Ask(` call
(every receiver is `cmdPrompt`): interactive questions wait for the
player's answer and are not refusals. The "nobody
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

## Slice 4 guards

Four more root tests (M7 slice 4, #371) keep usage, staff output and
narration off `CategorySystem`. Each has a probe test that feeds the scanner
a planted source and requires it to fail where it should.

- `usage_category_guard_test.go` (`TestUsageRidesCategoryUsage`): a send whose
  text opens with "Usage:", and every send inside a `*Usage(user)` function,
  rides `CategoryUsage`. It sees literal-led sends and the `*Usage`
  functions; a usage block built through a template is converted from the
  ledger, not caught. Probe: `TestUsageGuardCanFail`.
- `admin_category_guard_test.go` (`TestAdminOutputRidesCategoryAdmin`): no
  function reachable only from an admin-only command sends on
  `CategorySystem`. The admin set is derived from the command registry
  (`userCommands` entries whose `AdminOnly` field is true, plus module
  `AddUserCommand` and `RegisterCommand` registrations), not from file names, because `bounty` and
  `reload` live in `admin.*.go` and are player commands. Reach is followed
  through same-package callees, and tells selectors, fields and literal keys
  from function names. It cannot see staff subcommands of player commands
  (party admin, bounty, reload, weather, `bauble_admin`); those were
  converted from the admin ledger with no guard. Probe:
  `TestAdminGuardCanFail`.
- `system_recipient_guard_test.go` (`TestSystemReachesOnlyTheActingPlayer`,
  rule N1): inside a command handler (every function reachable from a
  registered command), a `CategorySystem` send goes to a parameter of the
  enclosing function, never to a looked-up local, a range variable over other
  players or a room. A parameter whose type names a Room (`room *rooms.Room`)
  never counts as the acting player. Hooks are excluded: an event has no
  single acting player. What it cannot see: a send through a method not named
  `SendText*`, a category passed in a variable rather than written as
  `messaging.CategorySystem`, and a call into another package (reach follows
  same-package callees only). Probe: `TestSystemRecipientGuardCanFail`.
- `system_subject_guard_test.go` (`TestNarrationAboutYouNeverRidesSystem`,
  rule N2): a `CategorySystem` line that opens with another creature (a name
  tag, a bare `%s`, "Someone", "Something") and addresses "you" or "your" is
  narration of something done to the player and takes a narration category.
  Narrowed for `track`'s own report ("%s is in the room with you!") and the
  "Something glints" find. Probe: `TestSystemSubjectGuardCanFail`.

R4: the two per-file tests stay (`internal/usercommands/special_move_categories_test.go`
and `internal/mobcommands/throttle_categories_test.go`).
`TestSpecialMoveFilesKeepOnlyNoticesOnSystem` now requires that `taunt.go`,
`throw.go`, `rally.go` and `warcry.go` hold no `messaging.CategorySystem` at
all. The defence-shortage note ("You mount a desperate response ...") reaches
the defender, so at each site it rides the category of the defence line sent
with it. On the melee path that category is suppressible at Light and Medium,
so the note is marked `combat.TaggedMessage.AlwaysShow` and still shows at
every verbosity, as it did on System.

## M7 complete

M7 (#371) shipped in slices 1 to 4: #547, #549, #559 and the slice 4 PR.
`CategorySystem` now carries reports, tables, help and confirmations of the
player's own action; refusals, usage, staff output, group notices and
narration each have their own category.
