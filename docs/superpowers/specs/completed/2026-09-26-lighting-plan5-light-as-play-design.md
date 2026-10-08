# Lighting plan 5: light and darkness as play

Design for plan 5 of the graded room lighting arc. It sits on top of the
2026-09-22 spec (`2026-09-22-graded-room-lighting-design.md`, "Source strength,
and the self-throttle") and the 2026-09-23 celestial amendment ("Plan 5
amendment: the darkness throttle"). Where this document and those two
disagree, this one wins, and each overturned point says so.

Plan 5 is **five slices**, like plan 3 was, because together they would pass
CI's 300-file lint inversion. This spec records the arc-level rulings for all
five and designs **5a** in full. 5b through 5e each get their own short spec
when their turn comes.

Brainstormed with the owner 2026-09-26. The tuning page used for the ladder is
`.superpowers/brainstorm/275933-1790437801/content/plan5-source-ladder.html`
(gitignored); it runs the shipped sun, moon and combine formulas.

---

## Facts verified against source (2026-09-26, master `cf502a2f8`)

| # | Fact | Where |
|---|---|---|
| 1 | Carried light is one flat term: if ANY mob or player in the room has the `lightsource` flag, the combine gets exactly one term of `cfg.DimBelow` (50). Ten torches equal one. | `internal/rooms/lighting.go:106` |
| 2 | The combine is `brightest + step*log2(sum 2^((t-brightest)/step))`, step 8; a room with no terms is 0. | `internal/lightscale/lightscale.go:55`, `lighting.go` `composeLight` |
| 3 | The flag is `EmitsLight Flag = "lightsource"`. Seven code sites read it: `actions/skill_helpers.go:34` (sneak modifiers), `characters/description.go:158`, `hooks/Awareness_LightChange.go:69`, `hooks/NewTurn_PruneConditions.go:139`, `rooms/rooms.go:1643,1739` (`FindHasLight`), `usercommands/go.go:819`; `behaviortree/sight.go:23` names it in a comment. | `internal/conditions/conditionspec.go:66` |
| 4 | Condition records carry a per-instance `Magnitude`; an effect declared as `magnitude` reads it. `Effects` aggregates by product, sum, cap or max; there is no per-record read. | `internal/conditions/conditions.go:31`, `effects.go` `Effect` |
| 5 | `AddConditionMagnitude(id, triggers, magnitude)` sets a record's magnitude; it is the per-instance door plan 2 built on. | `internal/conditions/conditions.go:339` |
| 6 | Re-adding a held condition resets ONLY `TriggersLeft`, `RoundCounter` and `Permanent`; other record fields survive. | `internal/conditions/conditions.go:302` |
| 7 | Worn items grant conditions through `WornConditionIds`; on every equipment change the refresh counts ids across all worn items and re-adds each (permanent) or removes it. | `internal/items/itemspec.go:274`, `internal/characters/conditions.go:241` |
| 8 | `chrysalis-glow` (alias `glow`) gives condition 1 Illumination; `primarystat: willpower`, cost 35, `difficulty: 0`. It is a starter spell for every new character. | `_datafiles/world/dogmud/spells/chrysalis-glow.yaml`, `internal/characters/character.go:369` |
| 9 | Condition 1 Illumination: `triggerrate: 5 real minutes`, `triggercount: 4`, flag `lightsource`. | `_datafiles/world/dogmud/conditions/1-illumination.yaml` |
| 10 | A spell's skill is `spellcasting`; the house scaling idiom is `base + stat.ValueAdj/D1 + skill/D2` (unarmed damage). | `internal/characters/spells.go:35`, `internal/characters/combat.go:96-97` |
| 11 | Window edges are constants: `windowDazzleEdge = 75`, `windowShiftCap = 24`, `windowFloor = 1`. | `internal/messaging/window.go:18,20,23` |
| 12 | `rooms` already imports `messaging` (`rooms.go`, `instances.go`, `roommanager.go`), so a rooms-side wrapper can read window edges without a cycle. | grep of imports |
| 13 | Oil Lantern 40038 is `type: object`, gives no light, and is referenced by 9 mob files; quest 14's giver hands one over for dark tunnels ("take this lantern"). | `items/materials-40000/40038-oil_lantern.yaml`, `dialogue/thornwall_city/96.yaml:148` |
| 14 | Tallow Candle 40077 is `type: object`, referenced by 8 mob files and 2 shop files. Votive candles 40106 and 40139 are sold only by the temple almoner and the Confluence offering seller. Lamp oil 40159 is stocked by Prue (9511). No torch item exists. | `items/materials-40000/`, grep |
| 15 | The only item that gives light today is `lantern` 20036 in the upstream **default** world (offhand, `wornconditionids: [1]`). The shipped config loads only `_datafiles/world/dogmud`. | `_datafiles/world/default/items/armor-20000/offhand/20036-lantern.yaml`, config `DataFiles` |
| 16 | Equipment slots are fields on `Worn`; `ComponentBag` is the precedent for a slot added later, touching about 15 Go files plus GMCP `Char`/`Mob` and the web client's icon map, item icons and `webclient-pure.html`. | `internal/characters/worn.go:8`, `internal/items/itemspec.go:113-126` |
| 17 | Rooms have `Nouns map[string]string`, highlighted in `<ansi fg="noun">` by `GetDescriptionFormatted`. Items have no nouns. | `internal/rooms/rooms.go:113,1845` |
| 18 | `cancel` exists and aborts only an in-progress activity (casting, crafting, salvaging). Nothing lets a player end a condition they hold. | `internal/usercommands/cancel.go`, `usercommands.go:84` |
| 19 | No command is named `hood`, `unhood`, `trim` or `flare`. | `internal/usercommands/usercommands.go` (the same grep finds `look` at :143), `keywords.yaml` |
| 20 | Help is template-driven: `help X` renders `templates/help/X.template` whether or not a command X exists; non-command topics (`arena`, `oasis`, `newbie`) are listed under `general:` in `keywords.yaml`, and `help-aliases:` maps extra words. | `internal/usercommands/help.go:161-195`, `_datafiles/world/dogmud/keywords.yaml:147,227` |
| 21 | Help exists for `weather`, `biome`, `chrysalis-glow`, `cats-eye-draught`, `cancel`, `equip`, `equipment`. None exists for `light`, `darkness`, `seasons`, `moons`. A `weather` command exists in the weather module. | `templates/help/`, `modules/weather/weather_commands.go` |
| 22 | `Awareness_LightChange` listens to `RoomChange` and `EquipmentChange` and carries a FUTURE note for light-spell cast and cancel. | `internal/hooks/Awareness_LightChange.go` |
| 23 | Three goldens cover light: `testdata/lighting_parity.golden`, `testdata/lighting_daycycle.golden`, `internal/narration/testdata/stores/light_notices.golden`; `tools/lighting_golden_diff.py` explains a move. | repo |
| 24 | Lighting knobs are absent from `config.yaml`, so the Go defaults are live: step 8, latitude 46.5, equinox noon 70, starlight 10, moons full 35, moon weights 4 / 1 / 0.5, bands 25 / 50. | `internal/configs/config.balance.lighting.go` |

---

## The arc: five slices

| Slice | Covers | Depends on |
|---|---|---|
| **5a** Carried light with real strengths | the light slot, the four light items, one trim function, glow strength and duration scaling, `hood` / `unhood`, `cancel <spell>`, item nouns, help | none |
| **5b** Dazzle gets teeth | the penalty for being dazzled; `windowDazzleEdge` becomes a config knob; the daylight-cost text on every vision grant | 5a |
| **5c** Vision spells and potion | a nightvision spell, an infravision spell, an infravision potion | 5b |
| **5d** Darkness | the darkness spell, a term below zero, the inverted trim; the web client's Game-window border showing the player's own band (see below) | 5a |
| **5e** Scheduled light sources | room fixtures and items with a schedule (lamps lit at night, pulsing runes) | 5a |

**Deferred to 5d (owner, 2026-09-26): the Game-window border.** The centre Game
window's border takes one of four subtle tones from the player's own band:
dark for blind, dim for shapes, the normal gold for faces, bright for dazzled,
with a short tooltip so it is not colour alone. The band is
`messaging.LightBand` (plan 3d), already per observer and nightvision-aware.
It travels as one small GMCP field holding only the player's own band, which
the text already tells them, so it does not widen the open "GMCP bypasses
darkness" leak (room contents). It updates at 3d's cadence (command, combat
round, move), matching the 3d ruling that an idle player learns of dusk when
they next act.

