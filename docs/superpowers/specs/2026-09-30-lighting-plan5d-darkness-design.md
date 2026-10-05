# Lighting plan 5d: darkness

Date: 2026-09-30. Arc: graded room lighting, plan 5 ("light as play"), slice
5d. Parent spec: `docs/superpowers/specs/2026-09-26-lighting-plan5-light-as-play-design.md`
(arc table, "Deferred to 5d: the Game-window border", arc rulings 1 to 10,
"3. The trim"). Depends on 5a (carried light, the trim, the light slot) and 5c
(infravision reach). Design approved by the owner on 2026-09-30; this spec
verifies it against source and records where the source disagrees with it.

## Facts verified against source (2026-09-30, master `47220bce3`)

Every row was read at `47220bce3` in the worktree `C:/tmp/dogmud-5d-spec`.
Negative rows name the search and a positive hit proving the same search could
match. Knob rows give the Go default AND the shipped value; "absent" means the
key is not in the `git show HEAD:_datafiles/config.yaml` blob, so the Go
default is live.

### The scale, composition and the trim

| # | Fact | Where |
|---|---|---|
| L1 | `Combine(step, terms...)` = `best + step*log2(sum 2^((t-best)/step))`; Absent (-Inf) and NaN terms skipped; no present term returns `Absent()` | `internal/lightscale/lightscale.go:55-84`; `Absent` `:27` |
| L2 | `Attenuate(step, light, fraction)`: fraction <= 0 returns Absent, else `light + step*log2(fraction)` | `lightscale.go:97-108` |
| L3 | `Polarity` with `Brightens = 1`, `Darkens = -1`; `Trim(step, others, max, target float64, p Polarity) float64` | `internal/lightscale/trim.go:10-17,52` |
| L4 | The light branch solves `Combine(others, s) = target` exactly: `need = target + step*log2(1 - 2^((others-target)/step))`, Absent when `others >= target` or `need < 0`, `min(target, max)` when `others` is Absent | `trim.go:73-83` |
| L5 | The Darkens branch is LINEAR: `cut = level - target` (Absent level read as 0), 0 when `cut <= 0` or `max <= 0`, else `min(cut, max)`. Its doc comment warns that at a target at or below 0 the light branch's Absent-room result `min(target, max)` needs a caller-side floor, "5d's problem" | `trim.go:38-42,47-51,62-72` |
| L6 | `lightscale.Darkens` has NO production caller. Grep `Darkens\b` in non-test `.go` outside `internal/lightscale/` finds nothing; the same grep for `lightscale.Brightens` finds `internal/rooms/light_trim.go:64` | grep |
| L7 | `composeWith(cfg, celestial, skyFilter, carried []float64) LightTerms`: sky term, lamp term, every carried term, one `Combine`; an Absent result becomes 0 ("magical darkness ... arrives in plan 5d"); `Level` rounds and clamps to [-100, 100] | `internal/rooms/lighting.go:97-146` (Absent to 0 at `:131-136`, clamp `:138-144`) |
| L8 | `LightTerms{Level int; Raw float64; Sky float64; SkyFilter float64; Lamp int; HasLamp bool; Carried bool}`; `Raw` is the combined light before rounding, "a trim solves against it" | `lighting.go:59-76` |
| L9 | `Raw` has exactly one reader: the trim (`light_trim.go:62`). Grep `\.Raw\b` in non-test Go finds that, the writer `lighting.go:130`, and unrelated `apiframework` fields | grep |
| L10 | `carriedLight(exclude)` walks every mob then player in the room and appends `rec.LightNow(spec)` for each of `c.Conditions.LightSources()` | `lighting.go:151-174` |
| L11 | `TrimLightFor(c)`: collects `LightSources()` records that are adjustable and not hooded, sets them all off, then for each: `others := composeLightExcluding(...).Raw`, `target := messaging.LightTrimTarget(c.NightVisionStrength(), cfg.DazzleAbove)`, `out := lightscale.Trim(step, others, full, target, lightscale.Brightens)` | `internal/rooms/light_trim.go:26-72` |
| L12 | `TrimLightFor` has two callers: `MoveToRoom` (`internal/rooms/roommanager.go:474`) and `Room.AddMob` (`internal/rooms/rooms.go:1253`) | grep |
| L13 | A spawned mob is listed with `listMobInRoom` (`rooms.go:896-907`), not `AddMob`, from `Room.Prepare` (`rooms.go:912`, `NewMobById` at `:1021`), so a spawn does not trim | as cited |

### The light record model

| # | Fact | Where |
|---|---|---|
| R1 | `EffectLightStrength EffectKind = "light_strength"`; the effect-kind set is closed (`AllEffectKinds`), "a new kind is a code change with a reader" | `internal/conditions/effects.go:10-13,41,46-49` |
| R2 | `ScaledKinds = {light_strength, nightvision_strength, infra_reach}`; a condition may read at most one from its magnitude | `effects.go:73,153-162` |
| R3 | `validateEffects` refuses a literal `light_strength` of zero or less, an `adjustable` record without `light_strength`, and a stacking light source | `effects.go:171-181` |
| R4 | `Conditions.Effect(EffectLightStrength)` returns 0: light is per record, never aggregated | `effects.go:201-204` |
| R5 | Record fields `LightTrim LightTrim`, `LightOutput float64`, `Hooded bool`; states `LightFull ""`, `LightTrimmed`, `LightOff` | `internal/conditions/conditions.go:41-43`; `internal/conditions/light.go:8-14` |
| R6 | `LightMax(spec)` (magnitude or literal of `light_strength`), `LightNow(spec) (float64, bool)` (false when expired, hooded, off or strengthless; a negative stored output is refused), `SetLightOutput(out)` (non-finite lands on `LightOff`), `ResetLight()`, `LightSources()` (every unexpired record whose spec `IsLightSource()`) | `light.go:20,36,67,80,87` |
| R7 | `IsLightSource()` is "the spec declares `light_strength`" | `internal/conditions/conditionspec.go:248-251` |
| R8 | Flags `Adjustable` and `Cancellable` | `conditionspec.go:109,112` |
| R9 | `AddConditionMagnitude` calls `ResetLight()` only when `spec.IsLightSource()` | `conditions.go:347,367-369` |
| R10 | Equipping a `type: light` item calls `ResetLight()` on every record of every `WornConditionIds` id, not gated on `IsLightSource` | `internal/characters/worn.go:592-600` |
| R11 | Every non-test reader of `IsLightSource()` / `LightSources()`: `characters/light.go:9` (`LightTerms`), `conditions/conditions.go:367`, `effects.go:179`, `hooks/NewTurn_PruneConditions.go:141`, `rooms/lighting.go:154`, `rooms/light_trim.go:35`, `usercommands/hood.go:27` | grep |
| R12 | `Character.EmitsLight()` is `len(c.LightTerms()) > 0`, and `LightTerms` reads `LightSources()` / `LightNow` | `internal/characters/light.go:7-22` |
| R13 | `EmitsLight()` readers: sneak modifiers `internal/actions/skill_helpers.go:35`, the `lit` adjective `internal/characters/description.go:158`, `internal/hooks/Awareness_LightChange.go:68`, waking sleepers `internal/usercommands/go.go:493`; `behaviortree/sight.go:23` names it in a comment | grep |
| R14 | Candlelight 124 is `secret`, `light_strength: 38`, `triggerrate: 5 real minutes`, `triggercount: 1`; Hooded Lantern Light 127 is the same shape at 54 with `adjustable` | `_datafiles/world/dogmud/conditions/124-candlelight.yaml`, `127-hooded_lantern_light.yaml` |
| R15 | Illumination 1: `light_strength: magnitude`, `adjustable`, `cancellable`, `triggerrate: 5 real minutes`, `triggercount: 4`, start and end lines on `{actee_plain}` | `conditions/1-illumination.yaml` |
| R16 | A light's END room line is judged "as lit" (`SendTextVisualAsLitHidingNames`), keyed on `IsLightSource()`; every other end line is judged by the room as it is | `internal/hooks/NewTurn_PruneConditions.go:127-145`; `internal/rooms/rooms.go:354-377` |
| R17 | A condition's START room line is sent after the record is added, through `SendTextVisualHidingNames`, i.e. judged by the room with the new record in it | `internal/hooks/Condition_ApplyConditions.go:102-110,165-176` |

