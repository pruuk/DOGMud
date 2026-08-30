# U11 filed findings: help copy, config and context.md

**Date:** 2026-08-30  
**Source:** U11 arc closer, `docs/superpowers/plans/2026-08-30-u11-arc-closer.md`

Filed, not fixed. Each item is real; each was out of scope for a closing
docs slice. Verify against source before acting -- this is a point-in-time
report.

---

## 1. Em dashes in help templates (154 files, 321 occurrences)

Found by inverting `u8ActionHelpPaths` into a whole-tree walk (U11 Task 7).
House style bans en/em dashes in player-facing copy, but that guard was
authored for the U8 action helpfiles. Applying it to all 454 templates is a
**policy decision, not a coverage fix**, and a mechanical substitution across
this much prose would damage sentences -- so the em-dash half of
`findForbiddenU8HelpDisclosures` stays scoped to `u8ActionHelpPaths` and the
numeric half runs tree-wide.

Owner decision 2026-08-30: fix the numbers now, file the dashes.

| Template | Count |
|---|---|
| `about` | 11 |
| `achievements` | 1 |
| `alchemy` | 4 |
| `alias` | 1 |
| `apex-predator` | 1 |
| `arena` | 2 |
| `assess` | 2 |
| `attack` | 8 |
| `bartering` | 3 |
| `blacksmithing` | 2 |
| `blood-boil` | 2 |
| `blood-frenzy` | 1 |
| `booming-lungs` | 2 |
| `cattail-down-cloak` | 1 |
| `chat` | 1 |
| `chrysalis-cocoon` | 1 |
| `chrysalis-haste` | 1 |
| `chrysalis-regeneration` | 1 |
| `chrysalis-setting` | 1 |
| `cloth-bandage` | 1 |
| `cloth-pants` | 1 |
| `colossus-form` | 2 |
| `commanding-presence` | 1 |
| `communion-of-flesh` | 2 |
| `conditions` | 2 |
| `conviction-armor` | 1 |
| `conviction-barrage` | 1 |
| `conviction-spike` | 2 |
| `conviction-surge` | 1 |
| `conviction-ward` | 1 |
| `cooking` | 1 |
| `copper-ring` | 1 |
| `core-discharge` | 4 |
| `core-drain` | 2 |
| `craft` | 9 |
| `death` | 1 |
| `dense-muscles` | 3 |
| `devtool` | 1 |
| `discorporation` | 1 |
| `dissonance-organ` | 1 |
| `dogmud` | 5 |
| `drink` | 2 |
| `drowned-veil` | 1 |
| `eat` | 1 |
| `empathic-shroud` | 1 |
| `energy-bread` | 1 |
| `engraved-bracelet` | 1 |
| `equipment` | 1 |
| `evil-eye` | 1 |
| `extra-arms` | 2 |
| `flee` | 1 |
| `forage` | 1 |
| `foraging` | 1 |
| `gc` | 1 |
| `gearup` | 1 |
| `gem-set-necklace` | 1 |
| `gold-signet-ring` | 1 |
| `grilled-meat` | 1 |
| `heal` | 2 |
| `hemorrhagic-burst` | 1 |
| `hemorrhagic-wave` | 1 |
| `herbal-tea` | 1 |
| `hollow-bones` | 2 |
| `hunter-eel-scale-vest` | 1 |
| `iron-buckler` | 1 |
| `iron-dagger` | 2 |
| `iron-short-sword` | 2 |
| `item-names` | 2 |
| `jewelcrafting` | 2 |
| `keen-senses` | 1 |
| `kinetic-backlash` | 1 |
| `kinetic-hurl` | 2 |
| `kinetic-shove` | 3 |
| `lake-iron-hook-spear` | 2 |
| `lake-tonic-of-steady-hand` | 1 |
| `leather-leggings` | 1 |
| `leather-satchel` | 1 |
| `leather-vest` | 1 |
| `linen-tunic` | 1 |
| `mail` | 1 |
| `map` | 13 |
| `masterwork-gem-pendant` | 1 |
| `masterwork-plate-helm` | 1 |
| `mend-wounds` | 1 |
| `mind-fog` | 2 |
| `mind-spike` | 2 |
| `mutation-catalyst` | 1 |
| `mutations` | 7 |
| `nerve-disruption` | 2 |
| `neural-stun` | 3 |
| `neural-toxin` | 2 |
| `newbie` | 1 |
| `oasis` | 5 |
| `ossified-frame` | 3 |
| `pet` | 1 |
| `petition` | 2 |
| `polished-stone-amulet` | 1 |
| `prehensile-tail` | 2 |
| `psychic-anchor` | 1 |
| `pyretic-surge` | 1 |
| `quicksilver-nerves` | 2 |
| `radiant-avatar` | 1 |
| `reach` | 1 |
| `reinforced-wooden-shield` | 1 |
| `remove` | 1 |
| `rending-claws` | 1 |
| `rep` | 1 |
| `repair-pulse` | 3 |
| `reply` | 1 |
| `report` | 1 |
| `resonant-larynx` | 2 |
| `rhetoric` | 5 |
| `search` | 4 |
| `second-sight` | 1 |
| `sensory-overload` | 2 |
| `sensory-veil` | 2 |
| `set-prompt` | 1 |
| `setdesc` | 1 |
| `silver-ring` | 1 |
| `simple-pendant` | 1 |
| `skill-attunement` | 1 |
| `skills` | 16 |
| `sparks` | 1 |
| `species` | 1 |
| `spellcasting` | 1 |
| `spells` | 7 |
| `spiracle-lungs` | 1 |
| `steel-buckler` | 1 |
| `steel-longsword` | 2 |
| `sticky-secretion` | 1 |
| `submission` | 6 |
| `suicide` | 1 |
| `surrender` | 4 |
| `synaptic-overload` | 1 |
| `tail` | 3 |
| `tailoring` | 2 |
| `tame` | 1 |
| `thick-hide` | 3 |
| `ticks` | 4 |
| `titan-growth` | 2 |
| `title` | 5 |
| `toxicity` | 2 |
| `track` | 4 |
| `trade` | 1 |
| `trail-rations` | 1 |
| `translucent-body` | 2 |
| `tremorsense` | 3 |
| `triggers` | 10 |
| `veil-rend` | 2 |
| `veiling-musk` | 1 |
| `vital-surge` | 1 |
| `weather` | 1 |
| `wool-cloak` | 1 |
| `zealous-conviction` | 2 |