## Arc-level rulings (owner, 2026-09-26; do not relitigate)

1. **One adjustment function for every adjustable source.** Spell light,
   the hooded lantern and the darkness spell call the same function. It
   makes the **minimum cut from full strength** that keeps the room inside
   the bearer's comfortable range. For light the edge is the top of the
   bearer's faces band (just under their dazzle edge). For darkness it is
   inverted: the bottom of the bearer's **usable** range (shapes), as the
   celestial amendment ruled. Chosen because "we don't want to incur a
   penalty voluntarily if we can avoid it". Darkness ITEMS are out of scope,
   but the function takes any bearer, so they drop in later unchanged.
2. **A fresh source is at full strength, and that is the weapon.** Casting,
   equipping a light item and `unhood` all set full strength. Re-equipping a
   lantern is the item's version of recasting.
3. **Trimming is automatic and happens on events only**: a mover's own
   sources trim when they enter a room, in entry order. Nothing trims on a
   round tick, and sources already in a room do not re-trim when someone
   else arrives (unchanged from 09-22).
4. **Ladder C** (fudged plausible) for carried sources, see below.
5. **The light slot.** Every character gets an equipment slot named `light`;
   equipping into it is how a light is turned on. Fuel is assumed and never
   tracked.
6. **No verb for spells.** Recast for full strength. `cancel <spell>` ends a
   spell you hold, opt-in per condition.