### Sight windows (messaging)

| # | Fact | Where |
|---|---|---|
| S1 | `windowFloor = 1`; `windowShiftCap = configs.LightWindowShiftCap` (24) | `internal/messaging/window.go:15,19`; `internal/configs/config.balance.lighting.go:10` |
| S2 | `SightThroughWindow(light, strength, reach, blindBelow, dimBelow)`: full at `light >= dim - s`; natural shapes at `light >= blind - s && light >= windowFloor`; infra shapes at `reach > 0 && light >= -reach`; else none. Negative light therefore reads none unless infra reaches it | `window.go:34-64` |
| S3 | `LightTrimTarget(strength, dazzleAbove) = dazzleAbove - clampShift(strength) - 1` | `window.go:95-97` |
| S4 | `BandThroughWindow` maps none/shapes/full to `BandDark`/`BandShapes`/`BandFaces` or `BandDazzled` at `light >= dazzleAbove - s`; `Band.String()` gives `dark`, `shapes`, `faces`, `dazzled` | `internal/messaging/band.go:21-58` |
| S5 | `LightBand(observer, room)` reads `room.LightLevel()`, `NightVisionStrength()`, `InfraReach()`; Blinded reads dark | `band.go:68-84` |
| S6 | `ComfortDistance` reads `room.LightLevel()`; the dark ramp caps at 1 below the blind edge; `infraDarkCap` needs `light >= -reach` | `internal/messaging/comfort.go:20-39,49-69,80-87` |
| S7 | `ParticipantSight` (`predicates.go:56`), `CanSeeClearly` (`:136`), `CanSeeSightImpairedOnly` (`:167`, `== SightFull`), `CanSeeShapes` (`:183`), `SightMult` (`sight_mult.go:32`) all read `room.LightLevel()` through `SightThroughWindow` or `ComfortDistance` | as cited |
| S8 | `CalcSneakScore(c, effectiveLit)` applies the emits-light modifiers from `EmitsLight()`; `CalcSneakScoreVsObserver` sets `effectiveLit := LightBand(observer, room) != BandDark` | `internal/actions/skill_helpers.go:28-47,65-66` |
| S9 | Crime witnesses are split by `CanSeeClearly` / `CanSeeShapes` | `internal/crimes/crimes.go:220-229` |
| S10 | `InfraReach()` log-combines every held `infra_reach` value and every mutation's rank-scaled `infraredvision` value, caps at `InfraReachCap`, rounds once | `internal/characters/vision.go:45-66` |
| S11 | A mob's behaviour-tree sight is `mobCanSee = CanSeeSightImpairedOnly`, i.e. `SightFull` only; infravision shapes are NOT seeing for it. It gates `players_in_room` (`conditions_player.go:118`), enemy counting (`:196`) and party aggro (`actions_party.go:238`). Ruled in slice F (`specs/completed/2026-09-11-followup-slice-f-mobs-perceive-darkness-design.md`) | `internal/behaviortree/sight.go:29-34` |

### Knobs

