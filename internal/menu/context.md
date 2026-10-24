# menu: Package Documentation

## Purpose

`internal/menu` is DOGMud's one menu primitive: a numbered list of choices, how
it looks, and how a typed line resolves against it. It is pure. It holds no
session state and does no I/O. `internal/users` (game menus) and
`internal/prompt` (builder and player flows) supply the input sessions and
send the text. Part of the quest-arc phase 0 work (#375).

## Files

| File | What it holds |
|---|---|
| `menu.go` | `Entry`, `Menu`, the `Marker*` constants, `Render`, column layout, word wrap |
| `resolve.go` | `ResultKind`, `Result`, `Resolve`, `Confirm` |
| `lines.go` | `CloseReason`, `InterruptLine`, `FollowLines` |
| `lint.go` | `LintLabels` |
| `session.go` | `Session`, the callbacks a caller hands `internal/users` when it opens a game menu |

## Data model

`Entry` is one numbered choice:

- `Id`: returned on choice, and matched when typed exactly.
- `Marker`: `""` or one of `MarkerQuest` (`[!]`), `MarkerHandIn` (`[?]`),
  `MarkerInfo` (`[*]`), `MarkerChecked` (`[x]`). The symbol carries the
  meaning; colour only reinforces it.
- `Label`: natural language, one line.
- `Detail`: optional second line, indented under the label.
- `Dim`: render grey (heard info, small talk).
- `Aliases`: extra exact words that pick this entry ("y", "n").

`Menu` is a heading, entries and an optional final Leave entry:

- `Heading`, `Entries`.
- `Leave`: label of the final numbered leave entry; `""` for none.
- `LeaveAliases`: extra exact words that pick Leave, checked before the
  number branch (so a builder's "0" can mean done); ignored when `Leave` is
  `""`.
- `Default`: the `Id` chosen on Enter; `""` for none.
- `Footer`: overrides the computed footer.
- `Columns`: `2` lays name-only lists side by side (long admin lists). In this
  mode `Detail` is not shown.
- `Prefix`: a unique label prefix resolves (builders only).
- `AcceptText`: an unmatched line resolves as `Text` (free-text builders).

## Render

`Render(m)` draws at the 80-column wrap. Callers send the result on
`messaging.CategoryScreen` (player menus) or `CategoryAdmin` (builders).

- Each line is a 2-space margin, the right-aligned number, a 4-wide marker
  column, then the label. Numbers are padded to the width of the largest.
- `Detail` wraps at 80 columns under the label, indented past the marker
  column.
- `Dim` entries render grey (colour 8); every other label and `Detail` line
  renders in the `menu-label` alias (white, 15). Menus ride the grey
  `screen` and `admin` categories, so without the explicit colour an
  undimmed line inherits the same grey and Dim shows nothing (#375 phase 1
  playtest). Pinned on the telnet bytes by
  `TestRender_DimLineIsDimmerOnTheTelnetPort`.
- `Columns: 2` places entries in two 39-wide cells; labels are cut to fit and
  `Detail` is dropped.
- The footer is `Footer` when set; otherwise "Type a number, or press Enter
  for N." when `Default` names an entry; otherwise "Type a number, or press
  Enter to leave." when `Leave` is set; otherwise "Type a number."

## Resolve

`Resolve(m, input)` decides what a typed line meant and returns a `Result`
with a `Kind`, and `Entry`, `Matches` or `Text` where they apply.

- Game menus resolve only a number, Enter, or an exact label, id or alias
  (case-insensitive). A prefix resolves only when `Prefix` is set, because in a
  non-modal game menu "n" or "l" is a movement or look command and must stay a
  command.
- `AcceptText` turns an otherwise unmatched line into a `Text` result.
- Digits only count as a number: "+2" and "-1" are text.

`ResultKind` values: `NotAnAnswer` (a game menu closes and the line runs as a
command), `Chosen`, `Leave` (the Leave entry, or Enter on a menu with Leave and
no Default), `NoSuchNumber`, `Ambiguous` (several prefix matches, `Matches`
set), `Empty` (Enter with no Default and no Leave), `Text`.

## Confirm

`Confirm(question, yesLabel, noLabel)` builds the two-entry yes/no menu. The
ids are `yes` and `no`, with aliases "y" and "n", and `Default` is `no`, so
Enter takes the way out. For an action that cannot be undone the labels name
the action ("Yes, disband it" / "No, keep it"); otherwise pass "Yes" and "No".

## Close reasons and lines

- `CloseReason` says why an open menu closed: `CloseNone` (no rule holds),
  `CloseChosen`, `CloseLeft`, `CloseOtherCommand`, `CloseReplaced`,
  `CloseCombat`, `CloseBusy`, `CloseDead`, `CloseAsleep`,
  `CloseControlled`, `CloseQuitting`, `CloseRoomChange`, `CloseFollow`,
  `CloseContextLost`, `ClosePartnerLeft` (the NPC the menu talks with left
  the room or died).
- `InterruptLine(reason, topic)` is the one line the player reads when an
  interrupt closes their menu ("You break off your talk with Kael as the fight
  begins."; for `ClosePartnerLeft`, "Your talk with Kael ends because they
  left."), or `""` when the close is silent (their own choice, a command
  whose output speaks for itself, death, a follow pull).
- `FollowLines(follower, leader, topic)` returns the three lines for a leader
  pulling a follower out of a menu: to the follower, to the leader, to the
  room left behind. Names arrive formatted; the caller hides them per reader.
- `Session` carries `Topic` (names the conversation in the lines above),
  `TopicNames` (the names inside `Topic`, hidden per reader by the sight
  rules), `OnChoose`, `OnClose` and `StillValid` (re-checks the caller's
  context before a choice acts; nil means valid), `ShowLast` (send the menu
  after every line of the event pass, quest banners and rewards included;
  the menu is open for input at once) and `MobInstanceId` (the NPC the menu
  talks with; `Room.RemoveMob` closes the menu with `ClosePartnerLeft`).

## LintLabels

`LintLabels(m)` returns one problem string per breach of the rules a machine
can check: empty label, label over 50 characters, `Detail` over 70, an em or
en dash in a label or detail, an "Ask:" or "Task:" prefix, a duplicate label,
a duplicate `Id`, and a `Default` that names no entry. The Leave label is
linted like any other entry. Phase 1 wires it into the dialogue validator.

## Menu language conventions

These are the nine conventions of the phase 0 spec (section 8). The same list
lives in `.claude/skills/dogmud-player-copy/SKILL.md`.

1. An info label is what the player says, in the player's voice, usually a
   question. A quest entry's label is the quest's title, and its `Detail` line
   is what the player says: a new quest offers help ("Do you want help settling
   the feud at the well?"), a hand-in reports the result ("I've settled the
   feud at the well.").
2. Never name the NPC in the third person and never use narrator voice in a
   label or a player line.
3. One line each, at most 50 characters for a label and 70 for a `Detail` line
   (a `Detail` starts at column 9 or 10, so 70 always fits in 80).
4. ESL-clear: plain words, no idioms, no em or en dashes, no raw numbers for
   stats or rewards.
5. Specific, not generic. No two labels in one menu read alike.
6. Leave lines may be in character ("Goodbye.") and default to "Leave".
7. Confirms are two entries; for actions that cannot be undone, the entries
   name the action.
8. Speak in the NPC's manner where it fits: gruff and plain, educated and
   pretentious. Labels may match the register of whom the player is addressing.
9. Validators enforce the checkable rules: length, no dashes, no duplicate
   labels in one menu, no "Ask:" / "Task:" prefix (`LintLabels`).

## Who uses it

- `internal/users`: the game menu session (`OpenMenu`, `RouteMenuInput`, the
  interrupt gate). See `internal/users/context.md`.
- `internal/prompt`: `Question.Menu`, `AskMenu`, `AskConfirm` and
  `Question.Resolve` for builder and player flows. See
  `internal/prompt/context.md`.
- `internal/usercommands`: the admin builders render their numbered lists
  through this package, and destructive confirms use `Confirm`. Player
  commands draw theirs here too (`character.go`, `start.go`, `picklock.go`,
  `guild.go`, `mail.go`, `renameself.go`, `deletecharacter.go`), and
  `menu_options.go` builds the option lists that use `MarkerChecked`.
- `internal/conversation`: the `talk` menu and an offer's Accept / Leave card
  (`Confirm`), opened through `OpenMenu`, plus the `Close*` reasons that word
  a refused `talk` (quest arc #375 phase 1).
- `internal/dialogue`: `DialogueFile.Menu` builds the talk menu (markers
  `MarkerQuest`, `MarkerHandIn`, `MarkerInfo`; heard info `Dim`), and
  `ValidateWorld` runs `LintLabels` over each NPC's topic labels.
- `internal/actions`: `BreakOffMenuToFollow` speaks `FollowLines`.
- `modules/auctions`: renders its numbered list with `Render`.

## Guard

`TestMenuGuard_NoNumberedMenusOutsideTheMenuPackage`
(`internal/usercommands/menu_guard_test.go`) forbids, in non-test Go code
outside this package, a `"tables/numbered-list` template render and the copied
`restNum > 0 && restNum <= len(` range check. Comments are ignored.

## Gotchas

- Do not add a prefix or fuzzy match to game menus. It would swallow "n", "l"
  and other short commands that a non-modal menu must let through.
- `Render` knows nothing about the sender. Pick the message category at the
  call site.
- A menu that sets `Columns: 2` loses its `Detail` lines; put anything the
  player needs in the label or use the single-column layout.
