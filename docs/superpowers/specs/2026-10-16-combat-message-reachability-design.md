# Combat-message reachability and species beast moves

Issues #541 (combat-message reachability) and #539 (unreachable throttle,
pounce and maul files), under the combat narration epic #385. Owner rulings
recorded on both issues on 2026-10-16. This spec designs the whole change;
the plan slices it.

## Facts verified against source (master `779746a31`, 2026-10-16)

| # | Fact | Source |
|---|---|---|
| 1 | `items.Prepare` is never requested. The only caller of `items.GetPreAttackMessage` for a pre-attack intensity is `combat.GetWaitMessages` (`internal/combat/combat.go:214`), called once, with `items.Wait` (`internal/hooks/NewRound_DoCombat_resolution.go:92`). Fumble and coup de grace also go through `GetPreAttackMessage` (`combat_helpers.go:1736,1746`) | grep |
| 2 | Every one of the 20 files in `_datafiles/world/dogmud/combat-messages/` has a `prepare` pool and a `wait` pool | per-file grep |
| 3 | The engage lines are hand-written: the player `attack` command, `internal/usercommands/attack.go:288` ("You prepare to enter into mortal combat with %s.") plus room line `:296`, and `:380` plus `:389,394` for a player target; the mob `attack` command, `internal/mobcommands/attack.go:98,103,153` ("%s prepares to fight you!") | grep "mortal combat", "prepares to fight" |
| 4 | `GetWaitMessages` picks the pool from `sourceChar.Equipment.Weapon.GetSpec().Subtype` only (`combat.go:218`); wait lines play during the engagement windup (`ConsumeRoundWaiting`, `NewRound_DoCombat_resolution.go:86`) | file |
| 5 | The swing path's rule: `buildWeaponSetup` (`combat_helpers.go:430-465`) starts from `items.Unarmed`, takes the species `NaturalAttack` when it has one, and takes an equipped weapon's subtype over both | file |
| 6 | `meleeDisplaySubtype` (`combat_helpers.go:1583`) narrates a `Shooting` weapon as `Bludgeoning` in round combat, on purpose; the `shoot` command uses `narration/special-moves/shoot.yaml` | file |
| 7 | Round combat never runs across rooms: `NewRound_DoCombat_unified.go:219` releases the target when the rooms differ. The round store's `separate` split (`combat_helpers.go:1828,1860`) is therefore never rendered. 17 `separate` blocks exist: 9 in `generic.yaml`, 8 in `shooting.yaml` | file, grep |
| 8 | Natural attacks allowed today: bite, claws, slam, gore, sting, siphon (`internal/species/species.go:212-219`, `validNaturalAttacks`; each needs a combat-message file of the same name) | file |
| 9 | Hit categories by subtype: `CategoryForWeaponSubtype` (`internal/combat/attackresult.go:261`); claws, bite, sting and gore ride `CategoryHitNaturalSharp` | file |
| 10 | Siphon was wired as a natural attack in #538 by: a subtype constant (`internal/items/itemspec.go:178`) and its description (`:108`), `validNaturalAttacks`, a hit category, species data, and a species validator | `git log` of #538 |
| 11 | AI beast moves are chosen by `CanUse*` gates in `internal/combat/ai.go` (rake 529, maul 555, pounce 577, gore 674, drain 702, throttle 818, hamstring 385) and player readiness by `internal/actions/command_readiness.go:103-132`. Both read attack-family predicates: `SpeciesIsFanged` (bite), `SpeciesIsClawed` (claws), `SpeciesIsHorned` (gore), `SpeciesIsQuadrupedPredator` (no hands, legs, fanged or clawed) and `SpeciesHasLifeDrain`. The action entry points repeat them (`actions/combat_maul.go:83`, `combat_throttle.go:95`, `combat_rake.go:83`, `combat_gore.go:78`, `combat_hamstring.go:92`) | grep |
| 12 | A mob can also be scripted to use a move through its `combatcommands` list (e.g. `charge` and `bash` on `mobs/cascade_pass_road/9539-a_tusked_forest_boar.yaml`); `charge` has no AI gate and is mob-only (`internal/actions/divergences.go:188`) | grep |
| 13 | Natural attack counts: bite 10 species, claws 7, slam 12, gore 2, siphon 2. Mobs: 2 bears, 4 cats, 19 serpents | grep |