| # | Knob | Go default | Shipped | Where |
|---|---|---|---|---|
| K1 | `LightBlindBelow` / `LightDimBelow` | 25 / 50 | absent | `config.balance.go:1150-1151`; coerced `config.balance.lighting.go:27-31` |
| K2 | `LightDazzleAbove` | 75 | 75 | `config.balance.go:1158`; blob `:946` |
| K3 | `LightDoublingStep` | 8 | absent | `config.balance.go:1196`; `config.balance.lighting.go:117` |
| K4 | `LightSpellStrengthBase` / `StatDivisor` / `SkillDivisor` | 40 / 10 / 2 | 40 / 10 / 2 | `config.balance.go:1276-1278`; `config.balance.lighting.go:193`; blob `:957-959` |
| K5 | `LightSpellDurationBase` / `StatDivisor` / `SkillDivisor` | 2 / 50 / 20 | 2 / 50 / 20 | `config.balance.go:1279-1281`; `config.balance.lighting.go:194`; blob `:960-962` |
| K6 | `LightInfraReachCap` / `LightInfraPenaltyFloor` | 50 / 0.90 | 50 / 0.90 | `config.balance.go:1302-1303`; `config.balance.lighting.go:205-210`; blob `:980-981` |
| K7 | `DarknessCombatPenalty` / `DazzleCap` | 0.80 / 0.80 | 0.80 / 0.80 | `config.balance.go:309,314`; `config.balance.combat.go:358-363`; blob `:933,939` |
| K8 | `SpellDiscoverySkillPerDifficulty` | 1.0 | 1.0 | `config.balance.go:651`; `config.balance.spells.go:39-40`; blob `:1852` |
| K9 | `SneakModEmitsLightDarkRoom` / `SneakModEmitsLightLitRoom` / `SneakModNoLightLitRoom` | 0.5 / 0.85 / 0.9 | 0.5 / 0.85 / 0.9 | `config.balance.go:295-297`; `config.balance.combat.go:277-285`; blob `:901-903` |
| K10 | The lighting accessor drops the `Light` prefix: yaml `LightInfraSpellBase` is `Lighting.InfraSpellBase` | | | `internal/configs/config.lighting_accessor.go:11-39,66-81` |
| K11 | `_datafiles/config.yaml`: in this fresh worktree `git ls-files -v` prints `H` (the skip-worktree bit lives in the main checkout's index, per CLAUDE.md, not here). The disk copy differs from the HEAD blob; a CR-stripped diff is empty, so here the difference is line endings only. Either way the plan builds the commit from the blob | | | `git ls-files -v`; `diff` |

### Spells and discovery

| # | Fact | Where |
|---|---|---|
| P1 | Chrysalis Glow: `targeting: single`, `schools: [mental]`, `cost: 35`, `waitrounds: 3`, `difficulty: 0`, `primarystat: willpower`, `condition_ids: [1]` | `_datafiles/world/dogmud/spells/chrysalis-glow.yaml` |
| P2 | Night Vision `difficulty: 15`, `cost: 45`, condition 128; Heat Sight `difficulty: 35`, `cost: 80`, condition 129; both mental, single, willpower | `spells/night-vision.yaml`, `spells/heat-sight.yaml` |
| P3 | `RequiredSkillFor(difficulty) = ceil(difficulty * SpellDiscoverySkillPerDifficulty)`; discovery requires `skillLevel >= RequiredSkillFor(sp.Difficulty)` | `internal/spells/spells.go:378-387,456` |
| P4 | Cast chance starts at 100 and subtracts `GetDifficulty()` | `internal/characters/spells.go:26-31` |
| P5 | `magnitudeSpellApplication` scales the condition's one `ScaledKind` via `SpellScaledMagnitude(kind, stat, skill)`; duration uses the SHARED light trio `SpellDuration*` for every kind | `internal/hooks/light_spell.go:24-45`; `internal/conditions/scaled_magnitude.go:17-27` |
| P6 | `CapScaledMagnitude` caps infra reach and nightvision; light is not capped ("the light scale clamps the room") | `scaled_magnitude.go:35-47` |
| P7 | `cancel <spell>` ends any held condition whose spec carries `Cancellable` and whose name matches | `internal/usercommands/cancel.go:87-105` |
| P8 | No spell uses the alias `pall` or the id `chrysalis-pall`: grep `aliases` for `dark\|pall\|shroud\|murk\|gloom` finds only `empathic-shroud.yaml:3` (`[shroud]`) | grep |

### Items, slot, mob and room

| # | Fact | Where |
|---|---|---|
| I1 | `Worn.Light` (`yaml:"light"`), listed in `AllSlots` as `"light"`; `GetAllItems` walks `AllSlots` | `internal/characters/worn.go:34,67,133-141` |
| I2 | Mobs use the same `Character.Equipment Worn`, so a mob YAML `equipment: light: itemid: N` fills the slot. No shipped mob does: grep `^    light:` over `mobs/` finds 0 files, the same grep for `^    offhand:` finds 8 | `internal/characters/character.go:148`; grep |
| I3 | A spawned mob runs `Character.Validate(true)`, which calls `reapplyPermanentConditions`, which re-adds every worn item's `WornConditionIds` | `internal/mobs/mobs.go:772-773`; `internal/characters/validate.go:718-719`; `internal/characters/conditions.go:215,248-253` |
| I4 | Mob `conditionids` are applied with `SetPermanentConditions` at creation (mob 225 Pale Lurker ships `conditionids: [9, 85]`). Grep `conditionids:.*\b85\b` over `mobs/` finds five files: 9554, 9555, 225, 227, 229 (a block-list form `- 85` finds none) | `internal/mobs/mobs.go:123,678`; `mobs/ironwind_steppe/225-pale_lurker.yaml:5`; grep |
| I5 | Death loot: equipped items roll `item.ShouldDrop(m.ItemDropChance)`; `ShouldDrop` uses the item's own `DropChance` (`yaml:"dropchance"`) when set. No shipped mob sets `dropchance` on an EQUIPPED item (multiline grep over `mobs/` for an `equipment:` block entry followed by `dropchance` finds none; the same pattern without `dropchance` finds 9 files in `thornwall_city/`, 272 among them) | `internal/hooks/Death_MobLoot.go:47-63`; `internal/items/items.go:47,110-122` |
| I6 | The light folder holds Torch 20096 and Hooded Lantern 20097 (`type: light`, `subtype: wearable`, `wornconditionids: [126]` / `[127]`; the lantern carries a `hood` noun). Oil Lantern 40038 and Tallow Candle 40077 are `type: light` in `materials-40000/` | `items/armor-20000/light/`; grep `type: light` |
| I7 | `TestShippedLightItemsMatchTheLadder` pins the four lights' strengths, `light/wearable`, one secret condition each, and adjustability | `internal/items/shipped_light_items_test.go:15-70` |
| I8 | `hoodedLight` accepts any light-slot condition that `IsLightSource()` AND is `Adjustable` | `internal/usercommands/hood.go:19-38` |
| I9 | `equip` sends "You wear your X." and the room line "... puts on their X." through `SendTextVisual` for a wearable, then runs `lightnotice.Check` for a `type: light` item | `internal/usercommands/equip.go:152-176` |
| M1 | Chrysalis Phantom 272: `conditionids: [9]`, `itemdropchance: 10`, `hostile: true`, `maxwander: 0`, `speciesid: 1`, combat command `flee`, equipment weapon and offhand 10024, carried 20065 at `dropchance: 10`; no light slot | `mobs/thornwall_city/272-chrysalis_phantom.yaml:3,8,9,11,34,57,74-81` |
| M2 | The Phantom has no infravision today: species 1 human declares no vision conditions, and grep `infraredvision` over `mutations/` finds nothing (the same grep over the world finds conditions 85, 129, 130) | `species/1-human.yaml`; grep |
| M3 | Condition 85 InfraredVision: `nightvision_strength: 12`, `infra_reach: 30`, `infraredvision`, secret, `triggercount: 1` with no trigger rate (a pure flag, never ticks) | `conditions/85-infraredvision.yaml` |
| M4 | Its tree `behaviors/thornwall_city/272-chrysalis_phantom.yaml` ambushes through `mob_has_condition 9` + `players_in_room` + `attack` (`:91-100`) and re-hides through `players_in_room` (`:122`) | as cited |
| M5 | A fleeing mob moves by `actions.RelocateMob`, which calls `AddMob`, which trims | `internal/hooks/NewRound_DoCombat_helpers.go:947`; L12 |
| RM1 | Room 508 "Chitin Throne": `biome: cave`, no `lamp`, no `skylight`; spawns 272 every 30 real minutes; exits north 505, south 509 (both `biome: cave`) | `rooms/thornwall_city/508.yaml:16,30-32`; `505.yaml:15`; `509.yaml:15` |
| RM2 | Biome `cave`: `skylight: 0.0`, no `lamp` key, `indoor: true`. So 508's sky term is Absent (L2), it has no lamp, and its light today with nobody carrying one is 0 (L7) | `biomes/cave.yaml`; `internal/rooms/biomes.go:97-102` |

### Notices, GMCP, web client, help

| # | Fact | Where |
|---|---|---|
| N1 | Light notice causes: `movement`, `carried`, `lamp`, `weather`, `sky`, `eyes`; each needs a file under `narration/light-notices/<cause>.yaml` authoring all six transitions, or boot fails | `internal/lightnotice/store.go:24-29,44,77-98,174-176` |
| N2 | `attribute(prev, now)`: movement, then the eyes counterfactual, then `Carried`, lamp, weather, sky, else eyes | `internal/lightnotice/tracker.go:146-170` |
| N3 | `Check(user, trigger)` computes the player's band (`observe` uses `LightBand` on `room.LightTerms().Level`) on every call, and is called on every command (`usercommands.go:331`), each combat round (`hooks/NewRound_DoCombat.go:164`), each move (`hooks/LightNotice_Triggers.go:22`), login (`:29`), and after equip, remove, hood, unhood, cancel | `tracker.go:187-209,253-273`; grep |
| N4 | `light_notices.golden` snapshots every cause's pool | `internal/narration/testdata/stores/light_notices.golden`; `internal/narration/snapshot_test.go:864` |
| G1 | No GMCP field carries light or band: grep `-i "light\|dark\|band"` over `modules/gmcp/gmcp.Char.go` and `gmcp.Room.go` finds only `Bandolier`, the toxicity band and the `Char.Enemies` darkness comment (`gmcp.Char.go:443`), which proves the grep matches | grep |
| G2 | `Char.Vitals` is a no-`omitempty` snapshot republished on every pool change; toxicity rides it as a band name | `modules/gmcp/gmcp.Char.go:560-578,1067-1085` |
| G3 | Events are plain structs with `Type()` (`events.CharacterVitalsChanged{UserId}`), which the GMCP module listens to | `internal/events/eventtypes.go:434-438`; `gmcp.Char.go:60` |
| W1 | The Game window is `<section class="dash-panel" id="panel-feed">` with title "Game" and a pop-out button | `_datafiles/html/public/webclient-pure.html:317-321` |
| W2 | `.dash-panel` has `border: 1px solid var(--antique-gold, #b89047)`; `#panel-feed` is the dominant flex panel | `_datafiles/html/public/static/css/dashboard.css:63-69,130` |
| W3 | GMCP packages dispatch through `GMCPUpdateHandlers[path]` | `webclient-pure.html:525,2097-2098` |
| H1 | `help X` resolves `help-aliases` BEFORE templates, and `darkness` is today an alias of `light` (`light: [..., dark, darkness]`) | `internal/usercommands/help.go:161-166`; `_datafiles/world/dogmud/keywords.yaml:303-304` |
| H2 | `light`, `moons`, `seasons` are `general:` topics; each page's "See also" links the others | `keywords.yaml:148,165-167`; `templates/help/light.template:56-58`, `seasons.template:17`, `moons.template:16` |

### Free ids (NEW)

| # | Id | Evidence |
|---|---|---|
| F1 | Conditions **131, 132, 133** | `python tools/id_inventory.py --alloc conditions 3` prints "reserve IDs 131-133 ... past current global max (130)"; grep `^conditionid: 13[1-3]$` finds nothing, the same grep for `130` finds `130-pitsense_tincture.yaml` |
| F2 | Item **20098** in `armor-20000/light/` | `id_inventory.py --type items` shows `light 20096-20097 ... next-in-zone 20098`; grep `^itemid: 20098$` finds nothing, the same grep for `20097` finds the hooded lantern |
| F3 | Names `Umbral`, `chrysalis-pall` unused | grep over `_datafiles/world` finds neither |

## Owner decisions (binding, 2026-09-30)

1. **A darkness term below zero.** Darkness sources are light records with
   darkening polarity. Lights combine as today; darkness sources combine
   among themselves with the same halving rule (owner: mirror light's
   diminishing returns); the combined darkness is subtracted from the room's
   light; the level clamps at -100 as today. An unlit room still starts at 0.
2. **The inverted trim on the log scale.** Each darkness source solves
   against the others through the halving rule. A fresh cast or equip is full
   strength, the weapon (arc ruling 2); on the bearer's room entry it trims
   silently to the bottom of the bearer's usable range (arc ruling 1):
   `BlindBelow - nightvision strength`, or `-reach` with infravision. Events
   only; darkness arriving is never auto-countered by lights already present
   (arc ruling 3).
3. **The spell**, learned by discovery at `difficulty: 25`, scaling in glow's
   shape under new knobs shipped at glow's values, `cancel`-able like glow.
4. **The Umbral Lantern** (owner-named): a light-slot item, darkness 50 at
   full, adjustable, no hood.
5. **The Chrysalis Phantom** gains infravision deep enough to see shapes under
   its own darkness, carries the Umbral Lantern in its light slot, and drops
   it on death at its own chance.
6. **The Game-window border**: four tones from the player's own
   `LightBand`, a tooltip, one small GMCP field carrying only that band,
   updated at 3d's cadence.
7. **Help**: a `darkness` topic linked from `light`, `seasons`, `moons`.
8. **Free consequences** read the new level unchanged; a darkness record is
   never a light (no sneak beacon).
9. **Tests and gates** as in "Testing and gates". Out of scope as listed.

## Owner rulings on the source findings (binding, 2026-09-30)

Nine items. D1, D2, D3 and D8 change the shape of the work; the rest are
small. The owner took the recommendation as written for D1, D2, D3, D4, D5,
D6, D7 and D9. D8 is ruled AGAINST its recommendation: all shapes count as
seeing for a mob's decisions, not infravision shapes alone. The rules that
follow are written to these rulings.

**D1. The record model has no polarity (R1, R3, R7, R11 to R13).** A light
record is "a spec that declares `light_strength`", a literal of 0 or less is
refused at load, and seven readers key on `IsLightSource()` / `LightSources()`,
including `EmitsLight`. A `darkens` flag on a `light_strength` record would
make every one of them count darkness as light unless each is patched, and one
missed reader makes the dark a sneak beacon (owner item 8). **Ruled (owner,
2026-09-30):** a NEW effect kind `darkness_strength` (`EffectDarknessStrength`), in
`AllEffectKinds` and `ScaledKinds`, sharing the record's `LightTrim`,
`LightOutput` and `ResetLight` state. `IsLightSource()` stays light-only, so
`EmitsLight`, `hood`, the as-lit end line and every other light reader exclude
darkness by construction. NEW `IsDarknessSource()`, NEW
`Conditions.DarknessSources()`, and `LightMax` / `LightNow` read whichever of
the two kinds the spec declares. A spec declaring both is refused at load.
That is still "a light record with darkening polarity": one record shape, one
trim state, one solve; the kind carries the polarity.

**D2. `Trim`'s one `others` cannot carry a darkness solve (L3 to L6).** Under
the halving rule the room is `L - Combine(D_others, d)`: the solve needs the
light `L` AND the other darkness `D_others`, two numbers. The linear Darkens
branch has no production caller (L6). Note what the algebra gives: keeping the
room at or above `floor` means `Combine(D_others, d) <= L - floor`, which is
the light solve exactly, with `others = D_others` and `target = L - floor`.
**Ruled (owner, 2026-09-30):** delete the linear branch and the `Polarity` argument (its only
reader is `Trim`), keep `Trim(step, others, max, target)` as the one solve, and
add NEW `TrimDarkness(step, light, otherDark, max, floor float64) float64`:
Absent light reads 0; `budget := light - floor`; `budget <= 0` returns
Absent (the room is already at or below the floor, so this source is not
needed); otherwise `Trim(step, otherDark, max, budget)`. This is L5's
caller-side floor, applied once. One solve serves both polarities, so arc
ruling 1 ("one adjustment function") holds. Rejected alternative: keep
`Polarity` and give `Trim` a `light` parameter only Darkens reads; every
light caller would pass an unused argument.

**D3. The light trim must see darkness too (L8, L9, L11).** `TrimLightFor`
solves against `Raw`. With darkness in the room, `Raw` is net light, and
`Combine(net, s)` is the wrong algebra. **Ruled (owner, 2026-09-30):** `LightTerms` gains NEW
`Light float64` (the combined light, Absent when none) and `Dark float64`
(the combined darkness, Absent when none) and `Darkened bool`; `Raw` becomes
the net value `Level` rounds (`Light` read as 0, minus `Dark`). The light
branch of the trim solves `Trim(step, Light, full, target + Dark)`: a light in
a darkened room may run brighter before it dazzles, which is what the room
does. Finishing this sibling path is in scope, not a follow-up.

**D4. Band notices would blame the eyes (N1, N2).** A darkness arriving or
lapsing moves no current term, so `attribute` falls to `CauseEyes` ("your
sight changed"), and `carried`'s lines ("The carried light is gone") are wrong
for it. **Ruled (owner, 2026-09-30):** NEW `CauseDarkness = "darkness"`, checked first among
the terms (`a.Darkened != b.Darkened` or `Dark` moved), with a NEW
`narration/light-notices/darkness.yaml` authoring all six transitions;
`light_notices.golden` re-recorded with the new pool only.

**D5. `help darkness` opens `light` today (H1).** **Ruled (owner, 2026-09-30):** remove
`darkness` from `light`'s aliases, add the `darkness` topic under `general:`
with aliases `umbral`, `pall`; `dark` stays with `light`.

**D6. Start lines go out into the dark they announce (R16, R17, I9).** The
spell condition's start observer line is judged after the record lands, so
observers the darkness has just blinded miss "a pall gathers around X"; the
equip line for the Umbral Lantern likewise. This is the mirror of the light
end-line problem R16 already solves. **Ruled (owner, 2026-09-30):** a darkness source's start
room line, and the equip room line of an item whose worn condition is a
darkness source, are judged as lit (`SendTextVisualAsLitHidingNames` /
`SendTextVisualAsLit`). End lines need nothing: the room is lighter by then.
**Amended (owner, 2026-10-05):** the start and equip lines are judged against
a snapshot of the room taken before the darkness lands, not as lit
(`Room.VisualSnapshot` before the condition is added or the item worn,
`Room.SendTextVisualToSnapshot` after). Judged as lit, the line named the
caster to an observer who was already blind in a pitch-dark room; this is an
order of operations, not a new rule. `SendTextVisualAsLit*` stays for the
light end line (R16) only.

**D7. No GMCP field exists (G1 to G3).** **Ruled (owner, 2026-09-30):** a NEW package
`Char.Sight` with one field, `{"band": "dark" | "shapes" | "faces" |
"dazzled"}` (`Band.String()`), built from `messaging.LightBand` in
`GetCharNode` and included in the full `Char` payload. `lightnotice.Check`,
which already computes the band on exactly the 3d cadence (N3), queues NEW
`events.SightBandChanged{UserId}` when the band differs from the player's
recorded one (or there is none), and the GMCP module answers it with a
`GMCPCharUpdate` for `Char.Sight`. Not on `Char.Vitals`: that rides every pool
change and would recompute room light on each.

**D8. Infravision shapes do not count as sight for a mob's decisions (S11,
M4). Blocking.** `mobCanSee` is `SightFull` only. Under its own darkness the
Phantom reads SHAPES by heat, so `players_in_room` fails and its signature
ambush (M4) never fires; it would fight only when attacked. Today, in its lair
at 0 with no infravision, the same gate is false too, and the ambush fires
only when players carry at least 50 of light in. After 5d the players would
need 100, so the ambush is dead for good.

**Ruled (owner, 2026-09-30): NOT the recommendation.** ALL shapes count as
seeing for a mob's decisions: `mobCanSee` returns true at `SightShapes` from
ANY cause, natural dim light or infravision, not infravision alone. This
overturns slice F's own ruling, that a mob's sight is `SightFull` only and
infravision shapes are not seeing for it
(`specs/completed/2026-09-11-followup-slice-f-mobs-perceive-darkness-design.md`),
and it overturns this spec's own recommendation above, which would have kept
natural dim shapes not-seeing.

`mobCanSee`'s callers, verified by grep (no non-test caller exists outside
these three): `condPlayersInRoom`
(`internal/behaviortree/conditions_player.go:118`, target acquisition for the
ambusher archetype, the bandit leader and the Phantom), the enemy count at
`conditions_player.go:196`, and the party-engage aggro check at
`internal/behaviortree/actions_party.go:238`. All three now fire on shapes,
so every behaviour they gate shifts. Consequences: the Phantom ambushes under
its own darkness (M4); the five condition-85 mobs (I4) in dark rooms engage,
which is the parent's "infravision is the darkness weapon" and 5c's stated
expectation; AND, because the rule is now shapes from any cause, every mob
everywhere in a DIM room (the shapes band, S2, S4) with a player present now
acts on the figure it can make out, for ambush, aggro and targeting alike,
whether or not either side has infravision. That is a broad, balance-visible
change across every dim room in the world, not one scoped to the Phantom or
to infravision.