---

## 2. `gmcp` module introduces two help categories not in the main index

Found by the U11 Task 12 category sweep. `help` groups topics by category, and
module overlays merge last-write-wins onto ONE flat map, so any category name a
module uses that the main file does not becomes a NEW heading in the rendered
index.

Main categories (13): `all`, `character`, `combat`, `communication`,
`configuration`, `crafting`, `general`, `information`, `items`, `locks`,
`parties`, `quests`, `shops`.

`modules/gmcp/files/data-overlays/keywords.yaml` adds:

| Category | Entries |
|---|---|
| `interface` | client, checkclient, mudletmap, mudletui |
| `integration` | discord |

**Not fixed, and not obviously wrong.** Unlike the auctions (`shop` vs `shops`)
and cleanup (`information` overriding `character`/`items`) defects that U11 did
fix, these are not near-duplicates of an existing category. `interface` is a
real grouping of four client topics. `integration` holding a single entry is
thin and `communication` already exists, but folding it in is a user-visible
reorganisation nobody asked for, and `modules/` is upstream-facing.

Decide deliberately rather than by drift.

---

## 3. ⚠️ TRAP: eleven help topics are provided by MODULES, not world data

`templates.readFile` searches the registered module filesystems **before** the
datafiles root, so these resolve at runtime and `help <topic>` works for all of
them -- but they do **not** exist under
`_datafiles/world/dogmud/templates/help/`:

`auction` (auctions) · `bury`, `trash` (cleanup) · `follow` (follow) ·
`checkclient`, `client`, `discord`, `mudletmap`, `mudletui` (gmcp) ·
`leaderboard` (leaderboards) · `time` (time)

**This already caused a defect during U11.** The cross-reference guard resolves
against the dogmud world root only, so when it was widened to the whole help
tree it reported `help time` in `sleep.template` as a broken link. It was
"fixed" by deleting the reference -- removing a link that worked -- before
anyone checked `modules/time/files/datafiles/templates/help/time.template`.
Caught and reverted the same day.

`internal/templates/u8_help_test.go` now carries `moduleProvidedHelpTopics` to
skip these. **Regenerate that list when a module adds a help topic:**

