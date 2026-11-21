# Messaging M4: the flip

Date: 2026-09-17
Arc: [messaging unification](2026-08-31-messaging-unification-design.md), M4
Status: designed, not implemented

M3 moved every store onto `internal/narration` while keeping each store's own
dialect as a parameter. M4 collapses those parameters: one token engine and
vocabulary, one role vocabulary, one loader policy, one `DefenseType`, one
defence band model, one send path, one sight verdict. It also takes on the
arc's hand-rolled Go narration ("Group C"), which the arc claimed but no stage
owned.

---

## Facts verified against source

Every row was read from source on 2026-09-17, master `ee8477064`.

### Tokens

| Fact | Value | Source |
|---|---|---|
| Core engine | `substitute(s, tokens)`, one `strings.NewReplacer`, unknown tokens left verbatim | `internal/narration/render.go:135-154` |
| Kind B adapter | `textutil.TokenContext` emits exactly `{source}` `{target}` `{source_plain}` `{target_plain}` | `internal/textutil/tokens.go:9-27` |
| Item token set | `items.TokenName`, incl. `{source}` `{target}` `{attacker}` `{defender}` `{weapon}` `{bodypart}` | `internal/items/itemspec.go:200-216` |
| Leftover engine 1 | `ItemMessage.SetTokenValue` (`strings.Replace`), used for combat fallback and feint lines | `internal/items/attack_messages.go:57`; `internal/combat/combat_helpers.go:1724-1748` |
| Leftover engine 2 | `grapplemessaging.RenderTemplate` (`strings.ReplaceAll`), live | `internal/grapplemessaging/render.go:14-16`; `internal/hooks/Position_GrappleTick.go:559` |
| Leftover engine 3 | `hooks.substitute` over `{position}` `{Character}` `{Controller}` `{Controlled}` | `internal/hooks/Position_Messaging.go:142` |
| "No second engine" guard | does not exist (the arc spec promised it) | grep of `*_test.go` |
| Inverted `{source}` | conditions: `{source}` is the HOLDER and renders into the Actee slot; every other store's `{source}` is the actor | `internal/conditions/narration.go:22-38`; `internal/hooks/Condition_ApplyConditions.go:150-156` |
| Shipped `{source}` / `{target}` | combat-messages 4368 / 2812, spells 59 / 2, taunt 57 / 54, conditions 33 / 0, quests 22 / 0 | `grep -rho` per directory under `_datafiles/world/dogmud` |
| Other name tokens | `{attacker}` 186, `{defender}` 414 (defence); `{controllerName}` 239, `{controlledName}` 172 (grapple); `{source_plain}` 17 | same method |
| Unknown-token check | conditions and spells WARN; quests FAIL on `room_text` only; crafting NONE | `conditions/conditionspec.go:298`, `spells/spells.go:328`, `quests/roomtext.go:34` |

### Roles

| Fact | Value | Source |
|---|---|---|
| Core roles | `Roles`/`Variants`: `Actor, Actee, Observer, ActeeObserver` | `internal/narration/render.go:23-44,158-165` |
| Delivery roles | `messaging.Trio{Actor, Actee, Observer Line}`, three only | `internal/messaging/trio.go:40` |
| Trio literal guard | `TestEveryTrioLiteralNamesAllThreeRoles` | `messaging_surface_guard_test.go:1582` |
| Alias table | none; each store maps keys to roles in its own `Narration`/`Render` | per store, e.g. `conditions/narration.go:28` |
| Role key spellings | `toattacker/todefender/toroom/toattackerroom/todefenderroom`, `controller/controlled/observers`, `self/partner`, `*_user_text/*_room_text`, `send_text/room_text`, `playermessage/roommessage`, `success_message/success_room_message`, `attacker/target/room` | store structs |

### A tenth store the arc never listed

| Fact | Value | Source |
|---|---|---|
| File | `_datafiles/messages/position_control.yaml`, 176 lines, OUTSIDE the world tree | disk |
| Keys | `attacker/target/room`, `self/room` | `internal/hooks/Position_Messaging.go:39-61` |
| Loader | hardcoded `filepath.Join("_datafiles","messages",...)`, lazy `sync.Once` | `Position_Messaging.go:73-74` |
| Guard coverage | NONE: the M0 surface guard walks `_datafiles/world/dogmud` only | `messaging_surface_guard_test.go:59` |