The fix must land in `behaviortree/sight.go`'s `mobCanSee`, not in
`messaging.CanSeeSightImpairedOnly`, which it calls today. That function is
shared with combat: `internal/hooks/NewRound_DoCombat_resolution.go:98,111`
and `internal/hooks/NewRound_DoCombat_unified.go:565,568` read it to gate
`Balance.DarknessCombatPenalty` on each side of a fight, and its own doc
comment warns that widening it "would hand every infrared character a silent
balance change", the concern behind the 2026-09-20 owner ruling that infrared
takes a REDUCED, not a zero, combat penalty
(`specs/completed/2026-09-20-messaging-m4d-send-path-design.md`, ruling 6).
So `mobCanSee` must call `messaging.ParticipantSight` directly and accept
`SightFull` or `SightShapes` there, leaving `CanSeeSightImpairedOnly` and the
combat darkness penalty untouched; only mob decisions widen.

**D9. Details the design left open.** **Ruled (owner, 2026-09-30):** spell `targeting:
single` (glow's; cast on another, it trims to that holder's eyes, P5), `cost:
50` (between Night Vision 45 and Heat Sight 80), `waitrounds: 3`, school
mental. Knob yaml names keep the `Light` family prefix, `LightDarknessSpell*`,
which the accessor exposes as `DarknessSpell*` (K10). The Phantom's heat
sense is a NEW literal condition at reach 50 rather than condition 85 (reach
30 reads nothing at -50), with no `nightvision_strength`, so it is not made
more easily dazzled; given through `conditionids`, no species change (I4).