```
find modules -path "*files/datafiles/templates/help/*.template" -exec basename {} .template \;
```

A guard that reports working links as broken makes help worse, because the
cheapest way to satisfy it is to delete the link.

---

## 4. 🔴 FIVE balance knobs are silently inert at zero (FILED, not fixed)

The trap: **a knob with a non-zero advertised default, a validator whose
condition is FALSE at zero, and no key in `config.yaml` is permanently stuck at
0.** An absent key unmarshals to the zero value, `Balance.Validate()` applies no
defaults before unmarshal (it only calls the six sub-validators), and a
`if x < 0 { x = default }` check cannot repair a zero. This is exactly how the
five `SurpriseAttack*Penalty` knobs U10d deleted came to auto-hit every limb.

The earlier sweep covered only the `< 0 || > 1.0` shape and found nothing. This
one generalises it: **every** validator whose condition is false at zero, cross
referenced against key presence in `config.yaml`. 35 such validators; 5 have no
key.

| Knob | Advertised default | Actual live value | Consequence |
|---|---|---|---|
| `StealCooldown` | 60 | **0** | steal has no cooldown; spammable every round |
| `ShadowCooldown` | 5 | **0** | shadow has no cooldown |
| `SneakFailCooldown` | 3 | **0** | no cooldown after a failed sneak (guarded by `> 0`, so it simply never applies) |
| `StealHiddenBonus` | 25 | **0** | stealing or planting while hidden gets NO bonus |
| `PackScatterRounds` | 2 | **0** | a pack does not scatter when its alpha dies |

Verified: consumers are `actions/steal.go:111,130`, `actions/plant.go:95,112`,
`actions/shadow.go:66`, `usercommands/skill.skullduggery.sneak.go:65`,
`mobs/pack_roaming.go:210`. All read the config value directly with no second
default.

**NOT FIXED, deliberately.** Adding these keys restores documented intent but
changes live behaviour in five places at once, three of them skullduggery
economy. A config edit inside a documentation slice is how an unreviewed balance
change ships. This wants its own slice with a playtest.

Note also a copy inconsistency at `actions/steal.go:108-114`: the cooldown is
formatted as `"%d real seconds"` but the refusal reports `"%d rounds
remaining"`.

**Two false positives from the same sweep, recorded so nobody re-reports them:**
`DrainHealRatio`'s validator is `<= 0`, which DOES fire at zero;
`ShopMaterialReserve` IS present in `config.yaml` (its `,omitempty` tag suffix
made a naive tag match miss it).

---

## 5. Six orphaned `config.yaml` keys removed (FIXED in U11)

Keys with **zero** Go readers anywhere in `internal/` or `modules/`. Removing a
key nothing reads is provably not a behaviour change, which is why this was in
scope where item 4 was not.

| Key | Why it is dead |
|---|---|
| `MobConverseChance: 3` | no reader; superseded by the conversations system |
| `GlobalDefenseMultiplier: 1.0` | no reader |
| `SpellAvoidanceDamageMultiplier: 0.50` | deleted by U6 Task 12 (see `config.balance.go:317`) |
| `RhetoricAvoidanceDamageMultiplier: 0.50` | deleted by U6 Task 12 |
| `sub_skill_weight: 1.5` | `SubSkillWeight` deleted by U6b |
| `sub_crit_z_threshold: 2.0` | `SubCritZThreshold` deleted by U6b |

`SpellAttackSkillFactor` was already absent; U6b removed it cleanly.

---

## 6. CORRECTION: the roadmap's floor-pair warning is obsolete

`UNIFIED_RESOLUTION_ROADMAP.md` warns U11's config audit that *"the three floor
pairs all ship at 0.05, which makes a wrong-pair wiring invisible in production
-- do not 'simplify' them into one knob during a tidy-up; they are one value by
coincidence, not by rule."*

**Those knobs no longer exist.** U6 already collapsed all eight per-channel
floors into a single `ContestFloor` (shipped **0.125**), with `ConcentrationFloor`
(0.02) as a deliberately separate, smaller mercy band. `MinContestSuccessChance`,
`MinSpellHitChance` and `MinManeuverHitChance` appear in `config.yaml` only
inside comments explaining what was deleted.

The warning was correct when written and is now a description of a hazard that
was already designed out. Kept here rather than silently dropped so a reader who
remembers it can see how it closed.

