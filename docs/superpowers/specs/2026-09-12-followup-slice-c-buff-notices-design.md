# Follow-up slice C: every buff tells its holder when it starts and ends

Slice C of the messaging M3 item 5a follow-ups (order ruled 2026-09-11: A, F,
B, C, D+E). The owner accepted the buff tick off-by-one on one condition:
"as long as the player is notified the buff is expiring." This slice makes
that true for every buff, twice over: authored flavour for each one, and a
generic line underneath so nothing can be silent again.

Owner rulings, 2026-09-12: told when it ends (no advance warning); option 1
(generic fallback plus guard) AND option 3 (author the missing lines), not
the conversion of existing flavour to the generic line.

## Facts verified against source (2026-09-12, master `99555b5bb`)

| Fact | Where |
|---|---|
| 102 buff files in the dogmud world. 43 carry neither `start_user_text` nor `end_user_text`; 6 more carry one half (0 Meditating, 9 Hidden, 31 Empathic Shroud lack the end; 79 Warcry, 80 Rally, 93 Bloom Detox lack the start); 0 are `secret: true`. The spec first said 46 and 46; the content pass counted | `_datafiles/world/dogmud/buffs/`, grep |
| Start text is sent from the apply hook only when `StartUserText` or `StartRoomText` is non-empty, on `CategoryBuffApply`, through `textutil.SendPhaseText` | `internal/hooks/Buff_ApplyBuffs.go:75-120` |
| End text is sent from the player prune pass only when `EndUserText` or `EndRoomText` is non-empty, on `CategoryBuffExpire` | `internal/hooks/NewTurn_PruneBuffs.go:44-58` |
| The mob prune pass sends end ROOM text only; mobs have no user line | `internal/hooks/NewTurn_PruneBuffs.go:102-120` |
| `BuffSpec.Secret bool` has no yaml tag, so the key is `secret`; `VisibleNameDesc()` renders a secret buff as "Mysterious Affliction" | `internal/buffs/buffspec.go:102,145-150` |
| `conditions` lists a player's buffs through `VisibleNameDesc()` | `internal/usercommands/conditions.go:56` |
| Buff 0 Meditating is the logout timer AND the quit narration: `quit` only adds buff 0, whose start and trigger text are all a quitting player reads; pruning it logs the player out | `internal/usercommands/quit.go:16`, `internal/hooks/NewTurn_PruneBuffs.go:61` |
| 9 Hidden and 31 Empathic Shroud omit `end_user_text` by design (you must not learn when your cover lapsed) and both carry the `hidden` flag | their YAML comments |
| 79 Warcry and 80 Rally are applied through `Character.AddBuff`, which never queues `events.Buff`, so `Buff_ApplyBuffs` never runs for them and a start line on the buff cannot reach the holder. 93 Bloom Detox DOES reach `Buff_ApplyBuffs`: the drink path calls `user.AddBuff` (which queues the event) on the unscaled branch, so its `silent-start` flag suppresses a real generic line in favour of the drink's own purge narration. The spec first lumped all three together; review corrected it | `internal/characters/buffs.go:109` (no queue), `internal/users/userrecord.go:422` (queues), `internal/usercommands/drink.go:255-261, 283-286` |
| Buff specs load through `fileloader.LoadAllFlatFiles` from the configured world; test binaries load none | `internal/buffs/buffspec.go:265-283` |
| Boot-time validator precedent: `species.ValidateSpeciesBuffIds(buffs.HasSpec)` wired after buffs load | `main.go:1694`, `internal/species/species.go:350` |
| Root guards walk `_datafiles/world` with `filepath.WalkDir` and parse YAML directly, without the game's loader | `messaging_surface_guard_test.go:337` |
| Buff names are asserted canonical at load, so a name is safe to print | `internal/buffs/buffspec.go:274-278` |

## Defect

Forty-three buffs apply and expire in silence from the buff's own text, and six more are silent at one end. A
potion prints "You drink the Warrior's Brew." from the item and then nothing,
neither when the effect lands nor when it fades. Afflictions such as Nausea,
Paralysed, Terrified and Throttled lift without a word. Nothing catches a new
buff added without text.

## Design

**One door for the player line.** Two methods on `BuffSpec`:

- `StartUserNotice() string`: `StartUserText` if set; otherwise
  `"<Name> takes effect."`; empty for a secret buff.
- `EndUserNotice() string`: `EndUserText` if set; otherwise
  `"<Name> has expired."`; empty for a secret buff.

