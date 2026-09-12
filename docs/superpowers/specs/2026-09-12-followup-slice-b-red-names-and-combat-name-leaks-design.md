# Follow-up slice B: red names, and two combat lines that still name the unseen

Slice B of the messaging M3 item 5a follow-ups (order ruled 2026-09-11: A, F,
B, C, D+E). Slice A shipped in PR #120 and slice F in PR #121. This slice takes
the owner's item 1 (red names) and the two slice A gaps slice F's playtest found
on the same combat round: the return-damage "recoil" lines and the
"You turn your attention to" retarget notice, both direct SendText calls that
never reach the seam that hides names.

## Facts verified against source (2026-09-12, master `a9459ab3e`)

| Fact | Where |
|---|---|
| The `aggro` suffix is chosen by `c.CurrentCombatTarget().UserId == viewingUserId`, nothing else | `internal/characters/formattedname.go`, `getFormattedName` |
| A character with no combat phase reports `state.ActorRef{}`, so its target UserId is 0 | `internal/characters/character.go:848` |
| `GetCharacterName(true)` always renders for viewer 0 | `formattedname.go`, `GetCharacterName` |
| Viewer-0 render sites today: 22 `GetCharacterName(true)`, 15 `mobDisplayName(..., 0)`, 7 `GetMobName(0`, 4 `GetMobNameIndexed(0` | grep over `internal/` and `modules/` |
| `username-aggro` is colour 196 and `mobname-aggro` colour 9 in dogmud; dup variants 197/203/167 | `_datafiles/world/dogmud/ansi-aliases.yaml:7-19` |
| Recoil lines: one room broadcast plus two direct `SendText` calls, tokens from `mobDisplayName` | `internal/hooks/NewRound_DoCombat_unified.go:436-484`, single caller `:402` |
| Retarget notice: four direct `SendText` calls at three sites, all naming the target. The facts table first said two; review found the other two | `internal/hooks/NewRound_DoCombat.go:137,139`, `emitRetargetMessage` `NewRound_DoCombat_unified.go:999-1016`, and `internal/mobcommands/go.go` `clearRoomAggroOnDeparture` (player-target and companion-target branches) |
| `RetargetOrEnd` picks whoever is already attacking you; it has no sight check and slice F left it so | `internal/hooks/combat_retarget.go:69` |
| The seam: `messaging.SendTrio` hides the other party's name per reader by `Room.ParticipantSight`; slice A's `sendCritEffectTrio` is the in-package template | `internal/messaging/trio.go:98`, `internal/hooks/crit_effect_trio.go` |
| `hideTaggedName` swallows a whole identity tag and its ` #N` dup index, and nothing after the closing tag | `internal/messaging/hidenames_tagged.go:27`, `identityCloseAfterName` in `hidenames.go` |
| `FormattedName.String` prints adjectives AFTER the identity tag as `` <ansi fg="black-bold">(dead)</ansi>`` | `formattedname.go`, `String` |
| Hooks test fixture: users 1 "Aliceia" and 2 "Bobrick" in room 1, mob instance 100 "Skeleton"; `darken(t, 1)` makes room 1 an unlit cave | `internal/hooks/hooks_test.go:37`, `narration_testhelpers_test.go:74` |

## Defects

1. **Nearly every name is red.** Viewer 0 means "no particular reader", but the
   suffix test compares the character's combat target UserId (0 when there is
   no target, or when the target is a mob) with the viewer id (0). So every
   character not fighting a player renders with the `aggro` suffix in every
   viewer-0 line: room broadcasts, buff text, quest text, `GetCharacterName(true)`.
   Observed in play as bright red names for everyone.
2. **Recoil names the unseen.** Room 3112, unlit, veteran without night vision:
   `You recoil from striking Stone Beetle Queen (dead)!` while every other line
   in the fight said "something". The defender's twin line and the room line
   share the defect.
3. **Retarget names the unseen.** `You turn your attention to X!` prints the new
   target's name regardless of the reader's sight, at the round driver's two
   sites and (found by review) at the mob-departure retarget in mobcommands.
4. **Wait-round lines name the unseen** (found by this slice's playtest, added
   mid-execution). The authored `{source} watches you intently` lines from
   `combat.GetWaitMessages` were drained to the participants by
   `handleCombatWaitRound` with no sight step; `replaceDarknessMessages` is
   swing-event based and does nothing for a wait round.

## Design

**Fix the primitive, not the call sites.** The `aggro` suffix requires a real
viewer: `viewingUserId > 0 && target.UserId == viewingUserId`. No viewer-0 site
changes. A room broadcast has no single reader, so it cannot know who the
character is hostile to, and now says so by rendering plain.

**Recoil goes through `SendTrio`,** the way slice A moved the crit lines. The
attacker is the Actor (they recoil), the defender the Actee. Tokens stay as
they are, so lit rooms keep the duplicate index and adjectives; the seam hides
each name for a reader who cannot see that party.

**The seam swallows the adjective span too.** After hiding a whole identity
tag, `hideTaggedName` also removes one directly following
`` <ansi fg="black-bold">(...)</ansi>`` span, so an unsighted reader reads
"something", not "something (dead)". This is what makes formatted names safe to
pass through the seam at all; the recoil line is the first caller that does.
Two corrections after review: the adjective is colour-patterned RUNE BY RUNE by
`CompileAdjectiveSwaps`, so the span body must admit nested colour tags, and
`Anonymize` needs the same rule because the room path anonymizes before it
hides names. One shared `adjectiveSpanBody` serves both patterns.

**Wait-round participant lines are name-hidden per reader** by
`hideForParticipant` in `handleCombatWaitRound`, the rule `SendTrio` applies to
its actor and actee lines. The room lines were already sight-gated.

**One retarget notice builder,** `actions.RetargetNotice(room, userId, target)`
in `internal/actions/retarget_notice.go`, used by all four calls at three sites
(it lives in `actions` because nobody may import `hooks` and both `hooks` and
`mobcommands` already import `actions`), hiding the name by the reader's
`ParticipantSight`. The notice
is not suppressed: the new target is already attacking the reader, and melee
in the dark still swings ("Something strikes you in the dark!"), so
"You turn your attention to something!" is the honest line.

## Out of scope

- Passing a real viewer at the viewer-0 sites so the aggro colour returns for
  the one reader it is true for. That is an enhancement per site, not this bug.
- Mob-side retargeting: mobs have no client.
- GMCP darkness leaks (own slice, filed).

## Gates

Unit tests per task, the three root narration guards, full `go test ./...`,
gofmt, `golangci-lint --new-from-rev=master`, the isolated-worktree boot check,
and one playtest lane: a dark fight in room 3112 against the Stone Beetle Queen
(she deals return damage) confirming the recoil line reads "something", plus a
lit `look` and a lit fight confirming names are no longer red for a bystander.