---

## 7. Newbie playtest findings, 2026-08-30 (run `9828eedc9fdaf7db`)

Report: `tools/playtest/reports/2026-08-30-local-feel-tester-u11-newbie-help.md`
(gitignored). Extracted here because reports are not committed.

### ✅ What the run VERIFIED about U11

- **Contested prone recovery WORKS**, observed from the attacker's side: five
  consecutive `Straw Effigy attempts to stand, but slips and falls in the chaos
  of battle.` followed by `Straw Effigy clambers to their feet in a rushed
  panic.` That failure line had not printed for any player since U10.
- **All three restored links resolve**: `help time`, `help dual-wield`,
  `help wimpy`.
- **Link rot is gone.** 24 topics opened, every followed link resolved, zero
  errors -- including 13 topics reachable only by following a page link.
- **No duplicate headings.** 14 distinct; `auction` sits under Shops with seven
  siblings.
- `help prone`, `help stand`, `help armor`, `help quell`, `help defy` all
  explain mechanics with no numbers.

### 🔴 Fixed in response

- `Integration` held `discord` alone -- a stranded one-entry heading, the exact
  shape the goal forbids. Moved to `communication`. The tester also read
  Configuration / Interface / Integration as "three names for the same thing".
- `help defense` had three orphaned line fragments (`on dodge` alone on a line).
  **Self-inflicted:** U11 split four over-80-column lines without re-flowing the
  paragraphs. Re-flowed.

### FILED, not fixed

1. **`help submission` prints raw HTML entities**:
   `set submission &amp;lt;mercy|subdue|cripple|lethal&amp;gt;`. Pre-existing; a
   double-escape somewhere in that template's pipeline.
2. **`kick` produced NO output at all** -- echoed, then silence, verified
   against the raw event log as the only command that round. Needs its own
   investigation; a command that answers nothing is indistinguishable from a
   dropped one.
3. **`help health` is a bare column of twelve percentage ranges** with no words
   attached. U11 exempted it from the numeric guard as a "resource-bar legend",
   reasoning the bands ARE the display. The tester's evidence argues the
   exemption was too generous for a *help page* as opposed to the bar itself.
   Revisit the exemption, or give the page prose.
4. **`bury` is filed under `character` and a newcomer could not find it** --
   they looked under Items, then Combat, then gave up. U11 restored `character`
   because that is what the main `keywords.yaml` says; the finding is that the
   main file's placement is unintuitive, not that the restore was wrong.
   `bury`/`trash` are also the only two pages using a different header format
   (`.: Help for the bury command.`).
5. **Ragged short-line wrapping is pervasive** across the help corpus. Inferred
   cause: the wrapper measures invisible markup, so lines carrying `<ansi>` tags
   break early. Worth one fix at the wrapper rather than 454 by hand.
6. **`consider straw effigy` says "You are severely outmatched"** against a
   target described as "A safe thing to learn a fight against" that dealt 3
   damage out of 406 health in fifteen rounds. `consider` was already known to
   call walkovers even; this is the same defect in the other direction.
7. **Combat vocabulary saturates**: three identical crit sentences in one round,
   a near-100% crit rate against the effigy, and `DEVASTATING BLOW!` paired with
   `(negligible damage)`. Consistent with the U10d saturation finding.
8. **Grammar: "Your fists flies wide"**, repeated 8+ times.
9. **`set charset` claims a conversion it does not perform** -- it says
   "Box-drawing characters will be converted to ASCII equivalents" and then
   leaves U+2501, em dashes, arrows and emoji in place. The toggle itself is
   known not to be a bug; the MESSAGE is what is wrong.
10. **`tools/playtest/engine-profile.yaml` is stale**: says spawn is Sanctum
    Basin (it is Pothole Coulee) and "ten skills" (the index lists 21).

### 🪤 Fixture note for the next combat playtest

**Nothing in the starting area can knock a player down.** Seven rooms explored
and scanned: Vorn is unattackable, the Training Dummy "poses no threat", the
Straw Effigy cannot hurt you, everything else is a healer, cleric or banker. No
hostile within five rooms of spawn. Combat is gated behind
`ask vorn train` after the Awakening rite. **Budget the tutorial into any run
that needs to be hit.**

### ⚠️ Tuning question for the owner

