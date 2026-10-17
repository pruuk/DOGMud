# Combat-message reachability and species beast moves

Issues #541 (combat-message reachability) and #539 (unreachable throttle,
pounce and maul files), under the combat narration epic #385, plus the
species corrections salvaged from PR #208. Owner rulings recorded on #541
and #539 on 2026-10-16. This spec designs the whole change in two slices,
each with its own plan (see Slicing).

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
| 14 | Many creatures carry the wrong species on master: the Ashwick timber wolf and forest fox are rodents (10), the Marches farm cat a reptile (21), the Greenford river otter and the east road hawk serpents (8), the Ashwick chicken an orb (20), the Marches feral hog a deer (7) | `git show origin/master:<mob file>` |
| 15 | PR #208 (FinalTwist, open) corrects 32 mob species and adds `species/25-bird.yaml` (claws, legs, small) and `species/26-horse.yaml` (slam, legs and tail, large); every one of the 32 mobs still carries #208's base species on master, so the port applies without drift. Moves by count: 9→25 ×4, 8→12 ×3, 10→2 ×3, 8→25, 8→24, 20→25, 10→12, 1→26 ×2 each, and one each of 8→9, 8→21, 8→13, 8→10, 7→6, 22→9, 21→11, 21→10, 20→9, 1→21, 1→2, 1→10 | review worktree `C:/tmp/dogmud-pr208-review` at `3ce674451`, diff against its base `c696c117a`; salvage survey item 13 (`docs/audits/2026-10-05-pr208-review/salvage-survey.md:46`) |

## Owner rulings (2026-10-16)

- **A.** `prepare` is shown at the engage moment, drawn by the same weapon-type rule as the swing path, and replaces the hand-written engage lines (player and mob).
- **B.** `wait` follows the swing path's weapon-type rule, so a creature waits with its own natural attack.
- **C.** What cannot render is retired: `shooting.yaml`'s swing intensities and every `separate` block, with their render path. `shooting.yaml` keeps `prepare` and `wait`.
- **D.** Bears' default attack becomes `maul`, cats' becomes `pounce`. Serpents stay on bite. No python species (none exist). `throttle` stays a special move only; the default-attack `combat-messages/throttle.yaml` is retired.
- **E. Principle.** A species' default attack is what it does most of the time. Its special moves are rarer or finishing moves.
- **F.** Which special moves a species can use becomes species data, reviewed as a table (section 5). `throttle` goes to the throat crushers: cats, canines, and any species the table names. Bears do not get the `maul` special move; their basic swings use maul lines.
- **G.** The species corrections in PR #208 are ported as slice 1 of this work, ahead of everything else, because section 5 grants moves by species and a wolf filed as a rodent would get a rodent's moves.
- **H.** Each plan updates the relevant `context.md` files with the principle (ruling E) and the species-data rule (ruling F), and puts a short code comment where a future author will meet them: the species `BeastMoves` field and the `CanUse*` gates.

## Slicing

Two plans, in order:

1. **Slice 1: species corrections** (section 0). Data only plus the two new species; no behaviour change beyond each creature now being the species it looks like.
2. **Slice 2: reachability and beast moves** (sections 1 to 6).

## Design

### 0. Slice 1: species corrections from PR #208

- Port `species/25-bird.yaml` and `species/26-horse.yaml` from the #208 review worktree, keeping only the fields master's species loader reads (read `internal/species/species.go`); any #208-only field (hunting part tables, alliance data) is dropped and listed in the plan, since hunting is salvage item 8, not this work. Both pass the species validator (`validNaturalAttacks`: claws and slam are allowed today).
- Apply the 32 `speciesid` changes (Fact 15) to the mob files on master, each checked against #208's base value before it is changed.
- What a player notices: a wolf fights, hunts and is described as a wolf; a hawk or chicken as a bird; a horse as a horse. Each mob's AI beast moves follow its new species under today's rules until slice 2 moves them to data.
- Tests: a table test pins the 32 mobs to their new species; the boot check loads every zone.
- PR #208 is told on the PR that its species corrections landed separately, with the commit.

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
| 25 | bird (new, slice 1) | claws | rake, pounce, hamstring (claws, legs, no hands) | none | crows, chickens and grouse peck; hunting birds of prey are raptors |
| 26 | horse (new, slice 1) | slam | none | none | a kick or a trample is a later move |
| 32, 33, 34 | wraith, spectre, vampire | siphon, siphon, claws | drain | drain | |
| 44 | smoke elemental | claws (slam split) | none | **smother** (new, mob-only) | smoke pours into the target's face; copy in Appendix A |