### Loaders

| Store | Failure on bad data | Loaded at boot |
|---|---|---|
| defence, combat-messages, itemvoices, casting, conditions, spells, quests, crafting | panic | yes (`main.go`) |
| taunt | log and continue, empty map | yes, `main.go:1937` |
| grapple outcomes | log and continue; path hardcoded `_datafiles/world/dogmud/messaging/grapple_outcomes.yaml` | NO, lazy `sync.Once`, `Position_GrappleTick.go:59-75` |
| position_control | lazy, hardcoded path | NO |
| gossip, tips | silent on missing file, panic on malformed | yes, `main.go:1650-1651` |
| weather emotes | never panics, by documented intent; a build-time guard holds the contract | plugin init, `modules/weather/content/emotes.go:118-126` |

The arc spec's risk row "Loader accepts both old and new locations through M3"
never happened. Every loader reads one path.

### DefenseType

| Fact | Value | Source |
|---|---|---|
| Message-store type | `items.DefenseType`, 5 defences + 4 counter pools | `internal/items/defensive_messages.go:15-34` |
| Combat currency | untyped consts `characters.DefenseDodge` etc., same five names | `internal/characters/character.go:728-740` |
| Bridge | unchecked conversion `items.DefenseType(out.DefenceType)` | `internal/combat/defence_multiplier.go:298` |
| Spell field | `SpellData.TargetDefenseType string`, comment `"" = none`, no validation | `internal/spells/spells.go:35` |
| Spell routing | `physical` -> `spell-physical` (dodge, block); `social` -> `social` (defy); anything else -> `spell-mental` (quell) | `internal/hooks/spell_resolution.go:1241-1260`; `internal/combat/defence_sets.go` |
| Uncontested check | player-target loop only, `TargetDefenseType == ""` | `spell_resolution.go:168` |
| Shipped values (dogmud, 59 spells) | physical 11, mental 9, social 1 (charm), `none` 13, absent 25 | `grep target_defense_type` |
| The 13 `none` | all summons (5 conjure, 6 raise, hive swarm, steppe spirit); skipped by `isSummon` so never contested | `spell_resolution.go:182` |
| The 25 absent | 24 help/neutral spells plus `core-drain` (harmarea, mob_only) | disk |
| core-drain's real channel | `ExecuteDrainArea` hardcodes `combat.ChannelMelee` (dodge, parry, block) | `internal/actions/combat_drain.go:287` |

### Bands

| System | Input | Cutoffs | Source |
|---|---|---|---|
| Melee defence (POST-M4c) | defensive crit, then `meleeDefenceMargin(best)` -- the same normalized, defence-positive margin `DefenceMitigation` mitigates off (0 when floored) | Heavy on crit, Normal >= `Balance.DefenceBandNormalThreshold` (shipped 0.5), else Weak -- the SAME function and knob as every other channel | `internal/combat/combat_helpers.go:1256` (`defenceBand`), `:1269` (`meleeDefenceMargin`), `:1292` (`sendDefenseMessages`); `internal/items/defensive_messages.go:136` (`RenderDefenseMessage`) |
| Channel defence and counters | defensive crit, then normalized margin `-Margin/(StdDev*sqrt2)` | Heavy on crit, Normal >= `Balance.DefenceBandNormalThreshold` (shipped 0.5), configurable | `internal/items/defensive_messages.go:136-151`; `internal/combat/defence_multiplier.go:616` |
| Attack narration | `pctDamage` = damage / expected damage, clamped by `attackMessagePct` | Critical >= 101 and Miss at 0 structural; Normal and Heavy configurable (shipped 30 / 75) | `internal/items/attack_messages.go:297`; `internal/combat/combat_helpers.go:1496` |
| Callers of the channel band | spells, shoot (user and mob), special-move defence (user and mob), taunt, counters | `RenderChannelDefenceMessages` call sites |
| Melee has the margin inputs | `bestDefenseResult.margin`, `.defRoll` from the same contest | `combat_helpers.go:768-785` |
| Weather felt (POST-M4c) | `Balance.WeatherStrongFeltThreshold` (shipped 0.5), read by the engine and threaded down as an explicit parameter -- the deleted `content.StrongFeltThreshold` const is gone | Strong >= threshold, else Mild; configurable | `internal/configs/config.balance.go:383`; `modules/weather/engine/emotes.go:49`; `modules/weather/content/emotes.go:182,360` |
| `AgingPhase` | item potency for drink, eat, shops, autoheal; NOT narration | `internal/usercommands/drink.go:156`, `internal/shops/buyrules.go:141` |
| Skill tiers | cumulative pool UNION at 34/67, not a band | `internal/items/attack_messages.go:99-104` |