The free `! SWEEP!` re-fires every round, so the tester saw **five consecutive
failed stands**. Contested recovery is working as designed, but a knockdown
attack that repeats every round against a floored target is close to a lock.
Worth feeling on the manual pass.

---

## 8. 🔴 CONTENT GAP: almost nothing in the game can knock a player down

Found by the veteran playtest (run `881a07a678dcbe39`, 2026-08-30) and
**re-verified independently**. This is the most consequential finding of the
U11 gate, because it bounds how much the slice's behaviour change is worth.

A player goes prone by exactly two routes:

1. **A mob's bash / trip / kick special**, which requires `specialmovechance`
   or `movepreferences` on the mob. Verified count across the whole world:

   ```
   grep -rln "specialmovechance\|movepreferences" _datafiles/world/dogmud/mobs/
   ```

   **10 files. 8 are under `mobs/test*/`.** The only two live ones are
   `labyrinth_of_low_tunnels/73-warren_warrior.yaml` and
   `75-warren_chieftain.yaml`, and the tester's BFS over every room file found
   **no walkable path there within 60 moves** of the starting area. Zero of the
   70 Pothole Coulee mobs set either field.

2. **The counter-sweep**, gated on `DodgeCritDetected`
   (`combat_shared_helpers.go:328`) -- a DEFENCE crit, which a low-statpool mob
   will essentially never land on a competent player.

**Consequence for U11.** Contested prone recovery is verified working and was
observed repeatedly, but almost entirely **from the attacker's side**: a player
holds a mob down. The player-as-victim half is close to structurally
unreachable in shipped content. The patch note was rewritten to lead with the
half players will actually meet, and to say plainly that the other half is rare.

**This cannot be closed by playing harder.** It needs authored content: mobs
that set `specialmovechance`, or a purpose-built fixture. Note that a related
defect is already filed separately -- 114 mobs' `aiprofile` is inert -- so the
special-move gap may be one symptom of a wider "authored mob behaviour does not
fire" problem. Worth investigating together.

**Fixture consequence for every future combat playtest:** there is currently no
way to test player-side knockdown, prone penalties, or recovery narration
without building a mob for it. Both U11 runs lost goals to this.

---

## 9. Veteran playtest, other findings (run `881a07a678dcbe39`)

Report: `tools/playtest/reports/2026-08-30-local-bug-finder-u11-vet-combat.md`.

### ✅ Verified

- **Mob-side contested recovery PASSES**, both outcomes seen verbatim,
  repeatedly, across two independent fights.
- **All combat helpfiles pass.** `help stand` correctly documents both the new
  contest and that the paid escape stays guaranteed. All 10 cross-references
  resolve.
- Actor and observer text correctly differ: the failing character reads
  *"You attempt to stand, but slip back down in the chaos of battle!"* while the
  room reads *"...slips and falls..."*. Both are correct; only the room form was
  quoted in the goals file.

### 🔴 Fixed in response

- **Companion engagement message doubled**, reproduced in two independent
  fights: both companions announced `prepares to fight Bandit Scout.` in two
  consecutive rounds. **A U11 regression.** The reactive assist path and the
  polling `handleCharmedMobAssist` both guard on `!IsInCombat()`, and that guard
  is insufficient because `Command()` only ENQUEUES -- the flag stays false
  until the command runs, so both passed in the same window. The reactive path
  was inert before this slice, so only one system ever ran. Fixed with a
  per-character, per-round claim (`TryClaimAssistCommand`).

### FILED

- `consider` calls the deliberately harmless Straw Effigy "severely
  outmatched", weighting its 100,200 HP pool. Same defect as finding 7.6, and
  `consider` was already known to call walkovers even.
- **The Drill Yard's three NPCs are all incapable of hitting back**, so a
  combat goals file bound to room 5227 starts where nothing can test defence.
- `look <unknown-noun>` replies "Look at what???" as though no argument was
  given.
- `Your fists flies wide but somehow lands...` (grammar, and the sentence
  contradicts itself).

### 🪤 Harness note, recorded to stop the next run losing time

**Movement must be sent ONE command per round.** A batch of nine moves advanced
the tester zero rooms with no error text at all. The AI port caps at 3
commands/round and silently drops the overflow AFTER echoing it, so a dropped
command is indistinguishable from a broken one. The tester initially filed two
"silent no-op" defects that were their own rate-limit overflow, and correctly
retracted both.