## The rules

### Rule 1: darkness is a record kind (D1)

- `EffectDarknessStrength EffectKind = "darkness_strength"` (NEW): a literal
  for an item, `magnitude` for a spell. `validateEffects` refuses a literal of
  0 or less, a spec with both kinds, and a stacking darkness source;
  `adjustable` requires one of the two kinds.
- `Effect()` returns 0 for it, as for light (R4).
- `AddConditionMagnitude` resets the record for either kind (R9).
- `LightTerms()` and `EmitsLight()` are unchanged and read light only (R12).
  NEW `Character.DarknessTerms()` mirrors `LightTerms()`.

### Rule 2: composition (owner item 1, D3)

`composeWith` takes the carried light terms and the carried darkness terms
(NEW second slice, from a `carriedDarkness(exclude)` twin of L10, or one pass
returning both):

```
light := Combine(step, sky, lamp, carriedLight...)     // Absent when none
dark  := Combine(step, carriedDark...)                 // Absent when none
net   := (light, or 0 if Absent) - (dark, or 0 if Absent)
Level := clamp(round(net), -100, 100)
```

`Carried` keeps meaning "someone carries a light"; `Darkened` means "someone
carries a darkness".

| Room | Light | Darkness | Level |
|---|---|---|---|
| 508 today, nobody carrying | none (0) | none | 0 |
| 508 with the Phantom's lantern | none (0) | 50 | -50 |
| 508, lantern, a torch (56) | 56 | 50 | 6 |
| 508, lantern, endgame glow (90) | 90 | 50 | 40 |
| 508, lantern, glow 90 and torch 56 | 90.6 | 50 | 41 |
| Tavern lamp 50, one darkness 50 | 50 | 50 | 0 |
| Open ground at equinox noon (70), new caster's pall (50) | 70 | 50 | 20 |
| The same, endgame pall (90) | 70 | 90 | -20 |
| Cave, two darkness 50 | 0 | 58 | -58 |
| Cave, darkness 50 and 40 | 0 | 54.05 | -54 |
| Cave, three endgame palls | 0 | 102.7 | -100 (clamped) |

