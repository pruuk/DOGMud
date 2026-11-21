# M5: the quality pass, the wrap decision, and crime in the dark

Stage M5 of the messaging unification arc. Written 2026-09-22 against master
`a5613be95`, after M4e PR 1a (#152) and PR 1b (#153) merged.

Arc spec: [`2026-08-31-messaging-unification-design.md`](2026-08-31-messaging-unification-design.md),
section "M5 Quality pass" (`:610`).

M5 is the arc's first stage since M0 whose items are mostly unrelated to each
other. It is a punch list, not a refactor. The one item with real design in it
is the crime gate, and the one item that grew on contact with the tree is the
wrap decision.

---

## Facts verified against source

Every row was read from the tree on 2026-09-22 at `a5613be95`. Rows marked
CORRECTS overturn something an existing arc document asserts.

| # | Fact | Evidence |
|---|------|----------|
| 1 | The wrap stage never fires. `shouldWrap` returns `false` for every category | `internal/messaging/pipeline.go:97-99` |
| 2 | It is pinned off by a test that loops every category | `internal/messaging/pipeline_test.go:51-56`, `TestShouldWrapDisabledByDefault` |
| 3 | The only production caller of `WrapAnsi` bypasses the pipeline entirely | `internal/usercommands/motd.go:44` |
| 4 | `UserRecord.LineWidth` is plumbed into `RenderInput` and then ignored | `internal/users/userrecord.go:486-500`, `pipeline.go:82` |
| 5 | A second, disconnected width exists, fed by telnet NAWS, read only by two admin commands | `internal/connections/clientsettings.go:4-31`; `admin.item.go:109`, `admin.mob.go:170` |
| 6 | 61 `messaging.Category` values are declared | `internal/messaging/messaging.go:20-109` |
| 7 | 🔴 CORRECTS the wrap ruling. `CategorySystem` carries BOTH one-line refusals AND the score sheet, inventory, `who`, help, admin `DynamicList` tables and the ASCII map | tables: `status.go:14`, `inventory.go:445`, `who.go:16`, `help.go:93`, `skill.map.go:71,212`, `admin.item.go:109`; refusals: `cast_admission.go:64,83`, `defuse.go:65,71,81,87` |
| 8 | `CategoryBroadcast` likewise carries both the hand-drawn MOTD box and free channel chat | `motd.go:77` and `ChannelMessage_SendToAll.go:30` |
| 9 | Tables do NOT bypass the pipeline. They reach `RenderForRecipient` and are protected today only by fact 1 | `internal/users/userrecord.go:486-500` |
| 10 | 🔴 `WrapAnsi` loses color across a balanced nested tag. It tracks one scalar `openTag` and clears it on any close | `internal/messaging/wrap.go:45`, `:85`. Reproduced 2026-09-22, see "The WrapAnsi probe" |
| 11 | `applyCategoryColor` wraps every non-default category in an outer ansi tag, so fact 10 fires routinely, not rarely | `internal/messaging/pipeline.go:111-116` |
| 12 | `WrapAnsi` counts bytes, not runes. The test oracle `displayWidth()` shares the bug | `wrap.go:116-117`; `wrap_test.go` helper. Compare `hidenames.go:70` which decodes runes correctly |
| 13 | 74 tips ship, 64 exceed 80 characters with the `[Tip] ` prefix, longest 211 | measured from `_datafiles/world/dogmud/tips.yaml` |
| 14 | Prior art for per-category switching is `skipStages`, a bitmask over an explicit list | `internal/messaging/normalize.go:26-37`; second example `verbosity.go:53-70` |
| 15 | Speech partly bypasses Category entirely. `SendTextCommunication` takes no category and never calls `RenderForRecipient` | `internal/rooms/rooms.go:265-274`, deliberate per `:257-264` |
| 16 | Speech self-echo is already wrapped at a hardcoded 80, ignoring `LineWidth` | `say.go:39`, `shout.go:63`, `reply.go:37`, `whisper.go:56` |
| 17 | Four categories have zero production sends: `GrappleHigh`, `Login`, `OOC`, `Toxin` | `messaging.go:40`, `:86`, `:72`, `:103` |
| 18 | 🔴 CORRECTS the arc spec and the M6 ledger. Of the 17 bare `_plain` condition lines, 15 were fixed on 2026-09-21 | `39b75fe07`; `Condition_ApplyConditions.go:170`, `NewRound_UserRoundTick.go:302`, `NewRound_MobRoundTick.go:287` |
| 19 | 🔴 CORRECTED while planning. **One** leaks, not two. `NewTurn_PruneConditions.go:130-138` passes no names, so neither End line can hide a name, but only `conditions/9-hidden.yaml:15` reaches a reader who should not see it | see row 19a |
| 19a | `conditions/1-illumination.yaml:12` is safe BY CONSTRUCTION, not by design. It carries `EmitsLight`, so it goes through `SendTextVisualAsLit`, which judges sight against `litRoom{}` (`rooms.go:345`, visibility hardcoded to 1). `ParticipantSight` therefore returns `SightFull` for anyone unblinded and `SightNone` otherwise, so `SightShapes` is unreachable on that path, and `HideNames` runs only at `SightShapes` (`rooms.go:370`). The safety is accidental and undefended: give the lit path a shapes tier and every light condition with a bare token leaks at once | `rooms.go:338-345`, `:362-376`; `predicates.go:51-68` |
| 20 | 🔴 NEW, in no document. `throw.go` builds one Audience with `ActeeName: NoName`, but `player_cast_interrupt` renders a real mob name into `{actee_plain}` | `usercommands/throw.go:290-296`, `:363-366`; `throw.yaml:26,28`. `NoName` is treated as "nobody to hide" at `trio.go:127` and `hidenames.go:48` |
| 21 | `Anonymize` strips identity TAGS only and says so in its own docstring | `internal/messaging/anonymize.go:25-31` |
| 22 | The guard for this class already exists in one store. Quest observer lines refuse `{actor_plain}` at load | `internal/quests/roomtext.go:32-34` |
| 23 | Only 16 shipped files use a `_plain` token: 14 conditions plus `grapple.yaml` and `throw.yaml`. Spells, quests, recipes, combat-messages, defense-messages and taunt-messages use none | exhaustive grep over `_datafiles` |
| 24 | 🔴 CORRECTS "world/default template shadowing". There is NO production shadowing. `DataFiles` is one path with no fallback and `world/default` is never read in play | `internal/configs/config.filepaths.go:22-24`; `_datafiles/config.yaml:232` |
| 25 | The defect is test-side. The whole `internal/templates` test binary is pointed at `world/default` | `internal/templates/process_test.go:27`, `:34`, `:40` |
| 26 | Flipping that const to dogmud breaks exactly two tests | probed 2026-09-22: `TestProcess_CharacterSkillsTemplate`, `TestU8CrossReferenceValidationRejectsMissingDOGMudOnlyTopic` |
| 27 | `context.md` documents wrap as a live stage. Its phantom claims are behavioral, not symbol level, so `tools/context_md_audit.py` is structurally blind to them | `internal/messaging/context.md:21-22`, `:259`; audit reports 0 phantom symbols |
| 28 | Crime witnessing consults no sight, no lighting and no sleep. A witness is any mob whose `Groups` overlap the victim's factions | `internal/crimes/crimes.go:198-228` |
| 29 | Witnesses are MOBS, never players. `WitnessesInRoom` walks `room.GetMobs()` | `crimes.go:198` |
| 30 | `PerpUnknown` already exists and is already returned when there are no witnesses | `crimes.go:232-237` |
| 31 | `crimes` importing `messaging` creates no cycle. `messaging` imports nothing back toward `crimes`, `rooms`, `mobs`, `actions` or `hooks` | `internal/messaging/predicates.go:1-7` |
| 32 | `CanSeeClearly` and `CanSeeShapes` already compose sleep through `awake()`. `ParticipantSight` deliberately does not | `predicates.go:70-75`, `:92-145` |
| 33 | 🔴 CORRECTS the arc spec's blocking finding. 74 of 641 shipped mobs carry NightVision, by species and by 7 explicit grants. The spec-era claim was that nothing granted it | species `29` in canine, troll, goblin, serpent, raptor, feline, arachnid, mustelid; explicit in `mobs/labyrinth_of_low_tunnels/` 72 to 78 |
| 34 | InfraredVision is granted by nothing: 0 species, 0 mobs, 0 items | grep over `_datafiles/world/dogmud/` |
| 35 | `item_procs.go:275` is still the last raw sender of `CategorySubmission` | verified 2026-09-22 |
| 36 | The SendTrio guard covers exactly three categories today | `send_trio_only_guard_test.go`, `sendTrioOnlyCategories = []string{"Kick", "Trip", "Bash"}` |

### The WrapAnsi probe

Fact 10 and fact 12 were reproduced directly, then the probe was removed.
Input was one outer `fg="214"` span containing an inner `fg="item"` span,
which is the exact shape fact 11 produces:

```
<ansi fg="214">The <ansi fg="item">iron sword</ansi> shatters into a thousand
bright splinters that scatter across the
floor</ansi>
```

Lines two and three carry no reopener, so they render uncolored. For fact 12,
twenty two-byte runes followed by `t ail` broke at visible column 21 against a
requested width of 40.

---

## Owner rulings, 2026-09-22 (do not relitigate)

1. **The crime gate is three tiers onto `PerpUnknown`.** Clear sight witnesses
   and identifies. Shapes-only witnesses but cannot name anyone, so the crime
   is recorded against an unknown perpetrator. No sight is not a witness. The
   shapes arm ships as a correct branch that no shipped content exercises
   (fact 34), and that is accepted.
2. **Wrap is decided by category**, narration wrapping and pre-formatted
   output not.
3. **On learning that `CategorySystem` and `CategoryBroadcast` are mixed
   buckets (fact 7, fact 8) and that `WrapAnsi` is broken (facts 10 and 12),
   M5 widens rather than cuts.** The repair is a prerequisite commit.
4. **The sleep gate is built in PR 3**, not filed. It arrives free by using
   the right predicate (fact 32).
5. **The six unreachable player special moves are parked**, unrelated to M5,
   awaiting a shapeshifter branch of the mutation web.

---

## PR 1: the quality pass

Four items, no player-visible change to a sighted reader. Each is independently
revertible.

### 1a. Close the bare `_plain` leak, then close the class

The remaining End-phase line (facts 19, 19a) is fixed the way the other 15
already were: `sendConditionEndRoomText` passes the holder's plain name so
`HideNames` runs. `throw.go`'s `player_cast_interrupt` (fact 20) gets the real
actee name into the hide list for that event.

Both End paths get names threaded through, including the lit one whose bare
token cannot currently leak (fact 19a), because that safety is an accident of
`litRoom{}` rather than a decision. A test pins the structural reason, so a
future shapes tier on the lit path fails loudly instead of silently converting
every light condition with a bare token into a live leak.

Then the class closes. Quest observer lines already refuse a bare
`{actor_plain}` at load (fact 22). That rule generalizes into a shipped-data
guard over every narration store: an authored line delivered to a room audience
may not contain a bare `_plain` token unless its delivery path passes that name
into `HideNames`. Two instances are a fix; the guard is the deliverable.

🪤 **The guard must be proven capable of failing before it is trusted.** Author
a bare `{actee_plain}` into a room line, watch it go red, remove it. An M4a
lesson three times over: a check that cannot fail is not a check.

### 1b. Point the templates tests at the world players actually read

Flip `dataFilesRoot` (fact 25) to `world/dogmud`. Two tests break (fact 26) and
both breaks are the defect confessing:

- `TestProcess_CharacterSkillsTemplate` fails because dogmud's real
  `character/skills.template` needs a `$blurbs` map the fixture never supplied.
  It was written for default's simpler stub, so it has never once rendered the
  template a player sees. It gets real fixture data.
- `TestU8CrossReferenceValidationRejectsMissingDOGMudOnlyTopic` genuinely wants
  the default tree. It registers it explicitly instead of inheriting it.

`world/default` itself is untouched, per the 2026-09-17 ruling.

### 1c. Correct `internal/messaging/context.md`

The file's stage table and its own numbered list disagree about whether the
pipeline has six stages or seven, and the table omits the sight gate
(fact 27). That inconsistency is independent of wrap and is corrected here.

**The wrap-stage claim itself is NOT corrected here.** It is corrected in PR
2c, so the document and the behavior change on the same day. If PR 2 were
abandoned, this claim would need correcting in its own right, which is the one
dependency between these two PRs.

### 1d. Retire the last raw `CategorySubmission` sender

`item_procs.go:275` moves to `SendTrio`'s observer-only form. It names nobody
so it is not a leak; the value is that `sendTrioOnlyCategories` widens from
three categories to four (facts 35, 36).

---

## PR 2: the wrap decision

Three commits, in this order. The order is the point: nothing may wrap until
the wrapper is correct.

### 2a. Repair `WrapAnsi`, oracle first

Fix the test oracle before the function, because `displayWidth()` shares the
byte-counting bug (fact 12) and a green suite against a broken oracle proves
nothing. Then replace the scalar `openTag` with a stack so balanced nesting
survives a break, and count runes rather than bytes.

This is a live bug today independent of any policy, because `motd.go:44`
already calls `WrapAnsi` on text the MOTD author may nest tags in.

New tests: balanced nesting across a break, three levels deep, a multi-byte
name at the boundary, and the existing unbalanced-input fallback preserved.

### 2b. `shouldWrap` becomes an explicit allowlist

Following `skipStages` (fact 14), which is this package's established idiom for
per-category policy. The default stays `false`, so any category nobody has
classified keeps today's behavior. That is the fail-safe direction: a missed
narration category stays unwrapped and slightly ugly, where a missed table
category would be mangled.

The allowlist admits the unmixed narration and ambient categories. It must not
admit:

- `CategorySystem` and `CategoryBroadcast`, which are mixed buckets (facts 7, 8)
- the speech family, which already wraps itself at a hardcoded 80 (fact 16) and
  would therefore wrap twice at two different widths
- the four dead categories (fact 17), which would be unreachable policy

### 🔴 Two findings that change what 2c can assert

Both measured while planning, neither known when this spec was drafted.

**`WrapAnsi` destroys a table without adding a line.** A real
`templates.DynamicList` table went from 494 bytes to 436 at width 55 with its
newline count unchanged, because `flushWord` collapses runs of padding spaces.
So the obvious guard, asserting the line count did not change, would pass while
an inventory listing was ruined. **The guard must be a checked-in golden of the
rendered bytes**, not a shape assertion.

**The pipeline already alters a table today, with wrap off.** Byte-identity
against the raw template render fails on day one: stage 5 wraps the sheet in
`<ansi fg="system">`, and stage 2 normalizes it, because
`skipStages(CategorySystem)` returns 0 and runs all five normalization stages
over table output. One live consequence: normalization appends sentence
punctuation to the status sheet's last row, so a player reads
`auto-tap-below 15.` where the template authored `auto-tap-below 15`.

🔑 **That stray period is the mixed-bucket problem again, in a second
mechanism.** It is not a wrap bug and it does not have a category-level fix:
adding `CategorySystem` to `skipStages` would disable capitalization and a/an
agreement across roughly 1975 refusal sites to protect a few dozen tables.
`Category` is the wrong axis for normalization for exactly the reason it is the
wrong axis for wrap. Filed, not fixed here, and the golden deliberately locks
in the stray period so nobody re-records it by accident.

### 2c. Guards

A golden that renders a real table and the MOTD banner through the pipeline and
fails if either changes. An assertion that `System`, `Broadcast` and the speech
family never appear in the allowlist. `TestShouldWrapDisabledByDefault`
(fact 2) is replaced, not deleted: the new test pins the allowlist's exact
membership, so adding a category is a deliberate edit with a diff.

`context.md`'s wrap documentation is corrected here rather than in PR 1, so the
doc and the behavior change on the same day.

---

## PR 3: crime in the dark

### The gate

`WitnessesInRoom` (fact 28) stops returning one list. For each candidate mob it
asks, in this order:

| Predicate | Outcome |
|-----------|---------|
| `CanSeeClearly(&mob.Character, room)` | identifying witness, `PerpPlayer`, unchanged from today |
| else `CanSeeShapes(...)` | witness, but the crime records `PerpUnknown` |
| else | not a witness at all |

Order matters: `CanSeeShapes` returns true for full sight as well, so the
clear test runs first.

🔑 **Nothing new is invented.** `PerpUnknown` already exists and is already the
no-witness answer (fact 30). The three tiers already exist. Sleep is already
composed by these two predicates and deliberately absent from the primitive
beneath them (fact 32), so the sleep gate is satisfied by choosing the right
predicate rather than by adding a gate. A sleeping mob fails both tests and
witnesses nothing, in a lit room or a dark one.

### The knowledge side

🔴 **CORRECTED while planning. The obvious rule is wrong and would leak.**

The intuitive split is to give a shapes-only witness `RecordCrimeWitnessed` but
not `RecordMet`, on the grounds that it saw a figure and not a face. That does
not work: **both calls are keyed on `knowledge.PlayerSubject(userId)`**
(`knowledge/types.go:15`), so `RecordCrimeWitnessed` already means "this mob
knows player X did it". Handing it to a witness who could not identify anyone
writes exactly the identity the tier exists to withhold.

The correct rule needs no split at all: **the knowledge loop iterates
`Identifying` only.**

🔴 **This is a live defect today, not merely a design note.** `perp` is
computed once for the whole room, so a single clear-sighted witness opens the
`perp.Type == PerpPlayer` guard at `aggression.go:46` and the loop beneath it
then writes player-subject knowledge for **every** witness in the room,
including ones that cannot see. `steal.go`, `plant.go` and
`MobDeath_FactionRep.go` are checked for the same shape.

### There are five call sites, not four

Found while planning. Besides `aggression.go`, `steal.go`, `plant.go` and
`MobDeath_FactionRep.go`, the revenge AI seeder reads the same list at
`internal/seeders/aggressive_action_to_revenge.go:68`. A mob that cannot see
should not seek revenge either, so it is in scope.

Changing the return type rather than adding a parameter makes the compiler
enumerate all five, which is this project's established refactoring idiom.

**All three open questions were answered by the owner on 2026-09-22:**

1. **Revenge seeding splits by RESPONSE, not by list.**
   `classifyWitnessResponse` already sorts witnesses into guard (no-op),
   noncombatant (`alarmReaction`, which names nobody) and everything else
   (`seedRevengeGoalIfAbsent`, which targets a player by id). So `Identifying`
   witnesses behave exactly as today, and `ShapesOnly` witnesses get the alarm
   only, whatever they would otherwise classify as. A creature that sensed a
   scuffle can recoil and run; it cannot hunt a person it never saw.
2. **`HadExternalWitness` reads `Identifying`, excluding the victim.** Its only
   consumer asks whether the assault was identified by somebody other than the
   victim, and a shapes-only bystander contributes nothing to identification.
3. **No new knowledge subject is needed.** `crimes.Record` already fires
   unconditionally, outside the `PerpPlayer` guard, and already stores
   `RoomId` and `Zone`, so a shapes-only crime already lands in the faction
   crime log, located and unattributed. The owner's "a place with frequent
   crime goes on high alert and hires more guards" is a future feature that
   reads that log; this stage ships its substrate and nothing more.

🪤 **A suspected free-reputation exploit here was chased and DISPROVED.** Do
not re-raise it: `FindRecentAssault` matches only rows whose perpetrator is
`PerpPlayer` with that user's id, so an unattributed assault row is never
found, murder-upgrade Case C is unreachable for it, and the kill takes the
fresh-record path instead.

### What this changes in play

74 of 641 mobs can see in an unlit room (fact 33), and they are overwhelmingly
wildlife: canines, serpents, raptors, spiders. City guards are human and cannot.
So an unlit city room becomes a genuinely low-risk place to commit a crime,
while an unlit forest is still patrolled. That asymmetry is the accepted
consequence of ruling 1, not an oversight, and it is what the playtest must
actually exercise.

### The gate this stage ends with

🔴 **DEFERRED by owner ruling, 2026-09-22, until the graded lighting arc
lands.** The gate is not cancelled and not weakened; it is resequenced,
because running it today would exercise almost nothing.

**Why.** Measuring the world to write the goals file overturned the play
consequence this ruling was accepted on:

- Sight treats a room as lit at `GetVisibility() >= 1`. The model is base 2,
  night minus 1, `darkarea` biome minus 2, lit biome plus 1, any light source
  plus 1. **So night never makes a room dark**: a forest at midnight sits at
  1, which reads as lit. "An unlit forest stays patrolled" was wrong, because
  there is no unlit forest.
- **Four** biomes carry `darkarea: true`: `cave`, `dungeon`, `swamp`, and
  `spiderweb` which no shipped room uses. That is **140 of 1387 rooms**.
- 🔑 **43 of those are permanently lit anyway.** All 31 Crash Site Interior
  rooms carry `hull_suppression` and all 12 Foldweave cave rooms carry
  `foldweave-glow`, both static room mutators with `lightmod: 2`, baked into
  the room YAML and never despawning. Two of the four dark zones content
  designers built are deliberately kept lit, so the gate is inert there **by
  design, not by accident.** Genuinely dark: **97 rooms**.
- **14 dark rooms DO spawn faction-aligned mobs**, in one contiguous cluster:
  an 11-room Warren enclave (`warren`), a 2-room goblin den
  (`ironwind_tribe`), and Tova in Stillwater Marsh (`stillwater_citizens`).
  Wander and forager territory add 2 more reachable dark rooms. So roughly
  **16 rooms, about 1% of the world**, across 3 of the 22 factions.
- 🔴 **And any player can cancel darkness at will.** No shipped ITEM grants
  light; the only source is the spell `chrysalis-glow`, which is a STARTER
  SPELL granted to every new character at creation, commented in
  `internal/characters/character.go` as "light source for caves". A player
  carrying light raises visibility for everyone in the room.

So the gate is a **real but narrow live edge**, not the broad behaviour change
the ruling was accepted on and not a no-op either. Thornwall's cellars, the
case the original prediction named, hold only wildlife: their tunnel rat
swarms are grouped `rats` and `animal`, neither a defined faction, so harming
one is not a crime and it cannot witness one.

**What changes that** is the graded lighting arc: ambient light varying with
time of day, light sources weighed rather than counted, light and darkness
spells contesting at the room level, and two thresholds instead of one so an
ordinary dim room lands in the shapes tier. That last point matters most here,
because it is what turns this stage's `ShapesOnly` arm from a correct branch
nothing exercises into live content.

**The gate as it will be run, once darkness is reachable:** a crime seen
clearly; a crime in the dark with only ordinary mobs present; a crime in the
dark with a night-vision mob present; and the sleep case, a lit room whose
only faction mob is asleep, which is the one that proves attention is read and
not just lighting.

🪤 The shapes tier cannot be reached by shipped content today (fact 34), so
until lighting is graded it is exercised only by an admin applying condition
85 to a mob, the same way infrared was finally verified on 2026-09-21.
`setcondition` is admin only and does accept a target, so the playtest
character must be Megalomania, not Meirok.

---

## Out of scope, filed

- **Speech double-wrap.** Fixing `say`/`shout`/`reply`/`whisper` to stop
  hardcoding 80 (fact 16) touches four commands and a wake trigger. Filed.
- **`SendTextCommunication` takes no category** (fact 15), so wrap policy
  cannot reach room speech at all. Filed with the above.
- **The four dead categories** (fact 17). Deleting them is a tidy-up with its
  own small risk. Filed.
- **The two divergent widths** (fact 5). `ScreenWidth` and `LineWidth` can
  differ per user, so pre-formatting math and wrap math can disagree even when
  the category split is correct. Filed as a follow-up, not solved here.
- **Making the shapes tier reachable** by granting InfraredVision to any mob or
  item (fact 34). A content decision, not M5's.
- **Em dashes in shipped tips.** The longest tip contains them. M6 ledger row
  55 already owns em dashes in shipped narration; tips are the same family.
- **Normalization runs over table output**, appending a stray sentence period
  to the status sheet. Same mixed-bucket root cause as the wrap exclusion, and
  like it, not fixable at category granularity. Filed for whoever splits
  `CategorySystem`.
- **Splitting `CategorySystem`.** Two separate mechanisms now misbehave because
  one category carries both prose and pre-formatted output. That is the real
  fix, and it is too large for M5.
- **Noise-based waking** (owner sidequest, 2026-09-22, scheduled after M5).
  Waking is already half data-driven: condition 15 ships `cancel-on-damage`,
  `cancel-on-combat` and `cancel-on-action`, so violence landing ON a sleeper
  wakes it. The gap is bystanders, and only `shout`, arriving with a light and
  a steal attempt wake a third party today.

  🔑 **This stage is where that behaviour is decided.** After PR 3 a sleeping
  bystander witnesses nothing however long a brawl runs beside it, which is
  correct per the sleep gate and is exactly what the noise system would
  nuance: one killing blow should not wake a guard, several rounds should.
  Filed in full, with the eleven hand-rolled wake sites and the trap that
  `mobs.OnSleeperWoken` is schedule bookkeeping rather than the wake itself.
- **A heat system** reading the crime log by room and zone, to raise alert and
  spawn guards where crime is frequent. Owner-proposed, future. Its substrate
  ships with PR 3.

---

## Risks

1. **The allowlist is a judgment call repeated 61 times.** Mitigated by the
   fail-safe default and by pinning membership in a test, but a miscategorized
   narration category ships as a cosmetic bug, not a caught one.
2. **PR 2 is the first player-visible line-shape change in the arc.** Every
   wrapped category changes how its text lands on screen. The wrap guards prove
   tables are untouched; they cannot prove prose reads well. That is a playtest
   question and PR 2 should carry a lighter playtest of its own.
3. **PR 3 changes faction and bounty consequences**, which touch saved state.
   The behavior is gated on lighting, so a world where every relevant room is
   lit sees no change at all, which is most of the world.
4. **Fact 18 means the anonymizer item is far smaller than every document
   says.** If a reviewer sizes PR 1 from the arc spec or the M6 ledger they
   will expect 17 fixes and find one real leak plus one hardening (facts 19,
   19a). The facts table exists to prevent that.