7. **Trims are silent.** Messages come only from hood, unhood, equipping and
   casting. No extra "hood your lantern" hint on the dazzle notice.
8. **Help topics do not need commands.** `light`, `seasons`, `moons` and
   (in 5d) `darkness` are topics in their own right.
9. **5a's gate includes a multi-agent party playtest** with several players
   carrying light sources, to check that entry order sets the trims the way
   these rules say.
10. **No AI-companion boot check for this arc** (owner, 2026-09-26). This
    overrides the 09-24 standing rule for plan 5; `perception.go` still reads
    `ParticipantSight`, whose signature 5a does not change.

### What this overturns

| Earlier decision | Now |
|---|---|
| 09-22: `my output = clamp(target - current room light, 0, max)` | A linear subtraction is wrong on a log scale; the trim solves the combine exactly (see "The trim") |
| 09-22: strength `stat/10 + skill/2` | Needs a base term: without one a new character's glow is 10, which is no light, and fifteen 3c rooms rely on it being playable |
| 09-22: "the manual control forces a light down" | There is no general manual control. `hood` / `unhood` exist on the hooded lantern only; spells recast |
| 09-22: light items are offhand (item 40038 "becomes functional") | They go in a new `light` slot, so a light never costs a shield |
| 09-24 standing rule: every lighting plan boots with the AI companion on | Waived for plan 5 (ruling 10) |

---

## Is this realistic?