Two deliberate silences are honoured by flag, so they are visible choices in
the YAML rather than missing text: a buff with the `hidden` flag never
announces its end (a hider must not learn when the cover lapsed; room text
still goes out), and a buff with the new `silent-start` flag leaves the start
to whatever applied it (Warcry, Rally, Bloom Detox).

`Buff_ApplyBuffs` and the player prune pass call these instead of reading the
raw fields, and the "only if text is non-empty" gates test the resolved
notice, so the generic line reaches the player through the same
`SendPhaseText` call, on the same category, as an authored one. Room text is
untouched: it stays authored-only for players and for mobs. Trigger text is
untouched.

**The guard.** A root test, `buff_notice_guard_test.go`, walks
`_datafiles/world/dogmud/buffs/*.yaml` and fails, naming the file, when:

- a non-secret buff lacks authored `start_user_text` (unless flagged
  `silent-start`) or `end_user_text` (unless flagged `hidden`); the generic
  line is a runtime net, never the shipped experience;
- a secret buff carries any of the six text fields;
- a non-secret buff has an empty `name` (the generic line would be blank).

At boot, `buffs.WarnSilentNotices()` logs one `mudlog.Warn` per non-secret
buff relying on the fallback, wired next to `ValidateSpeciesBuffIds`. It
warns rather than panics: the fallback exists so play continues, and the
root test is the gate that blocks a merge.

**The fallback is proven by unit tests** with synthetic specs (no name,
secret, authored, missing), because once the content lands no shipped buff
exercises it.

**Content.** Forty buffs get authored `start_user_text` and
`end_user_text`, and Meditating gets an end line in the player-copy style: 80 columns, second person, no
numbers, ESL-clear. Potions read as effects, not labels ("Heat spreads
through your limbs as the Warrior's Brew takes hold." / "The Warrior's Brew
fades, and your limbs feel ordinary again."). Afflictions read as relief
("The nausea passes."). Death Recovery and Death's Shadow stay visible and
get real text. Room text is added only where a bystander could see something
(a glow, a shudder), and never for potions.

**Three buffs become `secret: true`** and stay silent by design, which also
removes them from `conditions`, where they have no business showing:
81 Respawn Grace, 85 InfraredVision (the synthetic shapes-only buff), 99 Alt
Character Mob (mob-side gear lock). Meditating was on this list until the
content pass found it is the quit narration; it stays visible.

## Two mechanisms the playtest and the reviews forced into the slice

**The delivery path.** `UserRecord.AddBuff` queues `events.Buff`; the
character-level `AddBuff` / `AddBuffScaled` do not, so a player buff applied
through them lands in silence whatever the spec says. The playtest caught
Purging Weakness doing exactly that from the drink path. Fix at the primitive:
`events.Buff` carries `DurationMult`, `UserRecord.AddBuffScaled` queues it,
`Buff_ApplyBuffs` applies scaled, the four drink sites use it, and a second
root guard, `buff_apply_path_guard_test.go`, fails the build on any direct
character-level player add in `usercommands` or `actions` without an
allowlisted reason (Warcry and Rally party members, the throttle move, one
mob). Throttled (89) is flagged `silent-start`: the move narrates it and must
apply synchronously.

**Room mutator buffs refresh instead of lapsing.** `ApplyBuffIdToPlayers` (and
the two mob appliers) skipped players who already held the buff, so a
`triggercount: 3` buff expired and was re-added every third round; with
authored lines that narrated its end and start in a loop (crash-site
interior, the post-death waiting room). `Buffs.RefreshBuff` tops up the
triggers and touches nothing else (a plain re-add resets the tick counter and
would starve any buff with an interval above one round, and would demote a
permanent buff). Consequence the owner should know: the discharge rooms
(buffs 94, 96) now tick every round instead of three rounds in four.

## Out of scope

- The accepted tick off-by-one (buffs narrate one trigger fewer than
  `TriggerCount`; the player tick skips the final tick's effect).
- Mob buff end text, which only ever went to the room.
- Buff room lines that are sounds or smells being silent in the dark
  (accepted 2026-09-11).
- The 5b migration of buff text onto the narration core; this slice keeps
  today's send paths and only changes what they are given.

## Gates

Unit tests for the resolver; the new root guard green against the finished
content; the full suite; gofmt; `golangci-lint --new-from-rev=master`; the
isolated boot check with zero silent-notice warnings; and, because this is
content, an adversarial playtest: drink a potion and read both lines, take an
affliction and read its end, confirm the three secret buffs say nothing and
no longer appear in `conditions`, and `quit` still narrates the meditation.