### Rule 3: the trim (owner item 2, D2, D3)

NEW `messaging.DarknessTrimTarget(strength, reach, blindBelow int) float64`:
`-reach` when `reach > 0`, else `max(blindBelow - clampShift(strength),
windowFloor)` (S2: natural shapes need `light >= windowFloor`). Exactly the
floor, not a point inside it: the band edge counts as usable, and a solved
net within rounding of the floor rounds onto it.

Amended (5d playtest, 2026-10-05): the target is one point INSIDE the floor
(`-reach + 1`, else the clamped edge `+ 1`), mirroring `LightTrimTarget`. A
room parked exactly on the floor tipped a still bearer on a sunlit street
into the dark at the first downward drift of the afternoon sky. In the table
below every row the trim cuts now leaves the room one point higher (25 reads
26, 1 reads 2, -30 reads -29, a cave Phantom with no lamp -49) with the
output cut to match; the "off" rows and the "full" row at light 90 and at
infravision 50 in light 50 are unchanged.

`TrimLightFor` (kept name; it now trims both kinds) collects the bearer's
adjustable, unhooded light AND darkness records in held order, sets them all
off, then solves each in turn against the room as the earlier ones left it:

- light: `Trim(step, terms.Light, full, lightTarget + dark)`, where `dark` is
  `terms.Dark` or 0;
- darkness: `TrimDarkness(step, terms.Light, terms.Dark, full, darkTarget)`.

`terms` is `composeLightExcluding(..., rec)`, extended to leave the record out
of whichever combine it belongs to. Callers unchanged (L12); a spawn still
does not trim (L13).

Trim table for an Umbral Lantern (full 50) entering alone:

| Bearer's eyes | Floor | Room light | Other darkness | Output | Room after |
|---|---|---|---|---|---|
| normal | 25 | 0 (cave) | none | off | 0 |
| normal | 25 | 50 (tavern) | none | 25 | 25 |
| normal | 25 | 70 (noon) | none | 45 | 25 |
| normal | 25 | 90 | none | 50 (full) | 40 |
| nightvision 24 | 1 | 50 | none | 49 | 1 |
| infravision 30 | -30 | 0 | none | 30 | -30 |
| infravision 50 (the Phantom) | -50 | 0 | none | 50 (full) | -50 |
| infravision 50 | -50 | 50 | none | 50 (full) | 0 |
| normal | 25 | 50 | 20 | 12.9 | 25 |
| normal | 25 | 50 | 30 | off | 20 |

And one light row: a hooded lantern (54) on normal eyes entering open ground
at equinox noon (70) where someone's darkness 50 already sits solves against
a target of 74 + 50 on the light combine and runs at full, room 23. The same
lantern entering a room of light 74 with that darkness also runs at full
(room 26), where without it the lantern would go off.

"Off" in a cave is the inverse of a light going off in daylight: the room is
already past the bearer's usable edge without the source, so it is not
needed. A fresh cast or equip is still full strength until the next move.

### Rule 4: the spell (owner item 3, D9)