## Owner rulings (2026-10-16)

- **A.** `prepare` is shown at the engage moment, drawn by the same weapon-type rule as the swing path, and replaces the hand-written engage lines (player and mob).
- **B.** `wait` follows the swing path's weapon-type rule, so a creature waits with its own natural attack.
- **C.** What cannot render is retired: `shooting.yaml`'s swing intensities and every `separate` block, with their render path. `shooting.yaml` keeps `prepare` and `wait`.
- **D.** Bears' default attack becomes `maul`, cats' becomes `pounce`. Serpents stay on bite. No python species (none exist). `throttle` stays a special move only; the default-attack `combat-messages/throttle.yaml` is retired.
- **E. Principle.** A species' default attack is what it does most of the time. Its special moves are rarer or finishing moves.
- **F.** Which special moves a species can use becomes species data, reviewed as a table (section 5). `throttle` goes to the throat crushers: cats, canines, and any species the table names. Bears do not get the `maul` special move; their basic swings use maul lines.

## Design

### 1. One weapon-type rule

A single function, `combat.AttackSubtype(char *characters.Character) items.ItemSubType`, holds the rule now inside `buildWeaponSetup` (Fact 5): equipped weapon's subtype, else species natural attack, else `items.Unarmed`. `buildWeaponSetup`, `GetWaitMessages`, the engage sender (section 2) and the fumble and coup de grace lookups all call it. A test pins it for a weapon user, a natural attacker and a bare-handed humanoid.

### 2. Engage lines from `prepare`

`combat.EngageMessages(attacker, defender *characters.Character, ...)` renders ONE coordinated draw from the attacker's `prepare` pool, chosen by `AttackSubtype` directly (not `meleeDisplaySubtype`, so a bow user's engage line is `shooting.yaml`'s own, as its wait lines already are), with the same tokens the wait path fills, and returns the attacker, defender and room lines. The engage sites (Fact 3) send them on `CategoryForWeaponSubtype(subtype)` with the names hidden at each reader's sight, as `attack.go` does today, and replace the hand-written lines. The double-announce guards beside those sites (`characters/character.go:297`, `mobcommands/attack.go:139`) stay as they are: one engage line per engagement.

### 3. Retire what cannot render

- Every `separate:` block leaves the round store, with the `SeparateMessages` type, its `Render`, and the `together` branch at `combat_helpers.go:1828-1860` (the together render becomes the only path). A test fails if any combat-message file has a `separate:` key.
- `shooting.yaml` keeps only `prepare` and `wait`. The validator's "every intensity present" rule exempts the swing intensities for exactly the subtypes `meleeDisplaySubtype` re-narrates (read from that function, so the two cannot drift).
- `combat-messages/throttle.yaml` is deleted, with its guard-list entries and golden lines; the `throttle` special move keeps `narration/special-moves/throttle.yaml`.
- Goldens and the "20 files" counts are re-recorded in the same commits.

### 4. `maul` and `pounce` as default attacks

- `items.Maul` and `items.Pounce` subtype constants with descriptions, entries in `validNaturalAttacks`, and `CategoryHitNaturalSharp` in `CategoryForWeaponSubtype`.
- `species/3-bear.yaml` and `species/11-feline.yaml` set `natural_attack: maul` and `pounce`.
- The attack-family predicates describe the body, not the move list: `SpeciesIsClawed` is true for claws, maul and pounce; `SpeciesIsFanged` for bite. Other callers of the predicates (validators, mutation checks) keep their meaning.

### 5. Special moves as species data