Bears' `charge`: add `charge` to the two bear mobs' `combatcommands` (data only), so bears bull-rush without an AI gate. An AI-chosen `charge` for every bear is out of scope.

### 6. Testing

- Unit: `AttackSubtype` for the three cases; a wolf's engage and wait lines come from `bite.yaml`; a bear's swing from `maul.yaml`; a cat's from `pounce.yaml`; a bow user's engage line from `shooting.yaml`'s `prepare`.
- A table test pins every species' beast moves to section 5's Proposed column, and a guard proves the `CanUse*` gates read the list (a species whose list omits a move cannot use it even with the matching attack family).
- Goldens re-recorded only for the deliberate changes, each diff quoted in its commit.
- Boot check, and a playtest: a cat and a wolf fight a player while a second player watches; the engage line, wait lines and swings read the same from both seats; the cat throttles, the rat does not.

### 7. Docs and comments (ruling H)

- `internal/species/context.md`: the principle (a default attack is what a creature does most of the time; a special move is rarer or a finisher), `natural_attack` versus `beast_moves`, the validator's body checks, and how to add a species.
- `internal/combat/context.md`: one weapon-type rule (`AttackSubtype`), the engage lines from `prepare`, and that `CanUse*` reads the species list while the family predicates describe the body.
- `internal/actions/context.md`: the readiness and action-entry gates read the same list.
- Code comments: on the species `BeastMoves` field (the principle in one sentence and where the table lives), and above the `CanUse*` gates in `ai.go` (moves come from species data, never from the attack family).
- Slice 1 adds the bird and horse rows to `internal/species/context.md`'s species notes if it keeps a list.

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

## Appendix A: `smother` copy (owner-approved 2026-10-16, slam split #541)

The slam split spec (`2026-10-16-slam-split-design.md`, section 3) gives the smoke elemental (species 44) `claws` with the unarmedname "smoke-wrought claws", plus a new mob-only special move, `smother`. The copy was drafted and approved with the slam split; it lands here because an unreferenced special-move file or condition fails the store guards, so it ships with the move's code in slice 2.

Rulings that come with it (owner, 2026-10-16):

- On a hit the target is Blinded (condition 3) and Smothered (a new condition). A partial applies neither, and its lines claim neither.
- `cast_interrupt` is kept: smoke can choke a spell off mid-cast, as throttle does. It rides the shared special-move cooldown (`Cooldowns["special-move"]`, which every `CanUse*` gate refuses on), so it can happen at most once per cooldown window, never every round. Slice 2 adds a test that a second smother inside the cooldown is refused.
- Smothered keeps its per-round tick line (Throttled, 89, has none).
- Names stay bare `{actor}` / `{actee}`, as in `throttle.yaml`; the mob command wraps them in `moveIdentities`. Every line fits 80 columns with 20-character names and `{damage}` as "devastating wounds".

`_datafiles/world/dogmud/narration/special-moves/smother.yaml`:

```yaml
moveid: smother
# Mob-only: the smoke elemental's special move (slam split spec, section 3).
# Variant N of every role describes one moment. On a hit the target is
# Blinded (condition 3) and Smothered; the hit lines say both.
# The attacker has no body: smoke and embers only.
events:
  hit:
    actor:
      - 'You pour into {actee}''s eyes and lungs! (<ansi fg="damage">{damage}</ansi>)'
      - 'Your smoke fills {actee}''s eyes and throat! (<ansi fg="damage">{damage}</ansi>)'
      - 'You wrap {actee} in choking, blinding smoke! (<ansi fg="damage">{damage}</ansi>)'
      - 'Your embers blind {actee} as they choke! (<ansi fg="damage">{damage}</ansi>)'
      - 'You smother {actee}. They gasp, blinded! (<ansi fg="damage">{damage}</ansi>)'
    actee:
      - '{actor} pours into your eyes and lungs! (<ansi fg="damage">{damage}</ansi>)'
      - '{actor} fills your eyes and throat with smoke! (<ansi fg="damage">{damage}</ansi>)'
      - '{actor} wraps you in choking, blinding smoke! (<ansi fg="damage">{damage}</ansi>)'
      - '{actor}''s embers blind you as you choke! (<ansi fg="damage">{damage}</ansi>)'
      - '{actor} smothers you. You gasp, blinded! (<ansi fg="damage">{damage}</ansi>)'
    observer:
      - '{actor} pours into {actee}''s eyes and lungs!'
      - '{actor} fills {actee}''s eyes and throat with smoke!'
      - '{actor} wraps {actee} in choking, blinding smoke!'
      - '{actor}''s embers blind {actee}, who chokes!'
      - '{actor} smothers {actee}, who gasps, blinded!'
  # Smoke chokes a spell off mid-cast, as throttle does (combat_throttle.go).
  # It rides the shared special-move cooldown, so at most once per window.
  cast_interrupt:
    actor:
      - 'Your smoke chokes {actee}''s spell off mid-word!'
      - '{actee}''s spell collapses as they cough and gasp!'
    actee:
      - '{actor}''s smoke chokes your spell off mid-word!'
      - 'Your spell collapses as you cough and gasp!'
    observer:
      - '{actor}''s smoke chokes {actee}''s spell off mid-word!'
      - '{actee} coughs and gasps, and their spell collapses!'
  # A partial applies no condition (conditions ride on hit only),
  # so these lines claim neither blindness nor choking.
  partial:
    actor:
      - '{actee} turns aside, but your embers sting! (<ansi fg="damage">{damage}</ansi>)'
      - '{actee} holds their breath; you still sting! (<ansi fg="damage">{damage}</ansi>)'
      - 'You singe {actee} as they twist away! (<ansi fg="damage">{damage}</ansi>)'
    actee:
      - 'You turn aside, but {actor}''s embers sting! (<ansi fg="damage">{damage}</ansi>)'
      - 'You hold your breath; {actor} still stings! (<ansi fg="damage">{damage}</ansi>)'
      - 'You twist away, but {actor} singes you! (<ansi fg="damage">{damage}</ansi>)'
    observer:
      - '{actee} turns aside, but {actor}''s embers sting!'
      - '{actee} holds their breath, but {actor} still stings!'
      - '{actee} twists away, but {actor} singes them!'
  miss:
    actor:
      - 'You billow toward {actee}''s face, but miss!'
      - 'Your smoke drifts past {actee}!'
      - '{actee} ducks under your rolling smoke!'
      - '{actee} steps clear before you can engulf them!'
    actee:
      - '{actor} billows toward your face, but misses!'
      - '{actor}''s smoke drifts past you!'
      - 'You duck under {actor}''s rolling smoke!'
      - 'You step clear before {actor} can engulf you!'
    observer:
      - '{actor} billows toward {actee}''s face, but misses!'
      - '{actor}''s smoke drifts past {actee}!'
      - '{actee} ducks under {actor}''s rolling smoke!'
      - '{actee} steps clear before {actor} can engulf them!'
```

`_datafiles/world/dogmud/conditions/NN-smothered.yaml` (slice 2 picks the next free id; the tick values mirror Throttled, 89, and are balance placeholders to check against `config.yaml` and 89 when the code lands):

```yaml
conditionid: NN
name: Smothered
description: Smoke fills your lungs, and you can barely breathe.
start_actee: Smoke pours down your throat and you choke on it.
trigger_actee: You cough up smoke and fight for breath.
end_actee: You cough out the last of the smoke and breathe freely again.
triggerrate: 1 round
triggercount: 3
tick_pool: stamina
tick_percent: -0.06
tick_min: 2
flags:
  # The smother combat move narrates the smoke as it lands, and it has to
  # apply the condition synchronously, so Condition_ApplyConditions never
  # sees this condition and the start_actee line above is not its own line
  # to send.
  - silent-start
```