### Send path and perception

| Fact | Value | Source |
|---|---|---|
| `SendTrio` | per-reader name hiding via `ParticipantSight`; observer line via `SendTextVisualHidingNames` | `internal/messaging/trio.go:96-126` |
| `SendTrio` call sites | 149 in 31 files (non-test) | grep |
| Combat delivery | `AttackResult` buffers drained per verbosity: participants `u.SendText` (no sight gate), spectators `SendTextVisualToUser` | `internal/hooks/combat_verbosity.go:307-355` |
| Crafting delivery | `user.SendText` + `room.SendTextVisual` | `internal/usercommands/craft.go:135-137` |
| Sight verdict type | `SightDecision{SightFull, SightShapes, SightNone}` exists | `internal/messaging/pipeline.go:32-38` |
| Predicates | `CanSeeClearly` (blind, sleep, dark, nightvision), `CanSeeSightImpairedOnly` (no sleep; combat's), `CanSeeShapes` | `internal/messaging/predicates.go:24-92` |
| Temporary seam | "M2/M4 consolidate darkness, blindness and sleep into ONE perception verdict" | `predicates.go:66-70` |
| Crime witnessing | faction membership only, sight-blind | `internal/crimes/crimes.go:198,232` |
| NightVision granted | condition 29 by species feline, canine, arachnid, mustelid; condition 65 Cat's Eye Draught | `_datafiles/world/dogmud/species/*.yaml`, `conditions/65-cats_eye_draught.yaml` |
| Raw message guard | `TestNoRawEventsMessageOutsidePipeline`, 5 allowed files | `raw_events_message_guard_test.go:19-27` |

### Go narration (Group C)

| Fact | Value | Source |
|---|---|---|
| Arc ownership | "Hand-rolled viewpoint narration, 247 sites / 80 files ... **this arc**"; no stage moves it | arc spec, scope table |
| Still registered | `narrationViewpointRegistry`: 143 sites in 63 files (usercommands 102, hooks 25, actions 12, follow 2, behaviortree 1, cleanup 1) | `messaging_surface_guard_test.go:1164-1313` |
| M2 frozen files | `m2FrozenFiles`: 25 special-move files, literals still in Go | `messaging_surface_guard_test.go` |
| Overlap | 1 file | set intersection |

---

## Owner rulings (2026-09-17)

1. **M4 is several slices and PRs**, not the arc spec's one commit.
2. **YAML stays the format.** Canonical name tokens `{actor}` `{actee}`
   `{actor_plain}` `{actee_plain}`. Event tokens (`{weapon}`, `{bodypart}`,
   `{itemname}`, `{usesleft}`, `{stance}`, `{position}` and the rest) stay.
3. **Canonical role keys** `actor`, `actee`, `observer`, `remote_observer`.
   `remote_observer` is the core's `ActeeObserver`: the defender's room in
   ranged combat. There is no remote actor or actee; each is one person.
4. **Shipped YAML is rewritten**, tokens and role keys both, rather than
   aliased in Go.
5. **All audited Go narration moves to YAML**, self-only lines included.
   "Everything in one place and format."
6. **Defence bands read margin plus defensive crit on every path.** Spells
   defend against physical (dodge an area spell) or mental defences; both
   already band this way, so the change is melee.
7. **Loader failure policy has two tiers by the cost of silence.** Event
   narration fails the boot; ambient narration (weather, tips, gossip) warns
   and goes quiet. Every store also gets a shipped-data test that fails the
   build.
8. **`target_defense_type` is required, one convention.** Values `physical`,
   `mental`, `social`, `non-harm`. Harm spell types take the first three;
   help and neutral types take `non-harm`. The 13 summons move from `none`,
   and the 24 absent help and neutral spells gain it.
9. **core-drain: `physical`, and routed through the physical spell channel**
   (parry no longer applies). A balance change.
10. **`world/default` spells are GoMud holdovers.** Delete them, or change them
    minimally so they cause no error or log noise.

---

## Slices

| Slice | Content | Output proof |
|---|---|---|
| M4a | one token engine, canonical tokens | goldens byte-identical |
| M4b | canonical role keys, loader policy, `DefenseType`, spell defence convention | goldens identical modulo label translation; core-drain commit flagged |
| M4c | one defence band model | deliberate, reviewed diff |
| M4d | one send path with four audiences, one sight verdict | deliberate diff, playtest |
| M4e | Go narration into YAML, by domain | byte-identical per site |

Each slice gets its own plan, written when it is next.

### M4a: one token engine, one vocabulary

- `narration.substitute` is the only substitution engine. The three leftover
  engines move onto it. `SetTokenValue` and `RenderTemplate` are deleted.
- `position_control.yaml` joins the engine here, since its `{Character}`,
  `{Controller}`, `{Controlled}` tokens are part of the rewrite.
- Shipped YAML token rewrite by a committed script with a fixed translation
  table, per store, because the mapping is per store:
  - `{source}` -> `{actor}` everywhere except conditions, where `{source}` ->
    `{actee}`.
  - `{target}` -> `{actee}`; `{attacker}` -> `{actor}`; `{defender}` ->
    `{actee}`; `{controllerName}`/`{Controller}` -> `{actor}`;
    `{controlledName}`/`{Controlled}` -> `{actee}`; `{Character}` resolves by
    its call site, confirmed in the plan.
  - `_plain` variants follow their base token.
- `textutil.TokenContext` fields become `ActorName`, `ActorPlainName`,
  `ActeeName`, `ActeePlainName`.
- Unknown tokens fail validation at boot for every event store, crafting
  included.
- **Guard:** an AST test fails on any `strings.Replace`, `ReplaceAll` or
  `NewReplacer` whose arguments include a literal containing `{`, outside
  `internal/narration`.
- **Guard:** the M0 surface guard walks `_datafiles/messages` too, or the file
  moves under the world tree in M4b (preferred, see below).
- **Proof:** every golden byte-identical, `-update` forbidden. Goldens render
  with stand-in names, so a correct rewrite cannot change a byte.

### M4b: role keys, loaders, DefenseType

**Order inside the slice is load-bearing:** shipped-data tests first, then the
key rename. Renaming YAML keys and Go struct tags in one commit leaves a store
whose struct was missed loading empty, which an ambient store would survive
silently.

- **Shipped-data tests first.** Every store gains a test that loads the
  shipped files and fails the build on a missing role, an empty pool that is
  not empty by intent, or an unknown token. Each is proven capable of failing.
- **Role keys renamed** to `actor`, `actee`, `observer`, `remote_observer` in
  YAML and struct tags. Each store's `Narration` maps straight across.
  - Kind B stores whose lines sit on phase-prefixed keys
    (`start_user_text`) become a phase map with role keys beneath
    (`start: {actee: ..., observer: ...}`). Exact shape per store is fixed in
    the plan against the struct.
  - Conditions' holder line goes under `actee`, matching its token.
- **Proof:** the goldens label rows by authored key name, so labels change
  legitimately. The check is: old golden with labels passed through the same
  translation table equals the new golden, byte for byte. A swapped role still
  diffs. Re-recording is forbidden; it would bake a swap in invisibly (the M3
  trap).
- **Loaders:** the two-tier policy. Taunt, grapple and position_control fail
  the boot on bad data. Grapple and position_control load from `main.go` at
  boot, from the configured data path. `position_control.yaml` moves to
  `_datafiles/world/dogmud/messaging/`, beside `grapple_outcomes.yaml`.
  Weather keeps its fail-soft loader, which is now the documented ambient tier
  rather than an exception.
- **DefenseType:** one named type owned by `characters`, replacing both
  declarations; `items` uses it; the conversion at
  `defence_multiplier.go:298` disappears. Counter pools stay distinct values
  of the message-store key.
- **Spell defence convention:** `TargetDefenseType` becomes a typed value,
  required, validated against the spell's `type` at load. Data sweep per
  ruling 8. The uncontested check at `spell_resolution.go:168` reads
  `non-harm`. The mob-target loop gets the same check; the plan first traces
  whether a help spell can reach it today, and records the answer.
- **world/default spells:** deleted if nothing loads them, else minimally
  classified. The plan checks which.
- **Own commit, flagged in the PR:** core-drain moves from `ChannelMelee` to
  the physical spell channel, with a test pinning its defence set.

### M4c: one defence band model -- DONE (PR pending)

- `GetDefenseMessage` and its z-score banding are deleted. Melee auto-attacks
  band through `RenderDefenseMessage`, via a `defenceBand{crit, margin}`
  struct that `sendDefenseMessages` hands it unchanged; `margin` comes from
  the new `meleeDefenceMargin(best)`, the one derivation the mitigation curve
  and the narration band both read, so they cannot disagree.
- The margin cutoff moves to a balance knob, `Balance.DefenceBandNormalThreshold`
  in `config.balance.go` and `config.yaml`, shipping at today's 0.5.
- `StrongFeltThreshold` moves to `Balance.WeatherStrongFeltThreshold` in
  config, shipping at 0.5. It could not become a plain config read inside
  `modules/weather/content` the way the melee cutoff did:
  `TestContentPackageStaysPure` forbids that package from importing
  `internal/configs`, so the engine reads the live value and threads it down
  as an explicit `strongFeltThreshold` parameter to `content.Tables.Pick`,
  `content.SeasonalTables.Pick`, and `bandedSectionLines`.
- `AgingPhase` leaves the arc: it is item potency, not narration. Skill tiers
  stay a pool union under the M3 assembly rule. `TauntIntensity` and
  `items.Intensity` are caller-named outcomes, not thresholds.
- **End state:** one defence band function and one weather threshold, both
  configurable. This is not the whole narration surface: attack narration
  (`items.GetAttackMessage`, added to the Bands table above) keeps two
  boundaries in Go on purpose, Critical at 101 and Miss at 0, because
  `combat.attackMessagePct` pins them to the crit flag and the `***` banner;
  only its Normal and Heavy cutoffs are configurable. The arc does not claim a
  narration cutoff with zero hardcoded boundaries anywhere.
- **Proof:** a new golden renders melee defence at a fixed grid of margins and
  crit flags, recorded before the change. Its diff is the review: only band
  labels and the pool they draw from may move. A wording change is a defect.
  Spell, ranged and counter rows are in the golden and must not move.
- **Found during implementation, not anticipated by this spec:** the 93
  melee-seam rows in `internal/narration/testdata/stores/defense_messages.golden`
  turned out to be byte-identical duplicates of that same file's own store
  rows above them, so they could never fail on a band change and were
  deleted rather than updated; melee's production-path coverage is
  `internal/combat/testdata/melee_defence_bands.golden` instead. And the
  weather cutoff could not simply move into config the way the melee one
  did, because of the content-package purity guard above; it is threaded
  from the engine as a parameter instead.

### M4d: one send path, one sight verdict

- **Sight verdict:** `messaging.Sight(observer, room) SightDecision` becomes
  the only producer, covering blindness, sleep, darkness, NightVision and
  InfraredVision. `CanSeeClearly` and `CanSeeShapes` become wrappers or are
  deleted. Combat gets its own named predicate for the darkness and blindness
  disadvantage it actually applies, replacing its borrowing of
  `CanSeeSightImpairedOnly`. Crime witnessing is untouched until M5.
- **Four audiences:** `Trio` gains `RemoteObserver`. The Trio literal guard
  moves to four roles; existing literals get the field mechanically.
- **One path:** crafting, quests, caster-only spell effects and
  position_control deliver through `SendTrio`. Ambient stores use its
  observer-only form. Combat keeps per-line verbosity filtering, but its drain
  delivers through the same path, so participants get sight-judged name
  hiding.
- **Open question the plan answers first:** whether combat buffers already
  anonymize at composition. If they do, the combat change is plumbing; if not,
  dark-room combat text changes for participants and belongs in the diff and
  the playtest.
- **Guard:** narration categories leave only through `SendTrio`, sibling to
  `TestNoRawEventsMessageOutsidePipeline`.
- **Playtest gate:** a fight and a spell in an unlit room, both participant
  seats and a witness seat, lines quoted verbatim from both bridges.

### M4e: Go narration into YAML

🔴 **OWNER RULING 2026-09-20: M4e's first PR (special moves, user and mob) ALSO
carries the `canSeeInDark` sight migration.** Do not do it as a separate pass.

M4d PR 2's playtest found that a fully blind player reads the shapes-tier word
`a figure` on mob special-move defence lines and mob taunt room lines. The cause
is `internal/mobcommands/darkness.go`'s `canSeeInDark`, a BINARY predicate
(`visibility >= 1 || NightVision`) predating `SightDecision`. Anyone failing it,
whether they would resolve to `SightShapes` or `SightNone`, is routed through
`messaging.Anonymize`, which knows only the word `a figure`.

Measured on master `93d5795e9`: **30 references across 20 files** (19 in
`internal/mobcommands`: attack, bash, charge, drain, go, gore, grapple,
hamstring, howl, kick, maul, pounce, rake, shoot, skill_move_defence, taunt,
throttle, trip, plus darkness.go itself; and the `internal/usercommands`
skill_move_defence twin). Six of them also make hand-rolled `messaging.Anonymize`
calls outside the pipeline: `howl.go`, `shoot.go`, `skill_move_defence.go` and
`taunt.go` in both packages.

**Why it rides with M4e rather than being PR 3:** M4e's first PR opens exactly
those twenty files to move their literals into YAML. Migrating their sight
handling in the same pass means each file is opened once and reviewed once.
Doing it separately means opening all twenty twice, which is how inconsistency
gets introduced.

The migration target is the pair PR 1 established: `messaging.ParticipantSight`
for the verdict and `messaging.HideNames` for the substitution, so the three
tiers stop collapsing into one word. Both `skill_move_defence.go` and
`darkness.go` already carry comments naming M4's perception consolidation as the
landing spot, and `skill_move_defence.go:89` notes the two `canSeeInDark` twins
"should collapse".

- **Task 1 re-derives the site list** with a broader search than the audit
  walk's candidate definition, and reconciles it against the registry and
  `m2FrozenFiles`. Starting count: 143 sites in 63 files plus 25 special-move
  files, one shared.
- **Layout:** `_datafiles/world/dogmud/narration/<domain>/<name>.yaml`, events
  keyed by name, each with the four role keys. Go keeps the logic (which event,
  which branch) and names events by typed key; it keeps no wording.
- **User and mob special moves share one event file.** Howl and charge get
  their own files with today's wording, preserving the reskin voice for M6.
- **Net before migration:** a script records per site the old format literal
  and the argument each verb takes. A table test asserts that rendering the new
  event with placeholder values equals `fmt.Sprintf(oldLiteral, same values)`.
  Byte-identical per site without driving any command.
- **Guards:** every referenced event key exists in YAML and every YAML event is
  referenced. When the last PR lands, `narrationViewpointRegistry` is empty and
  becomes "no narration string literal in Go"; `TestM2LiteralsAreFrozen` is
  deleted as its files move.
- **PRs by domain:** (1) special moves, user and mob; (2) `hooks`, including
  `spell_resolution.go`; (3) item and inventory commands (`get`, `drop`,
  `give`, `equip`, `lock`, `unlock`, `use`, `sell`); (4) remaining
  `usercommands`, `actions`, `follow`, `behaviortree`, `cleanup`.

---

## How we know it worked

1. M4a, M4b, M4e: goldens byte-identical, or identical after label
   translation. `-update` forbidden.
2. M4c: the melee band diff reviewed row by row.
3. M4d: verbatim dark-room lines from both participant seats and a witness.
4. Guards failing the build: no second token engine; one `DefenseType`; typed,
   required spell defence value; two-tier loader policy with shipped-data
   tests; narration only through `SendTrio`; no narration literal in Go; the
   surface guard sees every narration YAML root.
5. Every guard proven capable of failing by a sabotage that compiles and turns
   it red.

## Risks

| Risk | Answer |
|---|---|
| Key rename leaves a struct tag behind; the store loads empty | Shipped-data tests land first in M4b |
| Re-recording a golden hides a swapped role | Forbidden; the label-translation equality is the proof |
| A translation table is wrong for one store (conditions' `{source}`) | Table is per store and committed; conditions is a named row |
| Moving `position_control.yaml` or grapple's path breaks boot | Boot check in an isolated worktree per PR |
| M4e's site list is incomplete | Task 1 re-derives it by a broader search; the final guard is "no literal in Go", which fails on anything missed |
| A guard passes while production is wrong | Each guard sabotage-proven, and keyed the way production reads (the M3 item 9 lesson) |

## Hands to M5 and M6

- **M5:** crime witnessing reads the one sight verdict; the arc spec's
  NightVision blocker is resolved (species and Cat's Eye Draught grant it).
  `{actor_plain}` leak (formerly `{source_plain}`), the wrap decision,
  `world/default` shadowing.
- **M6:** every store and every Go-sourced event has `actee` and
  `remote_observer` slots in YAML; authoring them is content work.
- **Deploy:** no save migration.

## Standing rule: leave `_datafiles/world/default` alone (owner, 2026-09-17)

**Do not edit the default world unless a test actually reads that store from
it, or `util.ValidateWorldFiles` requires the directory to exist.**

Three reasons were offered for touching it during M4a and M4b-1. Only two are
real, and neither applies to a narration store:

1. It is the Go default for `DataFiles` when the config key is empty
   (`internal/configs/config.filepaths.go:23`), so it is the world a TEST
   binary gets. Some tests genuinely read it: `internal/templates/process_test.go`
   reads its templates, and the hooks and usercommands tests write saves there.
2. `main.go:278` runs `util.ValidateWorldFiles`, which requires every
   DIRECTORY in the default world to exist in the real one
   (`internal/util/util.go:913-942`; one direction, structure only). So adding
   a directory there adds a boot requirement on `world/dogmud` for nothing.
3. ~~Upstream parity~~. **Void** (owner, 2026-09-17): cherry-picks from
   GoMud are inspirational, the trees are millions of lines apart, so nothing
   is owed to the parent repo.

That world does not boot in any case: it has no `defense-messages` directory
and `internal/items/itemspec.go:805` panics on that at `main.go:1643`, long
before any messaging loader runs. It also lacks `taunt-messages`,
`itemvoices`, `recipes`, `gossip_templates.yaml` and `tips.yaml`.

M4b-1 copied two messaging files there and then removed them again. M4a's
token rewrite of its `combat-messages` stays, because the Go code is shared:
if a test ever renders from that world, the canonical tokens are the ones that
work.

🅿️ **Filed, not scheduled: repoint the Go default at `world/dogmud` and delete
`world/default` entirely.** With upstream parity void, the only things holding
it up are the handful of tests above and the `ValidateWorldFiles` comparison,
which would lose its second operand. That is a cleanup slice of its own, not
messaging work.

## Out of scope, filed

- `none`-type spells face zero mitigation in
  `hooks/combat_shared_helpers.go`'s default arm: moot after ruling 8, since
  non-harm spells never reach damage.
- `localize/` and help templates stay out of the arc, as the arc spec rules.
- Group A refusals and admin output: their own arc.