**Chrysalis Pall** (`spells/chrysalis-pall.yaml`, spellid `chrysalis-pall`,
alias `pall`), paired with Chrysalis Glow: spores that drink light where
glow's spores give it. Mental, single target, willpower, `cost: 50`,
`waitrounds: 3`, `difficulty: 25` (discovery at spellcasting 25, P3; base cast
chance 75, P4), `effect_type: condition`, `condition_ids: [131]`. Found by
discovery, never taught (a header comment, as `heat-sight.yaml`).

Condition **131** "Chrysalis Pall" (NEW): `darkness_strength: magnitude`,
flags `adjustable`, `cancellable`, not secret (it lists, and its duration
tells the caster when to recast, as glow's does), `triggerrate: 5 real
minutes` like condition 1.

Scaling: `magnitudeSpellApplication` picks the knob trio by kind, and the
duration trio by kind too (darkness reads its own; the other three kinds keep
glow's, P5). NEW knobs in `config.balance.go` beside K4/K5, coerced in
`config.balance.lighting.go` like them, exposed on `Lighting`:

| yaml key | Go default and shipped |
|---|---|
| `LightDarknessSpellStrengthBase` | 40 |
| `LightDarknessSpellStrengthStatDivisor` | 10 |
| `LightDarknessSpellStrengthSkillDivisor` | 2 |
| `LightDarknessSpellDurationBase` | 2 |
| `LightDarknessSpellDurationStatDivisor` | 50 |
| `LightDarknessSpellDurationSkillDivisor` | 20 |

A new caster (100, 0) casts 50 for 4 triggers (20 minutes), a mid caster
(130, 30) 68, an endgame caster (175, 65) 90 for 9 (45 minutes). No cap
beyond the scale's clamp (P6). The plan writes the six keys into
`config.yaml` under a "LIGHT: DARKNESS SPELL (lighting plan 5d)" block after
the 5c block, building the commit from the `git show HEAD:` blob and never
from disk (K11, `dogmud-balance-config`).

Text (80 columns, no numbers, `dogmud-player-copy`):

- `cast_actor`: "You coax the Chrysalis spores around you to drink the light."
- `cast_observer`: "{actor} concentrates as the air around them dims and thickens."
- `wait_actor`: "You hold the image steady as the spores darken..."
- 131 `start_actee`: "A pall of dark spores gathers around you."
- 131 `start_observer`: "A pall of dark spores gathers around {actee_plain}." (as lit, D6)
- 131 `end_actee`: "Your pall thins, and the light comes back."
- 131 `end_observer`: "The pall around {actee_plain} thins away."
- 131 `description`, in 5c's warning style: it darkens the place for
  everyone, you included; it eases to what your own eyes can still use each
  time you move; the warmth of living things still shows through it to eyes
  that see heat.

`cancel pall` / `cancel chrysalis-pall` end it (P7).

### Rule 5: the Umbral Lantern (owner item 4)

Item **20098** `items/armor-20000/light/20098-umbral_lantern.yaml` (NEW):
`name: Umbral Lantern`, `namesimple: lantern`, `type: light`, `subtype:
wearable`, `wornconditionids: [132]`, no `nouns`, no `vendor_categories` (a
boss drop, not stocked). Description, wrapped at 80: a lantern of black glass
and tarnished iron in which a knot of dark spores drinks the light; it trims
its darkness to your eyes each time you step somewhere new.

Condition **132** "Umbral Dark" (NEW): `secret: true` (one entry per light,
5a's ruling; the item shows in the equipment panel), `darkness_strength: 50`,
`adjustable`, `triggerrate: 5 real minutes`, `triggercount: 1`, the shape of
127. `hood` refuses it with the existing "has no hood." line (I8 skips a
non-light record).

### Rule 6: the Chrysalis Phantom (owner item 5, D8, D9)

- `conditionids: [9, 133]`. Condition **133** "Phantom Heat Sense" (NEW):
  secret, `infra_reach: 50`, flag `infraredvision`, `triggercount: 1` and no
  trigger rate, the pure-flag shape of 85 (M3).
- `equipment: light: itemid: 20098, dropchance: 25`. The per-instance chance
  overrides the mob's `itemdropchance: 10` for this item only (I5).
- `maxwander: 0` stays. A spawn neither trims nor moves (L13), so the lantern
  is at full: 508 reads **0 before, -50 after**, and the Phantom's reach 50
  reads shapes at exactly -50 (`light >= -reach`, S2) with no dark penalty
  (5c's ramp is 1.0 at the cap). When it flees (M5) the lantern trims to its
  floor, -50, which in a cave is still full.
- Another darkness brought in pushes the lair below -50 and blinds it: a
  second darkness 50 makes -58.
- Under D8's ruling `mobCanSee` accepts `SightShapes` from any cause, so the
  Phantom reading shapes at its own reach 50 passes `players_in_room` and its
  ambush fires (M4). The fix lives in `mobCanSee`
  (`internal/behaviortree/sight.go`), not in `messaging.CanSeeSightImpairedOnly`,
  which combat's darkness penalty also reads (D8).

### Rule 7: the Game-window border (owner item 6, D7)

`Char.Sight {"band": ...}` as D7. The web client adds a
`GMCPUpdateHandlers['Char.Sight']` that sets one of four classes on
`#panel-feed` and a `title` on its header:

| Band | Border tone | Tooltip |
|---|---|---|
| dark | near black, a darkened antique gold | "Too dark to see." |
| shapes | dim, a muted gold | "Dim light: shapes, not faces." |
| faces | today's antique gold `#b89047` | "Good light." |
| dazzled | bright, a pale hot gold | "Too bright: the glare hurts." |

Tones are CSS custom properties in `dashboard.css`. The plan checks whether
the pop-out moves `#panel-feed` into its window or copies it, and puts the
class where the border is drawn in both states. The band is the player's own
and the text already tells them, so no room contents reach GMCP.

### Rule 8: help (owner item 7, D5)

- `templates/help/darkness.template` (NEW, `general:`, aliases `umbral`,
  `pall`): darkness is light taken away; it can go below the darkest cave;
  two darknesses add less than double; a pall or an Umbral Lantern trims to
  your own eyes when you move, and a fresh one is at full; lights already in
  a room do not push back; infravision is the answer to it, since heat shows
  a shape in darkness no eye can use; names Chrysalis Pall and the Umbral
  Lantern; a darkness is not a light, so it never gives you away when
  sneaking.
- `templates/help/chrysalis-pall.template` (NEW), in `heat-sight.template`'s
  shape.
- `light`, `seasons`, `moons` gain `help darkness` in "See also"; `light`'s
  alias list loses `darkness`.

## What changes in play

- **Rooms can be darker than any cave.** A pall or an Umbral Lantern takes
  light away; normal eyes see nothing below 25, nightvision nothing below 1,
  and only infravision reads shapes below that, down to minus its reach.
- **A fresh pall is a weapon.** Cast in a lit tavern by an endgame caster, it
  drops the room to -40 for everyone until the caster next moves; on entering
  a new room it trims to what the caster can still use.
- **Lights fight back only by arriving.** A torch carried INTO a darkened room
  adds its light; lights already there do not re-trim (arc ruling 3).
- **The Chitin Throne is black.** Room 508 reads -50 while the Phantom lives.
  A party with no infravision fights at the full dark penalty unless it brings
  heavy light in (a torch leaves it at 6, still dark to normal eyes); a
  Heat Sight caster reads shapes, at reach 50 with no penalty. With D8 the
  Phantom ambushes from the dark.
- **Every dim room's mobs now act on what they can make out.** D8 is ruled
  against its recommendation: `mobCanSee` accepts `SightShapes` from ANY
  cause, natural dim light or infravision, not infravision alone. A mob does
  not need infravision, and a player does not need to be seen clearly, for
  `players_in_room`, enemy counting or party aggro to fire; the shapes band
  (S2, S4) is enough everywhere, not only in the Phantom's lair. This is a
  broad, balance-visible change, overturning slice F's ruling that a mob's
  sight is `SightFull` only.
- **A new drop.** The Umbral Lantern, one in four kills.
- **The Game window's border** shows how well you can see.
- **Unchanged by construction:** sight gates, the shapes and blind tiers,
  dazzle and darkness penalties, crime witnesses and sneak contests all read
  `LightLevel()` (S2 to S9) and so read the new level with no change. A
  darkness bearer does not `EmitsLight()` (D1): no sneak beacon, no `lit`
  adjective, sleepers are not woken by it.

## What this overturns

| Earlier | Now |
|---|---|
| Parent arc ruling 1: "Darkness ITEMS are out of scope" | The Umbral Lantern ships in 5d (owner, 2026-09-30) |
| Parent 5a out of scope: "Mobs deliberately carrying lights: no content" | The Phantom carries a darkness in its light slot |
| 5a `Trim` Darkens: a linear cut (L5) | The darkness combine solved by the same log solve (D2) |
| 5a `LightTerms.Raw` is the light combine | `Raw` is net light; `Light` and `Dark` carry the two combines (D3) |
| Slice F ruling: a mob's sight is `SightFull` only, infravision shapes are not seeing for it (`specs/completed/2026-09-11-followup-slice-f-mobs-perceive-darkness-design.md`) | `mobCanSee` accepts `SightShapes` from any cause, natural dim light or infravision (owner ruling, D8, 2026-09-30, NOT this spec's own recommendation) |