Light adds linearly in physics (two torches are twice the light) and is
perceived compressively, roughly as the cube root (Stevens' power law). That
is why a 400,000-to-1 noon-to-moon ratio fits in 35 points, and why, anchored
at noon 70, ten times the light is worth about 9 points. The combine already
does the physics right: it converts terms back to intensity, adds, and takes
the log.

Two places are deliberately fudged, both shipped by plans 1 to 4 and kept:

- **The bands.** Real eyes read faces at about 1 lux (about 26 here) and find
  their way at about 0.01 lux (about 9). The game draws those lines at 50 and
  25, which makes game eyes about 500 times less sensitive than dark-adapted
  real ones. That is what makes darkness a mechanic at all. The shipped night
  sky already gets about +14 points to compensate.
- **The step.** On a strict perceptual reading, doubling the light adds about
  3 points, not 8, so stacking and shade bite harder here than in life.

With the moons' +14 applied to real carried lights at about 1.5 m, realism
gives candle 37, oil lantern 46, torch 52. The shipped tavern (50) and main
street (52) lamps are already plausible on that basis.

## The ladder (5a)

| Source | Strength | Trims | Cave, normal eyes | Cave, nightvision 24 | Outdoors, midsummer noon |
|---|---|---|---|---|---|
| Tallow candle 40077 | 38 | no | shapes | faces | 74, not dazzled |
| Oil lantern 40038 | 52 | no | faces | dazzled | 75, dazzled |
| Torch (new) | 56 | no | faces | dazzled | 76, dazzled |
| Hooded lantern (new) | up to 54 | yes | 54, faces | trims to 50, faces | trims, never dazzles its bearer |
| Glow, new character | 50 | yes | faces | faces | dazzles until the caster moves |
| Glow, endgame (WIL 175, skill 65) | 90 | yes | dazzles everyone until the caster moves | | |

Within about 5 points (about 3.5 times the light) of the realistic values.
Consequences, all intended:

- A plain lantern or torch dazzles normal eyes only near midsummer noon: the
  carried strength needed to reach 75 outdoors is about 47 at midsummer, 62
  at the equinox and 70 at midwinter. That is the owner's 09-25 ruling, and
  it is what the hood is for.
- Any fire light dazzles a nightvision creature in its own cave (its dazzle
  edge is 51), so light is a weapon as intended.
- A `city_thoroughfare` room already reaches 75 at midsummer noon with nothing
  carried (sky 72.8 plus lamp 52). That is the case ruled intended on 09-25.
- A nightvision-24 bearer is dazzled in every daylight scene and in a tavern
  by day. 5c's grant text states it; 5b gives it a cost.

The glow formula is `base + WIL/D1 + spellcasting/D2` with **base 40, D1 10,
D2 2**, all config knobs. Duration uses the same shape: a new character keeps
today's 20 real minutes and an endgame caster reaches about 45; the plan picks
the two divisors to land those two points. Record for the spell scaling
unification arc that it should absorb this rather than leave glow as an
exception.

---

## 5a design

### 1. One source model: every light is a condition record

- A new effect kind `light_strength` in `internal/conditions/effects.go`.
  It does NOT aggregate through `Effect()`: every record is its own term.
  A new per-record accessor returns each held light source.
- A spell source declares `light_strength: magnitude`; the cast sets the
  magnitude from the glow formula through `AddConditionMagnitude`. An item
  source declares a literal (`light_strength: 52`).
- Two new spec flags: `adjustable` (glow, the hooded lantern) and
  `cancellable` (glow). The Cat's Eye Draught stays non-cancellable, per the
  09-23 ruling.
- Two new record fields: the **current output** (unset means full strength)
  and **hooded**. Conditions already persist in saves, so no new save field
  and no migration. Fact 6 means an unrelated equipment change keeps them.

Rejected: a `light:` block on the item with state on the item instance. It
reads naturally for a lantern, but it gives two storage paths and two
persistence stories behind one function.

### 2. Composition

`composeLight` term 3 stops being one flat `DimBelow` term. Each light record
held by anyone in the room contributes its current output as its own term.
`LightTerms.Carried` keeps meaning "someone carries a light", so 3d's cause
attribution is unchanged.

The `lightsource` flag is retired. `EmitsLight` becomes a character predicate,
"holds a light record with output above zero", and its seven reading sites
(fact 3) move to it. A shut hood therefore stops the sneak "beacon" penalty, which is a
side benefit worth one help line. Nothing grants the flag through a mutation
today; `mutations/describe.go`'s `lightsource` phrase goes with it, found by
grep because the compiler cannot see a string. A future glow mutation uses the
effect.

### 3. The trim

A pure function in `internal/lightscale`, beside `Combine`:

```
Trim(step, others, max, target) -> output
  light:     the largest s <= max with Combine(others, s) <= target
             = min(max, target + step*log2(1 - 2^((others-target)/step)))
             Absent when others >= target (the room is bright enough)
  darkness:  inverted (5d): the smallest cut that keeps the room >= target
```

`others` is the room's combine with **this** source removed. The target comes
from the bearer's window, read in `messaging` because that package owns the
edges: for light, `windowDazzleEdge - strength - 1`, which is 74 for normal
eyes and 50 for nightvision 24 (not `- 0.5`: a room at exactly 74.5 rounds to
75, which is dazzled); for darkness (5d), the bearer's usable
floor (`BlindBelow - strength`, down to `windowFloor`, or `-reach` with
infravision).

A thin wrapper in `rooms` trims every adjustable source one mover carries,
one after another, each seeing the others already trimmed, in a stable order
(record order), so the result never depends on a hidden order.

### 4. Triggers

- **Enter a room** calls the wrapper for the mover. The seam is the
  `RoomChange` path `Awareness_LightChange` already listens to (fact 22).
- **Cast** sets full strength (a fresh record, or a recast replacing it).
- **Equip into the light slot** sets full strength with the hood open.
  Unequipping removes the record, so equipping after it starts fresh, but a
  direct swap between two items sharing a condition keeps the record (fact
  7 counts ids), so the equip path resets it explicitly. Swapping gloves
  must NOT reset a lantern, and fact 6 is why it does not.
- **`unhood`** sets full strength; **`hood`** sets output to Absent (the
  source stays lit and held). Hooded persists across moves until `unhood`
  or re-equip.

### 5. The light slot

A new `ItemType` `light` and a `Worn.Light` field, mirrored at every site that
enumerates `ComponentBag` (fact 16), including `AllSlots`, GMCP `Char`, and the
web client's equipment pane and icon map. The existing lanterns and candles
change `type: object` to `type: light`.

### 6. Items

| Item | Change |
|---|---|
| Oil Lantern 40038 | `type: light`, gives a new condition at 52. Quest 14 starts meaning what it says |
| Tallow Candle 40077 | `type: light`, new condition at 38 |
| Torch | new item, new condition at 56, sold cheaply by the same merchants |
| Hooded Lantern | new item, new condition up to 54, `adjustable`, priced above the oil lantern, sold by the same merchants; carries nouns (`hood`) |
| Votive candles 40106, 40139 | unchanged, offerings |
| Lamp oil 40159 | unchanged; fuel is not tracked |

IDs come from `tools/id_inventory.py` at plan time. The plan confirms which
of the 9 mob files referencing 40038 are merchants rather than carriers
before stocking the new items. Each item has its own condition, so condition
1 stays the glow's alone.

**A light shows at most once in the conditions list (owner, 2026-09-26).** The
web client's Status & Conditions panel and the `conditions` command both filter
on `ConditionSpec.Listed()` (`internal/conditions/conditionspec.go:235`: not
`secret`, not `hidden`) and show one entry per held record. Every item light
condition is therefore `secret: true`, since the item already shows in the
equipment panel; only the glow's Illumination lists, and its duration is what
tells a player when to recast. A shipped-data test pins every item light
condition as secret.

The default world's `lantern` 20036 and condition 1 are not loaded by the
shipped config. The plan checks whether they still validate once the flag is
retired and migrates them to the effect if a test loads them.

### 7. Item nouns

`ItemSpec` gains `nouns: map[string]string`, the same shape as rooms.
`look <item>` highlights them with `<ansi fg="noun">`, and `look <noun>`
resolves against the nouns of items you carry or wear after room nouns.
The hooded lantern's `hood` noun explains `hood` and `unhood`. A player who
never looks still gets the default trimming.

### 8. Commands

- `hood <item>` / `unhood <item>`: the hooded lantern in your light slot.
  One line to you, one to the room. Refused with a plain line for any other
  item.
- `cancel <spell>`: ends a `cancellable` condition you hold, with its normal
  end narration. `cancel` with no argument keeps today's behaviour.
- No new verb for spells.

Each new command is wired at every registration step (`usercommands.go`,
`keywords.yaml`, help template), per `dogmud-refactoring`.

### 9. Messages

Silent trims. One actor line and one observer line for hood, unhood, equip
and cast. Band changes use 3d's existing notices; no new hint.

### 10. Help

| Topic | New or updated |
|---|---|
| `light` | new, under `general:`; aliases `lighting`, `lantern`, `lanterns`, `torch`, `candle`, `dazzle`, `dazzled`, `dark`. Bands in words, carried sources, trimming, the hood, why a torch at noon hurts |
| `seasons` | new, under `general:`; aliases `season`, `winter`, `summer`, `calendar`. Day length and noon height through the year |
| `moons` | new, under `general:`; aliases `moon` and the three moons' names. What each does to night light |
| `hood` | new (covers `unhood` through an alias) |
| `chrysalis-glow`, `weather`, `equipment`, `cancel`, `biome` | updated |

Every page in the group links the others in "See also". 5c and 5d add their
topics and extend the links. Text follows `dogmud-player-copy`: 80 columns, no
raw numbers, light described as faces, shapes, dark and dazzled.

### 11. Config

New balance knobs: the glow base and two strength divisors, and the two
duration divisors, declared in the balance config and written into
`config.yaml` with the skip-worktree procedure (build the commit from the
`git show HEAD:` blob). Item strengths are content, in condition YAML, not
config.

---

## Testing and gates

- `lightscale.Trim` table tests: the exact combine inverse against
  `Combine`, the Absent case, the `max` cap, a cave with no other terms, and
  (for 5d's sake) the inverted direction.
- Composition: two torches beat one by the log amount; a hooded source
  contributes nothing; `EmitsLight` false under a shut hood.
- Entry order: three bearers enter one by one; each trims against what the
  earlier arrivals left, and nobody re-trims when a later one enters.
- Equipment: swapping another slot keeps a trimmed and hooded lantern as it
  is; re-equipping the lantern resets it to full with the hood open.
- Glow scaling: magnitude and duration at the starter and endgame points,
  with `SetConfigForTest` pinning the knobs (a test binary never loads
  `config.yaml`).
- Shipped-data tests: the four items resolve to the ladder values, and the
  glow condition is `adjustable` and `cancellable`.
- Goldens: every move is explained room for room with
  `tools/lighting_golden_diff.py` before re-recording. Expect moves only
  where a carried light is sampled, because the carried term changes from a
  flat 50 to real strengths.
- **The party playtest** (`playtest-scenario`): three or more agents, each
  with a different source (starter glow, oil lantern, hooded lantern), moving
  together through a dark dungeon, a lit street at night and open ground by
  day, reporting what each sees on arrival and after the next member enters.
  Ends with the mandatory adversarial content gate for the new items.

## Out of scope

Dazzle's mechanical penalty (5b). Vision spells and potion (5c). Darkness and
darkness items (5d). Scheduled sources (5e). Plan 6's infrared edge bug (faint
rooms 2 to 12 read dark to infravision). Mobs deliberately carrying lights:
the model supports it, and no content is authored for it here.
