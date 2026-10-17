# Split the slam attack by body

Part of #541 (combat-message reachability), under #385. It is its own slice
between slice 1 (species corrections, #568) and slice 2 (reachability and
beast moves) of `docs/superpowers/specs/2026-10-16-combat-message-reachability-design.md`.
It replaces that plan's "slam lines that only fit fists" follow-up (D6).
Owner rulings 2026-10-16.

## Facts verified against source (master `db6e7051b`, 2026-10-16)

| # | Fact | Source |
|---|---|---|
| 1 | 13 species use `natural_attack: slam`: fungal colony (15, "spore tendrils", body `[skin]`), slime (16, "pseudopods", `[skin]`), horse (26, "hooves"), the water, earth, air, fire, magma, sand, storm, ice and smoke elementals (36 to 44: "watery surge", "stone fists", "slicing gust", "scorching strike", "molten slam", "abrasive whirl", "arcing strike", "crystal strike", "ember lash"), and automaton (45, "servo-driven fists", `[arms, legs, skin]`) | `grep natural_attack: slam species/` |
| 2 | `combat-messages/slam.yaml` holds 115 records (actor, actee and observer per moment) for all of them; `{itemname}` is the species' `unarmedname` | file |
| 3 | The `{itemname}` audit (`docs/superpowers/audits/2026-10-16-slam-itemname-audit.md`, 407 of 407 lines read) flags 24 lines and 4 companion lines: a line that hurls, smashes, drives or flicks `{itemname}` fails for one kind of name ("hurl your hooves", "smash your molten slam into"); 3 lines already pass 80 columns (147, 318, 360); lines give slimes, fungal colonies and the air, water, smoke and storm elementals feet, a head, a chest or a shoulder they lack | audit |
| 4 | An equipped weapon's subtype overrides the species attack (`buildWeaponSetup`, `internal/combat/combat_helpers.go:430-465`). In a paid instance a mob equips its `loot_pool` gear (`internal/rooms/rooms.go:1157-1173`) | files |
| 5 | Who fights with the species attack: the Planar Oasis sand and storm elementals (318, 319; gear but no weapon), the summons 310 to 314 (water, earth, air, fire, magma; unless a player gears them), the Eastern Highlands earth wards (9550 to 9552), slimes and fungal colonies, automatons, Hob and Bran (horse). Who does not: the Elemental King (magma, Earthshaker Warhammer), Queen (ice, Crystal Sceptre), Prince (smoke, Wind Scimitar) and Princess (water, Drowned Claws) | mob files, `loot_pool` items |
| 6 | The Elemental Prince (mob 322) is the only smoke elemental; its scripted combat emotes already read "strikes from an impossible angle", "dissolves into mist and reforms behind its target" | file |
| 7 | Conditions that a smother can build on: `3-blinded.yaml` ("Your vision is obscured"), `89-throttled.yaml` ("A crushing grip on your throat", applied by the `throttle` move: `internal/actions/combat_throttle.go:149`) | files |
| 8 | A beast special move is built from: an action (`internal/actions/combat_<move>.go`), a mob command (`internal/mobcommands/<move>.go`), AI `CanUse<Move>` / `Score<Move>` (`internal/combat/ai.go`), readiness (`internal/actions/command_readiness.go`), its narration (`narration/special-moves/<move>.yaml`), balance knobs (`internal/configs/config.balance.go`), and a divergence entry when it is mob-only (`internal/actions/divergences.go`) | `throttle` and `charge` files |
| 9 | Adding a natural attack type means a subtype constant and description (`internal/items/itemspec.go`), an entry in `validNaturalAttacks` (`internal/species/species.go:212`), a hit category (`CategoryForWeaponSubtype`, `internal/combat/attackresult.go:261`) and a combat-message file of the same name; `siphon` (#538) is the model | #538 |

## Owner rulings (2026-10-16)

- **S1.** `slam` carries too much. Split it by body: each group gets a file whose lines say what that creature does, which also keeps lines short.
- **S2.** The groups and names: `slam` for heavy bodies (earth elemental, automaton); `trample` for the horse; `lash` for the amorphous (slime, fungal colony); `blast` for the energy elementals (water, air, fire, magma, sand, storm, ice).
- **S3.** The smoke elemental's default attack is `claws`: it materialises claws and rakes at you. Its signature is a new `smother` special move (smoke pours into your face: blind and choking). Option A: both are in scope; `smother` shows on the Prince between scimitar swings, since special moves do not depend on the weapon.
- **S4.** No spillover: every line in the four files fits 80 visible columns measured with the longest names and tokens.
- **S5.** This is a full slice: spec, plan, build, validate.

## Design

### 1. Four attack types

| Type | Species | `unarmedname` today | Hit category | File |
|---|---|---|---|---|
| `slam` (kept) | earth elemental 37, automaton 45 | stone fists, servo-driven fists | `CategoryHitBlunt` (as today) | `slam.yaml`, rewritten for fists and bulk |
| `trample` (new) | horse 26 | hooves | `CategoryHitBlunt` | `trample.yaml` |
| `lash` (new) | slime 16, fungal colony 15 | pseudopods, spore tendrils | `CategoryHitBlunt` | `lash.yaml` |
| `blast` (new) | water 36, air 38, fire 39, magma 40, sand 41, storm 42, ice 43 | as today | `CategoryHitCaster` (the caster-hit category `siphon` uses) | `blast.yaml` |
| `claws` (existing) | smoke 44 | **smoke-wrought claws** (was "ember lash") | `CategoryHitNaturalSharp` | `claws.yaml` |

Each new type is wired as `siphon` was (Fact 9). Species switch in the same change, and the `unarmedname` values stay unless the table says otherwise; the plan may propose a tidier name for an elemental if its current one reads badly in the new file, for owner review.

### 2. The copy

- Each new file follows the store's record layout (one record per moment, every role describing the same moment, the M6 rules, the banner and token guards) and holds the same intensities and tiers as `slam.yaml`.
- Lines say what the group does: `trample` kicks, rears and tramples; `lash` whips, smothers and engulfs; `blast` names the element through `{itemname}` and strikes with it. `slam` keeps fists, bulk and weight.
- No line names a body part its group lacks: no feet, head, chest or shoulder in `lash` or `blast`; `trample` may use hooves, legs, head and chest.
- `{itemname}` is used only in frames that read right for every species in that file (the audit's "with your {itemname}" frame is one).
- Width (S4): measured by script with 20-character names and the longest value of every token.
- Drafted by agents from the audit and the existing `slam.yaml`, then each file goes to the owner as a review table (today or new, proposed, why) before it lands, the way M6's batches did.

### 3. `smother`, the smoke elemental's special move

- Mob-only (no player can be a smoke elemental): an action `combat_smother.go`, a mob command, `CanUseSmother` / `ScoreSmother` in `ai.go`, a divergence entry, its narration file `narration/special-moves/smother.yaml`, and balance knobs in `config.balance.go` surfaced through `config.yaml`.
- Effect on a hit: the target is `Blinded` (condition 3) and choking, for a few rounds. Choking is a new condition, `Smothered` (smoke in the lungs), modelled on `Throttled` (89) but with its own text, since 89 speaks of a grip on the throat.
- Granted to the smoke species through slice 2's `beast_moves` list, so `smother` lands with slice 2 (it needs that list for its gate). This slice designs it, writes its narration and condition, and the plan sequences its code after `beast_moves` exists.
- The Prince's scripted `combatcommands` emotes stay.

### 4. Docs and comments

- `internal/species/context.md`: the four groups and why (S1), how to choose a type for a new species.
- Each new combat-message file's header: which species use it and what its lines may and may not name (S4 and the body-part rule).
- `internal/combat/context.md`: the new types and their hit categories.

## Testing

- A table test pins each species in section 1 to its new type, and the smoke elemental's `unarmedname`.
- Each new file passes the existing combat-message guards (record pairing, banner, token, tag spacing) and a width test over every line with the longest names and tokens.
- A body-part guard for the new files: `lash.yaml` and `blast.yaml` contain none of feet, head, chest, shoulder, footwork.
- A render test per type: a slime, a horse, a sand elemental and an automaton each produce a line from their own file.
- Goldens re-recorded only for the deliberate changes.
- Boot check, and a playtest: a horse, a slime and a summoned fire elemental fight a player while a second player watches; both seats read the same blow, no line wraps past 80.

## Out of scope

- `smother`'s code before slice 2's `beast_moves` (section 3).
- A `peck` attack for birds, `kick` for horses as a special move, an arachnid cave lurker (later, per slice 1's notes).
- The armed elementals' weapon copy.

## Risks

| Risk | Answer |
|---|---|
| Four new files is a lot of copy to review | Agents draft; the owner reviews one table per file; the audit already lists what `slam` gets wrong |
| A new hit category changes colour or wrapping | `blast` takes `CategoryHitCaster` (as `siphon`); `trample` and `lash` stay on `CategoryHitBlunt` with `slam`; the plan pins each and checks `shouldWrap` |
| Slice 2 changes the same species files | This slice touches only `natural_attack` and the smoke `unarmedname`; slice 2 adds `beast_moves`; the plan for slice 2 rebases on it |