Each species lists its AI and player beast moves: `beast_moves: [rake, pounce, ...]`. The species validator rejects an unknown name and a move whose body requirement the species lacks (each move keeps its anatomy check: no hands for every beast move, legs for pounce and hamstring, horns for gore, lifedrain for drain). `CanUse*` in `ai.go`, `command_readiness.go` and the action entry gates read the list instead of the attack family. Scripted `combatcommands` (Fact 12) are unchanged. `charge` joins the list only if it gains an AI gate (see bears below); until then it stays scripted.

Table (owner review: approve with the spec; rows not listed have no beast moves today and get none):

| Id | Species | Default attack | Beast moves today | Proposed | Why |
|---|---|---|---|---|---|
| 2 | canine | bite | maul, throttle, pounce, hamstring | maul, throttle, pounce, hamstring | wolves do all four |
| 3 | bear | claws → **maul** | rake, pounce, hamstring | rake, charge (scripted on bear mobs) | a swipe and a bull rush; bears do not pounce or hamstring |
| 6 | boar | gore | gore | gore | |
| 7 | deer | gore | gore | gore | |
| 8 | serpent | bite | maul, throttle | none | a bite is the serpent's whole attack; constrict is a later species |
| 9 | raptor | claws | rake, pounce, hamstring | rake, pounce | a stooping strike; no hamstring from the air |
| 10 | rodent | bite | maul, throttle, pounce, hamstring | hamstring | ankle-biters |
| 11 | feline | claws → **pounce** | rake, pounce, hamstring | rake, pounce, hamstring, **throttle** | the throat crusher (ruling F) |
| 12 | insectoid | bite | maul, throttle, pounce, hamstring | none | |
| 13 | fish | bite | maul, throttle | none | |
| 14 | carnivorous plant | bite | maul, throttle | none | |
| 17 | arachnid | bite | maul, throttle, pounce, hamstring | pounce | jumping spiders |
| 18 | worm | bite | maul, throttle | none | |
| 21 | reptile | bite | maul, throttle, pounce, hamstring | maul | the crocodile's roll |
| 22 | bat | claws | rake, pounce, hamstring | rake | |
| 24 | mustelid | bite | maul, throttle, pounce, hamstring | throttle, pounce | weasels kill with a neck bite |
| 32, 33, 34 | wraith, spectre, vampire | siphon, siphon, claws | drain | drain | |

Bears' `charge`: add `charge` to the two bear mobs' `combatcommands` (data only), so bears bull-rush without an AI gate. An AI-chosen `charge` for every bear is out of scope.

### 6. Testing

- Unit: `AttackSubtype` for the three cases; a wolf's engage and wait lines come from `bite.yaml`; a bear's swing from `maul.yaml`; a cat's from `pounce.yaml`; a bow user's engage line from `shooting.yaml`'s `prepare`.
- A table test pins every species' beast moves to section 5's Proposed column, and a guard proves the `CanUse*` gates read the list (a species whose list omits a move cannot use it even with the matching attack family).
- Goldens re-recorded only for the deliberate changes, each diff quoted in its commit.
- Boot check, and a playtest: a cat and a wolf fight a player while a second player watches; the engage line, wait lines and swings read the same from both seats; the cat throttles, the rat does not.

## Out of scope

- Creatures reaching expert and master tiers (#527).
- The room line for a deflected blow and "swing" in dodge lines (#521).
- Anatomy-blind combat text (#563) and the off-hand tier (#564).
- A python or constrict attack (no species yet).
- An AI gate for `charge`.

## Risks

| Risk | Answer |
|---|---|
| Changing bears' and cats' default attack changes which special moves they qualify for | Section 5 decouples moves from the attack family before section 4 switches the attack; the table test pins the result |
| An engage line now varies and could repeat a weapon name the reader cannot see | Names hidden at each reader's sight, as today; the dark test for the attack command extends to the new lines |
| Retiring `separate` breaks a path not yet found | Fact 7 plus a test that renders no `separate` key; a grep of every `.Separate` use in the plan |