## Testing and gates

- **`lightscale`**: `TrimDarkness` table (the Rule 3 rows, including off in a
  cave, the cap, other darkness below and above the budget) with each result
  checked by recomputing the room with `Combine`; `Trim` keeps its light
  tables; the old Darkens tests are replaced.
- **Composition** (`composeWith`): the Rule 2 table; darkness against sky,
  lamp and carried light; stacking; the -100 clamp; an unlit room with no
  darkness is still 0; `Carried` and `Darkened` independent.
- **Trim, both polarities** (`TrimLightFor`): the Rule 3 table through a real
  room; a bearer with a light and a darkness in held order; the light row
  with darkness present; entry order across three bearers; nobody re-trims on
  a later arrival; a spawn does not trim.
- **Records**: `validateEffects` refusals (literal 0, both kinds, stacking,
  adjustable without a kind); `LightNow` for a darkness record; the magnitude
  reset on recast.
- **Guard: darkness is never light.** A character holding 131 and 132 has
  empty `LightTerms()`, `EmitsLight()` false, `CalcSneakScore` takes the
  no-light branch, and `hood` refuses. Proven able to fail by temporarily
  making `IsLightSource()` accept `darkness_strength`.
- **Spell round trip**: cast at the three reference casters (magnitude and
  triggers, knobs pinned with `SetConfigForTest`; a test binary never loads
  `config.yaml`); full strength on cast and recast; `cancel pall`; discovery
  eligibility at spellcasting 24 (no) and 25 (yes).
- **Lantern round trip**: equip sets full, a move trims, swapping gloves keeps
  the trim, re-equip resets; `TestShippedLightItemsMatchTheLadder` gains a
  darkness row (20098, secret, 50, adjustable, no hood noun).
- **The Phantom's lair**: load the shipped world, spawn 272 in 508, assert
  level -50, its `InfraReach()` 50, `LightBand` shapes for it and dark for a
  normal player, the lantern in its light slot with drop chance 25; with D8,
  `mobCanSee` true for it and false for a normal-eyed mob there (at -50 a
  normal-eyed mob is still below `windowFloor`, so D8's widening does not
  reach it).
- **D8's widened `mobCanSee`**: a mob in a DIM (shapes-band) room with a
  player present now passes `mobCanSee` / `condPlayersInRoom`, regardless of
  infravision on either side; a mob in a genuinely dark room, below the
  shapes floor, with no infravision still does not. Proven able to fail by
  reverting `mobCanSee` to `SightFull` only.
- **Notices and GMCP**: the darkness cause fires on a cast and on expiry, not
  `eyes`; `Char.Sight` is queued on a band change and not on a repeat.
- **Goldens**: `light_notices.golden` and the conditions snapshot gain only
  the new entries; the lighting parity and daycycle goldens must not move
  (nothing sampled carries darkness), and any move is explained with
  `tools/lighting_golden_diff.py` before re-recording.
- **Docs**: `context.md` for `lightscale`, `conditions`, `rooms`,
  `messaging`, `lightnotice`, `characters`, `behaviortree` (D8) and
  `modules/gmcp`, each symbol checked with `tools/context_md_audit.py`; patch
  notes; the new files in `docs/README.md`.
- **Gate**: the full local gate, a `-race` run, and a boot check on private
  ports (CI minutes are exhausted for September). No AI-companion boot check
  (arc ruling 10).
- **Playtest** (`playtest-scenario`, ending with the adversarial content
  gate): a party walks into 508 once with no light and once with a torch and
  a glow, reporting band and what they see of the Phantom (and, with D8,
  whether it ambushes); a Heat Sight caster reads shapes there; a caster at
  spellcasting 25 casts Chrysalis Pall in a lit tavern, the others report the
  drop and the border, the caster moves out and back and reports the trimmed
  level; `cancel pall` restores the room; a kill of the Phantom with the
  lantern forced to drop (admin) is equipped and walked through a lit street
  and a cave.

## Out of scope

Other mobs using darkness; darkness potions; scheduled darkness and scheduled
light (5e); a darkness adjective on `look`; Mudlet UI for `Char.Sight`; the
open "GMCP bypasses darkness" leak for room contents; plan 6's infrared edge
work.
